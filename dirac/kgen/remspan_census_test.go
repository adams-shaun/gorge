package kgen

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

const remspanTSV = "testdata/remspan.tsv"

// remspanTexts maps a refusal-text substring to the set (rem or imp) it
// belongs to and, when the text names its own reader, that reader.
var remspanTexts = []struct{ text, set, reader string }{
	{"reads Remembered written by another ability", "rem", ""},
	{"a static reads Remembered written by an ability", "rem", "static"},
	{"a replacement reads Remembered written by an ability", "rem", "repl"},
	{"reads Imprinted written by another ability", "imp", ""},
	{"imprints without an unconditional Cleanup ClearImprinted$", "imp", ""},
	{"IsImprinted outside a chain that writes the imprinted set", "imp", ""},
	{"Defined$ Imprinted outside a DigUntil ImprintFound$", "imp", ""},
}

// remspanSet says which set a refusal reason is one of, and the reader its
// text names ("" when the node decides).
func remspanSet(reason string) (set, reader string) {
	for _, x := range remspanTexts {
		if strings.Contains(reason, x.text) {
			return x.set, x.reader
		}
	}
	if strings.Contains(reason, "unread params [") && strings.Contains(reason, "ClearImprinted") {
		return "imp", ""
	}
	return "", ""
}

// remspanTrigger names a Forge trigger's reader form.
func remspanTrigger(t *cards.Trigger) string {
	switch {
	case t.Mode == "Phase":
		return "phase"
	case t.Mode == "ChangesZone" && t.Params["Origin"] == "Battlefield" && t.Params["ValidCard"] == "Card.Self":
		return "leaves"
	}
	return "trigger"
}

// remspanReader names the reader form a refusal's node is.
func remspanReader(c *cards.Card, lc *lCard, node string) string {
	var face *cards.Face
	if len(c.Faces) > 0 {
		face = c.Faces[0]
	}
	trig := func(i int) string {
		if face == nil || i < 0 || i >= len(face.Triggers) {
			return "trigger"
		}
		return remspanTrigger(&face.Triggers[i])
	}
	idx := func(rest string) (int, bool) {
		i, _, ok := strings.Cut(strings.TrimPrefix(rest, "["), "]")
		n, err := strconv.Atoi(i)
		return n, ok && err == nil
	}
	switch {
	case strings.HasPrefix(node, "ab["):
		n, ok := idx(node[2:])
		if !ok {
			return "other"
		}
		for _, a := range lc.Abilities {
			if a.Index != n {
				continue
			}
			switch a.Kind {
			case akSpell:
				return "spell"
			case akActivated:
				return "activated"
			case akTriggered:
				if a.OriginOK && a.Origin&0x80 != 0 {
					return trig(a.Origin & 0x7f)
				}
				return "trigger"
			}
		}
		return "other"
	case strings.HasPrefix(node, "T["):
		if n, ok := idx(node[1:]); ok {
			return trig(n)
		}
	case strings.HasPrefix(node, "A["):
		n, ok := idx(node[1:])
		if ok && face != nil && n >= 0 && n < len(face.Abilities) {
			switch face.Abilities[n].Kind {
			case "SP":
				return "spell"
			case "AB":
				return "activated"
			}
		}
	case strings.HasPrefix(node, "S["):
		return "static"
	}
	return "other"
}

const (
	boundOne = iota
	boundN
	boundAll
)

// remspanWriterBound is one remembering/imprinting ability's bound.
func remspanWriterBound(sa *cards.SA) int {
	writes := false
	for k, v := range sa.Params {
		if (strings.HasPrefix(k, "Remember") || strings.HasPrefix(k, "Imprint")) && v != "False" {
			writes = true
		}
	}
	if !writes {
		return boundOne
	}
	switch sa.API {
	case "ChangeZoneAll", "PumpAll", "DestroyAll":
		return boundAll
	case "Dig", "Mill":
		if sa.Params["ChangeNum"] == "All" || sa.Params["NumCards"] == "X" {
			return boundAll
		}
	}
	for _, k := range []string{"ChangeNum", "TargetMax", "NumCards"} {
		if n, err := strconv.Atoi(sa.Params[k]); err == nil && n > 1 {
			return boundN
		}
	}
	return boundOne
}

// remspanBound is the card's widest writer bound across every face, ability,
// trigger effect and SVar chain.
func remspanBound(c *cards.Card) string {
	best := boundOne
	walk := func(sa *cards.SA) {
		for ; sa != nil; sa = sa.Sub {
			best = max(best, remspanWriterBound(sa))
		}
	}
	for _, f := range c.Faces {
		for _, a := range f.Abilities {
			walk(a)
		}
		for i := range f.Triggers {
			walk(f.Triggers[i].Effect)
		}
		names := make([]string, 0, len(f.SVars))
		for n := range f.SVars {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			walk(cards.ResolveSVar(f.SVars, n))
		}
	}
	return [...]string{"1", "n", "all"}[best]
}

