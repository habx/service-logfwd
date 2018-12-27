package scalyr

import (
	"fmt"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Server                       string            `envconfig:"SCALYR_SERVER"`                   // Scalyr target URL
	Token                        string            `envconfig:"SCALYR_WRITELOG_TOKEN"`           // Scalyr token
	KeysToMessageConversions     map[string]string `envconfig:"SCALYR_FIELDS_CONV_MESSAGE"`      // Logstash to scalyr events fields conversion
	KeysToSessionInfoConversions map[string]string `envconfig:"SCALYR_FIELDS_CONV_SESSION"`      // Logstash to scalyr session fields conversion
	RequestMaxNbEvents           int               `envconfig:"SCALYR_REQUEST_MAX_NB_EVENTS"`    // Scalyr max nb of events
	RequestMaxSize               int               `envconfig:"SCALYR_REQUEST_MAX_REQUEST_SIZE"` // Scalyr max request size
	RequestMinPeriod             int               `envconfig:"SCALYR_REQUEST_MIN_PERIOD"`       // Milliseconds between queries (mostly used for tests)
	QueueSize                    int               `envconfig:"SCALYR_QUEUE_SIZE"`               // Maximum number of events to queue between logstash and scalyr
	scalyrEndpoint               string
}

func NewConfig() *Config {
	return &Config{
		Server:             "https://www.scalyr.com",
		RequestMaxNbEvents: 20,
		RequestMaxSize:     2 * 1024 * 1024, // 2MB is much lower than the allowed 3MB
		RequestMinPeriod:   1000,
		QueueSize:          1000,
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
	}
}

func (c *Config) Load() error {
	if err := envconfig.Process("", c); err != nil {
		return fmt.Errorf("couldn't load config from env vars: %s", err)
	}
	if err := c.check(); err != nil {
		return fmt.Errorf("config check issue: %s", err)
	}

	c.scalyrEndpoint = fmt.Sprintf("%s/addEvents", c.Server)

	return nil
}

func (c *Config) Enabled() bool {
	return c.Token != ""
}

func (c *Config) check() error {
	if strings.HasSuffix(c.Server, "/") {
		return fmt.Errorf("do not end the URL by a /")
	}
	return nil
}
