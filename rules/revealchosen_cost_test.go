package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// revealchosen_cost_test.go pins the RevealChosen<...> cost heads (Stalking
// Leonin, Guardian Archon, Emissary of Grudges, A Killer Among Us) and the
// source-relative attackingYou predicate Stalking Leonin's target spec needs.
// Before this grammar existed each RevealChosen token hit ParseCost's
// unrecognised-symbol fallback: one generic mana was substituted and the head
// was reported as Unknown, so the ability cost {1} too much and the secret
// designation was never revealed.

// TestRevealChosenCostHeadsParse is the grammar-level leaf: both spellings
// parse to a RevealChosen component with the right Spec, and neither
// contributes generic mana nor an Unknown census entry.
func TestRevealChosenCostHeadsParse(t *testing.T) {
	for _, tc := range []struct {
		src  string
		spec string
	}{
		{"RevealChosen<Player>", "Player"},
		{"RevealChosen<Type/creature type>", "Type"},
		{"Sac<1/CARDNAME> RevealChosen<Type/creature type>", "Type"},
	} {
		c := ParseCost(tc.src)
		if c.Generic != 0 {
			t.Fatalf("ParseCost(%q).Generic = %d, want 0", tc.src, c.Generic)
		}
		if len(c.Unknown) != 0 {
			t.Fatalf("ParseCost(%q).Unknown = %v, want none", tc.src, c.Unknown)
		}
		if len(c.RevealChosen) != 1 || c.RevealChosen[0].Spec != tc.spec {
			t.Fatalf("ParseCost(%q).RevealChosen = %+v, want one %q part", tc.src, c.RevealChosen, tc.spec)
		}
	}
	// The near-miss heads stay on the reported fallback: they are separate
	// machinery (an either-or election, and an exile-reveal referent), so a
	// future parser must not quietly treat them as a designation reveal.
	for _, src := range []string{"RevealOrChoose<1/Dragon>", "RevealFromExile<1/Creature.YouOwn>"} {
		if c := ParseCost(src); len(c.RevealChosen) != 0 || c.Generic != 1 {
			t.Fatalf("ParseCost(%q) = generic %d, revealChosen %v; want the reported one-generic fallback", src, c.Generic, c.RevealChosen)
		}
	}
}

// battlefieldCorpus puts a corpus card straight onto p's battlefield with a
// logged move (battlefieldFixture's corpus sibling).
func battlefieldCorpus(t *testing.T, e *Engine, reg *cards.Registry, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	o := e.G.AddObject(mustCorpusCard(t, reg, name), p)
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	return o.ID
}

// notesWithPrefix returns every non-secret Note whose Text starts with prefix.
func notesWithPrefix(e *Engine, prefix string) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && !ev.Secret && strings.HasPrefix(ev.Text, prefix) {
			out = append(out, ev)
		}
	}
	return out
}

// activateAbility poses seat p's priority decision, submits the "ability"
// option for id, and returns the option list it was chosen from. It fails if
// the ability is not offered at all -- the offer gate is half of what this
// feature fixes.
func activateRevealAbility(t *testing.T, e *Engine, p state.PlayerID, id state.ObjID) {
	t.Helper()
	e.askPriority(p)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision pending: %+v", d)
	}
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == id {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("no ability option for object %d: %+v", id, d.Options)
}

// passUntilQuiet passes every priority decision until no decision is pending
// or a non-priority ask appears.
func passUntilQuiet(t *testing.T, e *Engine, limit int) {
	t.Helper()
	for i := 0; i < limit; i++ {
		d := e.Pending()
		if d == nil {
			return
		}
		if d.Kind != decision.KPriority {
			return
		}
		castFirst(t, e, "pass")
	}
}

