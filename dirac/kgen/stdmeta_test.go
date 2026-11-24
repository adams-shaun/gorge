package kgen

import (
	"os"
	"regexp"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// stdMetaSupportedPin and stdMetaRefusedPin pin the standard-meta pool's
// supported and refused counts (refused = 500 - supported: every card is one
// or the other). KGEN_REPIN=1 re-pins a rise in place (repinFile in
// ratchet_test.go); a fall is a lowering regression and is refused unless
// KGEN_REPIN_FALL=<reason> says why it is acceptable. Outside
// DIRAC_PIN_STRICT=1 a rise passes, so a ticket need not edit this line; the
// regen PR (scripts/regen-main.sh) carries the rise to main.
const (
	stdMetaSupportedPin, stdMetaRefusedPin = 455, 45
)

// stdMetaPinRow matches that const line; only the two numbers are replaced.
var stdMetaPinRow = regexp.MustCompile(`(?m)^(\tstdMetaSupportedPin, stdMetaRefusedPin *= *)\d+, *\d+`)

// The standard-meta pool (the 500 cards of 60 MTGGoldfish Standard decks) builds as
// one table: its 146 subtypes are past the old 128-bit masks, so the widening to
// kop.MaxSubtypes is what lets it build, and 53 of its cards stay refused.
func TestStdMetaPoolBuilds(t *testing.T) {
	cards := tableCorpus(t)
	p, _, prev, err := readPool(os.DirFS("../.."), "standard-meta", cards)
	if err != nil {
		t.Fatal(err)
	}
	if len(prev) == 0 {
		t.Fatal("standard-meta has no ids.tsv; run make dirac-tables")
	}
	d, cv, _, err := tableOf(p, "standard-meta", prev)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d cards, %d supported, %d subtypes", cv.cards, cv.cardsOK, len(d.Subtypes))
	if cv.cards != 500 {
		t.Errorf("%d cards, want 500", cv.cards)
	}
	if len(d.Subtypes) <= 128 {
		t.Errorf("%d subtypes: the pool no longer needs more than the old 128", len(d.Subtypes))
	}
	if len(d.Subtypes) > kop.MaxSubtypes {
		t.Errorf("%d subtypes, a table's masks hold %d", len(d.Subtypes), kop.MaxSubtypes)
	}
	refused := map[string]bool{}
	for _, r := range cv.refused {
		refused[r.Card] = true
	}
	pin := [2]int{stdMetaSupportedPin, stdMetaRefusedPin}
	got := [2]int{cv.cardsOK, len(refused)}
	if got != pin && os.Getenv("KGEN_REPIN") == "1" {
		repinMu.Lock()
		msg, err := repinFile("stdmeta_test.go", stdMetaPinRow, "standard-meta", pin, got, os.Getenv("KGEN_REPIN_FALL"))
		repinMu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		t.Log(msg)
		return
	}
	// judgePin, as the acceptance pins: a fall fails, a rise passes unless
	// DIRAC_PIN_STRICT=1 (the nightly), and scripts/regen-main.sh re-pins main.
	// An exact pin turned every ticket that lowers one more card red (#4317,
	// 11-24) without the change being wrong.
	if err := judgePin("standard-meta", pin, got, pinStrict()); err != nil {
		t.Error(err)
	}
}
