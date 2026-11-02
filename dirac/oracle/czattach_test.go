package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: ChangeZone OriginAlternative$
// (Delivery Moogle's library-and/or-graveyard artifact search, Last Light of
// Durin's Day's library-and/or-hand Dragon search) and AttachedTo$ (Eagle's
// Rescue returning itself to the Pump's target, Magnetic Snuffler returning an
// Equipment attached to itself). The decks carry cheap artifacts, Equipment,
// Auras, discard and instant removal so graveyards fill with the carriers'
// cards and the Pump target can be killed in response. gorge leads.
var kczattachDecks = [2][]cardset.Entry{
	{
		{Name: "Delivery Moogle", Count: 3}, {Name: "Last Light of Durin's Day", Count: 3},
		{Name: "Eagle's Rescue", Count: 3}, {Name: "Magnetic Snuffler", Count: 3},
		{Name: "Healer's Hawk", Count: 3}, {Name: "Ornithopter", Count: 2},
		{Name: "Bonesplitter", Count: 1}, {Name: "Short Sword", Count: 1}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Pirate's Pillage", Count: 2}, {Name: "Disenchant", Count: 1},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 6}, {Name: "Evolving Wilds", Count: 2},
	},
	{
		{Name: "Delivery Moogle", Count: 2}, {Name: "Eagle's Rescue", Count: 2},
		{Name: "Magnetic Snuffler", Count: 1}, {Name: "Healer's Hawk", Count: 3},
		{Name: "Ornithopter", Count: 2}, {Name: "Bonesplitter", Count: 2},
		{Name: "Short Sword", Count: 2}, {Name: "Mind Stone", Count: 1},
		{Name: "Pirate's Pillage", Count: 2}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Shatter", Count: 2}, {Name: "Disenchant", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8},
	},
}

// kczattachTally is what the played games reached.
type kczattachTally struct {
	games, div, stalls int
	elLib, elGrave     int // searches that elected the library only / the other zone only
	elBoth             int
	elHand             int // Last Light's hand-or-library searches that elected the hand
	shuffled, kept     int // the elected searches that shuffled the library / left it alone
	equipAttached      int // an Equipment returned attached
	auraAttached       int // an Aura returned attached
	lightSearch        int // Last Light's search resolutions
	erActs, erBacks    int // Eagle's Rescue activations / returns
}

// kczattachPolicy is the cast-first random policy; it tallies the zone
// elections as gorge poses them (gorge leads, so its decisions are the
// reference).
func kczattachPolicy(seed uint64, tl *kczattachTally, gs **GorgeSide) Policy {
	inner := eagerPolicy(seed, map[string]int{})
	zone, _ := chooseCode("zone")
	var picked [2]bool
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			*gs = c.G
		}
		a := inner.Pick(c)
		switch {
		case a.Kind() == ActChoose+CanonKind(zone):
			picked[a.Mode()&1] = true
			if picked[0] && picked[1] {
				tl.elBoth++
				tl.shuffled++
				picked = [2]bool{}
			}
		case a.Kind() == ActDone && (picked[0] || picked[1]):
			switch {
			case picked[0]:
				tl.elLib++
				tl.shuffled++
			default:
				tl.elGrave++
				tl.kept++
			}
			picked = [2]bool{}
		}
		return a
	})
}

// kczattachScan counts the attach outcomes in gorge's event log.
func kczattachScan(g *GorgeSide, tl *kczattachTally) {
	if g == nil {
		return
	}
	obj := func(id state.ObjID) *state.Object { return g.E.G.Obj(id) }
	name := func(id state.ObjID) string {
		if ob := obj(id); ob != nil && ob.Face() != nil {
			return ob.Face().Name
		}
		return ""
	}
	returned := map[state.ObjID]bool{}
	acts, backs := 0, 0
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.MoveZone:
			if ev.From == state.ZGraveyard && ev.To == state.ZBattlefield {
				switch name(ev.Obj) {
				case "Eagle's Rescue", "Bonesplitter", "Short Sword", "Bone Saw":
					returned[ev.Obj] = true
					if name(ev.Obj) == "Eagle's Rescue" {
						backs++
					}
				}
			}
		case events.Attach:
			if !returned[ev.Obj] {
				continue
			}
			delete(returned, ev.Obj)
			if name(ev.Obj) == "Eagle's Rescue" {
				tl.auraAttached++
			} else {
				tl.equipAttached++
			}
		case events.AbilityPush:
			if name(ev.Obj) == "Eagle's Rescue" {
				acts++
			}
		case events.TriggerPush:
			if name(ev.Obj) == "Last Light of Durin's Day" {
				tl.lightSearch++
			}
		}
	}
	tl.erActs += acts
	tl.erBacks += backs
}

