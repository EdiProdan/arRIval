package realtime

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

const (
	opText  = 0x1
	opClose = 0x8
	opPing  = 0x9
	opPong  = 0xA
)

var errWSClosed = errors.New("websocket closed")

type wsConn struct {
	conn        net.Conn
	reader      *bufio.Reader
	writer      *bufio.Writer
	writeMu     sync.Mutex
	pongHandler func()
}

func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (*wsConn, error) {
	if r.Method != http.MethodGet {
		return nil, fmt.Errorf("websocket upgrade requires GET")
	}

	if !headerContainsToken(r.Header, "Connection", "upgrade") || !headerContainsToken(r.Header, "Upgrade", "websocket") {
		return nil, fmt.Errorf("missing websocket upgrade headers")
	}

	secKey := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if secKey == "" {
		return nil, fmt.Errorf("missing Sec-WebSocket-Key")
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("response writer does not support hijacking")
	}

	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, fmt.Errorf("hijack connection: %w", err)
	}

	accept := websocketAcceptKey(secKey)
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n" +
		"\r\n"
	if _, err := rw.WriteString(response); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("write upgrade response: %w", err)
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("flush upgrade response: %w", err)
	}

	return &wsConn{
		conn:   conn,
		reader: rw.Reader,
		writer: rw.Writer,
	}, nil
}

func (c *wsConn) SetReadDeadline(deadline time.Time) error {
	return c.conn.SetReadDeadline(deadline)
}

func (c *wsConn) SetWriteDeadline(deadline time.Time) error {
	return c.conn.SetWriteDeadline(deadline)
}

func (c *wsConn) SetPongHandler(handler func()) {
	c.pongHandler = handler
}

func (c *wsConn) ReadMessage() (byte, []byte, error) {
	for {
		opcode, payload, err := readFrame(c.reader)
		if err != nil {
			return 0, nil, err
		}

		switch opcode {
		case opPing:
			if err := c.WritePong(payload); err != nil {
				return 0, nil, err
			}
			continue
		case opPong:
			if c.pongHandler != nil {
				c.pongHandler()
			}
			continue
		case opClose:
			return 0, nil, errWSClosed
		default:
			return opcode, payload, nil
		}
	}
}

func (c *wsConn) WriteText(payload []byte) error {
	return c.writeFrame(opText, payload)
}

func (c *wsConn) WritePing() error {
	return c.writeFrame(opPing, nil)
}

func (c *wsConn) WritePong(payload []byte) error {
	return c.writeFrame(opPong, payload)
}

func (c *wsConn) WriteClose() error {
	return c.writeFrame(opClose, nil)
}

func (c *wsConn) Close() error {
	return c.conn.Close()
}

func (c *wsConn) writeFrame(opcode byte, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if err := writeFrame(c.writer, opcode, payload, false); err != nil {
		return err
	}
	return c.writer.Flush()
}

func readFrame(reader *bufio.Reader) (byte, []byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return 0, nil, err
	}

	opcode := header[0] & 0x0F
	masked := header[1]&0x80 != 0
	payloadLen, err := readPayloadLength(reader, header[1]&0x7F)
	if err != nil {
		return 0, nil, err
	}

	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(reader, maskKey[:]); err != nil {
			return 0, nil, err
		}
	}

	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(reader, payload); err != nil {
			return 0, nil, err
		}
	}

	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	return opcode, payload, nil
}

func writeFrame(writer io.Writer, opcode byte, payload []byte, masked bool) error {
	first := byte(0x80) | (opcode & 0x0F)
	if _, err := writer.Write([]byte{first}); err != nil {
		return err
	}

	payloadLen := len(payload)
	maskBit := byte(0)
	if masked {
		maskBit = 0x80
	}

	switch {
	case payloadLen <= 125:
		if _, err := writer.Write([]byte{maskBit | byte(payloadLen)}); err != nil {
			return err
		}
	case payloadLen <= 65535:
		if _, err := writer.Write([]byte{maskBit | 126}); err != nil {
			return err
		}
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, uint16(payloadLen))
		if _, err := writer.Write(buf); err != nil {
			return err
		}
	default:
		if _, err := writer.Write([]byte{maskBit | 127}); err != nil {
			return err
		}
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(payloadLen))
		if _, err := writer.Write(buf); err != nil {
			return err
		}
	}

	if !masked {
		_, err := writer.Write(payload)
		return err
	}

	// Fixed mask for deterministic tests/client utility.
	maskKey := [4]byte{1, 2, 3, 4}
	if _, err := writer.Write(maskKey[:]); err != nil {
		return err
	}
	maskedPayload := make([]byte, len(payload))
	copy(maskedPayload, payload)
	for i := range maskedPayload {
		maskedPayload[i] ^= maskKey[i%4]
	}
	_, err := writer.Write(maskedPayload)
	return err
}

func readPayloadLength(reader io.Reader, base byte) (int, error) {
	switch base {
	case 126:
		buf := make([]byte, 2)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return 0, err
		}
		return int(binary.BigEndian.Uint16(buf)), nil
	case 127:
		buf := make([]byte, 8)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return 0, err
		}
		return int(binary.BigEndian.Uint64(buf)), nil
	default:
		return int(base), nil
	}
}

func websocketAcceptKey(secKey string) string {
	hash := sha1.Sum([]byte(secKey + wsGUID))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func headerContainsToken(header http.Header, key, token string) bool {
	value := strings.ToLower(header.Get(key))
	if value == "" {
		return false
	}
	token = strings.ToLower(token)
	for _, part := range strings.Split(value, ",") {
		if strings.TrimSpace(part) == token {
			return true
		}
	}
	return false
}
