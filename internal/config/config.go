package config

import "fmt"

type Env string

const (
	Dev  Env = "dev"
	Prod Env = "prod"
)

type Config struct {
	Env           Env
	SolrURL       string
	Collection    string
	APIKey        string
	Verbose       bool
	MaxIterations int
}

func (c *Config) SolrBase() string {
	return fmt.Sprintf("%s/solr", c.SolrURL)
}

func (c *Config) IsProd() bool { return c.Env == Prod }
