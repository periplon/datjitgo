package repl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	datjit "github.com/jmcarbo/datjitgo"
	"github.com/jmcarbo/datjitgo/core/model"
)

// commandFn implements a single REPL verb. args is the whitespace-split tail
// after the verb. Implementations write user-visible output to s.out and
// errors to s.errw; they return a non-nil error only to signal "exit the
// loop" (see errExit).
type commandFn func(s *Session, args []string) error

// command bundles a commandFn with its human-readable docstring so `help`
// can surface it.
type command struct {
	Fn   commandFn
	Help string
}

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}

func write(w io.Writer, args ...any) {
	_, _ = fmt.Fprint(w, args...)
}

// errExit is returned by exit/quit to break Session.Run out of its read loop
// cleanly. It is never surfaced to the caller.
var errExit = fmt.Errorf("repl: exit")

// Commands is the exported registry of REPL verbs. Tests inspect this map
// directly (see TestReplHelp / TestReplCompletion). It is populated in init
// so cmdHelp can reference the map without creating a package-init cycle.
var Commands map[string]command

func init() {
	Commands = map[string]command{
		"load":     {Fn: cmdLoad, Help: "load <path> - parse a schema file and make it the active document"},
		"reload":   {Fn: cmdReload, Help: "reload - re-parse the last loaded schema"},
		"show":     {Fn: cmdShow, Help: "show schema|entities|enums|rules|volume - display a section of the loaded schema"},
		"set":      {Fn: cmdSet, Help: "set <option> <value> - configure session state (seed/locale/format/volume/pretty/output/sql-dialect/entity)"},
		"generate": {Fn: cmdGenerate, Help: "generate - produce a dataset using the current session state"},
		"validate": {Fn: cmdValidate, Help: "validate - run static checks against the loaded schema"},
		"inspect":  {Fn: cmdInspect, Help: "inspect [--infer-tools] - summarise the loaded schema (entities, deps, volumes)"},
		"corpus":   {Fn: cmdCorpus, Help: "corpus list|info - inspect the embedded corpus"},
		"formats":  {Fn: cmdFormats, Help: "formats - list available output formats"},
		"help":     {Fn: cmdHelp, Help: "help [<command>] - show command list or one command's help"},
		"history":  {Fn: cmdHistory, Help: "history - print the in-memory command history"},
		"clear":    {Fn: cmdClear, Help: "clear - clear the terminal screen"},
		"exit":     {Fn: cmdExit, Help: "exit - leave the REPL"},
		"quit":     {Fn: cmdExit, Help: "quit - alias for exit"},
	}
}

