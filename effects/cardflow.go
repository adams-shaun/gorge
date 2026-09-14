package effects

import (
	"strconv"
	"strings"

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
	Register("Scry", effScry)
	Register("Surveil", effSurveil)
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
	// fx42: capture the answered discard choice into a local and clear
	// c.Discard before the target loop. The answer must stay scoped to the
	// discard primitive that asked: a DISCARD reached below this one in the
	// same walk (this effect's SubAbility$ chain) must pose its own ask
	// instead of inheriting this one's answered cards. Capturing first keeps
	// the load-bearing multi-target behaviour intact — every target of a
	// multi-target discard sees the SAME answered list, which is exactly what
	// the old per-target c.Discard read produced. Ctx.Discard's only reader is
	// this primitive, so clearing here is safe.
	answers := c.Discard
	c.Discard = nil
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
			if answers != nil {
				for _, id := range answers {
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
				if MatchesSpecCtx(g, valid, id, c.SpecContext(c.Controller)) {
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
			if answers != nil {
				for _, id := range answers {
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
				if MatchesSpecCtx(g, valid, id, c.SpecContext(c.Controller)) {
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
				if MatchesSpecCtx(g, valid, id, c.SpecContext(c.Controller)) {
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

// effDig implements Forge's Dig: look at the top DigNum cards of Defined$'s
// library, move up to ChangeNum of the ones matching ChangeValid$ (default
// "Card") to DestinationZone$ (default "Hand"), and leave everything else
// exactly where it already is -- on top of the library, in its existing
// relative order (the remainder-ordering decision is a separate, still-open
// ask; see the row's end). The brief's own spec names the remainder's
// destination "LibraryPosition2$", but that parameter does not exist anywhere
// in the fetched corpus (the real field there is "LibraryPosition$", also
// left unhandled here); M1 keeps the existing order for the untaken cards
// either way, the same simplification RearrangeTopOfLibrary makes for its own
// remainder.
//
// A real card can also write "ChangeNum$ All" (e.g. Goblin Guide's own Dig)
// to mean every matching card within the DigNum look, with no cap short of
// that. changeNum's own upper bound is already the size of the dug slice, so
// defaulting it to digNum and only overriding that default for a literal or
// SVar ChangeNum$ handles "All" for free: it is simply the case where
// nothing narrows the cap below the number of cards looked at.
//
// The look-and-take ask (dig1): where the top DigNum window holds STRICTLY
// more ChangeValid$-eligible cards than ChangeNum, the pick is a real
// decision and effDig poses it -- the same strict-supersets rule effDiscard's
// TgtChoose arm already uses, so a decision nobody could answer differently
// is never emitted. ChangeNum$ 0 (the corpus's three reveal-machinery digs:
// birthing_ritual, sanity_grinding, stomping_slabs) takes nothing, so the
// ask gate also requires changeNum > 0 -- otherwise a zero cap with any
// eligible card would pose a Min==Max==0 KChoose whose only legal answer is
// the empty one, a decision nobody could answer differently by definition. The look is recorded first as a Secret Note carrying the
// window's ids (only the library's owner may know what sat on top; the same
// channel effRearrangeTopOfLibrary uses, with the ids added so the owner's
// client can render what was seen -- view/redact.go rule (1) passes a Secret
// event's payload to its own Player and strips it from everyone else). The
// decision is a KChoose over the ELIGIBLE cards only, in library order: an
// ineligible card must not be pickable, so it is not offered (the window
// itself is on the look Note; the prompt names the card text). Min honours
// Optional$ -- 0 when the take is optional, ChangeNum when it is not -- and
// Max is ChangeNum. The answer re-enters through ResumeKind "dig" with
// Ctx.Dig/DigDone set (rules/resolution.go), scoped to this primitive like
// every other Ctx answer field.
//
// A host that cannot answer (the fuzz/no-engine stand-in, R-9) and the
// no-choice path (eligible <= ChangeNum) keep M1's silent behaviour
// deterministically: the first ChangeNum eligible cards in zone order move,
// the rest stay exactly where they are. On the no-choice path nothing new is
// emitted at all, so a game that never reaches a strict-superset Dig replays
// byte-identically to the pre-dig1 engine.
//
// Still unread here (each a real divergence, named in AGENTS.md's Dig row):
// Optional$ on the NO-CHOICE path (eligible <= ChangeNum still takes all
// eligible; ChangeNum$ 0 takes nothing silently, correctly), the remainder-ordering decision ("the rest on the bottom in any
// order"; RestRandomOrder$), DestinationZone2$, LibraryPosition$, Reveal$,
// ForceRevealToController$ (only the ask path records the look at all),
// Choser$ (the opponent-chooses planeswalker shape) and ChangeNum$ Any
// (the "Any" value falls into the take-all default, so it never asks).
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
	optional := sa.Params["Optional"] == "True"
	// fx42 scoping: capture and clear the answered pick BEFORE the target
	// loop, so every target sees the one answer (a multi-target Defined$ asks
	// only the first library -- the later targets' windows cannot contain the
	// answered ids) and a nested Dig below this walk poses its own ask.
	digAns := c.Dig
	digDone := c.DigDone
	c.Dig, c.DigDone = nil, false
	g := h.Game()
	for _, t := range Defined(h, c, sa) {
		p := PlayerOf(h, c, t)
		lib := zoneOf(g, state.ZLibrary, p)
		n := digNum
		if int32(len(lib)) < n {
			n = int32(len(lib))
		}
		top := append([]state.ObjID(nil), lib[:n]...)
		if digDone {
			// Re-entry: move exactly the answered cards that still sit in
			// THIS target's window (a per-window filter keeps a stray answer
			// from moving an object that left the window meanwhile), in the
			// player's answer order.
			for _, id := range digAns {
				if !containsID(top, id) {
					continue
				}
				h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
					From: state.ZLibrary, To: dest, Player: p, Secret: true})
			}
			continue
		}
		eligible := make([]state.ObjID, 0, len(top))
		for _, id := range top {
			if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				eligible = append(eligible, id)
			}
		}
		if changeNum > 0 && int32(len(eligible)) > changeNum {
			// A real choice: record the look, then ask the library's owner.
			h.Emit(events.Event{Kind: events.Note, Player: p,
				Text: "looks at the top of the library", IDs: top, Secret: true})
			minv := int32(0)
			if !optional {
				minv = changeNum
			}
			verb := "you may put up to "
			if !optional {
				verb = "put "
			}
			d := &decision.Decision{Player: p, Kind: decision.KChoose,
				Min:        int(minv),
				Max:        int(changeNum),
				Source:     c.Source,
				ResumeKind: "dig",
				ResumeSA:   sa,
				Prompt:     "Look at the top " + strconv.Itoa(int(n)) + " card(s) of your library: " + verb + strconv.Itoa(int(changeNum)) + " matching card(s) into " + digDestPhrase(dest)}
			for _, id := range eligible {
				name := "a card"
				if o := g.Obj(id); o != nil && o.Face() != nil {
					name = o.Face().Name
				}
				d.Options = append(d.Options, decision.Option{Index: len(d.Options),
					Kind: "dig", Label: name, Obj: id, Player: p})
			}
			if h.Ask(d) {
				return // resolution suspended; the answer re-enters with Ctx.Dig set.
			}
			// Fuzz/no-engine host: the deterministic stand-in (R-9) keeps
			// today's behaviour -- the first ChangeNum eligible cards in zone
			// order -- with the Note that records why the richer path did
			// not run.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: p,
				Text: "takes the first matching card(s) (no engine host to ask)", Secret: true})
			for i := int32(0); i < changeNum && i < int32(len(eligible)); i++ {
				h.Emit(events.Event{Kind: events.MoveZone, Obj: eligible[i],
					From: state.ZLibrary, To: dest, Player: p, Secret: true})
			}
			continue
		}
		// No choice to ask about: M1's silent behaviour, unchanged, and no
		// new event of any kind, so games that never reach a strict-superset
		// Dig replay byte-identically to the pre-dig1 engine.
		moved := int32(0)
		for _, id := range top {
			if moved >= changeNum {
				break
			}
			if !MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				continue
			}
			h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
				From: state.ZLibrary, To: dest, Player: p, Secret: true})
			moved++
		}
	}
}

