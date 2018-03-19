# gows

a simple implement of websocket written in go

## Install

```shell
go get gitee.com/cuberl/gows
```

## Usage

```go
package main

import (
	"fmt"
	"gitee.com/cuberl/gows"
)

func main() {
	gows.New("localhost", 8091, func(conn *gows.Conn) {
		for {
			data := conn.Read()
			fmt.Fprintf(conn, "hello, %s\n", string(data))
		}
	}).Start()
}
```