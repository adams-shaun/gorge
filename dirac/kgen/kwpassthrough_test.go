package kgen

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// T05: kgen stops refusing a keyword head gorge fully expands into IR
// (cards.KeywordExpansionOnly). When the expansion lines are on the face,
// lowerKeywords' pass-through arm consumes the K: node and the expansion's
// own abilities lower like any printed ability.

// TestKwPassThroughSoulshiftLowers: Body of Jukai's printed K:Soulshift:8 is
// consumed and its expansion trigger lowers: a ChangesZone death trigger
// (Battlefield -> Graveyard, gorge kwSoulshift) whose body is a
// graveyard-to-hand ChangeZone of a target Spirit card with mana value 8 or
// less.
func TestKwPassThroughSoulshiftLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Body of Jukai")
	if !ok || card == nil {
		t.Fatal("Body of Jukai not in the corpus")
	}
	// precondition: the printed keyword is on the face
	var kwLine string
	for _, k := range card.Faces[0].Keywords {
		if strings.HasPrefix(k, "Soulshift") {
			kwLine = k
		}
	}
	if kwLine == "" {
		t.Fatal("Body of Jukai has no printed Soulshift line: the fixture is stale")
	}
	c := lowerCorpusCard(t, "Body of Jukai")
	if len(c.Refusals) != 0 {
		t.Fatalf("Body of Jukai refused: %v", c.Refusals)
	}
	var found *lAbility
	for _, a := range c.Abilities {
		if a.Trigger != nil && a.Trigger.Event == "Dies" {
			found = a
		}
	}
	if found == nil {
		t.Fatalf("Body of Jukai has no death trigger; abilities %d", len(c.Abilities))
	}
	if len(found.Targets) != 1 {
		t.Fatalf("Soulshift trigger has %d targets, want 1", len(found.Targets))
	}
	tg := found.Targets[0]
	var spirit, cmc bool
	for _, cl := range tg.F.Clauses {
		spirit = spirit || cl.Subtype == "Spirit"
		cmc = cmc || cl.CmpStat == cmpCMC && cl.CmpOp == opLE && cl.CmpN == 8
	}
	if !spirit || !cmc {
		t.Fatalf("Soulshift target filter = %s, want Spirit + cmcLE8", tg.F)
	}
	var hand bool
	for _, in := range found.Prog {
		if in.Op == "Return" && in.C == int(kop.ZoneHand) && strings.Contains(in.CS, "Graveyard") {
			hand = true
		}
	}
	if !hand {
		t.Fatalf("Soulshift body moves the target nowhere from the graveyard: prog %v", found.Prog)
	}
}

// TestKwPassThroughFlankingStaysRefused: Flanking is not expansion-only
// (gorge's rules read the head, engine/rules/trigger_blocks.go), so the
// pass-through does not consume it and the refusal keeps its kw: class.
func TestKwPassThroughFlankingStaysRefused(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	name := "Benalish Cavalry"
	c0, ok := reg.Lookup(name)
	if !ok || c0 == nil {
		t.Fatalf("%s not in the corpus", name)
	}
	var hasKw bool
	for _, k := range c0.Faces[0].Keywords {
		hasKw = hasKw || strings.HasPrefix(k, "Flanking")
	}
	if !hasKw {
		t.Fatal("Benalish Cavalry has no printed Flanking line: the fixture is stale")
	}
	c := lowerCorpusCard(t, name)
	refused := false
	for _, r := range c.Refusals {
		if strings.Contains(r.Reason, `keyword "Flanking"`) {
			refused = true
		}
	}
	if !refused {
		t.Fatalf("%s refused %v, want the kw:Flanking keyword refusal", name, c.Refusals)
	}
}

// kwpassthroughLIRPin is testdata/kwpassthrough_lirkeys.tsv: the LIRKeys of
// every FDN, BLB and MBC card that accepted with no refusal before the
// pass-through landed, pinned so the pass-through can be shown to change no
// key of a card that lowered before (the additive rule). Cards that refused
// before are not pinned: a consumed keyword node removes their refusal, so
// their key moving is the pass-through working.
const kwpassthroughLIRPin = "testdata/kwpassthrough_lirkeys.tsv"

// TestKwPassThroughLIRKeysUnchanged: over the FDN, BLB and MBC lists, every
// card whose key the pin holds lowers again to that same key. A newly lowered
// card (its key joins the map) is fine; a moved or missing pinned key is a
// lowering change the pass-through must not make.
func TestKwPassThroughLIRKeysUnchanged(t *testing.T) {
	f, err := os.Open(kwpassthroughLIRPin)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	pin := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		name, key, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("pin row %q is not name<TAB>key", line)
		}
		pin[name] = key
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(pin) < 2650 {
		t.Fatalf("pin holds %d keys, want the FDN+BLB+MBC cards that accepted before", len(pin))
	}
	var names []string
	for n := range pin {
		names = append(names, n)
	}
	got, err := LIRKeysFor(ratchetCards(t), names)
	if err != nil {
		t.Fatal(err)
	}
	moved := 0
	for n, k := range pin {
		if got[n] != k {
			moved++
			if moved <= 5 {
				t.Errorf("%s: key %q, pinned %q", n, got[n], k)
			}
		}
	}
	if moved != 0 {
		t.Fatalf("%d of %d pinned keys moved", moved, len(pin))
	}
}
