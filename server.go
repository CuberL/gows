package gows

import (
	"fmt"
	"net"
)

type AcceptCallback func(conn *Conn)

type Server struct {
	Host     string
	Port     int
	server   net.Listener
	callback AcceptCallback
}

// It will listen the addr (Host):(Port).
// the callback will be call when a connection is accepted.
func New(Host string, Port int, callback AcceptCallback) *Server {
	return &Server{
		Host:     Host,
		Port:     Port,
		callback: callback,
	}
}

// Start to listening the port you specified
func (s *Server) Start() {
	var err error
	s.server, err = net.Listen("tcp", fmt.Sprintf("%s:%d", s.Host, s.Port))
	if err != nil {
		fmt.Println(err)
	}
	for {
		conn, err := s.server.Accept()
		if err != nil {
			fmt.Println(err)
		}
		newconn := &Conn{
			conn: conn,
		}
		go newconn.connHandler(s.callback)
	}
}
