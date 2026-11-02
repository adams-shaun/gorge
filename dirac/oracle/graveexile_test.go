package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The graveexile lockstep: one constructed pool for the five grammar
// sub-classes it lifts -- the Graveyard,Exile hidden pick and ChangeZoneAll
// (A), the Crew/Saddle TapPowerValue bonus (B), TargetedAndYou (C) and
// Defined$ Targeted.<qualifier> (D). E (UnlessPayer$ ThisTargetedController)
// is deferred: kgen refuses it. Run as its own shard (-run
// '^TestGraveExileAndTapPowerLockstep$'), not inside the remainder Lockstep$ shard.
var graveexileDecks = [][2][]cardset.Entry{{
	{
		{Name: "Ainok Wayfarer", Count: 4}, {Name: "Bramble Familiar", Count: 2}, {Name: "Picklock Prankster", Count: 2},
		{Name: "Most Decrepit Old Bird", Count: 2}, {Name: "Wasteful Harvest", Count: 2},
		{Name: "Defend the Rider", Count: 3},
		{Name: "Cantankerous Keepers", Count: 2}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Town Greeter", Count: 2}, {Name: "Adventurer's Inn", Count: 2},
		{Name: "Forest", Count: 13}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Hotshot Mechanic", Count: 4}, {Name: "Deathless Pilot", Count: 4}, {Name: "Dynamite Diver", Count: 4},
		{Name: "Weatherlight", Count: 4}, {Name: "Brightfield Glider", Count: 3},
		{Name: "Savannah Lions", Count: 4}, {Name: "Grizzly Bears", Count: 1},
		{Name: "Cloudspire Captain", Count: 2},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 3}, {Name: "Swamp", Count: 3},
	},
}, {
	{
		{Name: "Sky Crier", Count: 4}, {Name: "Thran Spider", Count: 4},
		{Name: "Loran of the Third Path", Count: 2},
		{Name: "Shields Up!", Count: 4}, {Name: "Tormod's Crypt", Count: 4},
		{Name: "Savannah Lions", Count: 3}, {Name: "Coral Merfolk", Count: 3},
		{Name: "Plains", Count: 10}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Gray Ogre", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 5},
		{Name: "Kaseto, Orochi Archmage", Count: 4},
		{Name: "Giant Spider", Count: 2}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Forest", Count: 9}, {Name: "Island", Count: 9},
	},
}}

// graveexileCarriers counts each lifted spelling's carrier plays. The A pick
// carriers mill and choose; Cantankerous Keepers is the ChangeZoneAll arm.
// A_change_all counts Keepers casts (a proxy: the ETB resolves ChangeZoneAll
// in this pool), not ChangeZoneAll executions.
var graveexileCarriers = map[string][]string{
	"A_pick":           {"A_pick"},
	"A_change_all":     {"Cantankerous Keepers"},
	"crew":             {"crew_activation"},
	"saddle":           {"saddle_activation"},
	"defend_token":     {"pilot_picked"},
	"targeted_and_you": {"Sky Crier", "Thran Spider", "Loran of the Third Path"},
	"targeted_true":    {"targeted_true"},
}

