package oracle

import (
	"encoding/json"
	"math/bits"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/events"
)

// synthReg is the corpus for a synth test, skipping as the tables do when there is none.
func synthReg(t *testing.T) *cards.Registry {
	t.Helper()
	dir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

// stdPools is the std-* pools, each whole Standard list or its pieces.
func stdPools(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, n := range cardsettest.SynthNames(t) {
		if strings.HasPrefix(n, "std-") {
			out = append(out, n)
		}
	}
	return out
}

func TestSynthPoolIsDeterministic(t *testing.T) {
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tdm")
	render := func(seed uint64) []byte {
		pool, files, err := SynthPool(reg, set, seed, SynthDecks)
		if err != nil {
			t.Fatal(err)
		}
		var out []byte
		for _, d := range pool {
			b, err := json.Marshal(files[d.Name])
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, d.Name...)
			out = append(out, b...)
		}
		return out
	}
	a, b := render(EvidenceSeed), render(EvidenceSeed)
	if !slices.Equal(a, b) {
		t.Error("two deals from one seed differ")
	}
	if c := render(EvidenceSeed + 1); slices.Equal(a, c) {
		t.Error("another seed dealt the same decks: the seed is ignored")
	}
}

// TestSynthEveryCardAppears: a lowered card the deal cannot reach would have no
// evidence and nothing to say why. One with no main-deck type has no slot.
func TestSynthEveryCardAppears(t *testing.T) {
	reg := synthReg(t)
	const floor = 10
	wide := 0
	for _, name := range stdPools(t) {
		set := cardsettest.Synth(t, name)
		pool, files, err := SynthPool(reg, set, EvidenceSeed, SynthDecks)
		if err != nil {
			t.Fatal(err)
		}
		decks := map[string]int{}
		for _, d := range pool {
			if len(d.Colors) > 2 {
				wide++
			}
			for _, e := range files[d.Name].Cards {
				decks[e.Name]++
			}
		}
		supported := set.Supported()
		for i, n := range set.Names()[:set.NumCards()] {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s: %q is not in the corpus", name, n)
			}
			switch {
			case !supported[i]:
				if decks[n] != 0 {
					t.Errorf("%s: unsupported %q was dealt", name, n)
				}
			case kindOf(c) == synthUndealable:
				if decks[n] != 0 {
					t.Errorf("%s: %q has no main-deck type and was dealt", name, n)
				}
			case decks[n] < floor:
				t.Errorf("%s: %q (%s) is in %d decks, want at least %d", name, n, maskString(pipMask(c)), decks[n], floor)
			}
		}
	}
	if wide == 0 {
		t.Error("no deck took a wider colour set: three-colour cards reached no deck by the rule under test")
	}
}

// refusing is a set with one more card refused by name.
type refusing struct {
	cardset.Set
	card int
}

func (r refusing) Status(i int) cardset.Status {
	if i == r.card {
		return cardset.StatusRefused
	}
	return r.Set.Status(i)
}

// TestRefusalMovesNoDeal: a set that plays the refused cards (the run that settles the
// list, and every test pinning a pair) deals the same decks whatever the list says, or a
// refused entry's repro would name a different game.
func TestRefusalMovesNoDeal(t *testing.T) {
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-dsk")
	dealt := func(s cardset.Set) ([]PoolDeck, map[string]deck.File) {
		pool, files, err := SynthPool(reg, s, EvidenceSeed, SynthDecks)
		if err != nil {
			t.Fatal(err)
		}
		return pool, files
	}
	pool, files := dealt(set)
	refused := 0
	for i, n := range set.Names()[:set.NumCards()] {
		if set.Status(i) == cardset.StatusNotLowered || cardset.IsSynthBasic(n) || set.Status(i) == cardset.StatusRefused {
			continue
		}
		pool2, files2 := dealt(refusing{set, i})
		if !slices.Equal(pool, pool2) {
			t.Fatalf("refusing %s moved the pool's decks", n)
		}
		for p := range SynthPairs {
			a, b := PairingAt(pool, p, EvidenceSeed), PairingAt(pool2, p, EvidenceSeed)
			if a != b {
				t.Fatalf("refusing %s moved pair %d: %v, %v", n, p, a, b)
			}
			for _, name := range []string{a.A, a.B} {
				if !slices.Equal(files[name].Cards, files2[name].Cards) {
					t.Fatalf("refusing %s changed deck %s in pair %d", n, name, p)
				}
			}
		}
		if refused++; refused == 5 {
			return
		}
	}
	t.Fatal("std-dsk has no card to refuse")
}

