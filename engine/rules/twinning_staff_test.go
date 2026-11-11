package rules

// R:Event$ CopySpell (Twinning Staff, the corpus's only carrier): "If you
// would copy a spell one or more times, instead copy it that many times plus
// an additional time. You may choose new targets for the additional copy."
// effCopySpellAbility's default arm asks the rules host (CopySpellExtra) for
// the extra count and emits each additional copy with the CR 707.10c
// new-targets permission. Twinning Staff, Lightning Bolt and Grapeshot are the
// REAL corpus scripts; the copy spell without MayChooseTarget$ is an inline
// fixture.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const staffEchoFixture = "Name:Staff Echo Fixture\nManaCost:U\nTypes:Instant\n" +
	"A:SP$ CopySpellAbility | ValidTgts$ Instant,Sorcery | TargetType$ Spell | SpellDescription$ Copy target instant or sorcery spell.\n" +
	"Oracle:Echoes a spell (test fixture).\n"

// staffGame seats Twinning Staff on the battlefields named by staffs (seat
// ids, one Staff each), seat 0's other cards in hand, and funds seat 0.
func staffGame(t *testing.T, reg *cards.Registry, staffs []state.PlayerID, hand []*cards.Card, mana string) (*Engine, Config) {
	t.Helper()
	staff := lookup(t, reg, "Twinning Staff")
	var x0, x1 []*cards.Card
	for _, p := range staffs {
		if p == 0 {
			x0 = append(x0, staff)
		} else {
			x1 = append(x1, staff)
		}
	}
	e, cfg := corpusEngineCfg(t, reg, append(x0, hand...), x1)
	for _, p := range staffs {
		moveByName(t, e, p, "Twinning Staff", state.ZBattlefield)
	}
	taken := map[state.ObjID]bool{}
	for _, c := range hand {
		var pick state.ObjID
		var from state.Zone
		for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
			for _, id := range e.G.Zone(z, 0) {
				if o := e.G.Obj(id); pick == 0 && !taken[id] && o.Face() != nil && o.Face().Name == c.Faces[0].Name {
					pick, from = id, z
				}
			}
		}
		if pick == 0 {
			t.Fatalf("precondition: %s not in seat 0's hand or library", c.Faces[0].Name)
		}
		taken[pick] = true
		if from != state.ZHand {
			e.emit(events.Event{Kind: events.MoveZone, Obj: pick, From: from, To: state.ZHand})
		}
	}
	addMana(t, e, 0, mana)
	return e, cfg
}

// stackCopies counts the StackCopy events of spell, and those carrying the
// may-choose-new-targets permission (Amount 1).
func stackCopies(e *Engine, spell state.ObjID) (all, mayChoose int) {
	for _, ev := range e.L.Events {
		if ev.Kind == events.StackCopy && ev.Obj == spell {
			all++
			if ev.Amount == 1 {
				mayChoose++
			}
		}
	}
	return all, mayChoose
}

// echoBolt casts Lightning Bolt at seat 1, then the echo fixture at the Bolt,
// and drains the stack; returns the Bolt's copy counts.
func echoBolt(t *testing.T, reg *cards.Registry, staffs []state.PlayerID) (all, mayChoose int) {
	t.Helper()
	echo := card(t, staffEchoFixture)
	e, cfg := staffGame(t, reg, staffs, []*cards.Card{lookup(t, reg, "Lightning Bolt"), echo}, "RU")
	bolt := findInHand(e, 0, "Lightning Bolt")
	castNamed(t, e, "Lightning Bolt")
	answerTargetPlayer(t, e, 1)
	castNamed(t, e, "Staff Echo Fixture")
	if d := e.Pending(); d == nil || len(d.Options) == 0 || d.Options[0].Obj != bolt {
		t.Fatalf("precondition: the echo's target ask should offer the Bolt first: %+v", d)
	}
	drainAnsweringFirst(t, e, 40)
	replayCheck(t, e, cfg)
	return stackCopies(e, bolt)
}

