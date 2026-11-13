package oracle

import (
	"math/rand/v2"
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMultikickerClassLockstep plays every lowered Multikicker card's own
// deck against gorge, decision by decision. The class lockstep (CR 702.43)
// covers the distinct resolutions: enters-with-counters (Everflowing
// Chalice), an ETB token (Wolfbriar Elemental), an ETB up-to-X target
// (Batroc the Leaper, Marshal's Anthem), an ETB life gain (Lightkeeper of
// Emeria), an instant's draw (Spell Contortion) and a kicked creature's
// counters (Apex Hawks, Enclave Elite, ...). The policy takes the multikicked
// cast whenever it is offered or lowered, answers a positive count, and
// otherwise casts a plain spell so the instant's counter window opens.
//
// Spell Contortion's deck adds Dark Ritual: its target is the Ritual still on
// the stack, and the seat holds six lands before casting it so the kicked
// {2}{U} + {1}{U} is payable.
func TestMultikickerClassLockstep(t *testing.T) {
	t.Parallel()
	seeds := uint64(32)
	if v := os.Getenv("KMULTIKICKER_SEEDS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			seeds = n
		}
	}
	class := []struct {
		name  string
		extra []cardset.Entry
		hold  bool // hold plain casts until six lands and two Islands
	}{
		{name: "Apex Hawks"},
		{name: "Batroc the Leaper"},
		{name: "Bloodhusk Ritualist"},
		{name: "Deathforge Shaman"},
		{name: "Enclave Elite"},
		{name: "Everflowing Chalice"},
		{name: "Flametongue Yearling"},
		{name: "Gnarlid Pack"},
		{name: "Joraga Warcaller"},
		{name: "Lightkeeper of Emeria"},
		{name: "Marshal's Anthem"},
		{name: "Quag Vampires"},
		{name: "Skitter of Lizards"},
		{
			name: "Spell Contortion",
			extra: []cardset.Entry{
				{Name: "Dark Ritual", Count: 8},
				{Name: "Island", Count: 14},
				{Name: "Swamp", Count: 14},
			},
			hold: true,
		},
		{name: "Voyager Drake"},
		{name: "Wolfbriar Elemental"},
	}
	for _, tc := range class {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			decks := [2][]cardset.Entry{mkClassDeck(tc.name, tc.extra), mkClassDeck(tc.name, tc.extra)}
			tl := &mkClassTally{casts: map[string]int{}, counts: map[int]int{}}
			runMultikicker(t, decks, seeds, func(seed uint64) Policy {
				return mkClassPolicy(seed, tc.name, tc.hold, tl)
			})
			t.Logf("multikicked casts %v; counts %v", tl.casts, tl.counts)
			if tl.casts[tc.name] == 0 {
				t.Errorf("no multikicked cast of %s", tc.name)
			}
			positive := false
			for n, c := range tl.counts {
				if n > 0 && c > 0 {
					positive = true
				}
			}
			if !positive {
				t.Errorf("no positive multikicker count was answered")
			}
		})
	}
}

// mkClassDeck is 4 copies of the card and 36 basics (the five colours, so
// every class cost is castable), or the explicit Spell Contortion deck.
func mkClassDeck(name string, extra []cardset.Entry) []cardset.Entry {
	if len(extra) != 0 {
		return append([]cardset.Entry{{Name: name, Count: 4}}, extra...)
	}
	return []cardset.Entry{
		{Name: name, Count: 4},
		{Name: "Plains", Count: 7}, {Name: "Island", Count: 7}, {Name: "Swamp", Count: 7},
		{Name: "Mountain", Count: 7}, {Name: "Forest", Count: 8},
	}
}

// mkClassTally counts the class's multikicked casts and count answers.
type mkClassTally struct {
	casts  map[string]int
	counts map[int]int
}

// mkClassPolicy takes a multikicked cast (offered, or a potential play the
// oracle lowers) whenever one is on the decision, answers a positive count
// two times in three, and otherwise casts a plain spell. A plain cast is
// only held (holdPlain) until the seat has six lands and two Islands, so the
// Spell Contortion deck can answer its own Dark Ritual with the kicked
// instant before the Ritual resolves.
func mkClassPolicy(seed uint64, name string, holdPlain bool, tl *mkClassTally) Policy {
	rnd := RandomPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x6d6b6361))
	kick, _ := castModeCode("kicked")
	var opts []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DAttackers || c.D.Kind == DBlockers {
			for _, a := range c.D.Options {
				if a.Kind() == ActDone {
					return a
				}
			}
			return rnd.Pick(c)
		}
		if code, ok := chooseCode("multikick"); ok {
			opts = opts[:0]
			for _, a := range c.D.Options {
				if a.Kind() == ActChoose+CanonKind(code) {
					opts = append(opts, a)
				}
			}
			if len(opts) > 0 {
				if len(opts) > 1 && r.IntN(3) != 0 {
					a := opts[1+r.IntN(len(opts)-1)]
					tl.counts[int(a.Mode())]++
					return a
				}
				tl.counts[0]++
				return opts[0]
			}
		}
		opts = opts[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCast && a.Mode()&0x0f == kick {
				opts = append(opts, a)
			}
		}
		if len(opts) > 0 && r.IntN(3) != 0 {
			tl.casts[name]++
			return opts[r.IntN(len(opts))]
		}
		if holdPlain && !mkSixLands(c) {
			return rnd.Pick(c)
		}
		for _, a := range c.D.Options {
			if a.Kind() == ActCast && a.Mode() == 0 {
				return a
			}
		}
		return rnd.Pick(c)
	})
}

// mkSixLands reports whether the acting seat controls six lands and two
// Islands, the Spell Contortion deck's cast-after-Dark-Ritual condition.
func mkSixLands(c *PickCtx) bool {
	if c.G == nil {
		return false
	}
	lands, islands := 0, 0
	for _, id := range c.G.E.G.Zone(state.ZBattlefield, state.PlayerID(c.D.Player)) {
		o := c.G.E.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		lands++
		if o.Face().Name == "Island" {
			islands++
		}
	}
	return lands >= 6 && islands >= 2
}