// commandNames returns the registry keys in sorted order for deterministic
// help output and tab completion.
func commandNames() []string {
	out := make([]string, 0, len(Commands))
	for k := range Commands {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ensureDoc guards commands that require a parsed document.
func ensureDoc(s *Session) bool {
	if s.state.Doc == nil {
		writeln(s.errw, "error: no schema loaded (use `load <path>`)")
		return false
	}
	return true
}

// cmdLoad reads args[0] from disk, parses it, and stores it on the session.
func cmdLoad(s *Session, args []string) error {
	if len(args) != 1 {
		writeln(s.errw, "usage: load <path>")
		return nil
	}
	return loadPath(s, args[0])
}

// loadPath is the shared implementation used by `load` and `reload`.
func loadPath(s *Session, path string) error {
	f, err := os.Open(path)
	if err != nil {
		writef(s.errw, "error: open %s: %v\n", path, err)
		return nil
	}
	defer func() { _ = f.Close() }()
	doc, err := s.svc.Parse(f, path)
	if err != nil {
		writef(s.errw, "error: parse %s: %v\n", path, err)
		return nil
	}
	s.state.Doc = doc
	s.state.Path = path
	writef(s.out, "loaded %s (domain=%s, entities=%d)\n", path, doc.Domain, doc.Entities.Len())
	s.updatePrompt()
	return nil
}

// cmdReload re-parses the previously loaded file from disk.
func cmdReload(s *Session, args []string) error {
	if s.state.Path == "" {
		writeln(s.errw, "error: nothing to reload (use `load <path>` first)")
		return nil
	}
	return loadPath(s, s.state.Path)
}

// cmdShow dispatches to a sub-section printer.
func cmdShow(s *Session, args []string) error {
	if len(args) != 1 {
		writeln(s.errw, "usage: show schema|entities|enums|rules|volume")
		return nil
	}
	if !ensureDoc(s) {
		return nil
	}
	doc := s.state.Doc
	switch args[0] {
	case "schema":
		writef(s.out, "domain: %s\nversion: %s\nentities: %d\nenums: %d\nrules: %d\n",
			doc.Domain, doc.Version, doc.Entities.Len(), doc.Enums.Len(), len(doc.Rules))
	case "entities":
		doc.Entities.Each(func(name string, ent *model.Entity) bool {
			writef(s.out, "%s (%d fields)\n", name, ent.Fields.Len())
			return true
		})
	case "enums":
		if doc.Enums.Len() == 0 {
			writeln(s.out, "(no enums)")
			return nil
		}
		doc.Enums.Each(func(name string, def model.EnumDef) bool {
			variants := make([]string, 0, len(def.Variants))
			for _, v := range def.Variants {
				variants = append(variants, v.Value)
			}
			writef(s.out, "%s: %s\n", name, strings.Join(variants, ", "))
			return true
		})
	case "rules":
		if len(doc.Rules) == 0 {
			writeln(s.out, "(no rules)")
			return nil
		}
		for i, r := range doc.Rules {
			writef(s.out, "#%d: %s\n", i+1, r.Expr)
		}
	case "volume":
		if len(doc.Volume) == 0 && len(s.state.Volumes) == 0 {
			writeln(s.out, "(no volume overrides)")
			return nil
		}
		// Document-declared volumes first, alphabetically.
		names := make([]string, 0, len(doc.Volume))
		for k := range doc.Volume {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			v := doc.Volume[k]
			writef(s.out, "%s: %s\n", k, formatVolume(v))
		}
		if len(s.state.Volumes) > 0 {
			writeln(s.out, "-- session overrides --")
			keys := make([]string, 0, len(s.state.Volumes))
			for k := range s.state.Volumes {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				writef(s.out, "%s: %d\n", k, s.state.Volumes[k])
			}
		}
	default:
		writef(s.errw, "error: unknown `show` target %q\n", args[0])
	}
	return nil
}

// formatVolume renders a VolumeSpec for `show volume` output.
func formatVolume(v model.VolumeSpec) string {
	if v.IsRange() {
		return fmt.Sprintf("%d..%d", v.Min, v.Max)
	}
	if v.Inferred {
		return "(inferred)"
	}
	return strconv.Itoa(v.Exact)
}

// cmdSet handles all `set <key> ...` configuration subcommands.
func cmdSet(s *Session, args []string) error {
	if len(args) < 2 {
		writeln(s.errw, "usage: set <option> <value>")
		return nil
	}
	key, rest := args[0], args[1:]
	switch key {
	case "seed":
		n, err := strconv.ParseInt(rest[0], 10, 64)
		if err != nil {
			writef(s.errw, "error: invalid seed %q: %v\n", rest[0], err)
			return nil
		}
		s.state.Seed = &n
		writef(s.out, "seed=%d\n", n)
	case "locale":
		s.state.Locale = rest[0]
		writef(s.out, "locale=%s\n", rest[0])
	case "format":
		formats := s.svc.Formats()
		if !contains(formats, rest[0]) {
			writef(s.errw, "error: unknown format %q (available: %s)\n", rest[0], strings.Join(formats, ", "))
			return nil
		}
		s.state.Format = rest[0]
		writef(s.out, "format=%s\n", rest[0])
	case "volume":
		// Accept one or more Entity=N pairs.
		for _, pair := range rest {
			eq := strings.IndexByte(pair, '=')
			if eq <= 0 {
				writef(s.errw, "error: bad volume override %q (want Entity=N)\n", pair)
				return nil
			}
			name := pair[:eq]
			n, err := strconv.Atoi(pair[eq+1:])
			if err != nil {
				writef(s.errw, "error: bad volume override %q: %v\n", pair, err)
				return nil
			}
			s.state.Volumes[name] = n
		}
		writef(s.out, "volume=%v\n", s.state.Volumes)
	case "pretty":
		switch strings.ToLower(rest[0]) {
		case "on", "true", "yes", "1":
			s.state.Pretty = true
		case "off", "false", "no", "0":
			s.state.Pretty = false
		default:
			writef(s.errw, "error: pretty must be on|off, got %q\n", rest[0])
			return nil
		}
		writef(s.out, "pretty=%t\n", s.state.Pretty)
	case "output":
		s.state.Output = rest[0]
		writef(s.out, "output=%s\n", rest[0])
	case "sql-dialect":
		s.state.SQLDialect = rest[0]
		writef(s.out, "sql-dialect=%s\n", rest[0])
	case "entity":
		if rest[0] == "none" {
			s.state.EntityFilter = ""
			writeln(s.out, "entity=(none)")
		} else {
			s.state.EntityFilter = rest[0]
			writef(s.out, "entity=%s\n", rest[0])
		}
	default:
		writef(s.errw, "error: unknown set option %q\n", key)
	}
	return nil
}

// cmdGenerate runs the generator + writer pipeline honouring session state.
func cmdGenerate(s *Session, args []string) error {
	if !ensureDoc(s) {
		return nil
	}
	// Apply session-level overrides via a fresh Service so the original
	// facade the caller handed us stays untouched.
	svc := s.svc
	var opts []datjit.Option
	if s.state.Seed != nil {
		opts = append(opts, datjit.WithSeed(*s.state.Seed))
	}
	if s.state.Locale != "" {
		opts = append(opts, datjit.WithLocale(s.state.Locale))
	}
	if len(s.state.Volumes) > 0 {
		opts = append(opts, datjit.WithVolume(s.state.Volumes))
	}
	if len(opts) > 0 {
		override, err := datjit.New(opts...)
		if err != nil {
			writef(s.errw, "error: configure service: %v\n", err)
			return nil
		}
		svc = override
	}
	ds, err := svc.Generate(s.state.Doc)
	if err != nil {
		writef(s.errw, "error: generate: %v\n", err)
		return nil
	}
	writer, closer, err := s.openOutput()
	if err != nil {
		writef(s.errw, "error: open output: %v\n", err)
		return nil
	}
	if closer != nil {
		defer func() { _ = closer.Close() }()
	}
	wopts := datjit.WriteOpts{
		Pretty:       s.state.Pretty,
		SQLDialect:   s.state.SQLDialect,
		EntityFilter: s.state.EntityFilter,
	}
	if err := svc.Write(ds, s.state.Doc, s.state.Format, writer, wopts); err != nil {
		writef(s.errw, "error: write: %v\n", err)
	}
	return nil
}

// openOutput resolves the session's configured destination to an io.Writer.
// The returned Closer is non-nil only when the caller must close a file.
func (s *Session) openOutput() (io.Writer, io.Closer, error) {
	if s.state.Output == "" || s.state.Output == "stdout" {
		return s.out, nil, nil
	}
	// Ensure parent dir exists for convenience.
	if dir := filepath.Dir(s.state.Output); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, nil, err
		}
	}
	f, err := os.Create(s.state.Output)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

// cmdValidate runs datjit.Service.Validate against the loaded document.
func cmdValidate(s *Session, args []string) error {
	if !ensureDoc(s) {
		return nil
	}
	if err := s.svc.Validate(s.state.Doc); err != nil {
		writef(s.errw, "validation error: %v\n", err)
		return nil
	}
	writeln(s.out, "ok")
	return nil
}

// cmdInspect renders the Inspection summary. --infer-tools mirrors the CLI
// inspect flag and appends inferred tool surface per entity.
func cmdInspect(s *Session, args []string) error {
	if !ensureDoc(s) {
		return nil
	}
	inferTools := false
	for _, a := range args {
		switch a {
		case "--infer-tools":
			inferTools = true
		default:
			writef(s.errw, "warning: ignoring unknown inspect flag %q\n", a)
		}
	}
	insp, err := s.svc.Inspect(s.state.Doc)
	if err != nil {
		writef(s.errw, "error: inspect: %v\n", err)
		return nil
	}
	// Render a human-friendly header then a JSON dump — the header is what
	// TestReplInspect greps for.
	writef(s.out, "domain: %s\n", insp.Domain)
	writef(s.out, "version: %s\n", insp.Version)
	writef(s.out, "entities: %d\n", insp.EntityCount)
	for _, e := range insp.Entities {
		writef(s.out, "- %s (fields=%d, deps=[%s], volume=%s)\n",
			e.Name, e.FieldCount, strings.Join(e.Dependencies, ","), formatVolume(e.VolumePlan))
	}
	if inferTools {
		writeln(s.out, "tools:")
		s.state.Doc.Entities.Each(func(name string, ent *model.Entity) bool {
			writef(s.out, "%s: %s\n", name, strings.Join(datjit.InferToolSurface(ent), ", "))
			return true
		})
	}
	return nil
}

// cmdCorpus offers corpus introspection backed by the Service corpus provider.
func cmdCorpus(s *Session, args []string) error {
	if len(args) == 0 {
		writeln(s.errw, "usage: corpus list|info")
		return nil
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			writeln(s.errw, "usage: corpus list")
			return nil
		}
		for _, k := range s.svc.CorpusKeys() {
			writeln(s.out, k)
		}
	case "info":
		if len(args) != 1 {
			writeln(s.errw, "usage: corpus info")
			return nil
		}
		if err := printCorpusInfo(s.out, s.svc); err != nil {
			writef(s.errw, "error: corpus info: %v\n", err)
		}
	default:
		writef(s.errw, "error: unknown corpus subcommand %q\n", args[0])
	}
	return nil
}

