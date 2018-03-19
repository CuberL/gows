package gows

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"time"
)

const ConnUUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

type Conn struct {
	Addr       net.Addr
	conn       net.Conn
	readMutex  sync.Mutex
	cancelPing func()
	pong       chan (bool)
	buf        chan ([]byte)
}

const (
	OpcodeExtraData       = 0x00
	OpcodeTextData        = 0x01
	OpcodeBinaryData      = 0x02
	OpcodeConnectionClose = 0x08
	OpcodePing            = 0x09
	OpcodePong            = 0x0A
)

func (c *Conn) getHeaders() (map[string]string, error) {
	reader := textproto.NewReader(bufio.NewReader(c.conn))
	result := map[string]string{}
	// drop first line
	reader.ReadLine()
	for {
		line, err := reader.ReadLine()
		if err != nil {
			return result, err
		}
		lineSplit := strings.Split(line, ":")
		if len(lineSplit) >= 2 {
			key := lineSplit[0]
			val := strings.Join(lineSplit[1:], ":")
			result[key] = strings.TrimSpace(val)
		} else {
			return result, nil
		}
	}
}

func (c *Conn) calAcceptKey(key string) string {
	sha1Caler := sha1.New()
	sha1Caler.Write([]byte(key + ConnUUID))
	_sha1 := sha1Caler.Sum(nil)
	return base64.StdEncoding.EncodeToString(_sha1)
}

func (c *Conn) sendResponse(key string) {
	c.conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\n"))
	c.conn.Write([]byte("Upgrade: websocket\r\n"))
	c.conn.Write([]byte("Connection: Upgrade\r\n"))
	c.conn.Write([]byte("Sec-WebSocket-Accept: " + key + "\r\n"))
	c.conn.Write([]byte("\r\n"))
}

func (c *Conn) connHandler(callback AcceptCallback) {
	c.Addr = c.conn.RemoteAddr()
	// get headers
	headers, err := c.getHeaders()
	if err != nil {
		fmt.Println(err)
		return
	}
	// get seckey
	secKey := headers["Sec-WebSocket-Key"]
	// cal accept key
	acceptKey := c.calAcceptKey(secKey)
	// send response msg
	c.sendResponse(acceptKey)
	c.buf = make(chan ([]byte), 1024)
	go c.readLoop()
	// call the callback
	callback(c)
}

func (c *Conn) readFin(b byte) bool {
	if uint8(b)&0x80 >= 0 {
		return true
	} else {
		return false
	}
}

func (c *Conn) readOpcode(b byte) uint8 {
	return uint8(b) & 0x0f
}

func (c *Conn) readIsMask(b byte) bool {
	if uint8(b)&0x80 >= 0 {
		return true
	} else {
		return false
	}
}

func (c *Conn) readMask() []byte {
	mask := make([]byte, 4)
	c.conn.Read(mask)
	return mask
}

func (c *Conn) readLen(b byte) uint64 {
	var length uint64
	length = uint64(uint8(b) & 0x7f)
	if length <= 125 {
		return length
	} else if length == 126 {
		lenBytes := make([]byte, 2)
		c.conn.Read(lenBytes)
		length = uint64(binary.BigEndian.Uint16(lenBytes))
	} else if length == 127 {
		lenBytes := make([]byte, 8)
		c.conn.Read(lenBytes)
		length = binary.BigEndian.Uint64(lenBytes)
	}
	return length
}

func (c *Conn) xorData(mask []byte, maskedData []byte) []byte {
	length := len(maskedData)
	intmask := binary.BigEndian.Uint32(mask)
	for i := 0; i < (length % 4); i++ {
		maskedData = append(maskedData, byte(0))
	}
	// xor
	for i := 0; i < length; i += 4 {
		intmaskedData := binary.BigEndian.Uint32(maskedData[i : i+4])
		binary.BigEndian.PutUint32(maskedData[i:i+4], intmask^intmaskedData)
	}
	return maskedData[:length]
}

func (c *Conn) readLoop() {
	for {
		headerBytes := make([]byte, 2)
		l, err := c.conn.Read(headerBytes)
		if l <= 0 || err != nil {
			c.buf <- make([]byte, 0)
			c.CancelPing()
			return
		}
		//	fin := c.readFin(headerBytes[0])
		opcode := c.readOpcode(headerBytes[0])
		if opcode == OpcodeConnectionClose {
			c.buf <- make([]byte, 0)
			c.CancelPing()
			return
		} else if opcode == OpcodePong {
			packLen := c.readLen(headerBytes[1])
			isMask := c.readIsMask(headerBytes[1])
			var dropData []byte
			if isMask {
				dropData = make([]byte, packLen+4)
			} else {
				dropData = make([]byte, packLen)
			}
			c.conn.Read(dropData)
			c.pong <- true
			continue
		}
		isMask := c.readIsMask(headerBytes[1])
		packLen := c.readLen(headerBytes[1])
		if isMask {
			mask := c.readMask()
			maskedData := make([]byte, packLen)
			c.conn.Read(maskedData)
			realData := c.xorData(mask, maskedData)
			fmt.Println(realData)
			c.buf <- realData
		} else {
			data := make([]byte, packLen)
			c.conn.Read(data)
			c.buf <- data
		}
	}
}

// Read a frame from client,
// It will return io.EOF when the connection is closed,
// It will be block if there is not data yet.
func (c *Conn) Read() ([]byte, error) {
	data := <-c.buf
	if len(data) == 0 {
		return data, io.EOF
	} else {
		return data, nil
	}
}

// Start to send keepalive data to client,
// It will ping client per (interval) seconds,
// the errCallback will be call when pong client didn't arrive in (interval) seconds,
// you can call CancelPing to cancel it.
func (c *Conn) Ping(interval int, errCallback func(*Conn)) {
	cancel := make(chan (bool))
	c.pong = make(chan (bool))
	c.cancelPing = func() {
		cancel <- true
	}
	go func(cancel chan (bool), c *Conn) {
		for {
			select {
			case <-time.After(time.Duration(interval) * time.Second):
				pack := c.makeSendData(OpcodePing, []byte(""))
				c.conn.Write(pack)
				// waiting for response
				select {
				case <-time.After(time.Duration(interval) * time.Second):
					errCallback(c)
					return
				case <-c.pong:
				}
			case <-cancel:
				return
			}
		}
	}(cancel, c)
}

// Cancel to send keepalive to client.
func (c *Conn) CancelPing() {
	if c.cancelPing != nil {
		c.cancelPing()
	}
}

func (c *Conn) makeSendData(opcode int, b []byte) []byte {
	length := len(b)
	var pack []byte
	if length <= 125 {
		pack = make([]byte, 2+length)
		pack[0] = byte(128 | opcode)
		pack[1] = byte(length)
		copy(pack[2:], b[:])
	} else if length > 125 && length <= 0xffff {
		pack = make([]byte, 4+length)
		pack[0] = byte(128 | opcode)
		pack[1] = byte(126)
		binary.BigEndian.PutUint16(pack[2:4], uint16(length))
		copy(pack[4:], b[:])
	} else {
		pack = make([]byte, 10+length)
		pack[0] = byte(128 | opcode)
		pack[1] = byte(127)
		binary.BigEndian.PutUint64(pack[2:10], uint64(length))
		copy(pack[10:], b[:])
	}
	return pack
}

// This function implements the interface io.Writer,
// So you can write data to websocket like a file
func (c *Conn) Write(b []byte) (n int, err error) {
	pack := c.makeSendData(OpcodeTextData, b)
	return c.conn.Write(pack)
}
