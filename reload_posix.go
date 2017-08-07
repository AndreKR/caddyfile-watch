// +build !windows

package caddyfile_watch

import (
	"fmt"
	"os"
	"syscall"
)

func doReload() {

	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		fmt.Println("Can't find own process, this should never happen")
	}
	fmt.Println("Reloading Caddyfile")
	p.Signal(syscall.SIGUSR1)

}
