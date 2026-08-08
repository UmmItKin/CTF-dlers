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

var mdLinkRe = regexp.MustCompile(`\[[^\]]*\]\((https?://[^)\s]+)\)`)

// ExtractAttachmentURLs pulls direct file links out of a markdown description
// (some CTFs list attachments there instead of in CTFd's files field). Viewer
// links that aren't direct downloads (Google Drive, localhost) are skipped.
func ExtractAttachmentURLs(description string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range mdLinkRe.FindAllStringSubmatch(description, -1) {
		u := m[1]
		low := strings.ToLower(u)
		if seen[u] || strings.Contains(low, "drive.google.com") || strings.Contains(low, "localhost") {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
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
