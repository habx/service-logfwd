package main

import (
	"fmt"
	"net"
	"net/http"
)

type Server struct {
	config     Config
	exit       chan int
	httpClient http.Client
}

func NewServer() *Server {
	return &Server{
		config: NewConfig(),
		exit:   make(chan int),
	}
}

func (srv *Server) listen() (net.Listener, error) {
	listener, err := net.Listen("tcp", srv.config.ListenAddr)
	if err != nil {
		log.Fatalw("Couldn't listen", "addr", srv.config.ListenAddr, "err", err)
		return nil, fmt.Errorf("couldn't listen on %s", srv.config.ListenAddr)
	}

	log.Infow("Listening for TCP connections", "addr", srv.config.ListenAddr)

	go srv.acceptConnections(listener)

	return listener, nil
}

func (srv *Server) acceptConnections(listener net.Listener) {
	clientNb := 0
	for {
		// Listen for an incoming connection.
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalw("Couldn't accept connection", "err", err)
			return
		}
		// Handle connections in a new goroutine.
		clientNb += 1
		go NewClientHandler(srv, conn, clientNb).run()
	}
}
