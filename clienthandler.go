package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/satori/go.uuid"
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
}

func NewClientHandler(server *Server, conn net.Conn, nb int) *ClientHandler {
	return &ClientHandler{
		server:      server,
		Conn:        conn,
		ID:          nb,
		events:      make(chan *LogEvent, server.config.QueueSize),
		arrivalTime: time.Now(),
		sessionInfo: make(map[string]interface{}),
		maxNbEvents: server.config.ScalyrMaxNbEvents,
	}
}

func (clt *ClientHandler) run() {
	log.Infow("Client connected", "clientId", clt.ID)

	// It would have been interesting to send a first message explaining that this client just connected
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

		// Event closing the scaly HTTP sender
		clt.events <- nil

		if err := clt.Conn.Close(); err != nil {
			log.Warnw("Issue closing connection", "err", err, "clientId", clt.ID)
		}
	}()
	reader := bufio.NewReaderSize(clt.Conn, clt.server.config.LogstashMaxEventSize)
	for {
		lineRaw, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				log.Infow("Client disconnected", "clientId", clt.ID)
			} else {
				log.Errorw("Couldn't read from client", "clientId", clt.ID, "err", err)
			}
			return
		}
		if err := clt.ParseLogstashLine(lineRaw); err != nil {
			log.Errorw("Couldn't parse line from client", "clientId", clt.ID, "err", err)
			return
		}
	}
}

// These are the attribute keys to convert within a message
var KeysToMessageConversions = map[string]string{
	"@source_host": "hostname",
	"@source_path": "file_path",
	"@message":     "message",
	"@type":        "logstash_type",
	"@source":      "logstash_source",
	"@tags":        "tags",
}

// These are the attribute keys to convert and move to the session
var keysToSessionInfoConversions = map[string]string{
	"appname": "serverHost",
	"env":     "logfile",
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

	log.Debugw(
		"Received from logstash",
		"clientId", clt.ID,
		"line", line,
	)

	if err := json.Unmarshal([]byte(line), &lineJson); err != nil {
		return err
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
		if targetKey, ok := KeysToMessageConversions[initialKey]; ok {
			event.Attributes[targetKey] = value
			delete(event.Attributes, initialKey)
		}
	}

	// Converting some keys to other keys in the session
	for initialKey, value := range event.Attributes {
		if targetKey, ok := keysToSessionInfoConversions[initialKey]; ok {
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

type LogEvent struct {
	Timestamp   int64                  `json:"ts"`
	Severity    int8                   `json:"sev"`
	Attributes  map[string]interface{} `json:"attrs"`
	sessionInfo map[string]interface{}
}

type ScalyrUploadData struct {
	Token       string                 `json:"token"`
	Session     string                 `json:"session"`
	SessionInfo map[string]interface{} `json:"sessionInfo,omitempty"`
	Threads     map[string]interface{} `json:"threads,omitempty"`
	Events      []*LogEvent            `json:"events"`
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
	events := make([]*LogEvent, clt.server.config.ScalyrMaxNbEvents)

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
			log.Warnw(
				"Problem sending data",
				"clientId", clt.ID,
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
		if size > clt.server.config.ScalyrMaxRequestSize {
			if clt.maxNbEvents > 1 {
				log.Debugw(
					"Query too big, reducing the number of events",
					"nbEvents", len(uploadData.Events),
					"maxNbEvents", clt.maxNbEvents,
					"size", size,
					"maxSize", clt.server.config.ScalyrMaxRequestSize,
					"clientId", clt.ID,
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
					clt.server.config.ScalyrMaxRequestSize,
				)
			}
		}
		break
	}

	if err != nil {
		log.Errorw(
			"Problem generating JSON",
			"clientId", clt.ID,
			"err", err,
		)
		return err
	}

	log.Debugw(
		"Scalyr HTTP Request",
		"nbSentEvents", len(uploadData.Events),
		"nbWaitingEvents", len(clt.events),
		"clientId", clt.ID,
		"data", string(rawJson),
	)

	backoffTime := time.Duration(0)
	backoffIncrement := time.Second
	for backoffIncrement < time.Minute {
		time.Sleep(time.Duration(time.Millisecond.Nanoseconds() * int64(clt.server.config.ScalyrMinTimeBetweenQueries)))

		var resp *http.Response
		resp, err = clt.httpClient.Post(clt.server.config.scalyrEndpoint, "application/json", bytes.NewBuffer(rawJson))

		if err != nil {
			log.Warnw(
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
			if clt.maxNbEvents < clt.server.config.ScalyrMaxNbEvents {
				clt.maxNbEvents += 1
			}
		}

		{
			var body []byte
			if body, err = ioutil.ReadAll(resp.Body); err != nil {
				log.Warnw(
					"Issue reading response",
					"err", err,
				)
				continue
			} else {
				log.Debugw("Scalyr HTTP Response",
					"statusCode", resp.StatusCode,
					"statusMsg", resp.Status,
					"body", string(body),
					"clientId", clt.ID,
				)
			}
		}

		if err = resp.Body.Close(); err != nil {
			log.Errorw(
				"Problem closing HTTP response body",
				"clientId", clt.ID,
				"err", err,
			)
			continue
		}

		return nil
	}
	return fmt.Errorf("couldn't send our data")
}