// submitTargetFor submits the target option whose Obj is want.
func submitTargetFor(t *testing.T, e *Engine, want state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want a target ask", d)
	}
	for _, o := range d.Options {
		if o.Obj == want {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("target %d not offered: %+v", want, d.Options)
}

// drainChoices answers every priority pass and every mid-resolution choice
// ask until the stack is quiet, so the resolved ability's outcome is
// observable. A choose ask reached here is the ETB ChoosePlayer's own
// secretly-choose election -- its single opponent option is the pick that
// records the designation the RevealChosen cost later reveals.
func drainChoices(t *testing.T, e *Engine, limit int) {
	t.Helper()
	for i := 0; i < limit; i++ {
		d := e.Pending()
		if d == nil {
			e.askPriority(0)
			d = e.Pending()
			if d == nil {
				return
			}
		}
		switch d.Kind {
		case decision.KPriority:
			castFirst(t, e, "pass")
		case decision.KChoose:
			submitChoices(t, e, d.Options[0].Index)
		default:
			return
		}
	}
}

// TestStalkingLeoninRevealChosenPlayerExilesAttacker is the end-to-end leaf
// the brief names. Stalking Leonin enters, its ETB trigger secretly chooses
// seat 1 through the REAL ChoosePlayer flow; a creature controlled by seat 1
// is attacking seat 0. The ability must be OFFERED (it needs the revealing
// cost AND the source-relative attackingYou target spec), the reveal must be
// a public Note naming the chosen player, and the target must be exiled
// (the ConditionPresent$ Card.ChosenCtrl gate now matches too).
func TestStalkingLeoninRevealChosenPlayerExilesAttacker(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := handEngine(t)
	leonin := battlefieldCorpus(t, e, reg, 0, "Stalking Leonin")

	// The ETB flow: pass priority so the ChangesZone trigger fires, then
	// answer its secretly-choose ask. Seat 0's only opponent is seat 1.
	drainChoices(t, e, 20)
	if got := e.G.Obj(leonin).Chosen; len(got) != 1 || !got[0].IsPlayer || got[0].Player != 1 {
		t.Fatalf("leonin Chosen = %+v, want seat 1 through the real ChoosePlayer flow", got)
	}

	// Seat 1's creature attacks seat 0.
	atko := e.G.AddObject(card(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 1)
	atk := atko.ID
	e.emit(events.Event{Kind: events.MoveZone, Obj: atk, From: state.ZLibrary, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 0, IDs: []state.ObjID{atk}})

	activateRevealAbility(t, e, 0, leonin)
	submitTargetFor(t, e, atk)
	drainChoices(t, e, 40)

	notes := notesWithPrefix(e, "revealed the chosen player:")
	if len(notes) != 1 {
		t.Fatalf("chosen-player reveal Notes = %+v, want exactly one", notes)
	}
	if got := notes[0].Text; got != "revealed the chosen player: b" {
		t.Fatalf("reveal Note text = %q, want the chain-safe seat name", got)
	}
	if o := e.G.Obj(atk); o == nil || o.Zone != state.ZExile {
		t.Fatalf("targeted attacker zone = %+v, want exile", o)
	}
}

// TestStalkingLeoninNotOfferedWithoutAChosenPlayer is the fail-closed half: a
// source that never made the secret choice cannot pay a RevealChosen cost, so
// the ability is not offered (rather than being offered and revealing
// nothing).
func TestStalkingLeoninNotOfferedWithoutAChosenPlayer(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := handEngine(t)
	leonin := battlefieldCorpus(t, e, reg, 0, "Stalking Leonin")
	atk := e.G.Obj(battlefieldCreature(t, e, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"))
	atk.Controller = 1
	e.G.SetZone(state.ZBattlefield, 1, append(e.G.Zone(state.ZBattlefield, 1), atk.ID))
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 0, IDs: []state.ObjID{atk.ID}})

	e.askPriority(0)
	d := e.Pending()
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == leonin {
			t.Fatalf("ability offered with no chosen player: %+v", o)
		}
	}
}

// TestRevealChosenTypeAbilityRevealsAndResolves drives the Type spelling on a
// fixture carrying the exact head: the chosen creature type is a non-empty
// Object.ChosenType, the ability is offered at zero mana, and the reveal Note
// names the type.
func TestRevealChosenTypeAbilityRevealsAndResolves(t *testing.T) {
	e := handEngine(t)
	c := e.G.AddObject(card(t, "Name:Type Revealer\nTypes:Creature Human\nPT:2/2\n"+
		"A:AB$ GainLife | Cost$ RevealChosen<Type/creature type> | LifeAmount$ 3 | SpellDescription$ x\nOracle:x\n"), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: c.ID, From: state.ZLibrary, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.Choose, Obj: c.ID, Counter: "type", Text: "Goblin"})

	activateRevealAbility(t, e, 0, c.ID)
	passUntilQuiet(t, e, 40)

	notes := notesWithPrefix(e, "revealed the chosen creature type:")
	if len(notes) != 1 {
		t.Fatalf("chosen-type reveal Notes = %+v, want exactly one", notes)
	}
	if notes[0].Text != "revealed the chosen creature type: Goblin" {
		t.Fatalf("reveal Note text = %q", notes[0].Text)
	}
	if e.G.Players[0].Life != 23 {
		t.Fatalf("life = %d, want 23 (20 + the 3 life the ability grants)", e.G.Players[0].Life)
	}
}

// TestRevealChosenTypeAbilityNotOfferedWithoutAChosenType is the Type half of
// the fail-closed guard.
func TestRevealChosenTypeAbilityNotOfferedWithoutAChosenType(t *testing.T) {
	e := handEngine(t)
	c := e.G.AddObject(card(t, "Name:Type Revealer\nTypes:Creature Human\nPT:2/2\n"+
		"A:AB$ GainLife | Cost$ RevealChosen<Type/creature type> | LifeAmount$ 3 | SpellDescription$ x\nOracle:x\n"), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: c.ID, From: state.ZLibrary, To: state.ZBattlefield})
	e.askPriority(0)
	d := e.Pending()
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == c.ID {
			t.Fatalf("ability offered with no chosen type: %+v", o)
		}
	}
}

// TestAttackingYouScopesToTheSourcesController pins the predicate's direction:
// a creature attacking the source's controller matches, one attacking anybody
// else does not, and the Watchdog continuous-static spelling
// (`Affected$ Creature.attackingYou`) reads the same predicate.
func TestAttackingYouScopesToTheSourcesController(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := New(Config{Seed: 11, Names: []string{"a", "b", "c"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40)}})
	watchdog := battlefieldCorpus(t, e, reg, 0, "Watchdog")
	if o := e.G.Obj(watchdog); o != nil && o.Tapped {
		e.emit(events.Event{Kind: events.Untap, Obj: watchdog})
	}

	atMe := battlefieldCreature(t, e, "Name:At Me\nManaCost:0\nTypes:Creature Bear\nPT:4/4\nOracle:x\n")
	atOther := battlefieldCreature(t, e, "Name:At Other\nManaCost:0\nTypes:Creature Bear\nPT:4/4\nOracle:x\n")
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 0, IDs: []state.ObjID{atMe}})
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 2, IDs: []state.ObjID{atOther}})

	// Watchdog's static is Affected$ Creature.attackingYou | AddPower$ -1:
	// the attacker at seat 0 (Watchdog's controller) shrinks, the one at seat
	// 2 does not.
	if p := e.Power(atMe); p != 3 {
		t.Fatalf("creature attacking Watchdog's controller has power %d, want 3 (4 - 1)", p)
	}
	if p := e.Power(atOther); p != 4 {
		t.Fatalf("creature attacking another player has power %d, want 4 (unaffected)", p)
	}
}
