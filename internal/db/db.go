// Package db opens the melia SQLite database for searching.
package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// defaultRelPath is the melia database location relative to the user's home
// directory.
const defaultRelPath = ".var/app/com.buxjr.melia/config/melia/melia.db"

// DefaultPath returns the default melia database path,
// ~/.var/app/com.buxjr.melia/config/melia/melia.db. It returns an empty string
// if the user's home directory cannot be determined.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, defaultRelPath)
}

// OpenReadOnly opens the SQLite database at path in read-only mode.
//
// A leading "~/" in path is expanded to the user's home directory. The database
// is opened with mode=ro and the query_only pragma so that no write can occur
// through the returned handle, and a busy timeout so transient locks held by a
// running melia instance do not fail the query immediately.
func OpenReadOnly(path string) (*sql.DB, error) {
	expanded, err := expandHome(path)
	if err != nil {
		return nil, err
	}

	if fi, err := os.Stat(expanded); err != nil {
		return nil, fmt.Errorf("cannot open database %q: %w", expanded, err)
	} else if fi.IsDir() {
		return nil, fmt.Errorf("cannot open database %q: is a directory", expanded)
	}

	db, err := sql.Open("sqlite", readOnlyDSN(expanded))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("open database %q: %w", expanded, err)
	}
	return db, nil
}

// readOnlyDSN builds a modernc.org/sqlite DSN that opens the file read-only.
func readOnlyDSN(absPath string) string {
	vals := url.Values{}
	vals.Set("mode", "ro")
	vals.Add("_pragma", "busy_timeout(5000)")
	vals.Add("_pragma", "query_only(true)")

	u := url.URL{Scheme: "file", Path: absPath, RawQuery: vals.Encode()}
	return u.String()
}

// expandHome expands a leading "~/" (or a bare "~") in path to the user's home
// directory and makes the result absolute.
func expandHome(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand %q: %w", path, err)
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", path, err)
	}
	return abs, nil
}
