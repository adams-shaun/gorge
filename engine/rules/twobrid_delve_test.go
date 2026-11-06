package rules

// Lockstep pair 836 (std-tdm, Gurmag Nightwatch under Teval, Arbiter of
// Virtue's delve grant): a spell whose cost is all twobrid pips has a
// resolved generic total only after CR 601.2b announces each pip, so the
// delve ask must defer past the pip announcements. Before the defer the ask
// read zero generic, burned delveDone, and the pip announcements (priced
// with no delve at all) refused every face -- the cast the offer gate had
// just offered stranded. The re-run in continueCast poses the ask with a
// real shortfall once every pip is announced, and announceFeasible prices
// the announcements with the graveyard's potential.
//
// The fixtures are probe cards so the shape stays pinned regardless of what
// the corpus printings do; the delve grant is Teval's stack-scoped static,
// the same shape the divergence ran under.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const twobridGurmagName = "Gurmag Nightwatch Probe"

func twobridGurmag(t *testing.T) *cards.Card {
	t.Helper()
	return card(t, "Name:"+twobridGurmagName+"\nManaCost:2B 2G 2U\nTypes:Creature Human Ranger\nPT:3/3\nOracle:x")
}

func twobridTeval(t *testing.T) *cards.Card {
	t.Helper()
	return card(t, "Name:Teval Arbiter Probe\nManaCost:0\nTypes:Creature Dragon\nPT:6/6\n"+
		"S:Mode$ Continuous | Affected$ Card.YouCtrl+wasCast | AffectedZone$ Stack | AddKeyword$ Delve\n"+
		"A:SP$ Draw | NumCards$ 1\nOracle:x")
}

// twobridDelveEngine reaches p0's first main phase with the twobrid delve
// spell in hand, Teval on the battlefield granting delve to spells, n bears
// in the graveyard, and the pool funded with the given symbols. All zone
// surgery happens through events followed by a priority round, so the menu
// the test reads is recomputed from the post-surgery state.
func twobridDelveEngine(t *testing.T, reg *cards.Registry, n int, pool string) (*Engine, state.ObjID) {
	t.Helper()
	gurmag := twobridGurmag(t)
	teval := twobridTeval(t)
	bear := card(t, "Name:Bear Probe\nManaCost:0\nTypes:Creature Bear\nPT:1/1\nOracle:x")
	deck0 := []*cards.Card{gurmag, teval}
	for len(deck0) < 40 {
		deck0 = append(deck0, bear)
	}
	deck1 := make([]*cards.Card, 40)
	for i := range deck1 {
		deck1[i] = lookup(t, reg, "Mountain")
	}
	e := New(Config{Seed: 42, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{deck0, deck1}})
	e.Advance()
	toMain1(t, e)
	// Empty the hand back into the library so the board is exactly the
	// fixture's: the spell in hand, Teval in play, n bears in the graveyard.
	for _, id := range append([]state.ObjID{}, e.G.Zone(state.ZHand, 0)...) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
	}
	// The opening hand is shuffled, so the spell is wherever the shuffle put
	// it: pull it back into the hand by name.
	gid := pullByName(t, e, 0, twobridGurmagName)
	e.emit(events.Event{Kind: events.MoveZone, Obj: gid, From: e.G.Obj(gid).Zone, To: state.ZHand})
	if f := e.G.Obj(gid).Face(); f == nil || f.Name != twobridGurmagName {
		t.Fatalf("precondition: hand holds %v, want the spell", e.G.Zone(state.ZHand, 0))
	}
	for _, name := range []string{"Teval Arbiter Probe"} {
		id := pullByName(t, e, 0, name)
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: e.G.Obj(id).Zone, To: state.ZBattlefield})
	}
	for i := 0; i < n; i++ {
		id := pullByName(t, e, 0, "Bear Probe")
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: e.G.Obj(id).Zone, To: state.ZGraveyard})
	}
	for _, r := range pool {
		e.emit(events.Event{Kind: events.ManaAdd, Player: state.PlayerID(0), Counter: string(r), Amount: 1})
	}
	if got := len(e.G.Zone(state.ZGraveyard, 0)); got != n {
		t.Fatalf("precondition: graveyard holds %d cards, want %d", got, n)
	}
	if got := len(e.G.Zone(state.ZBattlefield, 0)); got != 1 {
		t.Fatalf("precondition: battlefield holds %d objects, want 1 (Teval)", got)
	}
	e.priorityRound()
	return e, pullByName(t, e, 0, twobridGurmagName)
}

