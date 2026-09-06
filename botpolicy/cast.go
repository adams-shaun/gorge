package botpolicy

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/decision"
)

// Card is the casting half of the policy's picture of one object the
// deciding seat can legally see (its own hand, graveyard and battlefield):
// whether it is a creature, its derived power, its printed mana value, and
// whether it is a basic land. The facts are exactly the ones the casting
// rules below read, produced identically by boardFromView (off the
// projected CardViews the seat receives) and BoardFromGame (off
// state.Game), so a card the policy ranks means the same thing whichever
// host asked. A field no rule reads would be untested surface; these four
// are the ones chooseCast and chooseLand branch on.
//
// Power is the engine's derived power (ch.Power), which for a card in a
// hand or graveyard is its printed power — no continuous effect applies to
// a card that is not on the battlefield. Reading it the same way on both
// halves is what keeps the adapter pair in step; a fact one half fills and
// the other leaves zero would be a bot that casts differently depending on
// who asked.
type Card struct {
	Creature bool
	Power    int32
	CMC      int32
	Basic    bool
}

// CmcOf is the converted-mana-cost count a botpolicy.Card reads, a re-read
// of a card's Forge ManaCost string ("R", "U U", "1 BP BP", "X G", "no
// cost"). It deliberately re-derives rules/mana.go's ParseCost.CMC() by hand
// because botpolicy cannot import rules (Ruling F7); both adapter halves
// call this same exported function on the same printed field, so they can
// only agree. {X} counts as 0 (a printed X is 0 on the stack before a value
// is chosen — the engine's own CMC() agrees), a hybrid or Phyrexian or
// colourless symbol approximates as one generic, and a brace-form cost
// ("{2}{U}{U}") is normalised to the space-separated form first.
func CmcOf(mc string) int32 {
	mc = strings.NewReplacer("{", " ", "}", " ").Replace(mc)
	mc = strings.TrimSpace(mc)
	if mc == "" || strings.EqualFold(mc, "no cost") {
		return 0
	}
	var n int32
	for _, sym := range strings.Fields(mc) {
		if sym == "X" { // {X} is 0 off the stack
			continue
		}
		if len(sym) == 1 && strings.ContainsRune("WUBRGC", rune(sym[0])) { // a single coloured/colourless pip
			n++
			continue
		}
		if v, err := strconv.Atoi(sym); err == nil && v >= 0 {
			n += int32(v)
			continue
		}
		// Hybrid ("W/U"), Phyrexian ("UP"), and any other symbol: one generic.
		n++
	}
	return n
}

// hasTypeWord is botpolicy's type-membership test, re-expressed so the two
// adapter halves say the same thing without either importing view or rules:
// boardFromView splits the projected CardView.Types string; BoardFromGame
// reads the face's own Types slice. "Basic" being present is how a land is
// a basic land (a basic Plains is "Types:Basic Land Plains"; a dual like
// Underground Sea is "Types:Land Island Swamp" — subtypes "Island Swamp"
// but no Basic, which is the whole point of the L1 rule).
func hasTypeWord(words []string, want string) bool {
	for _, w := range words {
		if strings.EqualFold(w, want) {
			return true
		}
	}
	return false
}

