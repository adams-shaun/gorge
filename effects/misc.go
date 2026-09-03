package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("Mana", effMana)
	Register("Effect", effEffect)
	Register("Cleanup", effCleanup)
	Register("SetState", effSetState)
	Register("Counter", effCounter)
	Register("DelayedTrigger", effDelayedTrigger)
	Register("Repeat", effRepeat)
	Register("Charm", effCharm)
	Register("Vote", effVote)
	Register("BecomeMonarch", effBecomeMonarch)
	Register("RestartGame", effRestartGame)
}

// Token and CopySpellAbility are NOT registered. Both need to create a brand
// new game object mid-match (a token; a copy of a spell or ability already on
// the stack), and every state mutation in this engine goes through
// events.Apply -- there is no event kind whose Apply case can append to
// state.Game.Objs, and adding one is bigger than a single primitive's scope
// (a new Kind, a new Apply case that mints an ID and decides how a copy's
// Card/FaceIdx/Ability/Targets carry over, and -- since state.Object.Card ==
// nil already means "token" to the existing "token"/"!token" filter
// predicates in filter.go -- a token minted with no Card would still fail
// every type-based ValidTgts/ValidCards predicate, like "Creature", since
// matchesBase's hasType requires Face(); only the untyped "Any"/"Card"/
// "Permanent" specs would ever see it). Registering either as a Note-only
// stub would make Supported()/Coverage claim a card is playable when it
// cannot actually do what its text says. Left unregistered, Resolve's
// existing "unimplemented API" Note fallback applies, and Coverage correctly
// excludes any card that needs them. See the Task 18 report for the proposed
// design.

// effEffect is M1's version of "create a nameless effect object holding
// StaticAbilities$/Triggers$ for Duration$": like Pump/Animate/Protection,
// the actual registration needs Task 19's rules.Layers (and, for Duration$
// forms that outlive the current resolution, a holding zone this build does
// not have -- state.Zone has no Command-zone equivalent). M1 records the
// intent as a Note.
func effEffect(h Host, c *Ctx, sa *cards.SA) {
	dur := sa.Params["Duration"]
	if dur == "" {
		dur = "Permanent"
	}
	what := strings.TrimSpace(sa.Params["StaticAbilities"] + " " + sa.Params["Triggers"])
	h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
		Text: "registers a continuous effect (" + what + ") for " + dur})
}

// effCleanup is "DB$ Cleanup | ClearRemembered$ True": nothing in this build
// persists a Remembered/Imprinted list on an object yet (Ctx.Remembered is a
// per-resolution parameter, not stored state), so there is nothing to
// actually clear. The Note records that the step ran.
func effCleanup(h Host, c *Ctx, sa *cards.SA) {
	h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "clears remembered/imprinted objects"})
}

// effSetState flips a double-faced target to its other face. M1 does not
// model Mode$'s vocabulary (Transform/Flip/Meld all behave the same here):
// it just advances to the next face, wrapping to 0, which is correct for the
// overwhelmingly common two-face case and a no-op for anything with fewer
// than two faces (a token, or a single-faced card).
func effSetState(h Host, c *Ctx, sa *cards.SA) {
	mode := sa.Params["Mode"]
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Card == nil || len(o.Card.Faces) < 2 {
			continue
		}
		next := (int(o.FaceIdx) + 1) % len(o.Card.Faces)
		h.Emit(events.Event{Kind: events.Note, Obj: o.ID,
			Text: "flips to face " + strconv.Itoa(next) + " (" + mode + ")"})
		h.Emit(events.Event{Kind: events.FlipFace, Obj: o.ID, Amount: int32(next)})
	}
}

// effCounter removes the targeted spell from the stack to its owner's
// graveyard. This is CR 608.2b's canonical case: if the target is no longer
// on the stack by the time this resolves (already resolved, or itself
// countered by an earlier effect in the same response), it is simply skipped
// rather than moved from wherever it now sits.
func effCounter(h Host, c *Ctx, sa *cards.SA) {
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Zone != state.ZStack {
			continue
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID,
			From: state.ZStack, To: state.ZGraveyard, Text: "countered"})
	}
}

// effDelayedTrigger records that a delayed trigger would be registered.
// Actually firing Execute$ at Mode$ needs Task 20's trigger queue; M1 has
// nothing to hold the registration on, so it only leaves a Note.
func effDelayedTrigger(h Host, c *Ctx, sa *cards.SA) {
	h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
		Text: "registers a delayed trigger at " + sa.Params["Mode"]})
}