// digDestPhrase names the take's destination in the human-readable prompt;
// it is Dig's own phrasing (the picked card GOES to the destination, unlike
// KArrange's Kind which names pile B's), kept separate from
// destinationPhrase so the two vocabularies cannot drift into each other.
func digDestPhrase(dest state.Zone) string {
	switch dest {
	case state.ZHand:
		return "your hand"
	case state.ZGraveyard:
		return "your graveyard"
	case state.ZExile:
		return "exile"
	case state.ZBattlefield:
		return "the battlefield"
	case state.ZLibrary:
		return "the top of your library"
	default:
		return "its destination"
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
	// RevealOptional$ on the peek shape (task fb-3f1cc033, Delver of
	// Secrets): the peeking player is asked whether to reveal before the
	// Note goes out. The ask is the same mid-resolution vocabulary every
	// other asking primitive uses — KChoose yes/no with a ResumeKind, the
	// answer re-entering effReveal through rules' resumeResolution with
	// Ctx.RevealOpt set. A host that cannot ask (an effects-package double,
	// fuzz) keeps the pre-ask behaviour: the mandatory reveal, as the
	// deterministic fallback (the same R-9 degradation Scry/Surveil
	// carry). Out of scope, deliberately: RevealOptional$ on the non-peek
	// shapes (Reveal/RevealHand) and every other unread reveal param
	// (PeekAmount$, RevealValid$, NoReveal$/NoPeek$) — see the report's
	// Issues section.
	answer := c.RevealOpt
	c.RevealOpt = "" // fx42 scoping: consumed once; a nested peek poses its own ask
	optional := sa.API == "PeekAndReveal" &&
		strings.EqualFold(strings.TrimSpace(sa.Params["RevealOptional"]), "True")
	remember := strings.EqualFold(strings.TrimSpace(sa.Params["RememberRevealed"]), "True")
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
		if optional && answer == "" {
			// The ask must carry WHAT is being revealed: the peeking player is
			// deciding whether to reveal a card only they can see, and the
			// library is not projected to that seat (view exposes only
			// LibrarySize), so a count-only prompt asks a blind question.
			// The card names go into the prompt and the yes option's label,
			// and the top card rides the option's Obj — the same private
			// channel the hidden-library "search" options use (view.project
			// attaches a decision only to its own Decision.Player, so this
			// payload reaches the peeking seat alone; even an Omniscient
			// spectator gets no decision).
			names := make([]string, 0, n)
			for _, id := range pool[:n] {
				if o := g.Obj(id); o != nil && o.Face() != nil {
					names = append(names, o.Face().Name)
				}
			}
			prompt := "Reveal the top " + strconv.Itoa(int(n)) + " card(s) of your library?"
			if len(names) > 0 {
				prompt = "Reveal the top " + strconv.Itoa(int(n)) + " card(s) of your library — " + strings.Join(names, ", ") + "?"
			}
			yesLabel := "Yes — reveal"
			if len(names) == 1 {
				yesLabel = "Yes — reveal " + names[0]
			}
			d := &decision.Decision{Player: p, Kind: decision.KChoose, Min: 1, Max: 1,
				ResumeKind: "reveal_optional", ResumeSA: sa, Source: c.Source,
				Prompt: prompt,
				Options: []decision.Option{
					{Index: 0, Kind: "yes", Label: yesLabel, Obj: pool[0], Player: p},
					{Index: 1, Kind: "no", Label: "No", Player: p},
				}}
			if h.Ask(d) {
				return
			}
			// h.Ask false — no host to ask (R-9): fall through to the
			// mandatory reveal below, deterministic run to run.
		}
		if optional && answer == "no" {
			// Declined: no Note, and RememberRevealed$ finds nothing —
			// a chained gate (Delver's ConditionDefined$ Remembered)
			// correctly does not fire. The walk continues.
			continue
		}
		revealed := append([]state.ObjID(nil), pool[:n]...)
		// No Text: the Note's payload is the ids, and view.Describe renders
		// them ("player 0 reveals Mountain #82") — defect 1's second half,
		// the client's only data path for hidden-zone ids in a reveal. A
		// Text-carrying Note would need the names baked in at emit time,
		// duplicating Describe's obj() naming; an empty Text with ids keeps
		// the naming in one place. Ruling T23-w still passes the Note
		// through RedactEvents unchanged (it is non-Secret).
		h.Emit(events.Event{Kind: events.Note, Player: p, IDs: revealed})
		if remember {
			// RememberRevealed$ (task fb-3f1cc033): the revealed cards join
			// the walk's Remembered set, where a chained ConditionDefined$
			// Remembered gate (Delver's transform) reads them. Fresh backing
			// array: on an ability resume Ctx.Remembered aliases the stack
			// object's own Remembered slice, and appending in place would
			// write shared state without an event. Measured at the corpus
			// pin: of the 67 PeekAndReveal+RememberRevealed SVar lines, 43
			// have downstream subs that read Remembered — all of them
			// condition gates or Defined$ Remembered bodies that Forge
			// itself intends to see the reveal (the Kinship family), so
			// inheriting the reveal here is the semantics, not a leak.
			next := make([]state.Target, 0, len(c.Remembered)+len(revealed))
			next = append(next, c.Remembered...)
			for _, id := range revealed {
				next = append(next, state.Target{Obj: id})
			}
			c.Remembered = next
		}
	}
}

