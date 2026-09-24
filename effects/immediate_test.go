package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// RememberOriginalTokens$ and the ImmediateTrigger "when you do" primitive
// (effects/immediate.go). The token-remember half mirrors token_test.go's
// fixture style; the immediate-trigger half drives Resolve against inline
// SVars (the same shape effRepeatEach's tests use) and observes instances
// through the sub's own effects -- a DealDamage over Defined$ Remembered for
// the per-instance set, a Token mint for the instance count. The real
// suspension discipline is pinned end to end in rules/
// forum_filibuster_test.go (rules' Engine owns the resume machinery; this
// package's fake host never suspends).

// TestRememberOriginalTokensMirrorsRememberTokens pins the Gap-1 read: the
// flag takes the SAME branch as RememberTokens$ -- the ctx set AND the
// event-backed Choose events (eventRemember), because a suspension between
// the mint and the "when you do" ask must survive on both halves.
func TestRememberOriginalTokensMirrorsRememberTokens(t *testing.T) {
	h, c := fixtureHostWithTokens(t) // c.Source = object 1 (a real card)
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Token",
		Params: map[string]string{"TokenScript": "r_1_1_goblin", "RememberOriginalTokens": "True"}})
	if n := countKind(h, events.TokenCreate); n != 1 {
		t.Fatalf("%d TokenCreate events, want 1", n)
	}
	if len(c.Remembered) != 1 {
		t.Fatalf("ctx Remembered = %v, want the one minted token", c.Remembered)
	}
	token := c.Remembered[0].Obj
	if h.Game().Obj(token) == nil || !h.Game().Obj(token).IsToken {
		t.Fatalf("remembered %d is not the minted token", token)
	}
	chooses := 0
	for _, ev := range h.log {
		if ev.Kind == events.Choose && ev.Counter == "remembered" && len(ev.IDs) == 1 && ev.IDs[0] == token {
			chooses++
		}
	}
	if chooses != 1 {
		t.Fatalf("%d remembered Choose events, want 1 (log %+v)", chooses, h.log)
	}
}

// TestImmediateTriggerMissingExecuteNotes: no Execute$ (or an unresolvable
// SVar name) is ONE loud Note and a no-op, never a panic and never silence.
func TestImmediateTriggerMissingExecuteNotes(t *testing.T) {
	h, c := fixtureHostWithTokens(t)
	c.SVars = map[string]string{"Exec": "DB$ Token | TokenScript$ r_1_1_goblin"}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "ImmediateTrigger", Params: map[string]string{}})
	if n := countKind(h, events.TokenCreate); n != 0 {
		t.Fatalf("absent Execute$ ran %d times", n)
	}
	if len(h.log) != 1 || h.log[0].Kind != events.Note || h.log[0].Text != "ImmediateTrigger with no resolvable Execute$ " {
		t.Fatalf("absent Execute$ log = %+v", h.log)
	}
	h.log = nil
	Resolve(h, c, &cards.SA{Kind: "DB", API: "ImmediateTrigger", Params: map[string]string{"Execute": "NoSuch"}})
	if n := countKind(h, events.TokenCreate); n != 0 {
		t.Fatalf("unresolvable Execute$ ran %d times", n)
	}
	if len(h.log) != 1 || h.log[0].Text != "ImmediateTrigger with no resolvable Execute$ NoSuch" {
		t.Fatalf("unresolvable Execute$ log = %+v", h.log)
	}
}

// toBattlefield moves a fixture object onto the battlefield through a real
// MoveZone event (events.Apply owns every zone write), because a DealDamage
// only lands on a battlefield object and AddObject starts one in the library.
func toBattlefield(h *fakeHost, id state.ObjID) {
	h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
}

// countDamageTo counts the Damage events naming want.
func countDamageTo(h *fakeHost, want state.ObjID) int {
	n := 0
	for _, ev := range h.log {
		if ev.Kind == events.Damage && ev.Obj == want {
			n++
		}
	}
	return n
}

