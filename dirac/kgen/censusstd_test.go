package kgen

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestStdCensus runs the live census over the real Standard lists and
// asserts only invariants no grammar change can move: two runs are
// byte-identical, every set's accepted/listed equal the ratchet pins, and
// the short-of-90 total follows from the pins. Class orders, steps and
// bundle rows are pinned on constructed tables (TestStd*Constructed).
func TestStdCensus(t *testing.T) {
	heavyParallel(t)
	std, err := readCardList(filepath.Join("../../", standardListPath))
	if err != nil {
		t.Fatal(err)
	}
	cardsDir := ratchetCards(t)
	var firstMD, firstTSV string
	for i := 0; i < 2; i++ {
		res, err := computeSetlistCensus(testSetlistDir, cardsDir, std)
		if err != nil {
			t.Fatal(err)
		}
		var md, tsv strings.Builder
		writeStdCensus(&md, res)
		if err := writeStdTSV(&tsv, res); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			firstMD, firstTSV = md.String(), tsv.String()
			wantShort := 0
			for _, s := range res.sets {
				pin, ok := acceptancePins[s.set]
				if !ok {
					t.Fatalf("%s has no acceptance pin", s.set)
				}
				if s.accepted != pin[0] || s.listed != pin[1] {
					t.Errorf("%s accepted/listed = %d/%d, pin %d/%d", s.set, s.accepted, s.listed, pin[0], pin[1])
				}
				if s.std {
					if n := (9*pin[1]+9)/10 - pin[0]; n > 0 {
						wantShort += n
					}
				}
			}
			if res.stdTotalShort != wantShort {
				t.Errorf("std total short = %d, want %d from the pins", res.stdTotalShort, wantShort)
			}
			continue
		}
		if md.String() != firstMD {
			t.Fatal("census markdown differs between runs")
		}
		if tsv.String() != firstTSV {
			t.Fatal("census TSV differs between runs")
		}
	}
}

// TestStdGreedyConstructed pins completionGreedy on a hand-built table:
// sole-card count first, then the exact lcm score, then the shorter name,
// then the larger name.
func TestStdGreedyConstructed(t *testing.T) {
	ref := map[string][]string{
		"c1":  {"solo"},
		"c2":  {"solo"},
		"c3":  {"p", "q"},
		"c4a": {"p", "r"},
		"c5":  {"p", "q", "r"},
		"c6":  {"s", "t"},
		"c7":  {"ab"},
		"c8a": {"zz"},
	}
	// solo has 2 sole cards; zz and ab tie on imm and score, same length,
	// so zz wins. Then p (score) closes nothing; q and r tie, r wins and
	// closes mixedarms; q is then sole for c3 and c5; s and t tie, t wins first.
	order, got := completionGreedy(ref, 8)
	want := []greedyStep{
		{"solo", 2}, {"zz", 1}, {"ab", 1}, {"p", 0}, {"r", 1}, {"q", 2}, {"t", 0}, {"s", 1},
	}
	if !reflect.DeepEqual(order, want) || got != 8 {
		t.Fatalf("order/got = %v/%d, want %v/8", order, got, want)
	}
	// Equal imm and score: the shorter name wins over a larger one.
	order, _ = completionGreedy(map[string][]string{"a": {"long"}, "b": {"x"}}, 1)
	if want := []greedyStep{{"x", 1}}; !reflect.DeepEqual(order, want) {
		t.Fatalf("short-name tie = %v, want %v", order, want)
	}
	// Equal imm, score and length: the lexicographically larger name wins.
	order, _ = completionGreedy(map[string][]string{"a": {"m"}, "b": {"n"}}, 1)
	if want := []greedyStep{{"n", 1}}; !reflect.DeepEqual(order, want) {
		t.Fatalf("lex tie = %v, want %v", order, want)
	}
}

