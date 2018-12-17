package main

import (
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"strings"
)

type Config struct {
	ListenAddr                  string // Listening address
	ScalyrServer                string // Scalyr target URL
	ScalyrToken                 string // Scalyr token
	ScalyrMaxNbEvents           int    // Scalyr max events
	ScalyrMaxRequestSize        int    // Scalyr max request size
	ScalyrMinTimeBetweenQueries int    // Milliseconds between queries (mostly used for tests)
	LogstashMaxEventSize        int    // Maximum size accepted for reading data in logstash
	LogEnv                      string // Logging environment: dev or prod
	QueueSize                   int    // Maximum number of events to queue
	scalyrEndpoint              string // Endpoint
}

func NewConfig() Config {
	return Config{
		ListenAddr:                  ":5050",
		ScalyrServer:                "https://www.scalyr.com",
		ScalyrMaxNbEvents:           20,
		ScalyrMaxRequestSize:        2048, //1024 * 1024 * 2, // 2MB is much lower than the allowed 3MB
		ScalyrMinTimeBetweenQueries: 1000,
		LogstashMaxEventSize:        300 * 1024,
		QueueSize:                   1000,
	}
}

func (c *Config) Load() error {
	if err := envconfig.Process("ls2s", c); err != nil {
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
		return fmt.Errorf("no scalyr token was provided, use LS2S_TOKEN env var to specify it")
	}
	if strings.HasSuffix(c.ScalyrServer, "/") {
		return fmt.Errorf("do not end the URL by a /")
	}
	return nil
}
