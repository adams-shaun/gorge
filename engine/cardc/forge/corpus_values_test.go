package forge

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// valueUse is one distinct resolved value text: the text, a card table that
// resolves it, and its use count.
type valueUse struct {
	text  string
	svars map[string]string
	n     int
}

// resolvedKey is text plus the name=body pair of every SVar it reaches,
// sorted: two uses with one key lower alike.
func resolvedKey(text string, svars map[string]string) string {
	seen := map[string]bool{}
	var pairs []string
	var walk func(s string)
	walk = func(s string) {
		for _, tok := range strings.FieldsFunc(s, func(r rune) bool { return !isName(string(r)) && (r < '0' || r > '9') }) {
			if body, ok := svars[tok]; ok && !seen[tok] {
				seen[tok] = true
				pairs = append(pairs, tok+"="+body)
				walk(body)
			}
		}
	}
	walk(text)
	slices.Sort(pairs)
	return text + "\x00" + strings.Join(pairs, "\x00")
}

// valueUses walks one card like filterUses, counting the values of ValueKeys
// into uses by resolved key.
func valueUses(card *cards.Card, uses map[string]*valueUse) {
	for _, f := range card.Faces {
		visitParams := func(params map[string]string) {
			for _, k := range ValueKeys {
				v, ok := params[k]
				if !ok {
					continue
				}
				v = strings.TrimSpace(v)
				key := resolvedKey(v, f.SVars)
				if u := uses[key]; u != nil {
					u.n++
				} else {
					uses[key] = &valueUse{text: v, svars: f.SVars, n: 1}
				}
			}
		}
		visitSA := func(sa *cards.SA) {
			for seen := 0; sa != nil && seen < 64; sa, seen = sa.Sub, seen+1 {
				visitParams(sa.Params)
			}
		}
		for _, sa := range f.Abilities {
			visitSA(sa)
		}
		for _, tr := range f.Triggers {
			visitParams(tr.Params)
			visitSA(tr.Effect)
		}
		for _, st := range f.Statics {
			visitParams(st.Params)
		}
		for _, r := range f.Repls {
			visitParams(r.Params)
			visitSA(r.With)
		}
		names := make([]string, 0, len(f.SVars))
		for name := range f.SVars {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			visitSA(cards.ResolveSVar(f.SVars, name))
		}
	}
}

// corpusValueUses counts every corpus value text in censusShards shards.
func corpusValueUses(t *testing.T, all []*cards.Card) map[string]*valueUse {
	shards := make([]map[string]*valueUse, censusShards)
	var mu sync.Mutex
	t.Run("walk", func(t *testing.T) {
		for i := range censusShards {
			t.Run(fmt.Sprintf("shard-%d", i), func(t *testing.T) {
				t.Parallel()
				uses := map[string]*valueUse{}
				for j := i; j < len(all); j += censusShards {
					valueUses(all[j], uses)
				}
				mu.Lock()
				shards[i] = uses
				mu.Unlock()
			})
		}
	})
	merged := map[string]*valueUse{}
	for _, s := range shards {
		for key, u := range s {
			if m := merged[key]; m != nil {
				m.n += u.n
			} else {
				merged[key] = u
			}
		}
	}
	return merged
}

// readValueCensus reads testdata/value-refused.tsv into "kind\tatom" rows.
func readValueCensus(t *testing.T) map[string]bool {
	f, err := os.Open("testdata/value-refused.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		kind, _, ok := strings.Cut(line, "\t")
		if _, known := ParseRefusalKind(kind); !ok || !known {
			t.Fatalf("value-refused.tsv: bad row %q", line)
		}
		rows[line] = true
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

// TestCorpusValuesLower lowers every distinct resolved corpus value text and
// holds the refused (kind, atom) pairs to testdata/value-refused.tsv, both
// ways.
func TestCorpusValuesLower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	uses := corpusValueUses(t, reg.AllCards())
	keys := make([]string, 0, len(uses))
	for key := range uses {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	total, lowered := 0, 0
	perKind := map[RefusalKind]int{}
	perRow := map[string]int{}
	for _, key := range keys {
		u := uses[key]
		total += u.n
		svars := u.svars
		v, r := LowerValue(u.text, func(n string) (string, bool) { b, ok := svars[n]; return b, ok })
		if r != nil {
			perKind[r.Kind] += u.n
			perRow[r.Kind.String()+"\t"+r.Atom] += u.n
			continue
		}
		if err := ir.ValidateValue(v); err != nil {
			t.Errorf("LowerValue(%q) does not validate: %v", u.text, err)
			continue
		}
		lowered += u.n
	}
	if total < 40000 {
		t.Fatalf("census read %d value uses, want at least 40000: the walk is not reading the corpus", total)
	}
	ratio := float64(lowered) / float64(total)
	t.Logf("lowered %d of %d uses (ratio %.4f) over %d distinct resolved texts", lowered, total, ratio, len(keys))
	for k := RefuseNone + 1; k < refusalKindCount; k++ {
		if perKind[k] > 0 {
			t.Logf("refused %-18s %6d uses", k, perKind[k])
		}
	}
	if ratio < 0.85 {
		t.Errorf("lowered ratio %.4f, want at least 0.85", ratio)
	}

	rows := make([]string, 0, len(perRow))
	for row := range perRow {
		rows = append(rows, row)
	}
	slices.Sort(rows)
	if os.Getenv("FORGE_VALUE_CENSUS_DUMP") != "" {
		var b strings.Builder
		for _, row := range rows {
			fmt.Fprintf(&b, "%s\t%d\n", row, perRow[row])
		}
		t.Logf("measured set (kind, atom, uses):\n%s", b.String())
	}
	want := readValueCensus(t)
	for _, row := range rows {
		if !want[row] {
			t.Errorf("new refusal %q (%d uses): read it or add the row to testdata/value-refused.tsv", row, perRow[row])
		}
	}
	stale := make([]string, 0)
	for row := range want {
		if perRow[row] == 0 {
			stale = append(stale, row)
		}
	}
	slices.Sort(stale)
	for _, row := range stale {
		t.Errorf("stale: no corpus value refuses %q any more; delete the row", row)
	}
}
