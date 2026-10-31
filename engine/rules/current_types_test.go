package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/combat"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	enshrineSrc  = "Name:Enshrine\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Land | AddType$ Enchantment | Description$ Lands are enchantments.\nOracle:x\n"
	awakenAllSrc = "Name:Awaken All\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Land | AddType$ Creature | SetPower$ 4 | SetToughness$ 4 | Description$ Lands are 4/4 creatures.\nOracle:x\n"
	gildAllSrc   = "Name:Gild All\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Land | AddType$ Artifact | Description$ Lands are artifacts.\nOracle:x\n"
	stripBearSrc = "Name:Strip Bear\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Card.namedPlain Bear | RemoveCardTypes$ True | AddType$ Land | Description$ Plain Bears are only lands.\nOracle:x\n"
	bondedSrc    = "Name:Bonded\nManaCost:G\nTypes:Creature Human\nPT:1/1\nK:Soulbond\nOracle:x\n"
	bigCrySrc    = "Name:Big Cry\nManaCost:G\nTypes:Sorcery\nS:Mode$ CastWithFlash | ValidCard$ Card.Self | ValidSA$ Spell | EffectZone$ All | Caster$ You | Condition$ Ferocious | Description$ With a big creature, cast this any time.\n" + gainLifeLine + "Oracle:x\n"
)

// TestFerociousFlashReadsCurrentCreatures pins that "a creature with power 4
// or greater" is what the permanent is now (CR 613.1d).
func TestFerociousFlashReadsCurrentCreatures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		board   []string
		allowed bool
	}{
		{"an animated land counts", []string{bareLandSrc, awakenSrc}, true},
		{"a former creature does not", []string{artifactBearSrc, stripBearSrc}, false},
		{"a printed creature counts", []string{artifactBearSrc}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := handEngine(t, card(t, bigCrySrc))
			for _, src := range tc.board {
				onBoard(t, e, 0, src)
			}
			id := e.G.Zone(state.ZHand, 0)[0]
			st := e.G.Obj(id).Face().Statics[0]
			sv := staticView{Source: id, Controller: 0, Params: st.Params, PS: st.ParamSetOf()}
			if got := e.staticTimingGate(sv); got != tc.allowed {
				t.Fatalf("flash permission = %v, want %v", got, tc.allowed)
			}
		})
	}
}

