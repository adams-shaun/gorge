package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The AffectedZone$ Stack keyword-grant forms gap4-06 lowers: a gated grant
// (Maelstrom Nexus, Abaddon the Despoiler, Bloodbraid Marauder, Wild-Magic
// Sorcerer), an Effect-issued "next spell this turn" grant (Dark Apostle,
// Sloppity Bilepiper, Smoldering Stagecoach), a cast-from-hand grant
// (Wildsear, Scouring Maw; Yidris, Maelstrom Wielder), the two-instance grant
// (Zhulodok, Void Gorger) and the Effect-issued YouCtrl-only grants (Archway
// of Innovation, Flockchaser Phantom, Wand of the Worldsoul, Don & Raph).
// The fixture's cheap spells of each type let the cascades fire and find a
// free cast or a bottom.

// stackgrant4Decks is the four-seat pod: each seat carries one grant family
// beside cheap spells for the cascade to find.
func stackgrant4Decks() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		{
			{Name: "Abaddon the Despoiler", Count: 4}, {Name: "Maelstrom Nexus", Count: 2},
			{Name: "Bloodbraid Marauder", Count: 4}, {Name: "Wild-Magic Sorcerer", Count: 2},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Gray Ogre", Count: 2},
			{Name: "Lightning Bolt", Count: 4}, {Name: "Opt", Count: 2},
			{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 4},
			{Name: "Island", Count: 2}, {Name: "Forest", Count: 1}, {Name: "Plains", Count: 1},
		},
		{
			{Name: "Dark Apostle", Count: 4}, {Name: "Sloppity Bilepiper", Count: 4},
			{Name: "Smoldering Stagecoach", Count: 4}, {Name: "Shock", Count: 4},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Elite Vanguard", Count: 4},
			{Name: "Mind Stone", Count: 4}, {Name: "Mountain", Count: 6}, {Name: "Swamp", Count: 6},
		},
		{
			{Name: "Wildsear, Scouring Maw", Count: 4}, {Name: "Yidris, Maelstrom Wielder", Count: 2},
			{Name: "Zhulodok, Void Gorger", Count: 2}, {Name: "Seal of Primordium", Count: 4},
			{Name: "Journey to Nowhere", Count: 4}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Savannah Lions", Count: 4}, {Name: "Myr Enforcer", Count: 4},
			{Name: "Forest", Count: 6}, {Name: "Plains", Count: 4}, {Name: "Mountain", Count: 2},
		},
		{
			{Name: "Archway of Innovation", Count: 4}, {Name: "Flockchaser Phantom", Count: 4},
			{Name: "Wand of the Worldsoul", Count: 4}, {Name: "Don & Raph, Hard Science", Count: 4},
			{Name: "Pilgrim's Eye", Count: 4}, {Name: "Ornithopter", Count: 4},
			{Name: "Memnite", Count: 4}, {Name: "Mind Stone", Count: 4},
			{Name: "Plains", Count: 4}, {Name: "Island", Count: 4},
		},
	}
}

type stackgrant4Pod struct {
	reg   *cards.Registry
	set   cardset.Set
	tbl   *kop.Table
	decks [4][]cardset.Entry
}

func newStackgrant4Pod(t *testing.T) stackgrant4Pod {
	t.Helper()
	return newStackgrant4PodOf(t, stackgrant4Decks())
}

