// Copyright 2016 The G3N Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package logger

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Levels to filter log output
const (
	DEBUG = iota
	INFO
	WARN
	ERROR
	FATAL
)

// Flags used to format the log date/time
const (
	// Show date
	FDATE = 1 << iota
	// Show hour, minutes and seconds
	FTIME
	// Show milliseconds after FTIME
	FMILIS
	// Show microseconds after FTIME
	FMICROS
	// Show nanoseconfs after TIME
	FNANOS
)

// List of level names
var levelNames = [...]string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

// Default logger and global mutex
var Default *Logger = nil
var rootLoggers = []*Logger{}
var mutex sync.Mutex

// Interface for all logger writers
type LoggerWriter interface {
	Write(*Event)
	Close()
	Sync()
}

// LeveledLogger is an interface shared by most log libraries.
type LeveledLogger interface {
	Debug(...interface{})
	Debugf(string, ...interface{})

	Info(...interface{})
	Infof(string, ...interface{})

	Error(...interface{})
	Errorf(string, ...interface{})

	Fatal(...interface{})
	Fatalf(string, ...interface{})

	Panic(...interface{})
	Panicf(string, ...interface{})
}

// Logger Object state structure
type Logger struct {
	name     string
	prefix   string
	enabled  bool
	level    int
	format   int
	outputs  []LoggerWriter
	parent   *Logger
	children []*Logger
}

// Make sure Logger implements LeveledLogger correctly.
var _ LeveledLogger = new(Logger)

// Logger event passed from the logger to its writers.
type Event struct {
	time    time.Time
	level   int
	usermsg string
	fmsg    string
}

// creates the default logger
func init() {
	Default = New("G3N", nil)
	Default.SetFormat(FTIME | FMICROS)
	Default.AddWriter(NewConsole(false))
}

// New() creates and returns a new logger with the specified name.
// If a parent logger is specified, the created logger inherits the
// parent's configuration.
func New(name string, parent *Logger) *Logger {

	self := new(Logger)
	self.name = name
	self.prefix = name
	self.enabled = true
	self.level = ERROR
	self.format = FDATE | FTIME | FMICROS
	self.outputs = make([]LoggerWriter, 0)
	self.children = make([]*Logger, 0)
	self.parent = parent
	if parent != nil {
		self.prefix = parent.prefix + "/" + name
		self.enabled = parent.enabled
		self.level = parent.level
		self.format = parent.format
		parent.children = append(parent.children, self)
	} else {
		rootLoggers = append(rootLoggers, self)
	}
	return self
}

// SetLevel sets the current level of this logger.
// Only log messages with levels with the same or higher
// priorities than the current level will be emitted.
func (l *Logger) SetLevel(level int) {

	if level < DEBUG || level > FATAL {
		return
	}
	l.level = level
}

// SetLevelByName sets the current level of this logger by level name:
// debug|info|warn|error|fatal (case ignored.)
// Only log messages with levels with the same or higher
// priorities than the current level will be emitted.
func (l *Logger) SetLevelByName(lname string) error {
	var level int

	lname = strings.ToUpper(lname)
	for level = 0; level < len(levelNames); level++ {
		if lname == levelNames[level] {
			l.level = level
			return nil
		}
	}
	return fmt.Errorf("Invalid log level name: %s", lname)
}

// SetFormat sets the logger date/time message format
func (l *Logger) SetFormat(format int) {

	l.format = format
}

// AddWriter adds a writer to the current outputs of this logger.
func (l *Logger) AddWriter(writer LoggerWriter) {

	l.outputs = append(l.outputs, writer)
}

// RemoveWriter removes the specified writer from  the current outputs of this logger.
func (l *Logger) RemoveWriter(writer LoggerWriter) {

	for pos, w := range l.outputs {
		if w != writer {
			continue
		}
		l.outputs = append(l.outputs[:pos], l.outputs[pos+1:]...)
	}
}

// EnableChild enables or disables this logger child logger with
// the specified name.
func (l *Logger) EnableChild(name string, state bool) {

	for _, c := range l.children {
		if c.name == name {
			c.enabled = state
		}
	}
}

// Debug emits a DEBUG level log message.
func (l *Logger) Debug(v ...interface{}) {

	l.Log(DEBUG, v...)
}

// Debugf emits a DEBUG level log message with formatting.
func (l *Logger) Debugf(format string, v ...interface{}) {

	l.Logf(DEBUG, format, v...)
}

// Info emits an INFO level log message.
func (l *Logger) Info(v ...interface{}) {

	l.Log(INFO, v...)
}

// Infof emits an INFO level log message with formatting.
func (l *Logger) Infof(format string, v ...interface{}) {

	l.Logf(INFO, format, v...)
}

// Warn emits a WARN level log message
func (l *Logger) Warn(v ...interface{}) {

	l.Log(WARN, v...)
}

// Warnf emits a WARN level log message with formatting.
func (l *Logger) Warnf(format string, v ...interface{}) {

	l.Logf(WARN, format, v...)
}

// Error emits an ERROR level log message.
func (l *Logger) Error(v ...interface{}) {

	l.Log(ERROR, v...)
}

// Errorf emits an ERROR level log message with formatting.
func (l *Logger) Errorf(format string, v ...interface{}) {

	l.Logf(ERROR, format, v...)
}

