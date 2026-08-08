// Package browser reads CTFd session cookies from a local browser profile.
package browser

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// SessionCookie returns the "session" cookie for rawURL's host from a
// Firefox-family profile (Firefox, Floorp, LibreWolf).
func SessionCookie(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return "", fmt.Errorf("invalid url %q", rawURL)
	}
	host := u.Hostname()

	dbs := cookieDBs()
	if len(dbs) == 0 {
		return "", fmt.Errorf("no Firefox/Floorp/LibreWolf cookie database found")
	}
	for _, db := range dbs {
		if v, err := querySession(db, host); err == nil && v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("no session cookie for %s in your browser (are you logged in?)", host)
}

func cookieDBs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var out []string
	for _, root := range []string{".floorp", ".mozilla/firefox", ".librewolf"} {
		matches, _ := filepath.Glob(filepath.Join(home, root, "*", "cookies.sqlite"))
		out = append(out, matches...)
	}
	return out
}

func querySession(dbPath, host string) (string, error) {
	tmp, err := copyDB(dbPath)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(filepath.Dir(tmp))

	db, err := sql.Open("sqlite", "file:"+tmp+"?mode=ro")
	if err != nil {
		return "", err
	}
	defer db.Close()

	var value string
	err = db.QueryRow(
		`SELECT value FROM moz_cookies
		 WHERE name = 'session' AND (host = ? OR host = ?)
		 ORDER BY LENGTH(host) DESC LIMIT 1`,
		host, "."+host,
	).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

// copyDB copies cookies.sqlite (and any -wal/-shm) to a temp dir so a locked,
// in-use browser database can still be read.
func copyDB(dbPath string) (string, error) {
	dir, err := os.MkdirTemp("", "ctfdl-cookies-*")
	if err != nil {
		return "", err
	}
	dst := filepath.Join(dir, filepath.Base(dbPath))
	for _, suffix := range []string{"", "-wal", "-shm"} {
		data, err := os.ReadFile(dbPath + suffix)
		if err != nil {
			if suffix == "" {
				os.RemoveAll(dir)
				return "", err
			}
			continue
		}
		if err := os.WriteFile(dst+suffix, data, 0600); err != nil {
			os.RemoveAll(dir)
			return "", err
		}
	}
	return dst, nil
}
