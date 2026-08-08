package browser

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestQuerySession(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cookies.sqlite")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE moz_cookies (host TEXT, name TEXT, value TEXT)`); err != nil {
		t.Fatal(err)
	}
	db.Exec(`INSERT INTO moz_cookies VALUES ('play.example.com','session','abc123')`)
	db.Exec(`INSERT INTO moz_cookies VALUES ('play.example.com','other','nope')`)
	db.Close()

	got, err := querySession(dbPath, "play.example.com")
	if err != nil || got != "abc123" {
		t.Fatalf("got %q, err %v; want abc123", got, err)
	}

	if _, err := querySession(dbPath, "unknown.example.com"); err == nil {
		t.Error("expected error for host with no session cookie")
	}
}
