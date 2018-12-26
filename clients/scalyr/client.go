package scalyr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/habx/service-logfwd/clients"
	"github.com/satori/go.uuid"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
	"time"
)

type Client struct {
	srcClient   clients.ClientHandler
	config      *Config // This doesn't belong to us (we MUST not modify it)
	log         *zap.SugaredLogger
	events      chan *LogEvent
	httpClient  http.Client
	maxNbEvents int
}

func NewClient(ch clients.ClientHandler, baseConfig clients.Config) *Client {
	config := baseConfig.(*Config)
	clt := &Client{
		srcClient:   ch,
		config:      config,
		log:         ch.Logger().With("log2x", "scalyr"),
		events:      make(chan *LogEvent, config.QueueSize),
		maxNbEvents: config.RequestMaxNbEvents,
	}

	go clt.writeToScalyr()

	return clt
}

// The request as specified in the API doc ( https://www.scalyr.com/help/api#addEvents )
type UploadData struct {
	Token       string                 `json:"token"`
	Session     string                 `json:"session"`
	SessionInfo map[string]interface{} `json:"sessionInfo,omitempty"`
	Threads     map[string]interface{} `json:"threads,omitempty"`
	Events      []*LogEvent            `json:"events"`
}

// The log event as specified in the API doc
type LogEvent struct {
	Timestamp   int64                  `json:"ts"`
	Severity    uint8                  `json:"sev"`
	Attributes  map[string]interface{} `json:"attrs"`
	sessionInfo map[string]interface{}
}

func scalyrSeverityConversion(level clients.Level) uint8 {
	return uint8(level)
}

func (clt *Client) Name() string {
	return "scalyr"
}

func (clt *Client) Send(srcEvent *clients.LogEvent) {
	dstEvent := &LogEvent{
		Timestamp:  srcEvent.Timestamp.UnixNano(),
		Severity:   scalyrSeverityConversion(srcEvent.Severity),
		Attributes: make(map[string]interface{}),
	}

	// Converting some keys to other keys in the message
	for initialKey, value := range srcEvent.Attributes {
		if targetKey, ok := clt.config.KeysToMessageConversions[initialKey]; ok {
			if targetKey == "" {
				continue
			}
			dstEvent.Attributes[targetKey] = value
		} else if targetKey, ok := clt.config.KeysToSessionInfoConversions[initialKey]; ok {
			if dstEvent.sessionInfo == nil {
				dstEvent.sessionInfo = make(map[string]interface{})
			}
			dstEvent.sessionInfo[targetKey] = value
		} else {
			dstEvent.Attributes[initialKey] = value
		}
	}

	// Converting some keys to other keys in the session
	for initialKey, value := range dstEvent.Attributes {
		if targetKey, ok := clt.config.KeysToSessionInfoConversions[initialKey]; ok {
			if dstEvent.sessionInfo == nil {
				dstEvent.sessionInfo = make(map[string]interface{})
			}
			dstEvent.sessionInfo[targetKey] = value
			delete(dstEvent.Attributes, initialKey)
		}
	}

	clt.events <- dstEvent
}

func (clt *Client) Close() error {
	// Event closing the scaly HTTP sender
	clt.events <- nil
	return nil
}

