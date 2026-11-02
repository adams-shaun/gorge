package rules

// lb-canattackpresent: a face CanAttackDefender static gated by IsPresent$
// over a non-battlefield zone (Platypus-Bear's `IsPresent$ Lesson.YouOwn |
// PresentZone$ Graveyard`). The registration whitelist
// (effects.CanAttackDefenderParamsReadable) did not list PresentZone$, so the
// static was skipped whole and the bear stayed walled with a Lesson in the
// graveyard.

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const lessonFixture = "Name:Test Lesson\nManaCost:1 U\nTypes:Sorcery Lesson\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"

// TestPlatypusBearAttacksWithLessonInGraveyard: the real corpus Platypus-Bear
// is walled with no Lesson in its owner's graveyard and is offered as an
// attacker (and the declaration commits) once one is there.
func TestPlatypusBearAttacksWithLessonInGraveyard(t *testing.T) {
	t.Parallel()
	bear, ok := testutil.CorpusRegistry(t).Lookup("Platypus-Bear")
	if !ok {
		t.Fatal("corpus fixture: Platypus-Bear missing")
	}
	lesson := card(t, lessonFixture)
	e, cfg := restrictionGame(t, 7311, [][]*cards.Card{{lesson}, nil},
		[][]*cards.Card{{bear}, nil})
	bearID := bearOnBoard(t, e, 0, bear)
	if e.G.Obj(bearID).Zone != state.ZBattlefield || !e.IsCreature(bearID) {
		t.Fatal("precondition: Platypus-Bear is not a battlefield creature")
	}
	if !e.HasKeyword(bearID, "Defender") {
		t.Fatal("precondition: Platypus-Bear carries no Defender")
	}
	var lessonID state.ObjID
	for _, id := range append(append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...), e.G.Zone(state.ZLibrary, 0)...) {
		if o := e.G.Obj(id); o != nil && o.Card == lesson {
			lessonID = id
		}
	}
	if lessonID == 0 {
		t.Fatal("precondition: the Lesson is not in seat 0's hand or library")
	}
	for _, id := range e.G.Zone(state.ZGraveyard, 0) {
		if o := e.G.Obj(id); o != nil && o.Card == lesson {
			t.Fatal("precondition: a Lesson is already in the graveyard")
		}
	}

	// Gate closed: no Lesson in the graveyard, the wall stands.
	if e.canAttackPair(bearID, 1) {
		t.Fatal("Platypus-Bear may attack with no Lesson in its owner's graveyard")
	}

	e.emit(events.Event{Kind: events.MoveZone, Obj: lessonID, From: e.G.Obj(lessonID).Zone, To: state.ZGraveyard})
	if e.G.Obj(lessonID).Zone != state.ZGraveyard {
		t.Fatal("precondition: the Lesson did not reach the graveyard")
	}

	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	e.askAttackers()
	if d := e.Pending(); d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("no attackers decision with a Lesson in the graveyard: %+v", d)
	}
	if _, ok := attackerOption(t, e, bearID, 1); !ok {
		t.Fatalf("Platypus-Bear is not offered as an attacker with a Lesson in the graveyard: %+v", e.Pending().Options)
	}
	submitAttackerAt(t, e, bearID, 1)
	if o := e.G.Obj(bearID); !o.IsAttacking || o.Attacking != 1 {
		t.Fatalf("declaration did not commit: IsAttacking=%v Attacking=%d", o.IsAttacking, o.Attacking)
	}
	replayCheck(t, e, cfg)
}

// TestCanAttackDefenderPresentCompareNeedsAReadSpec: PresentCompare$ can
// invert the present gate, so it is admitted only over a non-blank spec the
// matcher reads; otherwise an EQ0 compare over a zero count would lift the
// wall unconditionally.
func TestCanAttackDefenderPresentCompareNeedsAReadSpec(t *testing.T) {
	t.Parallel()
	base := func(extra map[string]string) map[string]string {
		p := map[string]string{"Mode": "CanAttackDefender", "ValidCard": "Card.Self"}
		for k, v := range extra {
			p[k] = v
		}
		return p
	}
	cases := []struct {
		name   string
		params map[string]string
		want   bool
	}{
		{"zone", base(map[string]string{"IsPresent": "Lesson.YouOwn", "PresentZone": "Graveyard"}), true},
		{"compare", base(map[string]string{"IsPresent": "Lesson.YouOwn", "PresentCompare": "GE2"}), true},
		{"orphanCompare", base(map[string]string{"PresentCompare": "EQ0"}), false},
		{"blankSpecCompare", base(map[string]string{"IsPresent": " ", "PresentCompare": "EQ0"}), false},
		{"unknownPredicateCompare", base(map[string]string{"IsPresent": "Creature.noSuchPredicateXyz", "PresentCompare": "EQ0"}), false},
		{"unknownKey", base(map[string]string{"ForCost": "True"}), false},
	}
	for _, c := range cases {
		if got := effects.CanAttackDefenderParamsReadable(c.params); got != c.want {
			t.Errorf("%s: CanAttackDefenderParamsReadable(%v) = %v, want %v", c.name, c.params, got, c.want)
		}
	}
}

// knownUnreadableCanAttackDefender is the ratchet: corpus face
// CanAttackDefender statics the whitelist still refuses. It only shrinks.
var knownUnreadableCanAttackDefender = []string{}

// TestCorpusCanAttackDefenderStaticsReadable censuses every corpus face
// CanAttackDefender static through the registration whitelist the
// attacker-legality read consults.
func TestCorpusCanAttackDefenderStaticsReadable(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	carriers := 0
	var unreadable []string
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			if f == nil {
				continue
			}
			for _, s := range f.Statics {
				if s.Mode != "CanAttackDefender" {
					continue
				}
				carriers++
				if !effects.CanAttackDefenderParamsReadable(s.Params) {
					unreadable = append(unreadable, f.Name)
				}
			}
		}
	}
	// Measured 2026-10-12: 32 face carriers (the raw grep's 56 lines also
	// count Effect-delivered SVar bodies, which are not face statics).
	if carriers < 30 {
		t.Fatalf("precondition: only %d CanAttackDefender carriers in the corpus", carriers)
	}
	sort.Strings(unreadable)
	known := map[string]bool{}
	for _, n := range knownUnreadableCanAttackDefender {
		known[n] = true
	}
	for _, n := range unreadable {
		if !known[n] {
			t.Errorf("%s: CanAttackDefender static refused by effects.CanAttackDefenderParamsReadable (not in the ratchet)", n)
		}
	}
	if len(unreadable) < len(knownUnreadableCanAttackDefender) {
		t.Errorf("ratchet stale: %d unreadable now, %d listed -- shrink knownUnreadableCanAttackDefender to %v",
			len(unreadable), len(knownUnreadableCanAttackDefender), unreadable)
	}
}
