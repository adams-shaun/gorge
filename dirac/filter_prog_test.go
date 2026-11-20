package dirac

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Filter programs (filter_prog.go): the seed words against the struct
// encodings they restate, on FDN positions, and the matcher kinds that must
// reach the program.

// fpKind is the matcher kinds a word is compared in.
type fpKind uint8

const (
	fpPerm fpKind = 1 << iota // matchPermOne
	fpCard                    // matchCardOne: a card off the battlefield, the stack's included
	fpBoth = fpPerm | fpCard
)

// fpPair is one word's literal as a struct filter, as a program, and as a
// negated program. The three filters are in one table.
type fpPair struct {
	name         string
	group        string // pairs of one group (a word's subtype cases) count together
	kinds        fpKind
	st, pr, neg  kop.FilterID
	hits, misses [2]int // per fpKind index (perm, card): struct-side true and false counts
}

type fpCase struct {
	name  string
	kinds fpKind
	st    kop.Filter
	lit   kop.FLit
}

// fpCases are the seed words, each with the struct filter that reads the same
// thing. The subtype cases are the table's commonest subtypes, so each is
// held by some card in play.
func fpCases(tbl *kop.Table) []fpCase {
	cs := []fpCase{
		{"power<=2", fpBoth, kop.Filter{Pred: kop.PredPowerLE, PredN: 2}, kop.FLit{W: kop.FwPowerCmp, Op: kop.CmpLE, V: kop.Const(2)}},
		{"power>=3", fpBoth, kop.Filter{Pred: kop.PredPowerGE, PredN: 3}, kop.FLit{W: kop.FwPowerCmp, Op: kop.CmpGE, V: kop.Const(3)}},
		{"toughness<=2", fpBoth, kop.Filter{Pred: kop.PredToughnessLE, PredN: 2}, kop.FLit{W: kop.FwToughnessCmp, Op: kop.CmpLE, V: kop.Const(2)}},
		{"cmc<=2", fpBoth, kop.Filter{Pred: kop.PredCMCLE, PredN: 2}, kop.FLit{W: kop.FwCMCCmp, Op: kop.CmpLE, V: kop.Const(2)}},
		{"cmc>=3", fpBoth, kop.Filter{Pred: kop.PredCMCGE, PredN: 3}, kop.FLit{W: kop.FwCMCCmp, Op: kop.CmpGE, V: kop.Const(3)}},
		{"cmc==4", fpBoth, kop.Filter{Pred: kop.PredCMCEQ, PredN: 4}, kop.FLit{W: kop.FwCMCCmp, Op: kop.CmpEQ, V: kop.Const(4)}},
		{"counters>=1", fpPerm, kop.Filter{Pred: kop.PredCountersGE, PredN: int16(kop.CtrP1P1) | 1<<8}, kop.FLit{W: kop.FwCountersCmp, Arg: int16(kop.CtrP1P1), Op: kop.CmpGE, V: kop.Const(1)}},
		{"flying", fpBoth, kop.Filter{Pred: kop.PredHasKeyword, PredN: 0}, kop.FLit{W: kop.FwKeyword, Arg: 0}},
		{"tapped", fpPerm, kop.Filter{Flags: kop.FTapped}, kop.FLit{W: kop.FwTapped}},
		{"attacking", fpPerm, kop.Filter{Flags: kop.FAttacking}, kop.FLit{W: kop.FwAttacking}},
	}
	count := make([]int, len(tbl.Subtypes))
	for i := range tbl.Cards {
		for j := range count {
			if tbl.Cards[i].Subtypes.Has(j) {
				count[j]++
			}
		}
	}
	for rank := 0; rank < 4 && len(count) > 0; rank++ {
		best := 0
		for j := range count {
			if count[j] > count[best] {
				best = j
			}
		}
		count[best] = -1
		cs = append(cs, fpCase{"subtype" + string(rune('0'+rank)), fpBoth, kop.Filter{Sub: uint8(best + 1)}, kop.FLit{W: kop.FwSubtype, Arg: int16(best)}})
	}
	return cs
}

