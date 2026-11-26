package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Cipher (CR 702.99) lockstep: the resolving spell's encode ask (gorge
// effCipher's KModes over the caster's creatures, Done declining), the
// encoded card's combat-damage trigger under the creature's controller (gorge
// checkCipherTriggers and the "__kwCipher:" push) and its optional free copy
// cast, and the link ending when the creature leaves the battlefield. The
// kernel is dirac/kw_cipher.go; the expected answer at every decision is
// gorge's.
var cipherDecks = [2][]cardset.Entry{{
	{Name: "Call of the Nightwing", Count: 3}, {Name: "Last Thoughts", Count: 3}, {Name: "Hands of Binding", Count: 2},
	{Name: "Shadow Slice", Count: 2}, {Name: "Mental Vapors", Count: 2}, {Name: "Paranoid Delusions", Count: 2},
	{Name: "Midnight Recovery", Count: 1}, {Name: "Hidden Strings", Count: 1},
	{Name: "Merfolk of the Pearl Trident", Count: 4}, {Name: "Coral Merfolk", Count: 4},
	{Name: "Island", Count: 10}, {Name: "Swamp", Count: 6},
}, {
	{Name: "Undercity Plague", Count: 1}, {Name: "Voidwalk", Count: 2}, {Name: "Whispering Madness", Count: 1},
	{Name: "Writ of Return", Count: 2}, {Name: "Stolen Identity", Count: 1}, {Name: "Trait Doctoring", Count: 1},
	{Name: "Call of the Nightwing", Count: 3}, {Name: "Last Thoughts", Count: 3}, {Name: "Shadow Slice", Count: 2},
	{Name: "Merfolk of the Pearl Trident", Count: 4}, {Name: "Coral Merfolk", Count: 4},
	{Name: "Island", Count: 10}, {Name: "Swamp", Count: 6},
}}

// cipherTally counts gorge's side of the games.
type cipherTally struct {
	games, encoded, copies, broken, multi int
}

// cipherCheck scans gorge's events: an encode (the Imprint "encoded"), a copy
// cast (StackCopy "copy card for play"), an encoded creature leaving the
// battlefield (the link ends), and a creature carrying two encoded cards.
func cipherCheck(tal *cipherTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	var hosts map[state.ObjID]int
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen, hosts = ref, 0, map[state.ObjID]int{}
			tal.games++
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			switch {
			case ev.Kind == events.Imprint && ev.Text == "encoded":
				tal.encoded++
				hosts[ev.Obj]++
				if hosts[ev.Obj] == 2 {
					tal.multi++
				}
			case ev.Kind == events.StackCopy && ev.Text == "copy card for play":
				tal.copies++
			case ev.Kind == events.MoveZone && ev.From == state.ZBattlefield && hosts[ev.Obj] > 0:
				tal.broken++
				delete(hosts, ev.Obj)
			}
		}
		return nil
	}
}

// cipherRun plays seeds games of decks (two or four seats) lowered in process,
// returning the divergences and kernel stalls.
func cipherRun(t *testing.T, decks [][]cardset.Entry, seeds int, check func(*GorgeSide, Side) *Divergence, report bool) (divs, stalls int) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	var pool []*cards.Card
	seen := map[string]bool{}
	for _, d := range decks {
		for _, e := range d {
			if seen[e.Name] {
				continue
			}
			seen[e.Name] = true
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			pool = append(pool, c)
		}
	}
	pd, err := kgen.TableFromCards(reg, pool, "cipher-oracle")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	tbl, ids, err := set.MatchupN(decks)
	if err != nil {
		t.Fatal(err)
	}
	labels := []string{"cipher-a", "cipher-b", "cipher-c", "cipher-d"}[:len(decks)]
	var st podClassStats
	watch := func(c *PickCtx) bool { return c.G.fr.d != nil && c.G.fr.d.ResumeKind == "cipher" }
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		m, err := KernelMatchupN(reg, tbl, labels, ids, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{MaxDecisions: 4000, MaxTurns: 40, Check: check}
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, podClassPolicy(seed, watch, &st), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			divs++
			if report {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d: stalled: %s", seed, res.Stall)
		}
	}
	return divs, stalls
}

// TestCipherLockstep: two-seat Cipher games play in lockstep, covering the
// encode and its decline, the combat-damage trigger's copy cast accepted and
// declined, the link ending as the creature leaves, and two cards on one
// creature.
func TestCipherLockstep(t *testing.T) {
	t.Parallel()
	var tal cipherTally
	var ks dirac.CipherStats
	dirac.DebugCipherStats = &ks
	defer func() { dirac.DebugCipherStats = nil }()
	divs, stalls := cipherRun(t, cipherDecks[:], 40, cipherCheck(&tal), true)
	t.Logf("%d games, %d divergences, %d stalls: kernel asked %d encodes, encoded %d, queued %d triggers, resolved %d; gorge encoded %d, cast %d copies, broke %d links, %d creatures with two cards",
		tal.games, divs, stalls, ks.Asked, ks.Encoded, ks.Triggered, ks.Resolved, tal.encoded, tal.copies, tal.broken, tal.multi)
	if stalls != 0 {
		t.Errorf("%d games stalled", stalls)
	}
	if ks.Asked <= ks.Encoded || ks.Encoded < 10 || ks.Resolved <= tal.copies || tal.copies < 5 || tal.broken < 3 || tal.multi < 1 {
		t.Errorf("coverage: want declined encodes, >= 10 encodes, declined and >= 5 accepted copy casts, >= 3 broken links, a creature with two cards")
	}
}

// TestCipherLockstepMutation: with the encode skipped, or the link kept past
// the creature leaving the battlefield, some seed diverges.
func TestCipherLockstepMutation(t *testing.T) {
	for _, mc := range []struct {
		name string
		dbg  *bool
	}{
		{"no encode", &dirac.DebugCipherNoEncode},
		{"link kept", &dirac.DebugCipherKeepLink},
	} {
		*mc.dbg = true
		n, _ := cipherRun(t, cipherDecks[:], 40, nil, false)
		*mc.dbg = false
		if n == 0 {
			t.Errorf("mutation %q: no seed diverged", mc.name)
		}
	}
}

// TestFourSeatCipherLockstep: four-seat Cipher pods play in lockstep, the
// encoded creature's trigger firing on combat damage to any opponent.
func TestFourSeatCipherLockstep(t *testing.T) {
	deck := cipherDecks[0]
	podClassLockstep(t, deck, 12, 5, func(c *PickCtx) bool { return c.G.fr.d != nil && c.G.fr.d.ResumeKind == "cipher" }, &dirac.DebugCipherNoEncode)
}
