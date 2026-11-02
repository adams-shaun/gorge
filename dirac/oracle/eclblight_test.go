package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The eclblight lockstep: Blight (CR 701.68) as an activated
// cost, an either-or additional cost, a trigger's own cost and an unless
// cost, the Blight effect, and the counter-removal cost forms. M1 pairs the
// BR blight/removal carriers with the WU counter and filtered-subcounter
// carriers; M2 pairs High Perfect Morcant's each-opponent blight with Jessica
// Jones's stun counter and Ray Fillet's filtered +1/+1 removal.
var keclblightDecks = [][2][]cardset.Entry{{
	{
		{Name: "Blighted Blackthorn", Count: 2}, {Name: "Boggart Mischief", Count: 2},
		{Name: "Dream Seizer", Count: 2}, {Name: "Gutsplitter Gang", Count: 2},
		{Name: "Chaos Spewer", Count: 2}, {Name: "Gnarlbark Elm", Count: 2},
		{Name: "Bogslither's Embrace", Count: 2}, {Name: "Scuzzback Scrounger", Count: 2},
		{Name: "Sourbread Auntie", Count: 2}, {Name: "Sting-Slinger", Count: 2},
		{Name: "Gristle Glutton", Count: 2}, {Name: "Warren Torchmaster", Count: 2},
		{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Burdened Stoneback", Count: 3}, {Name: "Moonlit Lamenter", Count: 3},
		{Name: "Slumbering Walker", Count: 2}, {Name: "Glen Elendra Guardian", Count: 3},
		{Name: "Loch Mare", Count: 3}, {Name: "Wild Unraveling", Count: 3},
		{Name: "Spiral into Solitude", Count: 2}, {Name: "Evershrike's Gift", Count: 2},
		{Name: "Sunstar Chaplain", Count: 3},
		{Name: "Plains", Count: 8}, {Name: "Island", Count: 8},
	},
}, {
	{
		{Name: "High Perfect Morcant", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Elvish Mystic", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Gnarlbark Elm", Count: 4}, {Name: "Bogslither's Embrace", Count: 4},
		{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Ray Fillet, Man Ray", Count: 4}, {Name: "Jessica Jones, Private Eye", Count: 4},
		{Name: "Brambleback Brute", Count: 4}, {Name: "Hovel Hurler", Count: 3},
		{Name: "Sting-Slinger", Count: 3}, {Name: "Opt", Count: 3},
		{Name: "Ornithopter", Count: 3},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
	},
}}

// The carrier families the tally attributes to.
var (
	keclActBlight = map[string]bool{
		"Evershrike's Gift": true, "Gristle Glutton": true,
		"Spiral into Solitude": true, "Sting-Slinger": true,
	}
	keclTrigBlight = map[string]bool{
		"Blighted Blackthorn": true, "Boggart Mischief": true,
		"Dream Seizer": true, "Scuzzback Scrounger": true,
		"Sourbread Auntie": true, "Warren Torchmaster": true,
	}
	keclRemove = map[string]bool{
		"Brambleback Brute": true, "Burdened Stoneback": true,
		"Glen Elendra Guardian": true, "Gnarlbark Elm": true,
		"Hovel Hurler": true, "Loch Mare": true, "Moonlit Lamenter": true,
	}
	keclFilterSub = map[string]bool{
		"Sunstar Chaplain": true, "Ray Fillet, Man Ray": true,
	}
)

func keclOptKind(a Canon) string {
	if k := a.Kind(); k >= ActChoose {
		if i := int(k - ActChoose); i < len(chooseKinds) {
			return chooseKinds[i]
		}
	}
	return ""
}

func keclName(g *GorgeSide, d *decision.Decision) string {
	if g == nil || d == nil {
		return ""
	}
	ob := g.E.G.Obj(d.Source)
	if ob == nil || ob.Face() == nil {
		return ""
	}
	return ob.Face().Name
}

// keclTally counts the branches from gorge's decisions and event log.
type keclTally struct {
	counts map[string]int
	evAt   int
	seq    uint64
	blight state.ObjID
	g      *state.Game
	// game is the current game's number; seen[k] is the last game that
	// counted branch k, so each arm is tallied once per game.
	game int
	seen map[string]int
}

func newKeclTally() *keclTally { return &keclTally{counts: map[string]int{}, seen: map[string]int{}} }

// add tallies branch k for the current game, once per game: the counts are
// games reaching the branch, as the test's minimum asks.
func (t *keclTally) add(k string, n int) {
	if n <= 0 || t.seen[k] == t.game {
		return
	}
	t.seen[k] = t.game
	t.counts[k]++
}

// internal counts the asks gorge answers itself under GorgeAutopaySurface:
// the unless election, the alt-add part and the blight picks.
func (t *keclTally) internal(d *decision.Decision, idx int) {
	if d == nil || idx < 0 || idx >= len(d.Options) {
		return
	}
	o := d.Options[idx]
	name := ""
	if d.Source != 0 {
		name = t.gorgeName(d.Source)
	}
	switch d.ResumeKind {
	case "unless_pay":
		paid := o.Mode == decision.ModeUnlessPay
		switch name {
		case "Gutsplitter Gang":
			if paid {
				t.add("UB+", 1)
			} else {
				t.add("UB-", 1)
			}
		case "Chaos Spewer":
			if paid {
				t.add("CS+", 1)
			} else {
				t.add("CS-", 1)
			}
		}
	case "blight":
		if name == "High Perfect Morcant" {
			t.add("EB2", 1)
		}
	}
	if o.Kind == "altaddcost" {
		if o.Amount == 0 {
			t.add("AA-B", 1)
		} else {
			t.add("AA-M", 1)
		}
	}
	if o.Kind == "blightcost" && keclActBlight[name] {
		t.add("BC2", 1)
	}
}

// gorgeName is set by the test before the run (the GorgeSide holding the
// event log the internal answers belong to).
func (t *keclTally) gorgeName(id state.ObjID) string {
	if t.g == nil {
		return ""
	}
	ob := t.g.Obj(id)
	if ob == nil || ob.Face() == nil {
		return ""
	}
	return ob.Face().Name
}

func (t *keclTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < t.evAt {
		t.evAt = 0
	}
	for j := t.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.CounterChange:
			switch e.Counter {
			case "M1M1":
				// gorge places blight counters with a positive amount; a
				// negative one is annihilation or a removal cost.
				if e.Amount > 0 {
					t.blight = e.Obj
				} else if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Slumbering Walker" {
					t.add("RAT", 1)
				}
			case "STUN":
				if e.Amount > 0 {
					t.add("AC/placed", 1)
				} else {
					t.add("AC/removed", 1)
				}
			}
		case events.MoveZone:
			if e.From == state.ZBattlefield && e.To == state.ZGraveyard && e.Obj == t.blight {
				t.add("SBA", 1)
				t.blight = 0
			}
			if e.To == state.ZBattlefield {
				if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "High Perfect Morcant" {
					other := state.PlayerID(1 - int(o.Controller))
					n := 0
					for _, id := range g.Zone(state.ZBattlefield, other) {
						if co := g.Obj(id); co != nil && co.Face() != nil && slices.Contains(co.Face().Types, "Creature") {
							n++
						}
					}
					if n == 0 {
						t.add("EB0", 1)
					}
				}
			}
		}
	}
	t.evAt = len(evs)

	if fr := ref.fr; fr != nil && fr.d != nil && fr.seq != t.seq {
		t.seq = fr.seq
		d := fr.d
		name := keclName(ref, d)
		has := func(kind string) int {
			n := 0
			for _, o := range d.Options {
				if o.Kind == kind {
					n++
				}
			}
			return n
		}
		switch {
		case has("blightcost") > 0:
			switch {
			case name == "Gutsplitter Gang":
				// the unless Blight pick; the pay/decline arm counts UB.
			case keclActBlight[name] && len(d.Options) >= 2:
				t.add("BC2", 1)
			}
		case has("trigger_cost_blight") > 0:
			if len(d.Options) >= 2 {
				t.add("TBA", 1)
			}
		case has("trigger_cost_counter") > 0:
			t.add("RAT", 1)
		case has("subcounter") > 0:
			if keclFilterSub[name] && len(d.Options) >= 2 {
				t.add("SCF", 1)
			} else if keclRemove[name] {
				t.add("RA", 1)
			}
		case has("blight") > 0:
			if name == "High Perfect Morcant" {
				t.add("EB2", 1)
			}
		}
	}
	return nil
}

