package forge

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// refUse is one distinct reference text under the subject its key wants.
type refUse struct {
	text string
	want ir.Subject
}

// refUses counts the values of RefKeys on one card into uses.
func refUses(card *cards.Card, uses map[refUse]int) {
	eachParams(card, func(params map[string]string) {
		for _, k := range RefKeys {
			if v, ok := params[k.Key]; ok {
				uses[refUse{strings.TrimSpace(v), k.Subject}]++
			}
		}
	})
}

// definedKeys counts the keys starting "Defined" on one card.
func definedKeys(card *cards.Card, keys map[string]int) {
	eachParams(card, func(params map[string]string) {
		for k := range params {
			if strings.HasPrefix(k, "Defined") {
				keys[k]++
			}
		}
	})
}

// lowerRefFor lowers text for a key's subject; SubjNone reads it as an object,
// then on a wrong-subject refusal as a player.
func lowerRefFor(text string, want ir.Subject) (*ir.Ref, *Refusal) {
	if want != ir.SubjNone {
		return LowerRef(text, want)
	}
	r, ref := LowerRef(text, ir.SubjObject)
	if ref != nil && ref.Kind == RefuseWrongSubject {
		return LowerRef(text, ir.SubjPlayer)
	}
	return r, ref
}

// TestCorpusRefsLower lowers every distinct corpus reference text and holds
// the refused (kind, atom) pairs to testdata/ref-refused.tsv, both ways.
func TestCorpusRefsLower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	all := reg.AllCards()

	// Every corpus key starting "Defined" is classified.
	keys := corpusUses(t, all, definedKeys)
	for k := range keys {
		known := slices.Contains(NonRefDefinedKeys, k) ||
			slices.ContainsFunc(RefKeys, func(rk RefKey) bool { return rk.Key == k })
		if !known {
			t.Errorf("corpus key %s starts Defined but is in neither RefKeys nor NonRefDefinedKeys", k)
		}
	}

	uses := corpusUses(t, all, refUses)
	distinct := make([]refUse, 0, len(uses))
	for u := range uses {
		distinct = append(distinct, u)
	}
	slices.SortFunc(distinct, func(a, b refUse) int {
		if c := strings.Compare(a.text, b.text); c != 0 {
			return c
		}
		return int(a.want) - int(b.want)
	})

	total, lowered := 0, 0
	perKind := map[RefusalKind]int{}
	measured := map[string]bool{}
	for _, u := range distinct {
		n := uses[u]
		total += n
		ref, r := lowerRefFor(u.text, u.want)
		if r != nil {
			perKind[r.Kind] += n
			measured[r.Kind.String()+"\t"+r.Atom] = true
			continue
		}
		if err := ir.ValidateRef(ref); err != nil {
			t.Errorf("LowerRef(%q, %s) = %s does not validate: %v", u.text, u.want, ref, err)
			continue
		}
		lowered += n
	}
	if total < 15000 {
		t.Fatalf("census read %d reference uses, want at least 15000: the walk is not reading the corpus", total)
	}
	ratio := float64(lowered) / float64(total)
	t.Logf("lowered %d of %d uses (ratio %.4f) over %d distinct texts", lowered, total, ratio, len(distinct))
	for k := RefuseNone + 1; k < refusalKindCount; k++ {
		if perKind[k] > 0 {
			t.Logf("refused %-18s %6d uses", k, perKind[k])
		}
	}
	if ratio < 0.90 {
		t.Errorf("lowered ratio %.4f, want at least 0.90", ratio)
	}

	rows := make([]string, 0, len(measured))
	for row := range measured {
		rows = append(rows, row)
	}
	slices.Sort(rows)
	if os.Getenv("FORGE_REF_CENSUS_DUMP") != "" {
		t.Logf("measured set:\n%s", strings.Join(rows, "\n"))
	}
	want := readCensusFile(t, "testdata/ref-refused.tsv")
	for _, row := range rows {
		if !want[row] {
			t.Errorf("new refusal %q: read it or add the row to testdata/ref-refused.tsv", row)
		}
	}
	stale := make([]string, 0)
	for row := range want {
		if !measured[row] {
			stale = append(stale, row)
		}
	}
	slices.Sort(stale)
	for _, row := range stale {
		t.Errorf("stale: no corpus reference refuses %q any more; delete the row", row)
	}
}
