package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The turnuprepl lockstep: R:Event$ TurnFaceUp
// replacements. Bubble Smuggler and Crowd-Control Warden put counters on
// themselves as they are turned up; Karlov Watchdog (opponents' permanents
// can't be turned up during its controller's turn) and Unable to Scream
// (enchanted face-down creature can't be turned up) make the special action
// illegal. The Moxen and Priests float the mana a turn-up is paid from.

var kturnuprepDeck = []cardset.Entry{
	{Name: "Mox Pearl", Count: 2},
	{Name: "Mox Ruby", Count: 2},
	{Name: "Mox Emerald", Count: 1},
	{Name: "Mox Sapphire", Count: 1},
	{Name: "Iridescent Tiger", Count: 6},
	{Name: "Soul Summons", Count: 3},
	{Name: "Manifest Dread", Count: 3},
	{Name: "Bubble Smuggler", Count: 6},
	{Name: "Crowd-Control Warden", Count: 4},
	{Name: "Memnite", Count: 4},
	{Name: "Karlov Watchdog", Count: 4},
	{Name: "Unable to Scream", Count: 2},

	{Name: "Bolrac-Clan Basher", Count: 2},
}

func kturnuprepEnv(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("turnuprepl")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range kturnuprepDeck {
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
	tbl, _, _, err := set.Matchup(kturnuprepDeck, kturnuprepDeck)
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}

// kturnuprepPolicy drives the setup: the Moxen first, a turn-up whenever the
// floating pool pays for one (Bubble Smuggler first), the manifesters (a
// manifested creature card turns up for its printed mana cost, here {1}{U}),
// Iridescent Tiger's WUBRG once a face-down permanent is waiting to turn up,
// then face-down casts; the rest is random among the casts, activations and
// special actions. A manifested Memnite ({0}) turns up for free with no
// floating mana, so it is the face-down permanent a Karlov Watchdog holds
// on its controller's turn; its turn-up is left to the random pick. Crowd-
// Control Warden's hybrid Disguise cost ({3}{G/W}{G/W}) is paid from the
// pool by Iridescent Tiger's WUBRG.
func kturnuprepPolicy(seed uint64, up map[string]int) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	waiting := 0       // face-down permanents not yet turned up (both seats)
	wardenWaiting := 0 // face-down Crowd-Control Wardens not yet turned up
	return PolicyFunc(func(c *PickCtx) Canon {
		var (
			mox, summon, tiger, down, turn Canon
			haveMox, haveSummon, haveTiger bool
			haveDown, haveTurn             bool
			smuggler, warden               Canon
			haveSmuggler, haveWarden       bool
			watchdog                       Canon
			haveWatchdog                   bool
		)
		buf = buf[:0]
		for _, a := range c.D.Options {
			n := optionFaceName(c, a)
			switch {
			case a.Kind() == ActSpecial && a.Mode()&15 == kfacedownTurnUp:
				if n == "Bubble Smuggler" {
					smuggler, haveSmuggler = a, true
				}
				if n == "Crowd-Control Warden" {
					warden, haveWarden = a, true
				}
				if n != "Memnite" { // the free turn-up is a rare pick, so it is still face down on a Watchdog turn
					turn, haveTurn = a, true
				}
			case a.Kind() == ActCast && kfacedownMox(n):
				mox, haveMox = a, true
			case a.Kind() == ActCast && (n == "Soul Summons" || n == "Manifest Dread"):
				summon, haveSummon = a, true
			case a.Kind() == ActCast && n == "Karlov Watchdog":
				watchdog, haveWatchdog = a, true
			case a.Kind() == ActCast && n == "Iridescent Tiger":
				tiger, haveTiger = a, true
			case a.Kind() == ActCastFace && n != "Bubble Smuggler":
				down, haveDown = a, true
			}
			switch a.Kind() {
			case ActCastFace, ActCast, ActActivate, ActSpecial:
				if n == "Memnite" && a.Kind() == ActSpecial && c.Seq%40 != 0 {
					continue
				}
				buf = append(buf, a)
			}
		}
		pick := func(a Canon) Canon {
			n := optionFaceName(c, a)
			switch {
			case a.Kind() == ActCastFace:
				waiting++
				if n == "Crowd-Control Warden" {
					wardenWaiting++
				}
				up["down "+n]++
			case a.Kind() == ActCast && (n == "Soul Summons" || n == "Manifest Dread"):
				waiting++
				up["manifest "+n]++
			case a.Kind() == ActCast && (n == "Karlov Watchdog" || n == "Unable to Scream"):
				up["cast "+n]++
			case a.Kind() == ActSpecial && a.Mode()&15 == kfacedownTurnUp:
				up[n]++
				if waiting > 0 {
					waiting--
				}
				if n == "Crowd-Control Warden" && wardenWaiting > 0 {
					wardenWaiting--
				}
			}
			return a
		}
		switch {
		case haveSmuggler:
			return pick(smuggler)
		case haveWarden:
			return pick(warden)
		case haveTurn:
			return pick(turn)
		case haveMox:
			return pick(mox)
		case haveWatchdog:
			return pick(watchdog)
		case haveTiger && wardenWaiting > 0:
			return pick(tiger)
		case haveSummon:
			return pick(summon)
		case haveTiger && waiting > 0:
			return pick(tiger)
		case haveDown:
			return pick(down)
		}
		if len(buf) == 0 { // only the held-back free turn-up and passes: keep the passes
			for _, a := range c.D.Options {
				if a.Kind() != ActSpecial {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			buf = append(buf, c.D.Options...)
		}
		d := *c.D
		d.Options = buf
		return pick(rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq}))
	})
}

