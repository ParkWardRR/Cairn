package plugin

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
)

type PluginMeta struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	Type          string `json:"type"`
	EntryFunction string `json:"entry_function"`
	Description   string `json:"description"`
}

type Plugin struct {
	Meta     PluginMeta
	compiled wazero.CompiledModule
}

type Host struct {
	pluginDir string
	plugins   map[string]*Plugin
	runtime   wazero.Runtime
	ctx       context.Context
	mu        sync.RWMutex
}

const (
	maxMemoryPages = 256 // 256 * 64KB = 16MB
	execTimeout    = 30 * time.Second
)

func NewHost(pluginDir string) (*Host, error) {
	ctx := context.Background()

	cfg := wazero.NewRuntimeConfig().
		WithCloseOnContextDone(true).
		WithMemoryLimitPages(maxMemoryPages)

	rt := wazero.NewRuntimeWithConfig(ctx, cfg)

	h := &Host{
		pluginDir: pluginDir,
		plugins:   make(map[string]*Plugin),
		runtime:   rt,
		ctx:       ctx,
	}

	if err := h.scanPlugins(); err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("scan plugins: %w", err)
	}

	return h, nil
}

func (h *Host) scanPlugins() error {
	entries, err := os.ReadDir(h.pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("plugin directory %s does not exist, no plugins loaded", h.pluginDir)
			return nil
		}
		return fmt.Errorf("read plugin dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wasm") {
			continue
		}
		wasmPath := filepath.Join(h.pluginDir, entry.Name())
		if err := h.loadPlugin(wasmPath); err != nil {
			log.Printf("skip plugin %s: %v", entry.Name(), err)
		}
	}

	log.Printf("loaded %d plugins from %s", len(h.plugins), h.pluginDir)
	return nil
}

func (h *Host) loadPlugin(wasmPath string) error {
	manifestPath := strings.TrimSuffix(wasmPath, ".wasm") + ".json"
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest %s: %w", manifestPath, err)
	}

	var meta PluginMeta
	if err := json.Unmarshal(manifestData, &meta); err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}
	if meta.Name == "" || meta.EntryFunction == "" {
		return fmt.Errorf("manifest missing name or entry_function")
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return fmt.Errorf("read wasm: %w", err)
	}

	compiled, err := h.runtime.CompileModule(h.ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("compile wasm: %w", err)
	}

	h.mu.Lock()
	h.plugins[meta.Name] = &Plugin{
		Meta:     meta,
		compiled: compiled,
	}
	h.mu.Unlock()

	log.Printf("loaded plugin %s v%s (%s)", meta.Name, meta.Version, meta.Type)
	return nil
}

func (h *Host) Execute(pluginName string, input []byte) ([]byte, error) {
	h.mu.RLock()
	p, ok := h.plugins[pluginName]
	h.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("plugin %q not found", pluginName)
	}

	ctx, cancel := context.WithTimeout(h.ctx, execTimeout)
	defer cancel()

	// Fresh instance per call for isolation. Empty name avoids collisions
	// between concurrent instantiations of the same compiled module.
	mod, err := h.runtime.InstantiateModule(ctx, p.compiled, wazero.NewModuleConfig().WithName(""))
	if err != nil {
		return nil, fmt.Errorf("instantiate module: %w", err)
	}
	defer mod.Close(ctx)

	alloc := mod.ExportedFunction("alloc")
	if alloc == nil {
		return nil, fmt.Errorf("plugin %q does not export 'alloc'", pluginName)
	}

	entryFn := mod.ExportedFunction(p.Meta.EntryFunction)
	if entryFn == nil {
		return nil, fmt.Errorf("plugin %q does not export %q", pluginName, p.Meta.EntryFunction)
	}

	inputLen := uint64(len(input))
	results, err := alloc.Call(ctx, inputLen)
	if err != nil {
		return nil, fmt.Errorf("alloc(%d): %w", inputLen, err)
	}
	inputPtr := uint32(results[0])

	mem := mod.Memory()
	if !mem.Write(inputPtr, input) {
		return nil, fmt.Errorf("write input to wasm memory at offset %d (len %d)", inputPtr, len(input))
	}

	callResults, err := entryFn.Call(ctx, uint64(inputPtr), inputLen)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", p.Meta.EntryFunction, err)
	}

	// Return value is a pointer to a length-prefixed output buffer:
	// first 4 bytes = little-endian uint32 length, then that many bytes of JSON.
	outPtr := uint32(callResults[0])

	lenBuf, ok := mem.Read(outPtr, 4)
	if !ok {
		return nil, fmt.Errorf("read output length at offset %d", outPtr)
	}
	outLen := binary.LittleEndian.Uint32(lenBuf)

	output, ok := mem.Read(outPtr+4, outLen)
	if !ok {
		return nil, fmt.Errorf("read output data at offset %d (len %d)", outPtr+4, outLen)
	}

	// Copy before the module instance closes and frees its memory.
	result := make([]byte, len(output))
	copy(result, output)

	return result, nil
}

func (h *Host) List() []PluginMeta {
	h.mu.RLock()
	defer h.mu.RUnlock()

	metas := make([]PluginMeta, 0, len(h.plugins))
	for _, p := range h.plugins {
		metas = append(metas, p.Meta)
	}
	return metas
}

func (h *Host) Close() {
	h.runtime.Close(h.ctx)
}
