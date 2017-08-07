package caddyfile_watch

import (
	"fmt"
)

func doReload() {
	fmt.Println("Can't reload Caddy on Windows")
}
