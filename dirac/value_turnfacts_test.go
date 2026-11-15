package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Tests for State.OutlastsTurn and State.NextTurnPlayer (after work by
// Daniel Brooks), the two turn
// facts a search's :waittiehorizon=untap (waittie.go laterWindow)
// reads besides InstantSpeed.

var outlastsDeck = []cardset.Entry{{Name: "Mountain", Count: 8}, {Name: "Burst Lightning", Count: 2},
	{Name: "Fanatical Firebrand", Count: 2}, {Name: "Gorehorn Raider", Count: 2}}

// TestOutlastsTurn: at seat 0's main-1 priority decision, an instant cast
// from its own hand outlasts the turn plain or kicked, and not through a
// flashback, play-permission or free-cast mode, from another zone or from
// the other player's hand; a creature with no flash in hand does not (it
// is no instant-speed move of its own); an activated ability of a
// permanent seat 0 controls does, and not when it controls it until end of
// turn, when the other player controls it, or off the battlefield; pass,
// a land play, an object id past the objects and a non-priority decision
// are false. The real decision's own Burst Lightning casts (plain and
// kicked) and the Firebrand's ability are among the true ones, and the
// test allocates nothing.
func TestOutlastsTurn(t *testing.T) {
	sc, id := newObjScen(t, outlastsDeck)
	for i := 0; i < 5; i++ {
		sc.perm(id["Mountain"], 0)
	}
	bl := sc.hand(id["Burst Lightning"], 0)
	raider := sc.hand(id["Gorehorn Raider"], 0)
	theirs := sc.hand(id["Burst Lightning"], 1)
	fb := sc.perm(id["Fanatical Firebrand"], 0)
	oppFb := sc.perm(id["Fanatical Firebrand"], 1)
	grave := sc.obj(id["Burst Lightning"], 0)
	sc.s.place(grave, kop.ZoneGraveyard, false)
	gfb := sc.obj(id["Fanatical Firebrand"], 0)
	sc.s.place(gfb, kop.ZoneGraveyard, false)
	s := sc.s
	d := s.Step()
	if d.Kind != DkPriority || d.Player != 0 || s.StepNow() != kop.StepMain1 {
		t.Fatalf("decision kind %d player %d step %d, want seat 0's main-1 priority", d.Kind, d.Player, s.StepNow())
	}
	cast := func(o Obj, mode uint8) Action { return MakeAction(AkCast, TCode(o), 0, mode) }
	act := func(o Obj) Action { return MakeAction(AkActivate, TCode(o), 0, 0) }
	for _, c := range []struct {
		name string
		a    Action
		want bool
	}{
		{"instant from hand", cast(bl, 0), true},
		{"instant from hand, kicked", cast(bl, 1), true},
		{"instant from hand, flashback mode", cast(bl, 2), false},
		{"instant from hand, play-permission mode", cast(bl, castMayPlay), false},
		{"instant from hand, free-cast mode", cast(bl, castFree), false},
		{"creature without flash from hand", cast(raider, 0), false},
		{"instant from the other player's hand", cast(theirs, 0), false},
		{"instant from the graveyard", cast(grave, 0), false},
		{"own permanent's ability", act(fb), true},
		{"the other player's permanent's ability", act(oppFb), false},
		{"an ability off the battlefield", act(gfb), false},
		{"pass", MakeAction(AkPass, 0, 0, 0), false},
		{"a land play", MakeAction(AkPlayLand, TCode(bl), 0, 0), false},
		{"an object past the objects", cast(Obj(s.nobj), 0), false},
	} {
		if got := s.OutlastsTurn(c.a); got != c.want {
			t.Errorf("%s: OutlastsTurn %v, want %v", c.name, got, c.want)
		}
	}
	// an until-end-of-turn control change: the ability is the turn's only
	sl := sc.slot(fb)
	s.perms[sl].flags |= pfCtrlEOT
	if s.OutlastsTurn(act(fb)) {
		t.Error("an ability of a permanent controlled until end of turn outlasts the turn")
	}
	s.perms[sl].flags &^= pfCtrlEOT
	// the decision's own options
	var plain, kicked, ability bool
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		switch {
		case a.Kind() == AkCast && Obj(a.Source()) == bl:
			if !s.OutlastsTurn(a) {
				t.Errorf("Burst Lightning option %v does not outlast the turn", a)
			}
			plain, kicked = plain || a.Mode() == 0, kicked || a.Mode() == 1
		case a.Kind() == AkActivate && Obj(a.Source()) == fb:
			ability = s.OutlastsTurn(a)
		case a.Kind() == AkCast && Obj(a.Source()) == raider:
			if s.OutlastsTurn(a) {
				t.Error("the Raider's cast outlasts the turn")
			}
		}
	}
	if !plain || !kicked || !ability {
		t.Fatalf("options %v: plain %v kicked %v ability %v", d.Options[:d.N], plain, kicked, ability)
	}
	if n := testing.AllocsPerRun(20, func() { s.OutlastsTurn(cast(bl, 0)) }); n != 0 {
		t.Fatalf("OutlastsTurn allocates %v", n)
	}
	ctl := s.ctl
	s.ctl = ctlStepBegin
	if s.OutlastsTurn(cast(bl, 0)) {
		t.Error("OutlastsTurn true outside a priority decision")
	}
	s.ctl = ctl
}

