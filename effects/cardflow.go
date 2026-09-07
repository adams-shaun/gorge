package effects

import (
	"strconv"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("Draw", effDraw)
	Register("Discard", effDiscard)
	Register("Mill", effMill)
	Register("Dig", effDig)
	Register("Reveal", effReveal)
	Register("RevealHand", effReveal)
	Register("PeekAndReveal", effReveal)
	Register("RearrangeTopOfLibrary", effRearrangeTopOfLibrary)
	Register("NameCard", effNameCard)
}

// zoneOf is a bounds-checked g.Zone. PlayerOf returns a target's raw Player
// field (or, for effNameCard, Ctx.Controller is passed straight through)
// with no validation of its own, and Game.Zone computes
// int(p)*numZones+int(z) and indexes a fixed-size slice without checking
// that p names a real seat -- an out-of-range PlayerID reaches that
// arithmetic and panics with index out of range (Ruling T18-a). Every call
// site in this file that reads a zone to decide what to move goes through
// this rather than g.Zone directly, mirroring count.go's own PlayerID bounds
// check ahead of g.Players[c.Controller]. An invalid seat degrades to nil --
// the same shape as a real, empty zone -- so the primitive simply finds
// nothing there rather than panicking or erroring.
func zoneOf(g *state.Game, z state.Zone, p state.PlayerID) []state.ObjID {
	if int(p) >= len(g.Players) {
		return nil
	}
	return g.Zone(z, p)
}

// DrawFor is exported so the rules package can use the same code path for the
// draw step. Drawing from an empty library is a loss, checked by SBAs.
func DrawFor(h Host, p state.PlayerID) {
	g := h.Game()
	lib := zoneOf(g, state.ZLibrary, p)
	if len(lib) == 0 {
		h.Emit(events.Event{Kind: events.PlayerLost, Player: p, Text: "drew from an empty library"})
		return
	}
	h.Emit(events.Event{Kind: events.Draw, Player: p, Obj: lib[0],
		From: state.ZLibrary, To: state.ZHand, Secret: true})
}

func effDraw(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "NumCards", 1)
	for _, t := range Defined(h, c, sa) {
		p := PlayerOf(h, c, t)
		for i := int32(0); i < n; i++ {
			DrawFor(h, p)
		}
	}
}

