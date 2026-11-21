package dirac

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestModalBackGrantPlanTaps is the payment planner's key 8 for a modal DFC on
// the battlefield as its land back face (printed {T}: Add {R}) under a granted
// basic land type (Urborg, Tomb of Yawgmoth makes every land a Swamp):
// gorge's census lists the back face's printed ability and the granted Swamp's
// intrinsic ability as two alternatives of one source, and its rank key 8
// (pay.CompareSteps) orders the intrinsic one first, so a tied plan taps the
// back face for {B} and pays the {R} pip from a Mountain. The kernel's exact
// planner must rank the same way: the back face and a Forest and a Mountain,
// not the back face and Foul Orchard ({B} or {G}) paying the pip.
func TestModalBackGrantPlanTaps(t *testing.T) {
	set, err := cardset.SynthByName("tail-32")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no tail-32 table (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	deck := []cardset.Entry{
		{Name: "Evaporate", Count: 1},
		{Name: "Akoum Warrior", Count: 1},
		{Name: "Urborg, Tomb of Yawgmoth", Count: 1},
		{Name: "Foul Orchard", Count: 1},
		{Name: "Forest", Count: 6},
		{Name: "Mountain", Count: 6},
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = c
		}
	}
	if len(tbl.Cards[id["Akoum Warrior"]].Faces) != 2 {
		t.Fatalf("Akoum Warrior has %d faces; want 2", len(tbl.Cards[id["Akoum Warrior"]].Faces))
	}
	teethBack := tbl.Cards[id["Akoum Warrior"]].Faces[1]
	// The surface is autopay: the cast pays through the payment planner's
	// plan(), the path the lockstep divergence ran on.
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true, GorgeAutopaySurface: true})
	sc.perm(id["Urborg, Tomb of Yawgmoth"], 1) // p1's board: the grant is global (CR 305.7)
	if !sc.k.gen.blgOn {
		t.Fatal("precondition: blgOn unset: Urborg grants no basic land type")
	}
	m29 := sc.perm(id["Mountain"], 0)
	m28 := sc.perm(id["Mountain"], 0)
	f14 := sc.perm(id["Forest"], 0)
	f11 := sc.perm(id["Forest"], 0)
	// entry order mirrors the recorded game: the Forest (its gorge id 10) and
	// the back face predate the Orchard (id 15); the Mountain (id 25) is last
	f10 := sc.perm(id["Forest"], 0)
	teeth := sc.perm(teethBack, 0)
	m30 := sc.perm(id["Mountain"], 0)
	orchard := sc.perm(id["Foul Orchard"], 0)
	m25 := sc.perm(id["Mountain"], 0)
	// both ETB tapped (Akoum Teeth "enters tapped"; Foul Orchard likewise):
	// untap them, so the payment planner's census lists them as sources
	sc.s.perms[sc.slot(teeth)].flags &^= pfTapped
	sc.s.perms[sc.slot(orchard)].flags &^= pfTapped
	// the recorded game's availability at the cast: only the back face, the
	// Orchard, one Forest and one Mountain are untapped (the rest already
	// spent), so the plan's sources are these four and the tie is the back
	// face's intrinsic {B} against its printed {R}
	for _, o := range []Obj{m29, m28, f14, f11, m30} {
		sc.s.perms[sc.slot(o)].flags |= pfTapped
	}
	if bm := sc.s.blgMask(sc.k, sc.slot(teeth)); bm != kop.ColB {
		t.Fatalf("precondition: the back face's granted mask %#x; want Swamp's %#x", bm, kop.ColB)
	}
	if pc := sc.s.manaPCGen(sc.k, sc.slot(teeth)); pc.manaCol != kop.ColB|kop.ColR || !pc.manaMulti {
		t.Fatalf("precondition: the back face's view is %#x multi=%v; want the printed {R} with the granted {B}", pc.manaCol, pc.manaMulti)
	}
	// The Orchard's own {B} or {G} is a choice shape: the kernel's census unit
	// holds no printed ability of its own (its colour choice order is the flat
	// one), so the plan that uses the back face's intrinsic {B} leaves it alone.
	if pc := sc.s.manaPCGen(sc.k, sc.slot(orchard)); pc.manaCol != kop.ColB|kop.ColG {
		t.Fatalf("precondition: Foul Orchard's view is %#x; want {B}{G}", pc.manaCol)
	}

	evap := sc.hand(id["Evaporate"], 0)
	sc.do(t, AkCast, TCode(evap), scAny) // the payment runs at the spell's manaStep
	for i := 0; i < 6; i++ {
		if d := sc.s.Step(); d.Kind != DkPriority {
			t.Fatalf("a decision mid-cast: kind %d opts %v", d.Kind, d.Options[:d.N])
		}
	}
	tap := func(o Obj) bool { return sc.s.perms[sc.slot(o)].flags&pfTapped != 0 }
	if !tap(teeth) || !tap(f10) || !tap(m25) {
		t.Errorf("kernel tapped teeth=%v f10=%v m25=%v; want the back face for {B} (its granted intrinsic), the {R} pip from the Mountain and the generic {1} from the Forest",
			tap(teeth), tap(f10), tap(m25))
	}
	if tap(orchard) {
		t.Error("Foul Orchard is tapped: the plan spent the printed {R} pip on the back face (the intrinsic {B} must rank first)")
	}
	if pool := sc.s.pl[0].pool; pool != ([6]uint8{}) {
		t.Errorf("the pool holds %v after the payment; want empty", pool)
	}
}
