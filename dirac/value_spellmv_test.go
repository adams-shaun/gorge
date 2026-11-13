package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// valueSpellmvScen is facesScen over the generated spellmanavalue pool: the SpellCast carriers
// of TriggeredSpellAbility$CardManaCostLKI and the split, X, copy and
// Adventure fixtures.
func valueSpellmvScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, "spellmanavalue")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the spellmanavalue table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Plains"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// TestTriggerSpellMVLKI: the value reads the triggering spell as gorge's
// pre-cast LKI: a split card sums its halves (CR 709.4b), an announced {X} is
// not part of the object (CR 202.3e), an Adventure reads its cast face, and a
// spell copy has no snapshot so it reads its copied face plus X.
func TestTriggerSpellMVLKI(t *testing.T) {
	sc, id := valueSpellmvScen(t, "Fire", "Mind Spring", "Brazen Borrower")
	fire := id["Fire"]
	ice := sc.k.t.Cards[fire].Faces[1]
	ms := id["Mind Spring"]
	petty := sc.k.t.Cards[id["Brazen Borrower"]].Faces[1]
	if !sc.k.gen.anySplit {
		t.Fatal("anySplit not set")
	}
	spell := func(c CardID, x uint8) *stackEntry {
		o := sc.obj(c, 0)
		sc.s.objs[o].zone = kop.ZoneStack
		sc.s.push(stackEntry{kind: seSpell, obj: o, ctrl: 0, card: c, trigObj: NoObj, x: objWord(x)})
		return &sc.s.stack[sc.s.sp-1]
	}
	copyOf := func(e *stackEntry) *stackEntry {
		sc.s.objs[e.obj].of |= ofCopy
		return e
	}
	cases := []struct {
		name string
		e    *stackEntry
		want uint8
	}{
		{"cast split front (Fire)", spell(fire, 0), 2},
		{"cast split back (Ice)", spell(ice, 0), 2},
		{"cast X spell (Mind Spring), X=3", spell(ms, 3), 5},
		{"cast Adventure half (Petty Theft)", spell(petty, 0), 2},
		{"copy X spell (Mind Spring), X=3", copyOf(spell(ms, 3)), 5},
		{"copy split half (Ice)", copyOf(spell(ice, 0)), 2},
	}
	for _, c := range cases {
		if got := sc.s.triggerSpellMVLKI(sc.k, c.e); got != c.want {
			t.Errorf("%s: mana value %d, want %d", c.name, got, c.want)
		}
	}
}

// TestTriggerSpellMVLKISnapshot: the value is fixed when the trigger is
// queued. A cast Adventure half leaves the stack before the trigger
// resolves, and stackExitFace turns the card back to its front (MV 3); the
// trigger entry's snapshot still reads the cast half (2), as gorge's
// pre-push LKI does.
func TestTriggerSpellMVLKISnapshot(t *testing.T) {
	sc, id := valueSpellmvScen(t, "Brazen Borrower")
	front := id["Brazen Borrower"]
	petty := sc.k.t.Cards[front].Faces[1]
	o := sc.obj(petty, 0)
	sc.s.objs[o].zone = kop.ZoneStack
	spell := stackEntry{kind: seSpell, obj: o, ctrl: 0, card: petty, trigObj: NoObj}
	mv := sc.s.triggerSpellMVLKI(sc.k, &spell)
	if mv != 2 {
		t.Fatalf("cast Adventure half: mana value %d, want 2", mv)
	}
	trig := stackEntry{kind: seTrigger, ctrl: 0, card: id["Brazen Borrower"], trigObj: o, trigMV: objWord(mv)}
	sc.s.stackExitFace(sc.k, o, kop.ZoneGraveyard, false)
	if c := sc.s.objs[o].card; c != front {
		t.Fatalf("stack exit did not reset the Adventure card to its front: %d", c)
	}
	x := kop.Expr{Kind: kop.ExTriggerSpellMVLKI}
	if got := sc.s.exprCold(sc.k, &x, evalCtx{e: &trig}); got != 2 {
		t.Fatalf("after stack exit: mana value %d, want the snapshot 2", got)
	}
}