// effDiscard moves cards from a player's hand to their graveyard. Which
// cards, and who chooses, is driven by the corpus params that the real
// discard spells use and that this primitive now reads:
//
//   - Mode$ RevealYouChoose (Thoughtseize, Duress): the CASTER — c.Controller,
//     not the discarding player — looks at the target's hand and chooses which
//     card is discarded. This is a real mid-resolution ask: the effect poses a
//     KModes decision over the DiscardValid$-filtered hand, suspends, and on
//     re-entry discards exactly the card Ctx.Discard names (the continuation
//     arm in rules/resolution.go set it from the recorded answer). A host
//     that cannot ask falls back to the deterministic front-card stand-in.
//   - Mode$ TgtChoose (Mind Rot, Faithless Looting, Thirst for Knowledge,
//     Riddlesmith): the ordinary "discard N cards" — the DISCARDING player,
//     p (the target, not the caster), chooses which of their own cards to
//     drop, without the hand being revealed first. Same mid-resolution ask
//     shape as RevealYouChoose, same ResumeKind "discard", but the decision's
//     Player is p and the no-ask fallback takes the front of the FILTERED
//     hand. A hand with fewer eligible cards than NumCards$ discards what it
//     owns and asks nothing (there is no choice to be made), and a hand
//     whose eligible count is at or below NumCards$ likewise resolves
//     deterministically with no question.
//   - Mode$ RevealDiscardAll (Cabal Therapy): a FILTER, not a choice. Every
//     card in the target's hand matching DiscardValid$ is discarded, no ask.
//   - Mode$ absent / Hand / Random / Defined / LookYouChoose / YouChoose /
//     RevealTgtChoose (the cleanup step, Delve-style costs and the wheel
//     family): still the deterministic front-of-hand discard NumCards times —
//     right, because those paths have no player choice to make (or the
//     approximation is elsewhere), and must not become a question.
//
// DiscardValid$ is a Forge filter spec ("Card.nonLand", "Card.NamedCard"),
// evaluated with MatchesSpecFrom (the same resolver effDig uses for
// ChangeValid$). Its default is "Card". The chooser/target split is what keeps
// Thoughtseize from letting the opponent pick their own discard, and what
// keeps a Mind Rot target's own choice from being made by the caster.
func effDiscard(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	mode := sa.Params["Mode"]
	valid := sa.Params["DiscardValid"]
	if valid == "" {
		valid = "Card"
	}
	for _, t := range Defined(h, c, sa) {
		p := PlayerOf(h, c, t)
		hand := zoneOf(g, state.ZHand, p)

		switch mode {
		case "RevealYouChoose":
			// Re-entry: the caster's choice was answered and the continuation
			// set Ctx.Discard to the chosen object(s). Discard exactly those
			// that sit in this target's hand — a single-target spell resolves
			// to one card, and the per-hand filter keeps a stray answer from
			// moving an object that left the hand meanwhile.
			if c.Discard != nil {
				for _, id := range c.Discard {
					if !containsID(hand, id) {
						continue
					}
					h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZHand, To: state.ZGraveyard, Player: p})
				}
				continue
			}
			// First pass: narrow the target's hand to the cards DiscardValid$
			// allows, then ask the CASTER which to discard.
			eligible := make([]state.ObjID, 0, len(hand))
			for _, id := range hand {
				if MatchesSpecFrom(g, valid, id, c.Controller, c.Source) {
					eligible = append(eligible, id)
				}
			}
			if len(eligible) == 0 {
				continue
			}
			n := Num(h, c, sa, "NumCards", 1)
			if n < 1 {
				n = 1
			}
			if int(n) > len(eligible) {
				n = int32(len(eligible))
			}
			opts := make([]decision.Option, 0, len(eligible))
			for _, id := range eligible {
				name := "a card"
				if o := g.Obj(id); o != nil && o.Face() != nil {
					name = o.Face().Name
				}
				opts = append(opts, decision.Option{Index: len(opts), Kind: "discard",
					Label: "Discard " + name, Obj: id, Player: c.Controller})
			}
			d := &decision.Decision{Player: c.Controller, Kind: decision.KModes,
				Min: int(n), Max: int(n), Source: c.Source,
				ResumeKind: "discard", ResumeSA: sa,
				Prompt:  "Choose " + strconv.Itoa(int(n)) + " card(s) to discard",
				Options: opts}
			if h.Ask(d) {
				return // resolution suspended; the answer re-enters with Ctx.Discard set.
			}
			// Fuzz/no-engine host: the deterministic front-card stand-in
			// (R-9), with the Note that records why the richer path did not run.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "discards its first card (no engine host to ask)"})
			if len(hand) > 0 {
				h.Emit(events.Event{Kind: events.MoveZone, Obj: hand[0],
					From: state.ZHand, To: state.ZGraveyard, Player: p})
			}

		case "TgtChoose":
			// Re-entry: the discarding player's choice was answered and the
			// continuation set Ctx.Discard to the chosen object(s). Discard
			// exactly those that sit in this target's hand (a per-hand filter
			// keeps a stray answer from moving an object that left the hand
			// meanwhile).
			if c.Discard != nil {
				for _, id := range c.Discard {
					if !containsID(hand, id) {
						continue
					}
					h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZHand, To: state.ZGraveyard, Player: p})
				}
				continue
			}
			// First pass: narrow the target's hand to the cards DiscardValid$
			// allows. This is the discarding player's own hand, so the choice
			// is presented to p.
			eligible := make([]state.ObjID, 0, len(hand))
			for _, id := range hand {
				if MatchesSpecFrom(g, valid, id, c.Controller, c.Source) {
					eligible = append(eligible, id)
				}
			}
			if len(eligible) == 0 {
				continue
			}
			n := Num(h, c, sa, "NumCards", 1)
			if n < 1 {
				n = 1
			}
			// Only a real choice when there are STRICTLY more eligible cards
			// than must be discarded. A hand with NumCards$ eligible cards (or
			// fewer) must drop all of them with no question: the player could
			// not answer differently, so emitting a decision nobody can
			// meaningfully resolve would just be noise (R-9 contract).
			if int32(len(eligible)) <= n {
				for _, id := range eligible {
					h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZHand, To: state.ZGraveyard, Player: p})
				}
				continue
			}
			opts := make([]decision.Option, 0, len(eligible))
			for _, id := range eligible {
				name := "a card"
				if o := g.Obj(id); o != nil && o.Face() != nil {
					name = o.Face().Name
				}
				opts = append(opts, decision.Option{Index: len(opts), Kind: "discard",
					Label: "Discard " + name, Obj: id, Player: p})
			}
			d := &decision.Decision{Player: p, Kind: decision.KModes,
				Min: int(n), Max: int(n), Source: c.Source,
				ResumeKind: "discard", ResumeSA: sa,
				Prompt:  "Choose " + strconv.Itoa(int(n)) + " card(s) to discard",
				Options: opts}
			if h.Ask(d) {
				return // resolution suspended; the answer re-enters with Ctx.Discard set.
			}
			// Fuzz/no-engine host: the deterministic front-of-ELIGIBLE-hand
			// stand-in (R-9) for the discarding player, with the Note that
			// records why the richer path did not run.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "discards its first card (no engine host to ask)"})
			for i := int32(0); i < n; i++ {
				h.Emit(events.Event{Kind: events.MoveZone, Obj: eligible[i],
					From: state.ZHand, To: state.ZGraveyard, Player: p})
			}

		case "RevealDiscardAll":
			// A FILTER, not a choice (Cabal Therapy): discard every card in
			// the target's hand that DiscardValid$ allows, no matter what
			// NumCards$ says. No ask.
			for _, id := range hand {
				if MatchesSpecFrom(g, valid, id, c.Controller, c.Source) {
					h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZHand, To: state.ZGraveyard, Player: p})
				}
			}

		default:
			// Deterministic discard from the top of hand order. Real discard is
			// a choice, but the clean-up step and Delve-style costs have no
			// player to ask and must stay exactly as they were; "first in hand"
			// is deterministic and adequate there.
			n := Num(h, c, sa, "NumCards", 1)
			for i := int32(0); i < n; i++ {
				// Re-read the hand each iteration (B2): events.remove rebuilds
				// the zone slice rather than mutating it in place, so a hand
				// captured once — as this primitive used to — never sees the
				// card it just moved, and a NumCards$ >= 2 discard emits the
				// SAME front card N times instead of N distinct cards. Reading
				// the zone per iteration is what the original pre-hoist code
				// did, and is what makes the N-card discard honest.
				cur := zoneOf(g, state.ZHand, p)
				if len(cur) == 0 {
					break
				}
				h.Emit(events.Event{Kind: events.MoveZone, Obj: cur[0],
					From: state.ZHand, To: state.ZGraveyard, Player: p})
			}
		}
	}
}

