package searchtest

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/dirac/search"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
)

// pickOnly hides a Declarer's Declaration: the searcher walks a
// declaration by asking Pick at every pick (the walk before ks-hbot-decl).
type pickOnly struct {
	b interface {
		Pick(*dirac.State, *dirac.Decision) int
	}
}

func (p pickOnly) Pick(s *dirac.State, d *dirac.Decision) int { return p.b.Pick(s, d) }

// pickDeclaration is the Declarer contract's Pick: the list's first pair
// that is not yet declared and is offered, else AkDone, else option 0.
func pickDeclaration(s *dirac.State, d *dirac.Decision, l []dirac.Action) int {
	for _, x := range l {
		if d.Kind == dirac.DkAttackers {
			if _, ok := s.HBotAttackTarget(dirac.Obj(x.Source())); ok {
				continue
			}
		} else if s.HBotBlocking(dirac.Obj(x.Source())) != dirac.NoObj {
			continue
		}
		for i := 0; i < int(d.N); i++ {
			if d.Options[i] == x {
				return i
			}
		}
	}
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkDone {
			return i
		}
	}
	return 0
}

// advBot is a Declarer whose declarations are adversarial in the two ways
// that make the closing-step skip's guards (declSkippable) load-bearing:
//
//   - attackers: it declares every non-must attacker and leaves every
//     must-attacker undeclared, so dirac withholds AkDone after the
//     list is declared (Config.GorgeMustAttack);
//   - blockers: it declares at most one block on each menace attacker, so
//     a menace attacker is left with a lone blocker and dirac
//     withholds AkDone.
//
// Its Pick follows the Declarer contract, so the Pick walk answers exactly
// as this list would. TestDeclWalkMustAttackGuard and
// TestDeclWalkClosingStepGuard in dirac/search pin the closing-step skip
// itself: the Search-level comparison here is masked by candidate
// regeneration (attackCands/blockCands add the missing pair back), so it
// only checks that the adversarial list reaches such a root.
type advBot struct{ b *hbot.Bot }

func (a advBot) Declaration(s *dirac.State, d *dirac.Decision, out []dirac.Action) []dirac.Action {
	switch d.Kind {
	case dirac.DkAttackers:
		seen := map[uint8]bool{}
		for i := 0; i < int(d.N); i++ {
			o := d.Options[i]
			if o.Kind() != dirac.AkAttack || seen[uint8(o.Source())] || s.MustAttack(dirac.Obj(o.Source())) {
				continue
			}
			seen[uint8(o.Source())] = true
			out = append(out, o)
		}
	case dirac.DkBlockers:
		seen := map[uint8]bool{}    // one block per blocker
		menaced := map[uint8]bool{} // one block per menace attacker
		for i := 0; i < int(d.N); i++ {
			o := d.Options[i]
			if o.Kind() != dirac.AkBlock || seen[uint8(o.Source())] || !s.HasKeyword(dirac.Obj(o.Target()), kop.KwMenace) || menaced[uint8(o.Target())] {
				continue
			}
			seen[uint8(o.Source())] = true
			menaced[uint8(o.Target())] = true
			out = append(out, o)
		}
		for i := 0; i < int(d.N); i++ {
			o := d.Options[i]
			if o.Kind() != dirac.AkBlock || seen[uint8(o.Source())] || s.HasKeyword(dirac.Obj(o.Target()), kop.KwMenace) {
				continue
			}
			seen[uint8(o.Source())] = true
			out = append(out, o)
		}
	}
	return out
}

func (a advBot) Pick(s *dirac.State, d *dirac.Decision) int {
	if d.Kind != dirac.DkAttackers && d.Kind != dirac.DkBlockers {
		return a.b.Pick(s, d)
	}
	var buf [dirac.MaxPerm]dirac.Action
	return pickDeclaration(s, d, a.Declaration(s, d, buf[:0]))
}

// menaceBot leaves one block on each of two menace attackers, so the
// dirac withholds AkDone for two lone-blocker deficits: declSkippable
// must be false and the walk must step on to complete both. Skipping
// unconditionally truncates the declaration, and blockCands cannot
// regenerate the completion from a lone-block list (every single added
// block still leaves the other menace attacker short), so Search diverges.
type menaceBot struct{ b *hbot.Bot }