func newStackgrant4PodOf(t *testing.T, decks [4][]cardset.Entry) stackgrant4Pod {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("stackgrant4")
	if err != nil {
		t.Fatal(err)
	}
	lists := make([][]cardset.Entry, len(decks))
	for i, d := range decks {
		n := 0
		for _, e := range d {
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d: %d cards", i, n)
		}
		lists[i] = d
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	return stackgrant4Pod{reg: reg, set: set, tbl: tbl, decks: decks}
}

func stackgrant4Expand(d []cardset.Entry) []string {
	var out []string
	for _, e := range d {
		for j := 0; j < e.Count; j++ {
			out = append(out, e.Name)
		}
	}
	return out
}

// stackgrantCascadeProbe counts the cascade free-cast elections the kernel
// poses; a lockstep green without one is a game where nothing cascaded.
type stackgrantCascadeProbe struct {
	base Policy
	rs   *realSide
	n    *int
}

func (p *stackgrantCascadeProbe) Pick(c *PickCtx) Canon {
	if p.rs.d.Sub == dirac.ChGenCastFree {
		*p.n++
	}
	return p.base.Pick(c)
}

// play4 plays one four-seat seed and returns the result and the cascade
// election count.
func (p stackgrant4Pod) play4(t *testing.T, seed uint64, maxTurns int32) (Result, int) {
	t.Helper()
	decks := p.decks
	expand := stackgrant4Expand
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"},
		Cards:     [2][]string{expand(decks[0]), expand(decks[1])},
		MoreDecks: []string{"x2", "x3"},
		MoreCards: [][]string{expand(decks[2]), expand(decks[3])},
		Seed:      seed, PassElision: true, Surface: "full"}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	k, err := SetSide(p.set, p.tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	probe := &stackgrantCascadeProbe{base: castFirstPolicy(seed), rs: k.(*realSide), n: &n}
	res := LockstepLed(m, seed, lead, k, probe, rc)
	return res, n
}

// TestStackGrantGatesFourSeatLockstep plays the stack-grant pod at four
// seats (Commander's count): every granted keyword, gate, Effect-issued
// grant and cast-origin word must pose gorge's exact option list, and the
// decisions reached must include a cascade election.
func TestStackGrantGatesFourSeatLockstep(t *testing.T) {
	p := newStackgrant4Pod(t)
	total := 0
	for _, seed := range []uint64{1, 2, 3, 5} {
		res, n := p.play4(t, seed, 40)
		if res.Div != nil {
			lo := max(res.Div.At-12, 0)
			t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v",
				seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
				res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced,
				actionStrings(res.Actions[lo:res.Div.At]))
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		total += n
		t.Logf("seed %d: %d decisions, over %v %s; %d cascade election(s)", seed, res.Decisions, res.Over, res.Stall, n)
	}
	if total == 0 {
		t.Fatal("no cascade election in any four-seat game: the grants never fired")
	}
}

// stackgrant4Pair is one two-seat matchup of the fixture.
type stackgrant4Pair struct {
	a, b []cardset.Entry
}

func stackgrant4Pairs() []stackgrant4Pair {
	d := stackgrant4Decks()
	return []stackgrant4Pair{{d[0], d[1]}, {d[2], d[3]}}
}

// TestStackGrantGatesTwoSeatLockstep plays the fixture's two-seat matchups:
// the gated/Effect pair and the cast-from-hand/Effect-grant pair, both with
// a cascade election reached.
func TestStackGrantGatesTwoSeatLockstep(t *testing.T) {
	reg := synthReg(t)
	set, err := cardset.RawByName("stackgrant4")
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, pair := range stackgrant4Pairs() {
		tbl, _, _, err := set.Matchup(pair.a, pair.b)
		if err != nil {
			t.Fatal(err)
		}
		for _, seed := range []uint64{1, 2, 3, 5} {
			r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"},
				Cards: [2][]string{stackgrant4Expand(pair.a), stackgrant4Expand(pair.b)},
				Seed:  seed, PassElision: true, Surface: "full"}
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			rc := r.RunConfig()
			rc.MaxDecisions, rc.MaxTurns = 4000, 40
			k, err := SetSide(set, tbl, rc.Config)(m, seed)
			if err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
			n := 0
			probe := &stackgrantCascadeProbe{base: castFirstPolicy(seed), rs: k.(*realSide), n: &n}
			res := Lockstep(m, seed, k, probe, rc)
			CloseSide(k)
			if res.Div != nil {
				lo := max(res.Div.At-12, 0)
				t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v",
					seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
					res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced,
					actionStrings(res.Actions[lo:res.Div.At]))
			}
			if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
			}
			total += n
		}
	}
	if total == 0 {
		t.Fatal("no cascade election in any two-seat game: the grants never fired")
	}
}

