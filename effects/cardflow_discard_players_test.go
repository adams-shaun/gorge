package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestDiscardRememberingPlayersPersistently pins the second half of the
// RememberDiscardingPlayers$ rider (Professor Onyx's ultimate, Snort): the
// discarding player joins not only the resolution-local Ctx.Remembered set
// but ALSO the source object's event-backed persistent remembered list --
// the one a LATER ability's Player.IsRemembered read (CR 608.2c's remembered
// result used after the resolving effect is gone) sees. The write rides the
// same event mechanism as RememberDiscarded$ and
// rememberInvestigatingPlayers; dedup is retained, so a player who discards
// twice under one resolution is persisted once.
func TestDiscardRememberingPlayersPersistently(t *testing.T) {
	ah, ctx, ids := discardBoard(t, creature(t, "Frog"), creature(t, "Bird"), creature(t, "Cat"))
	s := sa(t, "SP$ Discard | ValidTgts$ Player | Mode$ Defined | DefinedCards$ Remembered"+
		" | RememberDiscardingPlayers$ True")

	src := ah.g.Obj(ctx.Source)
	if src == nil {
		t.Fatal("precondition: the resolving source object is gone")
	}
	// The discarder (seat 1, whose hand the board fills) and the source
	// (seat 0's object) must be distinct, or a "remembered" write could be
	// the source remembering itself and prove nothing about the rider.
	if src.Controller == 1 {
		t.Fatal("precondition: the source object belongs to the discarding seat — the rider assertions would not discriminate")
	}
	if len(src.Remembered) != 0 {
		t.Fatalf("precondition: the source object's persistent remembered list is not empty (%v)", src.Remembered)
	}
	if ah.g.Zone(state.ZHand, 1)[0] != ids[0] || len(ah.g.Zone(state.ZHand, 1)) != 3 {
		t.Fatal("precondition: seat 1's hand is not the three dealt creatures")
	}

	// First discard: two named cards, one discarding player.
	ctx.Remembered = []state.Target{{Obj: ids[1]}, {Obj: ids[2]}}
	effDiscard(ah, ctx, s)
	for _, want := range []state.ObjID{ids[1], ids[2]} {
		if !inZone(ah.g, state.ZGraveyard, 1, want) {
			t.Fatalf("the DefinedCards$ card %d was not discarded", want)
		}
	}

	// Resolution-local half: the discarding player, exactly once.
	seen := 0
	for _, tg := range ctx.Remembered {
		if tg.IsPlayer && tg.Player == 1 {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("Ctx.Remembered recorded the discarder %d time(s), want exactly 1", seen)
	}

	// Persistent half: the source object's event-backed remembered list
	// carries the player as a PlayerRef, and (no RememberDiscarded$ on this
	// script) carries no cards -- the only writes here are the player ones.
	persistent := ah.g.Obj(ctx.Source).Remembered
	pSeen := 0
	for _, tg := range persistent {
		if tg.IsPlayer && tg.Player == 1 {
			pSeen++
		}
		if !tg.IsPlayer {
			t.Fatalf("the persistent list gained a card entry %v under a RememberDiscardingPlayers$-only script", tg)
		}
	}
	if pSeen != 1 {
		t.Fatalf("the source object's persistent remembered list recorded the discarder %d time(s), want exactly 1: %v", pSeen, persistent)
	}

	// Dedup: a second discard by the SAME player in the same resolution
	// must not append a second player entry to either half. The player entry
	// from the first discard stays in the resolution's remember set (the
	// resolution is still live), and the new card joins it.
	ctx.Remembered = append(ctx.Remembered, state.Target{Obj: ids[0]})
	effDiscard(ah, ctx, s)
	if !inZone(ah.g, state.ZGraveyard, 1, ids[0]) {
		t.Fatal("the second discard's card was not discarded")
	}
	seen = 0
	for _, tg := range ctx.Remembered {
		if tg.IsPlayer && tg.Player == 1 {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("after a second discard by the same player, Ctx.Remembered holds the player %d time(s), want 1", seen)
	}
	persistent = ah.g.Obj(ctx.Source).Remembered
	pSeen = 0
	for _, tg := range persistent {
		if tg.IsPlayer && tg.Player == 1 {
			pSeen++
		}
	}
	if pSeen != 1 {
		t.Fatalf("after a second discard by the same player, the persistent list holds the player %d time(s), want 1: %v", pSeen, persistent)
	}

	// The Snort/Onyx-shaped follow-up: a LATER ability with the same source
	// and an empty transient remember reads Player.IsRemembered off the
	// source card's persistent list and finds the discarder. Before the
	// persistent write this read returned nobody, so Professor Onyx punished
	// the wrong players and Snort's remembered-player damage hit nobody.
	ctx2 := &Ctx{Source: ctx.Source, Controller: 0}
	got := definedPlayers(ah, ctx2, sa(t, "SP$ Draw | Defined$ Player.IsRemembered"))
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("a later ability's Defined$ Player.IsRemembered read %v, want the discarder [1]", got)
	}
}

// TestDiscardRememberingPlayerAlreadyInTransientContext is the case the
// first version of the fix missed (reviewer MAJOR, round 1): the discarding
// player is ALREADY in the resolution-local Ctx.Remembered set when the
// RememberDiscardingPlayers$ rider runs -- an earlier subeffect in the same
// resolution (e.g. ChoosePlayer's RememberChosen$) put them there WITHOUT
// persisting them -- but the source object's persistent remembered list is
// still empty. The persistent write must not be conditional on the transient
// entry, or a later ability's Player.IsRemembered read (Professor Onyx's
// ultimate, Snort's follow-up) still cannot see the discarder.
func TestDiscardRememberingPlayerAlreadyInTransientContext(t *testing.T) {
	ah, ctx, ids := discardBoard(t, creature(t, "Frog"), creature(t, "Bird"), creature(t, "Cat"))
	s := sa(t, "SP$ Discard | ValidTgts$ Player | Mode$ Defined | DefinedCards$ Remembered"+
		" | RememberDiscardingPlayers$ True")

	src := ah.g.Obj(ctx.Source)
	if src == nil {
		t.Fatal("precondition: the resolving source object is gone")
	}
	if src.Controller == 1 {
		t.Fatal("precondition: the source object belongs to the discarding seat — the persistence assertions would not discriminate")
	}
	// The whole point of this test: the player is already transiently
	// remembered, but NOT persistently remembered.
	ctx.Remembered = []state.Target{{Player: 1, IsPlayer: true}}
	if len(src.Remembered) != 0 {
		t.Fatalf("precondition: the source object's persistent remembered list is not empty (%v) — the persistence assertion would be vacuous", src.Remembered)
	}

	ctx.Remembered = append(ctx.Remembered, state.Target{Obj: ids[1]}, state.Target{Obj: ids[2]})
	effDiscard(ah, ctx, s)

	if !inZone(ah.g, state.ZGraveyard, 1, ids[1]) || !inZone(ah.g, state.ZGraveyard, 1, ids[2]) {
		t.Fatal("the DefinedCards$ cards were not discarded — the rider did not run")
	}
	// Transient half: still exactly one entry.
	seen := 0
	for _, tg := range ctx.Remembered {
		if tg.IsPlayer && tg.Player == 1 {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("Ctx.Remembered held the discarder %d time(s) after the discard, want exactly 1", seen)
	}
	// Persistent half: the write must have happened DESPITE the transient
	// entry already existing.
	persistent := ah.g.Obj(ctx.Source).Remembered
	pSeen := 0
	for _, tg := range persistent {
		if tg.IsPlayer && tg.Player == 1 {
			pSeen++
		}
	}
	if pSeen != 1 {
		t.Fatalf("with the discarder already transiently remembered, the source's persistent list recorded them %d time(s), want exactly 1: %v", pSeen, persistent)
	}

	// A later ability reading Player.IsRemembered off the source sees them.
	ctx2 := &Ctx{Source: ctx.Source, Controller: 0}
	got := definedPlayers(ah, ctx2, sa(t, "SP$ Draw | Defined$ Player.IsRemembered"))
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("a later ability's Defined$ Player.IsRemembered read %v, want the discarder [1]", got)
	}

	// Dedup on the persistent half: a second discard in the same resolution
	// must not append a second player entry.
	ctx.Remembered = append(ctx.Remembered, state.Target{Obj: ids[0]})
	effDiscard(ah, ctx, s)
	if !inZone(ah.g, state.ZGraveyard, 1, ids[0]) {
		t.Fatal("the second discard's card was not discarded")
	}
	persistent = ah.g.Obj(ctx.Source).Remembered
	pSeen = 0
	for _, tg := range persistent {
		if tg.IsPlayer && tg.Player == 1 {
			pSeen++
		}
	}
	if pSeen != 1 {
		t.Fatalf("after a second discard by the same player, the persistent list holds them %d time(s), want 1: %v", pSeen, persistent)
	}
}
