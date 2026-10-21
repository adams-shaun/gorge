package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/chars"
)

// TestCopySupertypesMatchChars holds the copy-exception supertype list equal
// to the layer-4 walk's.
func TestCopySupertypesMatchChars(t *testing.T) {
	t.Parallel()
	if got, want := effects.CopySupertypes(), chars.SupertypeWords(); !slices.Equal(got, want) {
		t.Fatalf("effects.CopySupertypes %v != chars.SupertypeWords %v", got, want)
	}
}
