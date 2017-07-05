package alog

import (
	"time"

	"github.com/Sirupsen/logrus"
)

type logEntry struct {
	Fields    logrus.Fields
	Type      logrus.Level
	Content   []interface{}
	Timestamp time.Time
}

var gLogger *logrus.Logger
var logQueue chan logEntry

func init() {
	logQueue = make(chan logEntry, 999)
}

func Start() {
	go func() {
		for {
			entry := <-logQueue
			lr := gLogger.WithFields(entry.Fields)
			switch entry.Type {
			case logrus.DebugLevel:
				lr.Debug(entry.Content...)
			case logrus.InfoLevel:
				lr.Info(entry.Content...)
			case logrus.WarnLevel:
				lr.Warning(entry.Content...)
			case logrus.ErrorLevel:
				lr.Error(entry.Content...)
			case logrus.FatalLevel:
				lr.Fatal(entry.Content...)
			case logrus.PanicLevel:
				lr.Panic(entry.Content...)
			}
		}
	}()
}

func SetLogger(logger *logrus.Logger) {
	gLogger = logger
}

func Log(level logrus.Level, fields logrus.Fields, object ...interface{}) {
	logQueue <- logEntry{
		Fields:    fields,
		Type:      level,
		Content:   object,
		Timestamp: time.Now(),
	}
}

func Debug(fields logrus.Fields, object ...interface{}) {
	Log(logrus.DebugLevel, fields, object...)
}

func Info(fields logrus.Fields, object ...interface{}) {
	Log(logrus.InfoLevel, fields, object...)
}

func Warn(fields logrus.Fields, object ...interface{}) {
	Log(logrus.WarnLevel, fields, object...)
}

func Error(fields logrus.Fields, object ...interface{}) {
	Log(logrus.ErrorLevel, fields, object...)
}

func Fatal(fields logrus.Fields, object ...interface{}) {
	Log(logrus.FatalLevel, fields, object...)
}

func Panic(fields logrus.Fields, object ...interface{}) {
	Log(logrus.PanicLevel, fields, object...)
}
