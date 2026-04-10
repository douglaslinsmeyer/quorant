package app_test

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/quorant/quorant/internal/platform/app"
	"github.com/stretchr/testify/assert"
)

func testDeps() app.Dependencies {
	return app.Dependencies{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

type testModule struct {
	name       string
	registered bool
}

func (m *testModule) Name() string { return m.name }
func (m *testModule) Register(mux *http.ServeMux, deps app.Dependencies) {
	m.registered = true
}

func TestRegistry_RegisterAll(t *testing.T) {
	reg := app.NewRegistry()

	m1 := &testModule{name: "mod1"}
	m2 := &testModule{name: "mod2"}
	m3 := &testModule{name: "mod3"}

	reg.Add(m1)
	reg.Add(m2)
	reg.Add(m3)

	assert.Equal(t, []string{"mod1", "mod2", "mod3"}, reg.Names())

	mux := http.NewServeMux()
	reg.RegisterAll(mux, testDeps())

	assert.True(t, m1.registered)
	assert.True(t, m2.registered)
	assert.True(t, m3.registered)
}

func TestRegistry_Empty(t *testing.T) {
	reg := app.NewRegistry()
	assert.Empty(t, reg.Names())

	// RegisterAll with no modules should not panic
	mux := http.NewServeMux()
	reg.RegisterAll(mux, testDeps())
}
