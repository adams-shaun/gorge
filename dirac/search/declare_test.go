package search

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestBotDeclarationStopsAtTurnBoundary: after the bot declares its last
// attacker, StepInto auto-applies the Done-only re-pose, the empty blocks
// and the pass-only priorities, so the old walk (Kind and Player only)
// could land on the same seat's attackers decision on a later turn and
// return that turn's attack as part of this declaration. The scripted
// prefix below reaches a vanilla game's turn-5 P0 attackers decision (seed
// 4246, the deal the scan found) where the next P0 attackers decision is
// turn 7 and every decision between is forced.
func TestBotDeclarationStopsAtTurnBoundary(t *testing.T) {
	sr := NewSearcher(DefaultOptions(), Uniform{}, Heuristic{})
	var s dirac.State
	s.Init(v.Table, v.Decks[4246%5], v.Decks[(4246+2)%5], 4246, dirac.Config{})
	script := []uint32{
		17498112, 35061760, 19398656, 17825792, 68680960,
		19791872, 17760256, 35848192, 117505280, 154075136, 155058176,
	}
	var d dirac.Decision
	for i, a := range script {
		s.StepInto(&d)
		want := twoSeatAction(a)
		offered := false
		for j := 0; j < int(d.N); j++ {
			if d.Options[j] == want {
				s.Apply(want)
				offered = true
				break
			}
		}
		if !offered {
			t.Fatalf("prefix action %d (%d) not offered at decision %d (kind %d player %d opts %v)",
				i, a, i, d.Kind, d.Player, d.Options[:d.N])
		}
	}
	s.StepInto(&d)
	if d.Kind != dirac.DkAttackers || d.Player != 0 || s.Turn() != 5 {
		t.Fatalf("setup: decision kind %d player %d turn %d, want P0 attackers on turn 5",
			d.Kind, d.Player, s.Turn())
	}
	bot := -1
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkAttack {
			if bot >= 0 {
				t.Fatalf("setup: more than one attacker offered: %v", d.Options[:d.N])
			}
			bot = i
		}
	}
	if bot < 0 {
		t.Fatalf("setup: no attacker offered: %v", d.Options[:d.N])
	}
	// The old walk must reach the same kind for the same player on a later
	// turn; otherwise this state does not exercise the bug.
	sr.tmp.CopyFrom(&s)
	sr.tmp.Apply(d.Options[bot])
	sr.tmp.StepInto(&sr.td)
	if sr.td.Kind != d.Kind || sr.td.Player != d.Player || sr.tmp.Turn() == s.Turn() {
		t.Fatalf("setup: after the attack the walk reaches kind %d player %d turn %d, want P0 attackers on a later turn",
			sr.td.Kind, sr.td.Player, sr.tmp.Turn())
	}
	var picks [MaxPerm]uint32
	n := sr.botDeclaration(&s, &d, bot, &picks)
	if n != 1 {
		t.Fatalf("botDeclaration returned %d picks %v, want 1 (it walked into turn %d)",
			n, picks[:n], sr.tmp.Turn())
	}
	if dirac.Action(picks[0]) != d.Options[bot] {
		t.Fatalf("botDeclaration pick %d, want the offered attack %d", picks[0], uint32(d.Options[bot]))
	}
}

