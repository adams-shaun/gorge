package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// An Effect-delivered MayPlayLimit$ grant is its own permission: its cap
// counts the plays made through THAT grant, not the player's total may-plays
// (cr-g217; Forge's StaticAbility.getMayPlayTurn is per static ability, and an
// Effect's static lives on the Effect card).

// effLimitArtifact is an artifact whose activation delivers one Effect-
// carried may-play grant: the Tomb of Aclazotz's body with a caller-chosen
// static line.
func effLimitArtifact(name, static string) string {
	return "Name:" + name + "\nTypes:Artifact\n" +
		"A:AB$ Effect | Cost$ T | StaticAbilities$ StaticPlay | SpellDescription$ x\n" +
		"SVar:StaticPlay:" + static + "\nOracle:x\n"
}

const effLimitTomb = "Mode$ Continuous | Affected$ Creature.nonLand+YouOwn | MayPlay$ True | MayPlayLimit$ 1 | AffectedZone$ Graveyard | Description$ You may cast a creature spell from your graveyard this turn."

func effLimitBear(name string) string {
	return "Name:" + name + "\nManaCost:G\nTypes:Creature Bear\nPT:1/1\nOracle:x\n"
}

// effLimitGrants counts the live limited Effect-delivered may-play grants.
func effLimitGrants(e *Engine) int {
	n := 0
	for _, ce := range e.active() {
		if ce.MayPlay && ce.FromEffect && ce.MayPlayLimit > 0 {
			n++
		}
	}
	return n
}

// effLimitCastThroughMayPlay reports whether id was cast through a may-play
// permission (the pay-time CastInfo's "mayplay" flag).
func effLimitCastThroughMayPlay(e *Engine, id state.ObjID) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.CastInfo && ev.Obj == id && strings.Contains(ev.Counter, "mayplay") {
			return true
		}
	}
	return false
}

// effLimitActivate activates the artifact's Effect and drains the stack.
func effLimitActivate(t *testing.T, e *Engine, art state.ObjID) {
	t.Helper()
	activateTokenForge(t, e, art)
}

// effLimitOffered reports whether seat 0's priority decision (re-asked with
// one fresh G so a spent pool never hides an offer) offers a may-play cast
// of id.
func effLimitOffered(t *testing.T, e *Engine, id state.ObjID) bool {
	t.Helper()
	addMana(t, e, 0, "G")
	d := e.Pending()
	if d == nil {
		t.Fatalf("no decision pending")
	}
	return mayPlayOption(d, id) != nil
}

// effLimitCast casts id through its may-play option and drains the stack.
func effLimitCast(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	addMana(t, e, 0, "G")
	opt := mayPlayOption(e.Pending(), id)
	if opt == nil {
		t.Fatalf("may-play cast of %d not offered: %+v", id, e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 50)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("cast card %d zone=%v, want battlefield", id, o)
	}
}

func TestTwoEffectGrantsEachAllowOneCast(t *testing.T) {
	t.Parallel()
	artA := card(t, effLimitArtifact("Tomb A", effLimitTomb))
	artB := card(t, effLimitArtifact("Tomb B", effLimitTomb))
	bearA, bearB, bearC := card(t, effLimitBear("Bear A")), card(t, effLimitBear("Bear B")), card(t, effLimitBear("Bear C"))
	e, cfg := tokenReplGame(t, 5, artA, artB, bearA, bearB, bearC)
	a := moveSeededCard(t, e, 0, artA, state.ZBattlefield)
	b := moveSeededCard(t, e, 0, artB, state.ZBattlefield)
	ia := moveSeededCard(t, e, 0, bearA, state.ZGraveyard)
	ib := moveSeededCard(t, e, 0, bearB, state.ZGraveyard)
	ic := moveSeededCard(t, e, 0, bearC, state.ZGraveyard)
	effLimitActivate(t, e, a)
	effLimitActivate(t, e, b)
	if n := effLimitGrants(e); n != 2 {
		t.Fatalf("limited Effect may-play grants live = %d, want 2", n)
	}
	for _, id := range []state.ObjID{ia, ib, ic} {
		if !effLimitOffered(t, e, id) {
			t.Fatalf("graveyard creature %d not offered with two fresh grants", id)
		}
	}
	effLimitCast(t, e, ia)
	if !effLimitOffered(t, e, ib) {
		t.Fatalf("second grant spent by the first cast: Bear B not offered")
	}
	effLimitCast(t, e, ib)
	if effLimitOffered(t, e, ic) {
		t.Fatalf("a third graveyard creature offered with both one-use grants spent")
	}
	replayCheck(t, e, cfg)
}