// castScore ranks one "cast" option. It is the one function whichever
// chooseCast reads, so the C-rules below are exactly this arithmetic, and
// a mutation that takes the first "cast" option, or treats every cast the
// same, changes it:
//
//   - a creature scores 30 + 4*Power, which outranks every non-creature
//     this policy can read (a non-creature scores its mana value, a
//     realistic ceiling of a handful all the while a 1/1 sits at 34) — the
//     C1 preference that the damage engine is worth more than a one-shot
//     — and higher power outranks lower (C2);
//   - a non-creature scores its mana value, so the biggest spell the bot
//     can afford wins the "which one-shot" question (C3);
//   - an alternative-cost cast (Mode kicked/surged vs flashback/miracle)
//     is never scored as an ordinary cast (C4): each adds its own small
//     premium, because a kicked/surge larger effect and a flashback/
//     miracle second use are both net-upside casts the bot is committing
//     to anyway — but the premium is tiny so it only ever decides an
//     otherwise-tied pair of the same-strength cards, not the card class
//     preference above.
//
// A cast whose Obj is in no zone the board reads cards for scores as a
// non-creature of mana value 0 (C5): it can only win against another
// zero-fact card, and never beats a real read.
func (b Board) castScore(o decision.Option) int32 {
	c := b.Cards[o.Obj]
	var s int32
	if c.Creature {
		s = 30 + c.Power*4
	} else {
		s = c.CMC
	}
	switch o.Mode {
	case "kicked", "surged":
		s += 6
	case "flashback", "miracle":
		s += 4
	}
	return s
}

// chooseCast is the KPriority cast ranking: it picks ONE of the offered
// "cast" options, or returns -1 when none is offered. The rules, each
// stated for what it reads:
//
//   - C1 (board over one-shots): a creature outranks a non-creature. A
//     creature is the only card type that attacks, and therefore deals
//     damage, every turn; casting one adds a permanent source of damage —
//     the fast path to ending the game. A one-shot (burn/removal) removes
//     one thing or deals damage once and is done. With the effect unread,
//     a creature is the choice that moves the game toward a finish.
//   - C2 (more imminent damage): among creatures, the higher power wins —
//     the more damage it will deal the next time it attacks, and every
//     time after.
//   - C3 (bigger effect): among non-creatures, the higher mana value wins
//     — the printed cost the bot is spending, its best guess at the bigger
//     effect when it cannot read effects.
//   - C4 (no alternative cost is an ordinary cast): a kicked/surged/
//     flashback/miracle "cast" option scores apart from an ordinary one
//     (see castScore), read off Option.Mode never the label.
//   - C5 (unreadable): an option whose Obj carries no board card facts
//     scores as a non-creature of mana value 0, a low rank, never a crash.
//   - C6 (deterministic tie): ties break on option index, so no map
//     iteration order reaches the answer.
//
// Like the target and combat branches it consumes no rng: the pick is a
// pure function of the offered options and the board facts.
func (b Board) chooseCast(d *decision.Decision) int {
	best := -1
	var bestScore int32 = -1
	for _, o := range d.Options {
		if o.Kind != "cast" {
			continue
		}
		s := b.castScore(o)
		if best == -1 || s > bestScore || (s == bestScore && o.Index < best) {
			best, bestScore = o.Index, s
		}
	}
	return best
}

// chooseLand is the KPriority land-drop ranking: it picks ONE of the
// offered "play_land" options, or -1 when none is offered.
//
//   - L1 (reliable first): a basic land outranks a nonbasic. A basic land
//     unconditionally produces exactly one coloured mana and never enters
//     the battlefield tapped nor demands life or a condition; a nonbasic
//     can carry any of those (enters tapped, pays life, requires a
//     threshold), and the policy cannot read most of them from the facts
//     both adapters carry, so the reliable basic is preferred whenever one
//     is offered. Colour-aware choice — picking the land whose colour the
//     hand's spells need — is a follow-up (the View does not carry a
//     nonbasic's produced colour; see the report).
//   - L2 (deterministic tie): two lands of equal basic-ness tie on option
//     index, so the answer is a pure function of the options plus the one
//     readable land fact. No rng, no map order.
func (b Board) chooseLand(d *decision.Decision) int {
	best := -1
	bestBasic := false
	for _, o := range d.Options {
		if o.Kind != "play_land" {
			continue
		}
		basic := b.Cards[o.Obj].Basic // zero facts read as nonbasic, never a crash
		if best == -1 || basic && !bestBasic || basic == bestBasic && o.Index < best {
			best, bestBasic = o.Index, basic
		}
	}
	return best
}
