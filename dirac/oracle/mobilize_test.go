package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Mobilize lockstep: Mobilize 1 and 2 (CR 702.181a) mint
// tapped attacking Warrior tokens and sacrifice the surviving group as one
// batch at the next end step (gorge's RememberChain$ False delayed trigger).
// The policy attacks with a Mobilize creature whenever it can and casts
// Shock / Disfigure at a Warrior token whenever one is on the battlefield
// (never in an end step, so a Warrior leaving in an end step was sacrificed).
var kmobilizeDecks = [][2][]cardset.Entry{{
	{
		{Name: "Stadium Headliner", Count: 4}, {Name: "Shock Brigade", Count: 4}, {Name: "Zurgo's Vanguard", Count: 4},
		{Name: "Nightblade Brigade", Count: 4}, {Name: "Venerated Stormsinger", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Dragonback Lancer", Count: 4}, {Name: "Voice of Victory", Count: 4}, {Name: "Bone-Cairn Butcher", Count: 4},
		{Name: "Reigning Victor", Count: 4}, {Name: "Disfigure", Count: 4}, {Name: "Ornithopter", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8},
	},
}, {
	{
		{Name: "Dragonback Lancer", Count: 4}, {Name: "Voice of Victory", Count: 4}, {Name: "Bone-Cairn Butcher", Count: 4},
		{Name: "Reigning Victor", Count: 4}, {Name: "Disfigure", Count: 4}, {Name: "Ornithopter", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Raging Goblin", Count: 6}, {Name: "Shock", Count: 6},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}}

var kmobilizeCreatures = map[string]bool{
	"Dragonback Lancer": true, "Nightblade Brigade": true, "Reigning Victor": true, "Shock Brigade": true,
	"Stadium Headliner": true, "Venerated Stormsinger": true, "Zurgo's Vanguard": true,
	"Bone-Cairn Butcher": true, "Voice of Victory": true,
}

// kmobilizeTally counts, from gorge's board between decisions, the group
// branches of the Test section.
type kmobilizeTally struct {
	games, m1, m2, mp, mm, survived int
}

// kmobilizeTracker follows the Warrior tokens of one game across decisions.
type kmobilizeTracker struct {
	prev, cur map[state.ObjID]state.PlayerID
	prevStep  state.Step
	group     map[state.ObjID]int // token -> group number
	size      map[int]int         // group -> tokens minted
	early     map[int]int         // group -> tokens gone before an end step
	next      int
	prevMob   [2]map[state.ObjID]bool
	endTurn   int32 // the turn of the last end-step sacrifice
	endGroups int   // groups sacrificed in that end step
}

func newKmobilizeTracker() *kmobilizeTracker {
	return &kmobilizeTracker{prev: map[state.ObjID]state.PlayerID{}, group: map[state.ObjID]int{}, size: map[int]int{}, early: map[int]int{}}
}

// board returns the Warrior tokens and, per player, the Mobilize creatures
// on the battlefield.
func kmobilizeBoard(g *state.Game) (map[state.ObjID]state.PlayerID, [2]map[state.ObjID]bool) {
	w := map[state.ObjID]state.PlayerID{}
	var mob [2]map[state.ObjID]bool
	for p := state.PlayerID(0); p < 2; p++ {
		mob[p] = map[state.ObjID]bool{}
		for _, id := range g.Zone(state.ZBattlefield, p) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			switch {
			case o.IsToken && strings.Contains(o.Face().Name, "Warrior"):
				w[id] = p
			case !o.IsToken && kmobilizeCreatures[o.Face().Name]:
				mob[p][id] = true
			}
		}
	}
	return w, mob
}

func (tr *kmobilizeTracker) observe(g *state.Game, tal *kmobilizeTally) {
	cur, mob := kmobilizeBoard(g)
	var fresh []state.ObjID
	for id := range cur {
		if _, ok := tr.prev[id]; !ok {
			fresh = append(fresh, id)
		}
	}
	if len(fresh) > 0 {
		tr.next++
		for _, id := range fresh {
			tr.group[id] = tr.next
		}
		tr.size[tr.next] = len(fresh)
	}
	gone := map[int]int{}
	ctrl := map[int]state.PlayerID{}
	for id, p := range tr.prev {
		if _, ok := cur[id]; !ok {
			gone[tr.group[id]]++
			ctrl[tr.group[id]] = p
		}
	}
	atEnd := tr.prevStep == state.StepEnd
	for gid, n := range gone {
		if !atEnd {
			tr.early[gid] += n
			continue
		}
		if tr.endTurn != g.Turn {
			tr.endTurn, tr.endGroups = g.Turn, 0
		}
		if tr.endGroups++; tr.endGroups == 2 {
			tal.mm++ // a second group's sacrifice in one end step (separate resolutions)
		}
		switch {
		case tr.size[gid] == 1 && n == 1:
			tal.m1++
		case tr.size[gid] == 2 && n == 2:
			tal.m2++
		}
		if tr.early[gid] > 0 {
			tal.mp++
		}
		for id := range tr.prevMob[ctrl[gid]] {
			if mob[ctrl[gid]][id] {
				tal.survived++
				break
			}
		}
	}
	tr.prev, tr.prevStep, tr.prevMob = cur, g.Step, mob
}

// kmobilizePolicy is the Test section's policy; every fifth seed plays at
// random so blocks and removal vary.
func kmobilizePolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	var seq int = -1
	chosen := map[Canon]bool{}
	return PolicyFunc(func(c *PickCtx) Canon {
		if seed%5 == 0 || c.G == nil {
			return rnd.Pick(c)
		}
		g := c.G.E.G
		name := func(r Ref) string {
			for p := state.PlayerID(0); p < 2; p++ {
				for _, z := range []state.Zone{state.ZHand, state.ZBattlefield} {
					for _, id := range g.Zone(z, p) {
						if c.G.InstRef(id) == r {
							if o := g.Obj(id); o != nil && o.Face() != nil {
								return o.Face().Name
							}
						}
					}
				}
			}
			return ""
		}
		warrior := func(r Ref) bool {
			for p := state.PlayerID(0); p < 2; p++ {
				for _, id := range g.Zone(state.ZBattlefield, p) {
					if c.G.InstRef(id) == r {
						o := g.Obj(id)
						return o != nil && o.IsToken && o.Face() != nil && strings.Contains(o.Face().Name, "Warrior")
					}
				}
			}
			return false
		}
		if c.Seq != seq {
			seq = c.Seq
			clear(chosen)
		}
		pick := func(opts []Canon) Canon {
			d := *c.D
			d.Options = opts
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		buf = buf[:0]
		switch c.D.Kind {
		case DAttackers:
			for _, a := range c.D.Options {
				if a.Kind() == ActAttack && !chosen[a] && kmobilizeCreatures[name(a.Src())] {
					chosen[a] = true
					return a
				}
			}
		case DTarget:
			for _, a := range c.D.Options {
				if a.Kind() == ActTarget && warrior(a.Tgt()) {
					buf = append(buf, a)
				}
			}
		case DPriority:
			removal := g.Step != state.StepEnd && g.Step != state.StepCleanup
			hasW := false
			if removal {
				for p := state.PlayerID(0); p < 2 && !hasW; p++ {
					for _, id := range g.Zone(state.ZBattlefield, p) {
						if o := g.Obj(id); o != nil && o.IsToken && o.Face() != nil && strings.Contains(o.Face().Name, "Warrior") {
							hasW = true
							break
						}
					}
				}
			}
			var rm []Canon
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActPlayLand:
					if n := name(a.Src()); n == "Shock" || n == "Disfigure" {
						if removal && hasW {
							rm = append(rm, a)
						}
						continue
					}
					buf = append(buf, a)
				}
			}
			if len(rm) > 0 {
				return pick(rm)
			}
		}
		if len(buf) > 0 {
			return pick(buf)
		}
		return rnd.Pick(c)
	})
}

// kmobilizeRun plays seeds 1..n of every matchup in lockstep and returns
// the divergent games.
func kmobilizeRun(t *testing.T, n uint64, report bool, tal *kmobilizeTally) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mobilize")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range kmobilizeDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= n; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"mb-a", "mb-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			tr := newKmobilizeTracker()
			check := func(ref *GorgeSide, _ Side) *Divergence {
				if tal != nil {
					tr.observe(ref.E.G, tal)
				}
				return nil
			}
			res := Lockstep(m, seed, k, kmobilizePolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
			CloseSide(k)
			if tal != nil {
				tal.games++
			}
			if res.Stall != "" && res.Stall != "turns" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				div++
				if !report {
					t.Logf("matchup %d seed %d diverges (decision %d: %s)", mi, seed, d.At, d.What)
					return div
				}
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return div
}

func TestMobilizeLockstep(t *testing.T) {
	t.Parallel()
	var tal kmobilizeTally
	kmobilizeRun(t, 200, true, &tal)
	t.Logf("%d games: Mobilize 1 sacrifices %d, Mobilize 2 sacrifices %d, partial groups %d, double-group end steps %d, attacker survived %d",
		tal.games, tal.m1, tal.m2, tal.mp, tal.mm, tal.survived)
	for _, c := range []struct {
		name string
		n    int
	}{{"M1", tal.m1}, {"M2", tal.m2}, {"M-P", tal.mp}, {"MM", tal.mm}, {"attacker survived", tal.survived}} {
		if c.n < 10 {
			t.Errorf("%s: %d, want at least 10", c.name, c.n)
		}
	}
}