func (m menaceBot) Declaration(s *dirac.State, d *dirac.Decision, out []dirac.Action) []dirac.Action {
	switch d.Kind {
	case dirac.DkAttackers:
		return m.b.Declaration(s, d, out)
	case dirac.DkBlockers:
		seen := map[uint8]bool{}
		for i := 0; i < int(d.N); i++ {
			o := d.Options[i]
			if o.Kind() != dirac.AkBlock || !s.HasKeyword(dirac.Obj(o.Target()), kop.KwMenace) || seen[uint8(o.Target())] {
				continue
			}
			seen[uint8(o.Target())] = true
			out = append(out, o)
		}
	}
	return out
}

func (m menaceBot) Pick(s *dirac.State, d *dirac.Decision) int {
	if d.Kind != dirac.DkAttackers && d.Kind != dirac.DkBlockers {
		return m.b.Pick(s, d)
	}
	var buf [dirac.MaxPerm]dirac.Action
	return pickDeclaration(s, d, m.Declaration(s, d, buf[:0]))
}

// advPick hides advBot's Declaration.
type advPick struct{ a advBot }

func (p advPick) Pick(s *dirac.State, d *dirac.Decision) int { return p.a.Pick(s, d) }

// TestHBotDeclarerSearchExact: a searcher whose bot is hbot (a
// search.Declarer: one Declaration per attackers/blockers decision, the
// walk ending without a step once the list is declared) answers every
// decision of FDN games exactly as one asking hbot's Pick at every pick:
// the same candidates, visits, values and chosen declaration. The
// two-menace subtest makes the closing-step skip's blockers guard
// observable: it fails if the skip is made unconditional.
func TestHBotDeclarerSearchExact(t *testing.T) {
	t.Parallel()

	for _, adv := range []bool{false, true} {
		t.Run(map[bool]string{false: "hbot", true: "adversarial"}[adv], func(t *testing.T) { declarerExact(t, adv) })
	}
	t.Run("two-menace", twoMenaceExact)
}

// twoMenaceExact: with a lone block on each of two menace attackers,
// skipping the closing step truncates the declaration, blockCands rejects
// it and cannot rebuild the completion, and the Declarer search's
// candidates, visits and value differ from the Pick walk's.
func twoMenaceExact(t *testing.T) {
	a := []cardset.Entry{
		{Name: "Sire of Seven Deaths", Count: 4},
		{Name: "Swamp", Count: 36},
	}
	b := []cardset.Entry{
		{Name: "Crypt Feaster", Count: 4},
		{Name: "Swamp", Count: 36},
	}
	set := rawSetT(t, "fdn")
	tbl, da, db, err := set.Matchup(a, b)
	if err != nil {
		t.Fatalf("matchup: %v", err)
	}
	var s dirac.State
	var d dirac.Decision
	found := false
	for seed := uint64(1); seed <= 64 && !found; seed++ {
		s.Init(tbl, da, db, seed, dirac.Config{GorgeMustAttack: true, NoElide: true})
		drive := hbot.New(hbot.NewFacts(tbl))
		for i := 0; i < 8000; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if d.Kind == dirac.DkBlockers && menacedTargets(&s, &d) >= 2 {
				found = true
				break
			}
			s.Apply(d.Options[drive.Pick(&s, &d)])
		}
	}
	if !found {
		t.Fatal("no blockers decision with two menace attackers each having two blockers")
	}
	opts := search.DefaultOptions()
	opts.Sims = 40
	mb := menaceBot{hbot.New(hbot.NewFacts(tbl))}
	x := search.NewSearcher(opts, nil, mb)
	y := search.NewSearcher(opts, nil, pickOnly{mb})
	ra := *x.Search(&s, 7)
	rb := *y.Search(&s, 7)
	if ra.N != rb.N || ra.Keys != rb.Keys || ra.Visits != rb.Visits || ra.Q != rb.Q ||
		ra.NActs != rb.NActs || ra.Acts != rb.Acts {
		t.Fatalf("Declarer search %v %v != Pick search %v %v", ra.Acts[:ra.NActs], ra.Visits[:ra.N], rb.Acts[:rb.NActs], rb.Visits[:rb.N])
	}
}

// menacedTargets counts the attackers in a blockers decision that have
// menace and at least two distinct offered blockers.
func menacedTargets(s *dirac.State, d *dirac.Decision) int {
	byTarget := map[uint8]map[uint8]bool{}
	for i := 0; i < int(d.N); i++ {
		o := d.Options[i]
		if o.Kind() != dirac.AkBlock || !s.HasKeyword(dirac.Obj(o.Target()), kop.KwMenace) {
			continue
		}
		if byTarget[uint8(o.Target())] == nil {
			byTarget[uint8(o.Target())] = map[uint8]bool{}
		}
		byTarget[uint8(o.Target())][uint8(o.Source())] = true
	}
	n := 0
	for _, srcs := range byTarget {
		if len(srcs) >= 2 {
			n++
		}
	}
	return n
}