// TestSynthDealsNoRefusedCard: a set that turns refused cards away deals none of them, so
// no pair of the evidence run is lost to one, and still deals every other lowered card,
// unverified ones included.
func TestSynthDealsNoRefusedCard(t *testing.T) {
	reg := synthReg(t)
	for _, name := range []string{"std-tla", "std-dsk"} {
		set := cardsettest.Synth(t, name)
		view := cardset.AllowUnverified(set)
		pool, files, err := SynthPool(reg, view, EvidenceSeed, SynthDecks)
		if err != nil {
			t.Fatal(err)
		}
		held := map[string]int{}
		for _, d := range pool {
			for _, e := range files[d.Name].Cards {
				held[e.Name]++
			}
		}
		refused := 0
		for i, n := range view.Names()[:view.NumCards()] {
			switch st := view.Status(i); {
			case st == cardset.StatusRefused:
				refused++
				if held[n] != 0 {
					t.Errorf("%s: refused %q was dealt into %d decks", name, n, held[n])
				}
			case st == cardset.StatusUnverified && kindOf(mustLookup(t, reg, n)) != synthUndealable && held[n] == 0:
				t.Errorf("%s: unverified %q was dealt into no deck", name, n)
			}
		}
		if refused == 0 {
			t.Errorf("%s has no refused card to leave out", name)
		}
	}
}

func mustLookup(t *testing.T, reg *cards.Registry, name string) *cards.Card {
	t.Helper()
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("%q is not in the corpus", name)
	}
	return c
}

func TestSynthDecksAre40(t *testing.T) {
	reg := synthReg(t)
	for _, name := range []string{"std-tdm", "std-big", "std-dsk"} {
		set := cardsettest.Synth(t, name)
		pool, files, err := SynthPool(reg, set, EvidenceSeed, SynthDecks)
		if err != nil {
			t.Fatal(err)
		}
		if len(pool) != SynthDecks || len(files) != SynthDecks {
			t.Fatalf("%s: %d decks, %d files", name, len(pool), len(files))
		}
		for _, d := range pool {
			f := files[d.Name]
			sum, nonbasic := 0, 0
			for _, e := range f.Cards {
				sum += e.Count
				if !cardset.IsSynthBasic(e.Name) {
					nonbasic += e.Count
					c, ok := reg.Lookup(e.Name)
					if !ok {
						t.Fatalf("%s: %q not in the corpus", d.Name, e.Name)
					}
					if kindOf(c) == synthSpell && e.Count > synthMaxCopies {
						t.Errorf("%s: %d copies of %q", d.Name, e.Count, e.Name)
					}
					if m := pipMask(c); kindOf(c) == synthSpell && m&^colourMask(d.Colors) != 0 {
						t.Errorf("%s: %q needs %s", d.Name, e.Name, maskString(m))
					}
				}
			}
			if sum != synthDeckSize {
				t.Errorf("%s: %d cards", d.Name, sum)
			}
			if nonbasic > synthSpells+synthLands {
				t.Errorf("%s: %d nonbasic cards", d.Name, nonbasic)
			}
			if !slices.IsSortedFunc(f.Cards, func(a, b deck.Entry) int { return strings.Compare(a.Name, b.Name) }) {
				t.Errorf("%s: entries are not in name order", d.Name)
			}
			if bits.OnesCount8(colourMask(d.Colors)) < 2 {
				t.Errorf("%s: colours %q", d.Name, d.Colors)
			}
		}
	}
}

