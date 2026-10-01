package gitbackend

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestNormalizePin(t *testing.T) {
	if got := normalizePin("SHA256:abc"); got != "SHA256:abc" {
		t.Fatalf("got %s", got)
	}
	if got := normalizePin("abc"); got != "SHA256:abc" {
		t.Fatalf("got %s", got)
	}
	if got := normalizePin("  SHA256:x  "); got != "SHA256:x" {
		t.Fatalf("got %s", got)
	}
}

func TestHostPortFromSSHURL(t *testing.T) {
	cases := []struct {
		raw  string
		host string
		port int
		ok   bool
	}{
		{"ssh://git@github.com:2222/o/r.git", "github.com", 2222, true},
		{"ssh://git@github.com/o/r.git", "github.com", 22, true},
		{"ssh://127.0.0.1:2222/x.git", "127.0.0.1", 2222, true},
		{"ssh://[::1]:2222/x.git", "::1", 2222, true},
		{"git@github.com:org/repo.git", "github.com", 22, true},
		{"git@10.0.0.1:repo", "10.0.0.1", 22, true},
		{"github.com:org/repo.git", "github.com", 22, true},
		// 非 SSH / 非地址
		{"https://github.com/o/r.git", "", 0, false},
		{"git://github.com/o/r.git", "", 0, false},
		{"/local/path/repo.git", "", 0, false},
		{"./relative", "", 0, false},
		{"blob:none", "", 0, false}, // --filter 的值不得误认成主机
		{"file", "", 0, false},
		{"", "", 0, false},
	}
	for _, c := range cases {
		host, port, ok := hostPortFromSSHURL(c.raw)
		if ok != c.ok || host != c.host || port != c.port {
			t.Errorf("hostPortFromSSHURL(%q) = (%q,%d,%v), want (%q,%d,%v)",
				c.raw, host, port, ok, c.host, c.port, c.ok)
		}
	}
}

func TestGitSubcommandOfSkipsConfigPairs(t *testing.T) {
	if got := gitSubcommandOf([]string{"-c", "user.name=x", "fetch", "--all"}); got != "fetch" {
		t.Fatalf("got %q", got)
	}
	if got := gitSubcommandOf([]string{"push", "origin"}); got != "push" {
		t.Fatalf("got %q", got)
	}
	if got := gitSubcommandOf(nil); got != "" {
		t.Fatalf("got %q", got)
	}
}

// knownHostsLineOf 构造与 ssh-keyscan 同构的 known_hosts 行。
func knownHostsLineOf(t *testing.T, hostport string, signer ssh.Signer) string {
	t.Helper()
	blob := base64.StdEncoding.EncodeToString(signer.PublicKey().Marshal())
	return hostport + " " + signer.PublicKey().Type() + " " + blob
}

func TestFilterPinnedLines(t *testing.T) {
	hi, err := genHostKey()
	if err != nil {
		t.Fatal(err)
	}
	other, err := genHostKey()
	if err != nil {
		t.Fatal(err)
	}
	pin := ssh.FingerprintSHA256(hi.PublicKey())

	raw := strings.Join([]string{
		"# comments skipped",
		knownHostsLineOf(t, "[host]:2222", hi),
		knownHostsLineOf(t, "[host]:2222", other), // 不匹配的键
		"garbage line",
		"",
	}, "\n")

	got, err := filterPinnedLines(raw, pin, "host", 2222)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != knownHostsLineOf(t, "[host]:2222", hi) {
		t.Fatalf("want exactly the pinned key line, got %v", got)
	}

	// pin 与全部密钥不匹配 → fail-closed
	wrong := "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if _, err := filterPinnedLines(raw, wrong, "host", 2222); err == nil {
		t.Fatal("want error for zero-match pin")
	}
}

func TestResolveSSHPin_NotApplicable(t *testing.T) {
	b := &NativeGitBackend{gitPath: "git", logger: NewNoopLogger()}

	// 非 SSH / 未设指纹 / 显式不安全：原样返回，noop 清理
	for _, auth := range []AuthConfig{
		{Type: AuthHTTPToken, Token: "t", HostKeyFingerprint: "SHA256:x"},
		{Type: AuthSSH, HostKeyFingerprint: ""},
		{Type: AuthSSH, HostKeyFingerprint: "SHA256:x", InsecureSkipTLS: true},
	} {
		out, cleanup, err := b.resolveSSHPin(context.Background(), "", []string{"push", "origin"}, auth)
		if err != nil || cleanup == nil {
			t.Fatalf("auth %+v: err=%v", auth, err)
		}
		if out.KnownHostsPath != auth.KnownHostsPath || out.HostKeyFingerprint != auth.HostKeyFingerprint {
			t.Fatalf("auth %+v should pass through unchanged, got %+v", auth, out)
		}
	}

	// 非联网子命令：钉扎不参与，不报错
	out, cleanup, err := b.resolveSSHPin(context.Background(), "", []string{"status"}, AuthConfig{Type: AuthSSH, HostKeyFingerprint: "SHA256:x"})
	if err != nil {
		t.Fatalf("non-network subcommand must not fail: %v", err)
	}
	if out.KnownHostsPath != "" || out.HostKeyFingerprint != "SHA256:x" {
		t.Fatalf("non-network subcommand should leave auth untouched: %+v", out)
	}
	cleanup()
}