// TestStdJointConstructed pins jointGreedy across three sets and eight
// classes, including an imm tie broken by score, a score tie broken by the
// larger name, and a card shared by two sets.
func TestStdJointConstructed(t *testing.T) {
	sets := []string{"S1", "S2", "S3"}
	need := map[string]int{"S1": 2, "S2": 1, "S3": 1}
	ref := map[string]map[string][]string{
		"S1": {"a": {"A"}, "b": {"A"}, "c": {"B", "C"}, "d": {"D"}},
		"S2": {"e": {"A", "E"}, "f": {"F"}},
		"S3": {"g": {"G"}, "h": {"H"}, "i": {"C", "H"}},
	}
	steps := jointGreedy(sets, ref, need)
	want := []jointStep{{"A", 2, 2}, {"H", 1, 1}, {"F", 1, 0}}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("steps = %v, want %v", steps, want)
	}
	// A pure tie (imm 1, equal score): the larger name wins.
	steps = jointGreedy([]string{"S"}, map[string]map[string][]string{"S": {"a": {"M"}, "b": {"N"}}}, map[string]int{"S": 1})
	if want := []jointStep{{"N", 1, 0}}; !reflect.DeepEqual(steps, want) {
		t.Fatalf("tie steps = %v, want %v", steps, want)
	}
}

// TestStdBundlesConstructed pins computeBundles and the Standard-union card
// dedupe on constructed rows.
func TestStdBundlesConstructed(t *testing.T) {
	orders := [][]string{
		{"c1", "c2", "c3", "x"},
		{"c3", "c4a", "c5", "c6", "c7"},
		{"c3"},
	}
	labels := map[string]map[string]bool{
		"c1":  {"A": true, "B": true},
		"c2":  {"A": true, "B": true},
		"c3":  {"A": true, "B": true}, // refused in all three sets: one card
		"c4a": {"A": true},
		"c5":  {"C": true, "D": true},
		"c6":  {"C": true, "D": true},
		"c7":  {"C": true, "D": true},
		"x":   {},
	}
	sets := map[string]map[string]bool{
		"c1": {"S1": true}, "c2": {"S1": true}, "c3": {"S1": true, "S2": true, "S3": true},
		"c4a": {"S2": true}, "c5": {"S2": true}, "c6": {"S2": true}, "c7": {"S2": true},
	}
	cards := dedupeBundleCards(orders, labels, sets)
	if len(cards) != 7 {
		t.Fatalf("cards = %+v, want 7 (x has no labels, c3 once)", cards)
	}
	rows := computeBundles(cards)
	want := []stdBundleRow{
		{cards: 3, covered: 4, bundle: "A + B", sets: []stdSetCount{{"S1", 3}, {"S2", 2}, {"S3", 1}}, first: 0},
		{cards: 3, covered: 3, bundle: "C + D", sets: []stdSetCount{{"S2", 3}}, first: 1},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %+v, want %+v", rows, want)
	}
	// Triple-counting c3 would make A + B carry 5 cards; with only two
	// distinct A+B cards besides c3 the dedupe is what keeps it at 3.
}

