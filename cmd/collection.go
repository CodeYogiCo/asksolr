package cmd

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var collectionCmd = &cobra.Command{
	Use:   "collection",
	Short: "Manage Solr collections",
}

var collectionListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all collections",
	RunE: func(cmd *cobra.Command, args []string) error {
		color.Cyan("Listing collections [%s]…", cfg.Env)
		colls, err := solrClient.ListCollections(context.Background())
		if err != nil {
			return fmt.Errorf("failed to list collections: %w", err)
		}
		if len(colls) == 0 {
			color.Yellow("  No collections found.")
			return nil
		}
		for _, c := range colls {
			fmt.Printf("  • %s\n", c)
		}
		return nil
	},
}

var collectionCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new collection",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		shards, _ := cmd.Flags().GetInt("shards")
		replicas, _ := cmd.Flags().GetInt("replicas")

		color.Cyan("Creating collection [%s] shards=%d replicas=%d [%s]…", name, shards, replicas, cfg.Env)

		if err := solrClient.CreateCollection(context.Background(), name, shards, replicas); err != nil {
			return fmt.Errorf("failed to create collection: %w", err)
		}
		color.Green("✓  Collection [%s] created.", name)
		return nil
	},
}

var collectionDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a collection",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if cfg.IsProd() {
			color.Red("⚠  Deleting PRODUCTION collection [%s]. Are you sure? (yes/N)", name)
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		color.Yellow("Deleting collection [%s]…", name)
		if err := solrClient.DeleteCollection(context.Background(), name); err != nil {
			return fmt.Errorf("failed to delete collection: %w", err)
		}
		color.Green("✓  Collection [%s] deleted.", name)
		return nil
	},
}

func init() {
	collectionCreateCmd.Flags().IntP("shards", "s", 1, "Number of shards")
	collectionCreateCmd.Flags().IntP("replicas", "r", 1, "Replication factor")

	collectionCmd.AddCommand(collectionListCmd)
	collectionCmd.AddCommand(collectionCreateCmd)
	collectionCmd.AddCommand(collectionDeleteCmd)
}
