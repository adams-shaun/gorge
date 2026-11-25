package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Sisay is a Vanguard, not a battlefield permanent: its printed
// `Mode$ TapsForMana | Static$ True | TriggerZones$ Command` line adds one mana
// of the type a land its controller taps produced (CR 106.12a, 605.3b). The
// card starts in the command zone and is never cast, so the matchup seats it
// there with SeatCommandZone (gorge's Config.Commanders without
// FormatCommander; the kernel's Config.SeatCommanders).
//
// TestTapsForManaSisayLockstep plays the tapsformana pool's lands and cheap
// spells with Sisay in both command zones. TestTapsForManaSisayMutation drops
// the command-zone StatTapExtra from the kernel's table and requires a
// divergence, so the seeds exercise the static rather than passing because no
// land is ever tapped under it.
var sisayDecks = [2][]cardset.Entry{
	{
		{Name: "Sisay", Count: 1},
		{Name: "Grizzly Bears", Count: 8},
		{Name: "Llanowar Elves", Count: 7},
		{Name: "Forest", Count: 20},
		{Name: "Opt", Count: 4},
	},
	{
		{Name: "Sisay", Count: 1},
		{Name: "Grizzly Bears", Count: 12},
		{Name: "Forest", Count: 19},
		{Name: "Shock", Count: 8},
	},
}

// sisayCardDef is the matchup table's Sisay record.
func sisayCardDef(t *testing.T, tbl *kop.Table) *kop.CardDef {
	t.Helper()
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Sisay" {
			return &tbl.Cards[i]
		}
	}
	t.Fatal("Sisay is not in the matchup's table")
	return nil
}

// sisayCommandExtra reports whether def carries Sisay's command-zone reflected
// tap extra.
func sisayCommandExtra(def *kop.CardDef) bool {
	for _, a := range def.Abilities {
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatTapExtra &&
			a.Static.Zone == kop.ZoneCommand && a.Static.StFlags&kop.StTapReflect != 0 {
			return true
		}
	}
	return false
}

// sisayRun plays the decks with Sisay in both command zones, returning the
// number of divergent seeds (stopping at the first unless report).
func sisayRun(t *testing.T, seeds uint64, report bool, mutate func(*kop.Table)) int {
	t.Helper()
	reg, set, gd := lockstepPool(t, "tapsformana", sisayDecks)
	tbl, _, _, err := set.Matchup(sisayDecks[0], sisayDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	if !report {
		if def := sisayCardDef(t, tbl); def.Types&kop.TVanguard == 0 || !sisayCommandExtra(def) {
			t.Fatalf("Sisay's table entry is not a Vanguard with a command-zone reflected TapExtra static: types=%#x abilities=%+v", def.Types, def.Abilities)
		}
	}
	div := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"tapsformana-sisay-a", "tapsformana-sisay-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		m.SeatCommandZone = true
		m.Commanders = [2]int{0, 0}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if res.Div != nil {
			div++
			if !report {
				from := max(res.Div.At-12, 0)
				t.Errorf("seed %d last actions %v\nseed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
					seed, actionStrings(res.Actions[from:res.Div.At]), seed, res.Div.At, res.Div.What,
					res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions)
				return div
			}
		}
	}
	return div
}

func TestTapsForManaSisayLockstep(t *testing.T) {
	t.Parallel()
	if n := sisayRun(t, 120, false, nil); n != 0 {
		t.Fatalf("%d divergent seeds", n)
	}
}

func TestTapsForManaSisayMutation(t *testing.T) {
	t.Parallel()
	dropped := 0
	n := sisayRun(t, 120, true, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			var keep []kop.Ability
			for _, a := range cards[i].Abilities {
				if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatTapExtra && a.Static.Zone == kop.ZoneCommand {
					dropped++
					continue
				}
				keep = append(keep, a)
			}
			cards[i].Abilities = keep
		}
		tbl.Cards = cards
	})
	if dropped == 0 {
		t.Fatal("the kernel's table carries no command-zone StatTapExtra static to drop")
	}
	if n == 0 {
		t.Fatal("no seed diverged with Sisay's command-zone tap-extra static dropped: the pool does not exercise the card")
	}
}
