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
	// OnBattlefield reports whether this object is currently a permanent on
	// a battlefield (the deciding seat's own). It is the zone signal the
	// casting Card census carries but Castable does not: a graveyard card
	// is also not "castable" without Flashback, and a battlefield permanent
	// is never castable, so Castable alone cannot tell a live mana source
	// from a spent one. chooseLand (and the availability fold behind it)
	// reads OnBattlefield to know which cards are actually producing mana
	// on the battlefield, so a land drop is aimed at a colour the hand
	// still lacks rather than one a permanent already supplies. Both
	// adapter halves fill it from the same zone-membership source (the
	// projected battlefield list on the view half; the ZBattlefield zone
	// walk on the game half), so the two agree.
	OnBattlefield bool
	// InstantSpeed reports whether the card can be cast at instant speed:
	// it is an Instant, or it carries the Flash keyword (a permanent with
	// flash enters at instant speed, and an instant with flash is still an
	// instant). Both halves fill it from the same sources -- the projected
	// CardView.Types/Keywords on the view half, the face's Types and the
	// engine's derived keyword list on the game half -- so the mana reserve
	// (reserve()) prices the same hand whichever host asks. A battlefield
	// permanent that has neither is not instant-speed, which is what keeps
	// the reserve from counting a spell the seat has already cast.
	InstantSpeed bool
	// Produces is what this card's mana abilities add to the pool when a
	// tap-for-mana activation runs them (cards.ManaProduction, plain data
	// -- botpolicy must not import view or rules, Ruling F7). It is filled
	// by both adapter halves from the same source -- the projected
	// CardView.Produces on the view half, cards.Face.ManaProduction on the
	// game half -- so the tap heuristic reads the same production whichever
	// host asks. It lets chooseTap pick a source that produces a colour a cast
	// needs and spend the least flexible one first. It also describes a land
	// card while it is still in hand, and it is the fact the land-drop ranking
	// (chooseLand) reads to pick a land in the colour the hand still needs.
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
//   - C7 (the mana reserve, B2): the bot keeps the mana pool at or above the
//     cost of the cheapest instant-speed card it holds (an Instant, or a
//     permanent with Flash; reserve() is 0 when it holds none, making C7
//     inert) by PREFERRING a cast that leaves that reserve over a
//     comparable one that empties it -- the reserve is a score bonus a
//     reserve-keeping cast earns, so among equivalent cards the bot holds
//     mana for an instant-speed play, but a clearly better cast (a strong
//     creature, an unprotected commander) is still made even if it spends
//     the reserve. A hard block here made the bot decline casting its good
//     hands and sit on mana the phase threw away, so C7 is deliberately a
//     preference, not a refusal. A command-zone commander cast is priced by
//     value alone, never held for the reserve.
//
// Like the target and combat branches it consumes no rng: the pick is a
// pure function of the offered options and the board facts.
func (b Board) chooseCast(d *decision.Decision) int {
	best := -1
	var bestScore int32 = -1
	// C7 (the mana reserve, B2): the reserve is the cost of the cheapest
	// instant-speed card the seat holds (0 when it holds none, in which case
	// C7 is inert). It is a PREFERENCE, not a hard block: a cast that would
	// leave the pool at or above the reserve scores reserve points higher, so
	// among comparable cards the bot keeps mana for an instant-speed play
	// rather than emptying the pool -- but a cast that is clearly better
	// (a strong creature, an unprotected commander) still gets made even if
	// it spends the reserve, because refusing the seat's best play to hoard
	// mana is not the point (and is exactly what a deck that never casts its
	// commander does). A hard block, by contrast, made the bot decline
	// casting its good hands and sat passively on mana the phase threw away.
	res := b.reserve()
	for _, o := range d.Options {
		if o.Kind != "cast" {
			continue
		}
		// CR1: price the command-zone tax on the commander's mana value, not
		// a flat power-equivalent, so a costly commander is abandoned before
		// a cheap one. CMC 0 (an unreadable commander, should not happen)
		// prices the tax at 0 and so casts at its base score (C5's own
		// degenerate shape); a real commander always carries CMC on both
		// adapter halves (combat.go's census fills it for the command zone,
		// and seat/bot.go's boardFromView the same). A command-zone cast is
		// also never held for the reserve's sake (the deck must be able to
		// cast its commander), so it is priced by value alone.
		inCmd := b.Commanders[o.Obj].InCommandZone
		s := b.castScore(o)
		if inCmd {
			s -= b.commandTax(o.Obj)
			if s < 0 {
				continue // CR1: the recast has priced itself out — do not cast
			}
		} else if res > 0 && b.Pool.Total()-b.castCost(o.Obj, b.Cards[o.Obj]) >= res {
			s += res * reserveBonusScale // C7: prefer a cast that keeps the reserve
		}
		if best == -1 || s > bestScore || (s == bestScore && o.Index < best) {
			best, bestScore = o.Index, s
		}
	}
	return best
}

