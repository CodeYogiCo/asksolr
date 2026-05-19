package solr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client is a Solr HTTP client.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a Solr client. baseURL should be e.g. http://localhost:8983/solr
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Ping checks Solr connectivity.
func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.get(ctx, "/admin/info/system", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("solr ping failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Query executes a Solr query against a collection.
func (c *Client) Query(ctx context.Context, collection string, p QueryParams) (*QueryResponse, error) {
	params := url.Values{}
	params.Set("q", p.Q)
	params.Set("wt", "json")
	if p.FQ != "" {
		params.Set("fq", p.FQ)
	}
	if p.Sort != "" {
		params.Set("sort", p.Sort)
	}
	if p.Rows > 0 {
		params.Set("rows", strconv.Itoa(p.Rows))
	} else {
		params.Set("rows", "10")
	}
	if p.FL != "" {
		params.Set("fl", p.FL)
	}

	path := fmt.Sprintf("/%s/select?%s", collection, params.Encode())
	resp, err := c.get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var qr QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&qr); err != nil {
		return nil, fmt.Errorf("failed to decode query response: %w", err)
	}
	if qr.Error != nil {
		return nil, qr.Error
	}
	return &qr, nil
}

// GetSchema retrieves the schema for a collection.
func (c *Client) GetSchema(ctx context.Context, collection string) (*Schema, error) {
	path := fmt.Sprintf("/%s/schema", collection)
	resp, err := c.get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var sr SchemaResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("failed to decode schema response: %w", err)
	}
	if sr.Error != nil {
		return nil, sr.Error
	}
	return &sr.Schema, nil
}

// ListCollections lists all Solr collections.
func (c *Client) ListCollections(ctx context.Context) ([]string, error) {
	params := url.Values{}
	params.Set("action", "LIST")
	params.Set("wt", "json")
	path := "/admin/collections?" + params.Encode()

	resp, err := c.get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var cr CollectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("failed to decode collections response: %w", err)
	}
	if cr.Error != nil {
		return nil, cr.Error
	}
	return cr.Collections, nil
}

// CreateCollection creates a new Solr collection.
func (c *Client) CreateCollection(ctx context.Context, name string, numShards, replicationFactor int) error {
	params := url.Values{}
	params.Set("action", "CREATE")
	params.Set("name", name)
	params.Set("numShards", strconv.Itoa(numShards))
	params.Set("replicationFactor", strconv.Itoa(replicationFactor))
	params.Set("wt", "json")
	path := "/admin/collections?" + params.Encode()

	resp, err := c.get(ctx, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var ur UpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ur); err != nil {
		return fmt.Errorf("failed to decode create response: %w", err)
	}
	if ur.Error != nil {
		return ur.Error
	}
	return nil
}

// DeleteCollection deletes a Solr collection.
func (c *Client) DeleteCollection(ctx context.Context, name string) error {
	params := url.Values{}
	params.Set("action", "DELETE")
	params.Set("name", name)
	params.Set("wt", "json")
	path := "/admin/collections?" + params.Encode()

	resp, err := c.get(ctx, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var ur UpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ur); err != nil {
		return fmt.Errorf("failed to decode delete response: %w", err)
	}
	if ur.Error != nil {
		return ur.Error
	}
	return nil
}

// IndexDocs indexes a slice of documents into a collection.
func (c *Client) IndexDocs(ctx context.Context, collection string, docs []map[string]interface{}) error {
	body, err := json.Marshal(docs)
	if err != nil {
		return fmt.Errorf("failed to marshal docs: %w", err)
	}

	path := fmt.Sprintf("/%s/update?commit=true&wt=json", collection)
	resp, err := c.post(ctx, path, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var ur UpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ur); err != nil {
		return fmt.Errorf("failed to decode update response: %w", err)
	}
	if ur.Error != nil {
		return ur.Error
	}
	return nil
}

// DeleteByQuery deletes documents matching a Solr query.
func (c *Client) DeleteByQuery(ctx context.Context, collection, query string) error {
	payload := map[string]interface{}{"delete": map[string]string{"query": query}}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal delete payload: %w", err)
	}

	path := fmt.Sprintf("/%s/update?commit=true&wt=json", collection)
	resp, err := c.post(ctx, path, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var ur UpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ur); err != nil {
		return fmt.Errorf("failed to decode delete response: %w", err)
	}
	if ur.Error != nil {
		return ur.Error
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.httpClient.Do(req)
}

func (c *Client) post(ctx context.Context, path, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	return c.httpClient.Do(req)
}
