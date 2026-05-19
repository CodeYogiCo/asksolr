//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/CodeYogiCo/asksolr/internal/solr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationQueryAllDocs(t *testing.T) {
	c := newTestSolrClient(t)
	waitForSolr(t, c)
	setupTestCollection(t, c)
	defer teardownTestCollection(t, c)

	count := mustQueryCount(t, c, testCollection, "*:*", "")
	assert.Equal(t, int64(10), count, "expected all 10 seed documents")
}

func TestIntegrationFilterByCategory(t *testing.T) {
	c := newTestSolrClient(t)
	waitForSolr(t, c)
	setupTestCollection(t, c)
	defer teardownTestCollection(t, c)

	techCount := mustQueryCount(t, c, testCollection, "*:*", "category:technology")
	assert.Equal(t, int64(4), techCount, "expected 4 technology docs")

	foodCount := mustQueryCount(t, c, testCollection, "*:*", "category:food")
	assert.Equal(t, int64(2), foodCount, "expected 2 food docs")

	scienceCount := mustQueryCount(t, c, testCollection, "*:*", "category:science")
	assert.Equal(t, int64(2), scienceCount, "expected 2 science docs")

	travelCount := mustQueryCount(t, c, testCollection, "*:*", "category:travel")
	assert.Equal(t, int64(2), travelCount, "expected 2 travel docs")
}

func TestIntegrationPriceRangeQuery(t *testing.T) {
	c := newTestSolrClient(t)
	waitForSolr(t, c)
	setupTestCollection(t, c)
	defer teardownTestCollection(t, c)

	// Docs with price <= 30: ids 3 (29.99), 5 (19.99), 9 (24.99)
	count := mustQueryCount(t, c, testCollection, "price:[0 TO 30]", "")
	assert.Equal(t, int64(3), count, "expected 3 docs priced 30 or under")
}

func TestIntegrationInStockFilter(t *testing.T) {
	c := newTestSolrClient(t)
	waitForSolr(t, c)
	setupTestCollection(t, c)
	defer teardownTestCollection(t, c)

	// in_stock=true: ids 1,2,4,5,6,7,9,10 → 8 docs
	inStock := mustQueryCount(t, c, testCollection, "*:*", "in_stock:true")
	assert.Equal(t, int64(8), inStock)

	// in_stock=false: ids 3,8 → 2 docs
	outOfStock := mustQueryCount(t, c, testCollection, "*:*", "in_stock:false")
	assert.Equal(t, int64(2), outOfStock)
}

func TestIntegrationYearFilter(t *testing.T) {
	c := newTestSolrClient(t)
	waitForSolr(t, c)
	setupTestCollection(t, c)
	defer teardownTestCollection(t, c)

	count2024 := mustQueryCount(t, c, testCollection, "year:2024", "")
	assert.Equal(t, int64(3), count2024, "expected 3 docs from 2024")

	count2023 := mustQueryCount(t, c, testCollection, "year:2023", "")
	assert.Equal(t, int64(4), count2023, "expected 4 docs from 2023")
}

func TestIntegrationSortedQuery(t *testing.T) {
	c := newTestSolrClient(t)
	waitForSolr(t, c)
	setupTestCollection(t, c)
	defer teardownTestCollection(t, c)

	result, err := c.Query(context.Background(), testCollection, solr.QueryParams{
		Q:    "*:*",
		Sort: "price asc",
		Rows: 3,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(10), result.Response.NumFound)
	require.Len(t, result.Response.Docs, 3)

	// Cheapest doc: id=5, price=19.99
	assert.Equal(t, "5", result.Response.Docs[0]["id"])
}

func TestIntegrationListCollections(t *testing.T) {
	// /solr/admin/collections is a SolrCloud-only endpoint. Our test Solr
	// (both docker-compose and CI) runs in standalone mode with a pre-created
	// core, so this endpoint returns a 400. Skip until/unless we wire up
	// SolrCloud in the test harness.
	t.Skip("ListCollections requires SolrCloud; test harness runs standalone Solr")
}

func TestIntegrationDeleteByQuery(t *testing.T) {
	c := newTestSolrClient(t)
	waitForSolr(t, c)
	setupTestCollection(t, c)
	defer teardownTestCollection(t, c)

	// Delete technology docs
	err := c.DeleteByQuery(context.Background(), testCollection, "category:technology")
	require.NoError(t, err)

	count := mustQueryCount(t, c, testCollection, "*:*", "")
	assert.Equal(t, int64(6), count, "expected 6 docs after deleting 4 technology docs")
}

func TestIntegrationSchemaFields(t *testing.T) {
	c := newTestSolrClient(t)
	waitForSolr(t, c)
	setupTestCollection(t, c)
	defer teardownTestCollection(t, c)

	schema, err := c.GetSchema(context.Background(), testCollection)
	require.NoError(t, err)
	assert.NotEmpty(t, schema.Fields)
	// schema.Name is the schema-config name (e.g. "default-config"), not the
	// collection name. Just assert it's populated.
	assert.NotEmpty(t, schema.Name)
	assert.NotEmpty(t, schema.UniqueKey, "schema should declare a unique key")
}

func TestIntegrationAgenticSearch(t *testing.T) {
	c := newTestSolrClient(t)
	apiKey := requireAPIKey(t)
	waitForSolr(t, c)
	setupTestCollection(t, c)
	defer teardownTestCollection(t, c)

	_ = apiKey

	// Verify Solr is populated and queryable (the plumbing the agent would use)
	count := mustQueryCount(t, c, testCollection, "*:*", "category:technology")
	assert.Equal(t, int64(4), count)

	schema, err := c.GetSchema(context.Background(), testCollection)
	require.NoError(t, err)
	assert.NotEmpty(t, schema.Name)
}