// TestStdCensusBuildPass pins the build pass: Aerie Mystics is refused only at emit
// (PumpAll keyword above bit N; Forge, Neverwinter Charlatan's ward—sacrifice lowers since
// Maha's ward—discard), while Grizzly Bears and
// Boros Charm build. Boros Charm's "target player or planeswalker" lowers since
// so it no longer has an emit-refusal row.
func TestStdCensusBuildPass(t *testing.T) {
	dir := t.TempDir()
	names := []string{"Aerie Mystics", "Boros Charm", "Grizzly Bears"}
	if err := os.WriteFile(filepath.Join(dir, "AAA.txt"), []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := computeStdCensus(dir, ratchetCards(t), []string{"AAA"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.sets) != 1 || res.sets[0].accepted != 2 {
		t.Fatalf("sets = %+v, want AAA accepted 2", res.sets)
	}
	var mystics []stdTSVRow
	for _, r := range res.tsv {
		switch r.card {
		case "Aerie Mystics":
			mystics = append(mystics, r)
		case "Boros Charm":
			if r.class != "" {
				t.Fatalf("Boros Charm still refused: %+v", r)
			}
		}
	}
	foundEmit := false
	for _, r := range mystics {
		if r.class == "other:instr N PumpAll: PumpAll keyword above bit N" {
			foundEmit = true
		}
	}
	if !foundEmit {
		t.Fatalf("Aerie Mystics rows = %+v, want the emit-stage PumpAll refusal", mystics)
	}
}

// TestStdCensusTokenUnpack pins token unpacking: Toby's wrapper and the
// token's own can't-attack-or-block-alone keyword both lower, so the card
// carries no refusal row and no token-only class.
func TestStdCensusTokenUnpack(t *testing.T) {
	dir := t.TempDir()
	name := "Toby, Beastie Befriender"
	if err := os.WriteFile(filepath.Join(dir, "AAA.txt"), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := computeStdCensus(dir, ratchetCards(t), []string{"AAA"})
	if err != nil {
		t.Fatal(err)
	}
	var rows []stdTSVRow
	for _, r := range res.tsv {
		if r.card == name {
			rows = append(rows, r)
		}
	}
	var classes []string
	for _, c := range classesFromRows(rows) {
		if c != "" {
			classes = append(classes, c)
		}
	}
	if len(classes) != 0 {
		t.Fatalf("Toby expanded classes = %q, want none: the token's alone keyword lowers (rows %+v)", classes, rows)
	}
	for _, r := range res.classes {
		if r.class == "other:token refused" {
			t.Fatalf("ranking still has other:token refused: %+v", r)
		}
	}
	for _, r := range res.tsv {
		if r.via == "w_4_4_beast_lonely" {
			t.Fatalf("TSV still has a w_4_4_beast_lonely token refusal: %+v", r)
		}
	}
}

// TestStdCensusRealDeterministic computes the real census twice and requires
// byte-equal markdown and TSV, so map-order nondeterminism cannot return.
func TestStdCensusRealDeterministic(t *testing.T) {
	heavyParallel(t)
	std, err := readCardList(filepath.Join("../../", standardListPath))
	if err != nil {
		t.Fatal(err)
	}
	cardsDir := ratchetCards(t)
	var firstMD, firstTSV string
	for i := 0; i < 2; i++ {
		res, err := computeSetlistCensus(testSetlistDir, cardsDir, std)
		if err != nil {
			t.Fatal(err)
		}
		var md, tsv strings.Builder
		writeStdCensus(&md, res)
		if err := writeStdTSV(&tsv, res); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			firstMD, firstTSV = md.String(), tsv.String()
			continue
		}
		if md.String() != firstMD {
			t.Fatal("real census markdown differs between runs")
		}
		if tsv.String() != firstTSV {
			t.Fatal("real census TSV differs between runs")
		}
	}
}

// TestStdCensusDeterministic runs the census ten times over cards whose
// lowering has several equally valid refusal keys: Winnowing's RepeatEach
// body has ControlledByPlayer$ and RememberChosen$, and Arrester's
// Admonition's ConditionPlayerTurn$ sits beside ConditionDefined$,
// ConditionPresent$ and ConditionPhases$. The report must be byte-identical
// and must name the first key in sorted order, not whichever the map
// happened to yield.
func TestStdCensusDeterministic(t *testing.T) {
	dir := t.TempDir()
	names := []string{"Winnowing", "Arrester's Admonition"}
	for _, list := range []string{"ZZZ.txt", "ALL.txt"} {
		if err := os.WriteFile(filepath.Join(dir, list), []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cardsDir := ratchetCards(t)
	var first string
	for i := 0; i < 10; i++ {
		res, err := computeStdCensus(dir, cardsDir, []string{"ZZZ"})
		if err != nil {
			t.Fatal(err)
		}
		var sb strings.Builder
		writeStdCensus(&sb, res)
		if i == 0 {
			first = sb.String()
			for _, want := range []string{
				`other:RepeatEach body ControlledByPlayer$ "_"`,
				`other:ConditionPlayerTurn$ beside ConditionDefined$`,
			} {
				if !strings.Contains(first, want) {
					t.Errorf("output lacks canonical class %q:\n%s", want, first)
				}
			}
			continue
		}
		if sb.String() != first {
			t.Fatalf("run %d differs from run 0:\n--- run 0 ---\n%s\n--- run %d ---\n%s", i, first, i, sb.String())
		}
	}
}

func TestCompletionGreedy(t *testing.T) {
	// A is the sole class of one card; B has larger fractional coverage but
	// no sole card, so sole-card count wins.
	ref := map[string][]string{
		"a": {"A"},
		"b": {"B", "C"},
		"c": {"B", "D"},
		"d": {"B", "E"},
	}
	order, got := completionGreedy(ref, 2)
	want := []greedyStep{{"A", 1}, {"B", 0}, {"E", 1}}
	if !reflect.DeepEqual(order, want) || got != 2 {
		t.Fatalf("order/got = %v/%d, want %v/2", order, got, want)
	}
	if order, got := completionGreedy(ref, 0); order != nil || got != 0 {
		t.Fatalf("need 0 = %v/%d, want nil/0", order, got)
	}
	// The reference case: X is the old set cover's first pick, but Z and Y
	// complete the two cards.
	ref = map[string][]string{
		"a": {"X"},
		"b": {"X", "Y"},
		"c": {"Y", "Z"},
		"d": {"Z"},
	}
	order, got = completionGreedy(ref, 2)
	want = []greedyStep{{"Z", 1}, {"Y", 1}}
	if !reflect.DeepEqual(order, want) || got != 2 {
		t.Fatalf("reference order/got = %v/%d, want %v/2", order, got, want)
	}
	// At equal (imm, score), the shorter class wins.
	order, got = completionGreedy(map[string][]string{"a": {"AB"}, "b": {"C"}}, 1)
	want = []greedyStep{{"C", 1}}
	if !reflect.DeepEqual(order, want) || got != 1 {
		t.Fatalf("short-class order/got = %v/%d, want %v/1", order, got, want)
	}
}

func TestJointGreedy(t *testing.T) {
	sets := []string{"A", "B"}
	need := map[string]int{"A": 1, "B": 1}
	ref := map[string]map[string][]string{
		"A": {"a": {"X", "Y"}},
		"B": {"b": {"Y"}},
	}
	steps := jointGreedy(sets, ref, need)
	want := []jointStep{{"Y", 1, 1}, {"X", 1, 0}}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("steps = %v, want %v", steps, want)
	}
	// A closes both S1 cards but gains one because S1's rem hits zero; A is
	// removed from S2's s, which lets B close both r and s.
	sets = []string{"S1", "S2"}
	need = map[string]int{"S1": 1, "S2": 1}
	ref = map[string]map[string][]string{
		"S1": {"p": {"A"}, "q": {"A"}},
		"S2": {"r": {"B"}, "s": {"A", "B"}},
	}
	steps = jointGreedy(sets, ref, need)
	want = []jointStep{{"A", 1, 1}, {"B", 1, 0}}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("reference steps = %v, want %v", steps, want)
	}
	// C beats B on imm at the second step.
	ref = map[string]map[string][]string{
		"S1": {"p": {"A"}},
		"S2": {"r": {"A", "B"}, "t": {"C"}},
	}
	steps = jointGreedy(sets, ref, need)
	want = []jointStep{{"A", 1, 1}, {"C", 1, 0}}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("second reference steps = %v, want %v", steps, want)
	}
}

func TestBundles(t *testing.T) {
	// A five-card case: A+B is carried by three cards and covers
	// four (the three plus A alone); A+B+C is carried by one card only.
	cards := []stdCardLabels{
		{[]string{"A", "B"}, []string{"S"}},
		{[]string{"A", "B"}, []string{"S"}},
		{[]string{"A", "B"}, []string{"S"}},
		{[]string{"A"}, []string{"S"}},
		{[]string{"A", "B", "C"}, []string{"S"}},
	}
	rows := computeBundles(cards)
	if len(rows) != 1 || rows[0].bundle != "A + B" || rows[0].cards != 3 || rows[0].covered != 4 {
		t.Fatalf("bundles = %+v, want A + B 3/4 only", rows)
	}
	cards = []stdCardLabels{
		{[]string{"A", "B"}, []string{"S1"}},
		{[]string{"A", "B"}, []string{"S1"}},
		{[]string{"A", "B"}, []string{"S1"}},
		{[]string{"A", "B", "C"}, []string{"S2"}},
		{[]string{"A"}, []string{"S1"}},
		{[]string{"A", "B", "C", "D"}, []string{"S2"}},
	}
	rows = computeBundles(cards)
	if len(rows) != 1 {
		t.Fatalf("bundles = %+v, want one 2-label bundle", rows)
	}
	got := rows[0]
	if got.cards != 3 || got.covered != 4 || got.bundle != "A + B" ||
		!reflect.DeepEqual(got.sets, []stdSetCount{{"S1", 4}}) {
		t.Fatalf("bundle = %+v, want 3/4 A + B S1 4", got)
	}
	// A 3-label bundle is emitted when three cards carry it, and a larger
	// bundle outranks a smaller one.
	cards = []stdCardLabels{
		{[]string{"A", "B", "C"}, []string{"S"}},
		{[]string{"A", "B", "C"}, []string{"S"}},
		{[]string{"A", "B", "C"}, []string{"S"}},
		{[]string{"A", "B"}, []string{"S"}},
		{[]string{"A", "B"}, []string{"S"}},
		{[]string{"A", "B"}, []string{"S"}},
		{[]string{"A", "B"}, []string{"S"}},
	}
	rows = computeBundles(cards)
	if len(rows) != 2 || rows[0].bundle != "A + B" || rows[0].cards != 4 ||
		rows[1].bundle != "A + B + C" || rows[1].cards != 3 {
		t.Fatalf("bundles = %+v, want A + B (4) then A + B + C (3)", rows)
	}
}

// TestStdCensusBundlesDedupeMultiSetCard pins the Standard-union rule: a card
// refused in several Standard lists is one card, not one per list. Sorcerous
// Spyglass carries exactly two refined labels, so three list entries would
// (wrongly) emit a bundle that the union does not have.
func TestStdCensusBundlesDedupeMultiSetCard(t *testing.T) {
	dir := t.TempDir()
	for _, code := range []string{"AAA", "BBB", "CCC"} {
		if err := os.WriteFile(filepath.Join(dir, code+".txt"), []byte("Sorcerous Spyglass\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := computeStdCensus(dir, ratchetCards(t), []string{"AAA", "BBB", "CCC"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.bundles) != 0 {
		t.Fatalf("bundles = %+v, want none: one card in three lists is still one card", res.bundles)
	}
}

func TestCardTSVRowsUnpacksToken(t *testing.T) {
	c := &lCard{Name: "Maker", Refusals: []refusal{{Card: "Maker", Node: "token Foo", Reason: "token refused"}}}
	tok := &lCard{Name: "Foo", Refusals: []refusal{{Card: "Foo", Node: "node", Reason: `DB$ Play: keyword "Ward"`}}}
	rows := cardTSVRows("S", c, map[string]*lCard{"Foo": tok})
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want the wrapper plus one unpacked row", rows)
	}
	if got := rows[0]; got.via != "" || got.class != "other:token refused" {
		t.Fatalf("row 0 = %+v, want the wrapper", got)
	}
	if got := rows[1]; got.via != "Foo" || got.node != "node" || got.class != "kw:Ward" || got.label != "kw:Ward" {
		t.Fatalf("row 1 = %+v, want via Foo, class/label kw:Ward", got)
	}
	// A token with no refusals of its own stays wrapped.
	rows = cardTSVRows("S", c, map[string]*lCard{"Foo": {Name: "Foo"}})
	if len(rows) != 1 || rows[0].class != "other:token refused" || rows[0].via != "" {
		t.Fatalf("rows = %+v, want one wrapped token refusal", rows)
	}
}

func TestStdCensusUnknownSet(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AAA.txt"), []byte("Grizzly Bears\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := computeStdCensus(dir, ratchetCards(t), []string{"ZZZ"})
	if err == nil || !strings.Contains(err.Error(), "ZZZ") {
		t.Fatalf("err = %v, want one naming ZZZ", err)
	}
}

func TestStdSetsFlag(t *testing.T) {
	got, err := stdSetCodes("AAA, BBB,AAA")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"AAA", "BBB"}) {
		t.Fatalf("stdSetCodes = %q, want AAA, BBB", got)
	}
	cards := ratchetCards(t)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join(wd, "..", "..")); err != nil {
		t.Fatal(err)
	}
	got, err = stdSetCodes("")
	if cerr := os.Chdir(wd); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 22 || got[0] != "BIG" || got[21] != "WOE" {
		t.Fatalf("default stdSetCodes = %q, want 22 codes BIG..WOE", got)
	}

	// A non-std list leaves the ranking and joint order unchanged and still
	// gets a per-set row marked non-std.
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "AAA.txt"), []byte("Grizzly Bears\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdRes, err := computeStdCensus(base, cards, []string{"AAA"})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AAA.txt"), []byte("Grizzly Bears\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ZZZ.txt"), []byte("Maha, Its Feathers Night\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := computeStdCensus(dir, cards, []string{"AAA"})
	if err != nil {
		t.Fatal(err)
	}
	var zzz *stdSetRow
	for i := range res.sets {
		if res.sets[i].set == "ZZZ" {
			zzz = &res.sets[i]
		}
	}
	if zzz == nil || zzz.std {
		t.Fatalf("ZZZ row = %+v, want present and non-std", zzz)
	}
	if !reflect.DeepEqual(res.classes, stdRes.classes) {
		t.Fatalf("ranking changed with non-std list:\nwith: %+v\nwithout: %+v", res.classes, stdRes.classes)
	}
	if !reflect.DeepEqual(res.joint, stdRes.joint) {
		t.Fatalf("joint changed with non-std list:\nwith: %+v\nwithout: %+v", res.joint, stdRes.joint)
	}
}

