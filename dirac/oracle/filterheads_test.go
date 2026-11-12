package oracle

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated
// filterheads pool, whose carriers read the six filter spellings
// lowers:
//
//   - ControlledBy TriggeredDefendingPlayer (F2DefendingCtrl): Spring
//     Splasher, Thunder Lasso and Captain America's Shield's attack
//     triggers;
//   - TargetedPlayerCtrl (F2TgtPlayerCtrl): Villainous Wrath's life loss;
//   - the Spell head of a target-gated reduction: Out of Air;
//   - the A;B cost type lists: Earthrumbler, Postmortem Professor;
//   - an absent creature-type head: Spider-Ham's anthem (the P/T
//     projection compares it).
//
// Syr Ginger's Sac<1/NICKNAME> lowers as the source itself now (kop.CostSacSelf:
// gorge's pay.SacrificeMatchSpec reads it as CARDNAME and offers the activation,
// engine/rules nickname_sac_test.go); this pool omits it all the same, its
// lockstep is the who pool's.
// Tezzeret, Cruel Captain (Targeted.!Creature) lowers but is left out too:
// gorge fires its emblem's Phase trigger once (an Effect's Mode$ Phase
// registration is one-shot), the kernel every combat, as for the four
// emblem siblings already accepted ;
// TestAnimateTargetedNegatedType covers its guard.
// The frozen FDN and BLB pair pools hold none of these carriers.

var kfhDecks = [][2][]cardset.Entry{{
	{
		{Name: "Spring Splasher", Count: 4}, {Name: "Thunder Lasso", Count: 3}, {Name: "Captain America's Shield", Count: 2},
		{Name: "Universal Automaton", Count: 4}, {Name: "Ornithopter", Count: 3}, {Name: "Out of Air", Count: 4},
		{Name: "Divination", Count: 2}, {Name: "Island", Count: 10}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Postmortem Professor", Count: 4}, {Name: "Villainous Wrath", Count: 4}, {Name: "Thought Scour", Count: 4},
		{Name: "Universal Automaton", Count: 3}, {Name: "Ornithopter", Count: 3}, {Name: "Swamp", Count: 12}, {Name: "Island", Count: 10},
	},
}, {
	kfhMirror, kfhMirror,
}}

var kfhMirror = []cardset.Entry{
	{Name: "Spider-Ham, Peter Porker", Count: 4},
	{Name: "Earthrumbler", Count: 4}, {Name: "Ornithopter", Count: 4},
	{Name: "Mind Stone", Count: 4}, {Name: "Universal Automaton", Count: 4}, {Name: "Grizzly Bears", Count: 6},
	{Name: "Forest", Count: 14},
}

// kfhBranches names the mechanic branches the test counts, one game each.
var kfhBranches = []string{"D+", "D-", "T+", "T0", "S+", "S-", "E-C", "E-A", "P-I", "P-S", "H+", "H-"}

const kfhMinGames = 10

// kfhWrath is Villainous Wrath on the stack, last seen at a decision: its
// caster's and its target player's creature counts.
type kfhWrath struct{ caster, target int }

// kfhGame is what one game's policy saw at its decisions.
type kfhGame struct {
	gs    *GorgeSide
	wrath map[state.ObjID]kfhWrath
	hit   map[string]bool
}

func kfhFace(g *GorgeSide, id state.ObjID) string {
	if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
		return ob.Face().Name
	}
	return ""
}

func kfhHasType(g *GorgeSide, id state.ObjID, typ string) bool {
	ob := g.E.G.Obj(id)
	return ob != nil && ob.Face() != nil && slices.Contains(ob.Face().Types, typ)
}

func kfhCreatures(g *GorgeSide, p state.PlayerID) int {
	n := 0
	for i := range g.E.G.Objs {
		ob := &g.E.G.Objs[i]
		if ob.Zone == state.ZBattlefield && ob.Controller == p && g.E.IsCreature(state.ObjID(i+1)) {
			n++
		}
	}
	return n
}

// kfhPolicy casts or activates whenever it can (eagerPolicy) and records per
// decision the Villainous Wrath counts and the Spider-Ham anthem's board.
func kfhPolicy(seed uint64, played map[string]int, gm *kfhGame) Policy {
	inner := eagerPolicy(seed, played)
	return PolicyFunc(func(c *PickCtx) Canon {
		g := c.G
		if g == nil {
			return inner.Pick(c)
		}
		gm.gs = g
		var ham [2]bool
		for i := range g.E.G.Objs {
			ob := &g.E.G.Objs[i]
			id := state.ObjID(i + 1)
			switch {
			case ob.Zone == state.ZStack && ob.Face() != nil && ob.Face().Name == "Villainous Wrath" && ob.Ability == nil:
				for _, t := range ob.Targets {
					if t.IsPlayer {
						gm.wrath[id] = kfhWrath{caster: kfhCreatures(g, ob.Controller), target: kfhCreatures(g, t.Player)}
					}
				}
			case ob.Zone == state.ZBattlefield && kfhFace(g, id) == "Spider-Ham, Peter Porker" && int(ob.Controller) < 2:
				ham[ob.Controller] = true
			}
		}
		for i := range g.E.G.Objs {
			ob := &g.E.G.Objs[i]
			if ob.Zone != state.ZBattlefield || int(ob.Controller) >= 2 || !ham[ob.Controller] {
				continue
			}
			switch kfhFace(g, state.ObjID(i+1)) {
			case "Grizzly Bears", "Universal Automaton":
				gm.hit["H+"] = true // a Bear, a Changeling: +1/+1
			case "Ornithopter":
				gm.hit["H-"] = true // a Thopter: no bonus
			}
		}
		return inner.Pick(c)
	})
}

