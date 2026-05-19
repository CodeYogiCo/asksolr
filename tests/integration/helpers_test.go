//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/CodeYogiCo/asksolr/internal/solr"
	"github.com/stretchr/testify/require"
)

const (
	testSolrURL    = "http://localhost:8983/solr"
	testCollection = "yogi_test"
)

// sampleDocs is the seed dataset for integration tests.
var sampleDocs = []map[string]interface{}{
	{"id": "1", "title": "Introduction to Machine Learning", "category": "technology", "price": 49.99, "year": 2022, "in_stock": true},
	{"id": "2", "title": "Advanced Go Programming", "category": "technology", "price": 59.99, "year": 2023, "in_stock": true},
	{"id": "3", "title": "Cooking Italian Food", "category": "food", "price": 29.99, "year": 2021, "in_stock": false},
	{"id": "4", "title": "Climate Change and the Economy", "category": "science", "price": 39.99, "year": 2024, "in_stock": true},
	{"id": "5", "title": "Budget Travel Guide", "category": "travel", "price": 19.99, "year": 2023, "in_stock": true},
	{"id": "6", "title": "Deep Learning with Python", "category": "technology", "price": 54.99, "year": 2024, "in_stock": true},
	{"id": "7", "title": "French Patisserie", "category": "food", "price": 34.99, "year": 2022, "in_stock": true},
	{"id": "8", "title": "Kubernetes in Action", "category": "technology", "price": 64.99, "year": 2023, "in_stock": false},
	{"id": "9", "title": "European Road Trip", "category": "travel", "price": 24.99, "year": 2024, "in_stock": true},
	{"id": "10", "title": "Renewable Energy Explained", "category": "science", "price": 44.99, "year": 2023, "in_stock": true},
}

// setupTestCollection creates the test collection and seeds data.
func setupTestCollection(t *testing.T, c *solr.Client) {
	t.Helper()
	ctx := context.Background()

	// Create collection (ignore error if already exists)
	_ = c.CreateCollection(ctx, testCollection, 1, 1)

	// Seed documents
	require.NoError(t, c.IndexDocs(ctx, testCollection, sampleDocs))

	// Allow Solr to commit
	time.Sleep(500 * time.Millisecond)
}

// teardownTestCollection deletes the test collection.
func teardownTestCollection(t *testing.T, c *solr.Client) {
	t.Helper()
	_ = c.DeleteCollection(context.Background(), testCollection)
}

// waitForSolr waits until Solr is reachable (up to 30 seconds).
func waitForSolr(t *testing.T, c *solr.Client) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if err := c.Ping(ctx); err == nil {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatal("Solr not reachable after 30 seconds")
}

// requireAPIKey skips the test if no Anthropic API key is set.
func requireAPIKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping agentic test")
	}
	return key
}

// newTestSolrClient creates a Solr client for integration tests.
func newTestSolrClient(t *testing.T) *solr.Client {
	t.Helper()
	url := os.Getenv("SOLR_URL")
	if url == "" {
		url = testSolrURL
	}
	return solr.New(url)
}

// mustQueryCount runs a query and returns the NumFound count.
func mustQueryCount(t *testing.T, c *solr.Client, collection, q, fq string) int64 {
	t.Helper()
	result, err := c.Query(context.Background(), collection, solr.QueryParams{
		Q:  q,
		FQ: fq,
	})
	require.NoError(t, err, "query %q failed", q)
	return result.Response.NumFound
}

func TestMain(m *testing.M) {
	fmt.Println("[integration] Starting Solr integration tests")
	fmt.Printf("[integration] Solr URL: %s\n", testSolrURL)
	os.Exit(m.Run())
}
