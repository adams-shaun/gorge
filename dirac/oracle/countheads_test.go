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
// countheads pool, whose carriers read the Count$ value shapes lowered in
// The first count package:
//
//   - distinct names (CR 201.2): Emil's Fractal, Audience with Trostani
//     (tokens only), All-Fates Scroll, Fungal Colossus's cost reduction;
//   - card types among (CR 205.2a): Loot's upkeep Dig;
//   - counters among (CR 122.1): Toph's power;
//   - LimitMin / LimitMax / SVar$: Triumphant Chomp, Flow State;
//   - HalfUp: The Goose Mother;
//   - ValidBattlefield,Graveyard: Gloom Ripper, Moon-Vigil Adherents,
//     Cid, Sinuous Benthisaur;
//   - CardToughness: Armored Armadillo, Captain America;
//   - CastTotalManaSpent: Molten Note (a spell), Astelli Reclaimer (a
//     self-ETB trigger, hard cast and warped);
//   - Remembered$Valid: Cache Grab.

var kchDecks = [][2][]cardset.Entry{{
	// M1: names/aggregates vs toughness/spend
	{
		{Name: "Emil, Vastlands Roamer", Count: 3}, {Name: "Fungal Colossus", Count: 2}, {Name: "Audience with Trostani", Count: 3},
		{Name: "Toph, the Blind Bandit", Count: 3}, {Name: "Flow State", Count: 3}, {Name: "Opt", Count: 2},
		{Name: "Unsummon", Count: 2}, {Name: "Cache Grab", Count: 3}, {Name: "Curious Forager", Count: 3}, {Name: "The Goose Mother", Count: 3},
		{Name: "Moon-Vigil Adherents", Count: 2}, {Name: "Forest", Count: 7}, {Name: "Island", Count: 2},
		{Name: "Hidden Nursery", Count: 1}, {Name: "Hidden Cataract", Count: 1},
	},
	{
		{Name: "Armored Armadillo", Count: 4}, {Name: "Captain America, Wings of Freedom", Count: 3}, {Name: "Agent of Atlas", Count: 4},
		{Name: "Triumphant Chomp", Count: 4}, {Name: "Belligerent Yearling", Count: 3}, {Name: "Molten Note", Count: 3},
		{Name: "Astelli Reclaimer", Count: 2}, {Name: "Mind Stone", Count: 3}, {Name: "Plains", Count: 6}, {Name: "Mountain", Count: 8},
	},
}, {
	// M2: graveyard Elves vs M1's B deck
	{
		{Name: "Gloom Ripper", Count: 4}, {Name: "Moon-Vigil Adherents", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Emil, Vastlands Roamer", Count: 2}, {Name: "Toph, the Blind Bandit", Count: 2},
		{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 9}, {Name: "Hidden Nursery", Count: 3},
	},
	{
		{Name: "Armored Armadillo", Count: 4}, {Name: "Captain America, Wings of Freedom", Count: 3}, {Name: "Agent of Atlas", Count: 4},
		{Name: "Triumphant Chomp", Count: 4}, {Name: "Belligerent Yearling", Count: 3}, {Name: "Molten Note", Count: 3},
		{Name: "Astelli Reclaimer", Count: 2}, {Name: "Mind Stone", Count: 3}, {Name: "Plains", Count: 6}, {Name: "Mountain", Count: 8},
	},
}, {
	// M3: Artificers and Caves vs a Temur aggregates deck
	{
		{Name: "Cid, Timeless Artificer", Count: 3}, {Name: "Aeronaut Tinkerer", Count: 4}, {Name: "Aerial Engineer", Count: 3},
		{Name: "Sinuous Benthisaur", Count: 3}, {Name: "Agent of Atlas", Count: 3}, {Name: "Opt", Count: 4},
		{Name: "Flow State", Count: 2}, {Name: "Unsummon", Count: 2}, {Name: "All-Fates Scroll", Count: 2},
		{Name: "Island", Count: 7}, {Name: "Plains", Count: 4}, {Name: "Hidden Cataract", Count: 2}, {Name: "Hidden Courtyard", Count: 1},
	},
	{
		{Name: "Loot, the Key to Everything", Count: 4}, {Name: "Mind Stone", Count: 3}, {Name: "All-Fates Scroll", Count: 3}, {Name: "Belligerent Yearling", Count: 2},
		{Name: "Armored Kincaller", Count: 3}, {Name: "Triumphant Chomp", Count: 2}, {Name: "Curious Forager", Count: 3},
		{Name: "Cache Grab", Count: 2}, {Name: "Forest", Count: 6}, {Name: "Island", Count: 5}, {Name: "Mountain", Count: 7},
	},
}}

func kchSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("countheads")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// kchMinGames is the least number of games each branch must show.
const kchMinGames = 10

// kchRows are the branches the scan counts (a game counts once per row).
var kchRows = []string{
	"names=1", "names>=2", "fungal reduced", "types>=2", "counters aggregate",
	"LimitMin floor", "LimitMin pass", "LimitMax/SVar$ 2", "LimitMax/SVar$ 1",
	"HalfUp odd", "HalfUp even", "battlefield+graveyard", "Caves", "Artificers",
	"CardToughness armadillo", "CardToughness captain", "spell spend",
	"self-ETB spend hard cast", "self-ETB spend warp",
	"Remembered$Valid true", "Remembered$Valid false",
}

// kchObj is one tracked object's last zone (a MoveZone event's Player is
// not the object's controller, so the owner and controller are read off the
// object: no card in the pool changes controller).
type kchObj struct {
	zone state.Zone
}

func kchFace(g *GorgeSide, id state.ObjID) *cards.Face {
	if ob := g.E.G.Obj(id); ob != nil {
		return ob.Face()
	}
	return nil
}

// kchSide is the player who has the object in zone z: its controller on the
// battlefield, else its owner.
func kchSide(g *GorgeSide, id state.ObjID, z state.Zone) state.PlayerID {
	ob := g.E.G.Obj(id)
	if ob == nil {
		return 255
	}
	if z == state.ZBattlefield {
		return ob.Controller
	}
	return ob.Owner
}

func kchHas(g *GorgeSide, id state.ObjID, word string) bool {
	f := kchFace(g, id)
	if f == nil {
		return false
	}
	for _, w := range f.Types {
		if w == word {
			return true
		}
	}
	return false
}

// kchScan walks one game's gorge event log, replaying zone moves into a
// small tally (who has what on the battlefield and in the graveyard), and
// returns the rows this game showed. The effect rows are read from the
// events after a resolution (to the next priority); the state rows (the
// pre-resolution tallies) from the tally just before it.
func kchScan(g *GorgeSide) map[string]bool {
	rows := map[string]bool{}
	if g == nil {
		return rows
	}
	type stk struct {
		src  state.ObjID
		ab   bool
		ctrl state.PlayerID
	}
	var stack []stk
	where := map[state.ObjID]kchObj{}
	lastX := map[state.ObjID]int32{}
	flags := map[state.ObjID]string{}
	evs := g.E.L.Events
	// count counts the tracked objects of player pl in zone z with the word
	count := func(pl state.PlayerID, z state.Zone, word string) int {
		n := 0
		for id, o := range where {
			if o.zone == z && kchSide(g, id, z) == pl && kchHas(g, id, word) {
				n++
			}
		}
		return n
	}
	landNames := func(pl state.PlayerID) int {
		seen := map[string]bool{}
		for id, o := range where {
			if o.zone == state.ZBattlefield && kchSide(g, id, state.ZBattlefield) == pl && kchHas(g, id, "Land") {
				seen[kchFace(g, id).Name] = true
			}
		}
		return len(seen)
	}
	for i, ev := range evs {
		switch ev.Kind {
		case events.TurnChange:
			stack = stack[:0]
		case events.MoveZone:
			if ev.Obj != 0 {
				where[ev.Obj] = kchObj{ev.To}
			}
			if strings.HasPrefix(ev.Text, "countered") && len(stack) > 0 && stack[len(stack)-1].ab {
				stack = stack[:len(stack)-1]
			}
		case events.CastInfo:
			lastX[ev.Obj] = ev.Amount
			flags[ev.Obj] = ev.Counter
		case events.PutOnStack:
			stack = append(stack, stk{ev.Obj, false, ev.Player})
			if f := kchFace(g, ev.Obj); f != nil && f.Name == "Fungal Colossus" && landNames(ev.Player) >= 1 {
				rows["fungal reduced"] = true
			}
		case events.TriggerPush, events.AbilityPush:
			stack = append(stack, stk{ev.Obj, true, ev.Player})
		case events.Resolve:
			var top stk
			if ev.Text == "" {
				if len(stack) == 0 {
					continue
				}
				top = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			} else {
				for len(stack) > 0 && stack[len(stack)-1].src != ev.Obj {
					stack = stack[:len(stack)-1]
				}
				if len(stack) == 0 {
					continue
				}
				top = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
			f := kchFace(g, top.src)
			if f == nil {
				continue
			}
			j := i + 1
			for j < len(evs) && evs[j].Kind != events.Priority {
				j++
			}
			win := evs[i+1 : j]
			dmg, draws, foods, libHand, libExile, ctrAmt := int32(-1), 0, 0, 0, 0, int32(0)
			var back []state.ObjID // cards moved from a graveyard or exile to the hand
			var mindStone bool
			for _, w := range win {
				switch w.Kind {
				case events.Damage:
					if dmg < 0 {
						dmg = w.Amount
					}
				case events.Draw:
					draws++
				case events.TokenCreate:
					if w.Text == "c_a_food_sac" {
						foods++
					}
				case events.CounterChange:
					if w.Counter == "P1P1" && w.Amount > 0 && kchFace(g, w.Obj) != nil && !kchHas(g, w.Obj, "Land") && ctrAmt == 0 {
						ctrAmt = w.Amount
					}
				case events.MoveZone:
					switch {
					case w.From == state.ZLibrary && w.To == state.ZHand:
						libHand++
					case w.From == state.ZLibrary && w.To == state.ZExile:
						libExile++
					case (w.From == state.ZGraveyard || w.From == state.ZExile) && w.To == state.ZHand:
						back = append(back, w.Obj)
					case w.From == state.ZGraveyard && w.To == state.ZBattlefield:
						if f2 := kchFace(g, w.Obj); f2 != nil && f2.Name == "Mind Stone" {
							mindStone = true
						}
					}
				}
			}
			switch f.Name {
			case "Emil, Vastlands Roamer":
				if ctrAmt == 1 {
					rows["names=1"] = true
				} else if ctrAmt >= 2 {
					rows["names>=2"] = true
				}
			case "Audience with Trostani":
				if draws == 1 {
					rows["names=1"] = true
				} else if draws >= 2 {
					rows["names>=2"] = true
				}
			case "Loot, the Key to Everything":
				if libExile >= 2 {
					rows["types>=2"] = true
				}
			case "Triumphant Chomp":
				if dmg == 2 {
					rows["LimitMin floor"] = true
				} else if dmg >= 3 {
					rows["LimitMin pass"] = true
				}
			case "Flow State":
				if libHand == 2 {
					rows["LimitMax/SVar$ 2"] = true
				} else if libHand == 1 {
					rows["LimitMax/SVar$ 1"] = true
				}
			case "The Goose Mother":
				if x := lastX[top.src]; x >= 1 {
					if x%2 == 1 && int32(foods) == (x+1)/2 {
						rows["HalfUp odd"] = true
					} else if x%2 == 0 && int32(foods) == x/2 {
						rows["HalfUp even"] = true
					}
				}
			case "Gloom Ripper":
				if count(top.ctrl, state.ZGraveyard, "Elf") >= 1 {
					rows["battlefield+graveyard"] = true
				}
			case "Sinuous Benthisaur":
				if count(top.ctrl, state.ZGraveyard, "Cave") >= 1 {
					rows["Caves"] = true
				}
			case "Armored Armadillo":
				rows["CardToughness armadillo"] = true
			case "Captain America, Wings of Freedom":
				if count(top.ctrl, state.ZBattlefield, "Hero") >= 2 {
					rows["CardToughness captain"] = true
				}
			case "Molten Note":
				if dmg >= 0 && !strings.Contains(flags[top.src], "flashback") && dmg == lastX[top.src]+2 {
					rows["spell spend"] = true
				}
			case "Astelli Reclaimer":
				if mindStone {
					if strings.Contains(flags[top.src], "warped") {
						rows["self-ETB spend warp"] = true
					} else {
						rows["self-ETB spend hard cast"] = true
					}
				}
			case "Cache Grab":
				if len(back) == 1 {
					sq := kchHas(g, back[0], "Squirrel")
					if sq && foods == 1 {
						rows["Remembered$Valid true"] = true
					} else if !sq && foods == 0 && count(top.ctrl, state.ZBattlefield, "Squirrel") == 0 {
						rows["Remembered$Valid false"] = true
					}
				}
			}
		}
	}
	return rows
}

// kchSample reads the live gorge state at a priority pick for the rows that
// are a standing value rather than a resolution: Toph's power (counters on
// lands, CR 122.1), Moon-Vigil Adherents' value beyond the creatures on the
// battlefield (graveyard cards counted) and Cid's Artificer graveyard cards.
func kchSample(c *PickCtx, rows map[string]bool) {
	g := c.G
	if g == nil || c.D.Kind != DPriority {
		return
	}
	for i := range g.E.G.Objs {
		ob := &g.E.G.Objs[i]
		if ob.Zone != state.ZBattlefield || ob.Face() == nil {
			continue
		}
		id := state.ObjID(i + 1)
		switch ob.Face().Name {
		case "Toph, the Blind Bandit":
			if g.E.Power(id) >= 1 {
				rows["counters aggregate"] = true
			}
		case "Moon-Vigil Adherents":
			bf := int32(0)
			for _, o := range g.E.G.Zone(state.ZBattlefield, ob.Controller) {
				if kchHas(g, o, "Creature") {
					bf++
				}
			}
			if g.E.Power(id) > bf {
				rows["battlefield+graveyard"] = true
			}
		case "Cid, Timeless Artificer":
			for _, o := range g.E.G.Zone(state.ZGraveyard, ob.Controller) {
				if kchHas(g, o, "Artificer") {
					rows["Artificers"] = true
				}
			}
		}
	}
}

func TestCountheadsLockstep(t *testing.T) {
	t.Parallel()
	reg, set := kchSetup(t)
	games := 0
	played := map[string]int{}
	stalls := map[string]int{}
	tally := map[string]int{}
	for mi, md := range kchDecks {
		decks := kcastDeck(t, reg, md)
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 256; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kch-a", "kch-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			var gs *GorgeSide
			sampled := map[string]bool{}
			inner := eagerPolicy(seed, played)
			pol := PolicyFunc(func(c *PickCtx) Canon {
				if c.G != nil {
					gs = c.G
				}
				kchSample(c, sampled)
				return inner.Pick(c)
			})
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls[res.Stall]++
			}
			rows := kchScan(gs)
			for r := range sampled {
				rows[r] = true
			}
			for r := range rows {
				tally[r]++
			}
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				kcastDump(t, gs)
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				return
			}
		}
	}
	var rs []string
	for _, r := range kchRows {
		rs = append(rs, fmt.Sprintf("%s=%d", r, tally[r]))
	}
	var ps []string
	for n, c := range played {
		ps = append(ps, fmt.Sprintf("%s:%d", n, c))
	}
	sort.Strings(ps)
	t.Logf("%d games, kernel stalls %v\nbranch games: %s\ncarrier plays: %s", games, stalls, strings.Join(rs, ", "), strings.Join(ps, " "))
	for _, r := range kchRows {
		if tally[r] < kchMinGames {
			t.Errorf("branch %q shown in %d games, want at least %d", r, tally[r], kchMinGames)
		}
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
}