// TestSynthDealsRooms: a set list spells a Room "A // B"; dealing it means the pool
// holds it under the name a deck uses, and the game can build a matchup from it.
func TestSynthDealsRooms(t *testing.T) {
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-dsk")
	raw, err := os.ReadFile(filepath.Join(setlistsDir(t), "DSK.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var rooms []string
	for _, l := range strings.Split(string(raw), "\n") {
		if front, _, ok := strings.Cut(l, " // "); ok {
			rooms = append(rooms, front)
		}
	}
	if len(rooms) < 15 {
		t.Fatalf("%d Rooms in DSK's list, want the set's 15 or more", len(rooms))
	}
	pool, files, err := SynthPool(reg, set, EvidenceSeed, SynthDecks)
	if err != nil {
		t.Fatal(err)
	}
	dealt := map[string]int{}
	for _, d := range pool {
		for _, e := range files[d.Name].Cards {
			dealt[e.Name]++
		}
	}
	supported := set.Supported()
	reached := 0
	for _, r := range rooms {
		i, ok := set.Lookup(r)
		if !ok {
			t.Errorf("%q is not in std-dsk under its front name", r)
			continue
		}
		if supported[i] {
			if dealt[r] == 0 {
				t.Errorf("supported Room %q was not dealt", r)
			}
			reached++
		}
	}
	if reached == 0 {
		t.Error("no Room is supported, so this test proves nothing")
	}
}

func setlistsDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "..", "cardset", "setlists")
}

// TestTailBucketsPartitionAll: every corpus card is in Standard, in exactly one tail
// pool, or named as not lowered. So no card can silently fall out of the corpus.
func TestTailBucketsPartitionAll(t *testing.T) {
	reg := synthReg(t)
	names := cardsettest.SynthNames(t)
	where := map[string]string{}
	inStd := map[string]bool{}
	for _, n := range cardset.SynthBasics {
		inStd[n] = true
	}
	buckets := 0
	for _, pool := range names {
		set := cardsettest.Synth(t, pool)
		std := strings.HasPrefix(pool, "std-")
		if !std {
			buckets++
			if set.NumCards() > cardset.TailBucket+len(cardset.SynthBasics) {
				t.Errorf("%s holds %d cards", pool, set.NumCards())
			}
		}
		for _, n := range set.Names()[:set.NumCards()] {
			switch {
			case cardset.IsSynthBasic(n):
			case std:
				if inStd[n] && where[n] != "" {
					t.Errorf("%q is in %s and %s", n, pool, where[n])
				}
				inStd[n] = true
			case where[n] != "":
				t.Errorf("%q is in %s and %s", n, pool, where[n])
			}
			if !cardset.IsSynthBasic(n) && !std {
				where[n] = pool
			}
		}
	}
	unlowered, err := cardset.SynthTailUnlowered()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.IsSorted(unlowered) || len(unlowered) == 0 {
		t.Errorf("%d unlowered names, sorted %v", len(unlowered), slices.IsSorted(unlowered))
	}
	for _, n := range unlowered {
		if where[n] != "" {
			t.Errorf("%q is both unlowered and in %s", n, where[n])
		}
		if inStd[n] {
			t.Errorf("%q is both unlowered and Standard", n)
		}
		where[n] = "unlowered"
	}
	missing, total := 0, 0
	for _, c := range reg.AllCards() {
		n := coverageName(c)
		if n == "" {
			continue
		}
		total++
		if !inStd[n] && where[n] == "" {
			if missing++; missing <= 10 {
				t.Errorf("%q is in no pool", n)
			}
		}
	}
	for n := range where {
		if _, ok := reg.Lookup(n); !ok {
			t.Errorf("%q in %s is not in the corpus", n, where[n])
		}
	}
	if missing > 0 {
		t.Errorf("%d of %d corpus cards are in no pool", missing, total)
	}
	if inStd["Plains"] != true || buckets < 50 {
		t.Errorf("%d tail buckets: the corpus looks wrong", buckets)
	}
	for n := range inStd {
		if where[n] != "" && !cardset.IsSynthBasic(n) {
			t.Errorf("%q is Standard and in %s", n, where[n])
		}
	}
}

// fakeReachSet is the part of a table SynthReach reads.
type fakeReachSet struct {
	cardset.Set
	names  []string
	status []cardset.Status
}

func (s fakeReachSet) Name() string                { return "std-x" }
func (s fakeReachSet) Names() []string             { return s.names }
func (s fakeReachSet) NumCards() int               { return len(s.names) }
func (s fakeReachSet) Status(i int) cardset.Status { return s.status[i] }
func (s fakeReachSet) Reason(int) string           { return "" }

