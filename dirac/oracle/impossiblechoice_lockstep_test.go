package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestImpossibleChoiceModesLockstep plays both modes of Impossible Choice
// (a charm: "Surgical Separation" destroys target creature and its controller
// makes two 1/1 red Officer tokens; "Remain Fused" draws two cards and each
// player loses 2 life) from the real corpus card, in lockstep. Player 0 casts
// it at the opponent's Emergency Medical Hologram (mode 0) or as soon as it can
// (mode 1). The mode's own effect is checked on gorge's state once the spell has
// resolved: the Officers go to the destroyed creature's controller, not the
// caster, and the draw and the life loss land on both players.
func TestImpossibleChoiceModesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	entries := [2][]cardset.Entry{
		{{Name: "Impossible Choice", Count: 8}, {Name: "Swamp", Count: 32}},
		{{Name: "Emergency Medical Hologram", Count: 12}, {Name: "Plains", Count: 28}},
	}
	for mode, name := range []string{"Surgical Separation", "Remain Fused"} {
		t.Run(name, func(t *testing.T) {
			reg := synthReg(t)
			set := cardsettest.Synth(t, "std-trk")
			decks, tbl := mccoyBoard(t, reg, set, entries)
			fired := 0
			for seed := uint64(1); seed <= 48 && fired < 3; seed++ {
				m, err := NewMatchup(reg, [2]string{"choice-a", "choice-b"}, decks, 0)
				if err != nil {
					t.Fatal(err)
				}
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				sc := &choiceScenario{mode: mode}
				res := Lockstep(m, seed, k, sc, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 14, Check: sc.check})
				CloseSide(k)
				if res.Div != nil {
					t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
						seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail,
						res.Div.RefOptions, res.Div.KerOptions)
				}
				if sc.err != "" {
					t.Fatalf("seed %d: %s", seed, sc.err)
				}
				if sc.cast && !sc.checked {
					t.Fatalf("seed %d: Impossible Choice was cast but never seen resolved", seed)
				}
				if sc.checked {
					fired++
					t.Logf("seed %d: %s resolved as expected, no divergence", seed, name)
				}
			}
			if fired == 0 {
				t.Fatalf("no seed resolved Impossible Choice in mode %q", name)
			}
		})
	}
}

// choiceSnap is the board when Impossible Choice is cast.
type choiceSnap struct {
	hand   int
	life   [2]int32
	tokens [2]int
	holos  int
}

type choiceScenario struct {
	mode    int
	cast    bool
	checked bool
	snap    choiceSnap
	err     string
}

func choiceTake(g *state.Game) choiceSnap {
	s := choiceSnap{hand: len(g.Zone(state.ZHand, 0))}
	for p := 0; p < 2; p++ {
		s.life[p] = g.Players[p].Life
	}
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone != state.ZBattlefield || o.Card == nil && !o.IsToken {
			continue
		}
		if o.IsToken {
			s.tokens[o.Controller]++
		} else if f := o.Face(); f != nil && f.Name == "Emergency Medical Hologram" && o.Controller == 1 {
			s.holos++
		}
	}
	return s
}

func (p *choiceScenario) Pick(c *PickCtx) Canon {
	d := c.D
	g := c.G.E.G
	switch d.Kind {
	case DMulligan:
		return pickKind(c, ActKeep)
	case DTarget:
		for i := range g.Objs {
			o := &g.Objs[i]
			if o.Zone != state.ZBattlefield || o.Controller != 1 || o.IsToken || o.Card == nil {
				continue
			}
			ref := c.G.InstRef(o.ID)
			if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActTarget && a.Tgt() == ref }); ok {
				return a
			}
		}
		return d.Options[0]
	case DModes:
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActMode && int(a.Mode()) == p.mode }); ok {
			return a
		}
		return d.Options[0]
	case DAttackers, DBlockers:
		return pickKind(c, ActDone)
	case DPriority:
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPlayLand }); ok {
			return a
		}
		if d.Player == 1 {
			if a, ok := reproHandCastAction(c, g, 1, "Emergency Medical Hologram"); ok {
				return a
			}
			return pickKind(c, ActPass)
		}
		if p.cast || len(g.Stack) != 0 {
			return pickKind(c, ActPass)
		}
		snap := choiceTake(g)
		if p.mode == 0 && snap.holos == 0 {
			return pickKind(c, ActPass)
		}
		if a, ok := reproHandCastAction(c, g, 0, "Impossible Choice"); ok {
			p.cast, p.snap = true, snap
			return a
		}
		return pickKind(c, ActPass)
	}
	return d.Options[0]
}

// check runs once, at the first decision after Impossible Choice has left the
// stack for the graveyard.
func (p *choiceScenario) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	if !p.cast || p.checked || len(g.Stack) != 0 {
		return nil
	}
	resolved := false
	for _, id := range g.Zone(state.ZGraveyard, 0) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Impossible Choice" {
			resolved = true
		}
	}
	if !resolved {
		return nil
	}
	p.checked = true
	s, now := p.snap, choiceTake(g)
	var bad []string
	if s.hand == 0 {
		bad = append(bad, "precondition: no hand at the cast")
	}
	if p.mode == 0 {
		if s.holos == 0 || s.tokens != [2]int{} {
			bad = append(bad, "precondition: want an opposing creature and no tokens before the cast")
		}
		if now.tokens[1] != s.tokens[1]+2 || now.tokens[0] != s.tokens[0] {
			bad = append(bad, "Officers: want +2 for player 1 and +0 for the caster, tokens "+choiceFmt(s.tokens)+" -> "+choiceFmt(now.tokens))
		}
		if now.holos != s.holos-1 {
			bad = append(bad, "the targeted creature was not destroyed")
		}
		if now.hand != s.hand-1 || now.life != s.life {
			bad = append(bad, "hand and life should change only by the cast card")
		}
	} else {
		if now.hand != s.hand+1 { // -1 for the spell, +2 drawn
			bad = append(bad, "caster's hand: want the spell cast and two drawn")
		}
		if now.life[0] != s.life[0]-2 || now.life[1] != s.life[1]-2 {
			bad = append(bad, "each player should lose 2 life")
		}
		if now.tokens != s.tokens {
			bad = append(bad, "Remain Fused made tokens")
		}
	}
	if len(bad) > 0 {
		p.err = strings.Join(bad, "; ")
	}
	return nil
}

func choiceFmt(t [2]int) string {
	return string(rune('0'+t[0])) + "/" + string(rune('0'+t[1]))
}
