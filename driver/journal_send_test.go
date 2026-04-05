package driver

import (
	"bytes"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestJournalAppendVar(t *testing.T) {
	t.Run("single-line", func(t *testing.T) {
		var buf bytes.Buffer
		journalAppendVar(&buf, "MESSAGE", "hello world")
		if got := buf.String(); got != "MESSAGE=hello world\n" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("multi-line", func(t *testing.T) {
		var buf bytes.Buffer
		journalAppendVar(&buf, "MESSAGE", "line1\nline2")
		b := buf.Bytes()

		// Expect: "MESSAGE\n" + uint64 LE length + "line1\nline2\n"
		value := "line1\nline2"
		header := "MESSAGE\n"
		if !bytes.HasPrefix(b, []byte(header)) {
			t.Fatalf("missing name header, got %q", b)
		}
		b = b[len(header):]

		var length uint64
		if err := binary.Read(bytes.NewReader(b[:8]), binary.LittleEndian, &length); err != nil {
			t.Fatal(err)
		}
		if int(length) != len(value) {
			t.Errorf("length field: got %d, want %d", length, len(value))
		}
		b = b[8:]

		if !bytes.HasPrefix(b, []byte(value+"\n")) {
			t.Errorf("value mismatch, got %q", b)
		}
	})
}

func TestJournalIsTooBig(t *testing.T) {
	make_opErr := func(errno syscall.Errno) error {
		return &net.OpError{Err: &os.SyscallError{Err: errno}}
	}

	if !journalIsTooBig(make_opErr(syscall.EMSGSIZE)) {
		t.Error("EMSGSIZE should be too big")
	}
	if !journalIsTooBig(make_opErr(syscall.ENOBUFS)) {
		t.Error("ENOBUFS should be too big")
	}
	if journalIsTooBig(make_opErr(syscall.ECONNREFUSED)) {
		t.Error("ECONNREFUSED should not be too big")
	}
	if journalIsTooBig(nil) {
		t.Error("nil should not be too big")
	}
}

func TestJournalSendTo(t *testing.T) {
	// Create a receiving socket at a temp path.
	sockPath := filepath.Join(t.TempDir(), "journal.sock")
	server, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: sockPath, Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	// Create a sender socket (named, for portability across OS).
	senderPath := filepath.Join(t.TempDir(), "sender.sock")
	sender, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: senderPath, Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()

	addr := &net.UnixAddr{Name: sockPath, Net: "unixgram"}
	vars := map[string]string{"CONTAINER_NAME": "myapp"}
	err = journalSendTo(sender, addr, "test message", PriInfo, vars)
	if err != nil {
		t.Fatalf("journalSendTo: %v", err)
	}

	buf := make([]byte, 4096)
	n, err := server.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := string(buf[:n])

	for _, want := range []string{
		"PRIORITY=6\n",
		"MESSAGE=test message\n",
		"CONTAINER_NAME=myapp\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in datagram:\n%s", want, got)
		}
	}
}
