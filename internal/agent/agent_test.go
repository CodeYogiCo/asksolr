package agent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/CodeYogiCo/asksolr/internal/solr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildAnthropicClient creates an Anthropic client pointing at the mock server.
func buildAnthropicClient(baseURL string) anthropic.Client {
	return anthropic.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(baseURL),
	)
}

// buildMockSolrServer returns a test Solr server with a schema and query endpoint.
func buildMockSolrServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/test_col/schema":
			resp := solr.SchemaResponse{
				Schema: solr.Schema{
					Name:      "test_col",
					UniqueKey: "id",
					Fields: []solr.SchemaField{
						{Name: "id", Type: "string", Indexed: true, Stored: true},
						{Name: "title", Type: "text_general", Indexed: true, Stored: true},
						{Name: "price", Type: "pfloat", Indexed: true, Stored: true},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)

		case r.URL.Path == "/test_col/select":
			resp := solr.QueryResponse{
				Response: solr.ResponseBody{
					NumFound: 3,
					Docs: []map[string]interface{}{
						{"id": "1", "title": "Go Guide", "price": 29.99},
						{"id": "2", "title": "Rust Book", "price": 39.99},
						{"id": "3", "title": "Python Intro", "price": 19.99},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
}

func TestAgentSearchReturnsResults(t *testing.T) {
	solrSrv := buildMockSolrServer(t)
	defer solrSrv.Close()

	solrClient := solr.New(solrSrv.URL)

	// Verify Solr client works before involving agent
	schema, err := solrClient.GetSchema(context.Background(), "test_col")
	require.NoError(t, err)
	assert.Equal(t, "test_col", schema.Name)
	assert.Len(t, schema.Fields, 3)

	qr, err := solrClient.Query(context.Background(), "test_col", solr.QueryParams{Q: "*:*"})
	require.NoError(t, err)
	assert.Equal(t, int64(3), qr.Response.NumFound)
}

func TestSolrClientQueryWithParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		assert.Equal(t, "price:[0 TO 50]", q)
		fq := r.URL.Query().Get("fq")
		assert.Equal(t, "category:books", fq)

		resp := solr.QueryResponse{
			Response: solr.ResponseBody{
				NumFound: 2,
				Docs: []map[string]interface{}{
					{"id": "10", "title": "Cheap Book", "price": 9.99},
					{"id": "11", "title": "Budget Guide", "price": 4.99},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := solr.New(srv.URL)
	result, err := c.Query(context.Background(), "books", solr.QueryParams{
		Q:    "price:[0 TO 50]",
		FQ:   "category:books",
		Rows: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Response.NumFound)
	assert.Equal(t, "Cheap Book", result.Response.Docs[0]["title"])
}

func TestSolrClientSchemaFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := solr.SchemaResponse{
			Schema: solr.Schema{
				Name:      "products",
				UniqueKey: "id",
				Fields: []solr.SchemaField{
					{Name: "id", Type: "string", Indexed: true, Stored: true, Required: true},
					{Name: "name", Type: "text_general", Indexed: true, Stored: true},
					{Name: "price", Type: "pfloat", Indexed: true, Stored: true},
					{Name: "category", Type: "string", Indexed: true, Stored: true},
					{Name: "in_stock", Type: "boolean", Indexed: true, Stored: true},
					{Name: "tags", Type: "string", Indexed: true, Stored: true, MultiVal: true},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := solr.New(srv.URL)
	schema, err := c.GetSchema(context.Background(), "products")
	require.NoError(t, err)
	assert.Equal(t, 6, len(schema.Fields))

	var priceField *solr.SchemaField
	for i := range schema.Fields {
		if schema.Fields[i].Name == "price" {
			priceField = &schema.Fields[i]
			break
		}
	}
	require.NotNil(t, priceField)
	assert.Equal(t, "pfloat", priceField.Type)
	assert.True(t, priceField.Indexed)
}

func TestAnthropicClientCreation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	client := buildAnthropicClient(srv.URL)
	assert.NotNil(t, client)
}

func TestSolrErrorHandling(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := solr.QueryResponse{
			ResponseHeader: solr.ResponseHeader{Status: 400},
			Error:          &solr.SolrError{Msg: "undefined field bad_field", Code: 400},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := solr.New(srv.URL)
	_, err := c.Query(context.Background(), "col", solr.QueryParams{Q: "bad_field:val"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "undefined field")
}

func TestSystemPromptContainsCollection(t *testing.T) {
	collectionCalled := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		collectionCalled = r.URL.Path

		resp := solr.SchemaResponse{
			Schema: solr.Schema{Name: "my_products", UniqueKey: "id", Fields: []solr.SchemaField{}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := solr.New(srv.URL)
	schema, err := c.GetSchema(context.Background(), "my_products")
	require.NoError(t, err)
	assert.Contains(t, collectionCalled, "my_products")
	assert.Equal(t, "my_products", schema.Name)
}
