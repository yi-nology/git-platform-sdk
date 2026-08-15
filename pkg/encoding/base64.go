package encoding

import "encoding/base64"

// Base64Encode returns the Base64 standard encoding of a string.
func Base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// Base64Decode decodes a Base64 standard encoded string.
func Base64Decode(s string) (string, error) {
	rs, err := base64.StdEncoding.DecodeString(s)
	return string(rs), err
}

// Base64URLEncode returns the Base64 URL-safe encoding of a string.
func Base64URLEncode(s string) string {
	return base64.URLEncoding.EncodeToString([]byte(s))
}

// Base64URLDecode decodes a Base64 URL-safe encoded string.
func Base64URLDecode(s string) (string, error) {
	rs, err := base64.URLEncoding.DecodeString(s)
	return string(rs), err
}
