package canon

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestChooseKindsPayLifeAppended: "paylife" is appended after every kind that
// predates it (not necessarily the tail: later appends such as "riot" go after
// it or before it only by landing order), is its own kind and never "number"'s,
// and the kinds before it keep their absolute ordinals (the vocabulary is
// append-only: recorded actions, nets and goldens key on them).
func TestChooseKindsPayLifeAppended(t *testing.T) {
	code, ok := ChooseCode("paylife")
	if !ok {
		t.Fatal("paylife is not in ChooseKinds")
	}
	// asunblocked (118) was the tail before paylife was appended; paylife is
	// after it, so it moved no existing ordinal.
	const lastBefore = 118
	if int(code) <= lastBefore || ChooseKinds[code] != "paylife" {
		t.Fatalf("paylife at %d of %d, want after the pre-existing tail %d", code, len(ChooseKinds), lastBefore)
	}
	if n, _ := ChooseCode("number"); n == code {
		t.Fatal("paylife shares number's code")
	}
	// the ordinals the entries before the append already had
	for kind, want := range map[string]int{"x": 0, "number": 8, "yes": 9, "asunblocked": lastBefore} {
		if got := slices.Index(ChooseKinds, kind); got != want {
			t.Errorf("%s is at %d, want %d", kind, got, want)
		}
	}
	// riot (origin/main) was appended before this change landed: it keeps its
	// ordinal right after asunblocked and paylife goes after it.
	if r := slices.Index(ChooseKinds, "riot"); r >= 0 && (r != lastBefore+1 || int(code) < r) {
		t.Errorf("riot at %d, paylife at %d: want riot at %d and paylife after it", r, code, lastBefore+1)
	}
	seen := map[string]bool{}
	for _, k := range ChooseKinds {
		if seen[k] {
			t.Errorf("kind %q listed twice", k)
		}
		seen[k] = true
	}
}

// TestChooseKindsPayLifeCanon: the kernel's pay-life ask (ChGenPayLife) canonicalises
// each amount as the "paylife" choose kind over the entering permanent, the
// amount in the mode, the form gorge's etbPayLifeOptions canonicalise to.
func TestChooseKindsPayLifeCanon(t *testing.T) {
	code, _ := ChooseCode("paylife")
	d := &dirac.Decision{Kind: dirac.DkChoose, Sub: dirac.ChGenPayLife, Player: 0, N: 3}
	for i := 0; i < 3; i++ {
		d.Options[i] = dirac.MakeAction(dirac.AkChoose, 5, 0, uint8(i*4))
	}
	tr := &Translator{S: new(dirac.State), D: d}
	if got := tr.DKind(); got != DChoose {
		t.Fatalf("decision kind %v, want DChoose", got)
	}
	for i := 0; i < 3; i++ {
		got := tr.Action(d.Options[i])
		want := Pack(ActChoose+CanonKind(code), 5, RefNone, uint8(i*4))
		if got != want {
			t.Fatalf("option %d = %#x, want %#x", i, got, want)
		}
		if got.Kind() == ActChoose+CanonKind(slices.Index(ChooseKinds, "number")) {
			t.Fatalf("option %d canonicalised as the number kind", i)
		}
	}
}