// effRearrangeTopOfLibrary looks at the top NumCards of Defined$'s library
// and poses a KArrange decision over them: the player picks the order, and
// rules' handleArrange applies it as an events.LibraryOrder (Ruling J0/J1
// - the one general decision shape Scry, Surveil and Dig later share).
//
// Unlike effReveal's Note (a deliberate reveal, public to every seat), the
// private-look record is a Secret Note: only p, the library's own owner, may
// know what sat on top. Ruling T23-w makes a Note public by default
// (view.RedactEvents' rule 3 exempts Note entirely, on the theory that a
// Note IS the engine's "tell everyone" channel), so the one Note that must
// stay private has to opt OUT by being Secret -- the same shape
// rules/engine.go's Shuffle and this file's own effDraw already use for
// their own hidden-zone payloads. The Note records the look, not the order;
// the order that follows is the player's to choose.
//
// Min == Max == k (pile B is empty for a full reorder), one option per top
// card in top-down order, each Option.Kind "bottom" (nothing goes there for
// a reorder, but the vocabulary stays uniform so Scry/Surveil reuse it
// unchanged). Decision.Player is p, the library's owner — the player who is
// looking at and reordering their own top cards.
func effRearrangeTopOfLibrary(h Host, c *Ctx, sa *cards.SA) {
	// Re-entry after rules' handleArrange applied the answered KArrange and
	// emitted the LibraryOrder event: this pass must only let the resolution
	// continue (the chained SubAbility$ runs), not re-ask or re-emit. Clear
	// the marker so a nested arrange — the fx42 class of leak — cannot read
	// an outer arrange's "done".
	if c.Arrange {
		c.Arrange = false
		return
	}
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
			Text: "looks at the top of the library", Secret: true})
		d := &decision.Decision{Player: p, Kind: decision.KArrange,
			Min:        int(k),
			Max:        int(k),
			Source:     c.Source,
			ResumeKind: "arrange",
			ResumeSA:   sa,
			Prompt:     "Rearrange the top " + strconv.Itoa(int(k)) + " card(s); the first card you pick goes on top"}
		for i := int32(0); i < k; i++ {
			name := "a card"
			if o := g.Obj(lib[i]); o != nil && o.Face() != nil {
				name = o.Face().Name
			}
			d.Options = append(d.Options, decision.Option{Index: int(i),
				Kind: "bottom", Label: name, Obj: lib[i], Player: p})
		}
		if h.Ask(d) {
			return // resolution suspended; the answer re-enters with Ctx.Arrange set.
		}
		// Fuzz/no-engine host: the deterministic stand-in keeps the existing
		// order -- pile A = the offered options in offered order (J3) -- with
		// the LibraryOrder recording that the order was (re)set unchanged.
		h.Emit(events.Event{Kind: events.LibraryOrder, Player: p,
			IDs:    append([]state.ObjID(nil), lib...),
			Secret: true})
	}
}