// TestTriggerSpellMVLKIWiring: the SpellCast trigger's e.trigObj reaches the
// new value (not just the helper): Manaplasm pumps by the triggering spell's
// pre-cast mana value -- Fire // Ice sums to 4 (5/5), Mind Spring's {X} is
// not counted (2, so 3/3).
func TestTriggerSpellMVLKIWiring(t *testing.T) {
	t.Run("split sum", func(t *testing.T) {
		sc, id := valueSpellmvScen(t, "Manaplasm", "Fire", "Mountain", "Island")
		mana := sc.perm(id["Manaplasm"], 0)
		sc.perm(id["Mountain"], 0)
		sc.perm(id["Island"], 0)
		fire := sc.hand(id["Fire"], 0)
		sc.stepTo(t, 30, func() bool { return sc.s.sp == 0 && sc.s.npend == 0 && sc.s.objs[fire].zone == kop.ZoneGraveyard },
			func(d *Decision) (Action, bool) {
				for i := 0; i < int(d.N); i++ {
					if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(fire)) {
						return a, true
					}
				}
				return 0, false
			})
		if p, tough := sc.s.PowerToughness(mana); p != 3 || tough != 3 {
			t.Fatalf("Manaplasm %d/%d after casting Fire // Ice, want 3/3", p, tough)
		}
	})
	t.Run("X excluded", func(t *testing.T) {
		sc, id := valueSpellmvScen(t, "Manaplasm", "Mind Spring", "Island")
		mana := sc.perm(id["Manaplasm"], 0)
		for i := 0; i < 5; i++ {
			sc.perm(id["Island"], 0)
		}
		ms := sc.hand(id["Mind Spring"], 0)
		sc.stepTo(t, 40, func() bool { return sc.s.sp == 0 && sc.s.npend == 0 && sc.s.objs[ms].zone == kop.ZoneGraveyard },
			func(d *Decision) (Action, bool) {
				if d.Kind == DkChoose && d.Sub == ChX {
					for i := 0; i < int(d.N); i++ {
						if d.Options[i].Mode() == 3 {
							return d.Options[i], true
						}
					}
				}
				for i := 0; i < int(d.N); i++ {
					if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(ms)) {
						return a, true
					}
				}
				return 0, false
			})
		if p, tough := sc.s.PowerToughness(mana); p != 6 || tough != 6 {
			t.Fatalf("Manaplasm %d/%d after casting Mind Spring with X=3, want 6/6", p, tough)
		}
	})
}

// TestTriggerSpellMVLKICopyTrigger: a copied spell fires the SpellCastOrCopy
// trigger with the copy as e.trigObj, and the new value reads the copy (the
// copied face plus X). Prismari Apprentice's counter is gated on the value
// being 5 or greater, so a copy of Mind Spring with X=3 earns it.
func TestTriggerSpellMVLKICopyTrigger(t *testing.T) {
	sc, id := valueSpellmvScen(t, "Prismari Apprentice", "Mind Spring", "Twincast", "Island")
	pa := sc.perm(id["Prismari Apprentice"], 0)
	for i := 0; i < 7; i++ {
		sc.perm(id["Island"], 0)
	}
	ms := sc.hand(id["Mind Spring"], 0)
	tc := sc.hand(id["Twincast"], 0)
	sc.stepTo(t, 60, func() bool {
		return sc.s.sp == 0 && sc.s.npend == 0 &&
			sc.s.objs[ms].zone == kop.ZoneGraveyard && sc.s.objs[tc].zone == kop.ZoneGraveyard
	}, func(d *Decision) (Action, bool) {
		if d.Kind == DkChoose && d.Sub == ChX {
			for i := 0; i < int(d.N); i++ {
				if d.Options[i].Mode() == 3 {
					return d.Options[i], true
				}
			}
		}
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			if a.Kind() == AkCast && a.Source() == TCode(uint8(tc)) {
				return a, true
			}
		}
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			if a.Kind() == AkTarget && a.Target() == TCode(uint8(ms)) {
				return a, true
			}
		}
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(ms)) {
				return a, true
			}
		}
		return 0, false
	})
	if p, tough := sc.s.PowerToughness(pa); p != 4 || tough != 4 {
		t.Fatalf("Prismari Apprentice %d/%d after copying Mind Spring with X=3, want 4/4", p, tough)
	}
}