// addFilterProgPairs appends the cases' filters to tbl, each in two
// variants: bare, and with a creature type mask that the program's struct
// fields must hold as its first conjunct.
func addFilterProgPairs(tbl *kop.Table) []fpPair {
	if len(tbl.FilterProgs) == 0 {
		tbl.FilterProgs = append(tbl.FilterProgs, kop.FilterProg{})
	}
	addF := func(f kop.Filter) kop.FilterID {
		tbl.Filters = append(tbl.Filters, f)
		return kop.FilterID(len(tbl.Filters) - 1)
	}
	prog := func(base kop.Filter, l kop.FLit) kop.FilterID {
		tbl.FilterProgs = append(tbl.FilterProgs, kop.FilterProg{Lits: []kop.FLit{l}})
		base.Pred, base.PredN = kop.PredProg, int16(len(tbl.FilterProgs)-1)
		return addF(base)
	}
	var out []fpPair
	for _, c := range fpCases(tbl) {
		for _, base := range []kop.Filter{{}, {Types: kop.TCreature}} {
			st := c.st
			st.Types = base.Types
			nl := c.lit
			nl.Neg = true
			suffix := map[bool]string{false: "", true: "+creature"}[base.Types != 0]
			group := c.name
			if strings.HasPrefix(c.name, "subtype") {
				group = "subtype"
			}
			out = append(out, fpPair{name: c.name + suffix, group: group + suffix, kinds: c.kinds, st: addF(st), pr: prog(base, c.lit), neg: prog(base, nl)})
		}
	}
	return out
}

// fpPositions plays random FDN games (the committed bench pairings, so no
// pool is needed) with fn run at every step-th decision, until n positions.
// Each game has its own table; setup gets it before the game starts.
func fpPositions(t *testing.T, n, step int, setup func(*kop.Table), fn func(s *State)) {
	t.Helper()
	decks := benchDecks(t, "fdn")
	if len(decks) < 2 {
		t.Skip("no committed FDN bench decks")
	}
	set := cardsettest.Set(t, "fdn")
	got := 0
	for g := 0; got < n; g++ {
		if g >= len(decks)/2 {
			t.Fatalf("%d bench pairings gave %d positions, want %d", len(decks)/2, got, n)
		}
		tbl, da, db, err := set.Matchup(decks[2*g], decks[2*g+1])
		if err != nil {
			t.Fatal(err)
		}
		setup(tbl)
		s := NewConfig(tbl, da, db, uint64(g)*0x9e37+1, Config{NoElide: true})
		pol := NewRandomPolicy(uint64(g)*7919 + 3)
		var d Decision
		for i := 0; got < n && i < 4000; i++ {
			s.StepInto(&d)
			if d.Kind == DkNone {
				break
			}
			if i%step == 0 {
				fn(s)
				got++
			}
			s.Apply(pol.Pick(&d))
		}
		ReleaseTable(tbl)
	}
}

// forObjects calls perm for each battlefield slot and card for each object
// elsewhere.
func forObjects(s *State, perm func(sl Slot), card func(o Obj)) {
	k := s.k()
	for m := s.occ; m.any(); m = m.rest() {
		perm(m.first())
	}
	for o := Obj(0); o < s.nobj; o++ {
		switch s.objs[o].zone {
		case kop.ZoneLibrary, kop.ZoneHand, kop.ZoneGraveyard, kop.ZoneExile, kop.ZoneStack, kop.ZoneCommand:
			if int(s.objs[o].card) < len(k.t.Cards) {
				card(o)
			}
		}
	}
}

