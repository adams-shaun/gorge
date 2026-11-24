package mtgish

import (
	"bufio"
	"errors"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func TestReadLock(t *testing.T) {
	root, err := ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	l, err := ReadLock(filepath.Join(root, "engine", "cardc", "MTGISH.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if l.Repo == "" || l.Upstream == "" || len(l.Ref) != 40 || l.Base == "" || len(l.Files) == 0 {
		t.Errorf("lock incomplete: %+v", l)
	}
	for _, f := range l.Files {
		if len(f.Sum) != 64 || f.Path == "" {
			t.Errorf("bad file line %+v", f)
		}
	}
	const fixture = "# c\nrepo r\nupstream u\nbase b\nref f\nsha256 aa data/x.json\npatch abc123 7\n\npatch def456 9\n"
	l, err = parseLock(bufio.NewScanner(strings.NewReader(fixture)), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	want := []LockPatch{{"abc123", 7}, {"def456", 9}}
	if !reflect.DeepEqual(l.Patches, want) || l.Repo != "r" || l.Upstream != "u" || l.Base != "b" || l.Ref != "f" ||
		!reflect.DeepEqual(l.Files, []LockFile{{"aa", "data/x.json"}}) {
		t.Errorf("parsed %+v", l)
	}
	if _, err := parseLock(bufio.NewScanner(strings.NewReader("patch x y\n")), "bad"); err == nil {
		t.Error("a non-numeric patch issue parsed")
	}
}

func TestTaggedHelpers(t *testing.T) {
	cards, err := ReadAll(strings.NewReader(`{"_Thing":"Tup","args":[1,{"_Inner":"Unit"}]}` + "\n" +
		`{"_Thing":"Strukt","Amount":3,"Who":{"_Who":"You"}}` + "\n" +
		`{"_Thing":"Unit"}` + "\n"))
	if err != nil || len(cards) != 0 {
		t.Fatalf("ReadAll of non-card lines: %v, %d cards", err, len(cards))
	}
	tup := map[string]any{"_Thing": "Tup", "args": []any{1.0, map[string]any{"_Inner": "Unit"}}}
	strukt := map[string]any{"_Thing": "Strukt", "Amount": 3.0, "Who": map[string]any{"_Who": "You"}}
	unit := map[string]any{"_Thing": "Unit"}
	if qualified(tup) != "Thing.Tup" || variantOf(strukt) != "Strukt" || enumOf(unit) != "Thing" {
		t.Error("qualified/variantOf/enumOf")
	}
	if len(argsOf(tup)) != 2 || variantOf(argN(tup, 1)) != "Unit" || argN(tup, 2) != nil {
		t.Error("tuple args")
	}
	if n, ok := intOf(field(strukt, "Amount")); !ok || n != 3 || variantOf(field(strukt, "Who")) != "You" {
		t.Error("struct fields")
	}
	if argsOf(unit) != nil || rawArgs(unit) != nil || qualified("x") != "?" || list(unit) != nil {
		t.Error("unit and non-node")
	}
	var got []string
	walk([]any{tup, strukt}, func(_ node, e, v string) bool {
		got = append(got, e+"."+v)
		return true
	})
	if want := []string{"_Thing.Tup", "_Inner.Unit", "_Thing.Strukt", "_Who.You"}; !reflect.DeepEqual(got, want) {
		t.Errorf("walk order %v, want %v", got, want)
	}
}

func TestLoweringRefusesUnregistered(t *testing.T) {
	c := &Ctx{Card: "Zz Test"}
	_, err := c.lower(map[string]any{"_Action": "NoSuchVariant"})
	var re *RefusalError
	if !errors.As(err, &re) || re.Key != "Action.NoSuchVariant" || re.Path != "Action.NoSuchVariant" {
		t.Fatalf("got %v", err)
	}
	if c.Src() != "" {
		t.Errorf("path not popped: %q", c.Src())
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	const key = "TestOnly.Dup"
	register(key, func(*Ctx, node) (any, error) { return nil, nil })
	t.Cleanup(func() { delete(registry, key) })
	defer func() {
		if recover() == nil {
			t.Error("duplicate register did not panic")
		}
	}()
	register(key, func(*Ctx, node) (any, error) { return nil, nil })
}

func TestTypedHelpersAndStubs(t *testing.T) {
	withStubs(t, map[string]lowerFunc{
		"TestOnly.Ref": func(c *Ctx, n node) (any, error) { return &ir.Ref{}, nil },
		"TestOnly.Val": func(c *Ctx, n node) (any, error) { return &ir.Ref{}, nil },
	})
	c := &Ctx{}
	if _, err := c.Ref(map[string]any{"_TestOnly": "Ref"}); err != nil {
		t.Error(err)
	}
	_, err := c.Value(map[string]any{"_TestOnly": "Val"})
	if err == nil || !strings.Contains(err.Error(), "TestOnly.Val lowers to *ir.Ref, want *ir.Value") {
		t.Errorf("wrong-type error: %v", err)
	}
}

func TestTargetSlotResolution(t *testing.T) {
	c := &Ctx{}
	for _, s := range []string{"TargetPermanent", "TargetPlayer", "TargetPermanent", "TargetPermanents"} {
		c.DeclareTarget(s)
	}
	cases := []struct {
		ref  string
		slot uint8
		ok   bool
	}{
		{"Ref_TargetPermanent", 2, true},
		{"Ref_TargetPermanent1", 0, true},
		{"Ref_TargetPermanent2", 2, true},
		{"Ref_TargetPermanents", 3, true},
		{"Ref_TargetSpell", 0, false},
		{"Ref_TargetPermanents_1", 0, false},
		{"Ref_TargetPermanent3", 0, false},
	}
	for _, tc := range cases {
		if slot, ok := c.TargetSlot(tc.ref); slot != tc.slot || ok != tc.ok {
			t.Errorf("%s = %d,%v want %d,%v", tc.ref, slot, ok, tc.slot, tc.ok)
		}
	}
	c.ResetTargets()
	if _, ok := c.TargetSlot("Ref_TargetPermanent"); ok {
		t.Error("slot after ResetTargets")
	}
	if got := c.DeclareTarget("TargetPlayer"); got != 0 {
		t.Errorf("slot after reset = %d, want 0", got)
	}
}

func TestIndexPrefersFrontFace(t *testing.T) {
	cards, err := ReadAll(strings.NewReader(
		`{"_OracleCard":"Preparer","Name":"Zz Preparer","Prepared":{"Name":"Zz Bolt"}}` + "\n" +
			`{"_OracleCard":"Card","Name":"Zz Bolt"}` + "\n" +
			`{"_OracleCard":"Split","Cards":[{"Name":"Zz Left"},{"Name":"Zz Right"}]}` + "\n" +
			`{"_OracleCard":"Card","Name":"Zz Right"}` + "\n"))
	if err != nil || len(cards) != 4 {
		t.Fatalf("%v, %d cards", err, len(cards))
	}
	ix := NewIndex(cards)
	if c, _ := ix.Lookup("Zz Bolt"); c != cards[1] {
		t.Error("Zz Bolt did not resolve to the Card")
	}
	if c, _ := ix.Lookup("Zz Preparer"); c != cards[0] {
		t.Error("Zz Preparer did not resolve to the Preparer")
	}
	if c, _ := ix.Lookup("Zz Left // Zz Right"); c != cards[2] {
		t.Error("join did not resolve to the Split")
	}
	if c, _ := ix.Lookup("Zz Left"); c != cards[2] {
		t.Error("front face of the Split")
	}
	if c, _ := ix.Lookup("Zz Right"); c != cards[3] {
		t.Error("a front name beats another card's back face")
	}
	if _, ok := ix.Lookup("Nope"); ok {
		t.Error("found a missing name")
	}
	if !sort.StringsAreSorted(ix.Keys()) || len(ix.Keys()) != 6 {
		t.Errorf("keys %v", ix.Keys())
	}
}

func TestSliceNamesResolve(t *testing.T) {
	cards := resolveSlice(t, "fdn", "blb", "decks", "legacy")
	if len(cards) < 500 {
		t.Errorf("only %d cards resolved", len(cards))
	}
}

func TestSchemaCoversData(t *testing.T) {
	all, _ := corpus(t)
	s := corpusSchema(t)
	var missing []string
	for _, vc := range Census(all) {
		if !s.Has(vc.Key) {
			missing = append(missing, vc.Key)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%d data variants missing from the schema: %v", len(missing), missing)
	}
}

func TestFaceKeysMatchSchema(t *testing.T) {
	s := corpusSchema(t)
	got := append([]string(nil), faceKeys...)
	sort.Strings(got)
	if want := s.FaceKeys(); !reflect.DeepEqual(got, want) {
		t.Errorf("faceKeys %v, schema %v", got, want)
	}
}

func TestCoverageHelperDetectsStale(t *testing.T) {
	census := []VariantCount{
		{Key: "Action.Draw", Cards: 3},
		{Key: "Action.Gain", Cards: 2},
		{Key: "Action.Scry", Cards: 1},
		{Key: "Cond.Other", Cards: 9},
	}
	reg := map[string]bool{"Action.Draw": true, "Action.Scry": true}
	registered := func(k string) bool { return reg[k] }
	deferred := map[string]string{"Action.Scry": "stale", "Action.Gone": "gone"}
	got := familyCoverageProblems(census, []string{"Action"}, deferred, registered)
	if len(got) != 3 ||
		!strings.HasPrefix(got[0], "Action.Gain:") || !strings.HasPrefix(got[1], "Action.Gone:") || !strings.HasPrefix(got[2], "Action.Scry:") {
		t.Errorf("problems: %q", got)
	}
	clean := familyCoverageProblems(census, []string{"Action"}, map[string]string{"Action.Gain": "later"}, registered)
	if len(clean) != 0 {
		t.Errorf("clean input reported %q", clean)
	}
}

func TestRegisteredKeysInSchema(t *testing.T) {
	s := corpusSchema(t)
	for _, k := range RegisteredKeys() {
		if !s.Has(k) {
			t.Errorf("registered key %s is not in the schema (misspelt or renamed)", k)
		}
	}
}
