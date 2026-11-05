package kgen

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The acceptance ratchets: kgen lowers every
// registered card list in dirac/cardset/setlists and the accepted count
// (cards that lower, whether or not refused by name) must EQUAL the pin. A fall
// is a regression; a rise is re-pinned in the same commit, so the pins are
// always the live counts.
//
// The count is LOWER + BUILD (checkAcceptance runs buildPool): a card the LIR
// pass accepts but `generate` refuses (counter kinds, cost shapes, filter
// compares) is not accepted here, so the pins equal what can be emitted.
// The earlier LIR-only pins were higher (ALL 17340 vs 16588 built).
var acceptancePins = map[string][2]int{ // list -> {accepted, cards}
	// +1 (Brad Boimler, Eager Ensign: the tap-triggered until-end-of-turn AddCounter replacement; TRK 280 -> 281).
	// +1 (Daniels, Temporal Agent: PutCardToLibFromGrave<1/-1/CARDNAME> self cost; TRK 281 -> 282).
	// +3 (Khan, Vengeful Heart; Misthollow Griffin; Eternal Scourge: MayPlay from exile; TRK 282 -> 283, ALL 21449 -> 21452).
	// +1 (Infinite Diversity: X free copies of an exiled instant or sorcery; TRK 283 -> 284, ALL 21452 -> 21453).
	// +3 (Kira Nerys; The Holodeck; Gabriel Lorca: land's TRK cards, Holo-Engineer being the same change already counted; TRK 285 -> 288, ALL 21454 -> 21457).
	// +1 (First Contact: the first-spell-cast rider; TRK 288 -> 289, ALL 21457 -> 21458).
	// +1 (Lwaxana Troi, Charming Guest: the keep-tapped untap election and the tap-bound Effect-delivered Panharmonicon; land; merged TRK 289 -> 290, ALL 21458 -> 21459).
	// +1 (Dr. McCoy, Surly Surgeon: Destroy replacement removes +1/+1 counters and heals damage; TRK 290 -> 291, ALL 21459 -> 21460).
	// +2 (Gowron, Eager for Battle: Sacrificed$Valid Card.Klingon; Shilgengar, Sire of Famine: Card.Angel; TRK 291 -> 292, ALL 21460 -> 21462).
	// +1 (Nog, Ambitious Cadet: the Officer Animate grants a graveyard land permission and an extra land play; TRK 292 -> 293, ALL 21462 -> 21463).
	// +1 (Stray from History: a charm mode's hand move reads the owner from that mode's own player slot; TRK 293 -> 294, ALL 21463 -> 21464).
	// +1 (Kai Winn, Faithless Leader: the as-enters nonland card name and the opponent cast/activation trigger over it; TRK 294 -> 295, ALL 21464 -> 21465).
	// +4 (Captain Benjamin Sisko: a Play's mana-value bound names any one-letter SVar and Valid$ carries YouOwn; land; merged TRK 295 -> 296, ALL 21465 -> 21469).
	// +1 (Dominion Saboteur: the remembered-origin counter rider of an ETB copy; merged TRK 297 -> 298, ALL 21470 -> 21471).
	// +1 (The Inner Light: an Effect's remembered target and the damage amount reach its DamageDone emblem trigger; merged TRK 297 -> 298, ALL 21470 -> 21471).
	// +4 (a library search's WithCountersType$ entry rider: Captain Jean-Luc Picard, Magmatic Hellkite, Neoform and Wakanda Forever!; merged ALL 21472 -> 21476; TDM 243 -> 244, TRK 299 -> 300).
	// +4 (H.M.S. Bounty: Count$YourExtraTurns beside Defined$ You on AddTurn; land; merged ALL 21476 -> 21480, TRK 300 -> 301).
	// +2 (Those Old Scientists, Turn the Earth: a multi-target graveyard shuffle's per-owner move lowers to OpMayShuffleMove; merged TRK 299 -> 300, ALL 21472 -> 21474).
	// +1 (Q, Stacking the Deck: the exile-granted copy-and-cast ability of a card exiled from the top of the library; merged TRK 299 -> 300, ALL 21472 -> 21473).
	// +2 (U.S.S. Enterprise-D, Galaxy-Class and Axgard Artisan: CounterAddedOnce FirstTime$ True's per-source once-a-turn latch; MOM 196 -> 197, TRK 305 -> 306, ALL 21486 -> 21488).
	// +3 (Dream Harvest; Improvisation Capstone: RememberRevealed$+MinTotalCMC$ DigUntil, ECL/SOS +1; Black Widow, Super Spy: ImprintFound$ alone, MSH +1; ALL 21496 -> 21499).
	// +1 (Sanar, Innovative First-Year: the DigUntil RememberRevealed$ library reveal and its five per-colour picks; ECL 232 -> 233, ALL 21523 -> 21524).
	// +1 (Teval, Arbiter of Virtue: a granted Delve on the stack lowers and the cast announces its graveyard exiles; merged ALL 21523 -> 21524, TDM 244 -> 245).
	// Pin history: git log -p dirac/kgen/ratchet_test.go; a commit lists the cards it moves.
	"ALL": {21527, 34442},
	"BIG": {23, 30},
	"BLB": {256, 278},
	"BRO": {206, 280},
	"DFT": {247, 276},
	"DMU": {193, 270},
	"DSK": {236, 276},
	"ECL": {233, 273},
	"EOE": {238, 266},
	"FDN": {508, 517},
	"FIN": {266, 312},
	"FRA": {245, 269},
	"HOB": {152, 172},
	"LCI": {250, 290},
	"MKM": {231, 279},
	"MOM": {198, 296},
	"MSH": {226, 276},
	"NEO": {198, 287},
	"ONE": {195, 271},
	"OTJ": {240, 276},
	"SNC": {185, 266},
	"SOS": {248, 270},
	"SPM": {160, 192},
	"TDM": {245, 276},
	"TLA": {246, 285},
	"TMT": {171, 194},
	"TRC": {287, 337},
	"TRK": {311, 311},
	"WOE": {252, 281},
}

