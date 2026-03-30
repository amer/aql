package aql_test

import (
	"testing"

	"github.com/amer/aql/internal/aql"
	"github.com/stretchr/testify/assert"
)

func TestExecute(t *testing.T) {
	result := aql.Execute("SELECT * FROM users")
	assert.Equal(t, "executed: SELECT * FROM users", result)
}

func TestExecuteEmpty(t *testing.T) {
	result := aql.Execute("")
	assert.Equal(t, "executed: ", result)
}