// TestSynthReachReportsEveryStage: each stage of the report counts what it says, from
// a pool with one card stopped at each.
func TestSynthReachReportsEveryStage(t *testing.T) {
	set := fakeReachSet{
		names: []string{"A Evidence", "B Keys", "C Few", "D NoGame", "E NotDealt", "F NotLowered", "Plains"},
		status: []cardset.Status{cardset.StatusUnverified, cardset.StatusUnverified, cardset.StatusUnverified, cardset.StatusUnverified,
			cardset.StatusUnverified, cardset.StatusNotLowered, cardset.StatusUnverified},
	}
	entry := func(names ...string) (out []deck.Entry) {
		for _, n := range names {
			out = append(out, deck.Entry{Name: n, Count: 1})
		}
		return out
	}
	files := map[string]deck.File{
		"d0": {Cards: entry("A Evidence", "B Keys", "Plains")},
		"d1": {Cards: entry("C Few", "Plains")},
		"d2": {Cards: entry("D NoGame", "Plains")},
	}
	pool := []PoolDeck{{Name: "d0"}, {Name: "d1"}, {Name: "d2"}}
	cov := NewCoverage()
	cov.Cards["A Evidence"] = &CardCoverage{Cast: EvidenceMinPlays, Used: map[string]int{}}
	cov.Cards["B Keys"] = &CardCoverage{Cast: EvidenceMinPlays, Keys: []string{"f0/a0"}, Used: map[string]int{}}
	cov.Cards["C Few"] = &CardCoverage{Cast: EvidenceMinPlays - 1, Used: map[string]int{}}
	r := SynthReach(set, pool, files, [][2]string{{"d0", "d1"}}, cov)
	if r.List != 6 || r.Lowered != 5 || r.Dealt != 4 || r.Appeared != 3 || r.MinPlays != 2 || r.Evident != 1 || r.Games != 1 {
		t.Errorf("reach = %+v", r)
	}
	want := map[string]string{"A Evidence": ReachEvidence, "B Keys": ReachKeys, "C Few": ReachFewPlays, "D NoGame": ReachNoGame, "E NotDealt": ReachNotDealt, "F NotLowered": ReachNotLowered}
	for _, c := range r.Cards {
		if want[c.Name] != c.Status {
			t.Errorf("%s: status %s, want %s", c.Name, c.Status, want[c.Name])
		}
	}
	if l := r.Line(); !strings.Contains(l, "6 cards in the list") || !strings.Contains(l, "1 evidence") {
		t.Errorf("line: %s", l)
	}
	if tsv := string(r.TSV()); !strings.Contains(tsv, "E NotDealt\tnot-dealt\t0\t0\t0\n") || strings.Contains(tsv, "Plains") {
		t.Errorf("TSV:\n%s", tsv)
	}
	// With no clean game a played card has no evidence and no plays, and still appeared.
	r = SynthReach(set, pool, files, [][2]string{{"d0", "d1"}}, nil)
	if r.Appeared != 3 || r.MinPlays != 0 || r.Evident != 0 {
		t.Errorf("no clean game: %+v", r)
	}
}

// TestEachPlayerDiscardMovesTogether: each player chooses a discard with the
// earlier player's cards still in hand, then everything moves at once (CR 101.4).
func TestEachPlayerDiscardMovesTogether(t *testing.T) {
	synthPairs(t, "std-dsk", []int{21, 36, 44}, nil)
	synthPairs(t, "std-fin", []int{1, 12}, nil)
	synthPairs(t, "std-otj", []int{2}, nil)
	synthPairs(t, "std-woe", []int{84}, nil)
}

// TestWaterbendAskRequiresTheTapsThePaymentNeeds: Done waits for the taps the mana
// window can't do without (CR 601.2h, 701.67a). The pairs are ones that diverge when
// the ask offers Done at once, so each carries the case.
func TestWaterbendAskRequiresTheTapsThePaymentNeeds(t *testing.T) {
	waterbendPairs(t, "std-tla", []int{0, 1, 16, 18})
}

// TestWaterbendIncompleteCensusKeepsTheOpenAsk: with a restricted mana source the
// census doesn't price, the ask requires nothing and the target ask keeps its offer.
// The pairs are ones that diverge when the census is taken as complete.
func TestWaterbendIncompleteCensusKeepsTheOpenAsk(t *testing.T) {
	waterbendPairs(t, "std-tla", []int{66, 105, 108, 116})
}