// remspanCensus counts distinct cards per bucket key over a lowered pool.
func remspanCensus(p *pool, lw *lowered) map[string]int {
	seen := map[string]map[string]bool{}
	add := func(key, card string) {
		if seen[key] == nil {
			seen[key] = map[string]bool{}
		}
		seen[key][card] = true
	}
	for _, lc := range lw.cards {
		c := p.byName[lc.Name]
		if c == nil {
			continue
		}
		var sets [2]bool // rem, imp
		other := [2]bool{}
		for _, r := range lc.Refusals {
			set, reader := remspanSet(r.Reason)
			if set == "" {
				if r.Reason != partialClass {
					other[0], other[1] = true, true
				}
				continue
			}
			if reader == "" {
				reader = remspanReader(c, lc, r.Node)
			}
			add(set+"."+reader, lc.Name)
			if set == "rem" {
				sets[0] = true
			} else {
				sets[1] = true
			}
			if set == "rem" {
				other[1] = true
			} else {
				other[0] = true
			}
		}
		b := remspanBound(c)
		for i, set := range []string{"rem", "imp"} {
			if !sets[i] {
				continue
			}
			add(set+".bound."+b, lc.Name)
			add(set+".cards", lc.Name)
			if !other[i] {
				add(set+".sole", lc.Name)
			}
		}
	}
	out := map[string]int{}
	for k, m := range seen {
		out[k] = len(m)
	}
	return out
}

func remspanTable(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("# TestRememberedSpanCensus: distinct ALL-list cards per Remembered/Imprinted refusal bucket (key<TAB>cards)\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "%s\t%d\n", k, m[k])
	}
	return b.String()
}

func parseRemspanTable(src string) (map[string]int, error) {
	m := map[string]int{}
	for _, line := range strings.Split(strings.TrimSuffix(src, "\n"), "\n") {
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "\t")
		n, err := strconv.Atoi(v)
		if !ok || err != nil {
			return nil, fmt.Errorf("bad line %q", line)
		}
		m[k] = n
	}
	return m, nil
}

// remspanJudge holds got against want and returns the moved lines, sorted, and
// whether they fail: under DIRAC_PIN_STRICT=1 (the nightly and regen-main's
// check) any difference does; outside it any move — a rise or a fall, a
// vanished bucket included — is only a warning the caller prints, because the
// tables are bot-owned (scripts/regen-main.sh re-pins main).
func remspanJudge(want, got map[string]int, strict bool) (moved []string, fail bool) {
	for k := range got {
		if got[k] != want[k] {
			moved = append(moved, fmt.Sprintf("%s %d -> %d", k, want[k], got[k]))
		}
	}
	for k := range want {
		if _, ok := got[k]; !ok {
			moved = append(moved, fmt.Sprintf("%s %d -> 0", k, want[k]))
		}
	}
	slices.Sort(moved)
	return moved, strict && len(moved) > 0
}

// TestRememberedSpanCensus counts the cards the Remembered and Imprinted
// cross-ability refusals block, by the reader's form and the writer's bound,
// and holds the table against testdata/remspan.tsv (KGEN_REPIN=1 rewrites it):
// outside DIRAC_PIN_STRICT=1 any move only warns (the tables are bot-owned;
// scripts/regen-main.sh re-pins main), strict holds them exact.
func TestRememberedSpanCensus(t *testing.T) {
	heavyParallel(t)
	p, err := loadCardList(testSetlistPath(t, "ALL"), ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	p.allBlockers = true // keep reading past a refusal: the family is counted whatever else blocks the card
	got := remspanCensus(p, lowerPool(p))
	table := remspanTable(got)
	t.Logf("\n%s", table)
	if os.Getenv("KGEN_REPIN") == "1" {
		if err := os.WriteFile(remspanTSV, []byte(table), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	b, err := os.ReadFile(remspanTSV)
	if err != nil {
		t.Fatal(err)
	}
	want, err := parseRemspanTable(string(b))
	if err != nil {
		t.Fatal(err)
	}
	// A bucket counts refused cards, so any move is report material, not a
	// regression the ticket carries: outside DIRAC_PIN_STRICT=1 (the nightly)
	// it only warns, and strict holds the table exact.
	moved, fail := remspanJudge(want, got, pinStrict())
	if fail {
		t.Errorf("remembered-span census moved from %s (re-pin with KGEN_REPIN=1):\n%s", remspanTSV, strings.Join(moved, "\n"))
	} else if len(moved) > 0 {
		t.Logf("WARNING: remspan census moved from %s (scripts/regen-main.sh re-pins main):\n%s", remspanTSV, strings.Join(moved, "\n"))
	}
}