// effScry implements the Scry prompt API (CR 701.18): look at the top
// ScryNum$ cards of Defined$'s library, put any number of them -- the ones
// the player does NOT pick -- on the BOTTOM of the library in the order they
// were offered, and the rest back on top in the order the player picks.
// It is the KArrange ask with Min 0, Max N and Option.Kind "bottom".
//
// The look is recorded as a Secret Note (the player alone may know what sat
// on top), then the answer is applied by rules' handleArrange, which routes
// the unchosen pile B to the destination named by the shared Kind
// ("bottom"). Re-entry after handleArrange set Ctx.Arrange must only let
// the chained SubAbility$ run, never re-ask -- the same done-marker
// discipline effRearrangeTopOfLibrary uses. The no-host stand-in (R-9)
// keeps every card on top in its existing order (pile B empty), which is
// narrower than the card text but deterministic.
func effScry(h Host, c *Ctx, sa *cards.SA) {
	effLookAndArrange(h, c, sa, "ScryNum", "bottom", "Scry")
}

// effSurveil implements the Surveil prompt API (CR 701.42): look at the top
// Amount$ cards of Defined$'s library, put any number of them -- the ones
// the player does NOT pick -- into the GRAVEYARD (surveil has no bottom
// pile; the fsv1 survey's bottom-pile reading is wrong and this code follows
// CR 701.42), and the rest back on top in the order the player picks. It is
// the KArrange ask with Min 0, Max N and Option.Kind "graveyard".
//
// Re-entry and the no-host stand-in are exactly effScry's (the same shared
// helper): the stand-in puts nothing in the graveyard, which is narrower
// than the card text but deterministic.
func effSurveil(h Host, c *Ctx, sa *cards.SA) {
	effLookAndArrange(h, c, sa, "Amount", "graveyard", "Surveil")
}