func (clt *Client) writeToScalyr() {
	uploadData := &UploadData{
		Token:   clt.config.Token,
		Session: fmt.Sprint(uuid.NewV4()),
	}

	loop := true

	sessionInfo := map[string]interface{}{
		"conn_src": clt.srcClient.Addr().String(),
		"conn_id":  clt.srcClient.ID(),
		"source":   "logfwd",
	}
	// sessionInfoLastTransmission := time.Unix(0, 0)
	events := make([]*LogEvent, clt.config.RequestMaxNbEvents)

	for loop {
		events := events[:0]

		// Every 3 minutes, we give some info about the current steam
		// sessionInfoSend := time.Now().Sub(sessionInfoLastTransmission) > time.Second*30

		// We read all the events
		for len(events) == 0 || (len(clt.events) > 0 && len(events) < clt.maxNbEvents) {
			event := <-clt.events
			if event == nil {
				loop = false
				// sessionInfoSend = true
			} else {
				// Adding all the event's sessionInfo to the stream session info
				if event.sessionInfo != nil {
					for k, v := range event.sessionInfo {
						if sessionInfo[k] != v {
							sessionInfo[k] = v
							// sessionInfoSend = true
						}
					}
					event.sessionInfo = nil
				}
				events = append(events, event)
			}
		}

		// Always send sessionInfo for now
		// sessionInfoSend = true

		// If we have some session data to send
		// if sessionInfoSend {
		// sessionInfoSend = false
		// sessionInfoLastTransmission = time.Now()
		uploadData.SessionInfo = sessionInfo
		// }

		uploadData.Events = events

		if err := clt.sendRequest(uploadData); err != nil {
			clt.log.Warnw(
				"Problem sending data",
				"err", err,
			)
		}
	}
}

func (clt *Client) sendRequest(uploadData *UploadData) error {
	var rawJson []byte
	var err error
	for {
		rawJson, err = json.Marshal(uploadData)
		size := len(rawJson)
		if size > clt.config.RequestMaxSize {
			if clt.maxNbEvents > 1 {
				clt.log.Debugw(
					"Query too big, reducing the number of events",
					"nbEvents", len(uploadData.Events),
					"maxNbEvents", clt.maxNbEvents,
					"size", size,
					"maxSize", clt.config.RequestMaxSize,
				)
				clt.maxNbEvents = len(uploadData.Events) - 1
				lastEvent := uploadData.Events[len(uploadData.Events)-1]
				uploadData.Events = uploadData.Events[:len(uploadData.Events)-1]
				go func() { // It could already be full and we would create a deadlock
					clt.events <- lastEvent
				}()
				continue
			} else {
				return fmt.Errorf(
					"request is too big: requestSize=%d > maxRequestSize=%d",
					len(rawJson),
					clt.config.RequestMaxSize,
				)
			}
		}
		break
	}

	if err != nil {
		clt.log.Errorw(
			"Problem generating JSON",
			"err", err,
		)
		return err
	}

	clt.log.Debugw(
		"Scalyr HTTP Request",
		"nbSentEvents", len(uploadData.Events),
		"nbWaitingEvents", len(clt.events),
		"data", string(rawJson),
	)

	backoffTime := time.Duration(0)
	backoffIncrement := time.Second
	for backoffIncrement < time.Minute {
		time.Sleep(time.Duration(time.Millisecond.Nanoseconds() * int64(clt.config.RequestMinPeriod)))

		var resp *http.Response
		resp, err = clt.httpClient.Post(clt.config.scalyrEndpoint, "application/json", bytes.NewBuffer(rawJson))

		if err != nil {
			clt.log.Warnw(
				"HTTP request error",
				"err", err,
			)
			backoffTime += backoffIncrement
			time.Sleep(backoffTime)
			backoffIncrement *= 2
			continue
		}

		if resp.StatusCode == 200 {
			if uploadData.SessionInfo != nil {
				uploadData.SessionInfo = nil
			}
			if clt.maxNbEvents < clt.config.RequestMaxNbEvents {
				clt.maxNbEvents += 1
			}
		}

		{
			var body []byte
			if body, err = ioutil.ReadAll(resp.Body); err != nil {
				clt.log.Warnw(
					"Issue reading response",
					"err", err,
				)
				continue
			} else {
				clt.log.Debugw("Scalyr HTTP Response",
					"statusCode", resp.StatusCode,
					"statusMsg", resp.Status,
					"body", string(body),
				)
			}
		}

		if err = resp.Body.Close(); err != nil {
			clt.log.Errorw(
				"Problem closing HTTP response body",
				"err", err,
			)
			continue
		}

		return nil
	}
	return fmt.Errorf("couldn't send our data")
}
