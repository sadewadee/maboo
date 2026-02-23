package phpengine

/*
#include <dlfcn.h>
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"
)

// ExtensionConfig from maboo.yaml
type ExtensionConfig struct {
	Required []string `yaml:"required"`
	Optional []string `yaml:"optional"`
}

// ExtensionManager handles PHP extension loading.
type ExtensionManager struct {
	phpVersion   string
	extensionDir string
	loaded       map[string]bool
	config       *ExtensionConfig
	mu           sync.RWMutex
}

// NewExtensionManager creates an extension manager.
func NewExtensionManager(phpVersion string, cfg *ExtensionConfig) *ExtensionManager {
	return &ExtensionManager{
		phpVersion:   phpVersion,
		extensionDir: fmt.Sprintf("/usr/local/lib/php/%s/extensions", phpVersion),
		loaded:       make(map[string]bool),
		config:       cfg,
	}
}

// SetExtensionDir sets a custom extension directory.
func (em *ExtensionManager) SetExtensionDir(dir string) {
	em.extensionDir = dir
}

// LoadExtensions loads PHP extensions based on config.
func (em *ExtensionManager) LoadExtensions() error {
	if em.config == nil {
		return nil
	}

	// Load required extensions (fail if missing)
	for _, ext := range em.config.Required {
		if err := em.loadExtension(ext, true); err != nil {
			return fmt.Errorf("required extension %s: %w", ext, err)
		}
	}

	// Load optional extensions (skip if missing)
	for _, ext := range em.config.Optional {
		if err := em.loadExtension(ext, false); err != nil {
			// Log warning but don't fail
			// log.Printf("optional extension %s not available: %v", ext, err)
			_ = err // Suppress unused variable warning
		}
	}

	return nil
}

// loadExtension loads a single PHP extension.
func (em *ExtensionManager) loadExtension(name string, required bool) error {
	em.mu.RLock()
	if em.loaded[name] {
		em.mu.RUnlock()
		return nil
	}
	em.mu.RUnlock()

	// Check if extension file exists
	extPath := filepath.Join(em.extensionDir, name+".so")

	if _, err := os.Stat(extPath); os.IsNotExist(err) {
		if required {
			return fmt.Errorf("extension not found: %s", extPath)
		}
		return err
	}

	// Load extension via dlopen
	cpath := C.CString(extPath)
	defer C.free(unsafe.Pointer(cpath))

	// RTLD_NOW | RTLD_GLOBAL = 0x002 | 0x100 = 0x102
	handle := C.dlopen(cpath, 0x102)
	if handle == nil {
		errMsg := C.GoString(C.dlerror())
		if required {
			return fmt.Errorf("failed to load extension %s: %s", name, errMsg)
		}
		return fmt.Errorf("dlopen failed: %s", errMsg)
	}

	// Find the get_module function
	getModuleSym := fmt.Sprintf("get_module")
	cgetModule := C.CString(getModuleSym)
	defer C.free(unsafe.Pointer(cgetModule))

	module := C.dlsym(handle, cgetModule)
	if module == nil {
		C.dlclose(handle)
		if required {
			return fmt.Errorf("extension %s has no get_module function", name)
		}
		return fmt.Errorf("get_module not found")
	}

	// Extension loaded successfully
	em.mu.Lock()
	em.loaded[name] = true
	em.mu.Unlock()

	return nil
}

// IsLoaded checks if an extension is loaded.
func (em *ExtensionManager) IsLoaded(name string) bool {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.loaded[name]
}

// LoadedExtensions returns a list of loaded extension names.
func (em *ExtensionManager) LoadedExtensions() []string {
	em.mu.RLock()
	defer em.mu.RUnlock()

	names := make([]string, 0, len(em.loaded))
	for name := range em.loaded {
		names = append(names, name)
	}
	return names
}

// UnloadAll unloads all extensions (for worker recycling).
func (em *ExtensionManager) UnloadAll() error {
	em.mu.Lock()
	defer em.mu.Unlock()

	// Note: PHP extensions typically can't be unloaded individually
	// This is mainly for cleanup tracking
	em.loaded = make(map[string]bool)
	return nil
}
