// Package app provides the module registry pattern for the Quorant modular monolith.
// Each domain module implements the Module interface and is registered at startup.
// This replaces the monolithic main.go wiring with a plug-and-play module system.
package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/quorant/quorant/internal/platform/queue"
)

// Module is implemented by each domain module to register its routes and dependencies.
type Module interface {
	// Name returns the module's identifier (e.g., "fin", "org", "gov").
	Name() string
	// Register wires up the module's repositories, services, and handlers,
	// then registers routes on the given mux.
	Register(mux *http.ServeMux, deps Dependencies)
}

// Dependencies holds all shared infrastructure that modules need.
// Modules receive this instead of importing individual packages.
type Dependencies struct {
	Pool            *pgxpool.Pool
	Redis           *redis.Client
	Logger          *slog.Logger
	Auditor         audit.Auditor
	Publisher       queue.Publisher
	TokenValidator  auth.TokenValidator
	PermChecker     middleware.PermissionChecker
	ResolveUserID   func(ctx context.Context) (uuid.UUID, error)
	EntitlementChecker middleware.EntitlementChecker
}

// Registry holds all registered modules and provides bulk operations.
type Registry struct {
	modules []Module
}

// NewRegistry creates an empty module registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Add registers a module. Modules are registered in dependency order.
func (r *Registry) Add(m Module) {
	r.modules = append(r.modules, m)
}

// RegisterAll calls Register on every module in order.
func (r *Registry) RegisterAll(mux *http.ServeMux, deps Dependencies) {
	for _, m := range r.modules {
		deps.Logger.Info("registering module", "module", m.Name())
		m.Register(mux, deps)
	}
}

// Names returns the names of all registered modules.
func (r *Registry) Names() []string {
	names := make([]string, len(r.modules))
	for i, m := range r.modules {
		names[i] = m.Name()
	}
	return names
}