// graveexilePolicy casts or activates whenever it can, else random, and counts the
// carrier plays the gorge side poses. It also counts Shields Up! target
// picks where the creature (true) and non-creature (false) qualifier arms are
// available, and the hidden-pick, Crew and Saddle occurrences (graveexileCount).
func graveexilePolicy(seed uint64, played map[string]int) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	lastAct := ""
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			k := a.Kind()
			if k != ActCast && k != ActActivate {
				continue
			}
			// Keep Tormod's Crypt on the battlefield so Shields Up! can
			// choose its non-creature artifact arm; otherwise the
			// cast/activate-always policy immediately sacrifices it.
			if k == ActActivate && c.G != nil {
				if op, ok := c.G.GorgeOption(a); ok {
					if ob := c.G.E.G.Obj(op.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Tormod's Crypt" {
						continue
					}
				}
			}
			buf = append(buf, a)
		}
		var a Canon
		if len(buf) == 0 {
			// The random fallback must not sacrifice Tormod's Crypt; the
			// D false arm needs it on the battlefield.
			opts := c.D.Options
			var keep []Canon
			for _, o := range opts {
				if !graveexileTormodActivation(c, o) {
					keep = append(keep, o)
				}
			}
			if len(keep) > 0 {
				d := *c.D
				d.Options = keep
				a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			} else {
				a = rnd.Pick(c)
			}
		} else {
			// Once Tormod's Crypt and a plain creature share the board,
			// cast Shields Up! so the D qualifier arms are reached.
			if c.G != nil {
				for _, o := range buf {
					op, ok := c.G.GorgeOption(o)
					if !ok {
						continue
					}
					ob := c.G.E.G.Obj(op.Obj)
					if ob == nil || ob.Face() == nil || ob.Face().Name != "Shields Up!" {
						continue
					}
					if graveexileShieldsReady(c.G) {
						a = o
						break
					}
				}
			}
			if a.Kind() != ActCast || !graveexileShieldsReady(c.G) {
				d := *c.D
				d.Options = buf
				a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		if k := a.Kind(); (k == ActCast || k == ActActivate) && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					played[ob.Face().Name]++
				}
			}
		}
		if a.Kind() == ActTarget && c.G != nil {
			// Shields Up! targets an artifact or creature you control and
			// its DBPutCounter reads Defined$ Targeted.Creature. A decision
			// offering both Tormod's Crypt (artifact, not a creature) and a
			// plain creature is that spell; Tormod is the false arm and any
			// other pick is the true arm. Loran's artifact/enchantment target
			// has no plain creature, so it cannot be counted here.
			crypt, plain := false, false
			for _, o := range c.D.Options {
				if o.Kind() != ActTarget {
					continue
				}
				op, ok := c.G.GorgeOption(o)
				if !ok {
					continue
				}
				ob := c.G.E.G.Obj(op.Obj)
				if ob == nil || ob.Face() == nil {
					continue
				}
				switch ob.Face().Name {
				case "Tormod's Crypt":
					crypt = true
				case "Loran of the Third Path", "Sky Crier", "Grizzly Bears", "Coral Merfolk":
					plain = true
				}
			}
			if crypt && plain {
				// Alternate the two qualifier arms deterministically.
				wantCrypt := c.Seq%2 == 0
				for _, o := range c.D.Options {
					if o.Kind() != ActTarget {
						continue
					}
					op, ok := c.G.GorgeOption(o)
					if !ok {
						continue
					}
					ob := c.G.E.G.Obj(op.Obj)
					if ob == nil || ob.Face() == nil {
						continue
					}
					isCrypt := ob.Face().Name == "Tormod's Crypt"
					if isCrypt != wantCrypt {
						continue
					}
					a = o
					if isCrypt {
						played["targeted_false"]++
					} else {
						played["targeted_true"]++
					}
					break
				}
			}
		}
		graveexileCount(c, a, played, &lastAct)
		return a
	})
}

// graveexileCount counts occurrences of the lifted spellings themselves: a hidden
// pick decision over Graveyard/Exile objects (keyed by the resolving stack
// object's face, so an adventure face is told from its creature), a
// Vehicle's Crew and a Mount's Saddle activation, and a Pilot token tapped
// (picked) into a payment.
func graveexileCount(c *PickCtx, a Canon, played map[string]int, lastAct *string) {
	if c.G == nil {
		return
	}
	pickZone := false
	for _, o := range c.D.Options {
		if o.Kind() < ActChoose {
			continue
		}
		if op, ok := c.G.GorgeOption(o); ok && op.Obj != 0 {
			if ob := c.G.E.G.Obj(op.Obj); ob != nil && (ob.Zone == state.ZGraveyard || ob.Zone == state.ZExile) {
				pickZone = true
				break
			}
		}
	}
	if pickZone {
		played["A_pick"]++
		if st := c.G.E.G.Stack; len(st) > 0 {
			if ob := c.G.E.G.Obj(st[len(st)-1]); ob != nil && ob.Face() != nil {
				played["A_pick:"+ob.Face().Name]++
			}
		}
	}
	if op, ok := c.G.GorgeOption(a); ok && op.Obj != 0 {
		if ob := c.G.E.G.Obj(op.Obj); ob != nil && ob.Face() != nil {
			n := ob.Face().Name
			if a.Kind() == ActTarget && *lastAct == "Kaseto, Orochi Archmage" {
				// Kaseto's Defined$ Targeted.Snake: Kaseto is the Snake
				if n == "Kaseto, Orochi Archmage" {
					played["kaseto_true"]++
				} else {
					played["kaseto_false"]++
				}
			}
			if a.Kind() == ActActivate || a.Kind() == ActCast {
				*lastAct = n
			}
			if a.Kind() == ActActivate {
				switch n {
				case "Weatherlight":
					played["crew_activation"]++
				case "Brightfield Glider":
					played["saddle_activation"]++
				}
			}
			if a.Kind() != ActCast && a.Kind() != ActActivate && a.Kind() != ActTarget && strings.Contains(n, "Pilot") && ob.Zone == state.ZBattlefield {
				played["pilot_picked"]++
				played["pilot_picked:"+n]++
			}
		}
	}
}

