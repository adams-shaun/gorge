package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// synthReproLockstep replays a recorded game on a synthetic pool's table, past its
// script, and fails on any divergence. A refused card plays: the fixture names the
// game that found a difference, whatever the list says today.
func synthReproLockstep(t *testing.T, file, pool string, cfg Config) {
	t.Helper()
	r, err := ReadRepro("testdata/" + file + ".repro.json")
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, pool)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		var es []cardset.Entry
		for _, n := range names {
			es = append(es, cardset.Entry{Name: n, Count: 1})
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	rc := RunConfig{Config: cfg, MaxDecisions: len(r.Actions) + 200}
	script := &ScriptPolicy{Actions: r.Actions}
	res := Lockstep(m, r.Seed, k, script, rc)
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s\n gorge %v\n dirac %v\n %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.RefOptions, res.Div.KerOptions, res.Div.Diff)
	}
}

// A trigger exiling the object that fired it, then returning it: Salvation Swan and a
// Bird entering, S.H.I.E.L.D. Flying Car's trigger copied onto the crewed Car. Both
// engines return the object at the end step, and its entering trigger follows.
func TestDelayedReturnOfTheObjectItsTriggerExiledLockstep(t *testing.T) {
	t.Parallel()
	autopay := Config{GorgeAutopaySurface: true}
	synthReproLockstep(t, "swan_exiles_its_bird", "std-blb", autopay)
	synthReproLockstep(t, "flying_car_exiles_itself", "std-msh", autopay)
}

// Zurgo's Vanguard is */3, and Fresh Start takes its power's CDA away: it stays a -5/3
// on the battlefield, whose toughness the lost ability did not set.
func TestLostCDALeavesThePrintedToughnessLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "lost_cda_toughness", "std-tdm", Config{GorgeAutopaySurface: true})
}

// A waterbend ability priced by its credit under a granted mana source: Geyser Leaper's
// credit covers the {4}, and Flexible Waterbender's {3} needs one tap of Great Divide
// Guide's granted mana, the prefix gorge's search finds.
func TestWaterbendCreditInTheGrantedPrefixSearchLockstep(t *testing.T) {
	t.Parallel()
	autopay := Config{GorgeAutopaySurface: true}
	synthReproLockstep(t, "waterbend_credit_covers", "std-tla", autopay)
	synthReproLockstep(t, "waterbend_granted_prefix", "std-tla", autopay)
}

// Ygra grants Maha its Food ability through an override static. With {2} floating after
// an {X} cast, both engines offer it.
func TestOverrideGrantedAbilityFromThePoolLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "ygra_food_from_pool", "std-blb", Config{})
}

// Flexible Waterbender carrying Trusty Boomerang: both engines price its printed
// waterbend beside the Boomerang's granted ability, whose Ability it shares in gorge.
func TestPrintedAbilityBesideAGrantLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "printed_beside_equipment_grant", "std-tla", Config{GorgeAutopaySurface: true})
}

// On the autopay surface a Gift spell is an ordinary plain cast: the promise costs no
// mana and is asked after the planner's witness is tapped.
func TestGiftCastIsPlannedOnAutopayLockstep(t *testing.T) {
	t.Parallel()
	cfg := Config{GorgeAutopaySurface: true}
	c := &Corpus{Reg: synthReg(t), decks: map[string][]*cards.Card{}}
	set := cardsettest.Synth(t, "std-blb")
	pool, files, err := SynthPool(c.Reg, set, 1, SynthDecks)
	if err != nil {
		t.Fatal(err)
	}
	c.AddDecks(files)
	cast := 0
	for _, i := range []int{37, 243, 253} {
		p := PairingAt(pool, i, 1)
		m, tbl, err := SetMatchup(set, c, p.A, p.B, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, p.Seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 100})
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("pair %d diverged at %d: %s %s", i, res.Div.At, res.Div.What, res.Div.Field)
		}
		for _, a := range res.Actions {
			if s := a.String(); strings.HasPrefix(s, "cast(") {
				if n := firstObjectCard(m, s); n == "Longstalk Brawl" || n == "Scrapshooter" {
					cast++
				}
			}
		}
	}
	if cast == 0 {
		t.Error("no pair cast a Gift spell")
	}
}

// A land animated the turn it entered is no creature that entered: Mishra's Factory
// leaves Bristlebane Outrider's "another creature entered" unmet (CR 603.10a's reasoning,
// read at the entry), and a Grizzly Bears meets it.
func TestEnteredAsCreatureLockstep(t *testing.T) {
	t.Parallel()
	deck := []cardset.Entry{{Name: "Bristlebane Outrider", Count: 10}, {Name: "Mishra's Factory", Count: 14},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 12}}
	lockstepSet(t, "enteredcreature", [2][]cardset.Entry{deck, deck}, 64, "KENTEREDCREATURE_SEEDS")
}
