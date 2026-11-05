package search

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/adams-shaun/gorge/dirac"
)

// Options.CardSims is a per-card simulation budget (:csims=NAME.N): a
// searched decision whose root candidates cast a listed card, or activate
// an ability of one, runs N simulations. A measurement found Burst Lightning cast in the own upkeep or draw at about half its
// casts and its games-in-hand win rate 4.3 points under 17Lands Sealed's,
// and an 800-sim oracle moved its decision about 1 time in 10 (towards
// casting more); the budget asks whether much deeper search at exactly
// those decisions changes the card's timing, at about +30% simulations a
// game for one card, instead of raising sims= everywhere. A diagnostic arm:
// per-kind budgets lost at equal compute, so it is judged by the
// card's timing shares and games-in-hand gap, not adopted on strength.
//
// The budget is read in Search after the root's point is built -- the
// candidate set decides it, so a cast the topk= cut (or the cands= cap)
// dropped does not boost, unless CardSimsForce keeps it (forceCards). It
// wins over every budget SeatSearch sets (Sims, a SimsByKind entry, the
// playout-cap CheapSims and the decided DecidedSims) and changes only the
// simulation count: a cheap decision stays unrecorded and noise-free, a
// decided one keeps its weight, an early-cut rule still applies, and
// Result.Sims (a record's sims) is the boosted budget. With several listed
// cards among the candidates the largest entry's N is the budget. MaxSims
// counts every entry, so the tree is preallocated for it (TreeBytes: 4,648
// bytes per simulation of the largest entry, per Searcher). A host that
// builds a Searcher per seat per worker pays it per seat: csims=NAME.3200
// on one seat is 13.2 MB a worker, 6.6 GB at 500 workers, and twice that
// when both seats of a self-play run list it.
// Under pcr= a boosted cheap search still runs noise-free and writes no
// record: the boost reaches the game's move, not the corpus.
// Only the root's candidates decide the budget; a listed card castable at
// an in-walk point does not. Cards match by table name (kop.CardDef.Name),
// so a matchup table and the pool table agree.

// validateCardNames refuses card-name lists a search must not run with
// (Options.Validate): a CardSims entry with a name but Sims < 1, or Sims
// but no name; an entry or name after the list's end (packed from index
// 0); a name listed twice; CardSimsForce without CardSims;
// HoldUpkeepExempt without HoldUpkeep; and RaidHoldOnly without RaidHold.
func (o *Options) validateCardNames() error {
	end := false
	for i, c := range o.CardSims {
		switch {
		case c.Name == "" && c.Sims == 0:
			end = true
			continue
		case c.Name == "":
			return fmt.Errorf("search: CardSims[%d] has %d sims and no card name", i, c.Sims)
		case end:
			return fmt.Errorf("search: CardSims[%d] %q follows the list's end (pack the entries from index 0)", i, c.Name)
		case c.Sims < 1:
			return fmt.Errorf("search: CardSims %q: %d sims, want >= 1", c.Name, c.Sims)
		}
		for _, p := range o.CardSims[:i] {
			if p.Name == c.Name {
				return fmt.Errorf("search: CardSims lists %q twice", c.Name)
			}
		}
	}
	if o.CardSimsForce && o.CardSims[0].Name == "" {
		return errors.New("search: CardSimsForce needs CardSims")
	}
	if err := validateNames(&o.HoldUpkeepExempt, o.HoldUpkeep, "HoldUpkeepExempt", "HoldUpkeep"); err != nil {
		return err
	}
	return validateNames(&o.RaidHoldOnly, o.RaidHold, "RaidHoldOnly", "RaidHold")
}

// validateNames refuses a card-name list (field, the option on its key)
// with a name after the list's end (packed from index 0), a name listed
// twice, or a name while on is false.
func validateNames(names *[MaxCardNames]string, on bool, field, key string) error {
	end := false
	for i, n := range names {
		switch {
		case n == "":
			end = true
			continue
		case end:
			return fmt.Errorf("search: %s[%d] %q follows the list's end (pack the names from index 0)", field, i, n)
		case !on:
			return fmt.Errorf("search: %s needs %s", field, key)
		}
		for _, p := range names[:i] {
			if p == n {
				return fmt.Errorf("search: %s lists %q twice", field, n)
			}
		}
	}
	return nil
}

