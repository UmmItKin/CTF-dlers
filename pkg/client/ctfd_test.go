package client

import "testing"

// token must not leak to a host other than the configured CTFd instance
func TestFileReq_TokenOnlyForSameHost(t *testing.T) {
	cfg := DefaultClientConfig()
	cfg.BaseURL = "https://ctf.example.com"
	cfg.Token = "secret"
	c, err := NewCTFdClient(cfg)
	if err != nil {
		t.Fatalf("NewCTFdClient: %v", err)
	}

	if got := c.fileReq("https://ctf.example.com/files/x/flag.zip").Header.Get(authorizationHeader); got == "" {
		t.Error("expected auth header for same-host download")
	}
	if got := c.fileReq("/files/x/flag.zip").Header.Get(authorizationHeader); got == "" {
		t.Error("expected auth header for relative download URL")
	}
	if got := c.fileReq("https://evil.example.net/steal").Header.Get(authorizationHeader); got != "" {
		t.Errorf("token leaked to foreign host: %q", got)
	}
}

// session cookie: normalized, same-host only, never to a foreign host
func TestFileReq_CookieOnlyForSameHost(t *testing.T) {
	cfg := DefaultClientConfig()
	cfg.BaseURL = "https://ctf.example.com"
	cfg.Cookie = "abc123" // bare value -> becomes session=abc123
	c, err := NewCTFdClient(cfg)
	if err != nil {
		t.Fatalf("NewCTFdClient: %v", err)
	}

	if got := c.apiReq().Header.Get("Cookie"); got != "session=abc123" {
		t.Errorf("bare cookie not normalized: %q", got)
	}
	if got := c.fileReq("/files/x/flag.zip").Header.Get("Cookie"); got == "" {
		t.Error("expected cookie for relative download URL")
	}
	if got := c.fileReq("https://evil.example.net/steal").Header.Get("Cookie"); got != "" {
		t.Errorf("cookie leaked to foreign host: %q", got)
	}

	if got := cookieHeader("session=xyz"); got != "session=xyz" {
		t.Errorf("full cookie pair should pass through, got %q", got)
	}
}
