package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// prevColGreen is the ChGenColor mode naming green (raPrevColor: W U B R G).
const prevColGreen uint8 = 4

// TestBarePreventionsNoOrderAskCombat: the combat ask site
// (replOrderCandsFz) of the bare-prevention rule. Player 0 casts Prismatic
// Strands twice naming green, so two PreventColor records match Forest
// Bear's combat damage to the Catacomb Crocodile blocking it. Both are bare
// full preventions: whichever applies, the damage is gone, so gorge poses no
// CR 616.1 order ask (engine/rules allBarePreventions) and neither does the
// kernel. The Crocodile takes nothing and its own (black) damage still kills
// the Bear, so the damage step ran.
func TestBarePreventionsNoOrderAskCombat(t *testing.T) {
	set := cardsettest.Set(t, "mbc")
	deck := []cardset.Entry{{Name: "Prismatic Strands", Count: 2}, {Name: "Forest Bear", Count: 1},
		{Name: "Catacomb Crocodile", Count: 1}, {Name: "Plains", Count: 8}}
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Fatalf("%s not in the MBC table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	s := sc.s
	if sc.k.gen.anyRepl&rbPrevCol == 0 {
		t.Fatal("precondition: rbPrevCol unset with Prismatic Strands in the table")
	}
	sc.lands(0, id["Plains"], 6)
	bear := sc.perm(id["Forest Bear"], 0)
	croc := sc.perm(id["Catacomb Crocodile"], 1)
	for range 2 {
		sc.castCard(t, sc.hand(id["Prismatic Strands"], 0))
		picked := false
		for i := 0; i < 30; i++ {
			d := s.Step()
			if d.Kind == DkPriority && s.sp == 0 && picked {
				break
			}
			switch {
			case d.Kind == DkChoose && d.Sub == ChGenColor:
				for _, a := range d.Options[:d.N] {
					if a.Mode() == prevColGreen {
						s.Apply(a)
						picked = true
						break
					}
				}
				if !picked {
					t.Fatalf("colour ask offers no green: %v", d.Options[:d.N])
				}
			case d.Kind == DkPriority:
				s.Apply(MakeAction(AkPass, 0, 0, 0))
			default:
				s.Apply(d.Options[0]) // the cost
			}
		}
		if !picked || s.sp != 0 {
			t.Fatalf("Strands did not resolve naming green (picked %v, stack %d)", picked, s.sp)
		}
	}
	green := 0
	for i := range s.g.prevCol {
		if r := &s.g.prevCol[i]; r.sc == 0 && r.col&kop.ColG != 0 {
			green++
		}
	}
	if green != 2 {
		t.Fatalf("precondition: %d green PreventColor records, want 2 (one per Strands)", green)
	}

	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(bear), scAny)
	sc.do(t, AkDone, scAny, scAny)
	sc.passUntil(t, kop.StepBlockers)
	sc.do(t, AkBlock, TCode(croc), TCode(bear))
	sc.do(t, AkDone, scAny, scAny)
	for i := 0; i < 50 && s.step != kop.StepMain2; i++ {
		d := s.Step()
		if d.Kind == DkChoose && d.Sub == ChGenReplOrder {
			t.Fatalf("combat order ask posed between two bare preventions: player %d opts %v", d.Player, d.Options[:d.N])
		}
		if d.Kind != DkPriority {
			t.Fatalf("unexpected decision kind %d sub %d player %d in combat", d.Kind, d.Sub, d.Player)
		}
		s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	if s.step != kop.StepMain2 {
		t.Fatalf("step %d, want main 2 after combat", s.step)
	}
	if !sc.onBF(croc) || s.perms[sc.slot(croc)].damage != 0 {
		t.Fatalf("Catacomb Crocodile on battlefield %v damage %d, want the Bear's green damage prevented", sc.onBF(croc), s.perms[sc.slot(croc)].damage)
	}
	if sc.onBF(bear) {
		t.Fatal("precondition: Forest Bear survived the Crocodile's 3 black damage; the damage step did not deal damage")
	}
}