// twobridCastOption finds the priority option that casts the spell.
func twobridCastOption(t *testing.T, e *Engine, d *decision.Decision) *decision.Option {
	t.Helper()
	for i := range d.Options {
		o := &d.Options[i]
		if o.Kind == "cast" {
			if ob := e.G.Obj(state.ObjID(o.Obj)); ob != nil && ob.Face() != nil && ob.Face().Name == twobridGurmagName {
				return o
			}
		}
	}
	return nil
}

// driveTwobridCast answers every pending decision after the cast submit:
// the delve exile ask is answered with as many exile picks as the decision's
// max allows, a pip announcement prefers the coloured face it names (the
// pool-funded case) and otherwise takes the only face offered, and anything
// else goes through the policy bot. It returns the asks it served.
func driveTwobridCast(t *testing.T, e *Engine, gurmag state.ObjID, b *testBot) []string {
	t.Helper()
	var served []string
	for n := 0; e.Pending() != nil && n < 200; n++ {
		d := e.Pending()
		desc := fmt.Sprintf("%s prompt=%q opts=[", d.Kind, d.Prompt)
		for i, o := range d.Options {
			if i > 0 {
				desc += " "
			}
			desc += o.Kind
		}
		desc += "]"
		served = append(served, desc)
		var in decision.Intent
		switch {
		case d.Player != 0:
			in = b.answer(e, d)
		case strings.HasPrefix(d.Prompt, "Delve: exile cards"):
			var picks []int
			for i, o := range d.Options {
				if o.Kind == "exile" && i < d.Max {
					picks = append(picks, int(o.Index))
				}
			}
			in = decision.Intent{Seq: d.Seq, Player: d.Player, Choices: picks}
		default:
			// A pip announcement (Min 1 Max 1, only pay_* faces): take the
			// first coloured face when one is offered, else the generic face
			// -- the shape a paying player announces under CR 601.2b.
			// Anything else goes through the policy bot.
			pipAsk := d.Min == 1 && d.Max == 1
			if pipAsk {
				for _, o := range d.Options {
					if !strings.HasPrefix(o.Kind, "pay_") {
						pipAsk = false
						break
					}
				}
			}
			if !pipAsk {
				in = b.answer(e, d)
			} else {
				pick := -1
				for _, o := range d.Options {
					if o.Kind != "pay_generic" {
						pick = int(o.Index)
						break
					}
				}
				if pick < 0 {
					pick = int(d.Options[0].Index)
				}
				in = decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pick}}
			}
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("submit %s: %v", desc, err)
		}
		if ob := e.G.Obj(gurmag); ob != nil && ob.Zone == state.ZBattlefield {
			return served
		}
	}
	if ob := e.G.Obj(gurmag); ob == nil || ob.Zone != state.ZBattlefield {
		zone := any("gone")
		if ob != nil {
			zone = ob.Zone
		}
		for i := len(e.L.Events) - 12; i < len(e.L.Events); i++ {
			if i >= 0 {
				t.Logf("event %d: %v", i, e.L.Events[i])
			}
		}
		t.Fatalf("the cast did not enter: zone=%v pending=%v served=%v", zone, e.Pending(), served)
	}
	return served
}