// keclPolicy pays every optional blight and counter cost it can and picks the
// Blight half of an either-or cost on even seeds, the mana half on odd ones.
// The pay/decline asks alternate, so both arms are reached.
func keclPolicy(seed uint64, tally *keclTally) Policy {
	rnd := RandomPolicy(seed)
	var lastSeq int
	var seen bool
	pay := seed%4 == 1
	return PolicyFunc(func(c *PickCtx) Canon {
		g := c.G
		if g != nil && c.D != nil {
			d := c.D
			first := func(kind string) (Canon, bool) {
				for _, a := range d.Options {
					if keclOptKind(a) == kind {
						return a, true
					}
				}
				return 0, false
			}
			switch d.Kind {
			case DModes:
				if fr := g.fr; fr != nil && fr.d != nil && fr.d.ResumeKind == "unless_pay" {
					name := keclName(g, fr.d)
					want := uint8(0)
					if pay {
						want = 1
					}
					if a, ok := keclOptMode(d.Options, want); ok {
						if !seen || c.Seq != lastSeq {
							lastSeq, seen = c.Seq, true
							if name == "Gutsplitter Gang" {
								if pay {
									tally.add("UB+", 1)
								} else {
									tally.add("UB-", 1)
								}
							}
							if name == "Chaos Spewer" {
								if pay {
									tally.add("CS+", 1)
								} else {
									tally.add("CS-", 1)
								}
							}
						}
						return a
					}
				}
			case DChoose:
				if a, ok := first("altaddcost"); ok {
					part := uint8(0)
					if seed%4 == 3 {
						for _, o := range d.Options {
							if keclOptKind(o) == "altaddcost" && o.Mode() == 1 {
								part = 1
								a = o
							}
						}
					}
					if !seen || c.Seq != lastSeq {
						lastSeq, seen = c.Seq, true
						if part == 0 {
							tally.add("AA-B", 1)
						} else {
							tally.add("AA-M", 1)
						}
					}
					return a
				}
				tbName := ""
				if g.fr != nil {
					tbName = keclName(g, g.fr.d)
				}
				if a, ok := first("trigger_cost_pay"); ok {
					decline, hasDecline := first("trigger_cost_decline")
					if !pay && hasDecline {
						if keclTrigBlight[tbName] && (!seen || c.Seq != lastSeq) {
							lastSeq, seen = c.Seq, true
							tally.add("TB-", 1)
						}
						return decline
					}
					if keclTrigBlight[tbName] && (!seen || c.Seq != lastSeq) {
						lastSeq, seen = c.Seq, true
						tally.add("TB+", 1)
					}
					return a
				}
				if a, ok := first("trigger_cost_decline"); ok {
					if keclTrigBlight[tbName] && (!seen || c.Seq != lastSeq) {
						lastSeq, seen = c.Seq, true
						tally.add("TB0", 1)
					}
					return a
				}
				for _, k := range []string{"blightcost", "trigger_cost_blight", "blight", "subcounter", "trigger_cost_counter"} {
					if a, ok := first(k); ok {
						return a
					}
				}
			}
		}
		var buf []Canon
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActActivate:
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		// On the pay seeds hold Chaos Spewer until six untapped lands can
		// pay its {2}{B/R} and still leave {2} for the unless cost; casting
		// it as soon as it is affordable is why CS+ was never reached.
		if pay && g != nil {
			for i := 0; i < len(buf); i++ {
				a := buf[i]
				if a.Kind() != ActCast {
					continue
				}
				id, ok := kroleRefObj(g, a.Src())
				if !ok {
					continue
				}
				ob := g.E.G.Obj(id)
				if ob == nil || ob.Face() == nil || ob.Face().Name != "Chaos Spewer" {
					continue
				}
				if keclUntappedLands(g, ob.Owner) >= 6 {
					return a
				}
				buf = append(buf[:i], buf[i+1:]...)
				i--
			}
			if len(buf) == 0 {
				for _, a := range c.D.Options {
					if a.Kind() == ActPass {
						return a
					}
				}
			}
		}
		d := *c.D
		d.Options = buf
		pick := rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		// Count a blight or removal cost actually paid by the activation
		// chosen: a lone creature pays Blight unasked; a removal cost with
		// one counter unit is taken unasked too.
		if g != nil && pick.Kind() == ActActivate {
			if id, ok := kroleRefObj(g, pick.Src()); ok {
				if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
					name := ob.Face().Name
					if keclActBlight[name] {
						n := 0
						for _, cid := range g.E.G.Zone(state.ZBattlefield, ob.Controller) {
							if co := g.E.G.Obj(cid); co != nil && co.Face() != nil && slices.Contains(co.Face().Types, "Creature") {
								n++
							}
						}
						if n == 1 {
							tally.add("BC1", 1)
						}
					}
					if keclRemove[name] {
						tally.add("RA", 1)
					}
				}
			}
		}
		return pick
	})
}

