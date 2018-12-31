package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/habx/service-logfwd/clients"
	"github.com/habx/service-logfwd/clients/list"
	"go.uber.org/zap"
)

// ClientHandler is structure instantiate for each new (logstash) incoming client
type ClientHandler struct {
	server        *Server
	Conn          net.Conn
	id            int
	totalNbEvents int
	arrivalTime   time.Time
	log           *zap.SugaredLogger
	outputs       []clients.OutputClient
}

// NewClientHandler instantiates a new client handler
func (srv *Server) NewClientHandler(conn net.Conn, nb int) *ClientHandler {
	clt := &ClientHandler{
		server:      srv,
		Conn:        conn,
		id:          nb,
		arrivalTime: time.Now(),
		log:         srv.log.With("clientID", nb),
	}

	clt.createClients()

	return clt
}

func (clt *ClientHandler) createClients() {
	clt.outputs = make([]clients.OutputClient, 0, len(list.LIST))
	for _, def := range list.LIST {
		conf := clt.server.config.OutputClientConfigs[def.Name()]
		if conf.Enabled() {
			clt.outputs = append(clt.outputs, def.Create(clt, conf))
		}
	}
}

func (clt *ClientHandler) ID() int {
	return clt.id
}

func (clt *ClientHandler) Addr() net.Addr {
	return clt.Conn.RemoteAddr()
}

func (clt *ClientHandler) Logger() *zap.SugaredLogger {
	return clt.log
}

func (clt *ClientHandler) send(event *clients.LogEvent) {
	for _, out := range clt.outputs {
		out.Send(event)
	}
}

func (clt *ClientHandler) end() {
	departure := time.Now()

	// We only log disconnection if at least one event was parsed
	if clt.totalNbEvents > 0 {
		clt.send(&clients.LogEvent{
			Severity:  clients.LvlDebug,
			Timestamp: departure,
			Attributes: map[string]interface{}{
				"message":       "Client disconnected",
				"action":        "client_disconnected",
				"duration":      departure.Sub(clt.arrivalTime) / time.Second,
				"totalNbEvents": clt.totalNbEvents,
			},
		})
	}

	if err := clt.Conn.Close(); err != nil {
		clt.log.Warnw("Issue closing connection", "err", err)
	}

	for _, out := range clt.outputs {
		if err := out.Close(); err != nil {
			clt.log.Errorf(
				"Issue closing output client",
				"clientName", out.Name(),
				"err", err,
			)
		}
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
	defer clt.end()

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

// https://www.scalyr.com/help/parsing-logs#specialAttrs
// nolint
var severityConversions = map[string]clients.Level{
	"finest":    clients.LvlFinest,
	"finer":     clients.LvlTrace,
	"trace":     clients.LvlTrace,
	"fine":      clients.LvlDebug,
	"debug":     clients.LvlDebug,
	"info":      clients.LvlInfo,
	"notice":    clients.LvlInfo,
	"warn":      clients.LvlWarning,
	"warning":   clients.LvlWarning,
	"error":     clients.LvlError,
	"fatal":     clients.LvlCritical,
	"emerg":     clients.LvlCritical,
	"emergency": clients.LvlCritical,
	"crit":      clients.LvlCritical,
	"critical":  clients.LvlCritical,
	"panic":     clients.LvlCritical,
	"alert":     clients.LvlCritical,
	//"i":         LvlInfo,
	//"w":         LvlWarning,
	//"err":       LvlError,
	//"e":         LvlError,
	//"f":         LvlCritical,
}

func (clt *ClientHandler) ParseLogstashLine(line string) error {
	var lineJSON map[string]interface{}
	authenticated := false

	clt.log.Debugw(
		"Received from logstash",
		"line", line,
	)

	if clt.server.config.LogstashAuthPrefixToken != "" {
		spl := strings.SplitN(line, " ", 2)
		if len(spl) != 2 {
			return errors.New("you need to have an auth prefix")
		}
		if spl[0] != clt.server.config.LogstashAuthPrefixToken {
			return errors.New("wrong auth prefix token authentication")
		}
		authenticated = true
		line = spl[1]
	}

	if err := json.Unmarshal([]byte(line), &lineJSON); err != nil {
		return err
	}

	// Checking authentication if required
	if !authenticated && clt.server.config.LogstashAuthKey != "" {
		value, ok := lineJSON[clt.server.config.LogstashAuthKey]
		if !ok || clt.server.config.LogstashAuthValue != value {
			return fmt.Errorf("wrong authentication with key %s", clt.server.config.LogstashAuthKey)
		}
		delete(lineJSON, clt.server.config.LogstashAuthKey)
	}

	event := &clients.LogEvent{
		Timestamp:  time.Now(),
		Attributes: lineJSON,
	}

	// Fetching the timestamp
	if strTs, ok := event.Attributes["@timestamp"].(string); ok {
		if timestamp, err := time.Parse(time.RFC3339Nano, strTs); err == nil {
			event.Timestamp = timestamp
			delete(event.Attributes, "@timestamp")
		}
	}

	// We copy the extra fields to the root (otherwise, scalyr will index them as big chunk of a JSON string)
	if fields, ok := event.Attributes["@fields"]; ok {
		if fields, ok := fields.(map[string]interface{}); ok {
			for k, v := range fields {
				if _, ok := event.Attributes[k]; !ok {
					event.Attributes[k] = v
				}
			}
		}
		delete(event.Attributes, "@fields")
	}

	if value, ok := event.Attributes["@message"]; ok {
		event.Attributes["message"] = value
		delete(event.Attributes, "@message")
	}

	{ // Fetching the loglevel
		var levelName string
		var level clients.Level
		var ok bool
		for _, keyName := range []string{"levelName", "levelname"} {
			if levelName, ok = event.Attributes[keyName].(string); ok {
				delete(event.Attributes, keyName)
			}
		}
		if ok {
			if level, ok = severityConversions[strings.ToLower(levelName)]; ok {
				event.Severity = level
			}
		}
		if !ok {
			event.Severity = clients.LvlInfo
		}
	}

	clt.totalNbEvents++
	clt.send(event)

	return nil
}
