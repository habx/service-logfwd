package clients

import (
	"io"
	"net"
	"time"

	"go.uber.org/zap"
)

// Level defines the severity level of each log
type Level uint8

const (
	// LvlFinest is the finest logging level (useless)
	LvlFinest = Level(0)

	// LvlTrace is for log tracing (useless)
	LvlTrace = Level(1)

	// LvlDebug is for debuggging (anything below info)
	LvlDebug = Level(2)

	// LvlInfo is for informaiton logs
	LvlInfo = Level(3)

	// LvlWarning is for warning logs
	LvlWarning = Level(4)

	// LvlError is for error logs
	LvlError = Level(5)

	// LvlCritical is for critical logs (which should result in immediate stop)
	LvlCritical = Level(6)
)

// LogEvent is what was received and parsed as input
type LogEvent struct {
	Timestamp  time.Time              // Timestamp of the event
	Attributes map[string]interface{} // Attributes of the event
	Severity   Level                  // Severity of logging
}

// OutputClient is the interface an output client needs
type OutputClient interface {
	io.Closer
	Send(event *LogEvent)
	Name() string
}

// ClientHandler describes the inbound connection
type ClientHandler interface {
	ID() int                    // ID of the connection on the server side
	Addr() net.Addr             // Address of the remote connection
	Logger() *zap.SugaredLogger // Logger used for this client
}

// Config describes a generic minimal requirement for the output clients
type Config interface {
	Load() error   // Loads the config
	Enabled() bool // Defines if the output client should be enabled
}

// OutputClientDefinition defines the client in a modular architecture
type OutputClientDefinition interface {
	// Name
	Name() string

	// Config instanciation
	Config() Config

	// Factory method
	Create(ch ClientHandler, config Config) OutputClient
}
