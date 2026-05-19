package cmd

import (
	"fmt"
	"os"

	"github.com/CodeYogiCo/asksolr/internal/config"
	"github.com/CodeYogiCo/asksolr/internal/solr"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var cfg = &config.Config{}
var solrClient *solr.Client

var rootCmd = &cobra.Command{
	Use:   "asksolr",
	Short: "Agentic Solr CLI — query Solr in plain English",
	Long: `asksolr is a Kubernetes-style CLI that uses Claude AI to translate
natural language queries into Solr queries and execute them.

Examples:
  asksolr search "find all products cheaper than $50 in stock" --dev
  asksolr search "documents about AI published after 2023" --prod --collection news
  asksolr collection list
  asksolr index add ./data.json --collection products`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// skip env banner for completion and help
		if cmd.Name() == "completion" || cmd.Name() == "__complete" {
			return nil
		}

		dev, _ := cmd.Flags().GetBool("dev")
		prod, _ := cmd.Flags().GetBool("prod")

		if dev && prod {
			return fmt.Errorf("--dev and --prod are mutually exclusive")
		}
		if prod {
			cfg.Env = config.Prod
			color.Yellow("⚠  PRODUCTION environment")
		} else {
			cfg.Env = config.Dev
			color.Cyan("→  dev environment")
		}

		// inherit API key from env if not set via flag
		if cfg.APIKey == "" {
			cfg.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		}

		solrClient = solr.New(cfg.SolrBase())
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().Bool("dev", false, "Target dev environment (default)")
	rootCmd.PersistentFlags().Bool("prod", false, "Target production environment")
	rootCmd.PersistentFlags().StringVar(&cfg.SolrURL, "solr-url", "http://localhost:8983", "Solr base URL")
	rootCmd.PersistentFlags().StringVarP(&cfg.Collection, "collection", "c", "default", "Default Solr collection")
	rootCmd.PersistentFlags().StringVar(&cfg.APIKey, "api-key", "", "Anthropic API key (or set ANTHROPIC_API_KEY env)")
	rootCmd.PersistentFlags().BoolVarP(&cfg.Verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().IntVar(&cfg.MaxIterations, "max-iterations", 5, "Maximum agentic loop iterations")

	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(indexCmd)
	rootCmd.AddCommand(collectionCmd)
	rootCmd.AddCommand(schemaCmd)
}
