package main

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	"github.com/jmcarbo/datjitgo"
	"github.com/jmcarbo/datjitgo/core/ports"
)

// embeddedCorpusKeys is the canonical list of keys the embedded corpus
// ships. It is hand-maintained so the CLI does not need to reach into the
// corpus package's private embed.FS. The list must stay in sync with
// corpus/data/*.json; drift is caught by TestCorpusList.
var embeddedCorpusKeys = []string{
	"address.cities",
	"address.countries",
	"address.states",
	"address.streets",
	"address.zip_prefixes",
	"color.names",
	"company.catch_phrases",
	"company.industries",
	"company.names",
	"email_domains",
	"file.extensions",
	"job.departments",
	"job.titles",
	"mime.types",
	"person.bios",
	"person.first_names",
	"person.last_names",
	"person.prefixes",
	"person.suffixes",
	"person.usernames",
	"phone_area_codes",
	"product.descriptions",
	"product.titles",
	"text.paragraphs",
	"text.sentences",
	"text.words",
	"timezones",
}

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
			keys := resolveCorpusKeys(datjit.NewDefault().Corpus())
			for _, k := range keys {
				fmt.Fprintln(cmd.OutOrStdout(), k)
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
			return printCorpusInfo(cmd.OutOrStdout(), datjit.NewDefault().Corpus())
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
			fmt.Fprintln(cmd.OutOrStdout(), "corpus update is deferred to phase 2")
			return nil
		},
	}
}

// resolveCorpusKeys returns the sorted set of keys the given provider
// actually resolves. We start from the canonical list and filter with Has
// so a forward-compat overlay provider could extend or constrain the
// surface without breaking the CLI.
func resolveCorpusKeys(p ports.CorpusProvider) []string {
	if p == nil {
		return nil
	}
	out := make([]string, 0, len(embeddedCorpusKeys))
	for _, k := range embeddedCorpusKeys {
		if p.Has(k) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// printCorpusInfo tallies entries per key and prints a compact summary.
func printCorpusInfo(w io.Writer, p ports.CorpusProvider) error {
	if p == nil {
		return fmt.Errorf("corpus: nil provider")
	}
	keys := resolveCorpusKeys(p)
	total := 0
	for _, k := range keys {
		entries, err := p.List("en-US", k)
		if err != nil {
			return fmt.Errorf("corpus list %s: %w", k, err)
		}
		total += len(entries)
	}
	fmt.Fprintf(w, "keys: %d\n", len(keys))
	fmt.Fprintf(w, "entries: %d\n", total)
	return nil
}
