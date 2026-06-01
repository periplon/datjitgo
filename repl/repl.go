// Package repl implements the interactive datjit shell. It is deliberately
// decoupled from cobra so the Session can be embedded by tests or
// alternative front-ends.
package repl

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"

	datjit "github.com/periplon/datjitgo"
)

// Session is one interactive shell instance. Construct with New, then drive
// it with Run. A Session is *not* safe for concurrent use.
type Session struct {
	svc     *datjit.Service
	state   *SessionState
	history []string

	// Writers set per-Run so embedded tests can capture output.
	out  io.Writer
	errw io.Writer

	// rl is the active readline instance when running on a TTY. Nil when
	// falling back to bufio.Scanner.
	rl *readline.Instance
}

// New returns a Session ready to Run. svc is retained by reference; callers
// that want isolated configuration should pass a freshly constructed
// Service.
func New(svc *datjit.Service) *Session {
	return &Session{
		svc:   svc,
		state: NewState(),
	}
}

// State exposes the internal SessionState so tests can inspect / mutate
// configuration without going through the command grammar.
func (s *Session) State() *SessionState { return s.state }

// Service returns the datjit façade this session is bound to.
func (s *Session) Service() *datjit.Service { return s.svc }

// Run starts the read-eval-print loop. It returns nil for a clean exit
// (EOF, `exit`/`quit`, or Ctrl-D). Non-nil errors indicate setup problems
// that prevented the loop from starting.
//
// When in is a TTY, readline is used for line editing, history, and tab
// completion. Otherwise Run falls back to a plain bufio.Scanner so scripted
// input (tests, `<<EOF` heredocs) works without TTY emulation.
func (s *Session) Run(ctx context.Context, in io.Reader, out, errw io.Writer) error {
	s.out = out
	s.errw = errw

	if isTTY(in) {
		return s.runInteractive(ctx, in, out, errw)
	}
	return s.runScripted(ctx, in)
}

// runInteractive uses readline for line editing + history.
func (s *Session) runInteractive(ctx context.Context, in io.Reader, out, errw io.Writer) error {
	stdin, ok := in.(io.ReadCloser)
	if !ok {
		stdin = io.NopCloser(in)
	}
	cfg := &readline.Config{
		Prompt:          s.prompt(),
		HistoryFile:     historyPath(),
		AutoComplete:    newCompleter(s),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		Stdin:           stdin,
		Stdout:          out,
		Stderr:          errw,
	}
	rl, err := readline.NewEx(cfg)
	if err != nil {
		// Fall back to scripted mode on readline init failure so we still
		// give the caller *some* shell rather than refusing outright.
		_, _ = fmt.Fprintf(errw, "warning: readline unavailable (%v), using line mode\n", err)
		return s.runScripted(ctx, in)
	}
	defer func() { _ = rl.Close() }()
	s.rl = rl
	defer func() { s.rl = nil }()

	for {
		if ctx.Err() != nil {
			return nil
		}
		rl.SetPrompt(s.prompt())
		line, err := rl.Readline()
		if err != nil {
			// readline.ErrInterrupt (^C) on empty line: keep prompting.
			if errors.Is(err, readline.ErrInterrupt) {
				if strings.TrimSpace(line) == "" {
					continue
				}
				continue
			}
			// EOF (^D) or stream close → clean exit.
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if stop, _ := s.dispatch(line); stop {
			return nil
		}
	}
}

// runScripted feeds bufio.Scanner lines through the same dispatcher.
func (s *Session) runScripted(ctx context.Context, in io.Reader) error {
	scanner := bufio.NewScanner(in)
	// Permit arbitrarily long lines so embedded expressions survive.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil
		}
		line := scanner.Text()
		if stop, _ := s.dispatch(line); stop {
			return nil
		}
	}
	// Scanner errors are surfaced to the caller; io.EOF is absorbed.
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// dispatch parses a line and invokes the matching command. The first return
// value reports whether the loop should terminate; the second carries any
// fatal error (currently always nil — command errors go to s.errw).
func (s *Session) dispatch(line string) (stop bool, err error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return false, nil
	}
	s.history = append(s.history, trimmed)

	fields := strings.Fields(trimmed)
	name := fields[0]
	args := fields[1:]
	cmd, ok := Commands[name]
	if !ok {
		_, _ = fmt.Fprintf(s.errw, "error: unknown command %q (try `help`)\n", name)
		return false, nil
	}
	if cerr := cmd.Fn(s, args); cerr != nil {
		if errors.Is(cerr, errExit) {
			return true, nil
		}
		_, _ = fmt.Fprintf(s.errw, "error: %v\n", cerr)
	}
	return false, nil
}

// prompt returns the current prompt string. It changes when a document is
// loaded to give the user a visual cue of the active domain.
func (s *Session) prompt() string {
	if s.state.Doc != nil && s.state.Doc.Domain != "" {
		return fmt.Sprintf("datjit[%s]> ", s.state.Doc.Domain)
	}
	return "datjit> "
}

// updatePrompt refreshes the readline prompt after a load changes state.
// No-op in scripted mode.
func (s *Session) updatePrompt() {
	if s.rl != nil {
		s.rl.SetPrompt(s.prompt())
	}
}

// isTTY reports whether r is a file descriptor pointing at a terminal. We
// only treat *os.File inputs as candidates; strings.Readers, pipes, and
// tests all fall through to the scripted code path.
func isTTY(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	return readline.IsTerminal(int(f.Fd()))
}

// historyPath returns the on-disk history file, preferring
// $XDG_STATE_HOME/datjit/history with a home-directory fallback. It does
// its best to create the parent directory; failure is non-fatal because
// readline just skips persistence if the file cannot be written.
func historyPath() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		dir := filepath.Join(xdg, "datjit")
		_ = os.MkdirAll(dir, 0o755)
		return filepath.Join(dir, "history")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".datjit_history")
}
