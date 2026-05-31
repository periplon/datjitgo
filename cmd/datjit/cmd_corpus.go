package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/periplon/datjitgo"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/corpus"
)

// cmdCorpus wires the `datjit corpus <sub>` group.
func cmdCorpus() *cobra.Command {
	c := &cobra.Command{
		Use:          "corpus",
		Short:        "Inspect the embedded corpus",
		SilenceUsage: true,
	}
	c.AddCommand(cmdCorpusList(), cmdCorpusInfo(), cmdCorpusUpdate())
	return c
}

// cmdCorpusList prints every corpus key, one per line, sorted.
func cmdCorpusList() *cobra.Command {
	var corpusDir string
	c := &cobra.Command{
		Use:   "list",
		Short: "List every key shipped in the embedded corpus",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			keys := corpusService(corpusDir).CorpusKeys()
			for _, k := range keys {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), k); err != nil {
					return err
				}
			}
			return nil
		},
	}
	c.Flags().StringVar(&corpusDir, "corpus-dir", "", "read corpus overlay files from this directory")
	return c
}

// cmdCorpusInfo prints summary counts (keys, total entries) for the
// embedded corpus.
func cmdCorpusInfo() *cobra.Command {
	var corpusDir string
	c := &cobra.Command{
		Use:   "info",
		Short: "Show corpus summary (key count, entry count)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return printCorpusInfo(cmd.OutOrStdout(), corpusService(corpusDir))
		},
	}
	c.Flags().StringVar(&corpusDir, "corpus-dir", "", "read corpus overlay files from this directory")
	return c
}

// cmdCorpusUpdate downloads configured corpus sources into an on-disk overlay.
func cmdCorpusUpdate() *cobra.Command {
	var (
		corpusDir string
		source    []string
	)
	c := &cobra.Command{
		Use:   "update",
		Short: "Refresh the on-disk corpus overlay",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			sources, err := corpus.DefaultUpdateSources()
			if err != nil {
				return err
			}
			for _, raw := range source {
				src, err := parseCorpusSource(raw)
				if err != nil {
					return &usageErr{err: err}
				}
				sources = append(sources, src)
			}
			updated, err := corpus.Update(context.Background(), corpusDir, sources)
			if err != nil {
				return err
			}
			dir := corpusDir
			if dir == "" {
				dir = corpus.DefaultOverlayDir()
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "updated %d corpus keys in %s: %s\n", len(updated), dir, strings.Join(updated, ", "))
			return err
		},
	}
	c.Flags().StringVar(&corpusDir, "corpus-dir", "", "write corpus overlay files to this directory")
	c.Flags().StringSliceVar(&source, "source", nil, "download source as key=url; repeat or comma-separate")
	return c
}

func corpusService(corpusDir string) *datjit.Service {
	if corpusDir == "" {
		return datjit.NewDefault()
	}
	svc, err := datjit.New(datjit.WithCorpus(corpus.NewWithOverlay(corpusDir)))
	if err != nil {
		return datjit.NewDefault()
	}
	return svc
}

func parseCorpusSource(raw string) (corpus.UpdateSource, error) {
	key, url, ok := strings.Cut(raw, "=")
	if !ok || strings.TrimSpace(key) == "" || strings.TrimSpace(url) == "" {
		return corpus.UpdateSource{}, fmt.Errorf("invalid --source %q (expected key=url)", raw)
	}
	return corpus.UpdateSource{Key: strings.TrimSpace(key), URL: strings.TrimSpace(url)}, nil
}

// resolveCorpusKeys returns the sorted set of keys the given provider resolves.
func resolveCorpusKeys(p ports.CorpusProvider) []string {
	lister, ok := p.(interface{ Keys() []string })
	if !ok {
		return nil
	}
	return lister.Keys()
}

// printCorpusInfo tallies entries per key and prints a compact summary.
func printCorpusInfo(w io.Writer, target any) error {
	var provider ports.CorpusProvider
	var keys []string
	switch v := target.(type) {
	case *datjit.Service:
		if v != nil {
			provider = v.Corpus()
			keys = v.CorpusKeys()
		}
	case ports.CorpusProvider:
		provider = v
		keys = resolveCorpusKeys(v)
	}
	if provider == nil {
		return fmt.Errorf("corpus: nil provider")
	}
	total := 0
	for _, k := range keys {
		entries, err := provider.List("en-US", k)
		if err != nil {
			return fmt.Errorf("corpus list %s: %w", k, err)
		}
		total += len(entries)
	}
	if _, err := fmt.Fprintf(w, "keys: %d\n", len(keys)); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "entries: %d\n", total)
	return err
}
