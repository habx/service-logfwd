package main

import (
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"strings"
)

type Config struct {
	ListenAddr                   string            `envconfig:"LISTEN_ADDR"`                     // Listening address
	ScalyrServer                 string            `envconfig:"SCALYR_SERVER"`                   // Scalyr target URL
	ScalyrToken                  string            `envconfig:"SCALYR_WRITELOG_TOKEN"`           // Scalyr token
	ScalyrRequestMaxNbEvents     int               `envconfig:"SCALYR_REQUEST_MAX_NB_EVENTS"`    // Scalyr max nb of events
	ScalyrRequestMaxSize         int               `envconfig:"SCALYR_REQUEST_MAX_REQUEST_SIZE"` // Scalyr max request size
	ScalyrRequestMinPeriod       int               `envconfig:"SCALYR_REQUEST_MIN_PERIOD"`       // Milliseconds between queries (mostly used for tests)
	LogstashMaxEventSize         int               `envconfig:"LOGSTASH_EVENT_MAX_SIZE"`         // Maximum size accepted for reading data in logstash
	LogEnv                       string            `envconfig:"LOG_ENV"`                         // Logging environment: dev or prod
	QueueSize                    int               `envconfig:"QUEUE_SIZE"`                      // Maximum number of events to queue between logstash and scalyr
	KeysToMessageConversions     map[string]string `envconfig:"FIELDS_CONV_MESSAGE"`             // Logstash to scalyr events fields conversion
	KeysToSessionInfoConversions map[string]string `envconfig:"FIELDS_CONV_SESSION"`             // Logstash to scalyr session fields conversion
	scalyrEndpoint               string            // Endpoint
}

func NewConfig() *Config {
	return &Config{
		ListenAddr:               ":5050",
		ScalyrServer:             "https://www.scalyr.com",
		ScalyrRequestMaxNbEvents: 20,
		ScalyrRequestMaxSize:     2 * 1024 * 1024, // 2MB is much lower than the allowed 3MB
		ScalyrRequestMinPeriod:   1000,
		LogstashMaxEventSize:     300 * 1024, // 300KB
		QueueSize:                1000,
		// These are the attribute keys to convert within a message
		KeysToMessageConversions: map[string]string{
			"@source_host": "hostname",
			"@source_path": "file_path",
			"@message":     "message",
			"@type":        "logstash_type",
			"@source":      "logstash_source",
			"@tags":        "tags",
		},
		// These are the attribute keys to convert and move to the session
		KeysToSessionInfoConversions: map[string]string{
			"appname": "serverHost",
			"env":     "logfile",
		},
		LogEnv: "prod",
	}
}

func (c *Config) Load() error {
	if err := envconfig.Process("", c); err != nil {
		return fmt.Errorf("couldn't load config from env vars: %s", err)
	}
	c.scalyrEndpoint = fmt.Sprintf("%s/addEvents", c.ScalyrServer)
	if err := c.check(); err != nil {
		return fmt.Errorf("config check issue: %s", err)
	}
	return nil
}

func (c *Config) check() error {
	if c.ScalyrToken == "" {
		return fmt.Errorf("no scalyr token was provided, use SCALYR_WRITELOG_TOKEN env var to specify it")
	}
	if strings.HasSuffix(c.ScalyrServer, "/") {
		return fmt.Errorf("do not end the URL by a /")
	}
	return nil
}
