//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// ============================================================
// Plugin DLL Contract (C/C++)
// ============================================================
//
// The DLL must export ONE function:
//
//   int QuokkaPlugin(const char* input, const char* output_path, HWND active_window)
//
// Parameters:
//   input       — current search query text (UTF-8, null-terminated)
//   output_path — path to a JSON file where the DLL should write results
//   active_window — HWND of the currently active (foreground) window
//
// The DLL writes a JSON array of entries to output_path:
//   [
//     { "name": "Entry Name", "path": "C:\\path\\to\\item", "icon": "" }
//   ]
//
// Return: 0 = success, non-zero = error.
//
// Example (C):
//   #include <stdio.h>
//   #include <windows.h>
//   __declspec(dllexport) int QuokkaPlugin(const char* input,
//                                          const char* output_path,
//                                          HWND active_window) {
//       FILE* f = fopen(output_path, "w");
//       if (!f) return 1;
//       fprintf(f, "[{\"name\":\"Hello from plugin\",\"path\":\"\",\"icon\":\"\"}]");
//       fclose(f);
//       return 0;
//   }

// PluginEntry represents a single entry returned by a plugin.
type PluginEntry struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Icon   string `json:"icon"`
	Source string `json:"source"` // plugin name that produced this entry
}

// PluginInfo describes a registered plugin.
type PluginInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
}

// pluginConfig is persisted to disk.
type pluginConfig struct {
	Plugins []PluginInfo `json:"plugins"`
}

// PluginManager manages registered DLL plugins.
type PluginManager struct {
	mu      sync.RWMutex
	config  pluginConfig
	cfgPath string
	dllCache map[string]dllHandle
}

type dllHandle struct {
	handle syscall.Handle
	proc   uintptr
	lastUsed time.Time
}

var (
	maxCachedDLLs = 5
	dllCacheMu    sync.Mutex
	dllCache      = make(map[string]dllHandle)
)

// NewPluginManager creates a plugin manager and loads config from disk.
func NewPluginManager() *PluginManager {
	cfgPath := pluginConfigFilePath("plugins.json")
	pm := &PluginManager{
		cfgPath: cfgPath,
	}
	pm.loadConfig()
	return pm
}

// pluginConfigFilePath returns a path under the user's config directory.
func pluginConfigFilePath(name string) string {
	appData := os.Getenv("APPDATA")
	dir := filepath.Join(appData, "Quokka")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, name)
}

func (pm *PluginManager) loadConfig() {
	data, err := os.ReadFile(pm.cfgPath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &pm.config)
}

func (pm *PluginManager) saveConfig() {
	data, err := json.MarshalIndent(pm.config, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(pm.cfgPath, data, 0644)
}

// AddPlugin registers a new DLL plugin. Copies the DLL to the plugins dir.
func (pm *PluginManager) AddPlugin(name, dllPath string) string {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Copy DLL to Quokka plugins directory
	pluginsDir := pluginConfigFilePath("plugins")
	os.MkdirAll(pluginsDir, 0755)

	dest := filepath.Join(pluginsDir, filepath.Base(dllPath))

	// Handle name collisions
	if _, err := os.Stat(dest); err == nil {
		// File exists — check if it's already registered
		for i, p := range pm.config.Plugins {
			if p.Path == dest {
				pm.config.Plugins[i].Enabled = true
				pm.config.Plugins[i].Name = name
				pm.saveConfig()
				return ""
			}
		}
		// Rename to avoid overwrite
		ext := filepath.Ext(dest)
		base := dest[:len(dest)-len(ext)]
		for i := 1; ; i++ {
			candidate := base + "_" + itoa(i) + ext
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				dest = candidate
				break
			}
		}
	}

	data, err := os.ReadFile(dllPath)
	if err != nil {
		return "Failed to read DLL: " + err.Error()
	}
	if err := os.WriteFile(dest, data, 0644); err != nil {
		return "Failed to copy DLL: " + err.Error()
	}

	pm.config.Plugins = append(pm.config.Plugins, PluginInfo{
		Name:    name,
		Path:    dest,
		Enabled: true,
	})
	pm.saveConfig()
	return ""
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}

// RemovePlugin removes a plugin by index.
func (pm *PluginManager) RemovePlugin(index int) string {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if index < 0 || index >= len(pm.config.Plugins) {
		return "Invalid plugin index"
	}

	os.Remove(pm.config.Plugins[index].Path)
	pm.config.Plugins = append(pm.config.Plugins[:index], pm.config.Plugins[index+1:]...)
	pm.saveConfig()
	return ""
}

