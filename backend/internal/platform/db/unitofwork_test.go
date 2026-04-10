// backend/internal/platform/db/unitofwork_test.go
package db_test

import (
	"testing"

	"github.com/quorant/quorant/internal/platform/db"
	"github.com/stretchr/testify/assert"
)

func TestNewUnitOfWorkFactory(t *testing.T) {
	factory := db.NewUnitOfWorkFactory(nil)
	assert.NotNil(t, factory)
}
