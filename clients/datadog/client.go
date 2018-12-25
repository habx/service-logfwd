package datadog

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/habx/service-logfwd/clients"
	"go.uber.org/zap"
	"time"
)

type Client struct {
	srcClient clients.ClientHandler
	config    *Config // This doesn't belong to us (we MUST not modify it)
	log       *zap.SugaredLogger
	events    chan *LogEvent
}

func NewClient(ch clients.ClientHandler, baseConfig clients.Config) *Client {
	config := baseConfig.(*Config)
	clt := &Client{
		srcClient: ch,
		config:    config,
		log:       ch.Logger().With("log2x", "datadog"),
		events:    make(chan *LogEvent, config.QueueSize),
	}

	go clt.writeToDatadogTcpInput()

	return clt
}

// The log event as specified in the API doc
type LogEvent struct {
	Timestamp  int64
	Severity   clients.Level
	Attributes map[string]interface{}
	Tags       map[string]string
}

// Export modifies the content of the event
func (ev *LogEvent) export() string {
	ev.Attributes["timestamp"] = ev.Timestamp
	tags := ""
	i := 0
	for k, v := range ev.Tags {
		if i > 0 {
			tags += ","
		}
		i += 1
		tags += fmt.Sprintf("%s:%s", k, v)
	}
	ev.Attributes["ddtags"] = tags
	if b, err := json.Marshal(ev.Attributes); err == nil {
		return string(b)
	} else {
		return "{}"
	}
}

func (clt *Client) Send(srcEvent *clients.LogEvent) {
	dstEvent := &LogEvent{
		Timestamp:  srcEvent.Timestamp.UnixNano() / (1000 * 1000), // nano to milliseconds
		Attributes: make(map[string]interface{}),
		Tags:       make(map[string]string),
	}
	dstEvent.Attributes["ddsource"] = "logfwd"

	// Converting some keys to other keys in the message
	for initialKey, value := range srcEvent.Attributes {
		if initialKey == "@tags" {
			continue
		}
		if targetKey, ok := clt.config.KeysToMessageConversions[initialKey]; ok {
			if targetKey == "" {
				continue
			}
			dstEvent.Attributes[targetKey] = value
		} else if targetKey, ok := clt.config.KeysToTagsConversions[initialKey]; ok {
			dstEvent.Tags[targetKey], _ = value.(string)
		} else {
			dstEvent.Attributes[initialKey] = value
		}
	}

	clt.events <- dstEvent
}

func (clt *Client) Close() error {
	// Event closing the scaly HTTP sender
	clt.events <- nil
	return nil
}

func (clt *Client) Name() string {
	return "datadog"
}

func (clt *Client) writeToDatadogTcpInput() {

	var conn *tls.Conn
	var err error

	for {
		event := <-clt.events

		if event == nil {
			if err := conn.Close(); err != nil {
				clt.log.Warnw(
					"Could not close data",
					"err", err,
				)
			}
			break
		}

		if conn == nil {
			conn, err = tls.Dial("tcp", clt.config.Server, &tls.Config{})
			if err != nil {
				clt.log.Warnw(
					"Could not connect",
					"err", err,
				)
				time.Sleep(time.Second * 5)
				continue
			}
		}

		line := fmt.Sprintf("%s %s\n", clt.config.Token, event.export())
		clt.log.Debugw(
			"Sending data",
			"line", line,
		)

		if _, err := conn.Write([]byte(line)); err != nil {
			go func() {
				clt.events <- event
			}()

			clt.log.Warnw(
				"Could not send data",
				"err", err,
			)

			if err := conn.Close(); err != nil {
				clt.log.Warnw(
					"Error while closing connection",
					"err", err,
				)
			}
			conn = nil
		}
	}
}