// kfhScan reads one game's gorge event log for the branches.
func kfhScan(mi int, gm *kfhGame) {
	g := gm.gs
	if g == nil {
		return
	}
	evs := g.E.L.Events
	tgtObj := map[state.ObjID][]state.ObjID{}
	tgtPl := map[state.ObjID]state.PlayerID{}
	paid := map[state.ObjID]int{}
	var paying state.ObjID
	var payer state.PlayerID
	for i, ev := range evs {
		ob := g.E.G.Obj(ev.Obj)
		switch ev.Kind {
		case events.PutOnStack:
			paying = 0
			if kfhFace(g, ev.Obj) == "Out of Air" {
				paying, payer = ev.Obj, ev.Player
			}
		case events.Priority, events.Resolve:
			paying = 0
		case events.ManaAdd:
			// the payer's pool debited while Out of Air is paid
			if paying != 0 && ev.Player == payer && ev.Amount < 0 {
				paid[paying] -= int(ev.Amount)
			}
		case events.TargetsChosen:
			switch ev.Amount {
			case 0:
				tgtObj[ev.Obj] = append([]state.ObjID(nil), ev.IDs...)
			case 2:
				tgtObj[ev.Obj] = append(tgtObj[ev.Obj], ev.IDs...)
			case 1, 3:
				tgtPl[ev.Obj] = ev.Player
			}
		case events.MoveZone:
			if ev.From == state.ZStack && ev.Text == "countered: no legal targets" && ob != nil && ob.Ability != nil && kfhDefending(g, ob) {
				gm.hit["D-"] = true
			}
			if ev.From == state.ZGraveyard && ev.To == state.ZExile {
				cr, ar := kfhHasType(g, ev.Obj, "Creature"), kfhHasType(g, ev.Obj, "Artifact")
				switch {
				case mi == 1 && cr && !ar:
					gm.hit["E-C"] = true
				case mi == 1 && ar && !cr:
					gm.hit["E-A"] = true
				case mi == 0 && kfhHasType(g, ev.Obj, "Instant"):
					gm.hit["P-I"] = true
				case mi == 0 && kfhHasType(g, ev.Obj, "Sorcery"):
					gm.hit["P-S"] = true
				}
			}
		}
		if ev.Kind != events.Resolve || ob == nil {
			continue
		}
		// the life changes this resolution made (up to the next Resolve)
		life := map[state.PlayerID]int32{}
		for _, nx := range evs[i+1:] {
			if nx.Kind == events.Resolve {
				break
			}
			if nx.Kind == events.LifeChange {
				life[nx.Player] += nx.Amount
			}
		}
		ts := tgtObj[ev.Obj]
		switch {
		case ob.Ability != nil && kfhDefending(g, ob):
			if len(ts) == 1 {
				if t := g.E.G.Obj(ts[0]); t != nil && t.Controller != ob.Controller {
					gm.hit["D+"] = true
				}
			}
		case ob.Ability == nil && kfhFace(g, ev.Obj) == "Villainous Wrath":
			w, ok := gm.wrath[ev.Obj]
			p, okp := tgtPl[ev.Obj]
			if !ok || !okp {
				break
			}
			if w.target >= 1 && w.caster >= 1 && -life[p] == int32(w.target) {
				gm.hit["T+"] = true
			}
			if w.target == 0 && life[p] == 0 {
				gm.hit["T0"] = true
			}
		case ob.Ability == nil && kfhFace(g, ev.Obj) == "Out of Air" && len(ts) == 1:
			if kfhHasType(g, ts[0], "Creature") && paid[ev.Obj] == 2 {
				gm.hit["S+"] = true
			}
			if !kfhHasType(g, ts[0], "Creature") && paid[ev.Obj] == 4 {
				gm.hit["S-"] = true
			}
		}
	}
}

// kfhDefending: ob is the attack trigger of a defending-player carrier.
func kfhDefending(g *GorgeSide, ob *state.Object) bool {
	switch kfhFace(g, ob.Source) {
	case "Spring Splasher":
		return ob.Ability.API == "Pump"
	case "Thunder Lasso", "Captain America's Shield":
		return ob.Ability.API == "Tap"
	}
	return false
}

func TestFilterheadsLockstep(t *testing.T) {
	t.Parallel()

	reg, _ := kcastSetup(t)
	set, err := cardset.RawByName("filterheads")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	played := map[string]int{}
	branch := map[string]int{}
	stalls := map[string]int{}
	const seeds = 128
	for mi, md := range kfhDecks {
		decks := kcastDeck(t, reg, md)
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kfh-a", "kfh-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			gm := &kfhGame{wrath: map[state.ObjID]kfhWrath{}, hit: map[string]bool{}}
			res := Lockstep(m, seed, k, kfhPolicy(seed, played, gm), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls[res.Stall]++
			}
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				kcastDump(t, gm.gs)
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				return
			}
			kfhScan(mi, gm)
			for b := range gm.hit {
				branch[b]++
			}
		}
	}
	var bs []string
	for _, b := range kfhBranches {
		bs = append(bs, fmt.Sprintf("%s:%d", b, branch[b]))
	}
	t.Logf("%d games, branch games %s, kernel stalls %v, carrier plays %v", games, strings.Join(bs, " "), stalls, played)
	for _, b := range kfhBranches {
		if branch[b] < kfhMinGames {
			t.Errorf("branch %s reached in %d games, want at least %d", b, branch[b], kfhMinGames)
		}
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
}
