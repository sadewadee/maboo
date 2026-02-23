package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLogOutputStdout(t *testing.T) {
	w, c := resolveLogOutput("stdout")
	if w != os.Stdout {
		t.Fatalf("expected stdout writer")
	}
	if c != nil {
		t.Fatalf("expected nil closer for stdout")
	}
}

func TestResolveLogOutputStderr(t *testing.T) {
	w, c := resolveLogOutput("stderr")
	if w != os.Stderr {
		t.Fatalf("expected stderr writer")
	}
	if c != nil {
		t.Fatalf("expected nil closer for stderr")
	}
}

func TestResolveLogOutputFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "maboo.log")

	w, c := resolveLogOutput(logPath)
	if w == nil {
		t.Fatalf("expected writer for file output")
	}
	if c == nil {
		t.Fatalf("expected closer for file output")
	}
	defer c.Close()

	f, ok := w.(*os.File)
	if !ok {
		t.Fatalf("expected *os.File writer, got %T", w)
	}

	_, err := io.WriteString(f, "test log\n")
	if err != nil {
		t.Fatalf("write log file: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if string(data) == "" {
		t.Fatalf("expected log file content")
	}
}
