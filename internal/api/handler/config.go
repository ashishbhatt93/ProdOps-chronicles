package handler

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"
)

// ConfigHandler provides get/set over base_configs.yaml.
// Keys are dot-separated paths (e.g. "ai.provider", "ai.api_key").
// Sensitive keys are masked on GET.
type ConfigHandler struct {
	configPath string
}

func NewConfigHandler(configPath string) *ConfigHandler {
	return &ConfigHandler{configPath: configPath}
}

var maskedKeys = map[string]bool{
	"ai.api_key": true,
}

// GET /api/v1/config
func (h *ConfigHandler) List(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.read()
	if err != nil {
		handleErr(w, err)
		return
	}
	masked := maskSensitive(cfg)
	respond(w, http.StatusOK, masked)
}

// GET /api/v1/config/:key
func (h *ConfigHandler) Get(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	cfg, err := h.read()
	if err != nil {
		handleErr(w, err)
		return
	}
	val := dotGet(cfg, key)
	if val == nil {
		handleErr(w, wrapNotFound("config key not found: "+key))
		return
	}
	if maskedKeys[key] {
		val = "***"
	}
	respond(w, http.StatusOK, map[string]any{"key": key, "value": val})
}

// PUT /api/v1/config/:key
func (h *ConfigHandler) Set(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	var req struct {
		Value any `json:"value"`
	}
	if err := decode(r, &req); err != nil {
		handleErr(w, wrapInvalidInput("invalid body"))
		return
	}
	cfg, err := h.read()
	if err != nil {
		handleErr(w, err)
		return
	}
	dotSet(cfg, key, req.Value)
	if err := h.write(cfg); err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "ok", "key": key})
}

func (h *ConfigHandler) read() (map[string]any, error) {
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		return nil, err
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (h *ConfigHandler) write(cfg map[string]any) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(h.configPath, data, 0600)
}

// dotGet traverses a map using a dot-separated key.
func dotGet(m map[string]any, key string) any {
	parts := strings.SplitN(key, ".", 2)
	val, ok := m[parts[0]]
	if !ok {
		return nil
	}
	if len(parts) == 1 {
		return val
	}
	sub, ok := val.(map[string]any)
	if !ok {
		return nil
	}
	return dotGet(sub, parts[1])
}

// dotSet writes a value at a dot-separated key, creating intermediate maps.
func dotSet(m map[string]any, key string, value any) {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) == 1 {
		m[parts[0]] = value
		return
	}
	sub, ok := m[parts[0]].(map[string]any)
	if !ok {
		sub = make(map[string]any)
		m[parts[0]] = sub
	}
	dotSet(sub, parts[1], value)
}

func maskSensitive(cfg map[string]any) map[string]any {
	result := make(map[string]any, len(cfg))
	for k, v := range cfg {
		if sub, ok := v.(map[string]any); ok {
			result[k] = maskSensitive(sub)
		} else {
			result[k] = v
		}
	}
	for key := range maskedKeys {
		parts := strings.SplitN(key, ".", 2)
		if len(parts) == 2 {
			if sub, ok := result[parts[0]].(map[string]any); ok {
				if _, exists := sub[parts[1]]; exists {
					sub[parts[1]] = "***"
				}
			}
		}
	}
	return result
}