// Fatal emits a FATAL level log message.
func (l *Logger) Fatal(v ...interface{}) {

	l.Log(FATAL, v...)
}

// Fatalf emits a FATAL level log message with formatting.
func (l *Logger) Fatalf(format string, v ...interface{}) {

	l.Logf(FATAL, format, v...)
}

// Panic emits a FATAL level log message.
func (l *Logger) Panic(v ...interface{}) {

	l.Log(FATAL, v...)
}

// Panicf emits a FATAL level log message with formatting.
func (l *Logger) Panicf(format string, v ...interface{}) {

	l.Logf(FATAL, format, v...)
}

// Log prints everything to log with no formatting.
func (l *Logger) Log(level int, v ...interface{}) {
	var b strings.Builder
	b.Grow(len(v) * 3)
	for i := 0; i < len(v); i++ {
		b.WriteString("%v ")
	}

	l.Logf(level, b.String(), v...)
}

// Logf emits a log message with the specified level.
func (l *Logger) Logf(level int, format string, v ...interface{}) {

	// Ignores message if logger not enabled or with level bellow the current one.
	if !l.enabled || level < l.level {
		return
	}

	// Formats date
	now := time.Now().UTC()
	year, month, day := now.Date()
	hour, min, sec := now.Clock()
	fdate := []string{}

	if l.format&FDATE != 0 {
		fdate = append(fdate, fmt.Sprintf("%04d/%02d/%02d", year, month, day))
	}
	if l.format&FTIME != 0 {
		if len(fdate) > 0 {
			fdate = append(fdate, "-")
		}
		fdate = append(fdate, fmt.Sprintf("%02d:%02d:%02d", hour, min, sec))
		var sdecs string
		if l.format&FMILIS != 0 {
			sdecs = fmt.Sprintf(".%.03d", now.Nanosecond()/1000000)
		} else if l.format&FMICROS != 0 {
			sdecs = fmt.Sprintf(".%.06d", now.Nanosecond()/1000)
		} else if l.format&FNANOS != 0 {
			sdecs = fmt.Sprintf(".%.09d", now.Nanosecond())
		}
		fdate = append(fdate, sdecs)
	}

	// Formats message
	usermsg := fmt.Sprintf(format, v...)
	prefix := l.prefix
	msg := fmt.Sprintf("%s:%s:%s:%s\n", strings.Join(fdate, ""), levelNames[level][:1], prefix, usermsg)

	// Log event
	var event = Event{
		time:    now,
		level:   level,
		usermsg: usermsg,
		fmsg:    msg,
	}

	// Writes message to this logger and its ancestors.
	mutex.Lock()
	defer mutex.Unlock()
	l.writeAll(&event)

	// Close all logger writers
	if level == FATAL {
		for _, w := range l.outputs {
			w.Close()
		}
		panic("LOG FATAL")
	}
}

// write message to this logger output and of all of its ancestors.
func (l *Logger) writeAll(event *Event) {

	for _, w := range l.outputs {
		w.Write(event)
		w.Sync()
	}
	if l.parent != nil {
		l.parent.writeAll(event)
	}
}

//
// Functions for the Default Logger
//

// Log emits a log message with the specified level.
func Log(level int, v ...interface{}) {

	Default.Log(level, v...)
}

// Logf emits a log message with the specified level with formatting.
func Logf(level int, format string, v ...interface{}) {

	Default.Logf(level, format, v...)
}

// SetLevel sets the current level of the default logger.
// Only log messages with levels with the same or higher
// priorities than the current level will be emitted.
func SetLevel(level int) {

	Default.SetLevel(level)
}

// SetLevelByName sets the current level of the default logger by level name:
// debug|info|warn|error|fatal (case ignored.)
// Only log messages with levels with the same or higher
// priorities than the current level will be emitted.
func SetLevelByName(lname string) {

	Default.SetLevelByName(lname)
}

// SetFormat sets the date/time message format of the default logger.
func SetFormat(format int) {

	Default.SetFormat(format)
}

// AddWriter adds a writer to the current outputs of the default logger.
func AddWriter(writer LoggerWriter) {

	Default.AddWriter(writer)
}

// Debugf emits a DEBUG level log message.
func Debugf(format string, v ...interface{}) {

	Default.Debugf(format, v...)
}

// Infof emits an INFO level log message.
func Infof(format string, v ...interface{}) {

	Default.Infof(format, v...)
}

// Warnf emits a WARN level log message.
func Warnf(format string, v ...interface{}) {

	Default.Warnf(format, v...)
}

// Errorf emits an ERROR level log message.
func Errorf(format string, v ...interface{}) {

	Default.Errorf(format, v...)
}

// Fatalf emits a FATAL level log message.
func Fatalf(format string, v ...interface{}) {

	Default.Fatalf(format, v...)
}

// Find finds a logger with the specified path.
func Find(path string) *Logger {

	parts := strings.Split(strings.ToUpper(path), "/")
	level := 0
	var find func([]*Logger) *Logger

	find = func(logs []*Logger) *Logger {

		for _, l := range logs {
			if l.name != parts[level] {
				continue
			}
			if level == len(parts)-1 {
				return l
			}
			level++
			return find(l.children)
		}
		return nil
	}
	return find(rootLoggers)
}
