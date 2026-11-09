package kgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMayPlayAltPayLife: a remembered-card MayPlay static whose alternative
// cost is life equal to the card's mana value (the corpus's Inside
// Information) lowers to a MayPlay carrying kop.MayPlayPayLife; the same
// static with any other alternative cost, a free or capped rider beside it,
// or a lifetime past the turn refuses by name.
func TestMayPlayAltPayLife(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Inside Information\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	c := lowerPool(p).cards[0]
	if got := mzReasons(c); got != "" {
		t.Fatalf("Inside Information refused: %s", got)
	}
	got := mzMayPlays(c)
	if len(got) != 1 || got[0].B.N&kop.MayPlayPayLife == 0 {
		t.Fatalf("Inside Information lowered to %v, want one MayPlay carrying MayPlayPayLife", got)
	}

	const head = "Name:Probe\nManaCost:1 U\nTypes:Sorcery\n" +
		"A:SP$ Dig | ValidTgts$ Opponent | DigNum$ 2 | ChangeNum$ All | DestinationZone$ Exile | RememberChanged$ True | SubAbility$ DBEffect | SpellDescription$ Probe.\n" +
		"SVar:DBEffect:DB$ Effect | RememberObjects$ RememberedCard | StaticAbilities$ Play | ForgetOnMoved$ Exile%s | SubAbility$ Wipe\n" +
		"SVar:Wipe:DB$ Cleanup | ClearRemembered$ True\n" +
		"SVar:Play:Mode$ Continuous | MayPlay$ True | Affected$ Card.IsRemembered | AffectedZone$ Exile | %s | Description$ Probe.\n"
	const life = "MayPlayAltManaCost$ PayLife<ConvertedManaCost>"
	ok := mzLower(t, fmt.Sprintf(head, "", life))
	if r := mzReasons(ok); r != "" {
		t.Fatalf("the probe shape refused: %s", r)
	}
	for _, tc := range []struct{ name, dur, rider, want string }{
		{"other cost", "", "MayPlayAltManaCost$ PayLife<2>", `MayPlayAltManaCost "PayLife<2>"`},
		{"mana cost", "", "MayPlayAltManaCost$ R", `MayPlayAltManaCost "R"`},
		{"free beside it", "", life + " | MayPlayWithoutManaCost$ True", "MayPlayAltManaCost"},
		{"capped", "", life + " | MayPlayLimit$ 1", "MayPlayAltManaCost"},
		{"past the turn", " | Duration$ UntilTheEndOfYourNextTurn", life, "MayPlayAltManaCost"},
	} {
		r := mzReasons(mzLower(t, fmt.Sprintf(head, tc.dur, tc.rider)))
		if !strings.Contains(r, tc.want) {
			t.Errorf("%s: refusal %q, want it to name %s", tc.name, r, tc.want)
		}
	}
}