// TestNextTurnPlayer: the next turn is the other player's, or the active
// player's when it has an extra turn coming, and the turn that follows is
// that player's (turn.go finishCleanup), the extra turn then taken.
func TestNextTurnPlayer(t *testing.T) {
	for _, extra := range []uint8{0, 1} {
		sc, _ := newObjScen(t, outlastsDeck)
		s := sc.s
		s.pl[0].extraTurns = extra
		want := uint8(1)
		if extra > 0 {
			want = 0
		}
		if got := s.NextTurnPlayer(); got != want {
			t.Fatalf("extra turns %d: NextTurnPlayer %d, want %d", extra, got, want)
		}
		turn := s.turn
		for i := 0; i < 200 && s.turn == turn; i++ {
			d := s.Step()
			if d.Kind == DkNone {
				t.Fatal("the game ended")
			}
			a := d.Options[0]
			if d.Kind == DkPriority {
				a = MakeAction(AkPass, 0, 0, 0)
			}
			s.Apply(a)
		}
		if s.turn != turn+1 || s.active != want || s.pl[0].extraTurns != 0 {
			t.Fatalf("extra turns %d: turn %d active %d extra %d, want turn %d active %d extra 0",
				extra, s.turn, s.active, s.pl[0].extraTurns, turn+1, want)
		}
	}
}

