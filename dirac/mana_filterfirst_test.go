package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// devoteeScen: p0 has Abzan Devotee ("{1}: Add {W}, {B}, or {G}. Activate
// only once each turn") and the given lands, and Doom Blade ({1}{B}) in
// hand; p1 has Grizzly Bears, its target.
func devoteeScen(t *testing.T, lands ...string) (sc *scen, devotee, blade Obj, ls []Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Abzan Devotee", Count: 1}, {Name: "Doom Blade", Count: 1},
		{Name: "Grizzly Bears", Count: 1}, {Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8}}
	set := cardsettest.Set(t, "manaabmanacost")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc = newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	for _, n := range lands {
		ls = append(ls, sc.perm(id[n], 0))
	}
	devotee = sc.perm(id["Abzan Devotee"], 0)
	sc.perm(id["Grizzly Bears"], 1)
	blade = sc.hand(id["Doom Blade"], 0)
	return sc, devotee, blade, ls
}

func bladeCast(sc *scen, blade Obj) (Action, bool) {
	d := sc.s.Step()
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && Obj(a.Source()) == blade {
			return a, true
		}
	}
	return 0, false
}

// TestDevoteeFilterPays: with two Plains and Abzan Devotee, Doom Blade's
// {1}{B} is payable only through the Devotee's filter (CR 605.3a). gorge's
// census misses it (a paid gap), so the cast is a scripted potential play
// (rules potentialPrefixScript): the first Plains funds the {1}, the
// Devotee makes {B} (its W answer leaves the pool and the tapped lands as
// they were, a state already visited: scriptStateKey reads no activation
// limit), and the planner pays the rest from the pool and the other Plains.
func TestDevoteeFilterPays(t *testing.T) {
	sc, devotee, blade, ls := devoteeScen(t, "Plains", "Plains")
	if !sc.s.scriptFilt(sc.k, 0).has(sc.slot(devotee)) {
		t.Fatal("Abzan Devotee is no census gap")
	}
	a, ok := bladeCast(sc, blade)
	if !ok {
		t.Fatal("Doom Blade's {1}{B} not offered over Abzan Devotee's filter")
	}
	sc.s.Apply(a)
	for i := 0; i < 4; i++ { // the target (the Bears), then the payment
		d := sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.objs[blade].zone != kop.ZoneStack {
		t.Fatalf("Doom Blade in zone %d, want the stack", sc.s.objs[blade].zone)
	}
	for _, o := range ls {
		if !sc.tapped(o) {
			t.Error("a Plains is untapped after the cast")
		}
	}
	if sc.tapped(devotee) {
		t.Error("Abzan Devotee tapped: its filter costs no {T}")
	}
	if sc.s.perms[sc.slot(devotee)].actUsed&1 == 0 {
		t.Error("Abzan Devotee's once-each-turn use not spent")
	}
	if pool := sc.s.pl[0].pool; pool != ([kop.NumManaKinds]uint8{}) {
		t.Errorf("pool %v after the cast, want empty", pool)
	}
	if sc.s.scriptFilt(sc.k, 0).any() {
		t.Error("the used Devotee is still a census gap")
	}
}

// TestDevoteeOncePerTurn: a Devotee used this turn is no gap (gorge's
// ActivationLimit$ gate), so Doom Blade is not offered; its use resets at
// the next turn's untap (turn.go), when it is a gap again.
func TestDevoteeOncePerTurn(t *testing.T) {
	sc, devotee, blade, _ := devoteeScen(t, "Plains", "Plains")
	sc.s.perms[sc.slot(devotee)].actUsed |= 1
	if sc.s.scriptFilt(sc.k, 0).any() {
		t.Fatal("a used Devotee is a census gap")
	}
	if _, ok := bladeCast(sc, blade); ok {
		t.Fatal("Doom Blade offered over a used Devotee")
	}
	sc.s.perms[sc.slot(devotee)].actUsed = 0
	if _, ok := bladeCast(sc, blade); !ok {
		t.Fatal("Doom Blade not offered once the Devotee's use reset")
	}
}

// TestDevoteeSickStillGap: a summoning-sick (or tapped) Devotee is still a
// gap: its filter has no {T}, which CR 302.6 alone binds.
func TestDevoteeSickStillGap(t *testing.T) {
	sc, devotee, blade, _ := devoteeScen(t, "Plains", "Plains")
	sc.s.perms[sc.slot(devotee)].flags |= pfSick | pfTapped
	if !sc.s.scriptFilt(sc.k, 0).has(sc.slot(devotee)) {
		t.Fatal("a summoning-sick, tapped Devotee is no census gap")
	}
	if _, ok := bladeCast(sc, blade); !ok {
		t.Fatal("Doom Blade not offered over a summoning-sick Devotee")
	}
}

// TestDevoteeNeedsOtherMana: alone, the Devotee's fee has no other mana to
// draw on, so gorge's census does not list it (PotentialMana's fixpoint
// never admits it) and nothing is offered over it.
func TestDevoteeNeedsOtherMana(t *testing.T) {
	sc, _, _, _ := devoteeScen(t)
	if sc.s.scriptFilt(sc.k, 0).any() {
		t.Fatal("a Devotee with no other mana is a census gap")
	}
}
