package driver

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
)

const journalSocket = "/run/systemd/journal/socket"

var (
	journalConn atomic.Pointer[net.UnixConn]
	journalOnce sync.Once
)

// defaultJournalSend writes a message to systemd journald via the native socket.
func defaultJournalSend(message string, priority Priority, vars map[string]string) error {
	conn := journalGetConn()
	if conn == nil {
		return fmt.Errorf("could not initialize socket to journald")
	}
	return journalSendTo(conn, &net.UnixAddr{Name: journalSocket, Net: "unixgram"}, message, priority, vars)
}

// journalSendTo sends a log entry to addr. Separated from defaultJournalSend for testability.
func journalSendTo(conn *net.UnixConn, addr *net.UnixAddr, message string, priority Priority, vars map[string]string) error {
	var buf bytes.Buffer
	journalAppendVar(&buf, "PRIORITY", strconv.Itoa(int(priority)))
	journalAppendVar(&buf, "MESSAGE", message)
	for k, v := range vars {
		journalAppendVar(&buf, k, v)
	}

	_, _, err := conn.WriteMsgUnix(buf.Bytes(), nil, addr)
	if err == nil {
		return nil
	}
	if !journalIsTooBig(err) {
		return err
	}

	// Datagram too large: write to tempfile and send fd as ancillary data.
	f, err := os.CreateTemp("/dev/shm", "journal.*")
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Unlink(f.Name()); err != nil {
		return err
	}
	if _, err := io.Copy(f, &buf); err != nil {
		return err
	}
	rights := syscall.UnixRights(int(f.Fd()))
	_, _, err = conn.WriteMsgUnix([]byte{}, rights, addr)
	return err
}

func init() {
	if journalGetConn() == nil {
		fmt.Fprintf(os.Stderr, "journald-plus: warning: systemd journal does not appear to be available\n")
		return
	}
	if conn, err := net.Dial("unixgram", journalSocket); err != nil {
		fmt.Fprintf(os.Stderr, "journald-plus: warning: systemd journal does not appear to be available\n")
	} else {
		conn.Close()
	}
}

func journalGetConn() *net.UnixConn {
	if c := journalConn.Load(); c != nil {
		return c
	}
	journalOnce.Do(func() {
		sock, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Net: "unixgram"})
		if err != nil {
			return
		}
		journalConn.Store(sock)
	})
	return journalConn.Load()
}

// journalAppendVar formats a field using the journald native protocol.
// Multi-line values use the binary-length-prefixed encoding.
func journalAppendVar(w io.Writer, name, value string) {
	if strings.ContainsRune(value, '\n') {
		fmt.Fprintln(w, name)
		binary.Write(w, binary.LittleEndian, uint64(len(value)))
		fmt.Fprintln(w, value)
	} else {
		fmt.Fprintf(w, "%s=%s\n", name, value)
	}
}

func journalIsTooBig(err error) bool {
	opErr, ok := err.(*net.OpError)
	if !ok {
		return false
	}
	sysErr, ok := opErr.Err.(*os.SyscallError)
	if !ok {
		return false
	}
	return sysErr.Err == syscall.EMSGSIZE || sysErr.Err == syscall.ENOBUFS
}
