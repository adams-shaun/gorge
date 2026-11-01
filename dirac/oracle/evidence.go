package oracle

// The evidence floor: what a card must reach, in clean lockstep games, before
// those games count as evidence for it.
const (
	// EvidenceMinPlays is the casts and land plays a card needs. A single play
	// can be a fizzle or an immediate answer; 3 costs little, since 3,794 of 4,022
	// Standard cards reach it in one synthetic sweep.
	EvidenceMinPlays = 3
	// EvidenceAllKeys requires every own ability key (coverageInventory) to have
	// been used: a card with an ability no game reaches is not verified.
	EvidenceAllKeys = true
	// EvidenceSeed is the seed the evidence corpus is dealt and paired from. Decks
	// and pairs are pure functions of it.
	EvidenceSeed = 1
	// SynthDecks is how many decks a synthetic pool deals: 400 x 23 slots over about
	// 240 spells, so each card recurs about 38 times.
	SynthDecks = 400
	// SynthPairs is how many games a synthetic pool plays per run, the same as a real
	// pool's gate slice.
	SynthPairs = 1500
)

// Evidence is the cards c's games evidence, sorted: at least EvidenceMinPlays
// plays and, under EvidenceAllKeys, no unused ability key. c should hold clean
// games only (MergeClean).
func (c *Coverage) Evidence() []string {
	var out []string
	for _, n := range c.names() {
		cc := c.Cards[n]
		if cc.Cast+cc.Land < EvidenceMinPlays {
			continue
		}
		if EvidenceAllKeys && len(cc.Unused()) > 0 {
			continue
		}
		out = append(out, n)
	}
	return out
}
