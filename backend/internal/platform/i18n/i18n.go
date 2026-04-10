package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed locales/*.json
var localeFS embed.FS

type Pack struct {
	Locale   string            `json:"locale"`
	Version  string            `json:"version"`
	Messages map[string]string `json:"messages"`
}

type Registry struct {
	packs map[string]*Pack
}

func NewRegistry() (*Registry, error) {
	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		return nil, fmt.Errorf("i18n: read locales dir: %w", err)
	}

	packs := make(map[string]*Pack, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := localeFS.ReadFile("locales/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("i18n: read %s: %w", entry.Name(), err)
		}

		var pack Pack
		if err := json.Unmarshal(data, &pack); err != nil {
			return nil, fmt.Errorf("i18n: parse %s: %w", entry.Name(), err)
		}

		locale := strings.TrimSuffix(entry.Name(), ".json")
		pack.Locale = locale
		packs[locale] = &pack
	}

	return &Registry{packs: packs}, nil
}

func (r *Registry) Get(locale string) (*Pack, bool) {
	p, ok := r.packs[locale]
	return p, ok
}

func (r *Registry) Locales() []string {
	locales := make([]string, 0, len(r.packs))
	for k := range r.packs {
		locales = append(locales, k)
	}
	sort.Strings(locales)
	return locales
}
