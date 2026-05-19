package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/CodeYogiCo/asksolr/internal/agent"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search Solr in plain English (agentic)",
	Long: `Translate a natural-language query into Solr using Claude AI.

Claude will:
  1. Inspect the collection schema
  2. Build an appropriate Solr query
  3. Execute and refine until results are satisfactory

Examples:
  yogi-solr search "products under $100 with 4+ star rating" --dev
  yogi-solr search "news articles about climate published in 2024" -c articles
  yogi-solr search "users who signed up last week and have not verified email" --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nlQuery := args[0]
		coll, _ := cmd.Flags().GetString("collection")
		if coll == "" {
			coll = cfg.Collection
		}
		asJSON, _ := cmd.Flags().GetBool("json")

		if cfg.APIKey == "" {
			return fmt.Errorf("Anthropic API key required: use --api-key or set ANTHROPIC_API_KEY")
		}

		color.Cyan("\n🔍 Query: %s", nlQuery)
		color.Cyan("   Collection: %s | env: %s\n", coll, cfg.Env)

		a := agent.New(cfg.APIKey, solrClient, cfg.MaxIterations, cfg.Verbose)

		result, err := a.Search(context.Background(), coll, nlQuery)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}

		if asJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}

		printSearchResult(result)
		return nil
	},
}

func printSearchResult(r *agent.SearchResult) {
	fmt.Println()
	color.Green("✓  Found %d document(s)  [Solr query: %s]", r.NumFound, r.SolrQuery)
	if r.Explanation != "" {
		color.White("   %s", r.Explanation)
	}
	fmt.Printf("   iterations: %d\n\n", r.Iterations)

	if len(r.Docs) == 0 {
		color.Yellow("   No documents returned.")
		return
	}

	for i, doc := range r.Docs {
		fmt.Printf("%s\n", color.CyanString("  [%d]", i+1))
		for k, v := range doc {
			valStr := fmt.Sprintf("%v", v)
			if len(valStr) > 120 {
				valStr = valStr[:120] + "…"
			}
			fmt.Printf("    %-20s %s\n", color.WhiteString(k+":"), valStr)
		}
		fmt.Println()
	}
	fmt.Println(strings.Repeat("─", 60))
}

func init() {
	searchCmd.Flags().String("collection", "", "Override collection (default from --collection flag)")
	searchCmd.Flags().Bool("json", false, "Output raw JSON")
}