// containsID reports whether id is present in ids.
func containsID(ids []state.ObjID, id state.ObjID) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// effMill moves cards from the top of a player's library straight to their
// graveyard -- Discard's sibling, minus the hand.
func effMill(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "NumCards", 1)
	if n < 0 {
		n = 0
	}
	g := h.Game()
	for _, t := range Defined(h, c, sa) {
		p := PlayerOf(h, c, t)
		for i := int32(0); i < n; i++ {
			lib := zoneOf(g, state.ZLibrary, p)
			if len(lib) == 0 {
				break
			}
			h.Emit(events.Event{Kind: events.MoveZone, Obj: lib[0],
				From: state.ZLibrary, To: state.ZGraveyard, Player: p})
		}
	}
}

// effDig is M1's simplification of Forge's Dig: look at the top DigNum cards
// of Defined$'s library, move up to ChangeNum of the ones matching
// ChangeValid$ (default "Card") to DestinationZone$ (default "Hand"), and
// leave everything else exactly where it already is -- on top of the
// library, in its existing relative order. The brief's own spec names the
// remainder's destination "LibraryPosition2$", but that parameter does not
// exist anywhere in the fetched corpus (the real field there is
// "LibraryPosition$", also left unhandled here); M1 keeps the existing order
// for the untaken cards either way, the same simplification
// RearrangeTopOfLibrary makes for its own remainder.
//
// A real card can also write "ChangeNum$ All" (e.g. Goblin Guide's own Dig)
// to mean every matching card within the DigNum look, with no cap short of
// that. changeNum's own upper bound is already the size of the dug slice, so
// defaulting it to digNum and only overriding that default for a literal or
// SVar ChangeNum$ handles "All" for free: it is simply the case where
// nothing narrows the cap below the number of cards looked at.
func effDig(h Host, c *Ctx, sa *cards.SA) {
	digNum := Num(h, c, sa, "DigNum", 1)
	if digNum < 0 {
		digNum = 0
	}
	changeNum := digNum
	if raw := sa.Params["ChangeNum"]; raw != "" && raw != "All" {
		changeNum = Num(h, c, sa, "ChangeNum", digNum)
	}
	if changeNum < 0 {
		changeNum = 0
	}
	spec := sa.Params["ChangeValid"]
	if spec == "" {
		spec = "Card"
	}
	destName := sa.Params["DestinationZone"]
	if destName == "" {
		destName = "Hand"
	}
	dest := ParseZone(destName)
	g := h.Game()
	for _, t := range Defined(h, c, sa) {
		p := PlayerOf(h, c, t)
		lib := zoneOf(g, state.ZLibrary, p)
		n := digNum
		if int32(len(lib)) < n {
			n = int32(len(lib))
		}
		top := append([]state.ObjID(nil), lib[:n]...)
		moved := int32(0)
		for _, id := range top {
			if moved >= changeNum {
				break
			}
			if !MatchesSpecFrom(g, spec, id, c.Controller, c.Source) {
				continue
			}
			h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
				From: state.ZLibrary, To: dest, Player: p, Secret: true})
			moved++
		}
	}
}

