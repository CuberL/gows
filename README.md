# gows

a simple implement of websocket written in go

## Usage

```go
package main

import (
	"fmt"
	"gows"
)

func main() {
	gows.New("localhost", uint32(8091), func(conn *gows.Conn) {
		for {
			data := conn.Read()
			fmt.Fprintf(conn, "hello, %s\n", string(data))
		}
	}).Start()
}
```