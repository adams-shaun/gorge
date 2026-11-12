package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
)

// The lockstep: target slots whose TargetMin$/TargetMax$ is a
// value, read as the slot is announced (gorge resolvedTargetBounds).
//
//   - Rabid Attack: TargetMax$ X, X the creatures you control (shroud
//     creatures leave fewer legal targets than the count);
//   - Coveted Falcon: the turn-face-up donation, X the permanents you control;
//   - Prismabasher: a self-ETB trigger, X the colors among your permanents;
//   - Divergent Equation: TargetMax$ X, X paid (X = 0 asks nothing);
//   - Heroic Feast: TargetMax$ X, X the life gained (the trigger amount);
//   - Twincast copies the instants, re-resolving their bounds for the copy.
//
// Coveted Falcon's donation (a turn-face-up trigger) is reached by
// TestTgtmaxvarFalconLockstep: gorge offers a face-down cast and a turn-up
// only when the floating pool pays them, so its deck casts Iridescent Tiger
// off Mox Ruby, Mox Sapphire and Islands (WUBRG floats), casts the Falcon
// face down ({3}) and turns it up ({1}{U}). The main pool has no such float.
//
// Refused, not lowered: Ureni, the Song Unending (its X damage is divided
// over a count: "divided over a value total") and Hide on the Ceiling (X..X
// targets returned at the end step: the delayed record holds one object).
var ktgtmaxvarDecks = [2][]cardset.Entry{
	{
		{Name: "Rabid Attack", Count: 4}, {Name: "Heroic Feast", Count: 3}, {Name: "Prismabasher", Count: 2},
		{Name: "Bogwater Lumaret", Count: 3}, {Name: "Vampire Nighthawk", Count: 3}, {Name: "Elvish Lookout", Count: 3},
		{Name: "Humble Budoka", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Forest", Count: 9}, {Name: "Swamp", Count: 7},
	},
	{
		{Name: "Divergent Equation", Count: 4}, {Name: "Coveted Falcon", Count: 4},
		{Name: "Unsummon", Count: 4}, {Name: "Divination", Count: 3}, {Name: "Twincast", Count: 2},
		{Name: "Wind Drake", Count: 4}, {Name: "Merfolk of the Pearl Trident", Count: 3}, {Name: "Island", Count: 16},
	},
}

// ktvTally counts, from gorge's event log and pending target asks, what the
// pool reached.
type ktvTally struct {
	evAt  int
	seq   uint64
	stack map[uint32]string // stack object -> the spell's name, for its CastInfo
	// spell resolutions by name, and triggers put on the stack by source name
	resolved map[string]int
	trigs    map[string]int
	// Divergent Equation casts with X = 0; Rabid Attack asks whose maximum
	// exceeds the legal targets
	deX0, raFewer int
	copies        int
}

func (p *ktvTally) reset() {
	p.evAt, p.seq = 0, 0
	p.stack = map[uint32]string{}
}

func (p *ktvTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.PutOnStack:
			p.stack[uint32(e.Obj)] = chName(g, e.Obj)
		case events.CastInfo:
			switch p.stack[uint32(e.Obj)] {
			case "Divergent Equation":
				delete(p.stack, uint32(e.Obj)) // a cast with X = 0 has no CastInfo
			}
		case events.Resolve:
			if e.Text != "" {
				p.resolved[e.Text]++
			}
			if e.Text == "Divergent Equation" && p.stack[uint32(e.Obj)] == e.Text {
				p.deX0++
			}
			delete(p.stack, uint32(e.Obj))
		case events.TriggerPush:
			p.trigs[chName(g, e.Obj)+"#"+strconv.Itoa(int(e.Amount))]++
		case events.StackCopy:
			p.copies++
		}
	}
	p.evAt = len(evs)
	if d := ref.Pending(); d != nil && d.Kind == decision.KTarget && uint64(d.Seq) != p.seq {
		p.seq = uint64(d.Seq)
		if chName(g, d.Source) == "Rabid Attack" && d.Max > len(d.Options) {
			p.raFewer++
		}
	}
	return nil
}

func ktvSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("tgtmaxvar")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range ktgtmaxvarDecks {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	return reg, set, decks
}

