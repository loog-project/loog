package ui

import (
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const maxLogLines = 1000

// Logger is a minimal interface for sending messages to the UI.
type Logger interface {
	Infof(source, format string, args ...any)
	Warningf(source, format string, args ...any)
	Errorf(source, format string, args ...any)
}

type LogLevel uint8

const (
	// LogLevelInfo is the default log level.
	LogLevelInfo LogLevel = iota
	// LogLevelWarning is the warning log level.
	LogLevelWarning
	// LogLevelError is the error log level.
	LogLevelError
)

type LogMsg struct {
	Time   time.Time
	Level  LogLevel
	Source string
	Text   string
}

type UILogger struct {
	program *tea.Program

	mutex          sync.Mutex
	unreadPerLevel map[LogLevel]int
	globalUnread   int

	messages []LogMsg
}

func NewUILogger() *UILogger {
	return &UILogger{
		unreadPerLevel: make(map[LogLevel]int),
	}
}

func (l *UILogger) Attach(p *tea.Program) {
	l.program = p
}

func (l *UILogger) send(level LogLevel, source, text string) {
	msg := LogMsg{
		Time:   time.Now(),
		Level:  level,
		Source: source,
		Text:   text,
	}

	l.mutex.Lock()
	if len(l.messages) >= maxLogLines {
		copy(l.messages, l.messages[1:])
		l.messages[len(l.messages)-1] = msg
	} else {
		l.messages = append(l.messages, msg)
	}
	l.unreadPerLevel[level]++
	l.globalUnread++
	l.mutex.Unlock()

	if l.program != nil {
		l.program.Send(msg)
	}
}

func (l *UILogger) Infof(source, format string, args ...any) {
	l.send(LogLevelInfo, source, fmt.Sprintf(format, args...))
}

func (l *UILogger) Warningf(source, format string, args ...any) {
	l.send(LogLevelWarning, source, fmt.Sprintf(format, args...))
}

func (l *UILogger) Errorf(source, format string, args ...any) {
	l.send(LogLevelError, source, fmt.Sprintf(format, args...))
}

func (l *UILogger) popUnread() (info, warn, errors int) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	info = l.unreadPerLevel[LogLevelInfo]
	warn = l.unreadPerLevel[LogLevelWarning]
	errors = l.unreadPerLevel[LogLevelError]

	l.unreadPerLevel[LogLevelInfo] = 0
	l.unreadPerLevel[LogLevelWarning] = 0
	l.unreadPerLevel[LogLevelError] = 0

	l.globalUnread = 0

	return info, warn, errors
}
