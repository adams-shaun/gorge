package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// The pile-B order (Intent.Rest) bot-side tests: the constraint the engine
// validates (Decision.validateRest, reached through Validate) binds on an
// arrange ask, so the bot's answer finaliser must never hand back a partition
// Submit rejects. Every bot answer flows through Clamp, and no bot arm builds
// a Rest today -- these tests pin both halves: the legacy-shaped answer stays
// valid, and a rest-carrying input is kept when it validates and dropped when
// it does not (falling back to the legacy offered-order complement).

// scryDecision is the ask shape effLookAndArrange poses for a Scry 3.
func scryDecision() *decision.Decision {
	opts := make([]decision.Option, 3)
	for i := range opts {
		opts[i] = decision.Option{Index: i, Kind: "bottom", Player: state.PlayerID(0)}
	}
	return &decision.Decision{Seq: 7, Player: 0, Kind: decision.KArrange,
		Min: 0, Max: 3, Restable: true, Options: opts}
}

// TestClampKeepsTheBotAnswerLegalOnAScryAsk runs the bot's own finaliser on
// the ask where the partition rule binds: the legacy-shaped answer (no Rest)
// must come back untouched and pass Validate.
func TestClampKeepsTheBotAnswerLegalOnAScryAsk(t *testing.T) {
	d := scryDecision()
	in := decision.Intent{Seq: d.Seq, Player: d.Player}
	out := Clamp(d, in)
	if err := d.Validate(out); err != nil {
		t.Fatalf("Clamp's bot answer on a scry ask fails Validate: %v (out = %+v)", err, out)
	}
	if len(out.Rest) != 0 {
		t.Fatalf("Clamp invented a Rest %v for a legacy-shaped answer", out.Rest)
	}
}

// TestClampKeepsAValidRestAndDropsAnInvalidOne: a rest that is the exact
// complement survives repair; one that is not (here: overlapping the chosen
// set) is dropped, and the returned answer still validates -- falling back to
// the legacy offered-order complement rather than an intent Submit rejects.
func TestClampKeepsAValidRestAndDropsAnInvalidOne(t *testing.T) {
	d := scryDecision()
	valid := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}, Rest: []int{2, 1}}
	out := Clamp(d, valid)
	if err := d.Validate(out); err != nil {
		t.Fatalf("Clamp dropped a well-formed rest: %v (out = %+v)", err, out)
	}
	if len(out.Rest) != 2 {
		t.Fatalf("Clamp altered a valid rest to %v", out.Rest)
	}

	invalid := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}, Rest: []int{0, 1}}
	out = Clamp(d, invalid)
	if err := d.Validate(out); err != nil {
		t.Fatalf("Clamp's repaired answer still fails Validate: %v (out = %+v)", err, out)
	}
	if len(out.Rest) != 0 {
		t.Fatalf("Clamp kept an overlapping rest %v; it must drop it and fall back to the offered-order complement", out.Rest)
	}
}
