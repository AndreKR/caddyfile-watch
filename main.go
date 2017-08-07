package caddyfile_watch

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/mholt/caddy"
	"path/filepath"
	"sync"
	"time"
)

var active bool
var watcher *fsnotify.Watcher
var importedFiles []struct {
	Dir  string
	Base string
}
var reportImportError error

func init() {

	caddy.RegisterCaddyfileLoader("caddyfile-watcher", caddy.LoaderFunc(load))

	flag.BoolVar(&active, "watch-conf-changes", false, "Watch the Caddyfile for changes and automatically reload configuration")

}

func load(serverType string) (caddy.Input, error) {

	var err error

	// Check if we are enabled

	if !active {
		return nil, nil
	}

	// Get the -conf flag for confLoader

	conf = flag.Lookup("conf").Value.String()

	// Try Caddyfile loading in the same way Caddy does

	input, err := emulateCaddyfileLoading(serverType)
	if err != nil {
		return nil, err
	}
	if input == nil {
		// We can already return an empty Input, because the normal loading will not find a Caddyfile either
		return caddy.CaddyfileInput{}, nil
	}

	// Reset state if this is a reload

	importedFiles = nil
	reportImportError = nil

	// Add the top level Caddyfile as the first imported file

	reportImportedFile(input.Path())

	// Parse the Caddyfile to find out about imports
	// The parser is modified so that it emits every imported file to importedFiles

	_, err = Parse(input.Path(), bytes.NewReader(input.Body()), nil)
	if err != nil {
		// TODO: If this happens, the watcher is already closed. It should continue to watch until the syntax is ok again.
		return nil, err
	}

	// Handle errors that happened during parsing

	if reportImportError != nil {
		return nil, reportImportError
	}

	// Stop the old watcher

	if watcher != nil {
		watcher.Close()
	}

	// Start watcher

	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	for _, i := range importedFiles {
		fmt.Println("Watching config file: " + filepath.Join(i.Dir, i.Base))
		err = watcher.Add(i.Dir) // Watch the directory, so we get notified about the creation of the file in case the editor uses temp files
		if err != nil {
			return nil, err
		}
	}

	go func() {

		for {
			select {
			case event := <-watcher.Events:
				if event.Name != "" {
					onFilechangeEventReceived(event.Name)
				}
			case err := <-watcher.Errors:
				if err != nil {
					// I don't think this ever happens
					fmt.Println("Error from file watcher: " + err.Error())
				}
			}
		}

	}()

	return input, nil
}

var debouncer struct {
	sync.Mutex
	Blind  bool
	Reason string
}

func onFilechangeEventReceived(filename string) {

	// If we are already in the blind time, there's no need to process events (premature optimization?)
	debouncer.Lock()
	if debouncer.Blind {
		debouncer.Unlock()
		return
	}
	debouncer.Unlock()

	abs, err := filepath.Abs(filename)
	if err != nil {
		fmt.Println("Error in config file watcher: " + err.Error())
		return
	}

	dir := filepath.Dir(abs)
	base := filepath.Base(abs)

	for _, i := range importedFiles {
		if i.Dir == dir && i.Base == base {
			triggerDebouncer(filename)
		}
	}

}

func triggerDebouncer(filename string) {

	debouncer.Lock()
	if debouncer.Blind {
		debouncer.Unlock()
		return
	}
	debouncer.Blind = true
	debouncer.Unlock()

	debouncer.Reason = filename

	time.AfterFunc(50*time.Millisecond, func() {

		debouncer.Lock()
		debouncer.Blind = false
		debouncer.Unlock()

		fmt.Println("File changed: " + debouncer.Reason)

		doReload()

	})

}

func emulateCaddyfileLoading(serverType string) (caddy.Input, error) {

	input, err := confLoader(serverType)
	if err != nil || input != nil {
		return input, err
	}

	return defaultLoader(serverType)

}

func reportImportedFile(filename string) {

	abs, err := filepath.Abs(filename)
	if err != nil {
		reportImportError = err
		return
	}
	dir := filepath.Dir(abs)
	base := filepath.Base(abs)

	for _, i := range importedFiles {
		if i.Dir == dir && i.Base == base {
			return
		}
	}
	importedFiles = append(importedFiles, struct {
		Dir  string
		Base string
	}{dir, base})
}
