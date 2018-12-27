package main

import (
	"fmt"
	"net"

	"go.uber.org/zap"
)

type Server struct {
	config *Config
	exit   chan int
	log    *zap.SugaredLogger
}

func NewServer(config *Config, log *zap.SugaredLogger) *Server {
	return &Server{
		config: config,
		exit:   make(chan int),
		log:    log,
	}
}

func (srv *Server) listen() (net.Listener, error) {
	listener, err := net.Listen("tcp", srv.config.ListenAddr)
	if err != nil {
		srv.log.Fatalw("Couldn't listen", "addr", srv.config.ListenAddr, "err", err)
		return nil, fmt.Errorf("couldn't listen on %s", srv.config.ListenAddr)
	}

	srv.log.Infow("Listening for TCP connections", "addr", srv.config.ListenAddr)

	go srv.acceptConnections(listener)

	return listener, nil
}

func (srv *Server) acceptConnections(listener net.Listener) {
	clientNb := 0
	for {
		// Listen for an incoming connection.
		conn, err := listener.Accept()
		if err != nil {
			srv.log.Fatalw("Couldn't accept connection", "err", err)
			return
		}
		// Handle connections in a new goroutine.
		clientNb++
		go srv.NewClientHandler(conn, clientNb).run()
	}
}