// TestFilterProgEqualsStruct: for each seed word, the struct filter and the
// same filter as a PredProg (with and without a struct conjunct beside it)
// agree on every object of 200 random FDN positions, in the permanent matcher
// and the card matcher, and the negated literal is the inverse (of the bare filter). Each pair must
// see both outcomes, so an agreement is not two constant answers.
func TestFilterProgEqualsStruct(t *testing.T) {
	var pairs []fpPair
	var perPos [][]fpPair // each game's pairs: the filter ids are per table
	byTable := map[*kop.Table][]fpPair{}
	positions, permCmp, cardCmp := 0, 0, 0
	fpPositions(t, 200, 5, func(tbl *kop.Table) {
		ps := addFilterProgPairs(tbl)
		byTable[tbl] = ps
		perPos = append(perPos, ps)
	}, func(s *State) {
		positions++
		k := s.k()
		ps := byTable[k.t]
		check := func(kind string, p *fpPair, idx int, want, got, neg bool, desc string) {
			if got != want {
				t.Fatalf("%s %s: struct %v, program %v on %s", kind, p.name, want, got, desc)
			}
			if !strings.HasSuffix(p.name, "+creature") && neg != !want { // a struct conjunct beside it is not inverted
				t.Fatalf("%s %s: struct %v, negated program %v on %s", kind, p.name, want, neg, desc)
			}
			if want {
				p.hits[idx]++
			} else {
				p.misses[idx]++
			}
		}
		forObjects(s, func(sl Slot) {
			for i := range ps {
				p := &ps[i]
				if p.kinds&fpPerm == 0 {
					continue
				}
				pl := &s.perms[sl]
				run := func(f kop.FilterID) bool { return s.matchPermOne(k, &k.t.Filters[f], sl, pl.ctrl, NoSlot, layerFull) }
				check("perm", p, 0, run(p.st), run(p.pr), run(p.neg), k.t.Cards[s.cardAt(sl)].Name)
				permCmp++
			}
		}, func(o Obj) {
			ob := &s.objs[o]
			for i := range ps {
				p := &ps[i]
				if p.kinds&fpCard == 0 {
					continue
				}
				run := func(f kop.FilterID) bool {
					return s.matchCardOne(k, &k.t.Filters[f], ob.card, ob.owner(), ob.owner(), o)
				}
				check("card", p, 1, run(p.st), run(p.pr), run(p.neg), fmt.Sprintf("%s in zone %d", k.t.Cards[ob.card].Name, ob.zone))
				cardCmp++
			}
		})
	})
	if positions != 200 {
		t.Fatalf("%d positions, want 200", positions)
	}
	if permCmp == 0 || cardCmp == 0 {
		t.Fatalf("compared %d permanent and %d card reads: a matcher kind was not exercised", permCmp, cardCmp)
	}
	// total over every game's table
	tot := map[string]*fpPair{}
	var order []string
	for _, ps := range perPos {
		for i := range ps {
			p := &ps[i]
			q := tot[p.group]
			if q == nil {
				c := *p
				tot[p.group] = &c
				order = append(order, p.group)
				continue
			}
			for j := range q.hits {
				q.hits[j] += p.hits[j]
				q.misses[j] += p.misses[j]
			}
		}
	}
	for _, n := range order {
		p := tot[n]
		pairs = append(pairs, *p)
		for idx, kind := range []fpKind{fpPerm, fpCard} {
			if p.kinds&kind == 0 {
				continue
			}
			if p.hits[idx] == 0 || p.misses[idx] == 0 {
				t.Errorf("%s (kind %d): the struct side was true %d and false %d times; the comparison proves nothing", n, kind, p.hits[idx], p.misses[idx])
			}
		}
	}
	t.Logf("%d positions, %d permanent and %d card comparisons over %d pairs", positions, permCmp, cardCmp, len(pairs))
}

// TestFilterProgTwoComparisons: power >= 2 AND mana value <= 3, the pair of
// numeric conjuncts one Filter has no slot for. The program equals the two
// struct filters both holding, and the conjunction differs from each half on
// some object (so the second literal is read, and so is the first).
func TestFilterProgTwoComparisons(t *testing.T) {
	type ids struct{ pw, mv, both, rev kop.FilterID }
	byTable := map[*kop.Table]ids{}
	var onlyPw, onlyMv, bothN, noneN int
	fpPositions(t, 200, 5, func(tbl *kop.Table) {
		if len(tbl.FilterProgs) == 0 {
			tbl.FilterProgs = append(tbl.FilterProgs, kop.FilterProg{})
		}
		var x ids
		add := func(f kop.Filter) kop.FilterID {
			tbl.Filters = append(tbl.Filters, f)
			return kop.FilterID(len(tbl.Filters) - 1)
		}
		prog := func(ls ...kop.FLit) kop.FilterID {
			tbl.FilterProgs = append(tbl.FilterProgs, kop.FilterProg{Lits: ls})
			return add(kop.Filter{Pred: kop.PredProg, PredN: int16(len(tbl.FilterProgs) - 1)})
		}
		pw := kop.FLit{W: kop.FwPowerCmp, Op: kop.CmpGE, V: kop.Const(2)}
		mv := kop.FLit{W: kop.FwCMCCmp, Op: kop.CmpLE, V: kop.Const(3)}
		x.pw = add(kop.Filter{Pred: kop.PredPowerGE, PredN: 2})
		x.mv = add(kop.Filter{Pred: kop.PredCMCLE, PredN: 3})
		x.both, x.rev = prog(pw, mv), prog(mv, pw)
		byTable[tbl] = x
	}, func(s *State) {
		k := s.k()
		x := byTable[k.t]
		note := func(a, b, got, rev bool, what string) {
			if got != (a && b) || rev != got {
				t.Fatalf("%s: power>=2 %v, cmc<=3 %v, program %v (reversed %v)", what, a, b, got, rev)
			}
			switch {
			case a && b:
				bothN++
			case a:
				onlyPw++
			case b:
				onlyMv++
			default:
				noneN++
			}
		}
		forObjects(s, func(sl Slot) {
			pl := &s.perms[sl]
			m := func(f kop.FilterID) bool { return s.matchPermOne(k, &k.t.Filters[f], sl, pl.ctrl, NoSlot, layerFull) }
			note(m(x.pw), m(x.mv), m(x.both), m(x.rev), "perm "+k.t.Cards[s.cardAt(sl)].Name)
		}, func(o Obj) {
			ob := &s.objs[o]
			m := func(f kop.FilterID) bool {
				return s.matchCardOne(k, &k.t.Filters[f], ob.card, ob.owner(), ob.owner(), o)
			}
			note(m(x.pw), m(x.mv), m(x.both), m(x.rev), "card "+k.t.Cards[ob.card].Name)
		})
	})
	if onlyPw == 0 || onlyMv == 0 || bothN == 0 || noneN == 0 {
		t.Fatalf("outcomes: both %d, power only %d, mana value only %d, neither %d; each must occur", bothN, onlyPw, onlyMv, noneN)
	}
}