// TestOutlastsTurnSick: a {T} (or {Q}) ability of a summoning-sick
// creature outlasts the turn only through haste that does: its own
// card's (pfSick clears only at its controller's untap step, turn.go, so
// the creature is still sick in the opponent's turn, and haste granted
// until end of turn is gone by then, finishCleanup). At seat 0's own end
// step (the window the untap-step horizon waits from) OutlastsTurn is
// false for a sick creature that has haste only until end of turn, a
// printed ability and an Equipment's granted one alike (and a printed
// haste the creature has lost for good does not count), and true for the
// same creature not sick, for a sick creature with printed haste (with
// or without an until-end-of-turn grant on top; its own ability or the
// Equipment's) and for a sick noncreature artifact; and it is true
// exactly where the ability is
// offered again: at every one of seat 0's priority windows in the
// opponent's turn that follows, and false where it is offered at none.
// A {Q} cost (no FDN card has one: the ability's flags are changed in
// the scene's own table) is read as {T} is, and a malformed option (an
// ability index past the card's, a grant's host past the objects) of a
// sick creature as turn-bound.
func TestOutlastsTurnSick(t *testing.T) {
	deck := []cardset.Entry{{Name: "Island", Count: 8}, {Name: "Strix Lookout", Count: 1}, {Name: "Rune-Sealed Wall", Count: 1},
		{Name: "Fanatical Firebrand", Count: 1}, {Name: "Ravenous Amulet", Count: 1}, {Name: "Fishing Pole", Count: 1}, {Name: "Gorehorn Raider", Count: 1},
		{Name: "Witness Protection", Count: 1}}
	// how the source has lost all its abilities, for good and before the
	// pole's grant: not, a one-shot loss (ovLoseAll), Witness Protection
	// attached (a generated table's static loss, lossAt)
	const (
		keeps = iota
		loseOnce
		loseAura
	)
	for _, c := range []struct {
		name           string
		card           string // the ability's source permanent
		sick, eotHaste bool
		pole           bool // equipped with Fishing Pole: the ability is its grant
		untap          bool // the ability's {T} made {Q}
		lose           int  // keeps, loseOnce or loseAura
		want           bool
	}{
		{"sick creature, haste until end of turn", "Strix Lookout", true, true, false, false, keeps, false},
		{"sick defender, haste until end of turn", "Rune-Sealed Wall", true, true, false, false, keeps, false},
		{"sick equipped creature, haste until end of turn: the granted ability", "Gorehorn Raider", true, true, true, false, keeps, false},
		{"sick equipped creature, printed haste lost once, haste until end of turn: the granted ability", "Fanatical Firebrand", true, true, true, false, loseOnce, false},
		{"sick equipped creature, printed haste lost to an Aura, haste until end of turn: the granted ability", "Fanatical Firebrand", true, true, true, false, loseAura, false},
		{"creature not sick, haste until end of turn", "Strix Lookout", false, true, false, false, keeps, true},
		{"creature not sick", "Strix Lookout", false, false, false, false, keeps, true},
		{"sick creature, printed haste", "Fanatical Firebrand", true, false, false, false, keeps, true},
		{"sick creature, printed haste and haste until end of turn", "Fanatical Firebrand", true, true, false, false, keeps, true},
		{"sick equipped creature, printed haste: the granted ability", "Fanatical Firebrand", true, false, true, false, keeps, true},
		{"sick artifact, no creature", "Ravenous Amulet", true, false, false, false, keeps, true},
		{"equipped creature not sick: the granted ability", "Gorehorn Raider", false, false, true, false, keeps, true},
		{"sick creature, haste until end of turn, a {Q} cost", "Strix Lookout", true, true, false, true, keeps, false},
		{"creature not sick, a {Q} cost", "Strix Lookout", false, true, false, true, keeps, true},
	} {
		sc, id := newObjScen(t, deck)
		s := sc.s
		for i := 0; i < 8; i++ {
			sc.perm(id["Island"], 0)
		}
		src := sc.perm(id[c.card], 0)
		p := &s.perms[sc.slot(src)]
		if c.sick {
			p.flags |= pfSick
		}
		if c.eotHaste {
			p.grantEOT(kop.KwHaste)
		}
		switch c.lose {
		case loseOnce:
			p.ovr.flags |= ovLoseAll
		case loseAura:
			wp := sc.perm(id["Witness Protection"], 0)
			s.perms[sc.slot(wp)].attached = sc.slot(src)
		}
		var pole Obj = NoObj
		if c.pole {
			pole = sc.perm(id["Fishing Pole"], 0)
			s.perms[sc.slot(pole)].attached = sc.slot(src)
		}
		if c.untap {
			tb := s.k().t
			cd := &tb.Cards[id[c.card]]
			abs := append([]kop.Ability(nil), cd.Abilities...)
			abs[0].Cost.Flags = abs[0].Cost.Flags&^kop.CostTap | kop.CostUntap
			cd.Abilities = abs
		}
		// the ability's option: the source's own ability, or the pole's grant
		isAb := func(a Action) bool {
			if a.Kind() != AkActivate || Obj(a.Source()) != src {
				return false
			}
			if c.pole {
				return a.Mode()&0x80 != 0 && Obj(a.Target()) == pole
			}
			return a.Mode()&0x80 == 0
		}
		var a Action
		found := false
		var windows, offered int
		for i := 0; i < 400 && s.turn < 4; i++ {
			d := s.Step()
			if d.Kind == DkNone {
				t.Fatalf("%s: the game ended", c.name)
			}
			if d.Kind == DkPriority && d.Player == 0 && s.sp == 0 {
				var here Action
				ok := false
				for j := 0; j < int(d.N); j++ {
					if isAb(d.Options[j]) {
						here, ok = d.Options[j], true
						break
					}
				}
				switch {
				case s.active == 0 && s.step == kop.StepEnd && !found:
					if !ok {
						t.Fatalf("%s: the ability is not offered at the own end step: %v", c.name, d.Options[:d.N])
					}
					a, found = here, true
					if s.NextTurnPlayer() != 1 {
						t.Fatalf("%s: the next turn is seat %d's", c.name, s.NextTurnPlayer())
					}
					if got := s.OutlastsTurn(a); got != c.want {
						t.Errorf("%s: OutlastsTurn %v, want %v", c.name, got, c.want)
					}
					if n := testing.AllocsPerRun(10, func() { s.OutlastsTurn(a) }); n != 0 {
						t.Errorf("%s: OutlastsTurn allocates %v", c.name, n)
					}
					if c.sick && c.eotHaste && c.lose == keeps {
						// an ability index past the card's, a grant's host past the objects: turn-bound
						for _, bad := range []Action{MakeAction(AkActivate, TCode(src), 0, 0x7f), MakeAction(AkActivate, TCode(src), TCode(s.nobj), 0x80)} {
							if s.OutlastsTurn(bad) {
								t.Errorf("%s: OutlastsTurn true for the malformed option %v", c.name, bad)
							}
						}
					}
				case s.active == 1 && found:
					windows++
					if ok && here == a {
						offered++
					}
				}
			}
			pick := d.Options[0]
			switch d.Kind {
			case DkPriority:
				pick = MakeAction(AkPass, 0, 0, 0)
			case DkAttackers, DkBlockers:
				for j := 0; j < int(d.N); j++ {
					if d.Options[j].Kind() == AkDone {
						pick = d.Options[j]
					}
				}
			}
			s.Apply(pick)
		}
		if !found || windows == 0 {
			t.Fatalf("%s: own end step reached %v, %d windows in the opponent's turn", c.name, found, windows)
		}
		if c.untap {
			continue // the kernel does not offer by {Q} (no FDN card has one): OutlastsTurn alone
		}
		if (offered == windows) != c.want || offered != 0 && offered != windows {
			t.Errorf("%s: offered at %d of seat 0's %d windows in the opponent's turn; OutlastsTurn should be %v", c.name, offered, windows, offered == windows)
		}
	}
}
