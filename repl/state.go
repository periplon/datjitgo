package repl

import (
	"github.com/periplon/datjitgo/core/model"
)

// SessionState holds the mutable interactive session configuration. It is
// separated from Session so commands can be unit-tested without spinning up
// I/O plumbing.
type SessionState struct {
	// Doc is the most recently parsed schema (nil until `load`).
	Doc *model.Document
	// Path is the on-disk path the current Doc was loaded from, used by the
	// `reload` command.
	Path string
	// Format is the serialiser to use for `generate` output.
	Format string
	// Pretty toggles pretty-printing for JSON/SQL.
	Pretty bool
	// Seed, when non-nil, overrides the document/service seed for each
	// generate call.
	Seed *int64
	// Locale overrides the document/service locale when non-empty.
	Locale string
	// Volumes is a per-entity row-count override map. Empty means "let the
	// document/service decide".
	Volumes map[string]int
	// SQLDialect is forwarded to the SQL writer. Empty → writer default.
	SQLDialect string
	// Output is "stdout" or a filesystem path.
	Output string
	// EntityFilter, when non-empty, restricts generate output to the named
	// entity (mirrors the WriteOpts field).
	EntityFilter string
}

// NewState returns a SessionState seeded with the REPL's documented defaults.
func NewState() *SessionState {
	return &SessionState{
		Format:  "json",
		Pretty:  false,
		Output:  "stdout",
		Volumes: map[string]int{},
	}
}
