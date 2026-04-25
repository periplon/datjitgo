package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/jmcarbo/datjitgo"
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
	return &cobra.Command{
		Use:   "list",
		Short: "List every key shipped in the embedded corpus",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			keys := datjit.NewDefault().CorpusKeys()
			for _, k := range keys {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), k); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// cmdCorpusInfo prints summary counts (keys, total entries) for the
// embedded corpus.
func cmdCorpusInfo() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show corpus summary (key count, entry count)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return printCorpusInfo(cmd.OutOrStdout(), datjit.NewDefault())
		},
	}
}

// cmdCorpusUpdate is a placeholder. Live updates ship in phase 2; today we
// just let the user know this politely and exit 0.
func cmdCorpusUpdate() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Refresh the on-disk corpus overlay (phase 2)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "corpus update is deferred to phase 2")
			return err
		},
	}
}

// printCorpusInfo tallies entries per key and prints a compact summary.
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
	if _, err := fmt.Fprintf(w, "keys: %d\n", len(keys)); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "entries: %d\n", total)
	return err
}
