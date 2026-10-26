package rules

// cr-g201: a may-play static's ReplaceGraveyard$ Exile rider ("if a spell cast
// this way would be put into your graveyard, exile it instead") was unread.
// Maestros Ascendancy (SNC) and Kess, Dissident Mage carry it on their
// Mode$ Continuous MayPlay$ grant. Forge installs the same replacement a DB$
// Play's rider uses when a spell is put on the stack through a may-play static
// with the key, so beginCast records it on the pending cast and the
// payCast/spellRestZone/spellFizzleZone path sends the card to exile (and the
// counter/ward destination reader does too).

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	// mrgSacSrc is the creature Maestros Ascendancy's RaiseCost$ Sac<1/Creature>
	// charges. It is seeded into seat 0's deck and moved to the battlefield with a
	// logged MoveZone so the log-only replay can rebuild it.
	mrgSacSrc  = "Name:Sac Lamb\nManaCost:1\nTypes:Creature Goat\nPT:0/1\nOracle:x\n"
	mrgSacName = "Sac Lamb"
	// mrgPlainSrc is Maestros Ascendancy's static line with the
	// ReplaceGraveyard$ Exile rider omitted: the negative half must still rest
	// the resolved spell in the graveyard.
	mrgPlainSrc = "Name:Plain Grave Vault\nManaCost:1\nTypes:Enchantment\n" +
		"S:Mode$ Continuous | Affected$ Instant.YouCtrl,Sorcery.YouCtrl | Condition$ PlayerTurn | MayPlay$ True | RaiseCost$ Sac<1/Creature> | MayPlayLimit$ 1 | EffectZone$ Battlefield | AffectedZone$ Graveyard | Description$ x\nOracle:x\n"
	// mrgCounterSrc is the fixture counterspell seat 1 answers the may-play Bolt
	// with; the countered spell must still be exiled by the rider.
	mrgCounterSrc  = "Name:Test Dismiss\nManaCost:U\nTypes:Instant\nA:SP$ Counter | TargetType$ Spell | ValidTgts$ Card | SpellDescription$ Counter target spell.\nOracle:x\n"
	mrgCounterName = "Test Dismiss"
)

// mrgBoard builds the cr-g201 scenario: seat 0 has the grant permanent and a
// creature to sacrifice on the battlefield, Lightning Bolt in the graveyard,
// {R} in the pool, and seat 1 has the fixture counterspell in hand and {U} in
// the pool. It is seat 0's turn, main 1, with seat 0 holding priority. The
// returned Config is the replay genesis.
func mrgBoard(t *testing.T, grant *cards.Card, seatOneCounter bool) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	bolt := searchCorpusCard(t, reg, "Lightning Bolt")
	mountain := searchCorpusCard(t, reg, "Mountain")

	deck0 := []*cards.Card{grant, bolt, card(t, mrgSacSrc)}
	for len(deck0) < 40 {
		deck0 = append(deck0, mountain)
	}
	deck1 := []*cards.Card{card(t, mrgCounterSrc)}
	for len(deck1) < 40 {
		deck1 = append(deck1, mountain)
	}
	cfg := seatZeroStart(Config{Seed: 9301, Names: []string{"maestros", "opponent"},
		Decks: [][]*cards.Card{deck0, deck1}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)

	grantID := searchMoveByName(t, e, grant.Faces[0].Name, state.ZBattlefield)
	sacID := searchMoveByName(t, e, mrgSacName, state.ZBattlefield)
	boltID := searchMoveByName(t, e, "Lightning Bolt", state.ZGraveyard)
	if seatOneCounter {
		negMoveByName(t, e, 1, mrgCounterName, state.ZHand)
		addMana(t, e, 1, "U")
	}
	addMana(t, e, 0, "R")

	// Preconditions: the rule looks at all three zones and at whose turn it is.
	if o := e.G.Obj(grantID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("grant permanent zone = %v, want the battlefield", o)
	}
	if o := e.G.Obj(sacID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("sacrifice creature zone = %v, want the battlefield", o)
	}
	if o := e.G.Obj(boltID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("Lightning Bolt zone = %v, want the graveyard (the granted zone)", o)
	}
	if e.G.Active != 0 || e.G.Step != state.StepMain1 {
		t.Fatalf("turn state = active %d step %v, want seat 0's main 1", e.G.Active, e.G.Step)
	}
	if got := e.G.Players[0].Pool.Total(); got != 1 {
		t.Fatalf("seat 0 pool = %d, want 1 ({{R}})", got)
	}
	return e, cfg, boltID, sacID
}

