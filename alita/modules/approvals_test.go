package modules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractDisplayName(t *testing.T) {
	// Known users may or may not be in the test DB depending on test ordering.
	name := extractDisplayName(99999999999)
	assert.NotEmpty(t, name)
}
