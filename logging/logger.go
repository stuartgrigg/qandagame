package logging

import (
	"log"
	"os"
	"strings"
	"time"
)

type Logger struct {
	file  *os.File
	myLog *log.Logger
}

func NewLogger(filename string) *Logger {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	myLog := log.New(f, "", 0)
	return &Logger{file: f, myLog: myLog}
}

func (l *Logger) Close() {
	l.file.Close()
}

func (l *Logger) logEvent(descriptions ...string) {
	joinedDescription := strings.Join(descriptions, " | ")
	l.myLog.Printf("%s | %s\n", time.Now().UTC().Format(time.RFC3339), joinedDescription)
}

func (l *Logger) LogGameStarted() {
	l.logEvent("Game", "Started")
}
