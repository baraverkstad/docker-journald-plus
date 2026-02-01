package driver

import (
	"fmt"
	"os"

	"github.com/coreos/go-systemd/v22/journal"
)

// defaultJournalSend writes a message to systemd journald via the native socket.
func defaultJournalSend(message string, priority Priority, vars map[string]string) error {
	return journal.Send(message, journal.Priority(priority), vars)
}

func init() {
	if !journal.Enabled() {
		fmt.Fprintf(os.Stderr, "journald-plus: warning: systemd journal does not appear to be available\n")
	}
}
