package applog

import (
	"log"
	"testing"
)

func TestLogSplitsLinesAndStripsTimestamps(t *testing.T) {
	l := Open(t.TempDir())
	defer l.Close()
	var notified []Entry
	l.OnEntry(func(e Entry) { notified = append(notified, e) })

	logger := log.New(l, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	logger.Print("[КЛИЕНТ] первая")
	_, _ = l.Write([]byte("частичная "))
	_, _ = l.Write([]byte("ошибка строки\n"))

	entries := l.Entries()
	if len(entries) != 2 || len(notified) != 2 {
		t.Fatalf("got %d entries, %d notified", len(entries), len(notified))
	}
	if entries[0].Message != "[КЛИЕНТ] первая" {
		t.Errorf("timestamp not stripped: %q", entries[0].Message)
	}
	if entries[1].Message != "частичная ошибка строки" || entries[1].Level != "error" {
		t.Errorf("unexpected entry %+v", entries[1])
	}
}
