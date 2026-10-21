package rules

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// grantedKeywordGapsFile pins every keyword head a corpus card GRANTS
// (Pump/PumpAll KW$, Animate/AnimateAll Keywords$, a static's AddKeyword$)
// that has no registered kw: primitive. The coverage walk
// (cards.Face.Primitives) reads only printed K: lines, so a card granting an
// unread keyword counts as playable while the grant is inert (CR audit
// S08a#5: Virulent Sliver's Poisonous, Banding Sliver's Banding). The list
// may only shrink.
const grantedKeywordGapsFile = "testdata/granted-keyword-gaps.txt"

// grantedKeywordHeads returns the sorted, distinct keyword heads face f
// grants through KW$, Keywords$ or AddKeyword$, walking the same reachable
// bodies as the coverage walk: every ability and its SubAbility$ chain,
// trigger and replacement bodies, every SVar ability, the face's statics and
// the raw statics an Effect names. HIDDEN-prefixed members and sentence
// members (ending in ".") are static-text keywords, not keyword abilities,
// and are left out.
func grantedKeywordHeads(f *cards.Face) []string {
	set := map[string]bool{}
	add := func(list string) {
		for _, m := range cards.SplitKeywordList(list) {
			if strings.HasPrefix(m, "HIDDEN") || strings.HasSuffix(m, ".") {
				continue
			}
			if h := cards.KeywordHead(m); h != "" {
				set[h] = true
			}
		}
	}
	seen := map[*cards.SA]bool{}
	var walk func(sa *cards.SA)
	walk = func(sa *cards.SA) {
		if sa == nil || seen[sa] {
			return
		}
		seen[sa] = true
		add(sa.ParamStr(cards.PKKW))
		add(sa.ParamStr(cards.PKKeywords))
		add(sa.ParamStr(cards.PKAddKeyword))
		walk(sa.Sub)
	}
	for _, a := range f.Abilities {
		walk(a)
	}
	for _, tr := range f.Triggers {
		walk(tr.Effect)
	}
	for _, r := range f.Repls {
		walk(r.With)
	}
	f.EachSVarAbility(walk)
	for _, s := range f.Statics {
		add(s.ParamStr(cards.PKAddKeyword))
	}
	f.EachRawEffectChild(func(c cards.EffectChild) {
		if c.Static != nil {
			add(c.Static.ParamStr(cards.PKAddKeyword))
		}
	})
	out := make([]string, 0, len(set))
	for h := range set {
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}

// TestGrantedKeywordHeadsWalksEveryGrantShape pins the extractor on one
// fixture face carrying each grant shape.
func TestGrantedKeywordHeadsWalksEveryGrantShape(t *testing.T) {
	t.Parallel()
	c := card(t, "Name:Granter\nManaCost:1\nTypes:Creature\nPT:1/1\n"+
		"A:AB$ Pump | Cost$ 1 | Defined$ Self | KW$ Banding & Flying | SubAbility$ DBAnimate\n"+
		"SVar:DBAnimate:DB$ Animate | Defined$ Self | Keywords$ Rampage:2\n"+
		"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddKeyword$ Poisonous:1 & HIDDEN CARDNAME can't block. & Prevent all damage that would be dealt by CARDNAME.\n"+
		"Oracle:x\n")
	got := strings.Join(grantedKeywordHeads(c.Faces[0]), ",")
	if want := "Banding,Flying,Poisonous,Rampage"; got != want {
		t.Fatalf("grantedKeywordHeads = %q, want %q", got, want)
	}
}

// TestGrantedKeywordHeadsAreRegisteredOrPinned is the class ratchet: every
// head the corpus grants either has a registered kw: primitive or is pinned
// in grantedKeywordGapsFile, and every pinned head is still an unregistered
// grant. Registering a head (with its proof test) means deleting its line;
// a newly granted unread head fails here by name. Regenerate after a
// deliberate change with GORGE_UPDATE_GRANTED_KEYWORDS=1.
func TestGrantedKeywordHeadsAreRegisteredOrPinned(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	count := map[string]int{}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			for _, h := range grantedKeywordHeads(f) {
				if !sup["kw:"+h] {
					count[h]++
				}
			}
		}
	}
	got := make([]string, 0, len(count))
	for h := range count {
		got = append(got, h)
	}
	sort.Strings(got)
	path := filepath.Join(".", grantedKeywordGapsFile)
	if os.Getenv("GORGE_UPDATE_GRANTED_KEYWORDS") == "1" {
		if err := os.WriteFile(path, []byte(strings.Join(got, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pinned := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line != "" {
			pinned[line] = true
		}
	}
	for _, h := range got {
		if !pinned[h] {
			t.Errorf("granted keyword %q (%d cards) has no registered kw: primitive and is not pinned in %s: implement and register it, or pin it", h, count[h], grantedKeywordGapsFile)
		}
	}
	for h := range pinned {
		if count[h] == 0 {
			t.Errorf("%s pins %q, which is now registered or no longer granted: delete the line", grantedKeywordGapsFile, h)
		}
	}
	t.Logf("%d unregistered granted keyword heads pinned", len(got))
}