// TestBotDeclarationDistinctSources: a declaration never declares the same
// source twice. Over the seeded seat-search games, check every attackers
// and blockers decision's bot declaration (the same call attackCands,
// blockCands and Search's unsearched root make).
func TestBotDeclarationDistinctSources(t *testing.T) {
	opts := DefaultOptions()
	opts.Sims = 8
	sr := NewSearcher(opts, Uniform{}, Heuristic{})
	checked := 0
	for seed := uint64(1); seed <= 8; seed++ {
		var st SeatState
		var s dirac.State
		s.Init(v.Table, v.Decks[seed%5], v.Decks[(seed+2)%5], seed, dirac.Config{})
		pol := dirac.NewRandomPolicy(seed ^ 0x1234)
		var d dirac.Decision
		for i := 0; i < 3000; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if d.Kind == dirac.DkAttackers || d.Kind == dirac.DkBlockers {
				bot := sr.Bot.Pick(&s, &d)
				if d.Options[bot].Kind() != dirac.AkDone {
					var picks [MaxPerm]uint32
					n := sr.botDeclaration(&s, &d, bot, &picks)
					checked++
					for j := 1; j < n; j++ {
						for k := 0; k < j; k++ {
							if dirac.Action(picks[j]).Source() == dirac.Action(picks[k]).Source() {
								t.Fatalf("seed %d decision %d: picks %d and %d share source %d (declaration %v)",
									seed, i, k, j, dirac.Action(picks[j]).Source(), picks[:n])
							}
						}
					}
				}
			}
			if d.Player == 0 {
				r := sr.SeatSearch(&s, &d, seed*0x51, &st)
				r.Play(&s)
				continue
			}
			s.Apply(pol.Pick(&d))
		}
	}
	if checked == 0 {
		t.Fatal("no declaration was checked")
	}
}

// oneMenaceBlock is a Declarer that declares one block on the first
// menace attacker and nothing else: dirac then withholds AkDone
// because that attacker has a lone blocker.
type oneMenaceBlock struct{}

func (oneMenaceBlock) Declaration(s *dirac.State, d *dirac.Decision, out []dirac.Action) []dirac.Action {
	if d.Kind != dirac.DkBlockers {
		return out
	}
	for i := 0; i < int(d.N); i++ {
		o := d.Options[i]
		if o.Kind() == dirac.AkBlock && s.HasKeyword(dirac.Obj(o.Target()), kop.KwMenace) {
			return append(out, o)
		}
	}
	return out
}

// TestDeclWalkClosingStepGuard: declSkippable must be false at a blockers
// decision whose Declarer list leaves a menace attacker with one blocker,
// so declWalk steps on and the Pick walk's second block is included. If
// the closing step is skipped unconditionally the walk stops after the
// listed block and returns one pick, not two.
func TestDeclWalkClosingStepGuard(t *testing.T) {
	a := []cardset.Entry{
		{Name: "Sire of Seven Deaths", Count: 4},
		{Name: "Swamp", Count: 36},
	}
	b := []cardset.Entry{
		{Name: "Frenzied Goblin", Count: 4},
		{Name: "Mountain", Count: 36},
	}
	var s dirac.State
	var d dirac.Decision
	found := false
	set := cardsettest.Set(t, "fdn")
	for seed := uint64(1); seed <= 32 && !found; seed++ {
		tbl, da, db, err := set.Matchup(a, b)
		if err != nil {
			t.Fatalf("matchup: %v", err)
		}
		s.Init(tbl, da, db, seed, dirac.Config{GorgeMustAttack: true, NoElide: true})
		f := hbot.NewFacts(tbl)
		bot := hbot.New(f)
		for i := 0; i < 5000; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if d.Kind == dirac.DkBlockers && menacePair(&s, &d) {
				found = true
				break
			}
			s.Apply(d.Options[bot.Pick(&s, &d)])
		}
	}
	if !found {
		t.Fatal("no blockers decision with a two-block menace attacker")
	}
	block := -1
	for i := 0; i < int(d.N); i++ {
		o := d.Options[i]
		if o.Kind() == dirac.AkBlock && s.HasKeyword(dirac.Obj(o.Target()), kop.KwMenace) {
			block = i
			break
		}
	}
	if block < 0 {
		t.Fatalf("setup: no menace block offered: %v", d.Options[:d.N])
	}
	sr := NewSearcher(DefaultOptions(), Uniform{}, Heuristic{})
	dc := oneMenaceBlock{}
	l := dc.Declaration(&s, &d, nil)
	if declSkippable(&s, &d, l) {
		t.Fatalf("declSkippable = true with a lone menace block %v", l)
	}
	var picks [MaxPerm]uint32
	n := sr.declWalk(dc, &s, &d, d.Options[block], &picks)
	if n != 2 {
		t.Fatalf("declWalk returned %d picks %v, want 2 (the listed block and the second block)", n, picks[:n])
	}
	if dirac.Action(picks[1]).Kind() != dirac.AkBlock || dirac.Action(picks[1]).Target() != dirac.Action(picks[0]).Target() {
		t.Fatalf("second pick %v is not a second block on the same attacker as %v", dirac.Action(picks[1]), dirac.Action(picks[0]))
	}
}