type kturnuprepResult struct {
	seeds, divs, stalls int
	up                  map[string]int
}

func kturnuprepRun(t *testing.T, seeds uint64, firstDiv bool) kturnuprepResult {
	t.Helper()
	reg, set, decks, tbl := kturnuprepEnv(t)
	res := kturnuprepResult{up: map[string]int{}}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kt-a", "kt-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		r := Lockstep(m, seed, k, kturnuprepPolicy(seed, res.up), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		res.seeds++
		if strings.HasPrefix(r.Stall, KernelStallPrefix) {
			res.stalls++
		}
		if d := r.Div; d != nil {
			res.divs++
			if !firstDiv {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(r.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			} else {
				t.Logf("seed %d: diverges at decision %d (%s %s)", seed, d.At, d.What, d.Field)
				return res
			}
		}
	}
	return res
}

func TestTurnupreplLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugTurnUpReplStats = true
	dirac.TurnUpBlockedHeld, dirac.TurnUpHeldCond, dirac.TurnUpHeldPlain, dirac.TurnUpReplCtrs = 0, 0, 0, 0
	defer func() { dirac.DebugTurnUpReplStats = false }()
	seeds := uint64(256)
	if testing.Short() {
		seeds = 32
	}
	res := kturnuprepRun(t, seeds, false)
	t.Logf("seeds=%d divergences=%d stalls=%d turn-ups=%v program-runs=%d held=%d (watchdog %d, aura %d)",
		res.seeds, res.divs, res.stalls, res.up, dirac.TurnUpReplCtrs, dirac.TurnUpBlockedHeld, dirac.TurnUpHeldCond, dirac.TurnUpHeldPlain)
	if res.divs != 0 || res.stalls != 0 {
		t.Errorf("%d divergences, %d stalls", res.divs, res.stalls)
	}
	if testing.Short() {
		return
	}
	if dirac.TurnUpReplCtrs < 10 {
		t.Errorf("counter-bearing turn-ups: %d, want >= 10", dirac.TurnUpReplCtrs)
	}
	if dirac.TurnUpHeldCond < 5 {
		t.Errorf("Karlov Watchdog withheld offers: %d, want >= 5", dirac.TurnUpHeldCond)
	}
	if res.up["Crowd-Control Warden"] < 3 {
		t.Errorf("Crowd-Control Warden turn-ups: %d, want >= 3", res.up["Crowd-Control Warden"])
	}
	if dirac.TurnUpHeldPlain < 3 {
		t.Errorf("Unable to Scream withheld offers: %d, want >= 3", dirac.TurnUpHeldPlain)
	}
}

// TestTurnupreplMutation: with turnUpBlocked off (dirac.DebugTurnUpReplOff)
// the kernel offers a turn-up gorge refuses, and the lockstep diverges.
func TestTurnupreplMutation(t *testing.T) {
	dirac.DebugTurnUpReplOff = true
	defer func() { dirac.DebugTurnUpReplOff = false }()
	res := kturnuprepRun(t, 128, true)
	if res.divs == 0 {
		t.Fatal("offering the turn-ups gorge refuses diverged in no game: the lockstep does not see the can't-be-turned-up path")
	}
}
