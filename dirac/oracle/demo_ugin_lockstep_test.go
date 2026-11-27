package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Ugin lockstep: Ugin, the Ineffable's [+1] exiles the top card face down,
// makes a 2/2 Spirit and registers a permanent Effect that may look at the
// exiled card and returns it to hand when the Spirit leaves. The pool's
// removal (Shock, Unsummon, Doom Blade) is the token's way off the
// battlefield, so the scan requires the return to happen, not merely a
// divergence-free game. Only seat 0 has Ugin, so two face-down cards alive at
// once can only come from that one source's second [+1]: the older Effect's
// later self-exile must not end the newer registration (or vice versa) and
// the long enough game reaches that line.
const demoUginPool = "demougin"

var demoUginDecks = [2][]cardset.Entry{
	{
		{Name: "Ugin, the Ineffable", Count: 1}, {Name: "Shock", Count: 8}, {Name: "Unsummon", Count: 4},
		{Name: "Mountain", Count: 14}, {Name: "Island", Count: 13},
	},
	{
		{Name: "Shock", Count: 8}, {Name: "Doom Blade", Count: 4},
		{Name: "Mountain", Count: 15}, {Name: "Swamp", Count: 13},
	},
}

// uginScan counts the face-down cards Ugin exiled and how many of them came
// back to hand, tracking each object by id. peak is the largest number of
// still-exiled imprinted cards seen at once over the games, which requires
// two live Effects from the one Ugin.
func uginScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	faceDown := map[state.ObjID]bool{}
	for _, ev := range g.E.L.Events {
		if ev.Kind != events.MoveZone {
			continue
		}
		if ev.To == state.ZExile && strings.Contains(ev.Counter, "exiled_with_face_down") {
			faceDown[ev.Obj] = true
			n["exiled"]++
			if len(faceDown) > n["peak"] {
				n["peak"] = len(faceDown)
			}
			continue
		}
		if ev.From == state.ZExile && ev.To == state.ZHand && faceDown[ev.Obj] {
			delete(faceDown, ev.Obj)
			n["returned"]++
		}
	}
}

// TestUginIneffableEffectLockstep: no divergence over 64 seeds, the imprinted
// token left the battlefield with the exiled card returned to hand, and at
// least one game held two imprints at once (the second [+1] before the first
// Effect ended).
func TestUginIneffableEffectLockstep(t *testing.T) {
	reg := synthReg(t)
	set, err := cardset.RawByName(demoUginPool)
	if err != nil {
		t.Fatal(err)
	}
	var cs [2][]*cards.Card
	for s := range cs {
		for _, e := range demoUginDecks[s] {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(demoUginDecks[0], demoUginDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	branch := map[string]int{}
	clean := 0
	for seed := uint64(1); seed <= 64; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{demoUginPool + "-a", demoUginPool + "-b"}, cs, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, map[string]int{})
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 40})
		CloseSide(k)
		uginScan(gs, branch)
		if d := res.Div; d != nil {
			lo := d.At - 10
			if lo < 0 {
				lo = 0
			}
			t.Fatalf("seed %d last actions %v\nseed %d: diverged at decision %d: %s %s %v\nref %v\nker %v",
				seed, actionStrings(res.Actions[lo:d.At]), seed, d.At, d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions)
		}
		clean++
	}
	if branch["exiled"] == 0 {
		t.Fatal("no game exiled a card face down: the pool did not reach Ugin's [+1]")
	}
	if branch["returned"] == 0 {
		t.Fatalf("%d games, %d face-down exiles and no return: the imprinted token never left the battlefield", clean, branch["exiled"])
	}
	if branch["peak"] < 2 {
		t.Fatalf("%d games, %d face-down exiles, peak %d at once: never reached the second [+1] while the first Effect was live", clean, branch["exiled"], branch["peak"])
	}
	t.Logf("%d clean games; %d face-down exiles, %d returns, %d imprints live at once", clean, branch["exiled"], branch["returned"], branch["peak"])
}