// mrgCastMayPlay asserts the may-play cast of bolt is offered and takes it.
func mrgCastMayPlay(t *testing.T, e *Engine, bolt state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("pending = %+v, want seat 0's priority", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == bolt && o.Mode == "mayplay" {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no may-play cast offer for the graveyard Bolt: %+v", d.Options)
	}
	submitChoices(t, e, idx)
}

// mrgDrive answers the scenarios' asks until the Bolt has settled in want and
// the stack is empty. seatOneCounter makes seat 1 cast the fixture counterspell
// at the Bolt when it next holds priority. It reports whether the raise's
// sacrifice ask and the Bolt's target ask were both seen.
func mrgDrive(t *testing.T, e *Engine, bolt, sac state.ObjID, want state.Zone, seatOneCounter bool) (sawSac, sawTarget bool) {
	t.Helper()
	for i := 0; i < 80; i++ {
		if o := e.G.Obj(bolt); o != nil && o.Zone == want && len(e.G.Stack) == 0 {
			return sawSac, sawTarget
		}
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while the Bolt was in %v", e.G.Obj(bolt).Zone)
		}
		switch d.Kind {
		case decision.KPriority:
			if seatOneCounter && d.Player == 1 {
				for _, o := range d.Options {
					if o.Kind == "cast" && o.Mode == "" && e.G.Obj(o.Obj) != nil &&
						e.G.Obj(o.Obj).Face().Name == mrgCounterName {
						submitChoices(t, e, o.Index)
						goto next
					}
				}
			}
			submitPass(t, e)
		case decision.KChoose:
			if len(d.Options) > 0 && d.Options[0].Kind == "sacrifice" {
				idx := -1
				for _, o := range d.Options {
					if o.Obj == sac {
						idx = o.Index
					}
				}
				if idx < 0 {
					t.Fatalf("sacrifice ask does not offer the raise's creature: %+v", d.Options)
				}
				submitChoices(t, e, idx)
				sawSac = true
				continue
			}
			submitChoices(t, e, d.Options[0].Index)
		case decision.KTarget:
			// The Bolt targets the opponent; the counterspell targets the Bolt.
			idx := -1
			for _, o := range d.Options {
				if o.Obj == bolt {
					idx = o.Index
				}
			}
			if idx < 0 {
				for _, o := range d.Options {
					if o.Kind == "player" && o.Player == 1 {
						idx = o.Index
					}
				}
			}
			if idx < 0 {
				t.Fatalf("target ask offers neither the Bolt nor the opponent: %+v", d.Options)
			}
			submitChoices(t, e, idx)
			sawTarget = true
		default:
			t.Fatalf("unexpected decision %d: kind=%s resume=%q", i, d.Kind, d.ResumeKind)
		}
	next:
	}
	t.Fatalf("the scenario did not settle within 80 decisions (Bolt in %v)", e.G.Obj(bolt).Zone)
	return false, false
}

// mrgCastInfoHasReplaceGraveyard reports whether the Bolt's pay-time CastInfo
// carries the replacegraveyard counter (the provenance the resting-zone reader
// consumes).
func mrgCastInfoHasReplaceGraveyard(e *Engine, bolt state.ObjID) bool {
	for _, info := range castInfosFor(e, bolt) {
		if len(info) > 0 && strings.Contains(info[0], "replacegraveyard") {
			return true
		}
	}
	return false
}