func TestResolveSSHPin_FailClosedWhenHostUnresolvable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX git behavior parity")
	}
	b := &NativeGitBackend{gitPath: "git", logger: NewNoopLogger()}
	repo := createTestRepo(t) // 本地仓库,无 SSH remote
	_, _, err := b.resolveSSHPin(context.Background(), repo, []string{"push"}, AuthConfig{Type: AuthSSH, HostKeyFingerprint: "SHA256:x"})
	if err == nil || !strings.Contains(err.Error(), "无法从命令解析出远端主机") {
		t.Fatalf("want fail-closed error, got %v", err)
	}
}

// startTestSSHServer 在 127.0.0.1 随机端口起一个最小 x/crypto/ssh 服务端，
// 返回其监听地址与主机密钥指纹——ssh-keyscan 会真实连接它完成握手。
func startTestSSHServer(t *testing.T) (addr string, fingerprint string) {
	t.Helper()
	signer, err := genHostKey()
	if err != nil {
		t.Fatal(err)
	}
	// NoClientAuth：keyscan 只做 KEX 取主机密钥、不做用户认证；
	// 服务端不配认证方式会在握手期直接断连（实测）
	cfg := &ssh.ServerConfig{NoClientAuth: true}
	cfg.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(5 * time.Second))
				sconn, chans, reqs, err := ssh.NewServerConn(c, cfg)
				if err != nil {
					return // keyscan 握手后直接断开是常态
				}
				go ssh.DiscardRequests(reqs)
				for range chans {
					// keyscan 不开 channel
				}
				_ = sconn.Close()
			}(conn)
		}
	}()

	return ln.Addr().String(), ssh.FingerprintSHA256(signer.PublicKey())
}

func TestResolveSSHPin_EndToEndWithKeyScan(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires ssh-keyscan and POSIX sh")
	}
	if _, err := net.LookupHost("127.0.0.1"); err != nil {
		t.Skip("no loopback network")
	}
	addr, fingerprint := startTestSSHServer(t)
	_, portStr, _ := net.SplitHostPort(addr)
	url := fmt.Sprintf("ssh://git@%s/o/r.git", addr) // addr = 127.0.0.1:port

	b := &NativeGitBackend{gitPath: "git", logger: NewNoopLogger()}

	t.Run("pin matches server key", func(t *testing.T) {
		auth := AuthConfig{Type: AuthSSH, HostKeyFingerprint: fingerprint}
		out, cleanup, err := b.resolveSSHPin(context.Background(), "",
			[]string{"ls-remote", url}, auth)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		if out.HostKeyFingerprint != "" {
			t.Fatalf("fingerprint should be rewritten to KnownHostsPath, got %+v", out)
		}
		data, err := os.ReadFile(out.KnownHostsPath)
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		// known_hosts 里必须是这台服务器的密钥（按 [host]:port 形式命中）
		want := "[" + "127.0.0.1" + "]:" + portStr
		if !strings.Contains(content, want+" ") {
			t.Fatalf("known_hosts missing %q entry: %q", want, content)
		}
		// sshHostKeyArgs 走 known_hosts 严格校验分支
		if args := sshHostKeyArgs(out); !strings.Contains(args, "UserKnownHostsFile") {
			t.Fatalf("sshHostKeyArgs should pin UserKnownHostsFile, got %q", args)
		}
		// cleanup 后临时文件删除
		p := out.KnownHostsPath
		cleanup()
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatal("temp known_hosts not cleaned")
		}
	})

	t.Run("pin mismatch fails closed", func(t *testing.T) {
		auth := AuthConfig{Type: AuthSSH, HostKeyFingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}
		if _, _, err := b.resolveSSHPin(context.Background(), "",
			[]string{"ls-remote", url}, auth); err == nil {
			t.Fatal("want mismatch error")
		}
	})

	t.Run("fetch resolves remote from repo config", func(t *testing.T) {
		repo := createTestRepo(t)
		gitOutput(t, repo, "remote", "add", "origin", url)

		auth := AuthConfig{Type: AuthSSH, HostKeyFingerprint: fingerprint}
		out, cleanup, err := b.resolveSSHPin(context.Background(), repo,
			[]string{"fetch", "--prune", "origin"}, auth)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if out.KnownHostsPath == "" {
			t.Fatal("fetch path should resolve remote URL and pin it")
		}
		if data, _ := os.ReadFile(out.KnownHostsPath); !strings.Contains(string(data), "127.0.0.1") {
			t.Fatalf("known_hosts should contain scanned host: %q", data)
		}
	})

	t.Run("clone parses url from argv with flags", func(t *testing.T) {
		auth := AuthConfig{Type: AuthSSH, HostKeyFingerprint: fingerprint}
		out, cleanup, err := b.resolveSSHPin(context.Background(), "",
			[]string{"clone", "--filter", "blob:none", url, filepath.Join(t.TempDir(), "dst")}, auth)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if out.KnownHostsPath == "" {
			t.Fatal("clone path should pin the URL host")
		}
	})
}

// genHostKey 生成真实 ed25519 SSH 主机密钥。
func genHostKey() (ssh.Signer, error) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, err
	}
	return ssh.NewSignerFromKey(priv)
}
