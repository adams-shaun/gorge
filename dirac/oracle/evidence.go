package oracle

import "github.com/adams-shaun/gorge/dirac/cardset"

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
	// EvidencePodMinPlays is how many clean four-seat games a Commander-legal
	// card needs before a pod row evidences it (OD-4 stage 2). A pod game has
	// commanders, so a key a two-seat game never reaches (Command Tower's
	// colour-identity mana, CR 903.4) is used there and credited exactly as a
	// two-seat game's key is. The floor is protected: lowering it would let a
	// pod row stand for fewer games than the design approved.
	EvidencePodMinPlays = 1
	// EvidenceScriptedMinGames is how many clean scripted games, over at least two
	// distinct seeds, must use a key organic games never used before a scripted
	// run credits the card (design section 5, D10). A scripted game credits key
	// uses only: the plays floor stays organic. scripts/dirac-first-refusals.py
	// holds the writer's copy, EVIDENCE_SCRIPTED_MIN_GAMES.
	EvidenceScriptedMinGames = 3
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

// GateMinGames is how many games a deck pool's gate slice, 1500 pairs at the evidence
// seed, must start. A refused or unverified card takes every deck holding it out of the pool,
// so the slice reads on past pairs the set turns away (PairingsPlayable); a pool too
// short of decks to fill it fails, and lowering a floor is an edit a reviewer reads.
var GateMinGames = map[string]int{"fdn": 1500, "blb": 1500, "pauper": 1500}

// SliceMinGames is the same floor on a committed slice, which holds SlicePairs pairs and
// so has none to read on to: a clone with no pool plays what the refusals leave of them.
const SliceMinGames = SlicePairs * 9 / 10

// gatePairsLimit is how far past n pairs PairingsPlayable reads, as a multiple of n.
const gatePairsLimit = 20

// PairingsPlayable is the pairs from first on until n of them hold only cards set
// plays. The pairs it turned away stay in the list, so a verdict counts them.
func PairingsPlayable(c *Corpus, set cardset.Set, pool []PoolDeck, first, n int, seed uint64) []Pairing {
	supported := set.Supported()
	playable := map[string]bool{}
	deckPlays := func(name string) bool {
		ok, seen := playable[name]
		if seen {
			return ok
		}
		f, err := c.deckFile(name)
		ok = err == nil
		for _, e := range f.Cards {
			if i, found := set.Lookup(e.Name); found && !supported[i] {
				ok = false
			}
		}
		playable[name] = ok
		return ok
	}
	var out []Pairing
	playing := 0
	for p := first; playing < n && p < first+n*gatePairsLimit; p++ {
		pair := PairingAt(pool, p, seed)
		out = append(out, pair)
		if deckPlays(pair.A) && deckPlays(pair.B) {
			playing++
		}
	}
	return out
}

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

// EvidenceForSeats is Evidence for a run of seats seats. A pod run (four or
// more seats) writes a row only with at least EvidencePodMinPlays clean games;
// otherwise the row would claim a pod game the run did not have. The ability-key
// rule is Evidence's at every seat count, so a commander-only key used in a
// clean four-seat game is credited exactly as an own key used at two seats. A
// two- or three-seat run is unchanged: a three-seat row is report only.
func (c *Coverage) EvidenceForSeats(seats int) []string {
	if seats >= 4 && c.Games < EvidencePodMinPlays {
		return nil
	}
	return c.Evidence()
}
