package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// Quantum Reduction's self grant (S:Mode$ CastWithFlash | ValidCard$ Card.Self |
// ValidSA$ Spell.Teamwork | EffectZone$ All, cast_selfflash.go). No BLB card
// carries it, so the scenario gives Wear Down (a sorcery with one target) a
// Teamwork threshold and the self grant on a table built in the test.

func selfFlashTeamworkScen(t *testing.T, sa uint8) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "blb")
	var deck []cardset.Entry
	for _, nm := range []string{"Wear Down", "Forest", "Fountainport Bell", "Jolly Gerbils"} {
		if i, ok := set.Lookup(nm); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the BLB table", nm)
		}
		deck = append(deck, cardset.Entry{Name: nm, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	cd := &tbl.Cards[id["Wear Down"]]
	cd.Teamwork = 2
	cd.SelfFlash, cd.SelfFlashSA = true, sa
	cd.Abilities = append(cd.Abilities, kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatCastWithFlash}})
	return newScenTable(t, tbl, Config{NoElide: true}), id
}

// selfFlashTWBoard: two Forests, a Gerbils (power 2) to tap, the Bell to
// target and Wear Down in hand, at the beginning of combat (no sorcery speed).
func selfFlashTWBoard(t *testing.T, sa uint8, instantSpeed bool) (sc *scen, wd, gerbils Obj) {
	t.Helper()
	sc, id := selfFlashTeamworkScen(t, sa)
	for i := 0; i < 2; i++ {
		sc.perm(id["Forest"], 0)
	}
	gerbils = sc.perm(id["Jolly Gerbils"], 0)
	sc.perm(id["Fountainport Bell"], 1)
	wd = sc.hand(id["Wear Down"], 0)
	if instantSpeed {
		sc.s.step = kop.StepBeginCombat
	}
	if !sc.k.teamwork || !sc.k.anySelfFlash {
		t.Fatalf("precondition: prep gates teamwork=%v anySelfFlash=%v", sc.k.teamwork, sc.k.anySelfFlash)
	}
	return sc, wd, gerbils
}

func TestSelfFlashTeamworkElection(t *testing.T) {
	// sorcery speed: both casts, as before
	sc, wd, gerbils := selfFlashTWBoard(t, kop.SfsaTeamwork, false)
	if got := sc.castOptions(wd); len(got) != 2 || got[0].Kind() != AkCast || got[1].Kind() != AkCastCasualty {
		t.Fatalf("sorcery speed cast options %v, want the plain cast then the teamworked one", got)
	}

	// instant speed: the teamworked cast alone
	sc, wd, _ = selfFlashTWBoard(t, kop.SfsaTeamwork, true)
	got := sc.castOptions(wd)
	if len(got) != 1 || got[0].Kind() != AkCastCasualty {
		t.Fatalf("instant speed cast options %v, want only the teamworked cast", got)
	}
	// unpaid election: the cast is reversed (gorge recheckIllegal, CR 601.2e)
	sc.castOption(t, wd, AkCastCasualty, 0)
	d := sc.s.Step()
	if d.Kind == DkChoose && d.Sub == ChGenGift {
		sc.s.Apply(MakeAction(AkChoose, 0, 0, 0)) // decline the gift (Wear Down's)
		d = sc.s.Step()
	}
	if d.Kind != DkChoose || d.Sub != ChGenTeamwork {
		t.Fatalf("decision kind %d sub %d, want the teamwork election", d.Kind, d.Sub)
	}
	sc.s.Apply(MakeAction(AkDone, 0, 0, 0))
	for i := 0; i < 20; i++ {
		if d = sc.s.Step(); d.Kind == DkPriority {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.sp != 0 || sc.s.objs[wd].zone != kop.ZoneHand {
		t.Fatalf("a declined election at instant speed left sp=%d zone=%d, want the cast reversed", sc.s.sp, sc.s.objs[wd].zone)
	}

	// paid election: the cast stands, the Gerbils are tapped
	sc, wd, gerbils = selfFlashTWBoard(t, kop.SfsaTeamwork, true)
	sc.castOption(t, wd, AkCastCasualty, 0)
	if d = sc.s.Step(); d.Kind == DkChoose && d.Sub == ChGenGift {
		sc.s.Apply(MakeAction(AkChoose, 0, 0, 0)) // decline the gift (Wear Down's)
	}
	sc.do(t, AkChoose, TCode(gerbils), scAny)
	for i := 0; i < 30; i++ {
		if d = sc.s.Step(); d.Kind == DkPriority {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.sp != 1 || sc.s.objs[wd].zone != kop.ZoneStack || sc.s.perms[sc.slot(gerbils)].flags&pfTapped == 0 {
		t.Fatalf("a paid election left sp=%d zone=%d tapped=%v, want the teamworked spell on the stack",
			sc.s.sp, sc.s.objs[wd].zone, sc.s.perms[sc.slot(gerbils)].flags&pfTapped != 0)
	}

	// control: the bare Spell grant (every other carrier) keeps the plain cast at instant speed
	sc, wd, _ = selfFlashTWBoard(t, kop.SfsaSpell, true)
	if got := sc.castOptions(wd); len(got) < 1 || got[0].Kind() != AkCast {
		t.Fatalf("bare-Spell self grant at instant speed offered %v, want the plain cast", got)
	}

	// no creature power to tap: nothing at instant speed
	sc, wd, gerbils = selfFlashTWBoard(t, kop.SfsaTeamwork, true)
	sc.s.perms[sc.slot(gerbils)].flags |= pfTapped
	if got := sc.castOptions(wd); len(got) != 0 {
		t.Fatalf("instant speed with the creatures tapped offered %v, want none", got)
	}
}
