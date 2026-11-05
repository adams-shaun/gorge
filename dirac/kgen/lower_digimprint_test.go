package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// sanarScript is an inline stand-in for Sanar, Innovative First-Year's
// script (the real one is Forge's, GPL, and never committed); %s holes are
// the pieces the refusal variants change.
const sanarScript = `Name:Sanar Probe
ManaCost:2 UR UR
Types:Legendary Creature Goblin Sorcerer
PT:2/4
T:Mode$ Phase | Phase$ Main1 | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ PrDig | TriggerDescription$ probe
SVar:PrDig:DB$ DigUntil | Valid$ Card.nonLand | Amount$ PrX | RememberRevealed$ True DIGEXTRA | RevealedDestination$ Library | SubAbility$ PrPickW
SVar:PrPickW:DB$ ChangeZone | Origin$ Library | Destination$ Exile | ChooseFromDefined$ Remembered.White | ChangeNum$ 1 | Imprint$ True | ConditionCheckSVar$ PrHaveW | SubAbility$ PrShuf
SVar:PrShuf:DB$ Shuffle | SubAbility$ PrEff
SVar:PrEff:DB$ Effect | StaticAbilities$ PrPlay | RememberObjects$ Imprinted | ForgetOnMoved$ Exile | SubAbility$ PrClean
SVar:PrPlay:Mode$ Continuous | MayPlay$ True | AffectedZone$ Exile | Affected$ Card.nonLand+IsRemembered | Description$ probe cast.
SVar:PrClean:DB$ Cleanup | ClearImprinted$ True | ClearRemembered$ True
SVar:PrX:XVALUE
SVar:PrHaveW:Count$Valid Permanent.White+YouCtrl
Oracle:probe
`

// sanarProbe lowers the inline script after replacing DIGEXTRA / XVALUE and
// applying edit, returning the refusal reasons and the lowered card.
func sanarProbe(t *testing.T, digExtra, xvalue string, edit func(string) string) (refusals []string, lc *lCard) {
	t.Helper()
	src := strings.NewReplacer("DIGEXTRA", digExtra, "XVALUE", xvalue).Replace(sanarScript)
	if edit != nil {
		src = edit(src)
	}
	card, diags := cards.ParseBytes("sanar_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("script did not parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{byName: map[string]*cards.Card{"Sanar Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lc = lowerCard(p, "Sanar Probe", card, false)
	for _, rf := range lc.Refusals {
		refusals = append(refusals, rf.Reason)
	}
	return refusals, lc
}

// TestSanarInnovativeFirstYearLowers: the real card lowers with a
// colour-count amount, a stay-in-place library reveal, five imprinting
// hidden picks and a chosen-set MayPlay tail.
func TestSanarInnovativeFirstYearLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Sanar, Innovative First-Year")
	if !ok || card == nil {
		t.Fatal("Sanar, Innovative First-Year not in the corpus")
	}
	dig := card.Faces[0].SVars["TrigDigUntil"]
	if !strings.Contains(dig, "RevealedDestination$ Library") || !strings.Contains(dig, "RememberRevealed$ True") {
		t.Fatalf("TrigDigUntil is not the library RememberRevealed$ shape: %q", dig)
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	lc := lowerCard(p, card.Faces[0].Name, card, false)
	for _, rf := range lc.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
	var digs, picks, plays, clears int
	for _, a := range lc.Abilities {
		for _, in := range a.Prog {
			switch in.Op {
			case "DigUntil":
				digs++
				if in.A.Kind != vValidAgg || in.A.N != kop.AggColors || in.C != zLibrary ||
					!strings.Contains(in.CS, "revealed=Library|FoundWithRevealed|RememberRevealed") {
					t.Errorf("DigUntil = %s", in)
				}
			case "ChooseMove":
				picks++
				if in.C != zExile || !strings.Contains(in.CS, "from=Library|Imprint|NoShuffle") ||
					in.F == nil || in.F.Clauses[0].Flags&fRemembered == 0 {
					t.Errorf("ChooseMove = %s", in)
				}
			case "MayPlay":
				plays++
				if in.Obj.Kind != rChosen {
					t.Errorf("MayPlay obj %v, want the chosen set", in.Obj)
				}
			case "ClearChosen":
				clears++
			}
		}
	}
	if digs != 1 || picks != 5 || plays != 1 || clears != 1 {
		t.Fatalf("dig/pick/play/clear = %d/%d/%d/%d, want 1/5/1/1", digs, picks, plays, clears)
	}
}

// TestDigUntilRememberRevealedLibraryRefusals: the library RememberRevealed$
// shape accepts exactly Sanar's, and every neighbouring spelling keeps its
// refusal.
func TestDigUntilRememberRevealedLibraryRefusals(t *testing.T) {
	const colors = "Count$Valid Permanent.YouCtrl$Colors"
	if rs, _ := sanarProbe(t, "", colors, nil); len(rs) != 0 {
		t.Fatalf("the unmodified inline shape is refused (%v): the refusal cases below prove nothing", rs)
	}
	cases := []struct {
		name, extra, x string
		edit           func(string) string
		want           string
	}{
		{"position", "| RevealedLibraryPosition$ -1", colors, nil, "DigUntil RememberRevealed$ shape"},
		{"random order", "| RevealRandomOrder$ True", colors, nil, "DigUntil RememberRevealed$ shape"},
		{"other amount", "", "Count$Valid Creature.YouCtrl", nil, "DigUntil RememberRevealed$ shape"},
		{"ChooseFromDefined outside the dig", "", colors, func(s string) string {
			return strings.Replace(s, "| RememberRevealed$ True", "", 1)
		}, ""},
	}
	for _, c := range cases {
		rs, _ := sanarProbe(t, c.extra, c.x, c.edit)
		if len(rs) == 0 {
			t.Errorf("%s: lowered, want a refusal", c.name)
			continue
		}
		joined := strings.Join(rs, "; ")
		if c.want != "" && !strings.Contains(joined, c.want) {
			t.Errorf("%s: refusals %q, want %q", c.name, joined, c.want)
		}
		if c.want == "" && !strings.Contains(joined, "ChooseFromDefined") {
			t.Errorf("%s: refusals %q, want a ChooseFromDefined$ refusal", c.name, joined)
		}
	}
}
