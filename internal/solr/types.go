package solr

import "fmt"

// QueryParams holds parameters for a Solr query.
type QueryParams struct {
	Q    string
	FQ   string
	Sort string
	Rows int
	FL   string
}

// QueryResponse is the top-level Solr query response.
type QueryResponse struct {
	ResponseHeader ResponseHeader `json:"responseHeader"`
	Response       ResponseBody   `json:"response"`
	Error          *SolrError     `json:"error,omitempty"`
}

type ResponseHeader struct {
	Status int `json:"status"`
	QTime  int `json:"QTime"`
}

type ResponseBody struct {
	NumFound int64                    `json:"numFound"`
	Start    int64                    `json:"start"`
	Docs     []map[string]interface{} `json:"docs"`
}

// SchemaResponse is returned by GET /schema.
type SchemaResponse struct {
	Schema Schema     `json:"schema"`
	Error  *SolrError `json:"error,omitempty"`
}

type Schema struct {
	Name          string        `json:"name"`
	UniqueKey     string        `json:"uniqueKey"`
	Fields        []SchemaField `json:"fields"`
	DynamicFields []SchemaField `json:"dynamicFields,omitempty"`
}

type SchemaField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Indexed  bool   `json:"indexed,omitempty"`
	Stored   bool   `json:"stored,omitempty"`
	MultiVal bool   `json:"multiValued,omitempty"`
	Required bool   `json:"required,omitempty"`
}

// CollectionsResponse is returned by the collections admin API.
type CollectionsResponse struct {
	ResponseHeader ResponseHeader `json:"responseHeader"`
	Collections    []string       `json:"collections"`
	Error          *SolrError     `json:"error,omitempty"`
}

// UpdateResponse is returned by update operations.
type UpdateResponse struct {
	ResponseHeader ResponseHeader `json:"responseHeader"`
	Error          *SolrError     `json:"error,omitempty"`
}

// SolrError is returned in the error field on failure.
type SolrError struct {
	Msg  string `json:"msg"`
	Code int    `json:"code"`
}

func (e *SolrError) Error() string {
	return fmt.Sprintf("solr error %d: %s", e.Code, e.Msg)
}
