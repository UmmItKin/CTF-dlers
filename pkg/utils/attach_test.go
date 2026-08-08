package utils

import "testing"

func TestExtractAttachmentURLs(t *testing.T) {
	desc := "Help!\n\n## Attachments\n" +
		"* [log.pcap](https://s3.example.com/Forensics/log.pcap)\n" +
		"* [same](https://s3.example.com/Forensics/log.pcap)\n" + // dup
		"* [drive](https://drive.google.com/file/d/abc/view)\n" + // viewer, skip
		"* [local](https://localhost:8088/x)\n" // skip
	got := ExtractAttachmentURLs(desc)
	if len(got) != 1 || got[0] != "https://s3.example.com/Forensics/log.pcap" {
		t.Fatalf("got %v", got)
	}
	if ExtractAttachmentURLs("no links here") != nil {
		t.Error("expected nil for no links")
	}
}
