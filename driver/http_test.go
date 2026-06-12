package driver

import (
	"bufio"
	"io"
	"net"
	"strings"
	"testing"
)

// httpClient wraps a net.Conn with a bufio.Reader for proper HTTP response parsing.
type httpClient struct {
	conn net.Conn
	br   *bufio.Reader
}

func newHTTPClient(routes map[string]func([]byte) []byte) *httpClient {
	client, server := net.Pipe()
	go serveConn(server, routes)
	return &httpClient{conn: client, br: bufio.NewReader(client)}
}

func (c *httpClient) do(method, path, body string) string {
	req := method + " " + path + " HTTP/1.1\r\nContent-Length: " + itoa(len(body)) + "\r\n\r\n" + body
	if _, err := io.WriteString(c.conn, req); err != nil {
		return ""
	}
	// Read status line
	c.br.ReadString('\n')
	// Read headers, extract Content-Length
	var bodyLen int
	for {
		h, _ := c.br.ReadString('\n')
		if h == "\r\n" {
			break
		}
		if n, ok := parseContentLength(h); ok {
			bodyLen = n
		}
	}
	// Read body
	out := make([]byte, bodyLen)
	io.ReadFull(c.br, out)
	return string(out)
}

func (c *httpClient) close() { c.conn.Close() }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	b := make([]byte, 0, 10)
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestServeConnDispatch(t *testing.T) {
	c := newHTTPClient(map[string]func([]byte) []byte{
		"/foo": func([]byte) []byte { return []byte(`{"Err":""}`) },
	})
	defer c.close()

	if got := c.do("POST", "/foo", ""); got != `{"Err":""}` {
		t.Errorf("got %q", got)
	}
}

func TestServeConnUnknownPath(t *testing.T) {
	c := newHTTPClient(map[string]func([]byte) []byte{})
	defer c.close()

	if got := c.do("POST", "/NoSuchMethod", ""); got != `{"Err":"unknown method"}` {
		t.Errorf("got %q", got)
	}
}

func TestServeConnBodyPassthrough(t *testing.T) {
	var received []byte
	c := newHTTPClient(map[string]func([]byte) []byte{
		"/echo": func(b []byte) []byte { received = b; return []byte(`{"Err":""}`) },
	})
	defer c.close()

	c.do("POST", "/echo", `{"File":"test"}`)
	if string(received) != `{"File":"test"}` {
		t.Errorf("body not passed through: got %q", received)
	}
}

func TestServeConnKeepAlive(t *testing.T) {
	calls := 0
	c := newHTTPClient(map[string]func([]byte) []byte{
		"/count": func([]byte) []byte { calls++; return []byte(`{"Err":""}`) },
	})
	defer c.close()

	for range 3 {
		c.do("POST", "/count", "")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestParseRequestPath(t *testing.T) {
	cases := []struct{ line, want string }{
		{"POST /Plugin.Activate HTTP/1.1\r\n", "/Plugin.Activate"},
		{"POST /LogDriver.StartLogging HTTP/1.1\r\n", "/LogDriver.StartLogging"},
		{"GET / HTTP/1.1\r\n", "/"},
		{"BADLINE\r\n", ""},
	}
	for _, c := range cases {
		if got := parseRequestPath(c.line); got != c.want {
			t.Errorf("parseRequestPath(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

func TestParseContentLength(t *testing.T) {
	cases := []struct {
		header string
		n      int
		ok     bool
	}{
		{"Content-Length: 42\r\n", 42, true},
		{"content-length: 0\r\n", 0, true},
		{"CONTENT-LENGTH: 123\r\n", 123, true},
		{"Content-Type: application/json\r\n", 0, false},
		{"Content-Length: abc\r\n", 0, false},
		{"Content-Length: -1\r\n", -1, true},
		{"Content-Length: -42\r\n", -42, true},
		{"\r\n", 0, false},
	}
	for _, c := range cases {
		n, ok := parseContentLength(c.header)
		if ok != c.ok || n != c.n {
			t.Errorf("parseContentLength(%q) = (%d, %v), want (%d, %v)", c.header, n, ok, c.n, c.ok)
		}
	}
}

// Malformed Content-Length must not panic or allocate unbounded memory;
// the connection is closed without a response.
func TestServeConnMalformedContentLength(t *testing.T) {
	for _, cl := range []string{"-1", "4000000000"} {
		t.Run(cl, func(t *testing.T) {
			c := newHTTPClient(map[string]func([]byte) []byte{
				"/foo": func([]byte) []byte { return []byte(`{"Err":""}`) },
			})
			defer c.close()

			req := "POST /foo HTTP/1.1\r\nContent-Length: " + cl + "\r\n\r\n"
			if _, err := io.WriteString(c.conn, req); err != nil {
				t.Fatal(err)
			}
			if _, err := c.br.ReadString('\n'); err != io.EOF {
				t.Errorf("expected connection close (EOF), got err=%v", err)
			}
		})
	}
}

// Ensure the Activate and Capabilities routes return expected JSON.
func TestServeConnPluginRoutes(t *testing.T) {
	d := NewWithSendFunc(func(string, Priority, map[string]string) error { return nil })
	routes := d.Routes()
	routes["/Plugin.Activate"] = func(_ []byte) []byte {
		return []byte(`{"Implements":["LogDriver"]}`)
	}
	c := newHTTPClient(routes)
	defer c.close()

	activate := c.do("POST", "/Plugin.Activate", "")
	if !strings.Contains(activate, "LogDriver") {
		t.Errorf("activate: got %q", activate)
	}

	caps := c.do("POST", "/LogDriver.Capabilities", "")
	if !strings.Contains(caps, "ReadLogs") {
		t.Errorf("capabilities: got %q", caps)
	}
}