// menacePair reports a blockers decision offering two blocks from
// distinct blockers on the same menace attacker.
func menacePair(s *dirac.State, d *dirac.Decision) bool {
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
	for _, srcs := range byTarget {
		if len(srcs) >= 2 {
			return true
		}
	}
	return false
}

// nonMustAttacker is a Declarer that lists every offered attack by a
// creature that need not attack, and nothing else: under
// Config.GorgeMustAttack dirac withholds AkDone until a must-attacker
// is declared, so the walk must step on and add it.
type nonMustAttacker struct{}

func (nonMustAttacker) Declaration(s *dirac.State, d *dirac.Decision, out []dirac.Action) []dirac.Action {
	if d.Kind != dirac.DkAttackers {
		return out
	}
	seen := map[uint8]bool{}
	for i := 0; i < int(d.N); i++ {
		o := d.Options[i]
		if o.Kind() != dirac.AkAttack || seen[uint8(o.Source())] || s.MustAttack(dirac.Obj(o.Source())) {
			continue
		}
		seen[uint8(o.Source())] = true
		out = append(out, o)
	}
	return out
}

// TestDeclWalkMustAttackGuard: declSkippable must be false at an attackers
// decision that withholds AkDone for an undeclared must-attacker, so
// declWalk steps on and declares it. If the closing step were skipped
// unconditionally the walk would stop after the listed non-must attacks.
func TestDeclWalkMustAttackGuard(t *testing.T) {
	a := []cardset.Entry{
		{Name: "Juggernaut", Count: 4},
		{Name: "Frenzied Goblin", Count: 4},
		{Name: "Mountain", Count: 32},
	}
	b := []cardset.Entry{
		{Name: "Sire of Seven Deaths", Count: 4},
		{Name: "Swamp", Count: 36},
	}
	var s dirac.State
	var d dirac.Decision
	found := false
	set := cardsettest.Set(t, "fdn")
	for seed := uint64(1); seed <= 64 && !found; seed++ {
		tbl, da, db, err := set.Matchup(a, b)
		if err != nil {
			t.Fatalf("matchup: %v", err)
		}
		s.Init(tbl, da, db, seed, dirac.Config{GorgeMustAttack: true, NoElide: true})
		bot := hbot.New(hbot.NewFacts(tbl))
		for i := 0; i < 5000; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if d.Kind == dirac.DkAttackers && !doneOffered(&d) && hasNonMustAttack(&s, &d) {
				found = true
				break
			}
			s.Apply(d.Options[bot.Pick(&s, &d)])
		}
	}
	if !found {
		t.Fatal("no attackers decision withholding AkDone with a non-must attacker")
	}
	dc := nonMustAttacker{}
	l := dc.Declaration(&s, &d, nil)
	if len(l) == 0 {
		t.Fatalf("setup: no non-must attack offered: %v", d.Options[:d.N])
	}
	if declSkippable(&s, &d, l) {
		t.Fatalf("declSkippable = true while AkDone is withheld for a must-attacker (%v)", l)
	}
	sr := NewSearcher(DefaultOptions(), Uniform{}, Heuristic{})
	var picks [MaxPerm]uint32
	n := sr.declWalk(dc, &s, &d, l[0], &picks)
	if n <= len(l) {
		t.Fatalf("declWalk returned %d picks %v, want more than the %d listed (a must-attacker)", n, picks[:n], len(l))
	}
	for j := 0; j < len(l); j++ {
		if dirac.Action(picks[j]) != l[j] {
			t.Fatalf("pick %d = %v, want listed pair %v", j, dirac.Action(picks[j]), l[j])
		}
	}
	for j := len(l); j < n; j++ {
		if !s.MustAttack(dirac.Obj(dirac.Action(picks[j]).Source())) {
			t.Fatalf("extra pick %d = %v is not a must-attacker", j, dirac.Action(picks[j]))
		}
	}
}

