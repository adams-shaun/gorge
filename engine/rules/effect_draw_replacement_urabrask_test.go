package rules

// The Effect-created draw replacement whose remembered set lives on the
// DB$ Effect activation, not on the source object: Urabrask, Heretic
// Praetor's opponent-upkeep "the next time they would draw a card this turn,
// instead they exile the top card of their library. They may play it this
// turn." One `DB$ Effect | ReplacementEffects$ DrawReplace |
// StaticAbilities$ PlayOpp | RememberObjects$ TriggeredPlayer |
// ExileOnMoved$ Exile` activation registers TWO sibling registrations that
// read ONE remembered list: the Draw line's
// `IsPresent$ Card.IsRemembered | PresentZone$ Exile | PresentCompare$ EQ0`
// one-shot gate (spent the moment the body exiles+remembers a card), and the
// PlayOpp grant's `Affected$ Card.IsRemembered | AffectedZone$ Exile` filter
// (which must hold nothing until the body runs, then exactly the exiled
// card). The body (`DB$ Dig ... | RememberChanged$ True`) is what grows the
// list -- through rules' RememberOnEffect, which appends to every live
// registration of the activation's group in place. This test replaces the
// sibling ticket's Urabrask-stays-noted guard: its Note assertion is the
// registration assertion below.

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// urabraskPending returns the two live registrations of Urabrask's
// opponent-upkeep Effect activation: the Draw replacement and the may-play
// grant. Either may be nil while that activation has not registered (or has
// ended).
func urabraskPending(e *Engine, src state.ObjID) (repl, grant *state.ContinuousEffect) {
	act := e.active()
	for i := range act {
		ce := &act[i]
		if ce.Source != src {
			continue
		}
		if ce.ReplacementEvent == "Draw" && repl == nil {
			repl = ce
		}
		if ce.MayPlay && strings.Contains(ce.Affects, "IsRemembered") && grant == nil {
			grant = ce
		}
	}
	return repl, grant
}

