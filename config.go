package main

import (
	"fmt"

	"github.com/habx/service-logfwd/clients"
	"github.com/habx/service-logfwd/clients/list"
	"github.com/kelseyhightower/envconfig"
)

// Config is the main config
type Config struct {
	ListenAddr           string `envconfig:"LISTEN_ADDR"`             // Listening address
	LogEnv               string `envconfig:"LOG_ENV"`                 // Logging environment: dev or prod
	LogstashMaxEventSize int    `envconfig:"LOGSTASH_EVENT_MAX_SIZE"` // Maximum size accepted for reading data in logstash
	LogstashAuthKey      string `envconfig:"LOGSTASH_AUTH_KEY"`       // Logstash authentication key
	LogstashAuthValue    string `envconfig:"LOGSTASH_AUTH_VALUE"`     // Logstash authentication value
	OutputClientConfigs  map[string]clients.Config
}

func NewConfig() *Config {
	return &Config{
		ListenAddr:           ":5050",
		LogstashMaxEventSize: 300 * 1024, // 300KB
		LogEnv:               "prod",
		OutputClientConfigs:  make(map[string]clients.Config),
	}
}

func (c *Config) Load() error {
	/*
		if err := c.Scalyr.Load(); err != nil {
			return fmt.Errorf("couldn't load scalyr config: %s", err)
		}
	*/
	if err := envconfig.Process("", c); err != nil {
		return fmt.Errorf("couldn't load config from env vars: %s", err)
	}
	if err := c.check(); err != nil {
		return fmt.Errorf("config check issue: %s", err)
	}

	{
		anOutputWasEnabled := false

		for _, oc := range list.LIST {
			conf := oc.Config()
			if err := conf.Load(); err != nil {
				return fmt.Errorf("couldn't load output client of %s: %s", oc.Name(), err)
			}
			anOutputWasEnabled = anOutputWasEnabled || conf.Enabled()
			c.OutputClientConfigs[oc.Name()] = conf
		}

		if !anOutputWasEnabled {
			return fmt.Errorf("at least one output client should be enabled")
		}
	}

	return nil
}

func (c *Config) check() error {
	return nil
}