// immHost is a fixture host whose Ctx carries an SVar table with one
// DealDamage-over-Remembered sub: the observable per-instance body.
func immHost(t *testing.T) (*fakeHost, *Ctx) {
	t.Helper()
	h, c := fixtureHost(t)
	c.SVars = map[string]string{"Hit": "DB$ DealDamage | Defined$ Remembered | NumDmg$ 1"}
	return h, c
}

// TestImmediateTriggerDefaultAmountExecutesOnce: no TriggerAmount$ is one
// instance, and with no RememberObjects$ every instance sees the whole
// parent Remembered set (Speed, Young Avenger's absent-RememberObjects
// shape).
func TestImmediateTriggerDefaultAmountExecutesOnce(t *testing.T) {
	h, c := immHost(t)
	toBattlefield(h, c.Source)
	src2 := h.Game().Obj(2).ID
	toBattlefield(h, src2)
	c.Remembered = []state.Target{{Obj: c.Source}, {Obj: src2}}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "ImmediateTrigger", Params: map[string]string{"Execute": "Hit"}})
	if n := countDamageTo(h, c.Source); n != 1 || countDamageTo(h, src2) != 1 {
		t.Fatalf("one whole-set instance expected (source %d, obj2 %d)", countDamageTo(h, c.Source), countDamageTo(h, src2))
	}
}

// TestImmediateTriggerAmountRunsNInstances: TriggerAmount$ 3 through the Num
// grammar (a literal here; the SVar/Count$ forms ride the same resolver).
func TestImmediateTriggerAmountRunsNInstances(t *testing.T) {
	h, c := immHost(t)
	toBattlefield(h, c.Source)
	c.Remembered = []state.Target{{Obj: c.Source}}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "ImmediateTrigger",
		Params: map[string]string{"Execute": "Hit", "TriggerAmount": "3"}})
	if n := countDamageTo(h, c.Source); n != 3 {
		t.Fatalf("%d instances, want 3", n)
	}
}

// TestImmediateTriggerRememberEachOneInstancePerObject: RememberObjects$
// Remembered + RememberEach$ True is one instance per parent remembered
// object, each instance's Ctx.Remembered exactly that object -- and the
// count clamps to the remembered count when TriggerAmount$ exceeds it
// (never index-past-end).
func TestImmediateTriggerRememberEachOneInstancePerObject(t *testing.T) {
	h, c := immHost(t)
	toBattlefield(h, c.Source)
	src2 := h.Game().Obj(2).ID
	toBattlefield(h, src2)
	c.Remembered = []state.Target{{Obj: c.Source}, {Obj: src2}}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "ImmediateTrigger",
		Params: map[string]string{"Execute": "Hit", "RememberObjects": "Remembered",
			"RememberEach": "True", "TriggerAmount": "Remembered$Amount"}})
	if n := countDamageTo(h, c.Source); n != 1 || countDamageTo(h, src2) != 1 {
		t.Fatalf("one instance per remembered object expected (source %d, obj2 %d)", countDamageTo(h, c.Source), countDamageTo(h, src2))
	}
	h.log = nil
	Resolve(h, c, &cards.SA{Kind: "DB", API: "ImmediateTrigger",
		Params: map[string]string{"Execute": "Hit", "RememberObjects": "Remembered",
			"RememberEach": "True", "TriggerAmount": "99"}})
	if n := countDamageTo(h, c.Source); n != 1 || countDamageTo(h, src2) != 1 {
		t.Fatalf("clamp to the remembered count expected (source %d, obj2 %d)", countDamageTo(h, c.Source), countDamageTo(h, src2))
	}
}

