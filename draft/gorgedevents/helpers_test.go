package gorgedevents_test

import (
	"path/filepath"
	"testing"
)

// gorgeDecks is gorge's testkit/testutil/decks directory.
func gorgeDecks(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "testkit", "testutil", "decks")
}