// pipKinds extracts the option kinds of the pip-announcement asks (pay_*) in
// the order they were served.
func hasKind(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func pipKinds(served []string) [][]string {
	var out [][]string
	for _, s := range served {
		i := strings.Index(s, "opts=[")
		if i < 0 {
			continue
		}
		inner := strings.TrimSuffix(s[i+len("opts=["):], "]")
		if !strings.HasPrefix(inner, "pay_") {
			continue
		}
		out = append(out, strings.Split(inner, " "))
	}
	return out
}

// TestTwobridDelveDefersPastPipAnnouncements: with an empty pool and six
// graveyard cards the offer gate offers the cast (the offer credits the
// graveyard's potential), the pip announcements offer only the generic faces
// -- the coloured half of a pip is not payable and the announcements are
// priced with the potential delve -- and the deferred ask, re-run once every
// pip is announced, exiles all six cards and the cast enters.
func TestTwobridDelveDefersPastPipAnnouncements(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, gurmag := twobridDelveEngine(t, reg, 6, "")
	d := e.Pending()
	if d == nil {
		t.Fatal("no pending priority decision")
	}
	opt := twobridCastOption(t, e, d)
	if opt == nil {
		t.Fatalf("precondition: the walk does not offer the twobrid delve cast: %v",
			optKinds(e.legalActions(0)))
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Acting(), Choices: []int{int(opt.Index)}}); err != nil {
		t.Fatalf("submit cast: %v", err)
	}
	served := driveTwobridCast(t, e, gurmag, newTestBot(9))
	pips := pipKinds(served)
	if len(pips) != 3 {
		t.Fatalf("got %d pip-announcement asks, want 3: %v", len(pips), served)
	}
	for i, kinds := range pips {
		if len(kinds) != 1 || kinds[0] != "pay_generic" {
			t.Fatalf("pip %d offered %v, want only the generic face (the coloured half is not payable and delve is not picked yet)", i, kinds)
		}
	}
	// The deferred ask exiled what the announced cost needed: six cards for
	// six generic. Nothing was exiled before the pips were announced.
	if got := len(e.G.Zone(state.ZExile, 0)); got != 6 {
		t.Fatalf("exile holds %d cards, want 6 (one per announced generic)", got)
	}
	if got := len(e.G.Zone(state.ZGraveyard, 0)); got != 0 {
		t.Fatalf("graveyard holds %d cards, want 0", got)
	}
}

// TestTwobridDelveColouredFacesFromPool: the pool pays the coloured halves
// the delve never could, and the deferred ask covers only the generic
// remainder. The first pip must be announced through the pool's {B} -- the
// generic face is not payable once the whole cost is priced -- and the
// second through the delve-covered generic face.
func TestTwobridDelveColouredFacesFromPool(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, gurmag := twobridDelveEngine(t, reg, 3, "BU")
	d := e.Pending()
	if d == nil {
		t.Fatal("no pending priority decision")
	}
	opt := twobridCastOption(t, e, d)
	if opt == nil {
		t.Fatalf("precondition: the walk does not offer the pool-funded twobrid delve cast: %v",
			optKinds(e.legalActions(0)))
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Acting(), Choices: []int{int(opt.Index)}}); err != nil {
		t.Fatalf("submit cast: %v", err)
	}
	served := driveTwobridCast(t, e, gurmag, newTestBot(9))
	pips := pipKinds(served)
	if len(pips) != 3 {
		t.Fatalf("got %d pip-announcement asks, want 3: %v", len(pips), served)
	}
	// {2/B}: the pool's {B} is offered; the pool's {G} face is not (no green
	// mana exists). The generic face may also be offered -- with three
	// graveyard cards the delve credit can eat its {2} once the whole cost is
	// priced -- but the green face must not.
	if !hasKind(pips[0], "pay_B") || hasKind(pips[0], "pay_G") {
		t.Fatalf("pip 0 offered %v, want pay_B and no pay_G (the pool has no {G})", pips[0])
	}
	if len(pips[1]) != 1 || pips[1][0] != "pay_generic" {
		t.Fatalf("pip 1 offered %v, want only pay_generic (the pool has no {G})", pips[1])
	}
	if got := len(e.G.Zone(state.ZExile, 0)); got == 0 {
		t.Fatal("the deferred delve ask exiled nothing")
	}
	if pl := &e.G.Players[0]; pl.Pool.Total() != 0 {
		t.Fatalf("pool holds %v, want drained", pl.Pool)
	}
}

