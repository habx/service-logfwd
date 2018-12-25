package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/satori/go.uuid"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

type ClientHandler struct {
	Conn          net.Conn
	ID            int
	server        *Server
	events        chan *LogEvent
	httpClient    http.Client
	totalNbEvents int
	arrivalTime   time.Time
	sessionInfo   map[string]interface{}
	maxNbEvents   int
	log           *zap.SugaredLogger
	needsAuth     bool
}

func (srv *Server) NewClientHandler(conn net.Conn, nb int) *ClientHandler {
	return &ClientHandler{
		server:      srv,
		Conn:        conn,
		ID:          nb,
		events:      make(chan *LogEvent, srv.config.QueueSize),
		arrivalTime: time.Now(),
		sessionInfo: make(map[string]interface{}),
		maxNbEvents: srv.config.ScalyrRequestMaxNbEvents,
		log:         srv.log.With("clientID", nb),
		needsAuth:   srv.config.LogstashAuthKey != "",
	}
}

func (clt *ClientHandler) run() {
	clt.log.Infow("Client connected")

	// It would have been interesting to send a first message explaining that this client just connected, but this
	// prevents from sending the proper initial sessionInfo extracted from the first logstash message.
	/*
		clt.events <- &LogEvent{
			Severity:  2,
			Timestamp: clt.arrivalTime.UnixNano(),
			Attributes: map[string]interface{}{
				"message": "Client connected",
				"action":  "client_connected",
			},
		}
	*/
	go clt.writeToScalyr()
	defer func() {
		departure := time.Now()

		// We only log disconnection if at least one event was parsed
		if clt.totalNbEvents > 0 {
			clt.events <- &LogEvent{
				Severity:  2,
				Timestamp: departure.UnixNano(),
				Attributes: map[string]interface{}{
					"message":       "Client disconnected",
					"action":        "client_disconnected",
					"duration":      departure.Sub(clt.arrivalTime) / time.Second,
					"totalNbEvents": clt.totalNbEvents,
				},
			}
		}

		// Event closing the scaly HTTP sender
		clt.events <- nil

		if err := clt.Conn.Close(); err != nil {
			clt.log.Warnw("Issue closing connection", "err", err)
		}
	}()
	reader := bufio.NewReaderSize(clt.Conn, clt.server.config.LogstashMaxEventSize)
	for {
		lineRaw, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				clt.log.Infow("Client disconnected")
			} else {
				clt.log.Errorw("Couldn't read from client", "err", err)
			}
			return
		}
		if err := clt.ParseLogstashLine(lineRaw); err != nil {
			clt.log.Errorw("Couldn't parse line from client", "err", err)
			if _, err := clt.Conn.Write([]byte(err.Error())); err != nil {
				clt.log.Infow("Couldn't write bye bye message", "err", err)
			}
			return
		}
	}
}

const (
	LvlFinest   = int8(0)
	LvlTrace    = int8(1)
	LvlDebug    = int8(2)
	LvlInfo     = int8(3)
	LvlWarning  = int8(4)
	LvlError    = int8(5)
	LvlCritical = int8(6)
)

// https://www.scalyr.com/help/parsing-logs#specialAttrs
var SeverityConversions = map[string]int8{
	"finest":    LvlFinest,
	"finer":     LvlTrace,
	"trace":     LvlTrace,
	"fine":      LvlDebug,
	"debug":     LvlDebug,
	"info":      LvlInfo,
	"notice":    LvlInfo,
	"warn":      LvlWarning,
	"warning":   LvlWarning,
	"error":     LvlError,
	"fatal":     LvlCritical,
	"emerg":     LvlCritical,
	"emergency": LvlCritical,
	"crit":      LvlCritical,
	"critical":  LvlCritical,
	"panic":     LvlCritical,
	"alert":     LvlCritical,
	//"i":         LvlInfo,
	//"w":         LvlWarning,
	//"err":       LvlError,
	//"e":         LvlError,
	//"f":         LvlCritical,
}