// effLookAndArrange is the shared KArrange body behind effScry and
// effSurveil: read the count (ScryNum$ / Amount$, default 1) through Num,
// resolve Defined$ (default = the ability's source, hence its controller),
// and pose one KArrange decision per target library over the top min(N,
// len(lib)) cards. The unchosen pile B's destination is the shared Option.Kind
// passed in; only that differs between the two primitives.
func effLookAndArrange(h Host, c *Ctx, sa *cards.SA, numKey, kind, verb string) {
	// Re-entry after rules' handleArrange applied the answered KArrange and
	// emitted the LibraryOrder event: this pass must only let the resolution
	// continue (the chained SubAbility$ runs), not re-ask or re-emit.
	if c.Arrange {
		c.Arrange = false
		return
	}
	n := Num(h, c, sa, numKey, 1)
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
			Text: "looks at the top of the library", Secret: true})
		d := &decision.Decision{Player: p, Kind: decision.KArrange,
			Min:        0,
			Max:        int(k),
			Source:     c.Source,
			ResumeKind: "arrange",
			ResumeSA:   sa,
			Prompt:     verb + " " + strconv.Itoa(int(k)) + ": pick the cards to keep on top, in order; the rest go to " + destinationPhrase(kind)}
		for i := int32(0); i < k; i++ {
			name := "a card"
			if o := g.Obj(lib[i]); o != nil && o.Face() != nil {
				name = o.Face().Name
			}
			d.Options = append(d.Options, decision.Option{Index: int(i),
				Kind: kind, Label: name, Obj: lib[i], Player: p})
		}
		if h.Ask(d) {
			return // resolution suspended; the answer re-enters with Ctx.Arrange set.
		}
		// Fuzz/no-engine host: the deterministic stand-in keeps every card
		// on top in its existing order (pile B empty for a Scry, nothing to
		// the graveyard for a Surveil), with the LibraryOrder recording that
		// the order was (re)set unchanged.
		h.Emit(events.Event{Kind: events.LibraryOrder, Player: p,
			IDs:    append([]state.ObjID(nil), lib...),
			Secret: true})
	}
}

// destinationPhrase names pile B's destination in the human-readable prompt;
// it mirrors the Option.Kind vocabulary so the prompt and the wire never
// disagree.
func destinationPhrase(kind string) string {
	switch kind {
	case "bottom":
		return "the bottom of your library"
	case "graveyard":
		return "your graveyard"
	case "exile":
		return "exile"
	case "hand":
		return "your hand"
	default:
		return "their destination"
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