// TestMaestrosAscendancyCastIsExiledOnResolution: the real corpus Maestros
// Ascendancy grants the graveyard cast and the raise is charged (the creature
// is sacrificed); the resolved Bolt damages the opponent and then rests in
// EXILE, not the graveyard, because the grant carries ReplaceGraveyard$ Exile.
func TestMaestrosAscendancyCastIsExiledOnResolution(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg, bolt, sac := mrgBoard(t, searchCorpusCard(t, reg, "Maestros Ascendancy"), false)
	mrgCastMayPlay(t, e, bolt)
	sawSac, sawTarget := mrgDrive(t, e, bolt, sac, state.ZExile, false)
	if !sawSac {
		t.Fatal("the RaiseCost$ Sac<1/Creature> was never asked (the offered cast was not the raised one)")
	}
	if !sawTarget {
		t.Fatal("the Bolt never posed its target ask")
	}
	if life := e.G.Players[1].Life; life != 17 {
		t.Fatalf("opponent life = %d, want 17 (the Bolt resolved)", life)
	}
	if o := e.G.Obj(sac); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("the sacrificed creature zone = %v, want the graveyard (the raise was paid)", o)
	}
	if o := e.G.Obj(bolt); o == nil || o.Zone != state.ZExile {
		t.Fatalf("the resolved Bolt zone = %v, want exile (the grant's ReplaceGraveyard$ Exile)", o)
	}
	if !mrgCastInfoHasReplaceGraveyard(e, bolt) {
		t.Fatal("the Bolt's cast_info carries no replacegraveyard flag")
	}
	replayCheck(t, e, cfg)
}

// TestMaestrosAscendancyCounteredSpellIsExiled: the same may-play cast is
// countered by seat 1; the counter/ward destination reader
// (state.ExilesInsteadOfGraveyard) reads the same flag, so a spell that never
// resolves still goes to exile instead of back to the graveyard.
func TestMaestrosAscendancyCounteredSpellIsExiled(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg, bolt, sac := mrgBoard(t, searchCorpusCard(t, reg, "Maestros Ascendancy"), true)
	mrgCastMayPlay(t, e, bolt)
	sawSac, _ := mrgDrive(t, e, bolt, sac, state.ZExile, true)
	if !sawSac {
		t.Fatal("the RaiseCost$ Sac<1/Creature> was never asked")
	}
	if o := e.G.Obj(bolt); o == nil || o.Zone != state.ZExile {
		t.Fatalf("the countered Bolt zone = %v, want exile (the grant's ReplaceGraveyard$ Exile)", o)
	}
	if o := e.G.Obj(sac); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("the sacrificed creature zone = %v, want the graveyard (the raise was paid)", o)
	}
	if !mrgCastInfoHasReplaceGraveyard(e, bolt) {
		t.Fatal("the counter-targeted Bolt's cast_info carries no replacegraveyard flag")
	}
	replayCheck(t, e, cfg)
}

// TestPlainGraveyardMayPlayStillGoesToGraveyard: the same static shape WITHOUT
// ReplaceGraveyard$ Exile must not exile anything -- the rider is per-static,
// never a may-play-mode default.
func TestPlainGraveyardMayPlayStillGoesToGraveyard(t *testing.T) {
	t.Parallel()
	e, cfg, bolt, sac := mrgBoard(t, card(t, mrgPlainSrc), false)
	mrgCastMayPlay(t, e, bolt)
	sawSac, _ := mrgDrive(t, e, bolt, sac, state.ZGraveyard, false)
	if !sawSac {
		t.Fatal("the RaiseCost$ Sac<1/Creature> was never asked (the offer was dead)")
	}
	if o := e.G.Obj(bolt); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("the rider-less may-play Bolt zone = %v, want the graveyard", o)
	}
	if mrgCastInfoHasReplaceGraveyard(e, bolt) {
		t.Fatal("the rider-less grant stamped the replacegraveyard flag")
	}
	replayCheck(t, e, cfg)
}
