package fin_test

import (
	"testing"

	"github.com/quorant/quorant/internal/fin"
)

// TestPostgresGLRepository_ImplementsInterface is a compile-time check that
// PostgresGLRepository satisfies the GLRepository interface.
func TestPostgresGLRepository_ImplementsInterface(t *testing.T) {
	var _ fin.GLRepository = (*fin.PostgresGLRepository)(nil)
}
