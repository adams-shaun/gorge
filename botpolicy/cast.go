package botpolicy

import (
	"strconv"
	"strings"

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
// host asked. A field no rule reads would be untested surface; these five
// are the ones chooseCast, chooseLand and the ability ranking branch on.
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
	// Types is the printed type line as a single space-joined string
	// ("Instant", "Basic Land Island", "Enchantment Aura") -- the exact
	// shape the projected CardView.Types already carries and the join of
	// the face's own Types slice on the game half, so the two adapter
	// halves produce the identical string and the Card stays comparable
	// (the adapter-parity test compares whole Cards). classifyCard reads
	// it alongside Text to separate a one-shot from a permanent, but the
	// classification is coarse: it cannot tell a removal from a counterspell
	// off a type line alone, which is why Text is the primary signal.
	Types string
	// Text is the card's oracle text, the same string both adapter halves
	// carry (boardFromView lifts it off the projected CardView.Text, which
	// the view fills from the face's Oracle; BoardFromGame reads the face's
	// Oracle directly). It is what classifyCard matches to place a non-
	// creature spell in a class -- the one readable signal that separates a
	// counterspell from a removal from a burn when their type lines (and so
	// their costs) are identical.
	Text string
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
	c := b.Cards[o.Obj]
	var s int32
	if c.Creature {
		s = 30 + c.Power*4
	} else {
		// A non-creature is ranked by what it does first, by its cost
		// second: the CMC the bot actually spends plus the value of the
		// broad class the oracle text places it in (see classifyCard). Cost
		// alone made a 3-mana counterspell and a 3-mana burn
		// indistinguishable, so the tie fell to option index -- the
		// first-offered spell, whatever it did. The class term is
		// deliberately small relative to the creature term below (a 1/1 at
		// 34 still outranks the best non-creature this policy can read), so
		// C1's board-over-one-shots preference survives.
		s = c.CMC + classValue(classifyCard(c))
	}
	switch o.Mode {
	case "kicked", "surged":
		s += 6
	case "flashback", "miracle":
		s += 4
	}
	return s
}

// spellClass is the broad effect class a non-creature cast is ranked in,
// the coarse read classifyCard produces. It is NOT a card catalog: it is a
// handful of classes whose worth a casting bot can reason about, matched by
// oracle phrase.
type spellClass int

const (
	classPlain spellClass = iota // nothing matched: ranked by cost alone (C5)
	classRemoval
	classBurn
	classDraw
	classRamp
	classCounterspell
)

// classValue is the score term a class adds to a non-creature's CMC. The
// ordering the casting policy wants: a counterspell (stops whatever the
// opponent is about to do) and a removal (answers the threat already on the
// board) rank above card advantage, above burn, above ramp, above a spell
// the classifier cannot place. All are small next to the creature term so
// C1 (board over one-shots) still keeps a creature ahead of every
// non-creature the policy can read -- this term only decides the order
// WITHIN the non-creature pile, which is exactly the tie task dp3 the audit
// found degenerating to option index.
func classValue(k spellClass) int32 {
	switch k {
	case classCounterspell:
		return 8
	case classRemoval:
		return 6
	case classDraw:
		return 4
	case classBurn:
		return 2
	case classRamp:
		return 2
	default:
		return 0
	}
}

// classifyCard is the effect-class read a non-creature cast is ranked by.
// It reads exactly what both adapter halves already carry for the same card
// -- the printed type words (Card.Types), the oracle text (Card.Text) -- and
// returns the broad class the spell falls into, or classPlain when nothing
// matches.
//
// It deliberately does not try to understand arbitrary card effects. It
// matches a small set of oracle phrases that identify the classes the
// casting order cares about, classifies by what it can read, and leaves what
// it cannot read as classPlain, ranked by cost alone. What it cannot see:
//
//   - it cannot read game state, so it cannot tell a removal that only hits
//     a 5/5 from one that whiffs, nor whether a burn is at lethal range;
//   - it cannot size an effect, so a 1-mana burn and a 5-mana burn are both
//     classBurn (their costs separate them);
//   - a plain artifact or enchantment (a 5-cost bomb and a 5-cost mana
//     rock) read alike as classPlain, ranked by cost;
//   - a card with no oracle text, or one whose phrases it does not match,
//     reads classPlain.
//
// The phrases are matched on the lowercased oracle text and are deliberately
// loose: "destroy" and "exile" identify removal whether the effect is a
// spell, a trigger or an activated ability, because the class the casting
// policy cares about is "this removes something the opponent controls".
func classifyCard(c Card) spellClass {
	text := strings.ToLower(c.Text)
	if text == "" {
		return classPlain
	}
	has := func(subs ...string) bool {
		for _, sub := range subs {
			if strings.Contains(text, sub) {
				return true
			}
		}
		return false
	}
	switch {
	case has("counter target", "counter that", "counter a ", "counter the",
		"counter it", "counter up to", "counters target", "counters the",
		"counter spell", "counterspell"):
		return classCounterspell
	case has("destroy", "exile", "return target", "return up to",
		"sacrifice target", "gets -", "-x/-x", "fight target",
		"damage equal to", "damage to target creature"):
		return classRemoval
	case has("damage to any target", "damage to target player",
		"damage to target opponent", "damage to target creature or player",
		"damage to target"):
		return classBurn
	case has("draw a card", "draws a card", "draw two", "draw three",
		"draw x cards", "draw cards", "draw that many", "draw :", "draw "):
		return classDraw
	case has("search your library for a", "search your library for up to",
		"basic land card", "to your mana pool", "produce mana",
		"mana pool", "untap"):
		return classRamp
	}
	return classPlain
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
//     ordinary castScore minus 20 per previous command-zone cast, and an
//     option scoring below zero is NOT cast at all. The tax is per prior
//     cast — the extra {2} this very cast pays — so the bot casts its
//     commander while the creature is still worth the growing outlay, and
//     stops when the exchange turns losing: a 4/4 is cast up to its third
//     command-zone cast (46, 26, 6), a 2/2 up to its second, and a
//     commander that keeps dying is not re-recruited forever. A commander
//     cast from the HAND scores as an ordinary card — NoTax (a hand cast
//     neither costs {2} nor counts), which is the whole reason the rule is
//     gated on InCommandZone and not on "is a commander". The 20-point
//     price is five power on the creature scale (30 + 4P), so the first
//     {2} tax of a 3/3 leaves it worth about a 2/2, and the scale lands
//     the second recast of a 4/4 just above any spell the bot can read
//     (26 vs a one-shot's mana value).
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
			s -= 20 * cmdr.Casts
			if s < 0 {
				continue // CR1: the tax has made this a losing exchange — do not cast
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
