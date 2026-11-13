package cards

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// TestCanonicalKeywordLineRewritesOnlyKnownSentences pins the translation
// table itself: a known Forge sentence head becomes the canonical head, any
// parameter is preserved, an unknown sentence is left untouched (so the
// coverage walk still reports it as an unknown primitive rather than
// advertising support), and an already-canonical line is unchanged (idempotent
// -- expandKeywords/link may run more than once on a cached face).
func TestCanonicalKeywordLineRewritesOnlyKnownSentences(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		// The known sentence, both a bare line and one carrying a parameter
		// (the canonical head must keep the parameter after its first colon).
		{"CARDNAME must be blocked if able.", "MustBlock"},
		{"CARDNAME must be blocked if able.:rider", "MustBlock:rider"},
		// Case-insensitive: Forge capitalises CARDNAME but the sentence
		// spelling is data, not a stable literal.
		{"cardname MUST be blocked if able.", "MustBlock"},
		// Already canonical -> unchanged (idempotent).
		{"MustBlock", "MustBlock"},
		// CR 506.5's alone family: the compound sentence and its two
		// siblings each canonicalise to their own head (the compound is not
		// CantAttackOrBlock -- it forbids a LONE declaration, not any).
		{"CARDNAME can't attack or block alone.", "CantAttackOrBlockAlone"},
		{"CARDNAME can't attack alone.", "CantAttackAlone"},
		{"CARDNAME can't block alone.", "CantBlockAlone"},
		{"cardname can't BLOCK alone.", "CantBlockAlone"},
		// CR 509.1c's two-blocker requirement (Gorm the Great) has its own
		// head, never MustBlock's.
		{"CARDNAME must be blocked by two or more creatures if able.", "MustBeBlockedByTwo"},
		// An unknown sentence that merely looks similar must NOT be rewritten:
		// "must be blocked by exactly one creature" is a different rule the
		// build does not model under either head.
		{"CARDNAME must be blocked by exactly one creature if able.",
			"CARDNAME must be blocked by exactly one creature if able."},
		// Ordinary keywords untouched.
		{"Flying", "Flying"},
		{"Equip:2", "Equip:2"},
	}
	for _, c := range cases {
		if got := CanonicalKeywordLine(c.in); got != c.want {
			t.Errorf("CanonicalKeywordLine(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestCanonicalKeywordLinePreservesUnknownSentencePrimitive is the fail-closed
// half in primitive terms: an unknown sentence must still intern its own
// `kw:` primitive, so the coverage walk cannot mistake it for a supported
// head.
func TestCanonicalKeywordLinePreservesUnknownSentencePrimitive(t *testing.T) {
	t.Parallel()
	c, diags := ParseBytes("unknown-sentence.txt", []byte(
		"Name:Unknown Sentence Fixture\nManaCost:1\nTypes:Creature Bear\nPT:2/2\n"+
			"K:CARDNAME can be your commander.\nOracle:x\n"))
	if len(diags) != 0 {
		t.Fatalf("parse diagnostics: %v", diags)
	}
	c.Link()
	prims := c.Primitives()
	found := false
	for _, p := range prims {
		if p == "kw:CARDNAME can be your commander." {
			found = true
		}
	}
	if !found {
		t.Fatalf("unknown sentence lost its own primitive: %v", prims)
	}
}

// TestMustBeBlockedCensus names every corpus card whose printed K: sentence
// "CARDNAME must be blocked if able." (CR 509.1a) is canonicalised to the
// MustBlock head. The census: the 16 printed
// carriers, plus Bumper Cars, which supplies the same requirement at runtime
// through a Pump SVar's `KW$ HIDDEN CARDNAME must be blocked if able.` and so
// never passes through the K: parser (it is read by the sentence arm of
// rules' parseHiddenKeyword). Raphael, Ninja Destroyer is the TMT
// carrier.
//
// The count is exact on purpose: a new upstream carrier or a dropped
// translation is a loud failure, which is the point of a census. A Forge
// corpus bump that adds carriers updates this list.
func TestMustBeBlockedCensus(t *testing.T) {
	r := compiledCorpus(t)
	var printed []string
	for _, c := range r.AllCards() {
		for _, f := range c.Faces {
			for _, k := range f.Keywords {
				if strings.EqualFold(KeywordHead(k), "MustBlock") {
					printed = append(printed, f.Name)
					break
				}
			}
		}
	}
	sort.Strings(printed)
	// Face names, not card names: a modal double-faced card carries the
	// keyword on the face that prints it (Tangleclaw Werewolf's back face is
	// Fibrous Entangler; Hinterland Hermit's is Hinterland Scourge; Lukamina,
	// Moon Druid's is Lukamina, Scorpion Form), and this walks faces.
	want := []string{
		"Canopy Stalker",
		"Fear of Being Hunted",
		"Fibrous Entangler",
		"Gaea's Protector",
		"Goblin Fire Fiend",
		"Gorm the Great",
		"Hinterland Scourge",
		"Holga, Relentless Rager",
		"Inescapable Brute",
		"Lukamina, Scorpion Form",
		"Madame Vastra",
		"Raphael, Ninja Destroyer",
		"Riveteers Decoy",
		"The Foretold Soldier",
		"Vinebred Brawler",
		"Zangief, the Red Cyclone",
	}
	if len(printed) != len(want) {
		t.Fatalf("must-be-blocked printed carriers = %d (%v), want %d (%v)",
			len(printed), printed, len(want), want)
	}
	for i := range want {
		if printed[i] != want[i] {
			t.Fatalf("must-be-blocked printed carrier[%d] = %q, want %q (full: %v)",
				i, printed[i], want[i], printed)
		}
	}

	// The runtime Pump carriers: a Pump/PumpAll SVar's `KW$ ... CARDNAME must
	// be blocked if able.` supplies the same requirement without passing
	// through the K: parser's canonicalisation, so rules reads it through the
	// sentence arm of parseHiddenKeyword. Measured 6 carriers (naming the
	// mechanism class); Bumper Cars is the first.
	const sentence = "HIDDEN CARDNAME must be blocked if able."
	var pump []string
	for _, c := range r.AllCards() {
		for _, f := range c.Faces {
			for _, body := range f.SVars {
				if strings.Contains(body, sentence) {
					pump = append(pump, f.Name)
					break
				}
			}
		}
	}
	sort.Strings(pump)
	wantPump := []string{
		"Bumper Cars",
		"Glorfindel, Dauntless Rescuer",
		"Goldenhide Ox",
		"Head Banger",
		"Magitek Scythe",
		"Neyith of the Dire Hunt",
	}
	if len(pump) != len(wantPump) {
		t.Fatalf("must-be-blocked runtime sentence carriers = %v, want %v", pump, wantPump)
	}
	for i := range wantPump {
		if pump[i] != wantPump[i] {
			t.Fatalf("must-be-blocked runtime carrier[%d] = %q, want %q (full: %v)",
				i, pump[i], wantPump[i], pump)
		}
	}
}

// TestCardNameSentenceKeywordClass pins the whole parser-artefact class the
// census covers: every printed `K:CARDNAME <english sentence>` head. A sentence
// head has no colon, so KeywordHead returns the whole sentence; only the
// heads in canonicalKeywordHeads are translated (the must-be-blocked one),
// and every other sentence stays an honest unknown primitive. This census
// names the class and asserts the translation did not silently swallow a
// sibling sentence, which is the failure mode a per-card fix would hide.
//
// Counted on faces (a modal DFC prints the sentence on one face); the class
// is 44 printed lines across 44 files at the corpus pin measured.
func TestCardNameSentenceKeywordClass(t *testing.T) {
	r := compiledCorpus(t)
	heads := map[string]int{}
	for _, c := range r.AllCards() {
		for _, f := range c.Faces {
			for _, k := range f.Keywords {
				head := KeywordHead(k)
				if strings.HasPrefix(head, "CARDNAME ") {
					heads[head]++
				}
			}
		}
	}
	// The must-be-blocked sentence and CR 506.5's alone family must NOT
	// survive as sentence heads: each is translated to its canonical head
	// (asserted above), so its carriers moved off the sentence spelling.
	if n := heads["CARDNAME must be blocked if able."]; n != 0 {
		t.Fatalf("%d face(s) still carry the untranslated must-be-blocked sentence head", n)
	}
	for _, h := range []string{
		"CARDNAME can't attack or block alone.",
		"CARDNAME can't attack alone.",
		"CARDNAME can't block alone.",
	} {
		if n := heads[h]; n != 0 {
			t.Fatalf("%d face(s) still carry the untranslated %q sentence head", n, h)
		}
	}
	// A sibling sentence whose behaviour is not modelled under any canonical
	// head stays untranslated, so the class is still visible to coverage.
	if heads["CARDNAME can be your commander."] == 0 {
		t.Fatalf("sibling sentence heads vanished: %v", heads)
	}
	var names []string
	for h, n := range heads {
		names = append(names, fmt.Sprintf("%s x%d", h, n))
	}
	sort.Strings(names)
	t.Logf("K:CARDNAME sentence class: %d distinct heads: %v", len(heads), names)
}
