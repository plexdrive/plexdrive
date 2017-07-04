package alog

import (
	"time"

	"github.com/Sirupsen/logrus"
)

type LogEntry struct {
	Fields    logrus.Fields
	Type      string
	Content   []interface{}
	Timestamp time.Time
}

var gLogger *logrus.Logger
var logQueue chan LogEntry

func init() {
	logQueue = make(chan LogEntry, 999)
}

func Start() {
	go func() {
		for {
			entry := <-logQueue
			lr := gLogger.WithFields(entry.Fields)
			switch entry.Type {
			case "panic":
				lr.Panic(entry.Content)
			}
		}
	}()
}

func SetLogger(logger *logrus.Logger) {
	gLogger = logger
}

func Panic(fields logrus.Fields, object ...interface{}) {
	logQueue <- LogEntry{
		Fields:    fields,
		Type:      "panic",
		Content:   object,
		Timestamp: time.Now(),
	}
}

func Info(fields logrus.Fields, object ...interface{}) {
	logQueue <- LogEntry{
		Fields:    fields,
		Type:      "info",
		Content:   object,
		Timestamp: time.Now(),
	}
}
