package modules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApprovalsModuleName(t *testing.T) {
	t.Run("approvalsModule has correct name", func(t *testing.T) {
		assert.Equal(t, "Approvals", approvalsModule.moduleName)
	})
}

func TestExtractDisplayName(t *testing.T) {
	// Pure function: should not panic with unknown IDs.
	// Known users may or may not be in the test DB depending on test ordering.
	name := extractDisplayName(99999999999)
	assert.NotEmpty(t, name)
}
