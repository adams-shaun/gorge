package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	bareLandSrc     = "Name:Bare Land\nTypes:Land\nOracle:x\n"
	artifactBearSrc = "Name:Plain Bear\nTypes:Artifact Creature Bear\nPT:4/4\nOracle:x\n"
	awakenSrc       = "Name:Awaken\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Land.YouCtrl | AddType$ Creature | SetPower$ 4 | SetToughness$ 4 | Description$ Your lands are 4/4 creatures.\nOracle:x\n"
	gildSrc         = "Name:Gild\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Land.YouCtrl | AddType$ Artifact | Description$ Your lands are artifacts.\nOracle:x\n"
	petrifySrc      = "Name:Petrify\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Permanent.Other+YouCtrl | RemoveCardTypes$ True | AddType$ Land | Description$ Your other permanents are only lands.\nOracle:x\n"
	gainLifeLine    = "A:SP$ GainLife | LifeAmount$ 1 | SpellDescription$ Gain 1 life.\n"
)

// TestTapPaymentsReadCurrentTypes pins that every tap-to-pay contribution
// reads the permanent's current types: tapping an "untapped creature" or
// "untapped artifact" (CR 702.51a, 702.126a, 702.180a, 701.67a) means what it
// is now, so a land made one qualifies and a creature that stopped being one
// does not. The offer gate must agree with the announcement, so each case
// starts from an empty pool and a cast that only the tap can pay for.
func TestTapPaymentsReadCurrentTypes(t *testing.T) {
	t.Parallel()
	const (
		convokeSpell   = "Name:Convoke Spell\nManaCost:1\nTypes:Sorcery\nK:Convoke\n" + gainLifeLine + "Oracle:x\n"
		improviseSpell = "Name:Improvise Spell\nManaCost:1\nTypes:Sorcery\nK:Improvise\n" + gainLifeLine + "Oracle:x\n"
		waterbendSpell = "Name:Waterbend Spell\nTypes:Sorcery\nS:Mode$ RaiseCost | ValidCard$ Card.Self | Type$ Spell | Cost$ Waterbend<1> | EffectZone$ All | Description$ Waterbend {1} to cast this.\n" + gainLifeLine + "Oracle:x\n"
	)
	for _, tc := range []struct {
		name, spell, static, permanent, kind string
		offered                              bool
	}{
		{"convoke an animated land", convokeSpell, awakenSrc, bareLandSrc, "convoke_generic", true},
		{"improvise an artifact land", improviseSpell, gildSrc, bareLandSrc, "improvise_generic", true},
		{"waterbend an animated land", waterbendSpell, awakenSrc, bareLandSrc, "waterbend_generic", true},
		{"waterbend an artifact land", waterbendSpell, gildSrc, bareLandSrc, "waterbend_generic", true},
		{"no convoke from an ex-creature", convokeSpell, petrifySrc, artifactBearSrc, "convoke_generic", false},
		{"no improvise from an ex-artifact", improviseSpell, petrifySrc, artifactBearSrc, "improvise_generic", false},
		{"no waterbend from an ex-creature", waterbendSpell, petrifySrc, artifactBearSrc, "waterbend_generic", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := handEngine(t, card(t, tc.spell))
			perm := addArtifacts(t, e, 1, tc.permanent)[0]
			addArtifacts(t, e, 1, tc.static)
			spell := e.G.Zone(state.ZHand, 0)[0]
			e.priorityRound()
			if !tc.offered {
				if d := e.Pending(); d != nil {
					for _, o := range d.Options {
						if o.Kind == "cast" && o.Obj == spell {
							t.Fatalf("cast offered with only a permanent that cannot pay: %+v", o)
						}
					}
				}
				return
			}
			submitChoices(t, e, castModeOption(t, e, spell, ""))
			submitChoices(t, e, tapPaymentOption(t, e, perm, tc.kind))
			if !e.G.Obj(perm).Tapped {
				t.Fatal("the announced permanent did not tap")
			}
			if e.G.Obj(spell).Zone != state.ZStack {
				t.Fatalf("spell in zone %v after the tap paid for it, want the stack", e.G.Obj(spell).Zone)
			}
		})
	}
}

// TestHarmonizeTapsAnimatedLandForItsPower pins Harmonize's creature read and
// its power together on a land that is a creature only by an effect.
func TestHarmonizeTapsAnimatedLandForItsPower(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Wild Ride"))
	land := addArtifacts(t, e, 1, bareLandSrc)[0]
	addArtifacts(t, e, 1, awakenSrc)
	e.G.Players[0].Pool[state.MR] = 1
	castMode(t, e, e.G.Zone(state.ZHand, 0)[0], "harmonize")
	option := tapPaymentOption(t, e, land, "harmonize")
	if got := e.Pending().Options[option].Amount; got != 4 {
		t.Fatalf("harmonize reduction %d, want the animated power 4", got)
	}
}

// TestTapCreaturesForManaCountsAnimatedLand pins the activation permission
// ("you may tap an untapped creature you control rather than pay that mana")
// at both the offer gate and the announcement.
func TestTapCreaturesForManaCountsAnimatedLand(t *testing.T) {
	t.Parallel()
	const tome = "Name:Tome\nTypes:Artifact\nA:AB$ GainLife | Cost$ 1 | TapCreaturesForMana$ True | LifeAmount$ 1 | SpellDescription$ Gain 1 life.\nOracle:x\n"
	e := handEngine(t)
	source := addArtifacts(t, e, 1, tome)[0]
	land := addArtifacts(t, e, 1, bareLandSrc)[0]
	addArtifacts(t, e, 1, awakenSrc)
	e.priorityRound()
	submitChoices(t, e, abilityOption(t, e, source, 0).Index)
	submitChoices(t, e, tapPaymentOption(t, e, land, "activation_creature"))
	if !e.G.Obj(land).Tapped {
		t.Fatal("the animated land did not tap for the activation")
	}
}

// TestWaterbendTapsEarthbentLand is the whole path: a land earthbent by a real
// Earthbend ability is a creature, so it may tap for a waterbend cost.
func TestWaterbendTapsEarthbentLand(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := searchEngine(t, reg, "North Pole Patrol", "Ba Sing Se", "Forest")
	_, forest := earthbendActivateBaSingSe(t, e, "Forest")
	patrol := searchMoveByName(t, e, "North Pole Patrol", state.ZBattlefield)
	e.G.Obj(patrol).SummonSick = false
	for i := 0; i < 3; i++ {
		onBoardReady(t, e, 0, "Name:Gear Bird\nManaCost:0\nTypes:Artifact Creature Bird\nPT:0/2\nOracle:x\n")
	}
	bears := onBoardReady(t, e, 1, "Name:Bear\nManaCost:0\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.priorityRound()
	d := patrolToWaterbendAsk(t, e, patrol, bears)
	for _, o := range d.Options {
		if o.Kind == "waterbend_generic" && o.Obj == forest {
			return
		}
	}
	t.Fatalf("the earthbent land is missing from the waterbend ask: %+v", d.Options)
}

// tapPaymentOption finds the pending announcement's option of kind for id.
func tapPaymentOption(t *testing.T, e *Engine, id state.ObjID, kind string) int {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatal("no announcement pending")
	}
	var seen []string
	for _, o := range d.Options {
		if o.Kind == kind && o.Obj == id {
			return o.Index
		}
		seen = append(seen, o.Kind)
	}
	t.Fatalf("no %s option for %d among [%s]", kind, id, strings.Join(seen, " "))
	return -1
}