// TestFilterProgNoAlloc: evaluating a program allocates nothing, in the
// permanent and the card matcher.
func TestFilterProgNoAlloc(t *testing.T) {
	sc, k, perm, card, fl := fpScenario(t)
	if !sc.s.matchPermOne(k, &fl.subPerm, sc.s.objs[perm].slot, 0, NoSlot, layerFull) {
		t.Fatal("precondition: the permanent has the subtype the program asks for")
	}
	if n := testing.AllocsPerRun(200, func() {
		sc.s.matchPermOne(k, &fl.subPerm, sc.s.objs[perm].slot, 0, NoSlot, layerFull)
	}); n != 0 {
		t.Errorf("permanent matcher: %v allocations per program, want 0", n)
	}
	if n := testing.AllocsPerRun(200, func() {
		sc.s.matchCardOne(k, &fl.subPerm, sc.s.objs[card].card, 0, 0, card)
	}); n != 0 {
		t.Errorf("card matcher: %v allocations per program, want 0", n)
	}
}

// fpFilters are the filters fpScenario adds.
type fpFilters struct {
	subPerm, subMiss kop.Filter // FwSubtype of the creature's subtype / one it lacks
	hasID, missID    kop.FilterID
}

// fpScenario is an FDN table with a creature (with a subtype) on the
// battlefield and the same card in hand, and the program filters that read
// its subtype.
func fpScenario(t *testing.T) (sc *scen, k *prep, perm, hand Obj, f fpFilters) {
	t.Helper()
	decks := benchDecks(t, "fdn")
	set := cardsettest.Set(t, "fdn")
	tbl, da, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ReleaseTable(tbl) })
	var card CardID = NoCard
	sub := -1
	for _, c := range da {
		cd := &tbl.Cards[c]
		if cd.Types&kop.TCreature == 0 {
			continue
		}
		for i := 0; i < len(tbl.Subtypes); i++ {
			if cd.Subtypes.Has(i) {
				card, sub = c, i
				break
			}
		}
		if sub >= 0 {
			break
		}
	}
	if card == NoCard {
		t.Skip("no creature with a subtype in the first bench deck")
	}
	miss := -1
	for i := 0; i < len(tbl.Subtypes); i++ {
		if !tbl.Cards[card].Subtypes.Has(i) {
			miss = i
			break
		}
	}
	if len(tbl.FilterProgs) == 0 {
		tbl.FilterProgs = append(tbl.FilterProgs, kop.FilterProg{})
	}
	prog := func(sub int) kop.FilterID {
		tbl.FilterProgs = append(tbl.FilterProgs, kop.FilterProg{Lits: []kop.FLit{{W: kop.FwSubtype, Arg: int16(sub)}}})
		tbl.Filters = append(tbl.Filters, kop.Filter{Pred: kop.PredProg, PredN: int16(len(tbl.FilterProgs) - 1)})
		return kop.FilterID(len(tbl.Filters) - 1)
	}
	f.hasID, f.missID = prog(sub), prog(miss)
	f.subPerm, f.subMiss = tbl.Filters[f.hasID], tbl.Filters[f.missID]
	sc = newScenTableLib(t, tbl, card, Config{NoElide: true})
	perm = sc.perm(card, 0)
	hand = sc.hand(card, 0)
	return sc, sc.k, perm, hand, f
}

