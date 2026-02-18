package realtime

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"
)

type testWSClient struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

func dialTestWS(t *testing.T, serverURL, path string) *testWSClient {
	t.Helper()

	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	conn, err := net.Dial("tcp", parsedURL.Host)
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}

	client := &testWSClient{
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
	}

	request := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n\r\n", path, parsedURL.Host)
	if _, err := client.writer.WriteString(request); err != nil {
		_ = conn.Close()
		t.Fatalf("write websocket handshake: %v", err)
	}
	if err := client.writer.Flush(); err != nil {
		_ = conn.Close()
		t.Fatalf("flush websocket handshake: %v", err)
	}

	statusLine, err := client.reader.ReadString('\n')
	if err != nil {
		_ = conn.Close()
		t.Fatalf("read websocket status: %v", err)
	}
	if !strings.Contains(statusLine, "101") {
		_ = conn.Close()
		t.Fatalf("websocket upgrade status = %q, want 101", strings.TrimSpace(statusLine))
	}

	for {
		line, err := client.reader.ReadString('\n')
		if err != nil {
			_ = conn.Close()
			t.Fatalf("read websocket header: %v", err)
		}
		if line == "\r\n" {
			break
		}
	}

	return client
}

func (c *testWSClient) ReadText(timeout time.Duration) ([]byte, error) {
	if err := c.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	for {
		opcode, payload, err := readFrame(c.reader)
		if err != nil {
			return nil, err
		}

		switch opcode {
		case opText:
			return payload, nil
		case opPing:
			if err := writeFrame(c.writer, opPong, payload, true); err != nil {
				return nil, err
			}
			if err := c.writer.Flush(); err != nil {
				return nil, err
			}
		case opClose:
			return nil, errWSClosed
		}
	}
}

func (c *testWSClient) Close() error {
	_ = writeFrame(c.writer, opClose, nil, true)
	_ = c.writer.Flush()
	return c.conn.Close()
}
