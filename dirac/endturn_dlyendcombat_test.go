package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestEndTheTurnDropsEndOfCombatPromises: end-the-turn during combat jumps to
// cleanup (CR 723.1), so this turn's end of combat step never comes. gorge's
// |TT= expiry (TurnChange) removes a Myriad token's exile promise and the
// SacrificeCombat rider; the kernel's record must not outlive the turn and
// fire at a later turn's end of combat. A record of another kind stays.
func TestEndTheTurnDropsEndOfCombatPromises(t *testing.T) {
	deck := []cardset.Entry{{Name: "Time Stop", Count: 1}, {Name: "Eager Trufflesnout", Count: 2}, {Name: "Island", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
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
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	sc.k.gen.anyTokEndCombat = true
	for i := 0; i < 6; i++ {
		sc.perm(id["Island"], 0)
	}
	atk := sc.perm(id["Eager Trufflesnout"], 0)
	stop := sc.hand(id["Time Stop"], 0)
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(atk), scAny)
	sc.do(t, AkDone, scAny, scAny)
	ver := objWord(s.objs[atk].ver)
	s.dly[0] = delayed{act: uint8(kop.OpExile), obj: atk, ver: ver, when: dlyEndCombat, creator: NoObj}
	s.dly[1] = delayed{act: uint8(kop.OpSacrifice), obj: atk, ver: ver, when: dlyUntracked, creator: NoObj}
	s.ndly = 2
	sc.do(t, AkCast, TCode(stop), scAny)
	for i := 0; i < 8 && (s.objs[stop].zone == kop.ZoneStack || s.objs[stop].zone == kop.ZoneHand); i++ {
		d := s.Step()
		s.Apply(d.Options[0])
	}
	if s.objs[stop].zone != kop.ZoneExile {
		t.Fatalf("Time Stop zone %d, want exile", s.objs[stop].zone)
	}
	s.Step()
	if s.ndly != 1 || s.dly[0].when != dlyUntracked {
		t.Fatalf("after the turn ended: ndly %d, first when %d; want only the dlyUntracked record kept", s.ndly, s.dly[0].when)
	}
	if s.dly[1] != (delayed{}) {
		t.Errorf("the vacated slot was not zeroed: %+v", s.dly[1])
	}
}
