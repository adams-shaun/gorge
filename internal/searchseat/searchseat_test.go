package searchseat

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

// Defaults must BE cmd/searchteacher's flag defaults, because those are the
// knobs the +5.80pp +/- 1.25 paired-dev result was measured with. If someone
// retunes a default here, "the seat plays the teacher" silently stops meaning
// the teacher that was measured -- so the values are pinned literally rather
// than asserted loosely.
func TestDefaultsAreTheMeasuredKnobs(t *testing.T) {
	d := Defaults()
	if d.Worlds != 8 || d.Attempts != 64 || d.Limit != 6 || d.MaxSubmits != 5000 {
		t.Errorf("worlds/attempts/limit/max-submits = %d/%d/%d/%d, want 8/64/6/5000",
			d.Worlds, d.Attempts, d.Limit, d.MaxSubmits)
	}
	if d.MinESS != 0 || d.Margin != 0 || d.HorizonTurns != 0 {
		t.Errorf("min-ess/margin/horizon = %v/%v/%v, want 0/0/0 (0 horizon = roll to game end)",
			d.MinESS, d.Margin, d.HorizonTurns)
	}
	if d.SampleSeed != 54321 {
		t.Errorf("SampleSeed = %d, want 54321", d.SampleSeed)
	}
	if d.Clairvoyant {
		t.Error("Clairvoyant must default false: it cheats by construction and is a measurement ceiling only")
	}
	if !d.Kinds["attackers"] || !d.Kinds["cast"] {
		t.Errorf("Kinds = %v, want both implemented kinds on", d.Kinds)
	}
}

// CastOptions counts DISTINCT objects, not options. One card offered several
// ways (an alternative cost, a kicked mode) is one choice of card, so a
// priority decision offering the same object twice is not a two-candidate
// decision and must not open a search.
func TestCastOptionsCountsDistinctObjects(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []decision.Option
		want int
	}{
		{"none", []decision.Option{{Kind: "pass"}}, 0},
		{"one card", []decision.Option{{Kind: "cast", Obj: 10}, {Kind: "pass"}}, 1},
		{"same card twice", []decision.Option{{Kind: "cast", Obj: 10}, {Kind: "cast", Obj: 10}}, 1},
		{"two cards", []decision.Option{{Kind: "cast", Obj: 10}, {Kind: "cast", Obj: 11}}, 2},
		{"non-cast ignored", []decision.Option{{Kind: "cast", Obj: 10}, {Kind: "ability", Obj: 11}}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &decision.Decision{Kind: decision.KPriority, Options: tc.opts}
			if got := CastOptions(d); got != tc.want {
				t.Errorf("CastOptions = %d, want %d", got, tc.want)
			}
		})
	}
}

// Eligible is the cheap pre-test a driver uses to decide whether a decision is
// worth observing at all, so it must agree exactly with the arm Choose would
// take. The kinds gate is part of that: a caller that turned a kind off must
// see it declined here too, or it would pay for sampling Choose then refuses.
func TestEligibleMatchesTheImplementedKinds(t *testing.T) {
	on := Defaults()
	twoCasts := &decision.Decision{Kind: decision.KPriority, Options: []decision.Option{
		{Kind: "cast", Obj: 1}, {Kind: "cast", Obj: 2}, {Kind: "pass"},
	}}
	oneCast := &decision.Decision{Kind: decision.KPriority, Options: []decision.Option{
		{Kind: "cast", Obj: 1}, {Kind: "pass"},
	}}
	attackers := &decision.Decision{Kind: decision.KAttackers}

	if !Eligible(twoCasts, on) {
		t.Error("a priority decision with two distinct castable objects should be eligible")
	}
	if Eligible(oneCast, on) {
		t.Error("a priority decision with ONE castable object is not a choice between casts; want ineligible")
	}
	if !Eligible(attackers, on) {
		t.Error("an attackers decision should be eligible")
	}

	// Every other kind delegates. These are the kinds the teacher has never
	// covered; listing them explicitly is what would catch a future arm added
	// to Choose without Eligible learning about it.
	for _, k := range []decision.Kind{
		decision.KTarget, decision.KBlockers, decision.KChoose, decision.KModes,
		decision.KMulligan, decision.KTriggerOrder, decision.KTriggerOptional,
		decision.KCommanderZone, decision.KReplacement, decision.KArrange,
	} {
		if Eligible(&decision.Decision{Kind: k}, on) {
			t.Errorf("kind %q should delegate, but Eligible returned true", k)
		}
	}
}

// A caller may turn a kind off, and then that kind must delegate even when its
// shape would otherwise qualify.
func TestEligibleHonoursTheKindsGate(t *testing.T) {
	off := Defaults()
	off.Kinds = map[string]bool{"attackers": true} // cast deliberately absent

	twoCasts := &decision.Decision{Kind: decision.KPriority, Options: []decision.Option{
		{Kind: "cast", Obj: 1}, {Kind: "cast", Obj: 2},
	}}
	if Eligible(twoCasts, off) {
		t.Error("cast is off, so a cast decision must delegate")
	}
	if !Eligible(&decision.Decision{Kind: decision.KAttackers}, off) {
		t.Error("attackers is on and must stay eligible")
	}

	none := Defaults()
	none.Kinds = nil
	if Eligible(twoCasts, none) || Eligible(&decision.Decision{Kind: decision.KAttackers}, none) {
		t.Error("a nil Kinds map must delegate everything rather than defaulting to on")
	}
}