// effRepeat runs RepeatSubAbility$ RepeatNum$ times (default 1). The brief
// names the amount parameter "RepeatNum$"; the fetched corpus's real cards
// use "MaxRepeat$" instead (RepeatNum$ appears nowhere), so against real
// cards this currently always falls back to the default of one run -- a
// documented, non-crashing degradation, not a special-cased alias. See the
// Task 18 report.
func effRepeat(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "RepeatNum", 1)
	if n < 0 {
		n = 0
	}
	name := sa.Params["RepeatSubAbility"]
	if name == "" || c.SVars == nil {
		return
	}
	sub := cards.ResolveSVar(c.SVars, name)
	if sub == nil {
		return
	}
	for i := int32(0); i < n; i++ {
		Resolve(h, c, sub)
	}
}

// effCharm chooses CharmNum$ of the Choices$ sub-abilities and runs it. M1
// always takes the first choice, regardless of CharmNum$ -- turning this into
// a real choice is Task 20's KModes decision.
func effCharm(h Host, c *Ctx, sa *cards.SA) {
	if c.SVars == nil {
		return
	}
	choices := strings.Split(sa.Params["Choices"], ",")
	if len(choices) == 0 {
		return
	}
	name := strings.TrimSpace(choices[0])
	if sub := cards.ResolveSVar(c.SVars, name); sub != nil {
		Resolve(h, c, sub)
	}
}

// effVote has each Defined$ player vote for the first Choices$ entry (M1's
// simplification -- a real vote is a per-player choice, Task 20's territory)
// and records one Note per vote rather than running any chosen mode: unlike
// Charm, the brief's own spec for Vote is "Note per vote", not "whatever the
// chosen mode emits".
func effVote(h Host, c *Ctx, sa *cards.SA) {
	first := ""
	if choices := sa.Params["Choices"]; choices != "" {
		first = strings.TrimSpace(strings.SplitN(choices, ",", 2)[0])
	}
	for _, t := range Defined(h, c, sa) {
		h.Emit(events.Event{Kind: events.Note, Player: PlayerOf(h, c, t), Text: "votes for " + first})
	}
}

// effBecomeMonarch records who becomes the monarch. Monarchy itself is
// game-level state Task 22 adds; M1 only has the Note.
func effBecomeMonarch(h Host, c *Ctx, sa *cards.SA) {
	targets := Defined(h, c, sa)
	if len(targets) == 0 {
		return
	}
	h.Emit(events.Event{Kind: events.Note, Player: PlayerOf(h, c, targets[0]), Text: "becomes the monarch"})
}

// effRestartGame ends the game as a draw. Actually restarting (leaving
// exiled permanents in play under the restarting player's control, per the
// real card text) is out of M1's scope; ending the match honestly rather
// than hanging or silently no-op-ing is the closest correct degradation.
func effRestartGame(h Host, c *Ctx, sa *cards.SA) {
	h.Emit(events.Event{Kind: events.GameOver, Text: "game restarted: ended as a draw"})
}

// effMana implements "AB$ Mana": add Amount mana of Produced's colour(s) to
// the activating player's pool. Absorbed from Task 14's stopgap: the
// negative-Amount clamp is Ruling T14-f, kept verbatim for the same reason as
// DealDamage's -- events.Apply's ManaAdd case is a plain "+=", so an
// unclamped negative would drop the pool below zero instead of doing
// nothing. Folded in on top of that: "Any"/"Combo Any" resolves to colourless
// rather than asking (a real choice is Task 20's job), and a dual-producing
// ability such as "Add {R}{R}" is walked one symbol at a time rather than
// split on whitespace, since Produced$ carries no spaces of its own.
func effMana(h Host, c *Ctx, sa *cards.SA) {
	produced := strings.TrimSpace(sa.Params["Produced"])
	if produced == "" || produced == "Any" || produced == "Combo Any" {
		produced = "C"
	}
	amt := Num(h, c, sa, "Amount", 1)
	if amt < 0 {
		amt = 0
	}
	for _, r := range strings.NewReplacer("{", "", "}", "", " ", "").Replace(produced) {
		h.Emit(events.Event{Kind: events.ManaAdd, Player: c.Controller,
			Counter: string(r), Amount: amt})
	}
}