// effReveal backs Reveal, RevealHand and PeekAndReveal, which the brief
// specifies as one row sharing a single Amount param (NumCards, default 1)
// and one behaviour: reveal cards without disturbing them, recorded via a
// non-Secret Note carrying their identities so view projection stops
// redacting them. PeekAndReveal looks at the library; Reveal and RevealHand
// look at hand. (RevealHand's real corpus params reveal the whole hand
// rather than a count; M1 follows the brief's shared NumCards spec for all
// three instead of special-casing that.)
func effReveal(h Host, c *Ctx, sa *cards.SA) {
	amt := Num(h, c, sa, "NumCards", 1)
	if amt < 0 {
		amt = 0
	}
	zone := state.ZHand
	if sa.API == "PeekAndReveal" {
		zone = state.ZLibrary
	}
	g := h.Game()
	for _, t := range Defined(h, c, sa) {
		p := PlayerOf(h, c, t)
		pool := zoneOf(g, zone, p)
		n := amt
		if int32(len(pool)) < n {
			n = int32(len(pool))
		}
		if n == 0 {
			continue
		}
		h.Emit(events.Event{Kind: events.Note, Player: p, Text: "reveals cards",
			IDs: append([]state.ObjID(nil), pool[:n]...)})
	}
}

// effRearrangeTopOfLibrary looks at the top NumCards of Defined$'s library.
// M1 keeps the existing order -- the choice of a new order is Task 20's
// territory -- so the only observable effect is the Note recording what was
// seen.
//
// Unlike effReveal's Note (a deliberate reveal, public to every seat), this
// one is a private LOOK: only p, the library's own owner, may know what sat
// on top. Ruling T23-w makes a Note public by default (view.RedactEvents'
// rule 3 exempts Note entirely, on the theory that a Note IS the engine's
// "tell everyone" channel), so the one Note that must stay private has to
// opt OUT by being Secret -- the same shape rules/engine.go's Shuffle and
// this file's own effDraw already use for their own hidden-zone payloads.
func effRearrangeTopOfLibrary(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "NumCards", 1)
	if n < 0 {
		n = 0
	}
	g := h.Game()
	for _, t := range Defined(h, c, sa) {
		p := PlayerOf(h, c, t)
		lib := zoneOf(g, state.ZLibrary, p)
		k := n
		if int32(len(lib)) < k {
			k = int32(len(lib))
		}
		h.Emit(events.Event{Kind: events.Note, Player: p,
			Text:   "looks at the top of the library, order unchanged",
			IDs:    append([]state.ObjID(nil), lib[:k]...),
			Secret: true})
	}
}

// effNameCard records a card-name choice. The real name is asked at cast
// time and recorded with a Choose event before this ever resolves (plan
// ruling R-6), so a source already carrying ChosenName is a no-op. Without
// one -- a script that uses NameCard outside an ETB replacement -- it names
// the first card in the controller's library, which is at least a
// deterministic, legal name for whatever downstream sub-ability expects
// one, recorded now as the Choose event so the choice survives replay.
func effNameCard(h Host, c *Ctx, sa *cards.SA) {
	if o := h.Game().Obj(c.Source); o != nil && o.ChosenName != "" {
		return
	}
	g := h.Game()
	name := "a card"
	if lib := zoneOf(g, state.ZLibrary, c.Controller); len(lib) > 0 {
		if o := g.Obj(lib[0]); o != nil && o.Face() != nil {
			name = o.Face().Name
		}
	}
	h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "name", Text: name})
}