func printCorpusInfo(w io.Writer, svc *datjit.Service) error {
	if svc == nil || svc.Corpus() == nil {
		return fmt.Errorf("corpus: nil provider")
	}
	keys := svc.CorpusKeys()
	total := 0
	for _, k := range keys {
		entries, err := svc.Corpus().List("en-US", k)
		if err != nil {
			return fmt.Errorf("corpus list %s: %w", k, err)
		}
		total += len(entries)
	}
	writef(w, "keys: %d\n", len(keys))
	writef(w, "entries: %d\n", total)
	return nil
}

// cmdFormats lists writer format IDs provided by the Service.
func cmdFormats(s *Session, args []string) error {
	for _, f := range s.svc.Formats() {
		writeln(s.out, f)
	}
	return nil
}

// cmdHelp lists every command or prints one docstring.
func cmdHelp(s *Session, args []string) error {
	if len(args) == 0 {
		for _, name := range commandNames() {
			writeln(s.out, Commands[name].Help)
		}
		return nil
	}
	c, ok := Commands[args[0]]
	if !ok {
		writef(s.errw, "error: no help for %q\n", args[0])
		return nil
	}
	writeln(s.out, c.Help)
	return nil
}

// cmdHistory dumps the in-memory history slice.
func cmdHistory(s *Session, args []string) error {
	for i, line := range s.history {
		writef(s.out, "%4d  %s\n", i+1, line)
	}
	return nil
}

// cmdClear emits the ANSI clear-screen escape.
func cmdClear(s *Session, args []string) error {
	write(s.out, "\033[2J\033[H")
	return nil
}

// cmdExit signals the run loop to terminate.
func cmdExit(s *Session, args []string) error {
	return errExit
}

// contains reports whether slice ss contains v.
func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