// TestStackGrantGateMutation is the mutation test: reading every gate as true
// lets a gated grant reach a cast its condition excludes, so one game of the
// pod must diverge.
func TestStackGrantGateMutation(t *testing.T) {
	p := newStackgrant4Pod(t)
	dirac.DebugStackGrantIgnoreGate = true
	defer func() { dirac.DebugStackGrantIgnoreGate = false }()
	res, _ := p.play4(t, 1, 40)
	if res.Div == nil {
		t.Fatal("ignoring every stack-grant gate diverged on no decision")
	}
	t.Logf("mutation diverged at %d: %s %s", res.Div.At, res.Div.What, res.Div.Field)
}

// TestStackGrantGatesPinnedRepro pins one focused matchup per grant family so
// a green lockstep is not a game where nothing happened: a gated cascade
// (Maelstrom Nexus), an Effect-issued cascade (Dark Apostle) and a
// cast-from-hand cascade (Wildsear, Scouring Maw).
func TestStackGrantGatesPinnedRepro(t *testing.T) {
	reg := synthReg(t)
	set, err := cardset.RawByName("stackgrant4")
	if err != nil {
		t.Fatal(err)
	}
	focused := []struct {
		name string
		a, b []cardset.Entry
	}{
		{"gated", []cardset.Entry{
			{Name: "Maelstrom Nexus", Count: 4}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Lightning Bolt", Count: 4}, {Name: "Opt", Count: 4}, {Name: "Divination", Count: 4},
			{Name: "Forest", Count: 4}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 4},
			{Name: "Plains", Count: 4}, {Name: "Swamp", Count: 4},
		}, []cardset.Entry{
			{Name: "Grizzly Bears", Count: 8}, {Name: "Gray Ogre", Count: 8},
			{Name: "Lightning Bolt", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8},
		}},
		{"effect", []cardset.Entry{
			{Name: "Dark Apostle", Count: 4}, {Name: "Lightning Bolt", Count: 4},
			{Name: "Shock", Count: 4}, {Name: "Divination", Count: 4}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Mountain", Count: 12}, {Name: "Island", Count: 8},
		}, []cardset.Entry{
			{Name: "Grizzly Bears", Count: 8}, {Name: "Gray Ogre", Count: 8},
			{Name: "Lightning Bolt", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8},
		}},
		{"cast-from-hand", []cardset.Entry{
			{Name: "Wildsear, Scouring Maw", Count: 4}, {Name: "Seal of Primordium", Count: 4},
			{Name: "Journey to Nowhere", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 4},
			{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 6}, {Name: "Plains", Count: 4},
		}, []cardset.Entry{
			{Name: "Grizzly Bears", Count: 8}, {Name: "Gray Ogre", Count: 8},
			{Name: "Lightning Bolt", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8},
		}},
	}
	for _, f := range focused {
		t.Run(f.name, func(t *testing.T) {
			tbl, _, _, err := set.Matchup(f.a, f.b)
			if err != nil {
				t.Fatal(err)
			}
			n := 0
			for _, seed := range []uint64{1, 2, 3, 4, 5} {
				r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"},
					Cards: [2][]string{stackgrant4Expand(f.a), stackgrant4Expand(f.b)},
					Seed:  seed, PassElision: true, Surface: "full"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				rc := r.RunConfig()
				rc.MaxDecisions, rc.MaxTurns = 4000, 40
				k, err := SetSide(set, tbl, rc.Config)(m, seed)
				if err != nil {
					t.Fatalf("seed %d: %v", seed, err)
				}
				probe := &stackgrantCascadeProbe{base: castFirstPolicy(seed), rs: k.(*realSide), n: &n}
				res := Lockstep(m, seed, k, probe, rc)
				CloseSide(k)
				if res.Div != nil {
					lo := max(res.Div.At-12, 0)
					t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %q\nker %q\n%s\nlast actions %v",
						seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff,
						res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail,
						actionStrings(res.Actions[lo:res.Div.At]))
				}
			}
			if n == 0 {
				t.Fatalf("%s grant: no cascade election over the seeds", f.name)
			}
			t.Logf("%s grant: %d cascade election(s)", f.name, n)
		})
	}
}

