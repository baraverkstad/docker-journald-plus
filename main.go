package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"syscall"

	"github.com/baraverkstad/docker-journald-plus/driver"
)

const socketPath = "/run/docker/plugins/journald-plus.sock"

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1.1+json")
		fmt.Fprintln(w, `{"Implements": ["LogDriver"]}`)
	})

	d := driver.New()
	d.RegisterHandlers(mux)

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
	if err := http.Serve(l, mux); err != nil {
		fmt.Fprintf(os.Stderr, "journald-plus: %v\n", err)
		os.Exit(1)
	}
}
