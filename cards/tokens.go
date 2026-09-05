package cards

import "path/filepath"

// TokensDir is where Fetch places Forge's token scripts inside dir. They
// share the card scripts' line grammar and licence (GPL-3.0, never
// committed) and are keyed by file stem — "r_1_1_goblin" — which is the
// name a card's TokenScript$ parameter uses.
func TokensDir(dir string) string { return filepath.Join(dir, "tokenscripts") }

// Token looks a token definition up by script stem.
func (r *Registry) Token(key string) (*Card, bool) {
	c, ok := r.Tokens[key]
	return c, ok
}