// stackgrantPairPolicy builds the policy for one focused pair; cascade
// counts the free-cast elections that prove a granted cascade fired.
type stackgrantPairPolicy func(seed uint64, rs *realSide, cascade *int) Policy

// playStackgrantPair plays one focused two-seat pair over seeds, counting the
// cascade free-cast elections, and fails on a divergence or an unmodelled
// stall. A nil mk uses castFirstPolicy with the cascade probe.
func playStackgrantPair(t *testing.T, set cardset.Set, reg *cards.Registry, a, b []cardset.Entry, seeds []uint64, mk stackgrantPairPolicy) (cascade int) {
	t.Helper()
	tbl, _, _, err := set.Matchup(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if mk == nil {
		mk = func(seed uint64, rs *realSide, cascade *int) Policy {
			return &stackgrantCascadeProbe{base: castFirstPolicy(seed), rs: rs, n: cascade}
		}
	}
	for _, seed := range seeds {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"},
			Cards: [2][]string{stackgrant4Expand(a), stackgrant4Expand(b)},
			Seed:  seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 4000, 40
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		probe := mk(seed, k.(*realSide), &cascade)
		res := Lockstep(m, seed, k, probe, rc)
		CloseSide(k)
		if res.Div != nil {
			lo := max(res.Div.At-12, 0)
			t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %q\nker %q\n%s\nlast actions %v",
				seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff,
				res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail,
				actionStrings(res.Actions[lo:res.Div.At]))
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
	}
	return cascade
}

// playStackgrantOne plays one focused two-seat seed and returns its result.
func playStackgrantOne(t *testing.T, set cardset.Set, reg *cards.Registry, a, b []cardset.Entry, seed uint64) Result {
	t.Helper()
	tbl, _, _, err := set.Matchup(a, b)
	if err != nil {
		t.Fatal(err)
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"},
		Cards: [2][]string{stackgrant4Expand(a), stackgrant4Expand(b)},
		Seed:  seed, PassElision: true, Surface: "full"}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 4000, 40
	k, err := SetSide(set, tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	n := 0
	probe := &stackgrantCascadeProbe{base: castFirstPolicy(seed), rs: k.(*realSide), n: &n}
	res := Lockstep(m, seed, k, probe, rc)
	CloseSide(k)
	return res
}

// TestStackGrantGatedCasualtyLockstep plays the gated Casualty grants
// (Anhelo's instant/sorcery and Ashad's nonlegendary artifact) in a plain
// two-seat game. The gate is a cast-time count; the kgen form test pins its
// lowering and this lockstep pins that the offered casualty lists agree.
func TestStackGrantGatedCasualtyLockstep(t *testing.T) {
	reg := synthReg(t)
	set, err := cardset.RawByName("stackgrant4")
	if err != nil {
		t.Fatal(err)
	}
	vanilla := []cardset.Entry{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Gray Ogre", Count: 8},
		{Name: "Lightning Bolt", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8},
	}
	pairs := []struct {
		name string
		a    []cardset.Entry
	}{
		{"anhelo", []cardset.Entry{
			{Name: "Anhelo, the Painter", Count: 4}, {Name: "Lightning Bolt", Count: 6},
			{Name: "Shock", Count: 4}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Mountain", Count: 6}, {Name: "Island", Count: 6}, {Name: "Swamp", Count: 6},
		}},
		{"ashad", []cardset.Entry{
			{Name: "Ashad, the Lone Cyberman", Count: 4}, {Name: "Ornithopter", Count: 6},
			{Name: "Memnite", Count: 6}, {Name: "Mind Stone", Count: 4}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Island", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Swamp", Count: 4},
		}},
	}
	seeds := []uint64{1, 2, 3, 4, 5, 6, 7, 8}
	for _, f := range pairs {
		t.Run(f.name, func(t *testing.T) {
			for _, seed := range seeds {
				res := playStackgrantOne(t, set, reg, f.a, vanilla, seed)
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %q\nker %q\n%s",
						seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff,
						res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail)
				}
			}
			t.Logf("%d seed(s) green", len(seeds))
		})
	}
}

