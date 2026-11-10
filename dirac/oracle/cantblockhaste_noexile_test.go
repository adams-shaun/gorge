package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Speed, Young Avenger's reflexive trigger grants a creature with haste "can't
// be blocked this turn except by creatures with haste" (a targeted Effect with
// no exile sweep), played decision by decision against gorge.
// TestStdSetReprosLockstep replays the repro; TestCantBlockHasteReprosReach
// requires the recorded game to reach the grant, since a lockstep entry alone
// would pass a script that never cast a noncreature spell beside Speed.
//
// The game is recorded by TestCantBlockHasteRecord (CANT_BLOCK_HASTE_RECORD=1)
// on the kernel table built from the decks: seat 0 holds Speed and Lightning
// Bolts and casts a Bolt only once Speed is out; seat 1 holds Grizzly Bears.
func cantBlockHasteCases() []effectLongCase {
	return []effectLongCase{{
		name:   "speed_young_avenger_grants_blockable_only_by_haste",
		prefer: []string{"Speed, Young Avenger", "Lightning Bolt", "Grizzly Bears"},
		decks: [2][]string{
			append(append(deckOf(6, "Speed, Young Avenger"), deckOf(6, "Lightning Bolt")...), deckOf(12, "Mountain")...),
			append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...),
		},
		surface: "autopay",
		reached: speedGranted,
	}}
}

// speedGranted: two triggers of Speed went on the stack (the cast trigger and
// the reflexive one its paid {1} makes), then seat 0 declared attackers later
// in the log.
func speedGranted(g *GorgeSide) bool {
	pushed := 0
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.TriggerPush:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Speed, Young Avenger" {
				pushed++
			}
		case events.DeclareAttackers:
			if pushed >= 2 && e.Player == 0 {
				return true
			}
		}
	}
	return false
}

func speedOut(g *GorgeSide) bool {
	if g == nil {
		return false
	}
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Speed, Young Avenger" {
			return true
		}
	}
	return false
}

// cantBlockHastePolicy is effectLongPolicy, except that no Lightning Bolt is
// cast while Speed is not on the battlefield.
func cantBlockHastePolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	inner := effectLongPolicy(m, c, seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind != DPriority || speedOut(p.G) {
			return inner.Pick(p)
		}
		d := *p.D
		d.Options = nil
		for _, o := range p.D.Options {
			if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Lightning Bolt" {
				continue
			}
			d.Options = append(d.Options, o)
		}
		return inner.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
	})
}

func TestCantBlockHasteReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range cantBlockHasteCases() {
		t.Run(c.name, func(t *testing.T) {
			cantBlockHasteReplay(t, reg, c)
		})
	}
}

func cantBlockHasteReplay(t *testing.T, reg *cards.Registry, c effectLongCase) {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c, r, printedDelveSet(t, r.Cards), script)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if !reached {
		t.Fatalf("the recorded game never reaches %s", c.name)
	}
}

// TestCantBlockHasteRecord plays the case's policy over seeds until one
// reaches the grant, and writes the repro. It runs only with
// CANT_BLOCK_HASTE_RECORD=1.
func TestCantBlockHasteRecord(t *testing.T) {
	if os.Getenv("CANT_BLOCK_HASTE_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range cantBlockHasteCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, cantBlockHastePolicy(m, c, seed))
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