func TestEffectGrantLimitIgnoresOtherPermissions(t *testing.T) {
	t.Parallel()
	free := card(t, effLimitArtifact("Instant Grant", "Mode$ Continuous | Affected$ Instant.YouOwn | MayPlay$ True | AffectedZone$ Graveyard | Description$ x"))
	tomb := card(t, effLimitArtifact("Tomb A", effLimitTomb))
	fizz := card(t, "Name:Fizz\nManaCost:G\nTypes:Instant\nA:SP$ GainLife | LifeAmount$ 1\nOracle:x\n")
	bear := card(t, effLimitBear("Bear A"))
	e, cfg := tokenReplGame(t, 5, free, tomb, fizz, bear)
	f := moveSeededCard(t, e, 0, free, state.ZBattlefield)
	a := moveSeededCard(t, e, 0, tomb, state.ZBattlefield)
	fz := moveSeededCard(t, e, 0, fizz, state.ZGraveyard)
	ib := moveSeededCard(t, e, 0, bear, state.ZGraveyard)
	effLimitActivate(t, e, f)
	effLimitActivate(t, e, a)
	if !effLimitOffered(t, e, fz) || !effLimitOffered(t, e, ib) {
		t.Fatalf("precondition: instant and creature must both be offered")
	}
	addMana(t, e, 0, "G")
	submitChoices(t, e, mayPlayOption(e.Pending(), fz).Index)
	passUntilStackEmpty(t, e, 50)
	if o := e.G.Obj(fz); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("instant zone=%v, want back in the graveyard after resolving", o)
	}
	if !effLimitCastThroughMayPlay(e, fz) {
		t.Fatalf("precondition: the instant was not cast through a may-play")
	}
	if !effLimitOffered(t, e, ib) {
		t.Fatalf("an unrelated unlimited may-play spent the Tomb grant")
	}
	replayCheck(t, e, cfg)
}

func TestEffectGrantLimitCountsOpponentOwnedCard(t *testing.T) {
	t.Parallel()
	art := card(t, effLimitArtifact("Thief", "Mode$ Continuous | Affected$ Creature.OppOwn | MayPlay$ True | MayPlayLimit$ 1 | AffectedZone$ Graveyard | Description$ x"))
	bearA, bearB := card(t, effLimitBear("Theirs A")), card(t, effLimitBear("Theirs B"))
	e, cfg := tokenReplGameSeats(t, 5, []*cards.Card{art}, []*cards.Card{bearA, bearB})
	a := moveSeededCard(t, e, 0, art, state.ZBattlefield)
	ia := moveSeededCard(t, e, 1, bearA, state.ZGraveyard)
	ib := moveSeededCard(t, e, 1, bearB, state.ZGraveyard)
	if e.G.Obj(ia).Owner != 1 {
		t.Fatalf("precondition: Theirs A owner = %d, want 1", e.G.Obj(ia).Owner)
	}
	effLimitActivate(t, e, a)
	if !effLimitOffered(t, e, ia) || !effLimitOffered(t, e, ib) {
		t.Fatalf("precondition: both opponent graveyard creatures must be offered")
	}
	effLimitCast(t, e, ia)
	if effLimitOffered(t, e, ib) {
		t.Fatalf("second opponent-owned card offered after the one-use grant was spent")
	}
	replayCheck(t, e, cfg)
}

func TestTombOfAclazotzOneCastPerActivation(t *testing.T) {
	t.Parallel()
	tomb := card(t, "Name:The Tomb of Aclazotz\nTypes:Legendary Land Cave\n"+
		"A:AB$ Effect | Cost$ T | StaticAbilities$ StaticPlay | SpellDescription$ You may cast a creature spell from your graveyard this turn.\n"+
		"SVar:StaticPlay:"+effLimitTomb+"\nOracle:x\n")
	bearA, bearB := card(t, effLimitBear("Bear A")), card(t, effLimitBear("Bear B"))
	e, cfg := tokenReplGame(t, 5, tomb, bearA, bearB)
	a := moveSeededCard(t, e, 0, tomb, state.ZBattlefield)
	ia := moveSeededCard(t, e, 0, bearA, state.ZGraveyard)
	ib := moveSeededCard(t, e, 0, bearB, state.ZGraveyard)
	effLimitActivate(t, e, a)
	if !effLimitOffered(t, e, ia) || !effLimitOffered(t, e, ib) {
		t.Fatalf("precondition: both graveyard creatures must be offered after one activation")
	}
	effLimitCast(t, e, ia)
	if effLimitOffered(t, e, ib) {
		t.Fatalf("a second graveyard creature offered after the activation's one cast")
	}
	replayCheck(t, e, cfg)
}
