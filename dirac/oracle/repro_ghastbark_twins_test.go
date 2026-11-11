package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Ghastbark Twins' "can block an additional creature each combat" is a
// Continuous static, CanBlockAmount$ 1 on itself (CR 509.1a): kgen lowers it
// as CardDef.ExtraBlocks, so dirac offers the Twins a second attacker after
// its first and has its controller divide its combat damage between the two
// (CR 510.1d), as gorge's BlockCapacity and divisionChooser do.
//
// The game: the first player attacks with every creature once it has two
// Grizzly Bears; the second player declares the Twins against both and
// answers the division with the 2/5 split (lethal to each 2/2). reached:
// the Twins blocked two attackers and each took damage after the blocks.
func ghastbarkTwinsCase() effectLongCase {
	return effectLongCase{
		name:   "repro_ghastbark_twins_blocks_two_attackers_and_divides",
		prefer: []string{"Ghastbark Twins", "Grizzly Bears"},
		decks: [2][]string{
			append(deckOf(10, "Forest"), deckOf(12, "Grizzly Bears")...),
			append(deckOf(12, "Forest"), deckOf(8, "Ghastbark Twins")...),
		},
		surface: "autopay",
		reached: ghastbarkTwinsReached,
	}
}

// ghastbarkTwinsReached reads the finished gorge log: a DeclareBlockers event
// pairs the Twins with two different attackers, and after it each of those
// attackers was dealt damage (only the Twins blocks, so the damage is its split).
func ghastbarkTwinsReached(g *GorgeSide) bool {
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	var hit map[state.ObjID]bool
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch {
		case e.Kind == events.DeclareBlockers && hit == nil:
			var atks []state.ObjID
			for _, pr := range e.Pairs {
				if name(pr[1]) == "Ghastbark Twins" && !slices.Contains(atks, pr[0]) {
					atks = append(atks, pr[0])
				}
			}
			if len(atks) == 2 {
				hit = map[state.ObjID]bool{atks[0]: false, atks[1]: false}
			}
		case e.Kind == events.Damage && e.Amount > 0 && hit != nil:
			if _, ok := hit[e.Obj]; ok {
				hit[e.Obj] = true
			}
		}
	}
	if len(hit) != 2 {
		return false
	}
	for _, v := range hit {
		if !v {
			return false
		}
	}
	return true
}

func ghastbarkTwinsPolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	base := effectLongPolicy(m, c, seed)
	division := canon.CanonKind(slices.Index(canon.ChooseKinds, "division"))
	return PolicyFunc(func(p *PickCtx) Canon {
		switch p.D.Kind {
		case DAttackers:
			if p.D.Player == 0 {
				n := 0
				for _, o := range p.D.Options {
					if o.Kind() == ActAttack {
						n++
					}
				}
				if n >= 2 {
					for _, o := range p.D.Options {
						if o.Kind() == ActAttack {
							return o
						}
					}
				}
				for _, o := range p.D.Options {
					if o.Kind() != ActAttack {
						return o // Done
					}
				}
			}
		case DBlockers:
			for _, o := range p.D.Options {
				if o.Kind() == ActBlock && m.InstanceCard(int(o.Src())) == "Ghastbark Twins" {
					return o
				}
			}
		case DChoose:
			for _, o := range p.D.Options {
				if o.Kind() == canon.ActChoose+division && o.Mode() == 2 {
					return o
				}
			}
		}
		return base.Pick(p)
	})
}

// TestReproGhastbarkTwinsBlocksTwoReach replays the recorded game: no
// divergence, every scripted action offered, and the Twins blocks two
// attackers and divides its damage.
func TestReproGhastbarkTwinsBlocksTwoReach(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	c := ghastbarkTwinsCase()
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
		t.Fatal("the recorded game never has the Twins block two attackers and divide its damage")
	}
}

// TestReproGhastbarkTwinsBlocksTwoRecord plays the policy over seeds until one
// reaches the double block, and writes the repro. It runs only with
// GHASTBARK_RECORD=1.
func TestReproGhastbarkTwinsBlocksTwoRecord(t *testing.T) {
	if os.Getenv("GHASTBARK_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := ghastbarkTwinsCase()
	for seed := uint64(1); seed <= 400; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"},
			Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
		set := reproSet(t, "custom", r)
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, reached := effectLongPlay(t, reg, c, r, set, ghastbarkTwinsPolicy(m, c, seed))
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
	t.Fatal("no seed reached the Twins' double block")
}
