package oracle

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated
// czparams pool, whose carriers read the Mill / ChangeZone choice params
// gorge honours:
//
//   - Mill Optional$ (mill_optional): Daggerfang Duo, Deathcap Marionette,
//     Mineshaft Spider, Song of Stupefaction, Lasyd Prowler, Rainveil
//     Rejuvenator, Patchwork Beastie, Mole Man's moloid token;
//   - the graveyard pick's Optional$ (hidden_pick_confirm): Carrion Cruiser,
//     Pothole Mole, Under the Skin;
//   - Attacking$ True (TokenAttacks "entered attacking"): Kinscaer Sentry (a
//     hand pick), Persistent Marshstalker;
//   - targeted Mandatory$ (a no-op): Disruptor Wanderglyph;
//   - DifferentNames$ on a library search: Fblthp, Knows the Way;
//   - ThisDefinedAndTgts$ Self: Floodpits Drowner (shuffle into libraries),
//     Sandman, Shifting Scoundrel (return from the graveyard).
//
// Not in the pool: Grim Reaper, Lethal Legionnaire (build-refused on its
// finality counter), The Neutrinos and Koya, Death from Above (kgen refuses
// Attacking$ / Mandatory$ beside an "up to one" target: gorge re-asks a
// declined target at resolution and moves the source, see lower_zone.go
// optionalTarget).

var kczDecks = [][2][]cardset.Entry{{
	// M0 mill / graveyard
	{
		{Name: "Daggerfang Duo", Count: 3}, {Name: "Deathcap Marionette", Count: 3}, {Name: "Mineshaft Spider", Count: 2},
		{Name: "Rainveil Rejuvenator", Count: 2}, {Name: "Lasyd Prowler", Count: 2}, {Name: "Patchwork Beastie", Count: 2},
		{Name: "Carrion Cruiser", Count: 2}, {Name: "Pothole Mole", Count: 2}, {Name: "Under the Skin", Count: 2},
		{Name: "Mole Man, Moloid Master", Count: 2}, {Name: "Swamp", Count: 8}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Song of Stupefaction", Count: 3}, {Name: "Floodpits Drowner", Count: 4}, {Name: "Disruptor Wanderglyph", Count: 3},
		{Name: "Deathcap Marionette", Count: 2}, {Name: "Daggerfang Duo", Count: 2}, {Name: "Carrion Cruiser", Count: 2},
		{Name: "Island", Count: 12}, {Name: "Swamp", Count: 12},
	},
}, {
	// M1 attacking
	{
		{Name: "Grizzly Bears", Count: 5}, {Name: "Kinscaer Sentry", Count: 4}, {Name: "Persistent Marshstalker", Count: 4},
		{Name: "Daggerfang Duo", Count: 3}, {Name: "Deathcap Marionette", Count: 2},
		{Name: "Plains", Count: 11}, {Name: "Swamp", Count: 11},
	},
	{
		{Name: "Kinscaer Sentry", Count: 6}, {Name: "Grizzly Bears", Count: 4}, {Name: "Disruptor Wanderglyph", Count: 3},
		{Name: "Plains", Count: 14}, {Name: "Mountain", Count: 13},
	},
}, {
	// M2 search / self
	{
		{Name: "Fblthp, Knows the Way", Count: 4}, {Name: "Sandman, Shifting Scoundrel", Count: 4}, {Name: "Pothole Mole", Count: 3},
		{Name: "Under the Skin", Count: 2}, {Name: "Grizzly Bears", Count: 5},
		{Name: "Forest", Count: 14}, {Name: "Island", Count: 4}, {Name: "Plains", Count: 4},
	},
	{
		{Name: "Floodpits Drowner", Count: 4}, {Name: "Mineshaft Spider", Count: 3}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Island", Count: 13}, {Name: "Forest", Count: 14},
	},
}}

// kczCarriers are the cards each played (cast or activated) at least
// kczMin times.
var kczCarriers = []string{
	"Daggerfang Duo", "Deathcap Marionette", "Mineshaft Spider", "Song of Stupefaction", "Lasyd Prowler",
	"Rainveil Rejuvenator", "Patchwork Beastie", "Mole Man, Moloid Master", "Carrion Cruiser", "Pothole Mole",
	"Under the Skin", "Kinscaer Sentry", "Persistent Marshstalker", "Disruptor Wanderglyph",
	"Fblthp, Knows the Way", "Floodpits Drowner", "Sandman, Shifting Scoundrel",
}

const kczMin = 10

// kczGame is what one game showed: how often each branch was taken.
type kczGame struct {
	n map[string]int
}

func (k *kczGame) add(key string) {
	if k.n == nil {
		k.n = map[string]int{}
	}
	k.n[key]++
}