// colourNeed is the aggregate coloured-pip demand of the seat's castable
// non-land cards -- the colours the hand wants to be able to pay this turn.
// It is the land-drop greedy's "what the hand wants to cast" side, expressed
// as per-colour pip counts so a land producing a wanted colour lands where
// the shortfall is. A card the seat cannot cast (a battlefield permanent, a
// non-Flashback graveyard card) contributes nothing: a land drop cannot
// enable it. A land (CMC 0) contributes nothing -- it has no pips.
func (b Board) colourNeed() [5]int32 {
	var need [5]int32
	for _, c := range b.Cards {
		if !c.Castable || c.CMC <= 0 {
			continue
		}
		pips := colourPips(c.ManaCost)
		for i := 0; i < 5; i++ {
			need[i] += pips[i]
		}
	}
	return need
}

// availableColours is the coloured mana the seat can already rely on: the
// current pool plus the guaranteed production of every source already on the
// battlefield (OnBattlefield). It is the land-drop greedy's "which colours
// are already available" side. Only demonstrably-produced colours count: an
// Any production reports its colourless amount and no colour slot, and an
// Indeterminate-amount source contributes 0 to every colour slot (detected
// via Colour, never by reading the Indeterminate flag), so a conditional
// source never makes a colour look already covered. The candidate lands
// themselves sit in the hand (OnBattlefield false) and are not counted, so
// their colour is exactly the marginal value chooseLand scores.
func (b Board) availableColours() [5]int32 {
	var avail [5]int32
	for i := 0; i < 5; i++ {
		avail[i] = b.Pool[i]
	}
	for _, c := range b.Cards {
		if !c.OnBattlefield {
			continue
		}
		for i := 0; i < 5; i++ {
			if c.Produces.Colour[i] > 0 {
				avail[i] += c.Produces.Colour[i]
			}
		}
	}
	return avail
}

// reserveBonusScale prices the C7 reserve preference: a cast that keeps the
// pool at or above the reserve earns reserve*reserveBonusScale points, so
// among comparable cards the bot holds mana for an instant-speed play
// rather than emptying the pool. It is a pricing constant like the target
// tier offsets -- not the reserve amount itself, which is evidence-driven
// (cheapest instant-speed card in hand) -- and it is sized so a
// reserve-keeping cheap card beats a comparable draining one (5 points per
// reserved mana) but never outranks a clearly better play (a creature's 30+
// base, an unprotected commander), which is what keeps the bot from sitting
// on its good hands forever.
const reserveBonusScale int32 = 5

// reserve is the mana the policy keeps unspent in its own main phase so it
// can answer on another seat's turn. It is evidence-driven, not a constant:
// the cheapest castable card in hand (or the command zone) that can be cast
// at instant speed -- an Instant, or a permanent with Flash -- its converted
// cost, or 0 when the seat holds no instant-speed castable card at all (in
// which case nothing is reserved and the policy behaves exactly as it did
// before). The rule reads this in one sentence: in its own main phase the
// bot prefers a cast that leaves the mana pool at or above the cost of the
// cheapest instant-speed card it holds over a comparable cast that empties
// it, but a clearly better cast (a strong creature, an unprotected
// commander) is still made even if it spends that reserve.
func (b Board) reserve() int32 {
	var min int32 = -1
	for _, c := range b.Cards {
		if !c.Castable || !c.InstantSpeed || c.CMC <= 0 {
			continue
		}
		if min == -1 || c.CMC < min {
			min = c.CMC
		}
	}
	if min < 0 {
		return 0
	}
	return min
}

