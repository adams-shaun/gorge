package kgen

import (
	"os"

	"github.com/adams-shaun/gorge/dirac/kop"
	"path/filepath"
	"strings"
	"testing"
)

// formCase is one lowering: the
// carrier must lower with no refusal (refused == "") or refuse with a reason
// containing refused (kept on purpose), and its lowered IR
// (card and tokens) must contain lir.
type formCase struct {
	card, form, lir, refused string
}

var staticConditionForms = []formCase{
	{card: "Fallaji Excavation", form: "token mana RestrictValid$ CantCastNonArtifactSpells (Powerstone)", lir: "restrict={types=0x4}"},
	{card: "Cast Down", form: "nonLegendary (NotTypes TLegendary)", lir: "types=0x2 not=0x100"},
	{card: "Angry Mob", form: "static Condition$ NotPlayerTurn", lir: "cond=!YourTurn()"},
	{card: "Carapace Forger", form: "static Condition$ Metalcraft", lir: "cond=Present({types=0x4 ctrl=1},op1,3)"},
	{card: "Bladeback Sliver", form: "static Condition$ Hellbent", lir: "cond=Value(op3,HandSizeOf(You),0)"},
	{card: "Stoic Rebuttal", form: "self ReduceCost Condition$ Metalcraft", lir: "static ReduceCost F={types=0x0 Self} F2=nil cond=Present("},
	{card: "Slithering Shade", form: "Hellbent outside Continuous (gorge gate unverified)", refused: `static Condition "Hellbent"`},
	{card: "Wheel of Fortune", form: "discard Mode$ Hand (whole hand, no ask)", lir: "Discard obj=EachPlayer A=127 CS=DiscarderChooses"},
	{card: "Dread Wight", form: "Phase$ EndCombat lowers (the refusal moves on to its body)", refused: `Effect replacement Duration$`},
	{card: "Realm Seekers", form: "Count$ValidHand Card (both hands)", lir: "A=Sum(HandSizeOf(n=1,You),HandSizeOf(n=1,EachOpp))"},
	{card: "Wasteland", form: "Land.nonBasic (NotTypes TBasic)", lir: "F={types=0x1 not=0x200}"},
	{card: "Ceremonious Rejection", form: "Colorless (NotColors WUBRG)", lir: "notcolor=WUBRG"},
	{card: "Reach of Shadows", form: "nonColorless (any-of Colors WUBRG)", lir: "color=WUBRG"},
	{card: "Thrull Surgeon", form: "discard Mode$ LookYouChoose = RevealYouChoose", lir: "CS=YouChoose"},
	{card: "Amnesia", form: "discard Mode$ RevealDiscardAll (every DiscardValid$ match, no ask)", lir: "A=127 CS=DiscarderChooses F={types=0x0 not=0x1}"},
	{card: "Avacyn's Memorial", form: "Permanent.Legendary", lir: "types=0x100 not=0x60"},
}

func TestStaticConditionAndFilterForms(t *testing.T) {
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	var names []string
	for _, f := range staticConditionForms {
		names = append(names, f.card)
	}
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	byName := map[string]*lCard{}
	for _, c := range lw.cards {
		byName[c.Name] = c
	}
	for _, f := range staticConditionForms {
		c := byName[f.card]
		if c == nil {
			t.Errorf("%s: not lowered", f.card)
			continue
		}
		var reasons []string
		for _, r := range c.Refusals {
			reasons = append(reasons, r.Node+": "+r.Reason)
		}
		got := strings.Join(reasons, "; ")
		if f.refused == "" && got != "" {
			t.Errorf("%s (%s): refused: %s", f.card, f.form, got)
		}
		if f.refused != "" && !strings.Contains(got, f.refused) {
			t.Errorf("%s (%s): want refusal %q, got %q", f.card, f.form, f.refused, got)
		}
		if f.lir == "" {
			continue
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		for _, tn := range c.Tokens {
			for _, tc := range lw.tokens {
				if tc.Name == tn {
					dumpLIR(&sb, tc)
				}
			}
		}
		if !strings.Contains(sb.String(), f.lir) {
			t.Errorf("%s (%s): lowered IR lacks %q:\n%s", f.card, f.form, f.lir, sb.String())
		}
	}
}

// TestLegendaryFilter: Legendary rides in the any-of Filter.Types alone
// (a Legendary head was silently dropped to "any card" before).
func TestLegendaryFilter(t *testing.T) {
	cb := &cardBuilder{}
	for _, tc := range []struct {
		expr          string
		types, nTypes kop.TypeMask
	}{
		{"Legendary", kop.TLegendary, 0},
		{"Card.Legendary", kop.TLegendary, 0},
		{"Permanent.Legendary+YouCtrl", kop.TLegendary, kop.TInstant | kop.TSorcery},
		{"Creature.nonLegendary", kop.TCreature, kop.TLegendary},
	} {
		f, err := parseFilter(tc.expr, func(string) bool { return false }, func(string) bool { return false })
		if err != nil {
			t.Fatalf("%s: %v", tc.expr, err)
		}
		kf, err := cb.clause(f.Clauses[0], kop.ZoneBattlefield)
		if err != nil {
			t.Fatalf("%s: %v", tc.expr, err)
		}
		if kf.Types != tc.types || kf.NotTypes != tc.nTypes {
			t.Errorf("%s: types %#x not %#x, want %#x not %#x", tc.expr, kf.Types, kf.NotTypes, tc.types, tc.nTypes)
		}
	}
}

// TestPhaseNames: gorge state.ParsePhases' EndCombat (COMBAT_END) and the
// corpus's "End Of Turn" spelling (case-insensitive in gorge).
func TestPhaseNames(t *testing.T) {
	if st, ok := stepMap[phaseSteps["EndCombat"]]; !ok || st != kop.StepEndCombat {
		t.Errorf("EndCombat -> %v %v", st, ok)
	}
	if st, ok := stepMap[phaseSteps["End Of Turn"]]; !ok || st != kop.StepEnd {
		t.Errorf("End Of Turn -> %v %v", st, ok)
	}
}
