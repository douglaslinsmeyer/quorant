package i18n

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	registry *Registry
}

func NewHandler(registry *Registry) *Handler {
	return &Handler{registry: registry}
}

func (h *Handler) GetPack(w http.ResponseWriter, r *http.Request) {
	locale := r.PathValue("locale")

	pack, ok := h.registry.Get(locale)
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("ETag", `"`+pack.Version+`"`)
	_ = json.NewEncoder(w).Encode(pack)
}

func (h *Handler) ListLocales(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"locales": h.registry.Locales(),
	})
}