// TestFilterProgReachesEveryMatcher: a PredProg filter used in each matcher
// kind reaches filterProg there: the permanent matcher, the card matcher for a
// card in hand and for a spell on the stack, and the layer-walk matcher. Each
// answers true for a subtype the object has and false for one it lacks, which
// a matcher that fell past PredProg would not. Holding a program also builds
// the table's genPrep, which the non-generated matchers (no arm for PredProg)
// never run without.
func TestFilterProgReachesEveryMatcher(t *testing.T) {
	sc, k, perm, hand, f := fpScenario(t)
	s := sc.s
	if k.gen == nil || !k.gen.uses.has(useFilterProg) {
		t.Fatalf("a table with a PredProg filter has no genPrep with useFilterProg: the non-generated matchers would run it")
	}
	sl := s.objs[perm].slot
	want := func(kind string, has, miss bool) {
		t.Helper()
		if !has || miss {
			t.Errorf("%s matcher: the subtype the object has -> %v, one it lacks -> %v; want true, false", kind, has, miss)
		}
	}
	has, miss := &k.t.Filters[f.hasID], &k.t.Filters[f.missID]
	want("permanent", s.matchPermOne(k, has, sl, 0, NoSlot, layerFull), s.matchPermOne(k, miss, sl, 0, NoSlot, layerFull))
	want("permanent (matchPerm)", s.matchPerm(k, f.hasID, sl, 0, NoSlot), s.matchPerm(k, f.missID, sl, 0, NoSlot))
	card := s.objs[hand].card
	want("card in hand", s.matchCardOne(k, has, card, 0, 0, hand), s.matchCardOne(k, miss, card, 0, 0, hand))
	want("card (matchCard)", s.matchCard(k, f.hasID, card, 0, 0), s.matchCard(k, f.missID, card, 0, 0))
	s.objs[hand].zone = kop.ZoneStack
	s.push(stackEntry{kind: seSpell, obj: hand, ctrl: 0, card: card, trigObj: NoObj})
	want("spell on the stack", s.matchCardOne(k, has, card, 0, 0, hand), s.matchCardOne(k, miss, card, 0, 0, hand))
	cur := dchars{subs: k.t.Cards[card].Subtypes}
	want("layer walk", s.matchCurOne(k, has, sl, 0, NoSlot, &cur), s.matchCurOne(k, miss, sl, 0, NoSlot, &cur))
}

// TestFilterProgRefusedAtPrep: a program the evaluation would trip over is
// refused when the table is prepared.
func TestFilterProgRefusedAtPrep(t *testing.T) {
	mk := func(lits ...kop.FLit) *kop.Table {
		return &kop.Table{
			Filters:     []kop.Filter{{}, {Pred: kop.PredProg, PredN: 1}},
			FilterProgs: []kop.FilterProg{{}, {Lits: lits}},
		}
	}
	if !tableHasFilterProg(mk(kop.FLit{W: kop.FwTapped})) {
		t.Fatal("a table with a program filter is not reported")
	}
	if tableHasFilterProg(&kop.Table{Filters: []kop.Filter{{}, {Pred: kop.PredPowerLE, PredN: 1}}}) {
		t.Fatal("a table with no program filter is reported")
	}
	for _, c := range []struct {
		name string
		tbl  *kop.Table
		want string
	}{
		{"no word", mk(kop.FLit{}), "no handler"},
		{"unknown word", mk(kop.FLit{W: kop.FilterWord(65535)}), "no handler"},
		{"index 0", &kop.Table{Filters: []kop.Filter{{}, {Pred: kop.PredProg}}, FilterProgs: []kop.FilterProg{{}}}, "outside Table.FilterProgs"},
		{"index past the end", &kop.Table{Filters: []kop.Filter{{}, {Pred: kop.PredProg, PredN: 2}}, FilterProgs: []kop.FilterProg{{}, {}}}, "outside Table.FilterProgs"},
		{"nested filter", mk(kop.FLit{W: kop.FwTapped, F: 9}), "nested filter"},
	} {
		func() {
			defer func() {
				r := recover()
				if s, _ := r.(string); !strings.Contains(s, c.want) {
					t.Errorf("%s: recovered %v, want a panic containing %q", c.name, r, c.want)
				}
			}()
			tableHasFilterProg(c.tbl)
		}()
	}
}