// kczPolicy is eagerPolicy that also records, from gorge's pending decision
// and the answer picked, the mill_optional and hidden_pick_confirm yes/no
// answers and the Fblthp search picks.
func kczPolicy(seed uint64, played map[string]int, gs **GorgeSide, gm *kczGame) Policy {
	inner := eagerPolicy(seed, played)
	fblthp := map[string]bool{} // distinct basics found in the current Fblthp search
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			*gs = c.G
		}
		a := inner.Pick(c)
		if c.G == nil {
			return a
		}
		d := c.G.Pending()
		if d == nil {
			return a
		}
		src := ""
		if ob := c.G.E.G.Obj(d.Source); ob != nil && ob.Face() != nil {
			src = ob.Face().Name
		}
		opt, ok := c.G.GorgeOption(a)
		switch d.ResumeKind {
		case "mill_optional", "hidden_pick_confirm":
			if ok && (opt.Kind == "yes" || opt.Kind == "no") {
				gm.add(d.ResumeKind + ":" + opt.Kind)
			}
		}
		if src == "Fblthp, Knows the Way" && ok && opt.Obj != 0 {
			if ob := c.G.E.G.Obj(opt.Obj); ob != nil && ob.Face() != nil {
				fblthp[ob.Face().Name] = true
				if len(fblthp) == 2 {
					gm.add("fblthp2")
				}
			}
		} else if src != "Fblthp, Knows the Way" {
			clear(fblthp)
		}
		return a
	})
}

// kczScan counts the mechanics in gorge's event log.
func kczScan(g *GorgeSide, gm *kczGame) {
	if g == nil {
		return
	}
	name := func(id state.ObjID) string {
		if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
			return ob.Face().Name
		}
		return ""
	}
	isLand := func(id state.ObjID) bool {
		if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
			for _, t := range ob.Face().Types {
				if t == "Land" {
					return true
				}
			}
		}
		return false
	}
	var trig string       // the source of the latest trigger or ability pushed
	var libMoves []string // battlefield -> library moves since the last shuffle
	lastLandBack := -10   // event index of the last land graveyard -> battlefield move
	for i, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.TriggerPush, events.AbilityPush:
			trig = name(ev.Obj)
		case events.TokenAttacks:
			if ev.Text != "entered attacking" {
				continue
			}
			gm.add("attacks")
			// a hand pick is Kinscaer Sentry's; Marshstalker returns itself
			who := "Kinscaer Sentry"
			if n := name(ev.Obj); n == "Persistent Marshstalker" {
				who = n
			}
			gm.add("attacks:" + who)
		case events.MoveZone:
			switch {
			case ev.To == state.ZLibrary && ev.From == state.ZBattlefield:
				libMoves = append(libMoves, name(ev.Obj))
			case ev.To == state.ZBattlefield && ev.From == state.ZGraveyard:
				if name(ev.Obj) == "Sandman, Shifting Scoundrel" && i-lastLandBack <= 6 {
					gm.add("sandman2")
				} else if isLand(ev.Obj) {
					lastLandBack = i
				}
			case ev.To == state.ZExile && (trig == "Disruptor Wanderglyph" && ev.From == state.ZGraveyard ||
				trig == "Koya, Death from Above" && ev.From == state.ZBattlefield) && name(ev.Obj) != trig:
				gm.add("mandatory:" + trig)
			}
		case events.Shuffle:
			if len(libMoves) >= 2 {
				for _, n := range libMoves {
					if n == "Floodpits Drowner" {
						gm.add("floodpits2")
						break
					}
				}
			}
			libMoves = libMoves[:0]
		}
	}
}

func kczDump(t *testing.T, g *GorgeSide) {
	if g == nil {
		return
	}
	evs := g.E.L.Events
	i := len(evs) - 1
	for i > 0 && evs[i].Kind != events.TurnChange {
		i--
	}
	for ; i < len(evs); i++ {
		ev := evs[i]
		name := ""
		if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil {
			name = ob.Face().Name
		}
		t.Logf("  ev %v obj %d %q player %d from %v to %v amt %d %q", ev.Kind, ev.Obj, name, ev.Player, ev.From, ev.To, ev.Amount, ev.Text)
	}
}

func TestCzparamsLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("czparams")
	if err != nil {
		t.Fatal(err)
	}
	seeds := uint64(256)
	games := 0
	played := map[string]int{}
	branch := map[string]int{} // games showing each branch
	stalls := map[string]int{}
	for mi, md := range kczDecks {
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
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kcz-a", "kcz-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			var gs *GorgeSide
			var gm kczGame
			res := Lockstep(m, seed, k, kczPolicy(seed, played, &gs, &gm), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls[res.Stall]++
			}
			kczScan(gs, &gm)
			for key := range gm.n {
				branch[key]++
			}
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				kczDump(t, gs)
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				return
			}
		}
	}
	var keys []string
	for k := range branch {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var bs []string
	for _, k := range keys {
		bs = append(bs, fmt.Sprintf("%s=%d", k, branch[k]))
	}
	t.Logf("%d games, games per branch: %s; kernel stalls %v; carrier plays %v", games, strings.Join(bs, " "), stalls, played)
	for _, n := range kczCarriers {
		if played[n] < kczMin {
			t.Errorf("carrier %s played %d times, want at least %d", n, played[n], kczMin)
		}
	}
	for _, want := range []string{
		"mill_optional:yes", "mill_optional:no", "hidden_pick_confirm:yes", "hidden_pick_confirm:no", "attacks",
		"fblthp2", "floodpits2", "sandman2", "mandatory:Disruptor Wanderglyph",
	} {
		if branch[want] < kczMin {
			t.Errorf("branch %s in %d games, want at least %d", want, branch[want], kczMin)
		}
	}
	nAtk := 0
	for _, c := range []string{"Kinscaer Sentry", "Persistent Marshstalker"} {
		if branch["attacks:"+c] > 0 {
			nAtk++
		}
	}
	if nAtk < 2 {
		t.Errorf("entered attacking from %d of the 2 Attacking carriers, want 2", nAtk)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
}
