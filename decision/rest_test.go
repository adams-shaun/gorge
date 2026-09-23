package decision

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// The pile-B order (Intent.Rest) unit tests: the partition rule's one home is
// Decision.validateRest, reached through Validate; ChosenRest is the handler's
// accessor. The engine-level behaviour (an actual scry) is covered in
// rules/arrange_rest_test.go.

// arrangeFixture is a scry-2-shaped arrange: three options, Min 0, Max 3,
// Restable, the shape effLookAndArrange poses.
func arrangeFixture() *Decision {
	opts := make([]Option, 3)
	for i := range opts {
		opts[i] = Option{Index: i, Kind: "bottom", Player: state.PlayerID(0)}
	}
	return &Decision{Seq: 7, Player: 0, Kind: KArrange, Min: 0, Max: 3,
		Restable: true, Options: opts}
}

// TestValidateAcceptsAnExactComplement: a well-formed partition passes.
func TestValidateAcceptsAnExactComplement(t *testing.T) {
	d := arrangeFixture()
	if err := d.Validate(Intent{Seq: 7, Player: 0, Choices: []int{0}, Rest: []int{2, 1}}); err != nil {
		t.Fatalf("Validate rejected a well-formed partition: %v", err)
	}
	// The full-window answer (everything chosen, empty complement).
	if err := d.Validate(Intent{Seq: 7, Player: 0, Choices: []int{0, 1, 2}}); err != nil {
		t.Fatalf("Validate rejected a full answer without Rest: %v", err)
	}
	if err := d.Validate(Intent{Seq: 7, Player: 0, Choices: []int{0, 1, 2}, Rest: []int{}}); err != nil {
		t.Fatalf("Validate rejected a full answer with an empty Rest: %v", err)
	}
}

// TestValidateRejectsRestOnOtherKinds: a non-arrange kind rejects a non-empty
// Rest outright -- the second list is an arrange concept.
func TestValidateRejectsRestOnOtherKinds(t *testing.T) {
	d := &Decision{Seq: 7, Player: 0, Kind: KChoose, Min: 1, Max: 1,
		Options: []Option{{Index: 0, Kind: "yes", Player: 0}, {Index: 1, Kind: "no", Player: 0}}}
	err := d.Validate(Intent{Seq: 7, Player: 0, Choices: []int{0}, Rest: []int{1}})
	if err == nil || !strings.Contains(err.Error(), "arrange") {
		t.Fatalf("Validate on a KChoose with Rest = %v, want a rejection naming the arrange-only rule", err)
	}
}

// TestChosenRestMirrorsChosenSemantics: nil on no rest and on an out-of-range
// index (all-or-nothing), the options in answer order otherwise.
func TestChosenRestMirrorsChosenSemantics(t *testing.T) {
	d := arrangeFixture()
	if got := d.ChosenRest(Intent{Seq: 7, Player: 0, Choices: []int{0}}); got != nil {
		t.Fatalf("ChosenRest without a Rest = %v, want nil (the legacy complement applies)", got)
	}
	if got := d.ChosenRest(Intent{Seq: 7, Player: 0, Choices: []int{0}, Rest: []int{9}}); got != nil {
		t.Fatalf("ChosenRest with an out-of-range index = %v, want nil (all-or-nothing)", got)
	}
	got := d.ChosenRest(Intent{Seq: 7, Player: 0, Choices: []int{0}, Rest: []int{2, 1}})
	if len(got) != 2 || got[0].Index != 2 || got[1].Index != 1 {
		t.Fatalf("ChosenRest = %+v, want options 2 then 1 in the answer's order", got)
	}
}

// TestRestRoundTripsWireShape: an intent carrying Rest serialises with the
// field and one without omits it -- old clients' bytes are unchanged.
func TestRestRoundTripsWireShape(t *testing.T) {
	with := Intent{Seq: 1, Player: 0, Choices: []int{0}, Rest: []int{2, 1}}
	b, err := json.Marshal(with)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), "\"rest\":[2,1]") {
		t.Fatalf("marshalled intent %s does not carry the rest list", b)
	}
	without := Intent{Seq: 1, Player: 0, Choices: []int{0}}
	b, err = json.Marshal(without)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "rest") {
		t.Fatalf("marshalled intent %s carries a rest field; omitempty must omit it", b)
	}
}
