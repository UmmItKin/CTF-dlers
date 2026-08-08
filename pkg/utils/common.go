package utils

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

func SanitizeName(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "*", "_")
	name = strings.ReplaceAll(name, "?", "_")
	name = strings.ReplaceAll(name, "\"", "_")
	name = strings.ReplaceAll(name, "<", "_")
	name = strings.ReplaceAll(name, ">", "_")
	name = strings.ReplaceAll(name, "|", "_")

	re := regexp.MustCompile(`[^\w\-_.]`)
	name = re.ReplaceAllString(name, "_")

	re = regexp.MustCompile(`_+`)
	name = re.ReplaceAllString(name, "_")

	name = strings.Trim(name, "_")

	if name == "" {
		name = "unnamed"
	}

	if len(name) > 100 {
		name = name[:100]
	}

	return name
}

func ExtractFilenameFromURL(fileURL string) (string, error) {
	parsedURL, err := url.Parse(fileURL)
	if err != nil {
		return "", err
	}

	filename := filepath.Base(parsedURL.Path)

	// reject traversal/edge names
	if filename == "" || filename == "/" || filename == "." || filename == ".." {
		filename = "download"
	}

	return filename, nil
}

var (
	mdLinkRe  = regexp.MustCompile(`\[[^\]]*\]\((https?://[^)\s]+)\)`)
	bareURLRe = regexp.MustCompile(`https?://[^\s)\]<>"']+`)
	fileExtRe = regexp.MustCompile(`\.[A-Za-z0-9]{1,8}$`)
)

// viewerHosts serve share/preview pages, not direct file bytes, so we can't
// download from them without a browser/API. Skip them.
var viewerHosts = []string{
	"drive.google.com", "docs.google.com", "drive.proton.me", "proton.me",
	"dropbox.com", "mega.nz", "mediafire.com", "1drv.ms", "onedrive.live.com",
	"wetransfer.com", "localhost",
}

// ExtractAttachmentURLs pulls downloadable file links out of a challenge
// description. Attachments often live there (as markdown links or bare URLs)
// pointing at object storage like S3 or DigitalOcean Spaces, which are plain
// public GETs needing no API. Share viewers (Google/Proton Drive, etc.) are
// skipped since they aren't direct downloads.
func ExtractAttachmentURLs(description string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(u string, requireExt bool) {
		u = strings.TrimRight(u, ".,;:!?)]}'\"")
		if seen[u] || isViewerHost(u) {
			return
		}
		if requireExt && !hasFileExtension(u) {
			return
		}
		seen[u] = true
		out = append(out, u)
	}
	// Author-labeled links are trusted even without a file extension (e.g. a
	// binary named "chall"); bare URLs must look like a file to avoid grabbing
	// website/discord/rules links.
	for _, m := range mdLinkRe.FindAllStringSubmatch(description, -1) {
		add(m[1], false)
	}
	for _, u := range bareURLRe.FindAllString(description, -1) {
		add(u, true)
	}
	return out
}

func isViewerHost(rawURL string) bool {
	low := strings.ToLower(rawURL)
	for _, h := range viewerHosts {
		if strings.Contains(low, h) {
			return true
		}
	}
	return false
}

func hasFileExtension(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return fileExtRe.MatchString(filepath.Base(u.Path))
}

func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