// waterbendPairs is synthPairs requiring a waterbend ask in each game: a pair is
// two decks dealt from the lowered list, so a card lowered later moves what a pair
// holds, and a pin whose pair no longer waterbends would pass having tested nothing.
func waterbendPairs(t *testing.T, name string, pairs []int) {
	t.Helper()
	asks := 0
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}, Check: func(ref *GorgeSide, _ Side) *Divergence {
		if d := ref.Pending(); d != nil {
			for _, o := range d.Options {
				if strings.HasPrefix(o.Kind, "waterbend") {
					asks++
					break
				}
			}
		}
		return nil
	}}
	synthPairsRun(t, name, rc, pairs, func(pair int, _ *Result) {
		if asks == 0 {
			t.Errorf("pair %d of %s poses no waterbend ask: the pin tests nothing", pair, name)
		}
		asks = 0
	})
}

// TestAbilityPlanLeavesOutCommittedPermanents: the settled cost's plan is made
// without the permanents tapped for the ability's costs, and stands if others pay
// (CR 601.2g-h, 602.2b).
func TestAbilityPlanLeavesOutCommittedPermanents(t *testing.T) {
	synthPairs(t, "std-tla", []int{240}, nil)
}

// TestGrantedManaPricesAnImpendedCast: the search through granted mana prices an
// impended cast at its alternative cost (CR 118.9, 702.176a).
func TestGrantedManaPricesAnImpendedCast(t *testing.T) {
	synthPairs(t, "std-dsk", []int{11, 43, 292, 479}, nil)
}

// TestShuffledPairReachesTheLibraryInOrder: Floodpits Drowner and its target reach
// the library in the order the shuffle reads (CR 701.24).
func TestShuffledPairReachesTheLibraryInOrder(t *testing.T) {
	synthPairs(t, "std-dsk", []int{7, 206, 298}, nil)
}

// TestOverrideValueKeepsItsCountedSubtype: PuPu UFO's "base power becomes the number
// of Towns you control" (CR 613.4b) counts a subtype only that value names, so the
// matchup's table keeps it.
func TestOverrideValueKeepsItsCountedSubtype(t *testing.T) {
	synthPairs(t, "std-fin", []int{0, 2, 6, 14}, nil)
}

// TestRestrictedSourceStaysInTheManualWindow: a restricted any-colour ability keeps
// its source offered in the window whatever is being paid for (CR 106.6, 601.2g).
func TestRestrictedSourceStaysInTheManualWindow(t *testing.T) {
	synthPairs(t, "std-ecl", []int{40, 111, 125}, nil)
}

// TestPowerUpOfferCountsRestrictedSources: the sources a power-up's printed cost is
// priced against include a restricted one its activation may spend (CR 106.6, 702.193b).
func TestPowerUpOfferCountsRestrictedSources(t *testing.T) {
	restrictedPowerUpPairs(t, "std-msh", []int{11, 88, 98, 112})
}

// restrictedPowerUpPairs is synthPairs requiring that each game activates a power-up
// after restricted mana has been made: a pair is two decks dealt from the lowered list,
// so a card lowered later moves what a pair holds, and a pin whose pair no longer
// has both would pass having tested nothing.
func restrictedPowerUpPairs(t *testing.T, name string, pairs []int) {
	t.Helper()
	seen, hits, armed := 0, 0, false
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}, Check: func(ref *GorgeSide, _ Side) *Divergence {
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := &evs[seen]
			switch ev.Kind {
			case events.ManaAdd:
				if ev.Amount > 0 && strings.HasPrefix(ev.Text, "mana-restriction:") {
					armed = true
				}
			case events.AbilityPush:
				if o := ref.E.G.Obj(ev.Obj); armed && o != nil && o.Face() != nil && int(ev.Amount) < len(o.Face().Abilities) &&
					o.Face().Abilities[ev.Amount].Params["PowerUp"] == "True" {
					hits++
				}
			}
		}
		return nil
	}}
	synthPairsRun(t, name, rc, pairs, func(pair int, _ *Result) {
		if hits == 0 {
			t.Errorf("pair %d of %s activates no power-up beside restricted mana: the pin tests nothing", pair, name)
		}
		seen, hits, armed = 0, 0, false
	})
}

// TestHandAbilityOfferCountsRestrictedSources: cycling and the other abilities that
// work from a hand may spend a restricted source that admits them (CR 106.6, 702.29).
func TestHandAbilityOfferCountsRestrictedSources(t *testing.T) {
	synthPairs(t, "std-dft", []int{127, 443}, nil)
	synthPairs(t, "std-msh", []int{7, 415}, nil)
}

