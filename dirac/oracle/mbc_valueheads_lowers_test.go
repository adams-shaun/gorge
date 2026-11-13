package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC values-2 spelling family kgen lowers, one card a spelling, each
// played against gorge in a fixture-table lockstep game (printedDelveSet):
//
//   - count:ValidBattlefield (Cloudkill): Count$ValidBattlefield,Command
//     Card.IsCommander+YouOwn$GreatestCardManaCost, kop.ExCmdGreatestMV, in a
//     Commander duel whose black commander has mana value 3;
//   - count:Revolt (Fatal Push): Count$Revolt.4.2 as the destroy bound,
//     revolt raised by Evolving Wilds and combat trades;
//   - PumpAll keyword above bit 15 (Dirge of Dread): KW$ Fear over every
//     creature, kop.ExtGrantKwAll;
//   - ConditionCheckSVar$ beside ConditionPresent$ (Abzan Beastmaster): the
//     upkeep draw gated on the greatest toughness.
//
// Each subtest counts, in gorge's events, the reads that make its game a
// test of the lowering (the commander where X sees it, a revolt-sized kill,
// the draw both given and withheld) and fails when a seed diverges, a kernel
// stall ends a game, or the reads were not reached.

// mv2Seat is one seat's name and deck; a Commander seat's first entry is its
// commander (Count 1).
type mv2Seat struct {
	name string
	deck []cardset.Entry
}

// mv2Tally is the per-spelling count of gorge-side reads.
type mv2Tally struct {
	resolved int // the class card's spell or trigger resolved / pushed
	yes, no  int // the read's two outcomes (meaning per subtest)
}

// mv2Scan is a subtest's per-event classifier over gorge's log: ev is the next
// event, ref the gorge side as the lockstep last checked it.
type mv2Scan func(ref *GorgeSide, ev events.Event, tal *mv2Tally)

func mv2Check(scan mv2Scan, tal *mv2Tally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen = ref, 0
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			scan(ref, evs[seen], tal)
		}
		return nil
	}
}

// mv2Run plays seeds 1..n of the two seats (a Commander duel when commander),
// failing t on a divergent seed or a kernel stall.
func mv2Run(t *testing.T, seats [2]mv2Seat, commander bool, n uint64, scan mv2Scan, tal *mv2Tally) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	var names [2][]string
	for s, st := range seats {
		for _, e := range st.deck {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			names[s] = append(names[s], e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
			}
		}
	}
	set := printedDelveSet(t, names)
	tbl, _, _, err := set.Matchup(seats[0].deck, seats[1].deck)
	if err != nil {
		t.Fatal(err)
	}
	stalls := 0
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{seats[0].name, seats[1].name}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		if commander {
			m.Commander = true
			m.Commanders, m.Partners = [2]int{0, 0}, [2]int{-1, -1}
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{MaxDecisions: 6000, MaxTurns: 40, Check: mv2Check(scan, tal)})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
}

// mv2Named is the face name of id in ref's game ("" when none).
func mv2Named(ref *GorgeSide, id state.ObjID) string {
	if o := ref.E.G.Obj(id); o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return ""
}

var mv2Cloudkill = [2]mv2Seat{
	{"cloudkill-cmd", []cardset.Entry{
		{Name: "Kalakscion, Hunger Tyrant", Count: 1}, {Name: "Cloudkill", Count: 10},
		{Name: "Typhoid Rats", Count: 4}, {Name: "Dark Ritual", Count: 6}, {Name: "Swamp", Count: 19},
	}},
	{"weenie-cmd", []cardset.Entry{
		{Name: "Isamaru, Hound of Konda", Count: 1}, {Name: "Savannah Lions", Count: 8},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4}, {Name: "Craw Wurm", Count: 2},
		{Name: "Plains", Count: 10}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 3},
	}},
}

var mv2FatalPush = [2]mv2Seat{
	{"push", []cardset.Entry{
		{Name: "Fatal Push", Count: 12}, {Name: "Evolving Wilds", Count: 16}, {Name: "Typhoid Rats", Count: 4},
		{Name: "Swamp", Count: 8},
	}},
	{"midrange", []cardset.Entry{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Gray Ogre", Count: 6}, {Name: "Hill Giant", Count: 6},
		{Name: "Craw Wurm", Count: 2}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	}},
}

