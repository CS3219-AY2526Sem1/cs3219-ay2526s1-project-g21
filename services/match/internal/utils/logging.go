package utils

import (
	"log"
	"os"
)

type Logger struct {
	l *log.Logger
}

func NewLogger() *Logger {
	return &Logger{l: log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)}
}

func (lg *Logger) Info(msg string, kv ...any)  { lg.l.Println(append([]any{"INFO:", msg}, kv...)...) }
func (lg *Logger) Warn(msg string, kv ...any)  { lg.l.Println(append([]any{"WARN:", msg}, kv...)...) }
func (lg *Logger) Error(msg string, kv ...any) { lg.l.Println(append([]any{"ERROR:", msg}, kv...)...) }
