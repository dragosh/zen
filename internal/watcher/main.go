package watcher

import (
	"log"
	"regexp"
	"time"

	"github.com/radovskyb/watcher"
)

func Start(path string, filesPattern string, callback func(string)) {
	w := watcher.New()

	// Ignore hidden files.
	w.IgnoreHiddenFiles(true)

	//only notify rename and move events.
	w.FilterOps(watcher.Write, watcher.Move, watcher.Rename, watcher.Create)

	// add recursive current directory
	if err := w.AddRecursive(path); err != nil {
		log.Fatalln(err)
	}
	if err := w.Ignore("./node_modules/*", "./.git/*"); err != nil {
		log.Fatalln(err)
	}

	r := regexp.MustCompile(filesPattern)
	w.AddFilterHook(watcher.RegexFilterHook(r, false))

	go func() {
		for {
			select {
			case event := <-w.Event:
				callback(event.Path)
			case err := <-w.Error:
				log.Fatalln(err)
			case <-w.Closed:
				return
			}
		}
	}()

	if err := w.Start(time.Millisecond * 10); err != nil {
		log.Fatalln(err)
	}
}
