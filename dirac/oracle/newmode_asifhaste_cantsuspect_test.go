package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Two statics played decision by decision against gorge:
// Shang-Chi, Master of Kung Fu's "activate abilities of creatures you control
// as though those creatures had haste" (a summoning-sick Prodigal Sorcerer
// pings the turn it entered), and Airtight Alibi's "enchanted creature can't
// become suspected" (a Repeat Offender wearing it activates its own suspect).
// TestStdSetReprosLockstep replays the repros; TestNewModeAsIfHasteCantSuspectReach
// requires each to reach the static's effect and the mutation switch to make
// it diverge. The games are recorded by TestNewModeAsIfHasteCantSuspectRecord
// (NEWMODE_ASIFHASTE_RECORD=1).
type newModeCase struct {
	effectLongCase
	act    string // the card whose activation the policy prefers
	mutate *bool  // the kernel mutation switch the repro must catch
}

func newModeCases() []newModeCase {
	return []newModeCase{
		{
			effectLongCase: effectLongCase{
				name:   "as_if_haste_sick_creature_activates_its_tap_ability",
				prefer: []string{"Shang-Chi, Master of Kung Fu", "Prodigal Sorcerer"},
				decks: [2][]string{
					append(deckOf(8, "Shang-Chi, Master of Kung Fu"), append(deckOf(8, "Prodigal Sorcerer"), append(deckOf(8, "Forest"), deckOf(8, "Island")...)...)...),
					deckOf(24, "Plains"),
				},
				surface: "autopay",
				reached: tappedTheTurnItEntered("Prodigal Sorcerer"),
			},
			act:    "Prodigal Sorcerer",
			mutate: &dirac.DebugAsIfHasteOff,
		},
		{
			effectLongCase: effectLongCase{
				name:   "cant_be_suspected_enchanted_creature_suspects_itself",
				prefer: []string{"Repeat Offender", "Airtight Alibi"},
				decks: [2][]string{
					append(deckOf(8, "Repeat Offender"), append(deckOf(6, "Airtight Alibi"), append(deckOf(9, "Swamp"), deckOf(9, "Forest")...)...)...),
					deckOf(24, "Plains"),
				},
				surface: "autopay",
				reached: activatedWhileAttached("Airtight Alibi"),
			},
			act:    "Repeat Offender",
			mutate: &dirac.DebugCantSuspectOff,
		},
	}
}

// objName is the front-face name of gorge object id ("" when gone).
func objName(g *GorgeSide, id state.ObjID) string {
	if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return ""
}

// tappedTheTurnItEntered: a permanent named name tapped in the turn it
// entered the battlefield (no turn began between its entry and the tap), so
// it paid a {T} cost while summoning sick.
func tappedTheTurnItEntered(name string) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		entered := map[state.ObjID]bool{}
		for _, e := range g.E.L.Events {
			switch {
			case e.Kind == events.TurnChange:
				clear(entered)
			case e.Kind == events.MoveZone && e.To == state.ZBattlefield && objName(g, e.Obj) == name:
				entered[e.Obj] = true
			case e.Kind == events.Tap && entered[e.Obj]:
				return true
			}
		}
		return false
	}
}

// activatedWhileAttached: the unsuspected permanent the Aura named aura is
// attached to puts an activated ability on the stack (Repeat Offender's
// suspect-or-grow), and never becomes suspected while it wears the Aura.
func activatedWhileAttached(aura string) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		bearers := map[state.ObjID]bool{}
		suspected := map[state.ObjID]bool{}
		reached := false
		for _, e := range g.E.L.Events {
			switch {
			case e.Kind == events.Attach && len(e.IDs) == 1 && objName(g, e.Obj) == aura:
				bearers[e.IDs[0]] = true
			case e.Kind == events.MoveZone && e.From == state.ZBattlefield:
				delete(bearers, e.Obj)
			case e.Kind == events.AlterAttribute && e.Text == "Suspected":
				if e.Amount > 0 && bearers[e.Obj] {
					return false
				}
				suspected[e.Obj] = e.Amount > 0
			case e.Kind == events.AbilityPush && bearers[e.Obj] && !suspected[e.Obj]:
				reached = true
			}
		}
		return reached
	}
}

// newModePolicy prefers an activation of the case's act card, then
// effectLongPolicy's casts and lands, then the seed's random pick.
func newModePolicy(m *Matchup, c newModeCase, seed uint64) Policy {
	inner := effectLongPolicy(m, c.effectLongCase, seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority {
			for _, o := range p.D.Options {
				if o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == c.act {
					return o
				}
			}
		}
		return inner.Pick(p)
	})
}

// TestNewModeAsIfHasteCantSuspectReach replays the recorded games: no
// divergence, every scripted action offered, the static's effect reached;
// with the kernel's mutation switch on, the same script diverges.
func TestNewModeAsIfHasteCantSuspectReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range newModeCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			set := printedDelveSet(t, r.Cards)
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatalf("the recorded game never reaches %s", c.name)
			}
			*c.mutate = true
			defer func() { *c.mutate = false }()
			res, _ = effectLongPlay(t, reg, c.effectLongCase, r, set, &ScriptPolicy{Actions: r.Actions})
			if res.Div == nil {
				t.Fatal("the mutation switch did not make the recorded game diverge")
			}
		})
	}
}

// TestNewModeAsIfHasteCantSuspectRecord plays the cases' policies over seeds
// until one reaches the static's effect, and writes the repro. It runs only
// with NEWMODE_ASIFHASTE_RECORD=1; the games it writes are the committed
// repros.
func TestNewModeAsIfHasteCantSuspectRecord(t *testing.T) {
	if os.Getenv("NEWMODE_ASIFHASTE_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range newModeCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 200; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, newModePolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if !reached {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions", seed, len(res.Actions))
				return
			}
			t.Fatalf("no seed reached %s", c.name)
		})
	}
}
