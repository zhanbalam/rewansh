package main

import (
	"log"
	"os"
)

// TODO: Replace implementation dependecny with the interface

type logger struct {
	log *log.Logger
}

func newLogger() *logger {
	return &logger{log: log.New(os.Stdout, "", 5)}
}

func (l *logger) Debug(args ...interface{}) {
	l.log.Println(args...)
}

func (l *logger) Debugf(format string, args ...interface{}) {
	l.log.Printf(format, args...)
}