// doneOffered reports whether d offers AkDone.
func doneOffered(d *dirac.Decision) bool {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkDone {
			return true
		}
	}
	return false
}

// hasNonMustAttack reports whether d offers an attack by a creature that
// need not attack.
func hasNonMustAttack(s *dirac.State, d *dirac.Decision) bool {
	for i := 0; i < int(d.N); i++ {
		o := d.Options[i]
		if o.Kind() == dirac.AkAttack && !s.MustAttack(dirac.Obj(o.Source())) {
			return true
		}
	}
	return false
}

// TestDeclarerLazyPriors: lazyPoint may stand in for a pure bot's Pick at
// an attackers or blockers decision, and hbot is both a PureBot and a
// Declarer. The stand-in must use the whole declaration, not a Pick walk
// that would stop after the first pair.
func TestDeclarerLazyPriors(t *testing.T) {
	a := []cardset.Entry{
		{Name: "Sire of Seven Deaths", Count: 4},
		{Name: "Swamp", Count: 36},
	}
	b := []cardset.Entry{
		{Name: "Frenzied Goblin", Count: 4},
		{Name: "Mountain", Count: 36},
	}
	var s dirac.State
	var d dirac.Decision
	found := false
	set := cardsettest.Set(t, "fdn")
	for seed := uint64(1); seed <= 32 && !found; seed++ {
		tbl, da, db, err := set.Matchup(a, b)
		if err != nil {
			t.Fatalf("matchup: %v", err)
		}
		s.Init(tbl, da, db, seed, dirac.Config{GorgeMustAttack: true, NoElide: true})
		bot := hbot.New(hbot.NewFacts(tbl))
		for i := 0; i < 5000; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if (d.Kind == dirac.DkAttackers || d.Kind == dirac.DkBlockers) && d.N >= 2 && lazyOneSource(&d) >= 0 {
				found = true
				break
			}
			s.Apply(d.Options[bot.Pick(&s, &d)])
		}
	}
	if !found {
		t.Fatal("no blockers decision with at least two options")
	}
	opts := DefaultOptions()
	opts.Sims = 8
	sr := NewSearcher(opts, &fakeNet{val: 0}, hbot.New(hbot.NewFacts(s.Table())))
	r := sr.Search(&s, 1)
	if r.N == 0 {
		t.Fatal("search with a pure Declarer bot and a net prior returned no candidates")
	}
	if sr.St.PriorSkips == 0 {
		t.Fatal("lazyPoint never accepted a pure Declarer bot's declaration")
	}
}

// twoSeatAction builds this build's Action from a scripted action written as
// the two-seat build packs it (kind<<24 | source<<16 | target<<8 | mode, a
// byte a code, StackTarget0 0xEC, PlayerTarget0 0xFC): the four-seat class
// packs sixteen-bit codes and moves the stack and player codes.
func twoSeatAction(a uint32) dirac.Action {
	code := func(c uint32) dirac.TCode {
		switch {
		case c < 0xEC:
			return dirac.TCode(c)
		case c < 0xFC:
			return dirac.StackTarget0 + dirac.TCode(c-0xEC)
		case c == 0xFE:
			return dirac.StackTargetGone
		case c == 0xFF:
			return dirac.NoTarget
		}
		return dirac.PlayerTarget0 + dirac.TCode(c-0xFC)
	}
	return dirac.MakeAction(dirac.ActionKind(a>>24), code(a>>16&0xFF), code(a>>8&0xFF), uint8(a))
}
