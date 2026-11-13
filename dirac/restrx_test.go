package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRestrictedManaPaysXSpell: Secluded Courtyard (Hydra chosen) is p0's
// only land and Wildwood Scourge ({X}{G}, a Hydra) is in hand. The cast is
// offered on the Courtyard's restricted {G} (a planner unit for a Hydra
// spell legal.go restrFallback), so the X ask must
// count that source as gorge's planner does. Before, the ask saw the ordinary pool and
// sources only, priced the {G} unpayable and posed X with no options:
// "a decision with no options" (random-policy FDN games 32669, 36843 and
// 121499 of a random-deck hunt).
func TestRestrictedManaPaysXSpell(t *testing.T) {
	deck := []cardset.Entry{{Name: "Secluded Courtyard", Count: 1}, {Name: "Wildwood Scourge", Count: 1}, {Name: "Plains", Count: 10}}
	sc, id := newObjScen(t, deck)
	sub := -1
	for i, n := range sc.k.t.Subtypes {
		if n == "Hydra" {
			sub = i
		}
	}
	if sub < 0 {
		t.Skip("no Hydra subtype in the table")
	}
	court := sc.perm(id["Secluded Courtyard"], 0)
	if d := sc.s.Step(); d.Kind == DkChoose { // its as-enters creature type: answered, then set
		sc.s.Apply(d.Options[0])
	}
	sc.s.perms[sc.slot(court)].chosen = uint8(sub)
	hydra := sc.hand(id["Wildwood Scourge"], 0)
	s := sc.s
	d := s.Step()
	if !castOffered(&d, hydra) {
		t.Fatalf("Wildwood Scourge not offered (Hydra chosen): decision kind %d, %d options", d.Kind, d.N)
	}
	s.Apply(MakeAction(AkCast, TCode(hydra), 0, 0))
	d = s.Step()
	if d.Kind != DkChoose || d.Sub != ChX || d.N != 1 || d.Options[0].Mode() != 0 {
		t.Fatalf("X ask: kind %d sub %d, %d options, want X = 0 only", d.Kind, d.Sub, d.N)
	}
	s.Apply(d.Options[0])
	s.Step()
	if s.rbAny(0) || s.pl[0].pool != ([kop.NumManaKinds]uint8{}) || s.sp != 1 {
		t.Fatalf("after the Scourge: restricted %v pool %v stack %d, want the restricted {G} spent and the Scourge on the stack", s.rb[0], s.pl[0].pool, s.sp)
	}
	if !sc.tapped(court) {
		t.Fatal("the Courtyard is not tapped by the plan")
	}
}

// TestRestrictedManaPaysKickedCast: two Secluded Courtyards (Goblin
// chosen) beside a Plains and a Forest; Goblin Bushwhacker's kicked cast
// ({R} plus kicker {R}) is offered on the Courtyards' restricted {R}{R}
// (planner units for a Goblin spell). The kicked cast's
// lowering taps the witness's sources for what the pool lacks; it must
// count that restricted mana as the witness does. Before, it priced {R}{R} against the
// Plains and the Forest and panicked "autopay found no plan"
// (random-policy FDN game 557584 of a random-deck hunt).
func TestRestrictedManaPaysKickedCast(t *testing.T) {
	deck := []cardset.Entry{{Name: "Secluded Courtyard", Count: 2}, {Name: "Goblin Bushwhacker", Count: 1}, {Name: "Plains", Count: 4}, {Name: "Forest", Count: 4}}
	sc, id := newObjScen(t, deck)
	sub := -1
	for i, n := range sc.k.t.Subtypes {
		if n == "Goblin" {
			sub = i
		}
	}
	if sub < 0 {
		t.Skip("no Goblin subtype in the table")
	}
	var courts []Obj
	for i := 0; i < 2; i++ {
		court := sc.perm(id["Secluded Courtyard"], 0)
		if d := sc.s.Step(); d.Kind == DkChoose { // its as-enters creature type: answered, then set
			sc.s.Apply(d.Options[0])
		}
		sc.s.perms[sc.slot(court)].chosen = uint8(sub)
		courts = append(courts, court)
	}
	sc.perm(id["Plains"], 0)
	sc.perm(id["Forest"], 0)
	bw := sc.hand(id["Goblin Bushwhacker"], 0)
	s := sc.s
	d := s.Step()
	kicked := MakeAction(AkCast, TCode(bw), 0, 1)
	if !hasOption(&d, kicked) {
		t.Fatalf("kicked Goblin Bushwhacker not offered: decision kind %d %v", d.Kind, d.Options[:d.N])
	}
	s.Apply(kicked)
	for i := 0; i < 10 && s.sp == 0; i++ {
		d = s.Step()
		if d.Kind == DkNone {
			break
		}
		s.Apply(d.Options[0])
	}
	if s.rbAny(0) || s.pl[0].pool != ([kop.NumManaKinds]uint8{}) || s.sp != 1 || s.stack[0].flags&sfKicked == 0 {
		t.Fatalf("after the Bushwhacker: restricted %v pool %v stack %d, want the restricted {R}{R} spent and the kicked Bushwhacker on the stack", s.rb[0], s.pl[0].pool, s.sp)
	}
	for _, c := range courts {
		if !sc.tapped(c) {
			t.Fatal("a Courtyard is not tapped by the plan")
		}
	}
}