// kczattachRun plays seeds 1..seeds with both seatings; DebugCzAttachOff is
// the mutation. It stops at the first divergence unless mutating.
func kczattachRun(t *testing.T, seeds uint64, mutate bool, report bool) (*kczattachTally, *Divergence) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("czattach")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugCzAttachOff
	dirac.DebugCzAttachOff = mutate
	defer func() { dirac.DebugCzAttachOff = old }()
	tl := &kczattachTally{}
	for swap := 0; swap < 2; swap++ {
		md := kczattachDecks
		if swap == 1 {
			md = [2][]cardset.Entry{kczattachDecks[1], kczattachDecks[0]}
		}
		var gd [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					gd[s] = append(gd[s], c)
				}
			}
			if len(gd[s]) != 40 {
				t.Fatalf("deck %d: %d cards", s, len(gd[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			m, err := NewMatchup(reg, [2]string{"kczattach-a", "kczattach-b"}, gd, 1)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			var gs *GorgeSide
			res := LockstepLed(m, seed, g, k, kczattachPolicy(seed, tl, &gs), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 20})
			CloseSide(k)
			tl.games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				tl.stalls++
			}
			kczattachScan(g, tl)
			if res.Div != nil {
				tl.div++
				if report {
					lo := max(res.Div.At-12, 0)
					t.Logf("swap %d seed %d diverged at decision %d: %s %s %v %s\nlast actions %v\nref %v\nker %v", swap, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, actionStrings(res.Actions[lo:res.Div.At]), res.Div.RefOptions, res.Div.KerOptions)
				}
				return tl, res.Div
			}
		}
	}
	return tl, nil
}

func (tl *kczattachTally) log(t *testing.T) {
	t.Helper()
	t.Logf("%d games, %d divergences, %d stalls", tl.games, tl.div, tl.stalls)
	t.Logf("zone elections: library only %d, other zone only %d, both %d; library shuffled %d, untouched %d",
		tl.elLib, tl.elGrave, tl.elBoth, tl.shuffled, tl.kept)
	t.Logf("returns: Equipment attached %d, Aura attached %d; Last Light triggers %d; Eagle's Rescue activations %d, returns %d",
		tl.equipAttached, tl.auraAttached, tl.lightSearch, tl.erActs, tl.erBacks)
}

func TestCzattachLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	tl, div := kczattachRun(t, 96, false, true)
	if div != nil {
		t.Errorf("diverged at decision %d: %s %s %v %s\nref %v\nker %v", div.At, div.What, div.Field, div.Diff, div.Detail, div.RefOptions, div.KerOptions)
		return
	}
	tl.log(t)
	if tl.stalls != 0 {
		t.Errorf("kernel capacity stalls %d, want 0", tl.stalls)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"library-only elections", tl.elLib}, {"other-zone-only elections", tl.elGrave}, {"both-zone elections", tl.elBoth},
		{"library shuffles", tl.shuffled}, {"searches without a shuffle", tl.kept},
		{"Equipment returned attached", tl.equipAttached}, {"Aura returned attached", tl.auraAttached},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d, want at least 3", c.name, c.n)
		}
	}
}

// TestCzattachMutation: with the election skipped (the library alone is
// searched) and every returned card entering unattached, the kernel
// diverges from gorge.
func TestCzattachMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	tl, div := kczattachRun(t, 32, true, false)
	if div == nil {
		t.Fatalf("no divergence in %d games with DebugCzAttachOff", tl.games)
	}
	t.Logf("diverged after %d games at decision %d: %s %s", tl.games, div.At, div.What, div.Field)
}