func TestTwinningStaffCopiesOneExtra(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	t.Run("registry", func(t *testing.T) {
		if got := reg.Unsupported(lookup(t, reg, "Twinning Staff"), effects.Supported()); len(got) != 0 {
			t.Fatalf("Twinning Staff unsupported: %v", got)
		}
	})
	t.Run("the Staff adds one copy, and only it may choose new targets", func(t *testing.T) {
		baseAll, baseMay := echoBolt(t, reg, nil)
		all, may := echoBolt(t, reg, []state.PlayerID{0})
		if baseAll != 1 || baseMay != 0 {
			t.Fatalf("precondition: without the Staff the echo makes %d copies (%d may retarget), want 1 (0)", baseAll, baseMay)
		}
		if all != 2 || may != 1 {
			t.Fatalf("with the Staff: %d copies (%d may retarget), want 2 (1: the additional copy)", all, may)
		}
	})
	t.Run("two Staffs add two", func(t *testing.T) {
		if all, may := echoBolt(t, reg, []state.PlayerID{0, 0}); all != 3 || may != 2 {
			t.Fatalf("two Staffs: %d copies (%d may retarget), want 3 (2)", all, may)
		}
	})
	t.Run("an opponent's Staff adds nothing", func(t *testing.T) {
		if all, _ := echoBolt(t, reg, []state.PlayerID{1}); all != 1 {
			t.Fatalf("seat 1's Staff: %d copies of seat 0's copy, want 1 (ValidPlayer$ You)", all)
		}
	})
	t.Run("the Staff's own ability makes two", func(t *testing.T) {
		e, cfg := staffGame(t, reg, []state.PlayerID{0}, []*cards.Card{lookup(t, reg, "Lightning Bolt")}, "RCCCCCCC")
		bolt := findInHand(e, 0, "Lightning Bolt")
		staff := e.G.Zone(state.ZBattlefield, 0)[0]
		if e.G.Obj(staff).Face().Name != "Twinning Staff" {
			t.Fatalf("precondition: seat 0's permanent is %s", e.G.Obj(staff).Face().Name)
		}
		castNamed(t, e, "Lightning Bolt")
		answerTargetPlayer(t, e, 1)
		idx := oracleAbilityOption(e, staff)
		if idx < 0 {
			t.Fatalf("precondition: the Staff's {7},{T} ability is not offered: %+v", e.Pending().Options)
		}
		submitChoices(t, e, idx)
		drainAnsweringFirst(t, e, 40)
		if all, may := stackCopies(e, bolt); all != 2 || may != 2 {
			t.Fatalf("Staff ability: %d copies (%d may retarget), want 2 (2)", all, may)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("Storm N copies become N+1", func(t *testing.T) {
		for _, tc := range []struct {
			staffs []state.PlayerID
			want   int
		}{{nil, 2}, {[]state.PlayerID{0}, 3}} {
			bolt := lookup(t, reg, "Lightning Bolt")
			e, _ := staffGame(t, reg, tc.staffs, []*cards.Card{bolt, bolt, lookup(t, reg, "Grapeshot")}, "RRRR")
			grapeshot := findInHand(e, 0, "Grapeshot")
			// Grapeshot is a sorcery: each Bolt resolves before the next cast.
			for i := 0; i < 2; i++ {
				castNamed(t, e, "Lightning Bolt")
				answerTargetPlayer(t, e, 1)
				drainAnsweringFirst(t, e, 20)
			}
			castNamed(t, e, "Grapeshot")
			answerTargetPlayer(t, e, 1)
			drainAnsweringFirst(t, e, 60)
			if all, _ := stackCopies(e, grapeshot); all != tc.want {
				t.Fatalf("staffs=%v: Grapeshot storm made %d copies, want %d", tc.staffs, all, tc.want)
			}
		}
	})
}