func (clt *ClientHandler) ParseLogstashLine(line string) error {
	var lineJson map[string]interface{}

	clt.log.Debugw(
		"Received from logstash",
		"line", line,
	)

	if err := json.Unmarshal([]byte(line), &lineJson); err != nil {
		return err
	}

	// Checking authentication if required
	if clt.needsAuth {
		value, ok := lineJson[clt.server.config.LogstashAuthKey]
		if !ok || clt.server.config.LogstashAuthValue != value {
			return fmt.Errorf("wrong authentication with key %s", clt.server.config.LogstashAuthKey)
		}
		clt.needsAuth = false
	}

	if clt.server.config.LogstashAuthKey != "" {
		delete(lineJson, clt.server.config.LogstashAuthKey)
	}

	event := &LogEvent{
		Attributes: lineJson,
	}

	// We copy the extra fields to the root (otherwise, scalyr will index them as big chunk of a JSON string)
	if value, ok := event.Attributes["@fields"]; ok {
		switch v := value.(type) {
		case map[string]interface{}:
			for fieldsKey, value := range v {
				if _, ok := event.Attributes[fieldsKey]; !ok {
					event.Attributes[fieldsKey] = value
				}
			}
		}
		delete(event.Attributes, "@fields")
	}

	// Converting some keys to other keys in the message
	for initialKey, value := range event.Attributes {
		if targetKey, ok := clt.server.config.KeysToMessageConversions[initialKey]; ok {
			event.Attributes[targetKey] = value
			delete(event.Attributes, initialKey)
		}
	}

	// Converting some keys to other keys in the session
	for initialKey, value := range event.Attributes {
		if targetKey, ok := clt.server.config.KeysToSessionInfoConversions[initialKey]; ok {
			if event.sessionInfo == nil {
				event.sessionInfo = make(map[string]interface{})
			}
			event.sessionInfo[targetKey] = value
			delete(event.Attributes, initialKey)
		}
	}

	// Fetching the timestamp
	if strTs, ok := event.Attributes["@timestamp"].(string); ok {
		if timestamp, err := time.Parse(time.RFC3339Nano, strTs); err == nil {
			event.Timestamp = timestamp.UnixNano()
			delete(event.Attributes, "@timestamp")
		}
	}

	{ // Fetching the loglevel
		var level string
		var severity int8
		var ok bool
		for _, keyName := range []string{"level", "levelname"} {
			if level, ok = event.Attributes[keyName].(string); ok {
				delete(event.Attributes, keyName)
			}
		}
		if ok {
			if severity, ok = SeverityConversions[strings.ToLower(level)]; ok {
				event.Severity = severity
			}
		}
		if !ok {
			event.Severity = 3
		}
	}

	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixNano()
	}

	clt.totalNbEvents += 1
	clt.events <- event

	return nil
}

// The request as specified in the API doc ( https://www.scalyr.com/help/api#addEvents )
type ScalyrUploadData struct {
	Token       string                 `json:"token"`
	Session     string                 `json:"session"`
	SessionInfo map[string]interface{} `json:"sessionInfo,omitempty"`
	Threads     map[string]interface{} `json:"threads,omitempty"`
	Events      []*LogEvent            `json:"events"`
}

// The log event as specified in the API doc
type LogEvent struct {
	Timestamp   int64                  `json:"ts"`
	Severity    int8                   `json:"sev"`
	Attributes  map[string]interface{} `json:"attrs"`
	sessionInfo map[string]interface{}
}

func (clt *ClientHandler) writeToScalyr() {
	uploadData := &ScalyrUploadData{
		Token:   clt.server.config.ScalyrToken,
		Session: fmt.Sprint(uuid.NewV4()),
	}

	loop := true

	sessionInfo := map[string]interface{}{
		"conn_src": clt.Conn.RemoteAddr().String(),
		"conn_id":  clt.ID,
		"source":   "logstash2scalyr",
	}
	sessionInfoLastTransmission := time.Unix(0, 0)
	events := make([]*LogEvent, clt.server.config.ScalyrRequestMaxNbEvents)

	for loop {
		events := events[:0]

		// Every 3 minutes, we give some info about the current steam
		sessionInfoSend := time.Now().Sub(sessionInfoLastTransmission) > time.Second*30

		// We read all the events
		for len(events) == 0 || (len(clt.events) > 0 && len(events) < clt.maxNbEvents) {
			event := <-clt.events
			if event == nil {
				loop = false
				sessionInfoSend = true
			} else {
				// Adding all the event's sessionInfo to the stream session info
				if event.sessionInfo != nil {
					for k, v := range event.sessionInfo {
						if sessionInfo[k] != v {
							sessionInfo[k] = v
							sessionInfoSend = true
						}
					}
					event.sessionInfo = nil
				}
				events = append(events, event)
			}
		}

		// Always send sessionInfo for now
		sessionInfoSend = true

		// If we have some session data to send
		if sessionInfoSend {
			sessionInfoSend = false
			sessionInfoLastTransmission = time.Now()
			uploadData.SessionInfo = sessionInfo
		}

		uploadData.Events = events

		if err := clt.sendRequest(uploadData); err != nil {
			clt.log.Warnw(
				"Problem sending data",
				"err", err,
			)
		}
	}
}

func (clt *ClientHandler) sendRequest(uploadData *ScalyrUploadData) error {
	var rawJson []byte
	var err error
	for {
		rawJson, err = json.Marshal(uploadData)
		size := len(rawJson)
		if size > clt.server.config.ScalyrRequestMaxSize {
			if clt.maxNbEvents > 1 {
				clt.log.Debugw(
					"Query too big, reducing the number of events",
					"nbEvents", len(uploadData.Events),
					"maxNbEvents", clt.maxNbEvents,
					"size", size,
					"maxSize", clt.server.config.ScalyrRequestMaxSize,
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
					clt.server.config.ScalyrRequestMaxSize,
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
		time.Sleep(time.Duration(time.Millisecond.Nanoseconds() * int64(clt.server.config.ScalyrRequestMinPeriod)))

		var resp *http.Response
		resp, err = clt.httpClient.Post(clt.server.scalyrEndpoint, "application/json", bytes.NewBuffer(rawJson))

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
			if clt.maxNbEvents < clt.server.config.ScalyrRequestMaxNbEvents {
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