func TestStdCensusMatchesRatchet(t *testing.T) {
	heavyParallel(t)
	std, err := readCardList(filepath.Join("../../", standardListPath))
	if err != nil {
		t.Fatal(err)
	}
	res, err := computeSetlistCensus(testSetlistDir, ratchetCards(t), std)
	if err != nil {
		t.Fatal(err)
	}
	isStd := map[string]bool{}
	for _, code := range std {
		isStd[code] = true
	}
	wantShort := 0
	for _, s := range res.sets {
		pin, ok := acceptancePins[s.set]
		if !ok {
			t.Fatalf("%s has no acceptance pin", s.set)
		}
		if s.accepted != pin[0] || s.listed != pin[1] {
			t.Errorf("%s accepted/listed = %d/%d, pin %d/%d", s.set, s.accepted, s.listed, pin[0], pin[1])
		}
		if isStd[s.set] {
			if n := (9*pin[1]+9)/10 - pin[0]; n > 0 {
				wantShort += n
			}
		}
	}
	if res.stdTotalShort != wantShort {
		t.Errorf("std total short = %d, want %d from the pins", res.stdTotalShort, wantShort)
	}
	if res.allAccepted != acceptancePins["ALL"][0] || res.allListed != acceptancePins["ALL"][1] {
		t.Errorf("ALL accepted/listed = %d/%d, pin %d/%d", res.allAccepted, res.allListed,
			acceptancePins["ALL"][0], acceptancePins["ALL"][1])
	}
}
