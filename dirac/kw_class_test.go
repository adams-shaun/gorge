package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Class enchantments (CR 716, kw_class.go) on BLB's table:
// level 1 -> 2 -> 3 in order at sorcery speed, each level's ability live
// only from its level, the "becomes level N" trigger firing once.

// classActs lists the ability indexes of o's activations offered now.
func (sc *scen) classActs(t *testing.T, o Obj) []uint8 {
	t.Helper()
	d := sc.s.Step()
	var out []uint8
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(o)) {
			out = append(out, a.Mode())
		}
	}
	return out
}

// classLevel is o's level (CtrLevel count + 1).
func (sc *scen) classLevel(o Obj) int {
	return sc.s.perms[sc.slot(o)].ctrN(sc.s, int(kop.CtrLevel)) + 1
}

// gatedTrigs counts o's triggered abilities of event ev that are live now
// (slotTriggers applies the gate).
func (sc *scen) gatedTrigs(o Obj, ev kop.TrigEvent) int {
	n := 0
	sc.s.slotTriggers(sc.k, sc.slot(o), func(a *kop.Ability) bool { return a.Trig.Event == ev }, func(trigListener, *kop.Ability) { n++ })
	return n
}

func eqActs(a []uint8, b ...uint8) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestClassLevelsInOrderAtSorcerySpeed(t *testing.T) {
	sc, id := giftScen(t, "Hunter's Talent", "Forest")
	if !sc.k.anyGate {
		t.Fatal("prep.anyGate is false on a table with a Class")
	}
	sc.lands(0, id["Forest"], 6)
	ht := sc.perm(id["Hunter's Talent"], 0)
	// level 1: only the level-2 activator (ability 0); the level-2 attack
	// trigger and the level-3 end-step trigger do not exist yet
	if got := sc.classActs(t, ht); !eqActs(got, 0) {
		t.Fatalf("level 1 activations %v, want [0] (CR 716.2a: only level 2)", got)
	}
	if sc.gatedTrigs(ht, kop.TrigAttackersDeclared) != 0 || sc.gatedTrigs(ht, kop.TrigPhase) != 0 {
		t.Fatal("a level-2/3 trigger is live at level 1")
	}
	// not at instant speed: with the activator on the stack, no level-up
	sc.do(t, AkActivate, uint8(ht), 0xff)
	if sc.classLevel(ht) != 1 {
		t.Fatal("the level rose on activation; it rises on resolution (gorge effClassLevelUp)")
	}
	if got := sc.classActs(t, ht); len(got) != 0 {
		t.Fatalf("activations %v offered with a non-empty stack; a level is gained as a sorcery", got)
	}
	sc.resolveAll(t)
	if sc.classLevel(ht) != 2 {
		t.Fatalf("level %d after the level-2 activation", sc.classLevel(ht))
	}
	if got := sc.classActs(t, ht); !eqActs(got, 1) {
		t.Fatalf("level 2 activations %v, want [1]", got)
	}
	if sc.gatedTrigs(ht, kop.TrigAttackersDeclared) != 1 || sc.gatedTrigs(ht, kop.TrigPhase) != 0 {
		t.Fatal("at level 2 the attack trigger must be live and the end-step draw not")
	}
	// a level is no counter (CR 716.4: gorge's designation)
	if m := sc.s.FeatCountersListed(ht, &[kop.NumCounterKinds2]uint16{}); m != 0 {
		t.Fatalf("counter list %#x: a Class level is listed as a counter", m)
	}
	// not in combat (sorcery timing)
	sc.passUntil(t, kop.StepBeginCombat)
	if got := sc.classActs(t, ht); len(got) != 0 {
		t.Fatalf("activations %v offered in combat", got)
	}
	for i := 0; i < 20; i++ {
		if d := sc.s.Step(); sc.s.step == kop.StepMain2 && d.Player == 0 {
			break
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	sc.do(t, AkActivate, uint8(ht), 0xff)
	sc.resolveAll(t)
	if sc.classLevel(ht) != 3 {
		t.Fatalf("level %d after the level-3 activation", sc.classLevel(ht))
	}
	if got := sc.classActs(t, ht); len(got) != 0 {
		t.Fatalf("level 3 activations %v, want none", got)
	}
	if sc.gatedTrigs(ht, kop.TrigAttackersDeclared) != 1 || sc.gatedTrigs(ht, kop.TrigPhase) != 1 {
		t.Fatal("at level 3 both gated triggers must be live")
	}
}

func TestClassLevelGainedFiresOnce(t *testing.T) {
	sc, id := giftScen(t, "Stormchaser's Talent", "Island", "Long River's Pull")
	sc.lands(0, id["Island"], 10)
	st := sc.perm(id["Stormchaser's Talent"], 0)
	lr := sc.obj(id["Long River's Pull"], 0)
	sc.s.place(lr, kop.ZoneGraveyard, false)
	h := sc.s.pl[0].handN
	sc.do(t, AkActivate, uint8(st), 0xff)
	sc.resolveAll(t)
	if sc.classLevel(st) != 2 {
		t.Fatalf("level %d", sc.classLevel(st))
	}
	if sc.s.objs[lr].zone != kop.ZoneHand || sc.s.pl[0].handN != h+1 {
		t.Fatalf("Long River's Pull zone %d hand %d: the level-2 trigger returns it", sc.s.objs[lr].zone, sc.s.pl[0].handN)
	}
	// level 3: the level-2 trigger does not fire again
	sc.s.place(sc.obj(id["Long River's Pull"], 0), kop.ZoneGraveyard, false)
	g := sc.s.pl[0].graveN
	sc.do(t, AkActivate, uint8(st), 0xff)
	sc.resolveAll(t)
	if sc.classLevel(st) != 3 || sc.s.pl[0].graveN != g {
		t.Fatalf("level %d graveyard %d (was %d): \"becomes level 2\" fired at level 3", sc.classLevel(st), sc.s.pl[0].graveN, g)
	}
}

// TestClassGatedTriggerRecheckedOnResolution: gorge ANDs a level band into
// the resolution-time intervening-if walk (triggerConditionHoldsWithSVars),
// so a level-2 trigger whose Class left the battlefield does nothing.
func TestClassGatedTriggerRecheckedOnResolution(t *testing.T) {
	for _, leave := range []bool{false, true} {
		sc, id := giftScen(t, "Hunter's Talent", "Forest", "Jolly Gerbils")
		ht := sc.perm(id["Hunter's Talent"], 0)
		sc.s.perms[sc.slot(ht)].setCtrN(sc.s, int(kop.CtrLevel), 1) // level 2
		jg := sc.perm(id["Jolly Gerbils"], 0)
		sc.passUntil(t, kop.StepAttackers)
		sc.do(t, AkAttack, uint8(jg), 0xff)
		sc.do(t, AkDone, 0xff, 0xff)
		// the trigger goes on the stack, targeting the only attacker
		for i := 0; i < 10 && sc.s.sp == 0; i++ {
			d := sc.s.Step()
			if d.Kind == DkPriority {
				t.Fatal("no attack trigger at level 2")
			}
			sc.s.Apply(d.Options[0])
		}
		if leave {
			sc.s.leave(sc.k, sc.slot(ht), kop.ZoneGraveyard, false)
		}
		sc.resolveAll(t)
		p, _ := sc.s.pt(sc.k, sc.slot(jg))
		want := 3
		if leave {
			want = 2
		}
		if p != want {
			t.Fatalf("Talent left=%v: Gerbils power %d, want %d", leave, p, want)
		}
	}
}
