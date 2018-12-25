package clients

import (
	"go.uber.org/zap"
	"io"
	"net"
	"time"
)

// Logging severity level
type Level uint8

const (
	LvlFinest   = Level(0)
	LvlTrace    = Level(1)
	LvlDebug    = Level(2)
	LvlInfo     = Level(3)
	LvlWarning  = Level(4)
	LvlError    = Level(5)
	LvlCritical = Level(6)
)

type LogEvent struct {
	Timestamp  time.Time              // Timestamp of the event
	Attributes map[string]interface{} // Attributes of the event
	Severity   Level                  // Severity of logging
}

type OutputClient interface {
	io.Closer
	Send(event *LogEvent)
	Name() string
}

type ClientHandler interface {
	ID() int
	Addr() net.Addr
	Logger() *zap.SugaredLogger
}

type Config interface {
	Load() error
	Enabled() bool
}

// Definition of the output client
type OutputClientDefinition interface {
	// Name
	Name() string

	// Config definition
	Config() Config

	// Factory method
	Create(ch ClientHandler, config Config) OutputClient
}
