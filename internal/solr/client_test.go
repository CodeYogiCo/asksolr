package solr_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CodeYogiCo/asksolr/internal/solr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/test_col/select", r.URL.Path)
		assert.Equal(t, "title:golang", r.URL.Query().Get("q"))
		assert.Equal(t, "json", r.URL.Query().Get("wt"))

		resp := solr.QueryResponse{
			ResponseHeader: solr.ResponseHeader{Status: 0},
			Response: solr.ResponseBody{
				NumFound: 2,
				Docs: []map[string]interface{}{
					{"id": "1", "title": "Go Programming"},
					{"id": "2", "title": "Go Concurrency"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := solr.New(srv.URL)
	result, err := c.Query(context.Background(), "test_col", solr.QueryParams{
		Q:    "title:golang",
		Rows: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Response.NumFound)
	assert.Len(t, result.Response.Docs, 2)
	assert.Equal(t, "Go Programming", result.Response.Docs[0]["title"])
}

func TestGetSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/my_col/schema", r.URL.Path)

		resp := solr.SchemaResponse{
			Schema: solr.Schema{
				Name:      "my_col",
				UniqueKey: "id",
				Fields: []solr.SchemaField{
					{Name: "id", Type: "string", Indexed: true, Stored: true},
					{Name: "title", Type: "text_general", Indexed: true, Stored: true},
					{Name: "price", Type: "pfloat", Indexed: true, Stored: true},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := solr.New(srv.URL)
	schema, err := c.GetSchema(context.Background(), "my_col")
	require.NoError(t, err)
	assert.Equal(t, "my_col", schema.Name)
	assert.Equal(t, "id", schema.UniqueKey)
	assert.Len(t, schema.Fields, 3)
	assert.Equal(t, "title", schema.Fields[1].Name)
}

func TestListCollections(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "LIST", r.URL.Query().Get("action"))

		resp := solr.CollectionsResponse{
			ResponseHeader: solr.ResponseHeader{Status: 0},
			Collections:    []string{"products", "news", "users"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := solr.New(srv.URL)
	colls, err := c.ListCollections(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"products", "news", "users"}, colls)
}

func TestIndexDocs(t *testing.T) {
	var receivedDocs []map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/update")

		if err := json.NewDecoder(r.Body).Decode(&receivedDocs); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		resp := solr.UpdateResponse{ResponseHeader: solr.ResponseHeader{Status: 0}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := solr.New(srv.URL)
	docs := []map[string]interface{}{
		{"id": "1", "title": "Test Doc"},
		{"id": "2", "title": "Another Doc"},
	}
	err := c.IndexDocs(context.Background(), "test_col", docs)
	require.NoError(t, err)
	assert.Len(t, receivedDocs, 2)
	assert.Equal(t, "Test Doc", receivedDocs[0]["title"])
}

func TestDeleteByQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		del := payload["delete"].(map[string]interface{})
		assert.Equal(t, "id:123", del["query"])

		resp := solr.UpdateResponse{ResponseHeader: solr.ResponseHeader{Status: 0}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := solr.New(srv.URL)
	err := c.DeleteByQuery(context.Background(), "test_col", "id:123")
	require.NoError(t, err)
}

func TestQuerySolrError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := solr.QueryResponse{
			ResponseHeader: solr.ResponseHeader{Status: 400},
			Error:          &solr.SolrError{Msg: "bad query syntax", Code: 400},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := solr.New(srv.URL)
	_, err := c.Query(context.Background(), "test_col", solr.QueryParams{Q: "bad["})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad query syntax")
}

func TestQueryWithFilterAndSort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "price:[0 TO 100]", r.URL.Query().Get("q"))
		assert.Equal(t, "category:electronics", r.URL.Query().Get("fq"))
		assert.Equal(t, "price asc", r.URL.Query().Get("sort"))

		resp := solr.QueryResponse{
			Response: solr.ResponseBody{NumFound: 5, Docs: []map[string]interface{}{}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := solr.New(srv.URL)
	result, err := c.Query(context.Background(), "products", solr.QueryParams{
		Q:    "price:[0 TO 100]",
		FQ:   "category:electronics",
		Sort: "price asc",
		Rows: 20,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(5), result.Response.NumFound)
}
