package main

import (
	"fmt"
	"net"
	"os"
	"syscall"

	"github.com/baraverkstad/docker-journald-plus/driver"
)

const socketPath = "/run/docker/plugins/journald-plus.sock"

func main() {
	d := driver.New()
	routes := d.Routes()
	routes["/Plugin.Activate"] = func(_ []byte) []byte {
		return []byte(`{"Implements":["LogDriver"]}`)
	}

	if err := os.MkdirAll("/run/docker/plugins", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "journald-plus: %v\n", err)
		os.Exit(1)
	}
	syscall.Unlink(socketPath) // ignore error — may not exist yet

	l, err := net.Listen("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "journald-plus: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "journald-plus: starting plugin server\n")
	if err := driver.Serve(l, routes); err != nil {
		fmt.Fprintf(os.Stderr, "journald-plus: %v\n", err)
		os.Exit(1)
	}
}
