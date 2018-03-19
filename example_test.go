package gows

import (
	"fmt"
	"io"

	"github.com/cuberl/gows"
)

func Example() {
	gows.New("localhost", 8091, func(conn *gows.Conn) {
		// ping the client per 30 secs.
		conn.Ping(30, func(conn *gows.Conn) {
			fmt.Println("ping timeout.")
		})
		for {
			data, err := conn.Read()
			if err != nil {
				if err == io.EOF {
					fmt.Println("Connection Close.")
				}
				break
			}
			fmt.Fprintf(conn, "hello, %s\n", string(data))
		}
	}).Start()
}
