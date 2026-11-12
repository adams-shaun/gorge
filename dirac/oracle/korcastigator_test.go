package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Kor Castigator ("can't be blocked by Eldrazi Scions"): a CantBlockBy static
// whose ValidBlocker$ names two subtypes (Creature.Eldrazi+Scion). A defender
// with Call the Scions' two tokens and a Grizzly Bears is offered the Bears
// as a blocker against the attacking Castigator and neither Scion (CR 509.1b,
// 509.1c). gorge and dirac must offer the same blocks at every decision; the
// recorded game is replayed by TestKorCastigatorReproReach.
const korCastigatorRepro = "kor_castigator_cant_be_blocked_by_eldrazi_scions"

func korCastigatorCase() effectLongCase {
	return effectLongCase{
		name: korCastigatorRepro,
		decks: [2][]string{
			append(deckOf(5, "Kor Castigator"), deckOf(19, "Plains")...),
			append(append(deckOf(6, "Call the Scions"), deckOf(6, "Grizzly Bears")...), deckOf(12, "Forest")...),
		},
		surface: "autopay",
		reached: korCastigatorBlocked,
	}
}

// korCastigatorBlocked: a Grizzly Bears blocked Kor Castigator while a Scion
// stood untapped on the battlefield, and no Scion ever blocked it.
func korCastigatorBlocked(g *GorgeSide) bool {
	bears := false
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind != events.DeclareBlockers {
			continue
		}
		for _, p := range e.Pairs {
			if sideObjName(g, p[0]) != "Kor Castigator" {
				continue
			}
			switch sideObjName(g, p[1]) {
			case "Eldrazi Scion Token":
				return false
			case "Grizzly Bears":
				bears = true
			}
		}
	}
	if !bears {
		return false
	}
	for i := range g.E.G.Objs {
		o := &g.E.G.Objs[i]
		if o.Zone == state.ZBattlefield && !o.Tapped && sideObjName(g, o.ID) == "Eldrazi Scion Token" {
			return true
		}
	}
	return false
}

// korCastigatorPolicy plays lands, casts, attacks and blocks; at each block
// decision it checks that gorge offers Bears but no untapped Scion against the
// Castigator, and counts the decisions that showed it.
func korCastigatorPolicy(m *Matchup, seed uint64, witnessed *int, leaked *int) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		var keep []Canon
		for _, o := range p.D.Options {
			switch o.Kind() {
			case ActPlayLand, ActCast, ActAttack, ActBlock:
				keep = append(keep, o)
			}
		}
		if p.D.Kind == DPriority {
			for _, k := range []canon.CanonKind{ActPlayLand, ActCast} {
				for _, o := range keep {
					if o.Kind() == k {
						return o
					}
				}
			}
		}
		if p.D.Kind == DBlockers && p.G != nil {
			var bears, scion Canon
			for _, o := range keep {
				if o.Kind() != ActBlock {
					continue
				}
				if g, ok := p.G.GorgeOption(o); ok && sideObjName(p.G, g.Attacker) == "Kor Castigator" {
					switch sideObjName(p.G, g.Obj) {
					case "Grizzly Bears":
						bears = o
					case "Eldrazi Scion Token":
						scion = o
					}
				}
			}
			if scion != 0 {
				*leaked++
			}
			if bears != 0 {
				for i := range p.G.E.G.Objs {
					o := &p.G.E.G.Objs[i]
					if o.Zone == state.ZBattlefield && !o.Tapped && sideObjName(p.G, o.ID) == "Eldrazi Scion Token" {
						*witnessed++
						return bears
					}
				}
			}
		}
		if len(keep) == 0 {
			return rnd.Pick(p)
		}
		d := *p.D
		d.Options = keep
		return rnd.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
	})
}

// TestKorCastigatorReproReach replays the recorded game: no divergence, every
// scripted action offered, the Bears blocking with a Scion untapped.
func TestKorCastigatorReproReach(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	c := korCastigatorCase()
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
		t.Fatal("the recorded game never has the Bears block the Castigator with a Scion untapped")
	}
}

// TestKorCastigatorRecord plays seeds until the block decision shows a Scion
// refused and the Bears offered, and writes the repro (KOR_CASTIGATOR_RECORD=1).
func TestKorCastigatorRecord(t *testing.T) {
	if os.Getenv("KOR_CASTIGATOR_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := korCastigatorCase()
	for seed := uint64(1); seed <= 400; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
			Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
		set := reproSet(t, "custom", r)
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		witnessed, leaked := 0, 0
		res, reached := effectLongPlay(t, reg, c, r, set, korCastigatorPolicy(m, seed, &witnessed, &leaked))
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if leaked != 0 {
			t.Fatalf("seed %d: gorge offered a Scion the block against Kor Castigator", seed)
		}
		t.Logf("seed %d: reached=%v witnessed=%d decisions=%d", seed, reached, witnessed, len(res.Actions))
		if !reached || witnessed == 0 {
			continue
		}
		r.setActions(res.Actions)
		if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: %d decisions", seed, len(res.Actions))
		return
	}
	t.Fatal("no seed reached a Scion refused beside the Bears")
}
