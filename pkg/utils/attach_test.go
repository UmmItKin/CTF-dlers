package utils

import "testing"

func TestExtractAttachmentURLs(t *testing.T) {
	desc := "Read the rules at https://ctf.example.com and join https://discord.gg/abc\n\n" +
		"## Attachments\n" +
		"* [log.pcap](https://s3.amazonaws.com/f/log.pcap)\n" +
		"* [chall](https://nyc3.digitaloceanspaces.com/f/chall)\n" + // no ext, markdown -> keep
		"* [dup](https://s3.amazonaws.com/f/log.pcap)\n" + // dup -> drop
		"* [gdrive](https://drive.google.com/file/d/abc/view)\n" + // viewer -> drop
		"* [proton](https://drive.proton.me/urls/xyz)\n" + // viewer -> drop
		"Bare direct file: https://fra1.digitaloceanspaces.com/f/dump.zip.\n" + // bare + ext -> keep
		"Bare site: https://example.com/about\n" // bare, no ext -> drop

	got := ExtractAttachmentURLs(desc)
	want := map[string]bool{
		"https://s3.amazonaws.com/f/log.pcap":            true,
		"https://nyc3.digitaloceanspaces.com/f/chall":    true,
		"https://fra1.digitaloceanspaces.com/f/dump.zip": true,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %d urls", got, len(want))
	}
	for _, u := range got {
		if !want[u] {
			t.Errorf("unexpected url kept: %q", u)
		}
	}

	if ExtractAttachmentURLs("no links here") != nil {
		t.Error("expected nil for no links")
	}
}
