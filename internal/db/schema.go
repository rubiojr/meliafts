package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// SupportedSchemaVersion is the melia schema version this build was developed
// against. melia stores an integer in settings.schema_version and bumps it on
// every migration; meliafts reads it to detect drift it cannot safely run
// against. (The schema is not contractually stable, so this is best-effort
// drift detection, not a compatibility guarantee.)
const SupportedSchemaVersion = 13

// SchemaError reports a melia schema version meliafts does not support, or that
// no version could be determined at all.
type SchemaError struct {
	Got   int  // version read from the database (meaningful only when Found)
	Want  int  // the version this build supports
	Found bool // whether any version could be read
}

func (e *SchemaError) Error() string {
	if !e.Found {
		return fmt.Sprintf("could not determine the melia schema version (this build supports %d)", e.Want)
	}
	return fmt.Sprintf("unsupported melia schema version %d (this build supports %d)", e.Got, e.Want)
}

// SchemaVersion reads the melia schema version, preferring the authoritative
// settings.schema_version row and falling back to PRAGMA user_version, which
// newer melia releases stamp in lockstep. The bool reports whether a version
// could be read.
func SchemaVersion(d *sql.DB) (int, bool) {
	var raw string
	if err := d.QueryRow("SELECT value FROM settings WHERE key = 'schema_version'").Scan(&raw); err == nil {
		if v, perr := strconv.Atoi(strings.TrimSpace(raw)); perr == nil {
			return v, true
		}
	}
	var uv int
	if err := d.QueryRow("PRAGMA user_version").Scan(&uv); err == nil && uv > 0 {
		return uv, true
	}
	return 0, false
}

// CheckSchema returns a *SchemaError when the database's schema version differs
// from SupportedSchemaVersion (or cannot be read), and nil when it matches.
func CheckSchema(d *sql.DB) error {
	v, ok := SchemaVersion(d)
	switch {
	case !ok:
		return &SchemaError{Want: SupportedSchemaVersion}
	case v != SupportedSchemaVersion:
		return &SchemaError{Got: v, Want: SupportedSchemaVersion, Found: true}
	default:
		return nil
	}
}
