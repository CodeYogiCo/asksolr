package cmd

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Inspect Solr collection schema",
}

var schemaShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display schema fields for a collection",
	RunE: func(cmd *cobra.Command, args []string) error {
		coll, _ := cmd.Flags().GetString("collection")
		if coll == "" {
			coll = cfg.Collection
		}

		color.Cyan("Schema for collection [%s] [%s]\n", coll, cfg.Env)

		schema, err := solrClient.GetSchema(context.Background(), coll)
		if err != nil {
			return fmt.Errorf("failed to get schema: %w", err)
		}

		fmt.Printf("  Name:       %s\n", schema.Name)
		fmt.Printf("  UniqueKey:  %s\n", schema.UniqueKey)
		fmt.Printf("  Fields (%d):\n", len(schema.Fields))
		fmt.Printf("  %-25s %-15s %-8s %-8s\n",
			color.WhiteString("Field"), color.WhiteString("Type"),
			color.WhiteString("Indexed"), color.WhiteString("Stored"))
		fmt.Printf("  %s\n", "─────────────────────────────────────────────────────")
		for _, f := range schema.Fields {
			fmt.Printf("  %-25s %-15s %-8v %-8v\n", f.Name, f.Type, f.Indexed, f.Stored)
		}
		return nil
	},
}

func init() {
	schemaShowCmd.Flags().String("collection", "", "Collection to inspect")
	schemaCmd.AddCommand(schemaShowCmd)
}
