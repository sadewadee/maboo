package phpengine

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestLogPHPMessageLevelMapping(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	SetLogger(logger)

	logPHPMessage(3, "error message", 1)
	logPHPMessage(4, "warn message", 1)
	logPHPMessage(6, "info message", 1)
	logPHPMessage(7, "debug message", 1)

	out := buf.String()
	for _, want := range []string{"error message", "warn message", "info message", "debug message"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected log output to contain %q, got %q", want, out)
		}
	}
}
