package oracle

import "testing"

// TestRememberedLKIReadsABouncedTokenCopy: Boomerang Basics returns a token
// copy of Toph to its owner's hand; the copy ceases at the next check, but the
// "if you controlled that permanent" draw reads the permanent as it last was
// (CR 608.2h), so both engines draw.
func TestRememberedLKIReadsABouncedTokenCopy(t *testing.T) {
	synthReproLockstep(t, "remlki_token_copy_bounced", "std-tla", Config{})
}
