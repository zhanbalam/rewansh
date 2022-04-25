package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	log "github.com/sirupsen/logrus"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	var (
		path    = f.String("c", "config.yaml", "Yaml configuration path")
		verbose = f.Bool("v", false, "Verbose mode")
	)
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	c, err := loadConfig(*path)
	if err != nil {
		return fmt.Errorf("Unable to load config: %v", err)
	}
	if *verbose {
		log.SetLevel(log.DebugLevel)
	}
	log.Info("Starting re[wan]sh...")
	wman := newWanManager(c, log.StandardLogger())
	go wman.run()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	for {
		select {
		case <-interrupt:
			log.Info("Shutting down re[wan]sh...")
			wman.stop()
			time.Sleep(time.Second)
			log.Info("Bye.")
			os.Exit(0)
		}
	}
}
