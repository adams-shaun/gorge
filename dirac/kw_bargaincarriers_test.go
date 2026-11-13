package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// kbargainFamily is the twelve Bargain family carriers kgen lowers.
var kbargainFamily = []string{
	"Agatha's Champion", "Brave the Wilds", "Candy Grapple", "Diminisher Witch",
	"Farsight Ritual", "High Fae Negotiator", "Kellan's Lightblades",
	"Realm-Scorcher Hellkite", "Stonesplitter Bolt", "Tenacious Tomeseeker",
	"Torch the Tower", "Troublemaker Ouphe",
}

// kbargainCarrierScene builds a one-card scenario for name: the carrier in
// hand, Ornithopter as the sacrifice candidate, and a Forest in the library.
// The floating pool pays every colour, so the cast never depends on a land.
func kbargainCarrierScene(t *testing.T, name string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: name, Count: 1}, {Name: "Ornithopter", Count: 1}, {Name: "Forest", Count: 1},
	}
	set := cardsettest.Set(t, "bargain")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatalf("%s: matchup: %v", name, err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if !tbl.Cards[id[name]].Bargain {
		t.Fatalf("%s: CardDef.Bargain is false (kgen lowering regression)", name)
	}
	if name == "Kellan's Lightblades" {
		// The printed target (attacking/blocking creature) is combat-only;
		// widen it to any creature so this test can drive the cast in the
		// first main. The Bargain row is what is under test here.
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
		ab := &tbl.Cards[id[name]].Abilities[0]
		ab.Targets[0].Filter = kop.FilterID(len(tbl.Filters) - 1)
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	sc.perm(id["Forest"], 0) // Brave the Wilds' bargained land target
	sc.s.pl[0].pool = [kop.NumManaKinds]uint8{20, 20, 20, 20, 20, 20}
	return sc, id
}

// kbargainCarrierCast drives one bargained or plain cast of o through the
// kernel's stages (the bargain election, X/colour choices, targets and
// payment) and returns the stack entry.
func kbargainCarrierCast(t *testing.T, sc *scen, o Obj, card CardID, cand Obj, bargained bool) stackEntry {
	t.Helper()
	d := sc.s.Step()
	kind := AkCast
	if bargained {
		kind = AkCastCasualty
	}
	cast := -1
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == kind && a.Source() == TCode(uint8(o)) && a.Mode() == 0 {
			cast = i
			break
		}
	}
	if cast < 0 {
		t.Fatalf("card %d: cast kind %d not offered (decision kind %d, %d options)", card, kind, d.Kind, d.N)
	}
	sc.s.Apply(d.Options[cast])
	onStack := false
	for step := 0; step < 80; step++ {
		if sc.s.sp > 0 && sc.s.stack[0].kind == seSpell && sc.s.stack[0].card == card {
			onStack = true
		}
		d = sc.s.Step()
		if onStack && d.Kind == DkPriority {
			return sc.s.stack[0]
		}
		if d.N == 0 {
			continue
		}
		pick := d.Options[0]
		if d.Kind == DkChoose && d.Sub == ChGenBargain {
			found := false
			for i := 0; i < int(d.N); i++ {
				if Obj(d.Options[i].Source()) == cand {
					pick, found = d.Options[i], true
					break
				}
			}
			if !found {
				t.Fatalf("card %d: the bargain election does not offer the candidate %d", card, cand)
			}
		}
		sc.s.Apply(pick)
	}
	t.Fatalf("card %d: cast did not reach the stack", card)
	return stackEntry{}
}

// kbargainResolveSearch passes priority until the stack is empty and reports
// whether the resolving spell posed a library-search pick.
func kbargainResolveSearch(sc *scen) bool {
	searched := false
	for step := 0; step < 40 && sc.s.sp > 0; step++ {
		d := sc.s.Step()
		if d.N == 0 {
			continue
		}
		if d.Kind == DkChoose {
			searched = true
		}
		sc.s.Apply(d.Options[0])
	}
	return searched
}

// TestBargainCarrierFlags casts every lowered carrier twice, bargained and
// plain, and checks the provenance each leaves on the stack. This is the
// per-card coverage the pool-pays lockstep cannot reach for the UU/BB and
// combat-timing carriers: a lowering regression (CardDef.Bargain false) or a
// kernel row that fails to set sfKicked is caught here.
func TestBargainCarrierFlags(t *testing.T) {
	for _, name := range kbargainFamily {
		t.Run(name, func(t *testing.T) {
			for _, bargained := range []bool{true, false} {
				sc, id := kbargainCarrierScene(t, name)
				card := id[name]
				cand := sc.perm(id["Ornithopter"], 0)
				o := sc.hand(card, 0)
				e := kbargainCarrierCast(t, sc, o, card, cand, bargained)
				got := e.flags&sfKicked != 0
				if got != bargained {
					t.Fatalf("bargained=%v: stack sfKicked=%v", bargained, got)
				}
				if name == "Brave the Wilds" {
					// the land is targeted only when bargained (CR 702.166d); either way it searches
					if (TCode(e.tgt[0]) != NoTarget) != bargained {
						t.Fatalf("bargained=%v: tgt[0]=%d", bargained, e.tgt[0])
					}
					if !kbargainResolveSearch(sc) {
						t.Fatalf("bargained=%v: the library search was not asked", bargained)
					}
				}
			}
		})
	}
}

// TestBargainHellkiteManaChoice resolves Realm-Scorcher Hellkite, bargained
// and plain: only the bargained cast's ETB trigger adds four mana, and it asks
// for each one's colour (ExtManaChoice, "Combo Any", ChGenManaChoice; CR 106.3,
// a triggered ability is no mana ability, CR 605.1a).
func TestBargainHellkiteManaChoice(t *testing.T) {
	const name = "Realm-Scorcher Hellkite"
	for _, bargained := range []bool{true, false} {
		sc, id := kbargainCarrierScene(t, name)
		card := id[name]
		cand := sc.perm(id["Ornithopter"], 0)
		o := sc.hand(card, 0)
		kbargainCarrierCast(t, sc, o, card, cand, bargained)
		asks, before, after := 0, uint8(0), uint8(0)
		sum := func() (n uint8) {
			for _, v := range sc.s.pl[0].pool {
				n += v
			}
			return n
		}
		for step := 0; step < 60; step++ {
			d := sc.s.Step()
			if d.N == 0 {
				continue
			}
			if d.Kind == DkChoose && d.Sub == ChGenManaChoice {
				if asks == 0 {
					before = sum()
				}
				asks++
				if d.N != 5 {
					t.Fatalf("bargained=%v: colour ask offers %d colours, want 5", bargained, d.N)
				}
				sc.s.Apply(d.Options[0])
				after = sum()
				continue
			}
			if asks == 4 {
				break
			}
			sc.s.Apply(d.Options[0]) // pass priority: the spell, then its trigger, resolve
		}
		want := 0
		if bargained {
			want = 4
		}
		if asks != want {
			t.Fatalf("bargained=%v: %d colour asks, want %d", bargained, asks, want)
		}
		if bargained && after != before+4 {
			t.Fatalf("pool %d after four picks, want %d", after, before+4)
		}
	}
}