// ktvRun plays seeds 1..seeds; stopAtDiv ends it at the first divergence.
func ktvRun(t *testing.T, seeds uint64, stopAtDiv bool) (tally *ktvTally, games, divs, stalls int, first string) {
	reg, set, decks := ktvSetup(t)
	tbl, _, _, err := set.Matchup(ktgtmaxvarDecks[0], ktgtmaxvarDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	tally = &ktvTally{resolved: map[string]int{}, trigs: map[string]int{}}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		m, err := NewMatchup(reg, [2]string{"ktv-a", "ktv-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		tally.reset()
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: tally.check})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			if first == "" {
				first = "seed " + strconv.FormatUint(seed, 10) + ": " + d.What + " " + d.Field
			}
			lo := max(d.At-60, 0)
			t.Logf("seed %d %s %s at %d last actions %v\ndiff %v %s\nref %v\nker %v", seed, d.What, d.Field, d.At, actionStrings(res.Actions[max(lo, d.At-8):d.At]), d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			if stopAtDiv {
				return
			}
		}
	}
	return
}

func TestTgtmaxvarLockstep(t *testing.T) {
	t.Parallel()
	tally, games, divs, stalls, first := ktvRun(t, 400, false)
	t.Logf("%d games, %d divergences, %d stalls; spell resolutions %v; triggers (source#index) %v; Divergent Equation casts with X=0: %d; Rabid Attack asks with fewer legal targets than the count: %d; copies %d",
		games, divs, stalls, tally.resolved, tally.trigs, tally.deX0, tally.raFewer, tally.copies)
	if divs != 0 {
		t.Fatalf("%d divergences, first %s", divs, first)
	}
	if stalls != 0 {
		t.Fatalf("%d stalls", stalls)
	}
	for _, c := range []string{"Rabid Attack", "Divergent Equation"} {
		if tally.resolved[c] < 5 {
			t.Errorf("%s resolutions %d, want at least 5", c, tally.resolved[c])
		}
	}
	for _, c := range []string{"Prismabasher#0", "Heroic Feast#1"} {
		if tally.trigs[c] < 5 {
			t.Errorf("%s triggers %d, want at least 5", c, tally.trigs[c])
		}
	}
	if tally.deX0 < 3 {
		t.Errorf("Divergent Equation casts with X = 0: %d, want at least 3", tally.deX0)
	}
	if tally.raFewer < 3 {
		t.Errorf("Rabid Attack asks with fewer legal targets than the count: %d, want at least 3", tally.raFewer)
	}
}

// TestTgtmaxvarMutation: with the evaluated maximum one too many the slot
// that takes every candidate asks a done-only decision gorge does not, so
// the lockstep must diverge within 128 seeds.
func TestTgtmaxvarMutation(t *testing.T) {
	dirac.DebugTgtBoundsMaxPlus1 = true
	defer func() { dirac.DebugTgtBoundsMaxPlus1 = false }()
	_, games, divs, _, first := ktvRun(t, 128, true)
	if divs == 0 {
		t.Fatalf("no divergence in %d games with DebugTgtBoundsMaxPlus1", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, first)
	if r := ktvFalconRun(t, 128, true); r.divs == 0 {
		t.Fatalf("no Falcon divergence in %d games with DebugTgtBoundsMaxPlus1", r.games)
	}
}

var ktvFalconDeck = []cardset.Entry{
	{Name: "Coveted Falcon", Count: 4}, {Name: "Iridescent Tiger", Count: 4},
	{Name: "Mox Ruby", Count: 4}, {Name: "Mox Sapphire", Count: 4},
	{Name: "Island", Count: 12}, {Name: "Mountain", Count: 12},
}

// ktvFalconPolicy walks the float line: lands and Moxen, the Tiger, the
// Falcon face down, then its turn-up; anything else (targets, the donation's
// answers) is random.
func ktvFalconPolicy(seed uint64, downs, ups *int) Policy {
	rnd := RandomPolicy(seed)
	turnUp := kfacedownTurnUp
	streak := 0
	takeAll := false
	var tbuf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		// A multi-object slot holds eight objects (maxXTgt): a policy that
		// picks "any number" at random would overflow it, loudly (StallXTgt).
		// A run that fits takes every candidate (where a bound one too high
		// shows as a done-only ask); a larger one finishes after three picks.
		var haveDone, haveTgt bool
		var done Canon
		nt := 0
		tbuf = tbuf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActDone:
				done, haveDone = a, true
			case ActTarget:
				haveTgt = true
				nt++
				tbuf = append(tbuf, a)
			}
		}
		if haveDone && haveTgt {
			if streak == 0 {
				takeAll = nt <= 8
			}
			if !takeAll && streak >= 3 {
				streak = 0
				return done
			}
			streak++
			d := *c.D
			d.Options = tbuf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		streak = 0
		var land, mox, tiger, down, up Canon
		var haveLand, haveMox, haveTiger, haveDown, haveUp bool
		for _, a := range c.D.Options {
			n := optionFaceName(c, a)
			switch {
			case a.Kind() == ActPlayLand:
				land, haveLand = a, true
			case a.Kind() == ActCast && strings.HasPrefix(n, "Mox "):
				mox, haveMox = a, true
			case a.Kind() == ActCast && n == "Iridescent Tiger":
				tiger, haveTiger = a, true
			case a.Kind() == ActCastFace && n == "Coveted Falcon":
				down, haveDown = a, true
			case a.Kind() == ActSpecial && a.Mode() == turnUp:
				up, haveUp = a, true
			}
		}
		switch {
		case haveUp:
			*ups++
			return up
		case haveDown:
			*downs++
			return down
		case haveTiger:
			return tiger
		case haveMox:
			return mox
		case haveLand:
			return land
		}
		return rnd.Pick(c)
	})
}