// castCost prices one card's cast: its printed converted cost plus the CR
// 903.8 command-zone tax ({2} per prior command-zone cast) when the card is
// its owner's commander sitting in the command zone -- the same number
// poolPays reads, so the reserve and the tap gate price the same cast.
func (b Board) castCost(id state.ObjID, c Card) int32 {
	cost := c.CMC
	if cmdr, ok := b.Commanders[id]; ok && cmdr.InCommandZone {
		cost += 2 * cmdr.Casts
	}
	return cost
}

// chooseLand is the KPriority land-drop ranking: it picks ONE of the
// offered "play_land" options, or -1 when none is offered.
//
//   - L1 (colour-first): a land is ranked by how much of the hand's unmet
//     colour need it covers. The unmet need (colourNeed minus
//     availableColours) is the per-colour shortfall the existing mana base
//     and pool leave; a candidate land's coverage is the amount of that
//     shortfall its own production fills. A land that produces a colour the
//     hand still needs outranks one that does not, so a hand needing {U}{U}
//     keeps an Island rather than a Plains, and a dual feeding a wanted
//     colour beats a basic of an already-covered one. The greedy is pip-
//     coverage: it aims the single land drop at the largest colour gap, the
//     cheap proxy for "the land that unlocks the most castable cards" without
//     walking the whole hand against the whole mana base per drop.
//   - L2 (reliability on a tie): two lands of equal colour coverage tie on
//     basic-ness, the reliable basic winning, because a basic unconditionally
//     produces its colour and never enters tapped nor demands a condition the
//     policy cannot read; basic-ness is only ever a tiebreak now, never the
//     primary criterion. A colourless or non-producing land thus still ranks,
//     just below any land that covers a real colour.
//   - L3 (flexibility kept): a tied pair of equally covering, equally basic
//     lands breaks toward the one with the FEWEST distinct colours -- the
//     least-flexible is played and the flexible source is kept in hand, the
//     same "spend the least flexible first" ergonomics the tap gate uses.
//   - L4 (deterministic tie): two lands still equal (empty coverage, equal
//     basic-ness, equal flexibility) tie on option index.
//
// It consumes no rng and ranges no map: the pick is a pure function of the
// offered options plus the board facts, so no map iteration order reaches it.
func (b Board) chooseLand(d *decision.Decision) int {
	need := b.colourNeed()
	avail := b.availableColours()
	var unmet [5]int32
	for i := 0; i < 5; i++ {
		unmet[i] = need[i] - avail[i]
		if unmet[i] < 0 {
			unmet[i] = 0
		}
	}
	best := -1
	bestCover := int32(-1)
	bestBasic := false
	bestFlex := 0
	for _, o := range d.Options {
		if o.Kind != "play_land" {
			continue
		}
		c := b.Cards[o.Obj] // zero facts read as no coverage/nonbasic/flex 0, never a crash
		prod := c.Produces
		var cover int32
		for i := 0; i < 5; i++ {
			if unmet[i] > 0 {
				take := prod.Colour[i]
				if take > unmet[i] {
					take = unmet[i]
				}
				cover += take
			}
		}
		basic := c.Basic
		flex := prod.DistinctColours()
		if best == -1 || cover > bestCover ||
			(cover == bestCover && basic && !bestBasic) ||
			(cover == bestCover && basic == bestBasic && flex < bestFlex) ||
			(cover == bestCover && basic == bestBasic && flex == bestFlex && o.Index < best) {
			best, bestCover, bestBasic, bestFlex = o.Index, cover, basic, flex
		}
	}
	return best
}