// TestStackGrantExileCastLockstep plays Wild-Magic Sorcerer beside Act on
// Impulse: the exile cast's own push is hidden from the cast ring by the
// cascade trigger walk (hideInflightCast), so the CastExile origin must come
// from the stack entry for "the first spell you cast from exile each turn"
// to reach the very cast it is for. A cascade election proves the grant
// fired.
func TestStackGrantExileCastLockstep(t *testing.T) {
	reg := synthReg(t)
	set, err := cardset.RawByName("stackgrant4")
	if err != nil {
		t.Fatal(err)
	}
	wild := []cardset.Entry{
		{Name: "Wild-Magic Sorcerer", Count: 4}, {Name: "Act on Impulse", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Opt", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Ornithopter", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 4},
	}
	vanilla := []cardset.Entry{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Gray Ogre", Count: 8},
		{Name: "Lightning Bolt", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8},
	}
	fromExile, wildCast, granted := 0, 0, 0
	dirac.DebugGrantedCascade = func(n int) { granted += n }
	defer func() { dirac.DebugGrantedCascade = nil }()
	mk := func(seed uint64, rs *realSide, cascade *int) Policy {
		base := castFirstPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.D.Kind == DPriority && c.D.Player == 0 && len(c.G.E.G.Stack) == 0 {
				// put the grant's source down first
				for _, a := range c.D.Options {
					if a.Kind() == ActCast && refCardName(c.G, a.Src()) == "Wild-Magic Sorcerer" {
						wildCast++
						return a
					}
				}
				// then spend an Act on Impulse's exiled cards, preferring a
				// spell with mana value >= 1 so cascade can find the 0-cost
				// fodder; only once the grant's source is on the battlefield,
				// so the first exile cast of the turn is the one the grant is
				// for
				if !hasBattlefieldCard(c.G, "Wild-Magic Sorcerer") {
					return base.Pick(c)
				}
				exiled := func(skipZero bool) (Canon, bool) {
					for _, a := range c.D.Options {
						if a.Kind() != ActCast || a.Src() >= RefStack {
							continue
						}
						if o := c.G.E.G.Obj(c.G.id.obj[a.Src()]); o != nil && o.Zone == state.ZExile {
							if skipZero && refCardName(c.G, a.Src()) == "Ornithopter" {
								continue
							}
							fromExile++
							return a, true
						}
					}
					return 0, false
				}
				if a, ok := exiled(true); ok {
					return a
				}
				if a, ok := exiled(false); ok {
					return a
				}
				// then make more
				for _, a := range c.D.Options {
					if a.Kind() == ActCast && refCardName(c.G, a.Src()) == "Act on Impulse" {
						return a
					}
				}
			}
			return base.Pick(c)
		})
	}
	cascade := playStackgrantPair(t, set, reg, wild, vanilla, []uint64{1, 2, 3, 4, 5, 6, 7, 8}, mk)
	if fromExile == 0 {
		t.Fatal("no cast from exile over the seeds: the scenario never happened")
	}
	if granted == 0 {
		t.Fatalf("no cascade granted to a cast from exile (%d cast(s), Wild-Magic cast %d time(s))", fromExile, wildCast)
	}
	t.Logf("%d cast(s) from exile, Wild-Magic cast %d time(s), %d granted cascade trigger(s), %d cascade election(s)", fromExile, wildCast, granted, cascade)
}

// hasBattlefieldCard reports whether g controls a permanent whose front face
// is named name.
func hasBattlefieldCard(g *GorgeSide, name string) bool {
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		o := g.E.G.Obj(id)
		if o != nil && o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0].Name == name {
			return true
		}
	}
	return false
}
