package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestProtectionFromEachOpponentOnAPermanent is the OBJECT half of Forge's
// bare player-relative protection quality: a permanent that itself carries
// `Protection:Player.Opponent:each of your opponents` is protected from
// sources controlled by an opponent of the permanent's CURRENT controller,
// and only from those. The player half (Absolute Virtue) already reads
// Player.Opponent through the shared player filter in playerProtectedFrom;
// this pins the same arm inside protectedFrom, with the protected permanent's
// controller as the "you".
//
// The control assertions (the controller's own creature and Aura are NOT
// withheld) are the "compared values actually differ" precondition: a
// predicate that simply returned true for every source would pass the
// opponent direction and fail here.
func TestProtectionFromEachOpponentOnAPermanent(t *testing.T) {
	t.Parallel()
	const (
		bearerSrc = "Name:Opp-Protected Avatar\nManaCost:1 W\nTypes:Creature Avatar\nPT:3/4\n" +
			"K:Protection:Player.Opponent:each of your opponents\nOracle:x\n"
		oppCreatureSrc = "Name:Opponent Bear\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
		ownCreatureSrc = "Name:Controller Bear\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
		oppAuraSrc     = "Name:Opponent Aura\nManaCost:U\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n"
		ownAuraSrc     = "Name:Controller Aura\nManaCost:U\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n"
	)
	e, cfg, _ := newFixtureDeck(t, 73, bearerSrc)
	bearer := putCreature(t, e, 0, bearerSrc)
	oppCreature := putToken(t, e, 1, oppCreatureSrc, state.ZBattlefield)
	ownCreature := putToken(t, e, 0, ownCreatureSrc, state.ZBattlefield)

	// Precondition: the bearer is on the battlefield carrying the exact
	// derived keyword, and the sources really sit under different
	// controllers (the two directions must not be the same seat's source).
	if got := e.Keywords(bearer); !slices.Contains(got, "Protection:Player.Opponent:each of your opponents") {
		t.Fatalf("precondition: bearer keywords = %v, want the inline Player.Opponent protection keyword", got)
	}
	if got := e.controllerOf(bearer); got != 0 {
		t.Fatalf("precondition: bearer controller = %d, want seat 0", got)
	}
	if got := e.controllerOf(oppCreature); got != 1 {
		t.Fatalf("precondition: opponent creature controller = %d, want seat 1", got)
	}
	if got := e.controllerOf(ownCreature); got != 0 {
		t.Fatalf("precondition: own creature controller = %d, want seat 0", got)
	}

	// Predicate: the opponent's source is withheld; the controller's own
	// source is not.
	if !e.protectedFrom(bearer, oppCreature) {
		t.Fatalf("protectedFrom(bearer, opponent creature) = false, want true; keywords=%v", e.Keywords(bearer))
	}
	if e.protectedFrom(bearer, ownCreature) {
		t.Fatal("protectedFrom(bearer, own creature) = true; Player.Opponent must not withhold the controller's own source")
	}

	// Damage: the opponent creature's damage is prevented; the controller's
	// own creature still deals its damage (the TestProtectionFromBlue shape).
	e.damaging = oppCreature
	e.emit(events.Event{Kind: events.Damage, Obj: bearer, Amount: 2})
	e.damaging = 0
	if got := e.G.Obj(bearer).Damage; got != 0 {
		t.Fatalf("opponent's damage was not prevented: Damage=%d, want 0", got)
	}
	e.damaging = ownCreature
	e.emit(events.Event{Kind: events.Damage, Obj: bearer, Amount: 2})
	e.damaging = 0
	if got := e.G.Obj(bearer).Damage; got != 2 {
		t.Fatalf("controller's own damage was prevented too: Damage=%d, want 2", got)
	}

	// Blocking: the opponent's creature cannot block it; the controller's
	// own creature can.
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 0, IDs: []state.ObjID{bearer}})
	e.pending = nil
	if e.canBlock(oppCreature, bearer) {
		t.Fatal("an opponent's creature blocked a permanent with protection from each opponent")
	}
	if !e.canBlock(ownCreature, bearer) {
		t.Fatal("the controller's own creature should be able to block")
	}

	// Attachment: attach legality reads the same predicate (attach.go's Aura
	// SBA). The opponent-controlled Aura otherwise legally enchants the
	// bearer, so the withheld attachment is protection, not an Enchant
	// mismatch; the controller's own Aura is not withheld.
	oppAura := putToken(t, e, 1, oppAuraSrc, state.ZBattlefield)
	ownAura := putToken(t, e, 0, ownAuraSrc, state.ZBattlefield)
	for _, aura := range []state.ObjID{oppAura, ownAura} {
		o := e.G.Obj(aura)
		if o == nil || o.Zone != state.ZBattlefield || !e.isAura(o) {
			t.Fatalf("precondition: Aura %d is not an on-battlefield Aura", aura)
		}
		if !e.auraStillMatchesEnchant(o, e.G.Obj(bearer)) {
			t.Fatalf("precondition: Aura %d no longer matches its enchant spec, so protection would not be the tested reason", aura)
		}
	}
	if !e.protectedFrom(bearer, oppAura) {
		t.Fatal("protectedFrom(bearer, opponent Aura) = false; protection must withhold an opponent's attachment")
	}
	if e.protectedFrom(bearer, ownAura) {
		t.Fatal("protectedFrom(bearer, own Aura) = true; Player.Opponent must not withhold the controller's own attachment")
	}

	replayCheck(t, e, cfg)
}
