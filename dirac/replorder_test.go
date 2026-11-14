package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// replScen is a generated-table scenario over deck (both players), with the
// card ids by name.
func replScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
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
	return newScenTable(t, tbl, Config{NoElide: true}), id
}

// burnToAsk casts Burst Lightning (unkicked) from player 0 at target code,
// lets it resolve and returns the next decision.
func burnToAsk(t *testing.T, sc *scen, id map[string]CardID, code TCode) Decision {
	t.Helper()
	s := sc.s
	sc.lands(0, id["Mountain"], 1)
	bl := sc.hand(id["Burst Lightning"], 0)
	sc.do(t, AkCast, TCode(bl), scAny)
	for i := 0; i < 8; i++ {
		d := s.Step()
		switch {
		case d.Kind == DkTarget:
			sc.do(t, AkTarget, scAny, code)
			continue
		case d.Kind == DkChoose && d.Sub != ChGenReplOrder:
			s.Apply(d.Options[0]) // the kicker: not paid (option order: decline first)
			continue
		case d.Kind == DkPriority && s.sp > 0:
			s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		return d
	}
	t.Fatal("no decision after Burst Lightning resolved")
	return Decision{}
}

// TestReplOrderNoncombatTwoTwinflames: two Twinflame Tyrants double the
// burn to the opponent twice. Gorge parks the damage event on its CR 616.1
// order ask (replacement.go, the event held as a Note), the resolution
// finishes (the spell is in the graveyard), the opponent is asked which
// Twinflame applies first, and the damage is dealt by the answer.
func TestReplOrderNoncombatTwoTwinflames(t *testing.T) {
	deck := []cardset.Entry{{Name: "Twinflame Tyrant", Count: 2}, {Name: "Burst Lightning", Count: 1},
		{Name: "Eager Trufflesnout", Count: 4}, {Name: "Mountain", Count: 10}}
	sc, id := replScen(t, deck)
	s := sc.s
	t0 := sc.perm(id["Twinflame Tyrant"], 0)
	t1 := sc.perm(id["Twinflame Tyrant"], 0)
	d := burnToAsk(t, sc, id, TCode(PlayerTarget1))
	if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.Player != 1 || d.N != 2 {
		t.Fatalf("want the opponent's order ask over the two Twinflames, got kind %d sub %d player %d opts %v", d.Kind, d.Sub, d.Player, d.Options[:d.N])
	}
	if s.pl[1].life != 20 || s.sp != 1 {
		t.Fatalf("before the answer: life %d stack %d, want 20 and the spell still resolving", s.pl[1].life, s.sp)
	}
	for i := 0; i < int(d.N); i++ {
		if c := Obj(d.Options[i].Source()); c != t0 && c != t1 {
			t.Fatalf("option source %d is not a Twinflame", c)
		}
	}
	s.Apply(d.Options[0])
	d = s.Step()
	if s.pl[1].life != 12 || s.sp != 0 {
		t.Fatalf("after the answer: life %d stack %d, want 12 (2 doubled twice) and the spell resolved", s.pl[1].life, s.sp)
	}
	if d.Kind != DkPriority || d.Player != 0 {
		t.Fatalf("after the answer: kind %d player %d, want the active player's priority", d.Kind, d.Player)
	}
}

// TestReplOrderNoncombatBarricade: Twinflame Tyrant's doubling and the
// opponent's Crystal Barricade's prevention both match burn to the
// opponent's other creature; the creature's controller is asked, and the
// damage is prevented whichever applies first.
func TestReplOrderNoncombatBarricade(t *testing.T) {
	deck := []cardset.Entry{{Name: "Twinflame Tyrant", Count: 1}, {Name: "Crystal Barricade", Count: 1},
		{Name: "Burst Lightning", Count: 1}, {Name: "Eager Trufflesnout", Count: 4}, {Name: "Mountain", Count: 10}}
	for pick := 0; pick < 2; pick++ {
		sc, id := replScen(t, deck)
		s := sc.s
		sc.perm(id["Twinflame Tyrant"], 0)
		sc.perm(id["Crystal Barricade"], 1)
		c := sc.perm(id["Eager Trufflesnout"], 1)
		d := burnToAsk(t, sc, id, TCode(c))
		if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.Player != 1 || d.N != 2 {
			t.Fatalf("want the creature controller's order ask, got kind %d sub %d player %d opts %v", d.Kind, d.Sub, d.Player, d.Options[:d.N])
		}
		s.Apply(d.Options[pick])
		s.Step()
		if !sc.onBF(c) || s.perms[sc.slot(c)].damage != 0 {
			t.Fatalf("pick %d: the creature took damage or died", pick)
		}
	}
}

// TestReplOrderEachPlayerTwoTwinflames: Slagstorm's each-player mode deals 3
// to both players while two Twinflame Tyrants double the opponent's event.
// rbDblDmg matches only the caster's opponents, so the caster's own event
// never parks; the caster is always the active player (Slagstorm is a
// sorcery), the kernel deals active-then-other, and the opponent's event is
// the one asked. The OpDealDamage RefEachPlayer arm deals the players
// through the resumable damage sequence (repl_gen.go) so a park on either
// player event resumes; the previous two direct damageTo calls also passed
// this test because only the second event can park.
func TestReplOrderEachPlayerTwoTwinflames(t *testing.T) {
	deck := []cardset.Entry{{Name: "Twinflame Tyrant", Count: 2}, {Name: "Slagstorm", Count: 1},
		{Name: "Eager Trufflesnout", Count: 4}, {Name: "Mountain", Count: 10}}
	sc, id := replScen(t, deck)
	s := sc.s
	t0 := sc.perm(id["Twinflame Tyrant"], 0)
	t1 := sc.perm(id["Twinflame Tyrant"], 0)
	sc.lands(0, id["Mountain"], 3)
	sg := sc.hand(id["Slagstorm"], 0)
	sc.do(t, AkCast, TCode(sg), scAny)
	var d Decision
	for i := 0; i < 16; i++ {
		d = s.Step()
		if d.Kind == DkModes {
			picked := false
			for j := 0; j < int(d.N); j++ {
				if d.Options[j].Mode() == 1 { // the each-player mode (modes 0-indexed)
					s.Apply(d.Options[j])
					picked = true
					break
				}
			}
			if !picked {
				t.Fatalf("modes decision %+v, want an each-player mode", d.Options[:d.N])
			}
			continue
		}
		if d.Kind == DkPriority && s.sp > 0 {
			s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		break
	}
	if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.Player != 1 || d.N != 2 {
		t.Fatalf("want the opponent's order ask over the two Twinflames, got kind %d sub %d player %d opts %v", d.Kind, d.Sub, d.Player, d.Options[:d.N])
	}
	if s.pl[0].life != 17 || s.pl[1].life != 20 || s.sp != 1 || s.objs[sg].zone != kop.ZoneStack {
		t.Fatalf("before the answer: life %d/%d stack %d spell zone %d, want 17/20, the spell resolving on the stack", s.pl[0].life, s.pl[1].life, s.sp, s.objs[sg].zone)
	}
	for i := 0; i < int(d.N); i++ {
		if c := Obj(d.Options[i].Source()); c != t0 && c != t1 {
			t.Fatalf("option source %d is not a Twinflame", c)
		}
	}
	s.Apply(d.Options[0])
	d = s.Step()
	if s.pl[0].life != 17 || s.pl[1].life != 8 || s.sp != 0 || s.objs[sg].zone != kop.ZoneGraveyard {
		t.Fatalf("after the answer: life %d/%d stack %d spell zone %d, want 17/8, resolved, the spell in the graveyard", s.pl[0].life, s.pl[1].life, s.sp, s.objs[sg].zone)
	}
	if d.Kind != DkPriority || d.Player != 0 {
		t.Fatalf("after the answer: kind %d player %d, want the active player's priority", d.Kind, d.Player)
	}
}

// TestDamageAllReplOrderEveryEvent: Seismic Rupture sweeps a board where one
// player has a Crystal Barricade and the caster a (flying) Twinflame Tyrant.
// Gorge parks EVERY damage event two replacements match, asking the affected player once per event in sweep
// order (CR 616.1), and the spell stays on the stack until the last answer.
// The kernel's OpDamageAll must park and resume between events, not ask only
// for the first one.
func TestDamageAllReplOrderEveryEvent(t *testing.T) {
	// Two Barricades would be two bare preventions, which neither engine
	// orders (repl_bareprev.go); the Tyrant's doubling does not commute.
	deck := []cardset.Entry{{Name: "Seismic Rupture", Count: 1}, {Name: "Crystal Barricade", Count: 1},
		{Name: "Twinflame Tyrant", Count: 1}, {Name: "Savannah Lions", Count: 4}, {Name: "Mountain", Count: 10}}
	sc, id := replScen(t, deck)
	s := sc.s
	tyrant := sc.perm(id["Twinflame Tyrant"], 0)
	b0 := sc.perm(id["Crystal Barricade"], 1)
	l0 := sc.perm(id["Savannah Lions"], 1)
	l1 := sc.perm(id["Savannah Lions"], 1)
	sc.perm(id["Savannah Lions"], 0) // no Barricade: no ask for its event
	sc.lands(0, id["Mountain"], 3)
	sr := sc.hand(id["Seismic Rupture"], 0)
	sc.do(t, AkCast, TCode(sr), scAny)

	var asked []Obj
	for i := 0; i < 12; i++ {
		d := s.Step()
		if d.Kind == DkPriority && s.sp > 0 {
			s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		if d.Kind == DkChoose && d.Sub == ChGenReplOrder {
			if d.Player != 1 || d.N != 2 {
				t.Fatalf("ask: kind %d sub %d player %d opts %v, want the Barricade controller's two options", d.Kind, d.Sub, d.Player, d.Options[:d.N])
			}
			if Obj(d.Options[0].Source()) != tyrant || Obj(d.Options[1].Source()) != b0 {
				t.Fatalf("ask options %d, %d, want gorge's scan order: the Tyrant, then the Barricade (%d, %d)", d.Options[0].Source(), d.Options[1].Source(), tyrant, b0)
			}
			if s.sp != 1 || s.objs[sr].zone != kop.ZoneStack {
				t.Fatalf("before the answer: stack %d spell zone %d, want the spell still resolving", s.sp, s.objs[sr].zone)
			}
			asked = append(asked, Obj(s.g.nc.code))
			s.Apply(d.Options[0])
			continue
		}
		if d.Kind == DkPriority && s.sp == 0 {
			break
		}
		t.Fatalf("unexpected decision kind %d sub %d player %d", d.Kind, d.Sub, d.Player)
	}
	if len(asked) != 2 || asked[0] != l0 || asked[1] != l1 {
		t.Fatalf("asks %v, want one per affected event in sweep order (%d, %d)", asked, l0, l1)
	}
	if s.objs[sr].zone != kop.ZoneGraveyard || s.sp != 0 {
		t.Fatalf("after the answers: spell zone %d stack %d, want it resolved", s.objs[sr].zone, s.sp)
	}
	for _, l := range []Obj{l0, l1} {
		if !sc.onBF(l) || s.perms[sc.slot(l)].damage != 0 {
			t.Fatalf("lion %d: on battlefield %v damage %d, want it prevented", l, sc.onBF(l), s.perms[sc.slot(l)].damage)
		}
	}
}

// TestReplOrderCombatNoDeathAtPause: a 4-power attacker (its combat damage to
// it prevented, Fleeting Flight's keyword) is blocked by a Twinflame Tyrant
// first and a Balmor second; its controller divides 4/0, and the walk deals the
// attacker's 4 to Balmor. Balmor's lethal damage waits for the end of the pass
// (CR 704.3), so it is still there at the second order ask, and the unblocked
// wizard's damage to the defender waits for that answer.
func TestReplOrderCombatNoDeathAtPause(t *testing.T) {
	deck := []cardset.Entry{{Name: "Twinflame Tyrant", Count: 1}, {Name: "Erudite Wizard", Count: 2},
		{Name: "Balmor, Battlemage Captain", Count: 1}, {Name: "Eager Trufflesnout", Count: 4}, {Name: "Mountain", Count: 12}}
	sc, id := replScen(t, deck)
	s := sc.s
	w0 := sc.perm(id["Erudite Wizard"], 0) // attacks, blocked twice
	w1 := sc.perm(id["Erudite Wizard"], 0) // attacks unblocked
	ty := sc.perm(id["Twinflame Tyrant"], 1)
	ba := sc.perm(id["Balmor, Battlemage Captain"], 1)
	s.perms[sc.slot(w0)].eotP = 2 // 4 power
	s.perms[sc.slot(w1)].eotP = 1 // 3 power
	s.perms[sc.slot(w0)].eotKw |= kop.KwPreventCombatDamageTo

	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(w0), scAny)
	sc.do(t, AkAttack, TCode(w1), scAny)
	sc.do(t, AkDone, scAny, scAny)
	sc.passUntil(t, kop.StepBlockers)
	sc.do(t, AkBlock, TCode(ty), TCode(w0))
	sc.do(t, AkBlock, TCode(ba), TCode(w0))
	sc.do(t, AkDone, scAny, scAny)

	var asks int
	for i := 0; i < 40; i++ {
		d := s.Step()
		switch {
		case d.Kind == DkChoose && d.Sub == ChDivide:
			// (0, 4): the Tyrant blocks first and gets nothing, Balmor lethal
			s.Apply(d.Options[0])
		case d.Kind == DkChoose && d.Sub == ChGenReplOrder:
			asks++
			if asks > 2 {
				t.Fatalf("more than two order asks")
			}
			if d.Player != 0 || d.N != 2 {
				t.Fatalf("ask %d: kind %d sub %d player %d opts %v, want the attacker's controller's two options", asks, d.Kind, d.Sub, d.Player, d.Options[:d.N])
			}
			// the options come canonically sorted by source; the Tyrant is
			// gorge's scan index 0 (the doubling), the prevention index 1
			var prev *Action
			for i := 0; i < 2; i++ {
				src, md := Obj(d.Options[i].Source()), d.Options[i].Mode()
				if src == ty && md != 0 || src == w0 && md != 1 {
					t.Fatalf("ask %d: option %d is %d mode %d, want the Tyrant (mode 0) and the prevention (mode 1)", asks, i, src, md)
				}
				if src == w0 {
					prev = &d.Options[i]
				}
			}
			if prev == nil {
				t.Fatalf("ask %d: no prevention option", asks)
			}
			if asks == 2 {
				if !sc.onBF(ba) {
					t.Fatalf("Balmor died at the pause")
				}
				if s.pl[1].life != 20 {
					t.Fatalf("at the second ask the unblocked wizard's damage is already dealt (life %d)", s.pl[1].life)
				}
			}
			s.Apply(*prev) // the prevention: nothing is dealt
		case d.Kind == DkPriority:
			if asks < 2 {
				s.Apply(MakeAction(AkPass, 0, 0, 0))
				continue
			}
			if s.pl[1].life != 17 {
				t.Fatalf("life %d, want 17 (the unblocked wizard's 3, after both asks)", s.pl[1].life)
			}
			if !sc.onBF(w0) || !sc.onBF(ty) || !sc.onBF(w1) || sc.onBF(ba) {
				t.Fatalf("w0 %v ty %v w1 %v ba %v, want the wizards and the Tyrant alive, Balmor dead", sc.onBF(w0), sc.onBF(ty), sc.onBF(w1), sc.onBF(ba))
			}
			return
		default:
			t.Fatalf("unexpected decision kind %d sub %d player %d opts %v", d.Kind, d.Sub, d.Player, d.Options[:d.N])
		}
	}
	t.Fatal("no end of combat")
}

// TestDamageAllReplOrderCrossSeat: the same sweep with a competition on
// BOTH battlefields, the permanents created so that slot order runs opposite
// to gorge's sweep (p1's board entered first, p0 casts). Gorge's
// effDamageAll walks seat 0's battlefield in entry order, then seat 1's. p0's
// Lion is matched by two bare preventions (two Barricades), which neither
// side orders (repl_bareprev.go), so the walk passes it unasked although its
// event comes first; p1's Lion (one Barricade, the caster's Twinflame Tyrant)
// is asked. No FDN card gives the caster's own creature a competition that
// does not commute, so a second ask cannot be built here.
func TestDamageAllReplOrderCrossSeat(t *testing.T) {
	deck := []cardset.Entry{{Name: "Seismic Rupture", Count: 1}, {Name: "Crystal Barricade", Count: 3},
		{Name: "Twinflame Tyrant", Count: 1}, {Name: "Savannah Lions", Count: 4}, {Name: "Mountain", Count: 10}}
	sc, id := replScen(t, deck)
	s := sc.s
	// p1's board first: its slots are lower than p0's
	b10 := sc.perm(id["Crystal Barricade"], 1)
	l1 := sc.perm(id["Savannah Lions"], 1)
	sc.perm(id["Crystal Barricade"], 0)
	sc.perm(id["Crystal Barricade"], 0)
	l0 := sc.perm(id["Savannah Lions"], 0)
	tyrant := sc.perm(id["Twinflame Tyrant"], 0)
	if sc.slot(l1) > sc.slot(l0) {
		t.Fatalf("probe shape: l1 slot %d l0 slot %d, want p1's slots lower", sc.slot(l1), sc.slot(l0))
	}
	sc.lands(0, id["Mountain"], 3)
	sr := sc.hand(id["Seismic Rupture"], 0)
	sc.do(t, AkCast, TCode(sr), scAny)

	var asked []Obj
	for i := 0; i < 16; i++ {
		d := s.Step()
		if d.Kind == DkPriority && s.sp > 0 {
			s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		if d.Kind == DkChoose && d.Sub == ChGenReplOrder {
			if d.Player != 1 || d.N != 2 {
				t.Fatalf("ask %d: kind %d sub %d player %d opts %v, want player 1's two options", len(asked), d.Kind, d.Sub, d.Player, d.Options[:d.N])
			}
			if Obj(s.g.nc.code) != l1 {
				t.Fatalf("ask %d: event for %d, want p1's Lion %d (p0's has only bare preventions)", len(asked), s.g.nc.code, l1)
			}
			if o0, o1 := Obj(d.Options[0].Source()), Obj(d.Options[1].Source()); !(o0 == tyrant && o1 == b10 || o0 == b10 && o1 == tyrant) {
				t.Fatalf("ask %d: options %d, %d, want the Tyrant and p1's Barricade (%d, %d)", len(asked), o0, o1, tyrant, b10)
			}
			if s.sp != 1 || s.objs[sr].zone != kop.ZoneStack {
				t.Fatalf("ask %d: stack %d spell zone %d, want the spell still resolving", len(asked), s.sp, s.objs[sr].zone)
			}
			asked = append(asked, l1)
			s.Apply(d.Options[0])
			continue
		}
		if d.Kind == DkPriority && s.sp == 0 {
			break
		}
		t.Fatalf("unexpected decision kind %d sub %d player %d", d.Kind, d.Sub, d.Player)
	}
	if len(asked) != 1 {
		t.Fatalf("asks %v, want p1's Lion alone (%d)", asked, l1)
	}
	if s.objs[sr].zone != kop.ZoneGraveyard || s.sp != 0 {
		t.Fatalf("after the answers: spell zone %d stack %d, want it resolved", s.objs[sr].zone, s.sp)
	}
	for _, l := range []Obj{l0, l1} {
		if !sc.onBF(l) || s.perms[sc.slot(l)].damage != 0 {
			t.Fatalf("lion %d: on battlefield %v damage %d, want it prevented", l, sc.onBF(l), s.perms[sc.slot(l)].damage)
		}
	}
}
