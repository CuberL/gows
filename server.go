package gows

import (
	"fmt"
	"net"
)

type acceptCallback func(conn *Conn)

type Server struct {
	Host     string
	Port     int
	server   net.Listener
	callback acceptCallback
}

func New(Host string, Port int, callback acceptCallback) *Server {
	return &Server{
		Host:     Host,
		Port:     Port,
		callback: callback,
	}
}

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