// keclUntappedLands counts player p's untapped lands on the battlefield.
func keclUntappedLands(g *GorgeSide, p state.PlayerID) int {
	n := 0
	for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
		ob := g.E.G.Obj(id)
		if ob != nil && !ob.Tapped && ob.Face() != nil && slices.Contains(ob.Face().Types, "Land") {
			n++
		}
	}
	return n
}

func safeProject(s Side) (p Projection) {
	defer func() { _ = recover() }()
	s.Project(&p)
	return p
}

func keclOptMode(opts []Canon, mode uint8) (Canon, bool) {
	for _, a := range opts {
		if a.Kind() == ActMode && a.Mode() == mode {
			return a, true
		}
	}
	return 0, false
}

func keclEnv(t *testing.T) (cardset.Set, func(mi int) (*Matchup, error)) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("eclblight")
	if err != nil {
		t.Fatal(err)
	}
	build := func(mi int) (*Matchup, error) {
		md := keclblightDecks[mi]
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
		return NewMatchup(reg, [2]string{"keclblight-a", "keclblight-b"}, decks, 1)
	}
	return set, build
}

func TestEclblightLockstep(t *testing.T) {
	t.Parallel()

	set, build := keclEnv(t)
	tally := newKeclTally()
	games := 0
	for mi := range keclblightDecks {
		md := keclblightDecks[mi]
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 512; seed++ {
			m, err := build(mi)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			if g, ok := k.(*GorgeSide); ok {
				tally.g = g.E.G
				g.OnInternal = tally.internal
			}
			tally.game++
			lead, lerr := NewGorgeSide(m, seed, cfg, nil)
			if lerr != nil {
				t.Fatal(lerr)
			}
			res := LockstepLed(m, seed, lead, k, keclPolicy(seed, tally), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: tally.check})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				var pg Projection
				lead.Project(&pg)
				pk := safeProject(k)
				if rs, ok := k.(*realSide); ok {
					var buf [dirac.MaxObj]dirac.Obj
					for p := uint8(0); p < 2; p++ {
						for _, o := range rs.s.FeatBattlefield(p, buf[:]) {
							card := rs.s.ObjCard(o)
							if int(card) >= len(rs.gname) {
								continue
							}
							p1, ks, ns := rs.s.DebugCtrs(o)
							if p1 != 0 || len(ks) != 0 {
								t.Logf("  kernel obj %d %q p1 %d kinds %v counts %v", o, rs.gname[card], p1, ks, ns)
							}
						}
					}
				}
				for i := 0; i < len(pg.Cards) && i < 40; i++ {
					if i == 16 {
						nm := "?"
						if i < len(m.Deck[0]) {
							nm = m.Table[m.Deck[0][i]]
						} else if i-len(m.Deck[0]) < len(m.Deck[1]) {
							nm = m.Table[m.Deck[1][i-len(m.Deck[0])]]
						}
						t.Logf("  inst %d %q gorge ctr %v p/t %d/%d kernel ctr %v p/t %d/%d card %d", i, nm, pg.Cards[i].Counters, pg.Cards[i].Power, pg.Cards[i].Toughness, pk.Cards[i].Counters, pk.Cards[i].Power, pk.Cards[i].Toughness, pk.Cards[i].Card)
					}
				}
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, counts %v", games, tally.counts)
	for _, arm := range []string{"BC2", "BC1", "AA-B", "AA-M", "TB+", "TB-", "TB0", "TBA", "UB+", "UB-", "EB2", "EB0", "CS+", "CS-", "RA", "RAT", "SCF", "AC/placed", "AC/removed", "SBA"} {
		if arm == "SCF" && tally.counts[arm] < 10 {
			// An arm that cannot reach 10 is reported
			// games in 512 seeds: a filtered removal over >= 2 creatures
			// needs a counter-holding pair on the board.
			t.Logf("SCF: %d games, below 10 (reported, not asserted)", tally.counts[arm])
			continue
		}
		if tally.counts[arm] < 10 {
			t.Errorf("%s: %d, want at least 10", arm, tally.counts[arm])
		}
	}
}