// ktvFalconTally is what a Falcon run reached.
type ktvFalconTally struct {
	games, divs, stalls, downs, ups, trig, asks int
}

// ktvFalconRun plays seeds 1..seeds of the Falcon deck; stopAtDiv ends it at
// the first divergence.
func ktvFalconRun(t *testing.T, seeds uint64, stopAtDiv bool) (r ktvFalconTally) {
	reg, set, _ := ktvSetup(t)
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range ktvFalconDeck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(ktvFalconDeck, ktvFalconDeck)
	if err != nil {
		t.Fatal(err)
	}
	var turnUpTrig = func(card dirac.CardID, ab uint8) bool {
		if int(card) >= len(tbl.Cards) {
			return false
		}
		abs := tbl.Cards[card].Abilities
		return int(ab) < len(abs) && abs[ab].Trig.Event == kop.TrigTurnFaceUp
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ktv-f", "ktv-g"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var d0, u0 int
		seen := map[[2]uint32]bool{}
		var lastSeq uint64
		check := func(ref *GorgeSide, side Side) *Divergence {
			if ks, ok := side.(*realSide); ok {
				s := &ks.s
				for i := 0; i < s.NumPending(); i++ {
					src, card, ab, _, _ := s.PendingTrigger(i)
					if turnUpTrig(card, ab) {
						seen[[2]uint32{uint32(src), uint32(ab)}] = true
					}
				}
				for i := 0; i < s.StackLen(); i++ {
					kind, src, card, _ := s.StackEntry(i)
					if kind == 2 {
						if ab := s.StackAbility(i); turnUpTrig(card, ab) {
							seen[[2]uint32{uint32(src), uint32(ab)}] = true
						}
					}
				}
			}
			if d := ref.Pending(); d != nil && d.Kind == decision.KTarget && uint64(d.Seq) != lastSeq {
				lastSeq = uint64(d.Seq)
				// the donation slot: the only "any number" ask in this deck (a
				// trigger's source object has no printed name to match)
				if d.Min == 0 && d.Max > 1 {
					r.asks++
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, ktvFalconPolicy(seed, &d0, &u0), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
		CloseSide(k)
		r.games++
		r.downs += d0
		r.ups += u0
		r.trig += len(seen)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			r.stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if dv := res.Div; dv != nil {
			r.divs++
			lo := max(dv.At-60, 0)
			t.Logf("seed %d %s %s at %d last actions %v\ndiff %v %s\nref %v\nker %v", seed, dv.What, dv.Field, dv.At, actionStrings(res.Actions[max(lo, dv.At-8):dv.At]), dv.Diff, dv.Detail, dv.RefOptions, dv.KerOptions)
			if stopAtDiv {
				return
			}
		}
	}
	return
}

// TestTgtmaxvarFalconLockstep: Coveted Falcon's turn-face-up donation
// ("any number of target permanents you control", TargetMax$ X over the
// permanents you control), a SubAbility$ slot behind a player-target root.
func TestTgtmaxvarFalconLockstep(t *testing.T) {
	t.Parallel()
	r := ktvFalconRun(t, 200, false)
	t.Logf("%d games, %d divergences, %d stalls; Falcon face-down casts %d, turn-ups %d, donation triggers %d, donation slot asks (minimum 0, maximum above 1) %d",
		r.games, r.divs, r.stalls, r.downs, r.ups, r.trig, r.asks)
	if r.divs != 0 || r.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", r.divs, r.stalls)
	}
	if r.trig < 3 {
		t.Errorf("Coveted Falcon turn-face-up donations: %d, want at least 3", r.trig)
	}
	if r.asks < 3 {
		t.Errorf("donation slot asks: %d, want at least 3", r.asks)
	}
}
