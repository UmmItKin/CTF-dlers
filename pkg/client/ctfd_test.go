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
