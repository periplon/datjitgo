package corpus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// UpdateSource describes one downloadable corpus key.
type UpdateSource struct {
	Key string
	URL string
}

var validUpdateKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(\.[A-Za-z][A-Za-z0-9_]*)*$`)

// DefaultOverlayDir returns the conventional writable corpus overlay path.
func DefaultOverlayDir() string {
	if dir := os.Getenv("DATJIT_CORPUS_DIR"); dir != "" {
		return dir
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "datjit", "corpus")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".datjit", "corpus")
	}
	return filepath.Join(".", ".datjit", "corpus")
}

// DefaultUpdateSources returns source URLs from DATJIT_CORPUS_SOURCE[S]. The
// value format is key=url, comma-separated for DATJIT_CORPUS_SOURCES.
func DefaultUpdateSources() ([]UpdateSource, error) {
	var out []UpdateSource
	for _, raw := range splitSourceEnv(os.Getenv("DATJIT_CORPUS_SOURCES")) {
		src, err := parseSource(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	if raw := os.Getenv("DATJIT_CORPUS_SOURCE"); raw != "" {
		src, err := parseSource(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, nil
}

// Update downloads every source into overlayDir/data as validated corpus JSON.
// Each file is written atomically after the payload is parsed successfully.
func Update(ctx context.Context, overlayDir string, sources []UpdateSource) ([]string, error) {
	if overlayDir == "" {
		overlayDir = DefaultOverlayDir()
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("no corpus update sources configured")
	}
	dataDir := filepath.Join(overlayDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	updated := make([]string, 0, len(sources))
	for _, src := range sources {
		if strings.TrimSpace(src.Key) == "" || strings.TrimSpace(src.URL) == "" {
			return nil, fmt.Errorf("invalid corpus source: key and url are required")
		}
		if !validUpdateKey.MatchString(src.Key) {
			return nil, fmt.Errorf("invalid corpus source key %q", src.Key)
		}
		raw, err := download(ctx, src.URL)
		if err != nil {
			return nil, fmt.Errorf("download %s: %w", src.Key, err)
		}
		entries, err := parseEntries(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", src.Key, err)
		}
		normalized, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", src.Key, err)
		}
		normalized = append(normalized, '\n')
		path := filepath.Join(dataDir, corpusFilename(src.Key))
		if err := atomicWrite(path, normalized); err != nil {
			return nil, fmt.Errorf("write %s: %w", src.Key, err)
		}
		updated = append(updated, src.Key)
	}
	return updated, nil
}

func splitSourceEnv(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseSource(raw string) (UpdateSource, error) {
	key, url, ok := strings.Cut(raw, "=")
	if !ok || strings.TrimSpace(key) == "" || strings.TrimSpace(url) == "" {
		return UpdateSource{}, fmt.Errorf("invalid corpus source %q (expected key=url)", raw)
	}
	return UpdateSource{Key: strings.TrimSpace(key), URL: strings.TrimSpace(url)}, nil
}

func download(ctx context.Context, url string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func atomicWrite(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
