package botpolicy

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// Card is the casting half of the policy's picture of one object the
// deciding seat can legally see (its own hand, graveyard and battlefield):
// whether it is a creature, its derived power, its printed mana value, and
// whether it is a basic land. The facts are exactly the ones the casting
// rules below read, produced identically by boardFromView (off the
// projected CardViews the seat receives) and BoardFromGame (off
// state.Game), so a card the policy ranks means the same thing whichever
// host asked. Every field below is read by a rule branch, so none is
// untested surface: the set is the ones chooseCast (Creature, Power,
// CMC), chooseLand (Basic), the ability ranking (AttachedTo) and the tap
// gate (Castable, ManaCost, Produces, and the creature facts it shares
// with chooseCast) actually branch on. The obligation that both halves
// fill every field identically is enforced, not aspirational: the
// adapter-parity tests (seat/integration_test.go's
// TestBotAdaptersAgreeOverWholeGame and
// TestBotAdaptersAgreeOverCommanderGame) compare the two halves' Card
// maps with maps.Equal -- every field of botpolicy.Card, on every intent
// of two whole games -- so a field one half fills and the other leaves
// zero is caught. A new Card field must be filled on both halves or not
// added at all.
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
	// AttachedTo is the permanent this Aura or Equipment is currently
	// attached to, 0 when unattached (state.ObjID's own zero convention).
	// It comes straight from state.Object.AttachedTo (and the projected
	// CardView.AttachedTo, which is that same field) on the two adapter
	// halves, so A1's equip no-op detection (ability.go) judges a re-attach
	// identically whichever host asks. Only a battlefield permanent carries
	// a non-zero AttachedTo; a hand or graveyard card reads 0 on both
	// halves, which is what lets them stay a plain casting fact.
	AttachedTo state.ObjID
	// ManaCost is the printed cost in Forge notation ("1 W", "U U",
	// "X G") -- the same string both understanding halves already read for
	// CMC (CmcOf) and the View carries as CardView.ManaCost. The tap gate
	// (tap.go) re-parses it for the coloured-pip demands of the card it is
	// pricing, which is where a colour-blind tap policy would otherwise
	// float the wrong colour and stall against a multi-coloured cost.
	ManaCost string
	// Castable reports whether the deciding seat could cast this card from
	// where it currently sits, given enough mana: true for the seat's own
	// hand, for the command zone (this engine only ever puts commanders
	// there), and for the graveyard when the card has the Flashback
	// keyword; false for the battlefield (a permanent is already cast).
	// The tap gate reads it to restrict "a spell worth mana" to spells the
	// seat can actually cast -- a battlefield permanent never justifies a
	// tap. Both halves fill it from the same zone membership (the projected
	// zone lists; the engine's zone walk plus the derived keyword list), so
	// the gate sees the same castability whichever host asks.
	Castable bool
	// Produces is what this card's mana abilities add to the pool when a
	// tap-for-mana activation runs them (cards.ManaProduction, plain data
	// -- botpolicy must not import view or rules, Ruling F7). It is filled
	// by both adapter halves from the same source -- the projected
	// CardView.Produces on the view half, cards.Face.ManaProduction on the
	// game half -- so the tap heuristic reads the same production whichever
	// host asks. It lets chooseTap pick a source that produces a colour a cast
	// needs and spend the least flexible one first. It also describes a land
	// card while it is still in hand, so a future land-drop ranking can use the
	// same fact.
	Produces cards.ManaProduction
}

// braceForm normalises a brace-form mana cost ("{2}{U}{U}") to the
// space-separated form CmcOf parses. It is built once at package scope, not
// per call: a strings.Replacer is immutable and safe for concurrent use, and
// CmcOf runs on every card of every zone of every projected board — building
// the trie inside the function cost ~165MB of garbage per host test.
var braceForm = strings.NewReplacer("{", " ", "}", " ")

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
	mc = braceForm.Replace(mc)
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
	s := b.cardWorth(o.Obj)
	switch o.Mode {
	case "kicked", "surged":
		s += 6
	case "flashback", "miracle":
		s += 4
	}
	return s
}

// cmdrTaxScale prices each prior command-zone cast by the commander's mana
// value. Five is the old 20-point penalty normalized to a CMC-4 reference;
// using the mana axis makes the recast value judgment track its cost.
const cmdrTaxScale = 5

// cardWorth prices cast desirability, not the usefulness of keeping a hand card.
func (b Board) cardWorth(id state.ObjID) int32 {
	c := b.Cards[id]
	if c.Creature {
		return 30 + 4*c.Power
	}
	return c.CMC
}

// commandTax shares CR1's mana-value-scaled recast penalty with commander_zone.
func (b Board) commandTax(id state.ObjID) int32 {
	return cmdrTaxScale * b.Commanders[id].Casts * b.Cards[id].CMC
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
//   - CR1 (the command zone is taxed, CR 903.8): a "cast" whose Obj is a
//     commander currently sitting in its owner's command zone ranks as its
//     ordinary castScore minus cmdrTaxScale*Casts*CMC — the tax priced on
//     the commander's own mana value (see the cmdrTaxScale comment), and
//     an option scoring below zero is NOT cast at all. The tax is per
//     prior cast, so the bot casts its commander while the recast is
//     still worth the mana it costs — a cheap commander is recast many
//     times (a 2/2, CMC 2, scores 38, 28, 18, 8 through its fourth cast
//     and refuses only the fifth), an expensive one is abandoned early (a
//     12/12, CMC 12, scores 78, 18 and refuses its third), and a
//     commander that keeps dying is not re-recruited forever. A commander
//     cast from the HAND scores as an ordinary card — NoTax (a hand cast
//     neither costs {2} nor counts), which is the whole reason the rule
//     is gated on InCommandZone and not on "is a commander". This price
//     is a value judgment on what the recast costs, NOT an affordability
//     check: the engine already refuses what the pool cannot pay
//     (rules/cast.go's commanderTaxFor gates the offer), so this is purely
//     "is the recast still worth bothering with".
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
		if cmdr, ok := b.Commanders[o.Obj]; ok && cmdr.InCommandZone {
			// CR1: price the tax on the commander's mana value, not a flat
			// power-equivalent, so a costly commander is abandoned before a
			// cheap one. CMC 0 (an unreadable commander, should not happen)
			// prices the tax at 0 and so casts at its base score (C5's own
			// degenerate shape); a real commander always carries CMC on both
			// adapter halves (combat.go's census fills it for the command
			// zone, and seat/bot.go's boardFromView the same).
			s -= b.commandTax(o.Obj)
			if s < 0 {
				continue // CR1: the recast has priced itself out — do not cast
			}
		}
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
//     is offered. Colour-aware choice is deliberately deferred: it needs
//     land-entry facts as well as this card's possible production.
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
