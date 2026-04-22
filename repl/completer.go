package repl

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jmcarbo/datjitgo/core/model"
)

// completer implements readline.AutoCompleter. It is also directly testable
// via its Do method.
//
// Completion rules:
//   - If the current token is the first word on the line, offer command names.
//   - After "load ", offer path completions relative to CWD.
//   - After "set format ", offer the list of known formats.
//   - After "set volume ", offer "Entity=" prefixes from the loaded schema.
//   - After "show ", offer the fixed subcommand set.
type completer struct {
	s *Session
}

// newCompleter returns a completer bound to s.
func newCompleter(s *Session) *completer {
	return &completer{s: s}
}

// Do matches the readline.AutoCompleter contract. It returns the list of
// candidate *completions* (suffixes that would extend the current word) and
// the number of runes that were consumed from the current word.
func (c *completer) Do(line []rune, pos int) ([][]rune, int) {
	// Work on the slice up to the cursor — typing in the middle of a line
	// is unusual in a REPL and readline's own widget handles re-rendering.
	prefix := string(line[:pos])
	// Split into words, preserving whether the trailing char is a space so
	// we can tell "load" from "load ".
	trailingSpace := strings.HasSuffix(prefix, " ")
	fields := strings.Fields(prefix)

	// Empty line or first token → command name completion.
	if len(fields) == 0 || (len(fields) == 1 && !trailingSpace) {
		cur := ""
		if len(fields) == 1 {
			cur = fields[0]
		}
		return matchCommand(cur), len([]rune(cur))
	}

	cmd := fields[0]
	// Determine the current (partial) token and the argument index it belongs
	// to. If the line ends with a space the user just committed a word, so
	// the *next* argument is empty and its index is len(fields)-1 (relative
	// to the args slice, which excludes the verb).
	var cur string
	if trailingSpace {
		cur = ""
	} else {
		cur = fields[len(fields)-1]
	}

	switch cmd {
	case "load":
		return matchPath(cur), len([]rune(cur))
	case "show":
		return matchStatic(cur, []string{"schema", "entities", "enums", "rules", "volume"}), len([]rune(cur))
	case "corpus":
		return matchStatic(cur, []string{"list", "info"}), len([]rune(cur))
	case "help":
		return matchCommand(cur), len([]rune(cur))
	case "set":
		return c.completeSet(fields, trailingSpace, cur), len([]rune(cur))
	}
	return nil, 0
}

// completeSet handles the nested grammar `set <option> <value>`.
func (c *completer) completeSet(fields []string, trailingSpace bool, cur string) [][]rune {
	// fields[0]="set", fields[1]=option (maybe partial), fields[2..]=values.
	// Figure out which slot we're completing.
	//
	// If trailingSpace: cur=="" and the next slot index is len(fields)-1
	// relative to {option,value1,value2,...}. Otherwise we're still editing
	// the last field.
	options := []string{"seed", "locale", "format", "volume", "pretty", "output", "sql-dialect", "entity"}

	// Case: still typing the option.
	if (len(fields) == 1 && trailingSpace) || (len(fields) == 2 && !trailingSpace) {
		return matchStatic(cur, options)
	}
	if len(fields) < 2 {
		return nil
	}
	opt := fields[1]
	switch opt {
	case "format":
		return matchStatic(cur, c.s.svc.Formats())
	case "pretty":
		return matchStatic(cur, []string{"on", "off"})
	case "sql-dialect":
		return matchStatic(cur, []string{"postgres", "mysql", "sqlite"})
	case "volume":
		return matchStatic(cur, c.entityVolumeCandidates())
	case "entity":
		ents := append([]string{"none"}, c.entityNames()...)
		return matchStatic(cur, ents)
	}
	return nil
}

// entityNames returns the loaded document's entity names in declaration
// order, or nil if nothing is loaded.
func (c *completer) entityNames() []string {
	if c.s.state.Doc == nil {
		return nil
	}
	var out []string
	c.s.state.Doc.Entities.Each(func(name string, _ *model.Entity) bool {
		out = append(out, name)
		return true
	})
	return out
}

// entityVolumeCandidates returns "Entity=" stubs so that after TAB the user
// only has to type the number.
func (c *completer) entityVolumeCandidates() []string {
	names := c.entityNames()
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, n+"=")
	}
	return out
}

// matchCommand returns command-name completions for the given prefix, as
// suffix slices. Readline expects just the *suffix* (i.e. what would be
// appended), so we strip the prefix before returning.
func matchCommand(prefix string) [][]rune {
	names := commandNames()
	return matchStatic(prefix, names)
}

// matchStatic filters candidates by prefix and returns the suffixes.
func matchStatic(prefix string, candidates []string) [][]rune {
	var out [][]rune
	for _, c := range candidates {
		if strings.HasPrefix(c, prefix) {
			out = append(out, []rune(strings.TrimPrefix(c, prefix)))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return string(out[i]) < string(out[j])
	})
	return out
}

// matchPath offers path completions relative to CWD. Returns the suffix that
// extends the typed prefix so readline can splice it in.
func matchPath(prefix string) [][]rune {
	// Resolve the directory to scan and the filename stub.
	dir, base := filepath.Split(prefix)
	if dir == "" {
		dir = "."
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out [][]rune
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, base) {
			continue
		}
		suffix := strings.TrimPrefix(name, base)
		if e.IsDir() {
			suffix += string(os.PathSeparator)
		}
		out = append(out, []rune(suffix))
	}
	sort.Slice(out, func(i, j int) bool {
		return string(out[i]) < string(out[j])
	})
	return out
}
