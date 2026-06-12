package driver

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

// Serve accepts connections on l and dispatches requests to routes.
// routes maps URL path to a handler receiving the request body and
// returning a JSON response body. Runs until l is closed.
func Serve(l net.Listener, routes map[string]func([]byte) []byte) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		go serveConn(conn, routes)
	}
}

// maxRequestBody caps request body allocation; Docker's log driver
// requests are tiny, so anything larger indicates a malformed request.
const maxRequestBody = 1 << 20

// serveConn handles the HTTP/1.1 keep-alive loop for a single connection.
func serveConn(conn net.Conn, routes map[string]func([]byte) []byte) {
	defer conn.Close()
	br := bufio.NewReader(conn)
	for {
		// Request line: "POST /path HTTP/1.1\r\n"
		line, err := br.ReadString('\n')
		if err != nil {
			return
		}

		// Headers: read until blank line, capture Content-Length
		var bodyLen int
		for {
			h, err := br.ReadString('\n')
			if err != nil || h == "\r\n" {
				break
			}
			if n, ok := parseContentLength(h); ok {
				bodyLen = n
			}
		}

		// Body
		if bodyLen < 0 || bodyLen > maxRequestBody {
			return
		}
		body := make([]byte, bodyLen)
		if _, err := io.ReadFull(br, body); err != nil {
			return
		}

		// Dispatch
		var out []byte
		if fn, ok := routes[parseRequestPath(line)]; ok {
			out = fn(body)
		} else {
			out = []byte(`{"Err":"unknown method"}`)
		}

		// Response — Docker only decodes the JSON body, headers are ignored.
		// Single write keeps headers and body together on the wire.
		resp := fmt.Appendf(nil, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\n\r\n", len(out))
		if _, err := conn.Write(append(resp, out...)); err != nil {
			return
		}
	}
}

// parseRequestPath extracts the URL path from an HTTP request line.
func parseRequestPath(line string) string {
	// "POST /path HTTP/1.1\r\n"
	i := strings.IndexByte(line, ' ')
	if i < 0 {
		return ""
	}
	line = line[i+1:]
	before, _, ok := strings.Cut(line, " ")
	if !ok {
		return strings.TrimRight(line, "\r\n")
	}
	return before
}

// parseContentLength parses a "Content-Length: N" header line (case-insensitive).
func parseContentLength(header string) (int, bool) {
	const prefix = "content-length:"
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(header[len(prefix):]))
	if err != nil {
		return 0, false
	}
	return n, true
}
