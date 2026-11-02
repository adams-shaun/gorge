package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	beamSpell   = "Name:Recall Note\nManaCost:U\nTypes:Sorcery\nK:Beam me up:2 U\nA:SP$ GainLife | Defined$ You | LifeAmount$ 3\nOracle:x\n"
	beamBear    = "Name:Away Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	beamGrantor = "Name:Chief Engineer Bear\nManaCost:1 U U\nTypes:Creature Bear\nPT:2/3\n" +
		"S:Mode$ Continuous | AddKeyword$ Beam me up | Affected$ Sorcery.YouOwn,Instant.YouOwn | AffectedZone$ Graveyard | Description$ x\n" +
		"S:Mode$ CantBeBeamedUp | ValidSA$ Spell.KeywordSource | ValidCard$ Creature.Self\nOracle:x\n"
	beamMaroon = "Name:Stranded\nManaCost:U\nTypes:Enchantment Aura\nK:Enchant:Creature:creature\nA:SP$ Attach | ValidTgts$ Creature\n" +
		"S:Mode$ CantBeBeamedUp | ValidCard$ Creature.EnchantedBy\nOracle:x\n"
)

// beamOptions lists the beam me up casts offered for id.
func beamOptions(t *testing.T, e *Engine, id state.ObjID) []decision.Option {
	t.Helper()
	var out []decision.Option
	for _, o := range castOptions(t, e) {
		if o.Mode == beamMeUpMode && o.Obj == id {
			out = append(out, o)
		}
	}
	return out
}

// beamFixture is a seat with a beam me up sorcery in its graveyard and three
// floating blue, in its main phase.
func beamFixture(t *testing.T, seed uint64, src string, extras ...string) (*Engine, Config, state.ObjID) {
	t.Helper()
	e, cfg, spell := newFixtureDeck(t, seed, src, extras...)
	e.emit(events.Event{Kind: events.MoveZone, Obj: spell, From: state.ZHand, To: state.ZGraveyard})
	addMana(t, e, 0, "UUU")
	e.Advance()
	return e, cfg, spell
}

// TestBeamMeUpReturnsACreatureThenExilesTheSpell: the keyword's reminder text
// (the CR at hand predates it). The spell comes from the graveyard for its cost,
// the returned creature goes to its owner's hand, and the spell ends in exile.
func TestBeamMeUpReturnsACreatureThenExilesTheSpell(t *testing.T) {
	t.Parallel()
	e, cfg, spell := beamFixture(t, 801, beamSpell, beamBear)
	if len(beamOptions(t, e, spell)) != 0 {
		t.Fatal("beam me up offered with no creature to return")
	}
	bear := putCreature(t, e, 0, beamBear)
	e.Advance()
	opts := beamOptions(t, e, spell)
	if len(opts) != 1 {
		t.Fatalf("beam me up options = %+v, want one", opts)
	}
	life := e.G.Players[0].Life
	submitChoices(t, e, opts[0].Index)
	submitChoices(t, e, 0) // return the Bear
	if z := e.G.Obj(bear).Zone; z != state.ZHand {
		t.Fatalf("returned creature is in %s, want its owner's hand", z)
	}
	resolveStack(t, e)
	if e.G.Players[0].Pool[state.MU] != 0 {
		t.Fatalf("pool after paying {2}{U} from {U}{U}{U}: %+v", e.G.Players[0].Pool)
	}
	if z := e.G.Obj(spell).Zone; z != state.ZExile {
		t.Fatalf("resolved beam me up spell is in %s, want exile", z)
	}
	if got := e.G.Players[0].Life; got != life+3 {
		t.Fatalf("life %d, want %d", got, life+3)
	}
	replayCheck(t, e, cfg)
}

