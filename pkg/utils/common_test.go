package utils

import "testing"

func TestExtractFilenameFromURL_NoTraversal(t *testing.T) {
	cases := map[string]string{
		"https://ctf.example.com/files/a/b/flag.zip": "flag.zip",
		"https://ctf.example.com/files/a/b/..":       "download", // must not become ".."
		"https://ctf.example.com/":                   "download",
		"https://ctf.example.com/files/.":            "download",
	}
	for in, want := range cases {
		got, err := ExtractFilenameFromURL(in)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", in, err)
		}
		if got == ".." || got == "." {
			t.Fatalf("%s: traversal filename returned: %q", in, got)
		}
		if got != want {
			t.Errorf("%s: got %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeName_NeutralizesSeparators(t *testing.T) {
	if got := SanitizeName("../../etc/passwd"); got == "" || got[0] == '/' {
		t.Fatalf("unsafe sanitized name: %q", got)
	}
	if got := SanitizeName(""); got != "unnamed" {
		t.Errorf("empty name: got %q, want unnamed", got)
	}
}