var mv2Dirge = [2]mv2Seat{
	{"dirge", []cardset.Entry{
		{Name: "Dirge of Dread", Count: 10}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
		{Name: "Swamp", Count: 10}, {Name: "Forest", Count: 8},
	}},
	{"blockers", []cardset.Entry{
		{Name: "Savannah Lions", Count: 6}, {Name: "Gray Ogre", Count: 6}, {Name: "Typhoid Rats", Count: 4},
		{Name: "Ornithopter", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Mountain", Count: 6}, {Name: "Swamp", Count: 6},
	}},
}

var mv2Abzan = [2]mv2Seat{
	{"beastmaster", []cardset.Entry{
		{Name: "Abzan Beastmaster", Count: 10}, {Name: "Llanowar Elves", Count: 6}, {Name: "Craw Wurm", Count: 4},
		{Name: "Forest", Count: 20},
	}},
	{"toughness", []cardset.Entry{
		{Name: "Hill Giant", Count: 6}, {Name: "Gray Ogre", Count: 6}, {Name: "Giant Spider", Count: 6},
		{Name: "Mountain", Count: 11}, {Name: "Forest", Count: 11},
	}},
}

// cloudkillScan: yes counts Cloudkill resolving while its caster's commander
// is on the battlefield or in the command zone (X is its mana value, 3), no
// while it is elsewhere (X is 0).
func cloudkillScan(ref *GorgeSide, ev events.Event, tal *mv2Tally) {
	if ev.Kind != events.Resolve || mv2Named(ref, ev.Obj) != "Cloudkill" {
		return
	}
	tal.resolved++
	o := ref.E.G.Obj(ev.Obj)
	for _, c := range ref.E.G.Players[o.Owner].Commanders {
		if z := ref.E.G.Obj(c).Zone; z == state.ZCommand || z == state.ZBattlefield {
			tal.yes++
			return
		}
	}
	tal.no++
}

// fatalPushScan: yes counts Fatal Push resolving at a mana-value 3 or 4
// creature with revolt (the Count$Revolt.4.2 bound's 4 arm destroys it), no
// at one without revolt (the 2 arm spares it).
func fatalPushScan() mv2Scan {
	tgt := map[state.ObjID]state.ObjID{}
	return func(ref *GorgeSide, ev events.Event, tal *mv2Tally) {
		switch {
		case ev.Kind == events.TargetsChosen && ev.Amount == 0 && len(ev.IDs) == 1 && mv2Named(ref, ev.Obj) == "Fatal Push":
			tgt[ev.Obj] = ev.IDs[0]
		case ev.Kind == events.Resolve && mv2Named(ref, ev.Obj) == "Fatal Push":
			tal.resolved++
			to := ref.E.G.Obj(tgt[ev.Obj])
			if to == nil {
				return
			}
			if mv := state.ManaValue(ref.E.G, to); mv < 3 || mv > 4 {
				return
			}
			if ref.E.RevoltHolds(ref.E.G.Obj(ev.Obj).Controller) {
				tal.yes++
			} else {
				tal.no++
			}
		}
	}
}

// dirgeScan counts Dirge of Dread's resolutions (no split).
func dirgeScan(ref *GorgeSide, ev events.Event, tal *mv2Tally) {
	if ev.Kind == events.Resolve && mv2Named(ref, ev.Obj) == "Dirge of Dread" {
		tal.resolved++
	}
}

// abzanScan: resolved counts Abzan Beastmaster's upkeep trigger pushes, yes
// the upkeep draws (the gate held; no other card of the decks draws then), so
// no is the pushes that drew nothing.
func abzanScan() mv2Scan {
	step := state.Step(0)
	var pushes, draws int
	return func(ref *GorgeSide, ev events.Event, tal *mv2Tally) {
		switch ev.Kind {
		case events.StepChange:
			step = ev.Step
		case events.TriggerPush:
			if mv2Named(ref, ev.Obj) == "Abzan Beastmaster" {
				pushes++
			}
		case events.Draw:
			if step == state.StepUpkeep { // only the Beastmaster draws in an upkeep here
				draws++
			}
		}
		tal.resolved, tal.yes, tal.no = pushes, draws, pushes-draws
	}
}

func TestMBCValues2Lowers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		seats     [2]mv2Seat
		commander bool
		scan      mv2Scan
		minYes    int
		minNo     int
	}{
		{"count:ValidBattlefield/Cloudkill", mv2Cloudkill, true, cloudkillScan, 3, 0},
		{"count:Revolt/Fatal Push", mv2FatalPush, false, fatalPushScan(), 3, 3},
		{"PumpAll keyword above bit 15/Dirge of Dread", mv2Dirge, false, dirgeScan, 0, 0},
		{"ConditionCheckSVar beside ConditionPresent/Abzan Beastmaster", mv2Abzan, false, abzanScan(), 3, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var tal mv2Tally
			mv2Run(t, c.seats, c.commander, 40, c.scan, &tal)
			t.Logf("%s: %d resolved, %d yes, %d no", c.name, tal.resolved, tal.yes, tal.no)
			if tal.resolved < 3 || tal.yes < c.minYes || tal.no < c.minNo {
				t.Errorf("precondition: want at least 3 resolved, %d yes, %d no; got %d, %d, %d", c.minYes, c.minNo, tal.resolved, tal.yes, tal.no)
			}
		})
	}
}
