// Package applog collects the process log for the UI and a log file.
package applog

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	maxEntries  = 3000
	maxFileSize = 5 << 20
)

type Entry struct {
	Time    int64  `json:"time"`
	Message string `json:"message"`
	Level   string `json:"level"`
}

// Log is an io.Writer for the standard logger. Each complete line becomes an
// Entry kept in memory and appended to the log file.
type Log struct {
	mu      sync.Mutex
	entries []Entry
	partial []byte
	file    io.WriteCloser
	notify  func(Entry)
}

func Open(dir string) *Log {
	l := &Log{}
	path := filepath.Join(dir, "wdtt.log")
	if info, err := os.Stat(path); err == nil && info.Size() > maxFileSize {
		_ = os.Rename(path, path+".old")
	}
	if file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
		l.file = file
	}
	return l
}

// OnEntry sets the callback that receives every new entry.
func (l *Log) OnEntry(notify func(Entry)) {
	l.mu.Lock()
	l.notify = notify
	l.mu.Unlock()
}

func (l *Log) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_, _ = l.file.Write(p)
	}
	l.partial = append(l.partial, p...)
	for {
		i := bytes.IndexByte(l.partial, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimRight(string(l.partial[:i]), "\r")
		l.partial = l.partial[i+1:]
		if line == "" {
			continue
		}
		entry := Entry{Time: time.Now().UnixMilli(), Message: stripTimestamp(line), Level: level(line)}
		l.entries = append(l.entries, entry)
		if len(l.entries) > maxEntries {
			l.entries = append(l.entries[:0:0], l.entries[len(l.entries)-maxEntries:]...)
		}
		if l.notify != nil {
			l.notify(entry)
		}
	}
	return len(p), nil
}

func (l *Log) Entries() []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append(make([]Entry, 0, len(l.entries)), l.entries...)
}

func (l *Log) Clear() {
	l.mu.Lock()
	l.entries = nil
	l.mu.Unlock()
}

func (l *Log) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
}

// stripTimestamp removes the "2006/01/02 15:04:05.000000 " prefix added by
// the standard logger; the UI shows its own time.
func stripTimestamp(line string) string {
	if len(line) > 27 && line[4] == '/' && line[7] == '/' && line[10] == ' ' {
		if i := strings.IndexByte(line[11:], ' '); i >= 0 {
			return line[11+i+1:]
		}
	}
	return line
}

func level(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "ошибк"), strings.Contains(lower, "error"), strings.Contains(lower, "не удалось"):
		return "error"
	case strings.Contains(lower, "капч"), strings.Contains(lower, "warn"), strings.Contains(lower, "пропущ"):
		return "warn"
	default:
		return "info"
	}
}
