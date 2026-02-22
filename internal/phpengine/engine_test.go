package phpengine_test

import (
	"testing"

	"github.com/sadewadee/maboo/internal/phpengine"
)

func TestNewEngine(t *testing.T) {
	// This test will skip if CGO bindings are not complete
	engine, err := phpengine.NewEngine("8.3")
	if err != nil {
		t.Skipf("CGO bindings not ready: %v", err)
	}
	defer engine.Shutdown()

	if engine.Version() != "8.3" {
		t.Errorf("expected version 8.3, got %s", engine.Version())
	}
}

func TestNewEngineInvalidVersion(t *testing.T) {
	_, err := phpengine.NewEngine("5.6")
	if err == nil {
		t.Error("expected error for unsupported version")
	}
}

func TestEngineLifecycle(t *testing.T) {
	engine, err := phpengine.NewEngine("8.3")
	if err != nil {
		t.Skipf("CGO bindings not ready: %v", err)
	}

	// Startup
	if err := engine.Startup(); err != nil {
		t.Errorf("startup failed: %v", err)
	}

	// Double startup should be safe
	if err := engine.Startup(); err != nil {
		t.Errorf("double startup failed: %v", err)
	}

	// Shutdown
	if err := engine.Shutdown(); err != nil {
		t.Errorf("shutdown failed: %v", err)
	}

	// Double shutdown should be safe
	if err := engine.Shutdown(); err != nil {
		t.Errorf("double shutdown failed: %v", err)
	}
}