// TestMinAllBlockCountsCurrentCreatures pins that a "can't be blocked unless
// all creatures block it" declaration counts the defender's current creatures
// (CR 509.1b, 613.1d), the same read CanBlock makes of each candidate.
func TestMinAllBlockCountsCurrentCreatures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		board []string
		want  int
	}{
		{"an animated land counts", []string{bareLandSrc, awakenSrc}, 1},
		{"a former creature does not", []string{artifactBearSrc, stripBearSrc}, 0},
		{"a printed creature counts", []string{artifactBearSrc}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := handEngine(t)
			for _, src := range tc.board {
				onBoard(t, e, 1, src)
			}
			if got := combat.DefenderCreatureCount(asBoard(e), 1); got != tc.want {
				t.Fatalf("DefenderCreatureCount = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestProtectionFromATypeReadsTheSourceNow pins CR 702.16a: protection from a
// card type asks what the source is at that moment.
func TestProtectionFromATypeReadsTheSourceNow(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, keyword, source, static string
		protected                     bool
	}{
		{"an animated land is a creature", "creatures", bareLandSrc, awakenAllSrc, true},
		{"a former creature is not", "creatures", artifactBearSrc, stripBearSrc, false},
		{"a land made an artifact is one", "artifacts", bareLandSrc, gildAllSrc, true},
		{"a former artifact is not", "artifacts", artifactBearSrc, stripBearSrc, false},
		{"a land made an enchantment is one", "enchantments", bareLandSrc, enshrineSrc, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := handEngine(t)
			target := onBoard(t, e, 1, "Name:Warded\nTypes:Creature Human\nPT:1/1\nK:Protection from "+tc.keyword+"\nOracle:x\n")
			source := onBoard(t, e, 0, tc.source)
			onBoard(t, e, 0, tc.static)
			if got := e.protectedFrom(target, source); got != tc.protected {
				t.Fatalf("protectedFrom = %v, want %v", got, tc.protected)
			}
		})
	}
}

// TestCloseEncounterChoosesACurrentCreature pins the choose-a-creature arm of
// the additional cost: "a creature you control" is what the permanent is now,
// and the damage is its power then.
func TestCloseEncounterChoosesACurrentCreature(t *testing.T) {
	castable := func(t *testing.T, e *Engine, spell state.ObjID) int {
		t.Helper()
		addMana(t, e, 0, "G1")
		for _, opt := range e.Pending().Options {
			if opt.Kind == "cast" && opt.Obj == spell {
				return opt.Index
			}
		}
		return -1
	}
	t.Run("an animated land can be chosen", func(t *testing.T) {
		e, _ := paidCostEngine(t, []string{"Close Encounter"}, []string{"Ancient Brontodon"})
		spell := paidCostMoveTo(t, e, 0, "Close Encounter", state.ZHand)
		receiver := paidCostMoveTo(t, e, 1, "Ancient Brontodon", state.ZBattlefield)
		onBoard(t, e, 0, bareLandSrc)
		onBoard(t, e, 0, awakenSrc)
		pick := castable(t, e, spell)
		if pick < 0 {
			t.Fatalf("an animated land did not enable the cost: %+v", e.Pending().Options)
		}
		submitChoices(t, e, pick)
		submitChoices(t, e, optionForObj(e.Pending(), receiver))
		passUntilStackEmpty(t, e, 40)
		if got := e.G.Obj(receiver).Damage; got != 4 {
			t.Fatalf("damage %d, want the animated land's power 4", got)
		}
	})
	t.Run("a former creature cannot", func(t *testing.T) {
		e, _ := paidCostEngine(t, []string{"Close Encounter"}, []string{"Ancient Brontodon"})
		spell := paidCostMoveTo(t, e, 0, "Close Encounter", state.ZHand)
		paidCostMoveTo(t, e, 1, "Ancient Brontodon", state.ZBattlefield)
		onBoard(t, e, 0, artifactBearSrc)
		onBoard(t, e, 0, stripBearSrc)
		if castable(t, e, spell) >= 0 {
			t.Fatal("a permanent that is no longer a creature enabled the cost")
		}
	})
}

// TestSoulbondPairsCurrentCreatures pins CR 702.95a and c: the partner is an
// unpaired creature as it is now, whatever the card prints.
func TestSoulbondPairsCurrentCreatures(t *testing.T) {
	t.Parallel()
	enterBonded := func(t *testing.T, e *Engine) *decision.Decision {
		t.Helper()
		bonded := e.G.AddObject(card(t, bondedSrc), 0)
		e.G.SetZone(state.ZLibrary, 0, append([]state.ObjID{bonded.ID}, e.G.Zone(state.ZLibrary, 0)...))
		e.emit(events.Event{Kind: events.MoveZone, Obj: bonded.ID, From: state.ZLibrary, To: state.ZBattlefield})
		e.priorityRound()
		return passToDecision(t, e, 8)
	}
	t.Run("an animated land can be a partner", func(t *testing.T) {
		t.Parallel()
		e, _, _ := newFixtureDeck(t, 196, "Name:Blank\nTypes:Sorcery\nOracle:x\n")
		land := onBoard(t, e, 0, bareLandSrc)
		onBoard(t, e, 0, awakenSrc)
		d := enterBonded(t, e)
		if d == nil || d.Kind != decision.KChoose || len(d.Options) != 1 || d.Options[0].Obj != land {
			t.Fatalf("partner choices = %+v, want only the animated land", d)
		}
	})
	t.Run("a former creature cannot", func(t *testing.T) {
		t.Parallel()
		e, _, _ := newFixtureDeck(t, 196, "Name:Blank\nTypes:Sorcery\nOracle:x\n")
		bear := onBoard(t, e, 0, artifactBearSrc)
		onBoard(t, e, 0, stripBearSrc)
		d := enterBonded(t, e)
		if d != nil && d.Kind == decision.KChoose {
			for _, o := range d.Options {
				if o.Obj == bear {
					t.Fatalf("a permanent that is no longer a creature was offered: %+v", d)
				}
			}
		}
	})
}
