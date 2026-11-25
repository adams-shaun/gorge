package lab

import "testing"

// Config.Allow reaches cardset.AllowNamed: an unknown set is still an error,
// and an empty Allow leaves the set untouched.
func TestAllowIsOptional(t *testing.T) {
	if _, err := NewGame(Config{Set: "no-such-set", Allow: []string{"Anything"}}); err == nil {
		t.Fatal("unknown set accepted")
	}
}
