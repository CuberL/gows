# gows

a simple implement of websocket written in go

## Install

```
go get gitee.com/cuberl/gows
```

## Usage

```go
import (
	"fmt"
	"io"

	"gitee.com/cuberl/gows"
)

func main() {
	gows.New("localhost", 8091, func(conn *gows.Conn) {
		conn.Ping(30, func(conn *gows.Conn) {
			fmt.Println("ping timeout.")
		})
		for {
			data, err := conn.Read()
			if err != nil {
				if err == io.EOF {
					fmt.Println("Connection Close.")
				} else {
					fmt.Println(err)
				}
				break
			}
			fmt.Fprintf(conn, "hello, %s\n", string(data))
		}
	}).Start()
}
```