// TestTwobridDelveOneCardIsNotEnough: with one graveyard card the potential
// cannot cover the announced generics, so the offer gate withholds the cast.
func TestTwobridDelveOneCardIsNotEnough(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, _ := twobridDelveEngine(t, reg, 1, "")
	d := e.Pending()
	if d == nil {
		t.Fatal("no pending priority decision")
	}
	if opt := twobridCastOption(t, e, d); opt != nil {
		t.Fatal("the walk offered a cast its potential cannot pay")
	}
}

// TestPlainDelveAsksBeforePips pins the defer's boundary: a cost with no
// twobrid pip has its resolved generic from the start, so its delve ask runs
// BEFORE the pip announcements, exactly as before -- the first ask after the
// cast submit is the exile ask.
func TestPlainDelveAsksBeforePips(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	plain := card(t, "Name:Plain Delve Probe\nManaCost:5 B\nTypes:Creature Nightmare\nPT:5/5\nK:Delve\nOracle:x")
	bear := card(t, "Name:Bear Probe\nManaCost:0\nTypes:Creature Bear\nPT:1/1\nOracle:x")
	deck0 := []*cards.Card{plain}
	for len(deck0) < 40 {
		deck0 = append(deck0, bear)
	}
	deck1 := make([]*cards.Card, 40)
	for i := range deck1 {
		deck1[i] = lookup(t, reg, "Mountain")
	}
	e := New(Config{Seed: 7, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{deck0, deck1}})
	e.Advance()
	toMain1(t, e)
	for _, id := range append([]state.ObjID{}, e.G.Zone(state.ZHand, 0)...) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
	}
	pid := pullByName(t, e, 0, "Plain Delve Probe")
	e.emit(events.Event{Kind: events.MoveZone, Obj: pid, From: e.G.Obj(pid).Zone, To: state.ZHand})
	for i := 0; i < 5; i++ {
		id := pullByName(t, e, 0, "Bear Probe")
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: e.G.Obj(id).Zone, To: state.ZGraveyard})
	}
	for _, r := range "B" {
		e.emit(events.Event{Kind: events.ManaAdd, Player: state.PlayerID(0), Counter: string(r), Amount: 1})
	}
	e.priorityRound()
	d := e.Pending()
	if d == nil {
		t.Fatal("no pending priority decision")
	}
	var opt *decision.Option
	for i := range d.Options {
		o := &d.Options[i]
		if o.Kind == "cast" {
			if ob := e.G.Obj(state.ObjID(o.Obj)); ob != nil && ob.Face() != nil && ob.Face().Name == "Plain Delve Probe" {
				opt = o
			}
		}
	}
	if opt == nil {
		t.Fatalf("precondition: the walk does not offer the plain delve cast: %v", optKinds(e.legalActions(0)))
	}
	gid := state.ObjID(opt.Obj)
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Acting(), Choices: []int{int(opt.Index)}}); err != nil {
		t.Fatalf("submit cast: %v", err)
	}
	ask := e.Pending()
	if ask == nil {
		t.Fatal("the cast flow asked nothing after the submit")
	}
	if !strings.HasPrefix(ask.Prompt, "Delve: exile cards") {
		t.Fatalf("first ask after the submit is %q, want the delve exile ask (a cost with no twobrid resolves its generic immediately)", ask.Prompt)
	}
	served := driveTwobridCast(t, e, gid, newTestBot(9))
	// A plain cost has no hybrid/twobrid/Phyrexian pip: no announcement ask
	// runs at all -- the flow went delve ask straight to payment.
	if n := len(pipKinds(served)); n != 0 {
		t.Fatalf("got %d pip asks, want 0 (a plain cost has nothing to announce): %v", n, served)
	}
	if ob := e.G.Obj(gid); ob == nil || ob.Zone != state.ZBattlefield {
		t.Fatalf("the cast did not enter: zone=%v", func() any {
			if ob == nil {
				return "gone"
			}
			return ob.Zone
		}())
	}
}