// TestImmediateTriggerCaptureExcludedFromAmount: a trigger's captured event
// object is NOT part of the remembered list the amount reads (the
// iterationBase convention). The fixture mints two real tokens first (the
// amount read counts LIVE remembered objects -- evalRefProperty's
// Remembered$Amount walk), then seeds the ctx the way rules seeds a trigger
// ctx for a capture that is NOT the source (Remembered = Captured = [the
// event object] plus the tokens): both Remembered$Amount and
// Count$RememberedNumber must count TWO, not three.
func TestImmediateTriggerCaptureExcludedFromAmount(t *testing.T) {
	h, c := fixtureHostWithTokens(t)
	c.SVars = map[string]string{"Mint": "DB$ Token | TokenScript$ r_1_1_goblin"}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Token", Params: map[string]string{"TokenScript": "r_1_1_goblin", "TokenAmount": "2"}})
	if n := countKind(h, events.TokenCreate); n != 2 {
		t.Fatalf("fixture minted %d tokens", n)
	}
	bf := h.Game().Zone(state.ZBattlefield, c.Controller)
	capture := state.Target{Obj: h.Game().Obj(2).ID}
	h.log = nil
	c.Remembered = []state.Target{capture, {Obj: bf[0]}, {Obj: bf[1]}}
	c.Captured = []state.Target{capture}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "ImmediateTrigger",
		Params: map[string]string{"Execute": "Mint", "TriggerAmount": "Remembered$Amount"}})
	if n := countKind(h, events.TokenCreate); n != 2 {
		t.Fatalf("%d instances, want 2 (capture excluded)", n)
	}
	h.log = nil
	Resolve(h, c, &cards.SA{Kind: "DB", API: "ImmediateTrigger",
		Params: map[string]string{"Execute": "Mint", "TriggerAmount": "Count$RememberedNumber"}})
	if n := countKind(h, events.TokenCreate); n != 2 {
		t.Fatalf("%d instances, want 2 (Count$RememberedNumber)", n)
	}
}

// TestImmediateTriggerDivideEvenlyDownHalves: TriggerAmount$
// Remembered$Amount/DivideEvenlyDown.2 is floor(amount/2) instances -- the
// diregraf_horde / faebloom_trick shape (two tokens, one instance).
func TestImmediateTriggerDivideEvenlyDownHalves(t *testing.T) {
	h, c := fixtureHostWithTokens(t)
	c.SVars = map[string]string{"Mint": "DB$ Token | TokenScript$ r_1_1_goblin"}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Token", Params: map[string]string{"TokenScript": "r_1_1_goblin", "TokenAmount": "2"}})
	if n := countKind(h, events.TokenCreate); n != 2 {
		t.Fatalf("fixture minted %d tokens", n)
	}
	bf := h.Game().Zone(state.ZBattlefield, c.Controller)
	h.log = nil
	c.Remembered = []state.Target{{Obj: bf[0]}, {Obj: bf[1]}}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "ImmediateTrigger",
		Params: map[string]string{"Execute": "Mint", "TriggerAmount": "Remembered$Amount/DivideEvenlyDown.2"}})
	if n := countKind(h, events.TokenCreate); n != 1 {
		t.Fatalf("%d instances, want 1 (2/2)", n)
	}
}

// TestImmediateTriggerUnknownRememberObjectsNotesAndStillRuns: a
// RememberObjects$ value outside the Remembered family is ONE loud Note
// naming the value, then the whole parent set -- never silence.
func TestImmediateTriggerUnknownRememberObjectsNotesAndStillRuns(t *testing.T) {
	h, c := immHost(t)
	toBattlefield(h, c.Source)
	c.Remembered = []state.Target{{Obj: c.Source}}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "ImmediateTrigger",
		Params: map[string]string{"Execute": "Hit", "RememberObjects": "Sacrificed"}})
	notes := 0
	for _, ev := range h.log {
		if ev.Kind == events.Note && ev.Text == "ImmediateTrigger RememberObjects$ Sacrificed is not implemented; every instance sees the whole remembered set" {
			notes++
		}
	}
	if notes != 1 {
		t.Fatalf("%d naming Notes, want 1 (log %+v)", notes, h.log)
	}
	if n := countDamageTo(h, c.Source); n != 1 {
		t.Fatalf("%d instances after the Note, want 1", n)
	}
}

