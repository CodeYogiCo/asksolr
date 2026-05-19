package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/CodeYogiCo/asksolr/internal/solr"
)

const defaultModel = "claude-opus-4-7"

// SearchResult holds the outcome of an agentic Solr search.
type SearchResult struct {
	NLQuery     string                   `json:"nl_query"`
	SolrQuery   string                   `json:"solr_query"`
	NumFound    int64                    `json:"num_found"`
	Docs        []map[string]interface{} `json:"docs"`
	Explanation string                   `json:"explanation"`
	Iterations  int                      `json:"iterations"`
}

// Agent wraps the Claude API and Solr client for agentic search.
type Agent struct {
	client     *anthropic.Client
	solrClient *solr.Client
	maxIter    int
	verbose    bool
}

// New creates a new Agent.
func New(apiKey string, solrClient *solr.Client, maxIter int, verbose bool) *Agent {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &Agent{
		client:     &client,
		solrClient: solrClient,
		maxIter:    maxIter,
		verbose:    verbose,
	}
}

// Search translates a natural-language query into Solr and returns results.
func (a *Agent) Search(ctx context.Context, collection, nlQuery string) (*SearchResult, error) {
	tools := a.buildTools()

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(
			fmt.Sprintf("Search for: %s", nlQuery),
		)),
	}

	result := &SearchResult{NLQuery: nlQuery}

	for i := 0; i < a.maxIter; i++ {
		result.Iterations = i + 1

		resp, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(defaultModel),
			MaxTokens: 4096,
			System: []anthropic.TextBlockParam{
				{Text: systemPrompt(collection)},
			},
			Tools:    tools,
			Messages: messages,
		})
		if err != nil {
			return nil, fmt.Errorf("claude API error: %w", err)
		}

		if a.verbose {
			fmt.Printf("[iter %d] stop_reason=%s\n", i+1, resp.StopReason)
		}

		// Append assistant turn
		messages = append(messages, resp.ToParam())

		// Collect tool results
		var toolResults []anthropic.ContentBlockParamUnion

		for _, block := range resp.Content {
			switch b := block.AsAny().(type) {
			case anthropic.TextBlock:
				if a.verbose {
					fmt.Printf("[claude] %s\n", b.Text)
				}
				result.Explanation = b.Text

			case anthropic.ToolUseBlock:
				if a.verbose {
					fmt.Printf("[tool] %s(%s)\n", b.Name, b.JSON.Input.Raw())
				}
				output, isErr, solrQ := a.dispatchTool(ctx, collection, b)
				if solrQ != "" {
					result.SolrQuery = solrQ
				}
				toolResults = append(toolResults, anthropic.NewToolResultBlock(b.ID, output, isErr))
			}
		}

		if len(toolResults) == 0 {
			break
		}
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	return result, nil
}

// dispatchTool routes a tool call and returns (jsonOutput, isError, solrQuery).
func (a *Agent) dispatchTool(ctx context.Context, collection string, b anthropic.ToolUseBlock) (string, bool, string) {
	var solrQuery string

	switch b.Name {
	case "get_schema":
		schema, err := a.solrClient.GetSchema(ctx, collection)
		if err != nil {
			return fmt.Sprintf(`{"error":%q}`, err.Error()), true, ""
		}
		out, _ := json.Marshal(schema)
		return string(out), false, ""

	case "execute_query":
		var input struct {
			Q    string `json:"q"`
			FQ   string `json:"fq"`
			Sort string `json:"sort"`
			Rows int    `json:"rows"`
			FL   string `json:"fl"`
		}
		if err := json.Unmarshal([]byte(b.JSON.Input.Raw()), &input); err != nil {
			return fmt.Sprintf(`{"error":%q}`, err.Error()), true, ""
		}
		if input.Rows == 0 {
			input.Rows = 10
		}
		solrQuery = input.Q

		qr, err := a.solrClient.Query(ctx, collection, solr.QueryParams{
			Q:    input.Q,
			FQ:   input.FQ,
			Sort: input.Sort,
			Rows: input.Rows,
			FL:   input.FL,
		})
		if err != nil {
			return fmt.Sprintf(`{"error":%q}`, err.Error()), true, solrQuery
		}

		out, _ := json.Marshal(map[string]interface{}{
			"num_found": qr.Response.NumFound,
			"docs":      qr.Response.Docs,
		})
		return string(out), false, solrQuery

	default:
		return fmt.Sprintf(`{"error":"unknown tool %q"}`, b.Name), true, ""
	}
}

func (a *Agent) buildTools() []anthropic.ToolUnionParam {
	toolParams := []anthropic.ToolParam{
		{
			Name:        "get_schema",
			Description: anthropic.String("Retrieve the Solr schema for the collection, including all field names and types. Call this first to understand the data structure."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]interface{}{},
			},
		},
		{
			Name:        "execute_query",
			Description: anthropic.String("Execute a Solr query against the collection. Returns num_found and matching documents."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]interface{}{
					"q": map[string]interface{}{
						"type":        "string",
						"description": "Main Solr query string using Lucene syntax. Use *:* to match all documents.",
					},
					"fq": map[string]interface{}{
						"type":        "string",
						"description": "Filter query — applied after q, not scored, and cached. Use for categorical/range filters.",
					},
					"sort": map[string]interface{}{
						"type":        "string",
						"description": "Sort expression, e.g. 'score desc' or 'price asc'.",
					},
					"rows": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results to return (default 10, max 100).",
					},
					"fl": map[string]interface{}{
						"type":        "string",
						"description": "Comma-separated list of fields to return. Omit to return all stored fields.",
					},
				},
				// q is required
			},
		},
	}

	tools := make([]anthropic.ToolUnionParam, len(toolParams))
	for i := range toolParams {
		tools[i] = anthropic.ToolUnionParam{OfTool: &toolParams[i]}
	}
	return tools
}