// TestBeamMeUpCounteredSpellIsExiled: exile applies however it leaves the stack,
// as flashback's does (Remand's hand destination included).
func TestBeamMeUpCounteredSpellIsExiled(t *testing.T) {
	t.Parallel()
	e, cfg, spell := beamFixture(t, 802, beamSpell, beamBear)
	putCreature(t, e, 0, beamBear)
	e.Advance()
	submitChoices(t, e, beamOptions(t, e, spell)[0].Index)
	submitChoices(t, e, 0)
	effects.Resolve(e, &effects.Ctx{Controller: 1, Targets: []state.Target{{Obj: spell}}, TargetsOffered: true},
		card(t, "Name:Send Back\nManaCost:U\nTypes:Instant\nA:SP$ Counter | TargetType$ Spell | ValidTgts$ Card | Destination$ Hand\nOracle:x\n").Faces[0].SpellAbility())
	if z := e.G.Obj(spell).Zone; z != state.ZExile {
		t.Fatalf("countered beam me up spell is in %s, want exile", z)
	}
	replayCheck(t, e, cfg)
}

// TestBeamMeUpGrantBarsOnlyItsGrantor: a grant costs the spell's mana cost, and
// its grantor can't pay for it while another creature can. The spell's printed
// permission is separate and the grantor may pay for that one.
func TestBeamMeUpGrantBarsOnlyItsGrantor(t *testing.T) {
	t.Parallel()
	plain := "Name:Old Note\nManaCost:U\nTypes:Sorcery\nA:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	e, _, spell := beamFixture(t, 803, plain, beamGrantor, beamBear)
	grantor := putCreature(t, e, 0, beamGrantor)
	e.Advance()
	if opts := beamOptions(t, e, spell); len(opts) != 0 {
		t.Fatalf("grant offered with only its grantor to return: %+v", opts)
	}
	bear := putCreature(t, e, 0, beamBear)
	e.Advance()
	opts := beamOptions(t, e, spell)
	if len(opts) != 1 {
		t.Fatalf("grant options = %+v, want one", opts)
	}
	submitChoices(t, e, opts[0].Index)
	d := e.Pending()
	if d == nil {
		t.Fatal("no return decision")
	}
	var offered []state.ObjID
	for _, o := range d.Options {
		offered = append(offered, o.Obj)
	}
	if len(offered) != 1 || offered[0] != bear {
		t.Fatalf("return candidates %v, want only the Bear %d (grantor %d)", offered, bear, grantor)
	}

	e2, _, printed := beamFixture(t, 804, beamSpell, beamGrantor, beamBear)
	grantor2 := putCreature(t, e2, 0, beamGrantor)
	e2.Advance()
	if opts := beamOptions(t, e2, printed); len(opts) != 1 || opts[0].Cost != "" {
		t.Fatalf("with only the grantor, want the printed permission alone: %+v", opts)
	}
	putCreature(t, e2, 0, beamBear)
	e2.Advance()
	opts2 := beamOptions(t, e2, printed)
	if len(opts2) != 2 {
		t.Fatalf("printed and granted costs differ, want two options: %+v", opts2)
	}
	if opts2[0].Cost != "" {
		t.Fatalf("first option is the printed one, got cost %q", opts2[0].Cost)
	}
	submitChoices(t, e2, opts2[0].Index)
	if d := e2.Pending(); d == nil || len(d.Options) != 2 || d.Options[0].Obj != grantor2 {
		t.Fatalf("the printed permission's return candidates = %+v, want the grantor %d and the Bear", d, grantor2)
	}
}

// TestCantBeBeamedUpBarsAnEnchantedCreature: a static with no ValidSA$ bars the
// creature from every beam me up.
func TestCantBeBeamedUpBarsAnEnchantedCreature(t *testing.T) {
	t.Parallel()
	e, _, spell := beamFixture(t, 806, beamSpell, beamBear, beamMaroon)
	bear := putCreature(t, e, 0, beamBear)
	aura := moveSeeded(t, e, 0, beamMaroon, state.ZBattlefield)
	attachFixture(t, e, aura, bear)
	e.Advance()
	if opts := beamOptions(t, e, spell); len(opts) != 0 {
		t.Fatalf("enchanted creature was the only one and beam me up was offered: %+v", opts)
	}
}