func declarerExact(t *testing.T, adv bool) {
	c, pool := testCorpus(t)
	opts := search.DefaultOptions()
	opts.Sims = 40
	compared, combat, mustGuards, menaceGuards := 0, 0, 0, 0
	ps := oracle.PairingsFrom(pool, 0, 6, 53)
	// Pair 116's seed-53 deal reaches an attackers decision with a
	// must-attacker (Juggernaut) under GorgeMustAttack; the adversarial
	// list leaves it undeclared. Pairs 1 and 4 (already in 0-5) reach
	// blockers decisions with a menace attacker. These are coverage
	// signals; the two-menace subtest and
	// TestDeclWalkMustAttackGuard/TestDeclWalkClosingStepGuard in
	// dirac/search are the tests that fail when the skip is made
	// unconditional.
	ps = append(ps, oracle.PairingsFrom(pool, 116, 1, 53)...)
	for _, p := range ps {
		m, tbl, err := oracle.SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			continue
		}
		side, err := oracle.RealKernel(tbl, nil, oracle.Config{PassElision: true})(m, p.Seed)
		if err != nil {
			continue
		}
		ks, kd, _ := oracle.KernelStateOf(side)
		f := oracle.HBotFacts(c.Reg, tbl, nil)
		var _ search.Declarer = hbot.New(f)
		var ba, bb search.Bot = hbot.New(f), pickOnly{hbot.New(f)}
		var ab advBot
		if adv {
			ab = advBot{hbot.New(f)}
			ba, bb = ab, advPick{ab}
		}
		a := search.NewSearcher(opts, nil, ba)
		b := search.NewSearcher(opts, nil, bb)
		for i := 0; i < 600; i++ {
			ks.StepInto(kd)
			if kd.Kind == dirac.DkNone {
				break
			}
			if adv {
				var buf [dirac.MaxPerm]dirac.Action
				l := ab.Declaration(ks, kd, buf[:0])
				switch {
				case kd.Kind == dirac.DkAttackers && len(l) > 0 && !offersDone(kd):
					mustGuards++
				case kd.Kind == dirac.DkBlockers && len(l) > 0 && targetsMenace(ks, l):
					menaceGuards++
				}
			}
			seed := p.Seed*31 + uint64(i)
			ra := *a.Search(ks, seed)
			rb := *b.Search(ks, seed)
			if ra.N != rb.N || ra.Keys != rb.Keys || ra.Visits != rb.Visits || ra.Q != rb.Q ||
				ra.NActs != rb.NActs || ra.Acts != rb.Acts {
				t.Fatalf("pair %d decision %d (kind %d): Declarer search %v %v != Pick search %v %v",
					p.Index, i, kd.Kind, ra.Acts[:ra.NActs], ra.Visits[:ra.N], rb.Acts[:rb.NActs], rb.Visits[:rb.N])
			}
			compared++
			if kd.Kind == dirac.DkAttackers || kd.Kind == dirac.DkBlockers {
				combat++
			}
			k0, p0 := kd.Kind, kd.Player
			var scratch dirac.Decision
			for j := 0; j < ra.NActs; j++ {
				if j > 0 && !search.Offered(ks, ra.Acts[j], k0, p0, &scratch) {
					continue
				}
				ks.Apply(ra.Acts[j])
			}
		}
	}
	if combat < 20 {
		t.Fatalf("too few combat roots compared: %d of %d", combat, compared)
	}
	if adv && mustGuards == 0 {
		t.Fatalf("no must-attacker root left undeclared: declSkippable's attackers guard is untested")
	}
	t.Logf("%d roots compared, %d at attackers/blockers, %d must-attacker guards, %d menace guards",
		compared, combat, mustGuards, menaceGuards)
}

// offersDone reports whether d offers AkDone.
func offersDone(d *dirac.Decision) bool {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkDone {
			return true
		}
	}
	return false
}

// targetsMenace reports whether a listed pair blocks a menace attacker.
func targetsMenace(s *dirac.State, l []dirac.Action) bool {
	for _, x := range l {
		if s.HasKeyword(dirac.Obj(x.Target()), kop.KwMenace) {
			return true
		}
	}
	return false
}
