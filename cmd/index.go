package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Manage Solr documents",
	Long:  `Add, delete, and inspect documents in a Solr collection.`,
}

var indexAddCmd = &cobra.Command{
	Use:   "add <file.json>",
	Short: "Index documents from a JSON file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		coll, _ := cmd.Flags().GetString("collection")
		if coll == "" {
			coll = cfg.Collection
		}

		f, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer f.Close()

		var docs []map[string]interface{}
		if err := json.NewDecoder(f).Decode(&docs); err != nil {
			// try single document
			var single map[string]interface{}
			if _, err2 := f.Seek(0, 0); err2 != nil {
				return fmt.Errorf("failed to parse JSON: %w", err)
			}
			if err2 := json.NewDecoder(f).Decode(&single); err2 != nil {
				return fmt.Errorf("failed to parse JSON (expected array or object): %w", err)
			}
			docs = []map[string]interface{}{single}
		}

		color.Cyan("Indexing %d document(s) into [%s] (%s)…", len(docs), coll, cfg.Env)

		if err := solrClient.IndexDocs(context.Background(), coll, docs); err != nil {
			return fmt.Errorf("index failed: %w", err)
		}

		color.Green("✓  Indexed %d document(s) into [%s]", len(docs), coll)
		return nil
	},
}

var indexDeleteCmd = &cobra.Command{
	Use:   "delete <query>",
	Short: "Delete documents matching a Solr query",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		coll, _ := cmd.Flags().GetString("collection")
		if coll == "" {
			coll = cfg.Collection
		}

		if cfg.IsProd() {
			color.Red("⚠  Deleting from PRODUCTION collection [%s] with query: %s", coll, query)
			fmt.Print("Confirm? (yes/N): ")
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		color.Yellow("Deleting documents matching [%s] from [%s]…", query, coll)

		if err := solrClient.DeleteByQuery(context.Background(), coll, query); err != nil {
			return fmt.Errorf("delete failed: %w", err)
		}

		color.Green("✓  Deleted documents matching [%s] from [%s]", query, coll)
		return nil
	},
}

func init() {
	indexAddCmd.Flags().String("collection", "", "Target collection (overrides --collection)")
	indexDeleteCmd.Flags().String("collection", "", "Target collection (overrides --collection)")

	indexCmd.AddCommand(indexAddCmd)
	indexCmd.AddCommand(indexDeleteCmd)
}
