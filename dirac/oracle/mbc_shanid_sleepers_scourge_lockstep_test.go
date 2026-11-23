package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/deck"
)

// CR 605.3b with CR 601.2g: Shanid, Sleepers' Scourge's owed evidence in
// dr-mbc-abbb-s000 (the loop's pair 38, seed 644180) was held by an
// options/options divergence. The loop's decks hold Starfleet Academy, whose
// "{5},{T}: Add X mana of one chosen colour" is a paid dynamic amount gorge's
// 160-node prefix search cannot price while the kernel's open any-colour
// production can — so gorge withholds the casts it funds and the kernel offers
// them, the very gap dirac/support/refused/starfleet-academy.json documents and
// refuses by name (TestMBCStarfleetAcademyReprosLockstep pins the gap itself
// live). gorge is wrong per CR 601.2g (a legal cast's payment may use that
// mana); the fix past a bounded change is the refusal, which landed after the
// loop's evidence run and takes Shanid's instance with it. With the card
// refused the pool's decks cannot hold it, so the divergence is unreachable:
// this replays the loop's pair the same dealing machinery deals without the
// refused enabler — Shanid with its six remaining legendary lands, whose plays
// trigger its "whenever you play a legendary land or cast a legendary spell,
// you draw a card and you lose 1 life" — and requires the game clean (gorge
// leads, full surface, the run's mulligans and caps).
func TestMBCShanidSleepersScourgeReprosLockstep(t *testing.T) {
	t.Parallel()
	const (
		pool      = "dr-mbc-abbb"
		seed      = 644180
		perDeck   = 6
		pair      = 38
		nDecks    = 400
		mulligans = 1
	)
	reg := synthReg(t)
	set, extras, err := shanidSleepersScourgeUnion()
	if err != nil {
		t.Fatal(err)
	}
	// the evidence run's -only: of the unverified cards, Shanid alone plays.
	set = cardset.AllowNamed(set, map[string]bool{"Shanid, Sleepers' Scourge": true})
	focus, err := ParseFocus([]byte(shanidSleepersScourgeFocus))
	if err != nil {
		t.Fatal(err)
	}
	poolDecks, files, err := FocusPoolWith(reg, set, extras, pool, focus, perDeck, seed, nDecks)
	if err != nil {
		t.Fatal(err)
	}
	p := PairingAt(poolDecks, pair, seed)
	// Precondition: both decks hold Shanid, the owed card the pair replays.
	for _, n := range p.Decks() {
		f, ok := files[n]
		if !ok {
			t.Fatalf("pair %d deck %s was not dealt", pair, n)
		}
		if !slices.ContainsFunc(f.Cards, func(e deck.Entry) bool { return e.Name == "Shanid, Sleepers' Scourge" }) {
			t.Fatalf("pair %d deck %s holds no Shanid, Sleepers' Scourge", pair, n)
		}
	}
	c, err := OpenCorpus(corpusDirOrSkip(t), "")
	if err != nil {
		t.Fatal(err)
	}
	c.AddDecks(files)
	m, tbl, err := SetMatchupN(set, c, p.Decks(), mulligans)
	if err != nil {
		t.Fatalf("pair %d refused: %v", pair, err)
	}
	rc := RunConfig{Config: Config{}, MaxDecisions: 20000, MaxTurns: 100}
	k, err := SetSide(set, tbl, rc.Config)(m, p.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), rc)
	if res.Div != nil {
		t.Fatalf("divergence at decision %d: %s %s", res.Div.At, res.Div.What, res.Div.Field)
	}
	if res.Stall != "" {
		t.Fatalf("stall: %s", res.Stall)
	}
	if res.Decisions == 0 {
		t.Fatal("no decisions were compared: the pair replayed no game")
	}
}

// shanidSleepersScourgeFocus is the evidence loop's ev-dr-mbc-abbb-s000 focus
// for Shanid, Sleepers' Scourge with Starfleet Academy dropped: kgen refuses it
// (735bcb96f2), and a focus list naming a card the pools do not play fails
// loudly (focus.go).
const shanidSleepersScourgeFocus = `# ev-dr-mbc-abbb-s000 minus Starfleet Academy (refused by name)
Shanid, Sleepers' Scourge	Three Tree City;Academy Ruins;Mikokoro, Center of the Sea;Eiganjo, Seat of the Empire;Gemstone Caverns;Minamo, School at Water's Edge;Otawara, Soaring City
`

// shanidWithPools is the evidence run's -synth-with list: the pools its focus
// takes enablers the main pool lacks from, first holder wins.
var shanidWithPools = []string{"std-blb", "std-trc", "std-trk", "dr-mbc-abba", "tail-03", "tail-04"}

// shanidSleepersScourgeUnion is the run's set, dr-mbc-abbb widened by the focus
// enablers it lacks, each taken from the first with-pool holding it, lowered as
// one union table (kfuzz -synth-with's synthWithSet). It returns the union and
// the lent names.
func shanidSleepersScourgeUnion() (cardset.Set, map[string]bool, error) {
	main, err := cardset.SynthByName("dr-mbc-abbb")
	if err != nil {
		return nil, nil, err
	}
	focus, err := ParseFocus([]byte(shanidSleepersScourgeFocus))
	if err != nil {
		return nil, nil, err
	}
	held := func(s cardset.Set, n string) bool {
		if _, ok := s.Lookup(n); ok {
			return true
		}
		f, _, _ := strings.Cut(n, " // ")
		for _, m := range s.Names()[:s.NumCards()] {
			if mf, _, _ := strings.Cut(m, " // "); mf == f {
				return true
			}
		}
		return false
	}
	extras := map[string]bool{}
	for _, fc := range focus {
		for _, n := range fc.With {
			if held(main, n) {
				continue
			}
			for _, w := range shanidWithPools {
				ws, err := cardset.SynthByName(w)
				if err != nil {
					return nil, nil, err
				}
				if held(ws, n) {
					extras[n] = true
					break
				}
			}
		}
	}
	if len(extras) == 0 {
		return main, nil, nil
	}
	names := slices.Clone(main.Names()[:main.NumCards()])
	for n := range extras {
		names = append(names, n)
	}
	slices.Sort(names)
	dir, err := cardset.CorpusDir()
	if err != nil {
		return nil, nil, err
	}
	d, err := kgen.SynthUnionTable(names, dir, "dr-mbc-abbb")
	if err != nil {
		return nil, nil, err
	}
	return cardset.Raw(cardset.NewDataSet(d)), extras, nil
}