// testSetlistDir is setlistDir relative to the test's working directory
// (dirac/kgen), not the module root the -census-std flag runs from.
const testSetlistDir = "../../" + setlistDir

func ratchetCards(t *testing.T) string {
	t.Helper()
	cardsDir := os.Getenv("KGEN_CARDS")
	if cardsDir == "" {
		var err error
		if cardsDir, err = cardset.CorpusDir(); err != nil {
			t.Skipf("SKIP: acceptance ratchet NOT RUN: %v; link the corpus as .cards here", err)
		}
	}
	if _, err := os.Stat(cardsDir); err != nil {
		t.Skipf("SKIP: acceptance ratchet NOT RUN: corpus %s missing (%v); link it as .cards here", cardsDir, err)
	}
	return cardsDir
}

func checkAcceptance(t *testing.T, cardsDir, list string) {
	t.Helper()
	pin, ok := acceptancePins[list]
	if !ok {
		t.Fatalf("%s.txt has no pin in acceptancePins", list)
	}
	p, err := loadCardList(testSetlistPath(t, list), cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	// Lower AND build: buildPool appends the build-level refusals (counter
	// kinds, cost shapes, filter compares) to each card, so the pin equals
	// what `generate` can emit, not just what the LIR pass accepts.
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatalf("%s: build: %v", list, err)
	}
	cv := computeCoverage(p, lw)
	t.Logf("%s: %d of %d accepted", list, cv.lowered, cv.cards)
	got := [2]int{cv.lowered, cv.cards}
	if got != pin && os.Getenv("KGEN_REPIN") == "1" {
		repinMu.Lock()
		defer repinMu.Unlock()
		msg, err := repinFile("ratchet_test.go", repinRow(list), list, pin, got, os.Getenv("KGEN_REPIN_FALL"))
		if err != nil {
			t.Fatal(err)
		}
		t.Log(msg)
		return
	}
	if got != pin {
		t.Errorf("%s: accepted %d of %d, pinned %d of %d: a fall is a lowering regression; a rise is re-pinned in dirac/kgen/ratchet_test.go in the same commit",
			list, got[0], got[1], pin[0], pin[1])
	}
}

// TestCorpusAcceptance is the corpus ratchet (the generated ALL list, ~3 s).
func TestCorpusAcceptance(t *testing.T) {
	checkAcceptance(t, ratchetCards(t), "ALL")
}

