package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// Adventure and Omen face casts from the hand in lockstep (CR 715.3a,
// 720.3a). Two recorded dr-mbc games once diverged at a face cast's offer:
//
//   - Beanstalk Giant's Fertile Footsteps ({2}{G}) beside a Forest, a Forest
//     under Abundant Growth and a Peat Bog with one depletion counter. gorge's
//     one prefix search activates every census gap in battlefield order, Peat
//     Bog's depletion rider beside the grant, so it offers the cast; the
//     kernel's grant search listed no Taps-deferred gap and dropped it
//     (dirac/mana_granttapsdefer.go; CR 601.2g-h, 605.3b).
//   - Disruptive Stormbrood's Petty Revenge with Commander Eesha (protection
//     from creatures) the only creature of power 3 or less. The Omen spell is
//     a sorcery (CR 720.3b), so it may target her (CR 702.16b); gorge judged
//     the targets with the creature front up and withheld the cast (rules
//     legal_walk_hand.go appendAlternateSpellFace).
//
// Each recording replays clean 200 decisions past its script, and gorge
// offers the face cast on the recorded board (the precondition: the replay
// reached the state the divergence was at).
func TestAdventureFaceCastLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range []struct {
		file, card, mode string
		board            func(g *GorgeSide) bool
	}{
		{"grant_search_activates_depletion_rider", "Beanstalk Giant", "adventure_alt", func(g *GorgeSide) bool {
			bog := findPerm(g, "Peat Bog")
			return bog != nil && !bog.Tapped && bog.Counter("DEPLETION") == 1 && findPerm(g, "Abundant Growth") != nil
		}},
		{"omen_targets_read_the_omen_face", "Disruptive Stormbrood", "omen_alt", func(g *GorgeSide) bool {
			return findPerm(g, "Commander Eesha") != nil
		}},
	} {
		t.Run(c.card, func(t *testing.T) {
			t.Parallel()
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.file+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			set := reproSet(t, r.Pool, r)
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			entries := func(names []string) []cardset.Entry {
				es := make([]cardset.Entry, len(names))
				for i, n := range names {
					es[i] = cardset.Entry{Name: n, Count: 1}
				}
				return es
			}
			tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
			if err != nil {
				t.Fatal(err)
			}
			rc := r.RunConfig()
			rc.Config.GorgeAutopaySurface = r.Surface != "full"
			rc.MaxDecisions = len(r.Actions) + 200
			offered := false
			inner := rc.Check
			rc.Check = func(ref *GorgeSide, ker Side) *Divergence {
				if !offered && offersFace(ref, c.card, c.mode) && c.board(ref) {
					offered = true
				}
				if inner != nil {
					return inner(ref, ker)
				}
				return nil
			}
			k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
			if err != nil {
				t.Fatal(err)
			}
			defer CloseSide(k)
			res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
			if res.Div != nil {
				t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
			}
			stallCheck(t, r, res, false)
			if !offered {
				t.Fatalf("gorge never offered %s's %s cast on the recorded board", c.card, c.mode)
			}
		})
	}
}

// offersFace reports whether the current canonical frame offers card's face
// cast of the given mode: on the autopay surface a face cast the pool cannot
// pay is a priced potential play, not one of the engine decision's options,
// so each canonical option is read back as the gorge option it stands for.
func offersFace(g *GorgeSide, card, mode string) bool {
	if g.fr == nil {
		return false
	}
	for _, a := range g.fr.cur.Options {
		o, ok := g.GorgeOption(a)
		if !ok || o.Kind != "cast" || o.Mode != mode {
			continue
		}
		if obj := g.E.G.Obj(o.Obj); obj != nil && obj.Card != nil && len(obj.Card.Faces) > 0 && obj.Card.Faces[0].Name == card {
			return true
		}
	}
	return false
}

// findPerm is the first permanent named name on either battlefield, nil none.
func findPerm(g *GorgeSide, name string) *state.Object {
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
			if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
				return o
			}
		}
	}
	return nil
}
