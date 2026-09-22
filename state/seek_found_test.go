package state

import "testing"

// TestCloneDeepDoesNotAliasSeekFound ensures the Seek ImprintFound$
// association remains independent in game and LKI clones. An in-place write
// catches shared backing storage; append alone could reallocate and hide it.
func TestCloneDeepDoesNotAliasSeekFound(t *testing.T) {
	orig := Object{ID: 7, SeekFound: []ObjID{8, 9}}
	clone := orig.CloneDeep()
	if len(clone.SeekFound) != 2 || clone.SeekFound[0] != 8 || clone.SeekFound[1] != 9 {
		t.Fatalf("clone SeekFound = %v, want [8 9]", clone.SeekFound)
	}

	clone.SeekFound[0] = 80
	if orig.SeekFound[0] == 80 {
		t.Fatal("CloneDeep aliases SeekFound")
	}
	if orig.SeekFound[0] != 8 {
		t.Fatalf("original SeekFound[0] = %d, want 8", orig.SeekFound[0])
	}
}