// TestBiteHitsEachObjectOfItsTargetSlot: Betrayal at the Vault's creature deals its
// power to each of two other targets, not the first alone (CR 120.3, 608.2b).
func TestBiteHitsEachObjectOfItsTargetSlot(t *testing.T) {
	synthPairs(t, "std-otj", []int{11, 32, 61, 226}, nil)
}

// TestStrandedPaymentReversesFirst: a helper or tap-cost answer that leaves too little
// mana is undone before any mana is activated (CR 601.2g-h, 602.2b, 733.1).
func TestStrandedPaymentReversesFirst(t *testing.T) {
	synthPairs(t, "std-ecl", []int{14, 200, 252, 353, 358}, nil)
	synthPairs(t, "std-dsk", []int{29}, nil)
}

// TestStateBasedActionsRepeatAfterTriggerPlacement: a final chapter removed for want of
// a legal target leaves its Saga to be sacrificed before anyone receives priority
// (CR 117.5, 603.3d, 714.4).
func TestStateBasedActionsRepeatAfterTriggerPlacement(t *testing.T) {
	synthPairs(t, "std-fin", []int{8, 18, 32, 33, 38, 42}, nil)
	synthPairs(t, "std-spm", []int{93, 101, 310, 320}, nil)
}

// TestEnteringPowerIsTheLivePermanents: "draw cards equal to its power" reads the
// permanent with its entry counters, not the card that was cast (CR 608.2h; Prime
// Speaker Zegana).
func TestEnteringPowerIsTheLivePermanents(t *testing.T) {
	synthPairs(t, "std-fdn", []int{6, 482}, nil)
}

// TestWindowCensusSkipsWhatThePaymentCannotSpend: a tapped restricted source, or an
// untapped one whose mana this payment can't spend, leaves the census complete, so a
// waterbend ask still takes the taps the payment needs (CR 106.6, 601.2g, 701.67a).
// The pairs diverge when a tapped restricted source (66, 414, 446, 503) or one whose
// mana the payment can't spend (66, 171, 760, 949) is counted, and each must pose a
// waterbend ask.
func TestWindowCensusSkipsWhatThePaymentCannotSpend(t *testing.T) {
	waterbendPairs(t, "std-tla", []int{66, 171, 414, 446, 503, 760, 949})
}

// manaTapTriggerPairs is synthPairs requiring that a mana tap in each game puts a Taps
// trigger on gorge's stack: a pair is two decks dealt from the lowered list, so a card
// lowered later moves what a pair holds.
func manaTapTriggerPairs(t *testing.T, name string, pairs []int) {
	t.Helper()
	seen, hits := 0, 0
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}, Check: func(ref *GorgeSide, _ Side) *Divergence {
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			if evs[seen].Kind.String() != "trigger_push" {
				continue
			}
			mana := false
			for j := seen - 1; j >= max(0, seen-6); j-- {
				switch k := evs[j].Kind.String(); {
				case k == "mana_add":
					mana = true
				case k == "tap" && evs[j].Obj == evs[seen].Obj && mana:
					hits++
				}
			}
		}
		return nil
	}}
	synthPairsRun(t, name, rc, pairs, func(pair int, _ *Result) {
		if hits == 0 {
			t.Errorf("pair %d of %s taps no Taps listener for mana: the pin tests nothing", pair, name)
		}
		seen, hits = 0, 0
	})
}

// TestGrantedManaTapFiresTapsTriggers: a creature tapped for a granted mana ability
// (Great Divide Guide gives every Ally one) becomes tapped, so its "whenever this
// becomes tapped" trigger (Master Pakku, Gran-Gran, Compassionate Healer) fires
// whether the tap is in the script before the play or in the payment (CR 603.2,
// 605.1b). The pairs diverge when a mana tap fires nothing.
func TestGrantedManaTapFiresTapsTriggers(t *testing.T) {
	manaTapTriggerPairs(t, "std-tla", []int{40, 121, 185, 473, 532})
}

// TestScriptTriggerWithATargetIsPlacedBeforeThePlay: a Taps trigger with a target
// that a script's mana tap causes (Master Pakku's mill) is answered by the script and
// stands on the stack below the play it was a prefix to, as gorge answers each prefix
// activation as its own decision (CR 603.3). The pairs diverge when it is left to be
// placed above the play.
func TestScriptTriggerWithATargetIsPlacedBeforeThePlay(t *testing.T) {
	manaTapTriggerPairs(t, "std-tla", []int{40, 185, 473, 532})
}