// cardSims is the CardSims entry for option a at s: its Sims when a casts
// or activates a listed card, 0 otherwise.
func (sr *Searcher) cardSims(s *dirac.State, a dirac.Action) int {
	if k := a.Kind(); k != dirac.AkCast && k != dirac.AkActivate {
		return 0
	}
	name := s.Table().Cards[s.ObjCard(dirac.Obj(a.Source()))].Name
	for i := range sr.Opts.CardSims {
		e := &sr.Opts.CardSims[i]
		if e.Name == "" {
			break
		}
		if e.Name == name {
			return int(e.Sims)
		}
	}
	return 0
}

// cardBudget is the root point pt's CardSims budget at s: the largest
// entry among its candidates' actions, 0 when none plays a listed card.
func (sr *Searcher) cardBudget(s *dirac.State, pt *point) int {
	n := 0
	for j := 0; j < int(pt.off[pt.n]); j++ {
		n = max(n, sr.cardSims(s, dirac.Action(pt.acts[j])))
	}
	return n
}

// forceCards is Options.CardSimsForce at the root's priority point pt
// (buildPoint, after the cut): every option of d that casts or activates a
// listed card and is not a candidate is appended, unless HoldUpkeep or
// RaidHold holds it (a held cast is never forced back) or InstWin / Defer
// blocks it (instwin.go instBlocked: nor is a blocked own-turn instant).
// The prior is then rebuilt over the candidates kept: the network's softmax
// over their logits (tempered by PriorTemp) when the point is ranked (the
// cut's renormalised prior, the forced ones included), else uniform, and
// Defer's main1 scale applied again (deferScale). Stats.CardForced counts
// the candidates added.
func (sr *Searcher) forceCards(s *dirac.State, d *dirac.Decision, pt *point, rank bool) {
	hold := sr.Opts.HoldUpkeep && sr.holdOn(s, d)
	raid := sr.Opts.RaidHold && sr.raidOn(s, d)
	own := (sr.Opts.InstWin != InstWinOff || sr.Opts.Defer > 0) && d.Player == sr.seat
	added := 0
	var one [1]uint32
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if sr.cardSims(s, a) == 0 || (hold && sr.holdsCast(s, a)) || (raid && sr.raidCast(s, a)) || (own && sr.instBlocked(s, a)) {
			continue
		}
		in := false
		for c := 0; c < pt.n && !in; c++ {
			in = pt.off[c+1]-pt.off[c] == 1 && pt.acts[pt.off[c]] == uint32(a)
		}
		if in {
			continue
		}
		one[0] = uint32(a)
		if !pt.addSeq(one[:]) {
			break
		}
		pt.keys[pt.n-1] = Key(mix(uint64(kindPriority), sr.actKey(s, a, false)))
		added++
	}
	if added == 0 {
		return
	}
	sr.St.CardForced += added
	for i := 0; i < pt.n; i++ {
		if rank {
			pt.prior[i] = sr.candLogit(d, kindPriority, pt, i)
		} else {
			pt.prior[i] = 1 / float64(pt.n)
		}
	}
	if rank {
		sr.temper(pt, false) // Options.PriorTemp (the root: the seat's)
		softmax(pt.prior[:pt.n])
	}
	if sr.Opts.Defer > 0 && d.Player == sr.seat {
		sr.deferScale(s, pt) // instwin.go, :defer=: as buildPoint applied it before the cut
	}
}

// TreeBytes is the size of the tree NewSearcher preallocates for opts:
// MaxSims+2 nodes, each with MaxCands edges (TestTreeBytesPerSim). A
// Searcher holds one, so a worker holds one per searching seat: csims= on
// both seats of a self-play run pays its largest entry twice per worker.
func TreeBytes(opts *Options) int64 {
	return int64(MaxSims(opts)+2) * int64(unsafe.Sizeof(node{})+MaxCands*unsafe.Sizeof(edge{}))
}