// graveexileTormodActivation reports an activation of Tormod's Crypt, whose
// sacrifice cost would remove the D false-arm target.
func graveexileTormodActivation(c *PickCtx, a Canon) bool {
	if a.Kind() != ActActivate || c.G == nil {
		return false
	}
	op, ok := c.G.GorgeOption(a)
	if !ok {
		return false
	}
	ob := c.G.E.G.Obj(op.Obj)
	return ob != nil && ob.Face() != nil && ob.Face().Name == "Tormod's Crypt"
}

// graveexileShieldsReady reports a Tormod's Crypt and a plain creature on the
// battlefield, the board Shields Up!'s two qualifier arms need.
func graveexileShieldsReady(g *GorgeSide) bool {
	crypt, plain := false, false
	for _, ob := range g.E.G.Objs {
		if ob.Face() == nil || ob.Zone != state.ZBattlefield {
			continue
		}
		switch ob.Face().Name {
		case "Tormod's Crypt":
			crypt = true
		case "Savannah Lions", "Coral Merfolk", "Sky Crier":
			plain = true
		}
	}
	return crypt && plain
}

func graveexileEnv(t *testing.T) (cardset.Set, func(mi int) (*Matchup, [2][]*cards.Card, error)) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("graveexile")
	if err != nil {
		t.Fatal(err)
	}
	build := func(mi int) (*Matchup, [2][]*cards.Card, error) {
		md := graveexileDecks[mi]
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
		m, err := NewMatchup(reg, [2]string{"kc9-a", "kc9-b"}, decks, 1)
		return m, decks, err
	}
	return set, build
}

func TestGraveExileAndTapPowerLockstep(t *testing.T) {
	t.Parallel()

	set, build := graveexileEnv(t)
	played := map[string]int{}
	games := 0
	for mi := range graveexileDecks {
		md := graveexileDecks[mi]
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 512; seed++ {
			m, _, err := build(mi)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, graveexilePolicy(seed, played), RunConfig{Config: cfg, MaxDecisions: 1500, MaxTurns: 16})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, carrier plays %v", games, played)
	for spelling, names := range graveexileCarriers {
		n := 0
		for _, name := range names {
			n += played[name]
		}
		if n < 10 {
			t.Errorf("%s: %d carrier plays, want at least 10 (%v)", spelling, n, names)
		}
	}
	for _, k := range []string{"kaseto_true", "kaseto_false"} {
		if played[k] < 10 {
			t.Errorf("Targeted.Snake %s: %d picks, want at least 10", k, played[k])
		}
	}
	if played["targeted_false"] < 10 {
		t.Errorf("Targeted.<q> false arm: %d picks, want at least 10", played["targeted_false"])
	}
}

// TestGraveExileAndTapPowerMutation ignores the TapPowerValue bonus and requires the lockstep
// to diverge. The grave-exile half is mutated by the kernel's
// TestGraveExilePickMutation and TestGraveExileChangeZoneAllMutation instead: the constructed pool has no supported
// mill-to-exile effect (Leyline of the Void and Rest in Peace are repl-hard),
// so no full-game path puts a remembered milled card in exile.
func TestGraveExileAndTapPowerMutation(t *testing.T) {
	set, build := graveexileEnv(t)
	old := dirac.DebugIgnoreTapPower
	dirac.DebugIgnoreTapPower = true
	defer func() { dirac.DebugIgnoreTapPower = old }()
	diverged := false
	for mi := range graveexileDecks {
		md := graveexileDecks[mi]
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 256 && !diverged; seed++ {
			m, _, err := build(mi)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, graveexilePolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 1500, MaxTurns: 16})
			CloseSide(k)
			if res.Div != nil {
				t.Logf("matchup %d seed %d diverged at decision %d", mi, seed, res.Div.At)
				diverged = true
			}
		}
	}
	if !diverged {
		t.Fatal("mutation diverged in no game")
	}
}
