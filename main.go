package main

import (
	"fmt"
	"os"

	"github.com/baraverkstad/docker-journald-plus/driver"
	"github.com/docker/go-plugins-helpers/sdk"
)

const socketName = "journald-plus.sock"

func main() {
	h := sdk.NewHandler(`{"Implements": ["LogDriver"]}`)
	d := driver.New()
	d.RegisterHandlers(h)

	fmt.Fprintf(os.Stderr, "journald-plus: starting plugin server\n")
	if err := h.ServeUnix(socketName, 0); err != nil {
		fmt.Fprintf(os.Stderr, "journald-plus: %v\n", err)
		os.Exit(1)
	}
}