// TestSetAcceptance is the per-set ratchet, one subtest per list.
func TestSetAcceptance(t *testing.T) {
	cardsDir := ratchetCards(t)
	ents, err := os.ReadDir(testSetlistDir)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, e := range ents {
		list, ok := strings.CutSuffix(e.Name(), ".txt")
		if !ok {
			continue
		}
		seen++
		t.Run(list, func(t *testing.T) { checkAcceptance(t, cardsDir, list) })
	}
	if seen != len(acceptancePins)-1 {
		t.Errorf("%d set lists, %d set pins: a list and its pin are added together", seen, len(acceptancePins)-1)
	}
}

var repinMu sync.Mutex

// repinRow matches one `"<LIST>": {a, b},` pin row; group 1 is everything up
// to the numbers, so only the numeric literals are replaced.
func repinRow(list string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^(\t"` + regexp.QuoteMeta(list) + `": *\{)\d+, *\d+(\},)`)
}

// repinSource rewrites the row in src to {got[0], got[1]}; comments and order
// are untouched. Groups 1 and 2 of row are what surrounds the numbers.
func repinSource(src string, row *regexp.Regexp, got [2]int) (string, bool) {
	if !row.MatchString(src) {
		return src, false
	}
	return row.ReplaceAllString(src, fmt.Sprintf("${1}%d, %d${2}", got[0], got[1])), true
}

// repinFile moves list's pin in path from pin to got and returns the line to
// print. A lower accepted count is refused, leaving the file alone, unless
// fallReason says why it is acceptable.
func repinFile(path string, row *regexp.Regexp, list string, pin, got [2]int, fallReason string) (string, error) {
	msg := fmt.Sprintf("%s: repinned to %d of %d", list, got[0], got[1])
	if got[0] < pin[0] {
		if fallReason == "" {
			return "", fmt.Errorf("%s: accepted fell from %d to %d: a fall is a regression; fix it, or set KGEN_REPIN_FALL=<reason> to accept it",
				list, pin[0], got[0])
		}
		msg += fmt.Sprintf(", accepted fell from %d: %s", pin[0], fallReason)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	out, ok := repinSource(string(b), row, got)
	if !ok {
		return "", fmt.Errorf("no pin row for %s in %s", list, path)
	}
	return msg, os.WriteFile(path, []byte(out), 0o644)
}

func TestRepinRewritesOnlyNumbers(t *testing.T) {
	src := "var acceptancePins = map[string][2]int{ // list\n\t// Pin history: x\n\t\"ALL\": {1, 2},\n\t// note\n\t\"BLB\": {3, 4},\n}\n"
	out, ok := repinSource(src, repinRow("BLB"), [2]int{30, 40})
	if !ok {
		t.Fatal("row not found")
	}
	want := strings.Replace(src, "{3, 4}", "{30, 40}", 1)
	if out != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
	if _, ok := repinSource(src, repinRow("XXX"), [2]int{1, 1}); ok {
		t.Error("missing list reported found")
	}
}

// fakePinFile is a pin file holding one list, FAKE, at {10, 20}.
func fakePinFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pins")
	if err := os.WriteFile(path, []byte("\t\"FAKE\": {10, 20},\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func wantFakePin(t *testing.T, path, want string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != "\t\"FAKE\": "+want+",\n" {
		t.Errorf("pin file holds %q, want the row at %s", got, want)
	}
}

func TestRepinRefusesFall(t *testing.T) {
	path := fakePinFile(t)
	_, err := repinFile(path, repinRow("FAKE"), "FAKE", [2]int{10, 20}, [2]int{9, 20}, "")
	if err == nil {
		t.Fatal("a fall with no reason was accepted")
	}
	if !strings.Contains(err.Error(), "FAKE") {
		t.Errorf("error does not name the list: %v", err)
	}
	wantFakePin(t, path, "{10, 20}")
}

func TestRepinFallWithReason(t *testing.T) {
	path := fakePinFile(t)
	msg, err := repinFile(path, repinRow("FAKE"), "FAKE", [2]int{10, 20}, [2]int{9, 20}, "a refusal that closes a divergence")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "a refusal that closes a divergence") {
		t.Errorf("message does not print the reason: %q", msg)
	}
	wantFakePin(t, path, "{9, 20}")
}

func TestRepinTakesARise(t *testing.T) {
	path := fakePinFile(t)
	if _, err := repinFile(path, repinRow("FAKE"), "FAKE", [2]int{10, 20}, [2]int{11, 21}, ""); err != nil {
		t.Fatal(err)
	}
	wantFakePin(t, path, "{11, 21}")
}
