package datadog

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

// Config is the datadog output client config
type Config struct {
	Token                    string            `envconfig:"DATADOG_TOKEN"`               // Datadog token
	Server                   string            `envconfig:"DATADOG_SERVER"`              // Datadog server
	QueueSize                int               `envconfig:"DATADOG_QUEUESIZE"`           // Datadog queue size
	KeysToMessageConversions map[string]string `envconfig:"DATADOG_FIELDS_CONV_MESSAGE"` // Logstash to events fields conversion
	KeysToTagsConversions    map[string]string `envconfig:"DATADOG_FIELDS_CONV_TAGS"`    // Logstash to session fields conversion
}

// NewConfig creates a new config instance
func NewConfig() *Config {
	return &Config{
		// "tcp-intake.logs.datadoghq.eu:443" for europe
		Server:    "intake.logs.datadoghq.com:15516",
		QueueSize: 20,
		KeysToMessageConversions: map[string]string{
			"appname": "service",
			// "hostname": "ddhostname",
		},
		KeysToTagsConversions: map[string]string{
			"env": "env",
		},
	}
}

// Load performs the config loading
func (c *Config) Load() error {
	if err := envconfig.Process("", c); err != nil {
		return fmt.Errorf("couldn't load config from env vars: %s", err)
	}
	if err := c.check(); err != nil {
		return fmt.Errorf("config check issue: %s", err)
	}
	return nil
}

// Enabled reports if the client should be used
func (c *Config) Enabled() bool {
	return c.Token != ""
}

func (c *Config) check() error {
	return nil
}