// TestCopyDividedRetarget: a copy of a divided-damage spell (Fire) is asked
// its new targets (CR 707.10c). The inherited division is a cast-time
// decision (CR 601.2d) and gorge's AnnouncedDivision applies it only while
// the target count is unchanged: a retarget to one object drops it and that
// object takes the whole 2; keeping both targets keeps the 1/1 split.
func TestCopyDividedRetarget(t *testing.T) {
	copyDividedScen := func(t *testing.T) (*scen, Obj, Obj, Obj, Obj, Obj) {
		t.Helper()
		sc, id := valueSpellmvScen(t, "Fire", "Twincast", "Grizzly Bears", "Mountain", "Island")
		sc.lands(0, id["Mountain"], 2)
		sc.lands(0, id["Island"], 2)
		fire := sc.hand(id["Fire"], 0)
		tc := sc.hand(id["Twincast"], 0)
		a := sc.perm(id["Grizzly Bears"], 1)
		b := sc.perm(id["Grizzly Bears"], 1)
		c := sc.perm(id["Grizzly Bears"], 1)
		return sc, fire, tc, a, b, c
	}
	cast := func(t *testing.T, sc *scen, fire, tc, a, b Obj) {
		t.Helper()
		sc.do(t, AkCast, TCode(fire), scAny)
		sc.do(t, AkTarget, scAny, TCode(a))
		sc.do(t, AkTarget, scAny, TCode(b))
		sc.do(t, AkCast, TCode(tc), scAny)
		sc.do(t, AkTarget, scAny, TCode(fire))
	}
	t.Run("one new target takes the whole total", func(t *testing.T) {
		sc, fire, tc, a, b, c := copyDividedScen(t)
		cast(t, sc, fire, tc, a, b)
		d := passToCopyAsk(t, sc.s)
		if d.Kind != DkTarget || d.Sub != SubCopyTarget || d.Player != 0 {
			t.Fatalf("want the copy's new-target ask, got kind %d sub %d player %d", d.Kind, d.Sub, d.Player)
		}
		sc.s.Apply(MakeAction(AkTarget, 0, TCode(c), 0))
		d = sc.s.Step()
		if d.Kind != DkTarget || d.Sub != SubCopyTarget {
			t.Fatalf("second pick: kind %d sub %d", d.Kind, d.Sub)
		}
		var done Action
		ok := false
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkDone {
				done, ok = d.Options[i], true
			}
		}
		if !ok {
			t.Fatalf("Done not offered once the minimum is met: %v", d.Options[:d.N])
		}
		sc.s.Apply(done)
		passToCopyResolved(t, sc.s)
		if sc.onBF(c) || sc.s.objs[c].zone != kop.ZoneGraveyard {
			t.Fatalf("one new target took %d damage, want the whole 2", 2)
		}
		if sc.s.perms[sc.slot(a)].damage != 0 || sc.s.perms[sc.slot(b)].damage != 0 {
			t.Fatalf("an inherited target took damage: %d %d", sc.s.perms[sc.slot(a)].damage, sc.s.perms[sc.slot(b)].damage)
		}
	})
	t.Run("keeping both keeps the split", func(t *testing.T) {
		sc, fire, tc, a, b, _ := copyDividedScen(t)
		cast(t, sc, fire, tc, a, b)
		passToCopyAsk(t, sc.s)
		sc.s.Apply(MakeAction(AkTarget, 0, TCode(a), 0))
		d := sc.s.Step()
		if d.Kind != DkTarget || d.Sub != SubCopyTarget {
			t.Fatalf("second pick: kind %d sub %d", d.Kind, d.Sub)
		}
		sc.s.Apply(MakeAction(AkTarget, 0, TCode(b), 0))
		passToCopyResolved(t, sc.s)
		if !sc.onBF(a) || !sc.onBF(b) {
			t.Fatalf("a kept target died: a %v b %v", sc.onBF(a), sc.onBF(b))
		}
		if sc.s.perms[sc.slot(a)].damage != 1 || sc.s.perms[sc.slot(b)].damage != 1 {
			t.Fatalf("kept targets took %d/%d damage, want the 1/1 split", sc.s.perms[sc.slot(a)].damage, sc.s.perms[sc.slot(b)].damage)
		}
	})
}
