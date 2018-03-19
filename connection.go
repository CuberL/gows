package gows

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"net/textproto"
	"strings"
)

const ConnUUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

type Conn struct {
	conn net.Conn
}

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

func (c *Conn) connHandler(callback acceptCallback) {
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

func (c *Conn) Read() []byte {
	headerBytes := make([]byte, 2)
	l, err := c.conn.Read(headerBytes)
	if l <= 0 || err != nil {
		fmt.Println(err)
		fmt.Println("err found")
		return headerBytes
	}
	//	fin := c.readFin(headerBytes[0])
	//	opcode := c.readOpcode(headerBytes[0])
	isMask := c.readIsMask(headerBytes[1])
	packLen := c.readLen(headerBytes[1])
	if isMask {
		mask := c.readMask()
		maskedData := make([]byte, packLen)
		c.conn.Read(maskedData)
		realData := c.xorData(mask, maskedData)
		return realData
	} else {
		data := make([]byte, packLen)
		c.conn.Read(data)
		return data
	}
}

// implement the io.Writer interface
func (c *Conn) Write(b []byte) (n int, err error) {
	length := len(b)
	var pack []byte
	if length <= 125 {
		pack = make([]byte, 2+length)
		pack[0] = byte(129)
		pack[1] = byte(length)
		copy(pack[2:], b[:])
	} else if length > 125 && length <= 0xffff {
		pack = make([]byte, 4+length)
		pack[0] = byte(129)
		pack[1] = byte(126)
		binary.BigEndian.PutUint16(pack[2:4], uint16(length))
		copy(pack[4:], b[:])
	} else {
		pack = make([]byte, 10+length)
		pack[0] = byte(129)
		pack[1] = byte(127)
		binary.BigEndian.PutUint64(pack[2:10], uint64(length))
		copy(pack[10:], b[:])
	}
	return c.conn.Write(pack)
}
