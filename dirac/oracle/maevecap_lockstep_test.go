package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// An until-end-of-turn Effect remembering its chain's target over an Attacks
// line (kgen emblemCapture's Attacks arm: Maeve, Insidious Singer's "goad
// target creature. Whenever that creature attacks one of your opponents this
// turn, you draw a card"). The kernel's Attacks emblem hook hands the line only
// the captured attacker (dirac emblemAtkListeners), gorge's ValidCard$
// Creature.IsRemembered over the registration's Remembered set; Attacked$
// Opponent is read from the emblem owner's side. Seats 0 and 1 are the
// two-seat match; the four-seat pod seats two Maeve decks so a goaded
// opponent's creature can attack a third player.
func maeveCapDecks() [4][]cardset.Entry {
	maeve := []cardset.Entry{
		{Name: "Maeve, Insidious Singer", Count: 8}, {Name: "Grizzly Bears", Count: 10},
		{Name: "Island", Count: 12}, {Name: "Forest", Count: 10},
	}
	lions := []cardset.Entry{
		{Name: "Savannah Lions", Count: 14}, {Name: "Coral Merfolk", Count: 6},
		{Name: "Plains", Count: 12}, {Name: "Island", Count: 8},
	}
	return [4][]cardset.Entry{maeve, lions, maeve, lions}
}

const maeveCapSeeds = 40

func TestMaeveCaptureLockstep(t *testing.T) {
	t.Parallel()
	d := maeveCapDecks()
	lockstepRun(t, "maevecap", [2][]cardset.Entry{d[0], d[1]}, maeveCapSeeds, true, nil)
}

// maeveCapNoCapture drops every emblem's capture: the Attacks line then
// fires for every attacker of its controller's turn.
func maeveCapNoCapture(tbl *kop.Table) {
	ems := append([]kop.Emblem(nil), tbl.Emblems...)
	for i := range ems {
		ems[i].Capture = kop.RefNone
	}
	tbl.Emblems = ems
}

// TestMaeveCaptureLockstepMutation: with the capture dropped some seed
// diverges, so the games reach an emblem while an uncaptured creature attacks.
func TestMaeveCaptureLockstepMutation(t *testing.T) {
	t.Parallel()
	d := maeveCapDecks()
	if n := lockstepRun(t, "maevecap", [2][]cardset.Entry{d[0], d[1]}, maeveCapSeeds, false, maeveCapNoCapture); n == 0 {
		t.Fatal("no seed diverged with the Attacks emblems' capture dropped")
	}
}

// TestMaeveCaptureFireMutation: with the Attacks emblem hook silenced some
// seed diverges, so the captured creature does attack an opponent under it.
// Not parallel: it sets a package switch.
func TestMaeveCaptureFireMutation(t *testing.T) {
	dirac.DebugIgnoreAtkEmblem = true
	defer func() { dirac.DebugIgnoreAtkEmblem = false }()
	d := maeveCapDecks()
	if n := lockstepRun(t, "maevecap", [2][]cardset.Entry{d[0], d[1]}, maeveCapSeeds, false, nil); n == 0 {
		t.Fatal("no seed diverged with the Attacks emblem hook silenced")
	}
}

// maeveCapPod plays one four-seat seed; mutate, when set, edits a copy of
// the table.
func maeveCapPod(t *testing.T, seed uint64, mutate func(*kop.Table)) Result {
	t.Helper()
	decks := maeveCapDecks()
	set, err := cardset.RawByName("maevecap")
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, err := set.MatchupN(decks[:])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		cp := *tbl
		mutate(&cp)
		tbl = &cp
	}
	exp := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for i := 0; i < e.Count; i++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{exp(decks[0]), exp(decks[1])},
		MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{exp(decks[2]), exp(decks[3])}, Seed: seed, PassElision: true, Surface: "full"}
	m, err := r.Matchup(synthReg(t))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, 40
	k, err := SetSide(set, tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	return LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc)
}

const maeveCapPodSeeds = 16

// TestMaeveCaptureFourSeatLockstep: every decision of the four-seat pods
// matches gorge and the kernel never stalls.
func TestMaeveCaptureFourSeatLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	for seed := uint64(1); seed <= maeveCapPodSeeds; seed++ {
		res := maeveCapPod(t, seed, nil)
		if res.Div != nil {
			lo := max(res.Div.At-12, 0)
			t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
				res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
	}
}

// TestMaeveCaptureFourSeatMutation: the four-seat pods reach the capture too.
func TestMaeveCaptureFourSeatMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for seed := uint64(1); seed <= maeveCapPodSeeds; seed++ {
		if res := maeveCapPod(t, seed, maeveCapNoCapture); res.Div != nil {
			t.Logf("seed %d diverged at %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("no four-seat seed diverged with the Attacks emblems' capture dropped")
}
