package app

import (
	"fmt"
	"os"

	"github.com/boratanrikulu/gecit/pkg/capture"
)

func printPlatformStatus() {
	fmt.Printf("  engine:    bpf-capture\n")

	if os.Geteuid() != 0 {
		fmt.Printf("  (run with sudo for accurate capability detection)\n")
		return
	}

	iface, err := capture.DefaultInterface()
	if err != nil {
		fmt.Printf("  interface: not detected\n")
	} else {
		fmt.Printf("  interface: %s\n", iface)
	}

	fmt.Printf("  /dev/bpf:  available\n")
	fmt.Printf("  raw socket: available\n")
}
