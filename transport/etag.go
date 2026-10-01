package transport

import (
	"bytes"
	"io"
	"net/http"
	"strconv"

	lru "github.com/hashicorp/golang-lru/v2"
)

// DefaultETagEntries is the default maximum number of responses kept in an
// ETagCache.
const DefaultETagEntries = 128

// DefaultMaxCachedBodySize caps how large a response body may be to enter
// the ETag cache (1 MiB). Larger bodies (archives, tarballs, huge diffs)
// are fetched in full every time; only their pages'-worth of JSON metadata
// benefit from conditional requests.
const DefaultMaxCachedBodySize = 1024 * 1024

// cacheHitHeader marks responses served from the ETag cache so response
// hooks and callers can distinguish a network 200 from a cached one.
const cacheHitHeader = "X-Go-Git-Platform-Cache"

// ETagCache implements HTTP conditional requests (RFC 9110 §13) for the
// transport pipeline. When attached to a Client (Client.ETag), GET requests
// carry If-None-Match with the stored validator; a 304 answer is
// transparently turned back into the cached 200 response, so callers and
// third-party SDK decoders see an ordinary response while the platform
// skips serializing the body. On the platforms that meter conditional
// requests favorably (GitHub's 304s do not count against the rate-limit
// budget) this materially raises sustainable polling rates — e.g.
// wait-for-CI loops re-reading commit statuses every few seconds.
//
// Semantics and deliberate simplifications:
//
//   - Only GET requests are cached.
//   - The cache key includes the URL, the Accept header, and the
//     Authorization header, so token rotation naturally partitions the
//     cache and Accept variants never cross-contaminate.
//   - Responses carrying Cache-Control: no-store are never cached.
//   - Entries are bounded by count (LRU) and by body size.
//   - Vary is not honored beyond Accept; platforms served by this SDK do
//     not vary their API payloads on other request headers.
//
// An ETagCache is safe for concurrent use.
type ETagCache struct {
	entries     *lru.Cache[string, etagEntry]
	maxBodySize int64
}

type etagEntry struct {
	etag   string
	header http.Header
	body   []byte
}

// NewETagCache builds a cache holding at most maxEntries responses with
// bodies up to DefaultMaxCachedBodySize. maxEntries <= 0 uses
// DefaultETagEntries.
func NewETagCache(maxEntries int) *ETagCache {
	if maxEntries <= 0 {
		maxEntries = DefaultETagEntries
	}
	// lru.New returns an error only for non-positive sizes, handled above.
	c, _ := lru.New[string, etagEntry](maxEntries)
	return &ETagCache{entries: c, maxBodySize: DefaultMaxCachedBodySize}
}

// Len reports the number of cached responses (for tests and diagnostics).
func (c *ETagCache) Len() int { return c.entries.Len() }

// etagKey scopes a cache entry to the exact request shape: method+URL plus
// the Accept and Authorization headers. Authorization participates so a
// refreshed token (different access scopes or identity) never serves
// another principal's cached payload.
func etagKey(req *http.Request) string {
	return req.Method + " " + req.URL.String() + "\n" +
		req.Header.Get("Accept") + "\n" +
		req.Header.Get("Authorization") + "\n" +
		req.Header.Get("PRIVATE-TOKEN")
}

// applyConditional adds If-None-Match to req when a cached validator
// exists for it. It must run after authentication (Authorization is part
// of the cache key) and is a no-op when req already carries
// If-None-Match. Safe on a nil cache.
func (c *ETagCache) applyConditional(req *http.Request) {
	if c == nil || req.Method != http.MethodGet || req.Header.Get("If-None-Match") != "" {
		return
	}
	if e, ok := c.entries.Get(etagKey(req)); ok {
		req.Header.Set("If-None-Match", e.etag)
	}
}

// cacheable reports whether resp may be stored for this req.
func (c *ETagCache) cacheable(req *http.Request, resp *http.Response) bool {
	return req.Method == http.MethodGet &&
		resp.StatusCode == http.StatusOK &&
		resp.Header.Get("ETag") != "" &&
		resp.Header.Get("Cache-Control") != "no-store"
}

// store caches body for req/resp. Content-Encoding and Content-Length are
// dropped from the stored headers: bodies are stored decoded, and the
// synthesized response recomputes the length.
func (c *ETagCache) store(req *http.Request, resp *http.Response, body []byte) {
	if !c.cacheable(req, resp) || int64(len(body)) > c.maxBodySize {
		return
	}
	h := resp.Header.Clone()
	h.Del("Content-Encoding")
	h.Del("Content-Length")
	c.entries.Add(etagKey(req), etagEntry{
		etag:   resp.Header.Get("ETag"),
		header: h,
		body:   append([]byte(nil), body...),
	})
}

// synthesize rebuilds a 200 response for a 304 using the cached entry. The
// cache-hit marker header lets response hooks observe conditional hits.
func (e etagEntry) synthesize(req *http.Request) *http.Response {
	h := e.header.Clone()
	h.Set(cacheHitHeader, "hit")
	return &http.Response{
		Status:        http.StatusText(http.StatusOK),
		StatusCode:    http.StatusOK,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        h,
		Body:          io.NopCloser(bytes.NewReader(e.body)),
		ContentLength: int64(len(e.body)),
		Request:       req,
	}
}

// process handles a completed Client.do-style exchange where the body has
// already been read in full: 304s are turned into the cached 200 (with the
// cached body returned as outBody), and 200s are stored. Everything else
// passes through unchanged.
func (c *ETagCache) process(req *http.Request, resp *http.Response, body []byte) (*http.Response, []byte) {
	if c == nil || resp == nil {
		return resp, body
	}
	switch resp.StatusCode {
	case http.StatusNotModified:
		if e, ok := c.entries.Get(etagKey(req)); ok {
			out := e.synthesize(req)
			return out, e.body
		}
	case http.StatusOK:
		c.store(req, resp, body)
	}
	return resp, body
}

// processRT handles a completed RoundTripper-style exchange whose body is
// still streaming: 304s are replaced by the cached 200, and cacheable 200s
// are buffered (so they can be stored) and re-wrapped for single
// consumption by the caller. Non-cacheable responses are returned
// untouched.
func (c *ETagCache) processRT(req *http.Request, resp *http.Response) *http.Response {
	if c == nil || resp == nil || resp.Body == nil {
		return resp
	}
	if resp.StatusCode == http.StatusNotModified {
		if e, ok := c.entries.Get(etagKey(req)); ok {
			_ = resp.Body.Close()
			return e.synthesize(req)
		}
		return resp
	}
	if !c.cacheable(req, resp) {
		return resp
	}
	// Read the full body once, then decide: within the size cap it enters
	// the cache; beyond it we merely replay it.
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		// A mid-body failure must not silently truncate: surface an empty
		// body with the original status so SDK decoders see the error they
		// would have seen anyway.
		resp.Body = io.NopCloser(bytes.NewReader(nil))
		resp.ContentLength = 0
		return resp
	}
	c.store(req, resp, body)
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Del("Content-Encoding")
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return resp
}
