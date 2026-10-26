package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const cbcSpellSrc = "Name:CBC Zap\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n"
const cbcFlashbackSrc = "Name:CBC Looting\nManaCost:R\nTypes:Sorcery\nK:Flashback:2 R\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"

// cbcEffect places a source whose DBEffect delivers the named CantBeCast body
// and resolves it for seat 0 (targets optional).
func cbcEffect(t *testing.T, e *Engine, body, extra string, targets []state.Target) {
	t.Helper()
	src := onBoardGrant(t, e, 0, "Name:CBC Source\nTypes:Artifact\n"+
		"SVar:STCantBeCast:"+body+"\n"+
		"SVar:DBEffect:DB$ Effect | StaticAbilities$ STCantBeCast"+extra+"\nOracle:x\n")
	o := e.G.Obj(src)
	effects.Resolve(e, &effects.Ctx{Source: src, Controller: 0, SVars: o.Face().SVars, Targets: targets},
		cards.ResolveSVar(o.Face().SVars, "DBEffect"))
}

func cbcNotes(e *Engine) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "CantBeCast unimplemented") {
			n++
		}
	}
	return n
}

func TestEffectCantBeCastBilbosGambit(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	spell1 := handCard(e, card(t, cbcSpellSrc), 1)
	spell0 := handCard(e, card(t, cbcSpellSrc), 0)
	if e.castRestricted(1, spell1) || e.castRestricted(0, spell0) {
		t.Fatal("precondition: spells restricted before the Effect resolves")
	}
	cbcEffect(t, e, "Mode$ CantBeCast | ValidCard$ Card | Caster$ Player", "", nil)
	if n := cbcNotes(e); n != 0 {
		t.Fatalf("unimplemented Note emitted %d times", n)
	}
	if !e.castRestricted(1, spell1) || !e.castRestricted(0, spell0) {
		t.Fatal("Effect-delivered CantBeCast did not restrict both players")
	}
	e.pending = nil
	e.setStep(state.StepCleanup)
	e.priorityRound()
	if e.castRestricted(1, spell1) || e.castRestricted(0, spell0) {
		t.Fatal("this-turn restriction survived cleanup")
	}
}

func TestEffectCantBeCastAvatarsWrathOpponentsOnly(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	fb1 := graveCard(e, card(t, cbcFlashbackSrc), 1, 1)
	hand1 := handCard(e, card(t, cbcSpellSrc), 1)
	fb0 := graveCard(e, card(t, cbcFlashbackSrc), 0, 0)
	cbcEffect(t, e, "Mode$ CantBeCast | ValidCard$ Card | Caster$ Player.Opponent | Origin$ Graveyard", "", nil)
	if !e.castRestricted(1, fb1) {
		t.Fatal("opponent's graveyard spell not restricted")
	}
	if e.castRestricted(1, hand1) {
		t.Fatal("opponent's hand spell restricted")
	}
	if e.castRestricted(0, fb0) {
		t.Fatal("controller's own graveyard spell restricted")
	}
}

func TestEffectCantBeCastMemoryVesselHandOnly(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	hand := handCard(e, card(t, cbcSpellSrc), 1)
	ex := e.G.AddObject(card(t, cbcSpellSrc), 1)
	ex.Zone = state.ZExile
	e.G.SetZone(state.ZExile, 1, []state.ObjID{ex.ID})
	cbcEffect(t, e, "Mode$ CantBeCast | ValidCard$ Card | Caster$ Player | Origin$ Hand", "", nil)
	if !e.castRestricted(1, hand) {
		t.Fatal("hand spell not restricted")
	}
	if e.castRestricted(1, ex.ID) {
		t.Fatal("exile-origin spell restricted by an Origin$ Hand lock")
	}
}

func TestEffectCantBeCastRememberedCaster(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	h0 := handCard(e, card(t, cbcSpellSrc), 0)
	h1 := handCard(e, card(t, cbcSpellSrc), 1)
	cbcEffect(t, e, "Mode$ CantBeCast | ValidCard$ Card | Caster$ Player.IsRemembered",
		" | RememberObjects$ TargetedPlayer", []state.Target{{IsPlayer: true, Player: 1}})
	if !e.castRestricted(1, h1) {
		t.Fatal("remembered player not restricted")
	}
	if e.castRestricted(0, h0) {
		t.Fatal("non-remembered player restricted")
	}
}

func TestCantBeCastPrintedOriginKunoros(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	fb := graveCard(e, card(t, cbcFlashbackSrc), 0, 0)
	hand := handCard(e, card(t, cbcSpellSrc), 0)
	if e.castRestricted(0, fb) {
		t.Fatal("precondition: graveyard spell restricted without Kunoros")
	}
	onBoardGrant(t, e, 1, "Name:Fake Kunoros\nTypes:Creature Hound\nPT:2/2\n"+
		"S:Mode$ CantBeCast | Origin$ Graveyard | Description$ x\nOracle:x\n")
	if !e.castRestricted(0, fb) {
		t.Fatal("graveyard cast not restricted by printed Origin$ Graveyard")
	}
	if e.castRestricted(0, hand) {
		t.Fatal("hand cast restricted by Origin$ Graveyard")
	}
}

func TestEffectCantBeCastUnreadParamStaysLoud(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	h := handCard(e, card(t, cbcSpellSrc), 1)
	cbcEffect(t, e, "Mode$ CantBeCast | ValidCard$ Card | Caster$ Player | Bogus$ Thing", "", nil)
	if cbcNotes(e) == 0 {
		t.Fatal("no unimplemented Note for an unread key")
	}
	if e.castRestricted(1, h) {
		t.Fatal("body with an unread key registered a restriction")
	}
}
