package main

import (
	"testing"
	"testing/fstest"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestAuditNotLoweredRealCard lowers a real corpus card kgen refuses to lower
// (Bubbling Muck) and checks the row says not-lowered. A fake lir map cannot
// show this: kgen.LIRKeysFor gives every loaded card a key, including one that
// records a lowering refusal.
func TestAuditNotLoweredRealCard(t *testing.T) {
	cardsDir := testutil.CorpusDirOrSkip(t)
	reg := twoCardCorpus(t)
	corpus := testutil.CorpusRegistry(t)
	muck, ok := corpus.Lookup("Bubbling Muck")
	if !ok {
		t.Fatal("Bubbling Muck is not in the corpus")
	}
	reg.Add(muck)

	support, err := kgen.SupportFor(cardsDir, []string{"Bubbling Muck"})
	if err != nil {
		t.Fatal(err)
	}
	cs, ok := support["Bubbling Muck"]
	if !ok {
		t.Fatal("kgen returned no support for Bubbling Muck")
	}
	if cs.Status != cardset.StatusNotLowered {
		t.Fatalf("precondition: Bubbling Muck status = %v, want not-lowered", cs.Status)
	}
	if cs.LIRKey == "" {
		t.Fatal("precondition: Bubbling Muck must still have a LIR key, or the test does not exercise the bug")
	}

	fsys := fstest.MapFS{
		"dirac/support/evidence/a.txt": {Data: []byte("Bubbling Muck\n")},
	}
	lower := func([]string) (lowerResult, error) {
		return lowerResult{
			keys:    map[string]string{"Bubbling Muck": cs.LIRKey},
			support: support,
		}, nil
	}
	rows, err := auditRows(fsys, reg, lower, opts{})
	if err != nil {
		t.Fatal(err)
	}
	var got *row
	for i := range rows {
		if rows[i].Card == "Bubbling Muck" {
			got = &rows[i]
			break
		}
	}
	if got == nil {
		t.Fatal("Bubbling Muck has no row")
	}
	if got.DClaim != "not-lowered" {
		t.Fatalf("Bubbling Muck d_claim = %q with lir_now %q, want not-lowered: a LIR key is not lowering",
			got.DClaim, got.LIRNow)
	}
	if got.LIRNow != cs.LIRKey {
		t.Fatalf("Bubbling Muck lir_now = %q, want %q", got.LIRNow, cs.LIRKey)
	}
}

// TestFourSeatFromSupportWithoutHunt: kgen's TwoSeat flag is the 4-seat
// priority source, so -need does not need -hunt-keys to know a card is not
// two-seat only.
func TestFourSeatFromSupportWithoutHunt(t *testing.T) {
	reg := twoCardCorpus(t)
	in := inputs{
		reg: reg,
		support: map[string]kgen.CardSupport{
			"Sol Ring": {Status: cardset.StatusSupported, TwoSeat: false},
			"Push":     {Status: cardset.StatusSupported, TwoSeat: true},
		},
		ev:   &evidence{lists: map[string][]string{}},
		hunt: &huntIndex{},
	}
	if r := buildRow("Sol Ring", in); !r.FourSeat {
		t.Error("Sol Ring FourSeat = false, want true from TwoSeat=false")
	}
	if r := buildRow("Push", in); r.FourSeat {
		t.Error("Push FourSeat = true, want false from TwoSeat=true")
	}
}