// TestEffectDrawReplacementUrabraskOneShot drives the real corpus card: the
// opponent-upkeep trigger registers the Draw replacement + may-play grant
// live (no unimplemented Note); the opponent's turn-based draw is replaced
// by exiling their library top into the activation's shared remembered list;
// the SECOND draw that turn is a plain draw (the EQ0 one-shot gate flipped
// -- asserted by the exiled card being in the group list); the grant reads
// the same list for the owner-side play permission; and Urabrask's own
// upkeep self-dig chain (its DB$ Cleanup | ClearRemembered$ against the
// object) neither clears nor is cleared by the opponent's pending
// replacement -- the two remembered lists are distinct.
func TestEffectDrawReplacementUrabraskOneShot(t *testing.T) {
	t.Parallel()
	e, cfg, id := drawReplGame(t, 137, "Urabrask, Heretic Praetor", false)

	// The sibling ticket's Note refusal is gone: drive to seat 1's upkeep
	// and require BOTH sibling registrations live.
	passThroughTurns(t, e, func() bool {
		repl, grant := urabraskPending(e, id)
		return repl != nil && grant != nil
	})
	if hasNote(e, "continuous replacement unimplemented (DrawReplace)") {
		t.Fatal("Urabrask's DrawReplace fell through to the unimplemented Note")
	}
	repl, grant := urabraskPending(e, id)
	if repl == nil || grant == nil {
		t.Fatal("no live Draw replacement / may-play grant from Urabrask's activation")
	}
	// The ValidPlayer$ Player.IsRemembered gate reads the registration's
	// RememberedPlayers -- the triggered opponent, not the controller.
	if !slices.Contains(repl.RememberedPlayers, state.PlayerID(1)) ||
		slices.Contains(repl.RememberedPlayers, state.PlayerID(0)) {
		t.Fatalf("registration RememberedPlayers %v, want exactly the triggered opponent [1]", repl.RememberedPlayers)
	}

	// Seat 1's turn-based draw is the replaced one (the line carries no
	// NotFirstCardInDrawStep$).
	h1 := len(e.G.Zone(state.ZHand, 1))
	lib1 := len(e.G.Zone(state.ZLibrary, 1))
	if lib1 == 0 {
		t.Fatal("precondition: seat 1 library is empty")
	}
	if got := len(e.G.Zone(state.ZExile, 1)); got != 0 {
		t.Fatalf("precondition: seat 1 exile = %d, want 0", got)
	}
	driveErr := func() string {
		var sb strings.Builder
		for _, ev := range e.L.Events {
			if ev.Kind == events.Note && ev.Text != "" {
				sb.WriteString("NOTE[" + ev.Text + "] ")
			}
		}
		repl2, grant2 := urabraskPending(e, id)
		fmt.Fprintf(&sb, "turn=%d step=%d active=%d repl=%v grant=%v hand1=%d lib1=%d ex1=%d",
			e.G.Turn, e.G.Step, e.G.Active, repl2 != nil, grant2 != nil,
			len(e.G.Zone(state.ZHand, 1)), len(e.G.Zone(state.ZLibrary, 1)), len(e.G.Zone(state.ZExile, 1)))
		return sb.String()
	}
	t.Logf("DEBUG pre-draw: %s", driveErr())
	if r0, _ := urabraskPending(e, id); r0 != nil {
		t.Logf("DEBUG reg: players=%v remembered=%v group=%d eot=%v perm=%v dur=%q untilTurn=%d params=%v body=%q",
			r0.RememberedPlayers, r0.Remembered, r0.EffectGroup, r0.UntilEOT, r0.Permanent,
			r0.Duration, r0.UntilTurn, r0.ReplacementParams, r0.ReplacementBody)
		with := replacementBodySA(r0.ReplacementBody)
		rPr := cards.Repl{Event: r0.ReplacementEvent, Params: r0.ReplacementParams, With: with}
		evProbe := events.Event{Kind: events.Draw, Player: 1, From: state.ZLibrary, To: state.ZHand, Secret: true}
		t.Logf("DEBUG match=%v hold=%v with=%v",
			e.replacementMatchesRememberedUngatedBy(rPr, id, evProbe, r0.Remembered, r0.RememberedPlayers, nil, 0, nil, chosenCapture{}),
			e.replacementConditionHoldsWith(rPr, id, 0, nil, r0.Remembered), with != nil)
		t.Logf("DEBUG possible=%v", e.effectReplacementsPossible())
		hb, lb := len(e.G.Zone(state.ZHand, 1)), len(e.G.Zone(state.ZLibrary, 1))
		e.drawCard(1)
		repl1, _ := urabraskPending(e, id)
		t.Logf("DEBUG after drawCard: hand %d->%d lib %d->%d exile1=%d replLive=%v notes=%d",
			hb, len(e.G.Zone(state.ZHand, 1)), lb, len(e.G.Zone(state.ZLibrary, 1)),
			len(e.G.Zone(state.ZExile, 1)), repl1 != nil, len(e.L.Events))
		mPr := replMatch{id: id, repl: &rPr, remembered: r0.Remembered,
			rememberedPlayers: r0.RememberedPlayers, chosen: r0.ChosenNumber,
			controller: r0.Controller, frozenController: true, key: effectReplKey(r0)}
		ctxPr := e.replCtx(mPr, evProbe)
		hb2, lb2, ex2 := len(e.G.Zone(state.ZHand, 1)), len(e.G.Zone(state.ZLibrary, 1)), len(e.G.Zone(state.ZExile, 1))
		ne0 := len(e.L.Events)
		e.runReplaceWith(ctxPr, 0, rPr.With, &evProbe)
		var sb2 strings.Builder
		for _, evx := range e.L.Events[ne0:] {
			fmt.Fprintf(&sb2, "{k=%d obj=%d p=%d from=%d to=%d amt=%d text=%q} ", evx.Kind, evx.Obj, evx.Player, evx.From, evx.To, evx.Amount, evx.Text)
		}
		ps, okp := effects.EffectOwnerPlayers(e, ctxPr, "ReplacedPlayer")
		t.Logf("DEBUG replPlayer=%+v definedPlayers=%v ok=%v", ctxPr.Repl.Player, ps, okp)
		withNC := replacementBodySA("DB$ Dig | Defined$ ReplacedPlayer | DigNum$ 1 | ChangeNum$ All | DestinationZone$ Exile | RememberChanged$ True")
		ne3 := len(e.L.Events)
		e.runReplaceWith(ctxPr, 0, withNC, &evProbe)
		var sb3 strings.Builder
		for _, evx := range e.L.Events[ne3:] {
			fmt.Fprintf(&sb3, "{k=%d obj=%d p=%d from=%d to=%d text=%q} ", evx.Kind, evx.Obj, evx.Player, evx.From, evx.To, evx.Text)
		}
		t.Logf("DEBUG nocond: exile1=%d lib1=%d newevents: %s", len(e.G.Zone(state.ZExile, 1)), len(e.G.Zone(state.ZLibrary, 1)), sb3.String())
		t.Logf("DEBUG body: exile1 %d->%d hand %d->%d lib %d->%d pending=%v newevents: %s",
			ex2, len(e.G.Zone(state.ZExile, 1)), hb2, len(e.G.Zone(state.ZHand, 1)),
			lb2, len(e.G.Zone(state.ZLibrary, 1)), e.Pending(), sb2.String())
	}
	for i := 0; i < 4000 && len(e.G.Zone(state.ZExile, 1)) == 0; i++ {
		if answerIfDiscard(t, e) {
			continue
		}
		d := e.Pending()
		switch {
		case d == nil:
			e.Advance()
		case d.Kind == decision.KPriority:
			passOnce(t, e)
		default:
			submitChoices(t, e, d.Options[0].Index)
		}
	}
	if len(e.G.Zone(state.ZExile, 1)) == 0 {
		t.Fatalf("no draw was replaced: %s", driveErr())
	}
	exiled := e.G.Zone(state.ZExile, 1)
	if len(exiled) != 1 {
		t.Fatalf("seat 1 exile = %d cards, want exactly 1 (the replaced draw)", len(exiled))
	}
	exiledID := exiled[0]
	if got := len(e.G.Zone(state.ZHand, 1)); got != h1 {
		t.Fatalf("seat 1 hand %d -> %d, the replaced draw must move no card to hand", h1, got)
	}
	if got := len(e.G.Zone(state.ZLibrary, 1)); got != lib1-1 {
		t.Fatalf("seat 1 library %d -> %d, want the top card exiled", lib1, got)
	}

	// The activation's remembered list holds the exiled card on BOTH sibling
	// registrations -- appended in place, shared, this is the precondition
	// the one-shot gate flipped on.
	repl, grant = urabraskPending(e, id)
	if repl == nil || grant == nil {
		t.Fatal("the pending replacement/grant ended with its first use; the one-shot is the EQ0 gate, not the registration")
	}
	if !slices.Contains(repl.Remembered, exiledID) {
		t.Fatalf("Draw registration Remembered %v, want the exiled card %d (the EQ0 gate must see it)", repl.Remembered, exiledID)
	}
	if !slices.Contains(grant.Remembered, exiledID) {
		t.Fatalf("may-play grant Remembered %v, want the same in-place-appended card %d", grant.Remembered, exiledID)
	}

	// The SECOND draw this turn is NOT replaced (EQ0 now counts 1).
	h2 := len(e.G.Zone(state.ZHand, 1))
	l2 := len(e.G.Zone(state.ZLibrary, 1))
	e.drawCard(1)
	if got := len(e.G.Zone(state.ZHand, 1)); got != h2+1 {
		t.Fatalf("second draw: hand %d -> %d, want a plain draw (the EQ0 gate flipped)", h2, got)
	}
	if got := len(e.G.Zone(state.ZLibrary, 1)); got != l2-1 {
		t.Fatalf("second draw: library %d -> %d, want a plain draw", l2, got)
	}

	// The owner-side play grant reads the same live list.
	_, grant = urabraskPending(e, id)
	if grant == nil {
		t.Fatal("the may-play grant ended before the opponent could use it")
	}
	if !grant.MayPlay || grant.MayPlayPlayer != "CardOwner" ||
		grant.AffectedZone != "Exile" || !strings.Contains(grant.Affects, "IsRemembered") {
		t.Fatalf("grant %+v, want MayPlay CardOwner over exiled remembered cards", grant)
	}
	if !slices.Contains(grant.Remembered, exiledID) {
		t.Fatalf("grant Remembered %v still lacks the exiled card %d", grant.Remembered, exiledID)
	}

	// Urabrask's OWN upkeep self-dig chain, run while the opponent's pending
	// replacement is live: its DB$ Dig | RememberChanged$ True and its
	// DB$ Cleanup | ClearRemembered$ work on the object and the chain's own
	// ctx -- they must neither clear the opponent's pending list nor grow
	// from it.
	face := e.G.Obj(id).Face()
	trigExile := cards.ResolveSVar(face.SVars, "TrigExile")
	if trigExile == nil {
		t.Fatal("precondition: corpus script lost TrigExile")
	}
	ownBefore := e.G.Zone(state.ZExile, 0)
	if len(face.Abilities) == 0 {
		t.Fatal("precondition: Urabrask face carries no abilities")
	}
	effects.Resolve(e, effects.NewCtxPtr(id, 0, effects.CtxInit{SVars: face.SVars}), trigExile)
	ownExiled := e.G.Zone(state.ZExile, 0)
	if len(ownExiled) != len(ownBefore)+1 {
		t.Fatalf("seat 0 exile %d -> %d, want the own-chain dig to exile exactly one card", len(ownBefore), len(ownExiled))
	}
	ownCard := state.ObjID(0)
	for _, o := range ownBefore {
		if ownExiled[len(ownExiled)-1] == o {
			t.Fatal("own-chain dig re-exiled a card already exiled")
		}
	}
	ownCard = ownExiled[len(ownExiled)-1]

	// The opponent's pending replacement survived the own chain's
	// ClearRemembered$ untouched, and holds no own-chain card.
	repl, grant = urabraskPending(e, id)
	if repl == nil || grant == nil {
		t.Fatal("Urabrask's own-chain ClearRemembered$ cleared the opponent's pending registrations")
	}
	if !slices.Contains(repl.Remembered, exiledID) || !slices.Contains(grant.Remembered, exiledID) {
		t.Fatalf("opponent pending Remembered %v/%v, want the exiled card %d kept through the own-chain cleanup",
			repl.Remembered, grant.Remembered, exiledID)
	}
	if slices.Contains(repl.Remembered, ownCard) || slices.Contains(grant.Remembered, ownCard) {
		t.Fatalf("opponent pending Remembered %v/%v hold the own-chain card %d: the two lists are not distinct",
			repl.Remembered, grant.Remembered, ownCard)
	}

	// And the own chain's own may-play grant holds exactly its own card --
	// the two activations' lists are distinct in both directions.
	var ownGrant *state.ContinuousEffect
	act := e.active()
	for i := range act {
		ce := &act[i]
		if ce.Source == id && ce.MayPlay && slices.Contains(ce.Remembered, ownCard) {
			ownGrant = ce
		}
	}
	if ownGrant == nil {
		t.Fatal("Urabrask's own upkeep chain registered no may-play grant for its exiled card")
	}
	if !slices.Contains(ownGrant.Remembered, ownCard) || slices.Contains(ownGrant.Remembered, exiledID) {
		t.Fatalf("own grant Remembered %v, want exactly the own-chain card %d and not the opponent's %d",
			ownGrant.Remembered, ownCard, exiledID)
	}

	replayCheck(t, e, cfg)
}