// TogglePlugin enables/disables a plugin by index.
func (pm *PluginManager) TogglePlugin(index int) string {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if index < 0 || index >= len(pm.config.Plugins) {
		return "Invalid plugin index"
	}
	pm.config.Plugins[index].Enabled = !pm.config.Plugins[index].Enabled
	pm.saveConfig()
	return ""
}

// ListPlugins returns all registered plugins.
func (pm *PluginManager) ListPlugins() []PluginInfo {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	result := make([]PluginInfo, len(pm.config.Plugins))
	copy(result, pm.config.Plugins)
	return result
}

// RunPlugins executes all enabled plugins and returns collected entries.
func (pm *PluginManager) RunPlugins(input string, activeHWND uintptr) []PluginEntry {
	pm.mu.RLock()
	plugins := make([]PluginInfo, len(pm.config.Plugins))
	copy(plugins, pm.config.Plugins)
	pm.mu.RUnlock()

	var allEntries []PluginEntry
	var wg sync.WaitGroup
	resultChan := make(chan []PluginEntry, len(plugins))

	for _, p := range plugins {
		if !p.Enabled {
			continue
		}
		wg.Add(1)
		go func(plugin PluginInfo) {
			defer wg.Done()
			entries := runSinglePlugin(plugin, input, activeHWND)
			if len(entries) > 0 {
				resultChan <- entries
			}
		}(p)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for entries := range resultChan {
		allEntries = append(allEntries, entries...)
	}

	return allEntries
}

// runSinglePlugin loads and executes a single plugin DLL.
func runSinglePlugin(info PluginInfo, input string, activeHWND uintptr) []PluginEntry {
	// Create temp output JSON file
	tmpFile, err := os.CreateTemp("", "quokka_plugin_*.json")
	if err != nil {
		return nil
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Check DLL cache first
	dllCacheMu.Lock()
	cached, ok := dllCache[info.Path]
	if ok {
		// Verify DLL still exists
		if _, err := os.Stat(info.Path); err != nil {
			syscall.FreeLibrary(cached.handle)
			delete(dllCache, info.Path)
			ok = false
		} else {
			cached.lastUsed = time.Now()
			dllCache[info.Path] = cached
		}
	}
	dllCacheMu.Unlock()

	var handle syscall.Handle
	var proc uintptr

	if !ok {
		// Load DLL
		handle, err = syscall.LoadLibrary(info.Path)
		if err != nil {
			return nil
		}

		// Look up QuokkaPlugin entry point
		proc, err = syscall.GetProcAddress(handle, "QuokkaPlugin")
		if err != nil {
			syscall.FreeLibrary(handle)
			return nil
		}

		// Cache the handle
		dllCacheMu.Lock()
		if len(dllCache) >= maxCachedDLLs {
			// Remove oldest entry
			oldestPath := ""
			oldestTime := time.Now()
			for path, h := range dllCache {
				if h.lastUsed.Before(oldestTime) {
					oldestTime = h.lastUsed
					oldestPath = path
				}
			}
			if oldestPath != "" {
				syscall.FreeLibrary(dllCache[oldestPath].handle)
				delete(dllCache, oldestPath)
			}
		}
		dllCache[info.Path] = dllHandle{
			handle:   handle,
			proc:     proc,
			lastUsed: time.Now(),
		}
		dllCacheMu.Unlock()
	} else {
		handle = cached.handle
		proc = cached.proc
	}

	// Prepare C strings
	inputBytes := append([]byte(input), 0)
	tmpPathBytes := append([]byte(tmpPath), 0)

	// Call: int QuokkaPlugin(const char* input, const char* output_path, HWND active_window)
	ret, _, _ := syscall.Syscall(proc, 3,
		uintptr(unsafe.Pointer(&inputBytes[0])),
		uintptr(unsafe.Pointer(&tmpPathBytes[0])),
		activeHWND,
	)

	if ret != 0 {
		return nil
	}

	// Read and parse the output JSON
	data, err := os.ReadFile(tmpPath)
	if err != nil || len(data) == 0 {
		return nil
	}

	var entries []PluginEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}

	// Tag each entry with the plugin source
	for i := range entries {
		entries[i].Source = info.Name
	}

	return entries
}