// effects/immediate.go -- it takes the whole-parent-set arm (no Note) and
// the Execute$ body still runs against the remembered object.
//
// The corpus shape (mount_velus_manticore.txt):
//
//	SVar:DBImmediateTrigger:DB$ ImmediateTrigger | ConditionDefined$ Remembered |
//	  ConditionPresent$ Card | ConditionCompare$ GE1 | Execute$ TrigDamage |
//	  RememberObjects$ RememberedCard | SubAbility$ DBCleanup | TriggerDescription$ ...
//
// and nine further carriers (cait_sith_fortune_teller, fiery_encore,
// god_eternal_kefnet, hidetsugu_devouring_chaos, lavabrink_floodgates,
// pyroclastic_hellion, superior_spider_man,
// torrent_sculptor_flamethrower_sonata, wall_of_stolen_identity) all spell the
// value the same way. This test drives the recogniser directly (not the full
// card: mount_velus_manticore additionally needs the unread
// TriggerRemembered$CardTypes property, a separate gap).
func TestImmediateTriggerRememberedCardDoesNotNote(t *testing.T) {
	h, c := immHost(t)
	toBattlefield(h, c.Source)
	c.Remembered = []state.Target{{Obj: c.Source}}

	// Precondition: the remembered object really is on the battlefield in the
	// zone DealDamage lands on, and nothing has damaged it yet -- so the
	// assertions below cannot pass vacuously.
	if obj := h.Game().Obj(c.Source); obj == nil || obj.Zone != state.ZBattlefield {
		t.Fatalf("precondition: source %d not on the battlefield (obj %+v)", c.Source, obj)
	}
	if n := countDamageTo(h, c.Source); n != 0 {
		t.Fatalf("precondition: %d Damage events before the resolve", n)
	}

	Resolve(h, c, &cards.SA{Kind: "DB", API: "ImmediateTrigger",
		Params: map[string]string{"Execute": "Hit", "RememberObjects": "RememberedCard"}})

	// The ticket's whole claim: the value must NOT reach the loud default arm.
	for _, ev := range h.log {
		if ev.Kind == events.Note {
			t.Fatalf("RememberedCard emitted Note %q; the value is recognised", ev.Text)
		}
	}
	// The Execute body ran against the remembered object -- so the arm was
	// taken as the whole-parent-set read, not silently ignored.
	if n := countDamageTo(h, c.Source); n != 1 {
		t.Fatalf("%d Damage events on the remembered object, want 1", n)
	}
}

// TestImmediateTriggerRememberedCardSeesWholeParentSet pins the documented
// reading for the recognised "RememberedCard" arm: every instance sees the
// whole parent remembered set (the switch's second case), matching the
// "Remembered" arm -- unlike RememberEach$.
func TestImmediateTriggerRememberedCardSeesWholeParentSet(t *testing.T) {
	h, c := immHost(t)
	toBattlefield(h, c.Source)
	src2 := h.Game().Obj(2).ID
	toBattlefield(h, src2)
	c.Remembered = []state.Target{{Obj: c.Source}, {Obj: src2}}

	if obj := h.Game().Obj(src2); obj == nil || obj.Zone != state.ZBattlefield {
		t.Fatalf("precondition: second object %d not on the battlefield (obj %+v)", src2, obj)
	}
	if countDamageTo(h, c.Source) != 0 || countDamageTo(h, src2) != 0 {
		t.Fatalf("precondition: damage before the resolve (source %d, obj2 %d)",
			countDamageTo(h, c.Source), countDamageTo(h, src2))
	}

	Resolve(h, c, &cards.SA{Kind: "DB", API: "ImmediateTrigger",
		Params: map[string]string{"Execute": "Hit", "RememberObjects": "RememberedCard"}})

	for _, ev := range h.log {
		if ev.Kind == events.Note {
			t.Fatalf("RememberedCard emitted Note %q; the value is recognised", ev.Text)
		}
	}
	if n := countDamageTo(h, c.Source); n != 1 || countDamageTo(h, src2) != 1 {
		t.Fatalf("whole-parent-set instance expected (source %d, obj2 %d), want 1 and 1",
			countDamageTo(h, c.Source), countDamageTo(h, src2))
	}
}
