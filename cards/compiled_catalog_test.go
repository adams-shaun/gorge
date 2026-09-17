package cards

import (
	"bytes"
	"testing"
)

func compiledCatalogFixture(reverse bool, changedParam bool) (*Registry, *Face, *Face, *Face, *Face) {
	params := make(map[string]string, 3)
	svars := make(map[string]string, 2)
	if reverse {
		params["Zed"] = "last"
		params["SubAbility"] = "Tail"
		params["Alpha"] = "first"
		svars["Zed"] = "Count$RememberedSize"
		svars["Tail"] = "DB$ FutureAPI | Amount$ 2"
	} else {
		params["Alpha"] = "first"
		params["SubAbility"] = "Tail"
		params["Zed"] = "last"
		svars["Tail"] = "DB$ FutureAPI | Amount$ 2"
		svars["Zed"] = "Count$RememberedSize"
	}
	if changedParam {
		params["Alpha"] = "changed"
	}
	tail := &SA{Kind: "DB", API: "FutureAPI", Params: map[string]string{"Amount": "2"}, Line: "DB$ FutureAPI | Amount$ 2"}
	spell := &SA{Kind: "SP", API: "Draw", Params: params, Sub: tail, Line: "SP$ Draw | Alpha$ first | SubAbility$ Tail | Zed$ last"}
	unknownKind := &SA{Kind: "FX", API: "Draw", Params: map[string]string{}, Line: "FX$ Draw"}
	cardFace := &Face{
		Name:           "Alpha",
		ManaCost:       "1 W",
		Types:          []string{"Legendary", "Creature", "Wizard"},
		PT:             "2/3",
		Keywords:       []string{"Flying", "Ward:2"},
		Abilities:      []*SA{spell, unknownKind},
		Triggers:       []Trigger{{Mode: "FutureTrigger", Params: map[string]string{"Mode": "FutureTrigger", "Phase": "Upkeep"}, Effect: tail}},
		Statics:        []Static{{Mode: "FutureStatic", Params: map[string]string{"Mode": "FutureStatic"}}},
		Repls:          []Repl{{Event: "FutureReplacement", Params: map[string]string{"Event": "FutureReplacement"}, With: tail}},
		SVars:          svars,
		power:          2,
		toughness:      3,
		cmc:            2,
		colourIdentity: ColourWhite,
	}
	secondFace := &Face{Name: "Beta", Types: []string{"Basic", "Land", "Forest"}}
	aTokenFace := &Face{Name: "A Token", Types: []string{"Token", "Creature"}, PT: "1/1", power: 1, toughness: 1}
	zTokenFace := &Face{Name: "Z Token", Types: []string{"Token", "Artifact"}}
	r := NewRegistry()
	r.Add(&Card{Path: "alpha.txt", Faces: []*Face{cardFace}})
	r.Add(&Card{Path: "beta.txt", Faces: []*Face{secondFace}})
	if reverse {
		r.Tokens["z_token"] = &Card{Path: "z_token.txt", Faces: []*Face{zTokenFace}}
		r.Tokens["a_token"] = &Card{Path: "a_token.txt", Faces: []*Face{aTokenFace}}
	} else {
		r.Tokens["a_token"] = &Card{Path: "a_token.txt", Faces: []*Face{aTokenFace}}
		r.Tokens["z_token"] = &Card{Path: "z_token.txt", Faces: []*Face{zTokenFace}}
	}
	return r, cardFace, secondFace, aTokenFace, zTokenFace
}

func TestCompiledCatalogDeterministicLayout(t *testing.T) {
	a, af, ab, aa, az := compiledCatalogFixture(false, false)
	b, bf, bb, ba, bz := compiledCatalogFixture(true, false)
	if err := a.CompileMetadata(); err != nil {
		t.Fatalf("compile first catalog: %v", err)
	}
	if err := b.CompileMetadata(); err != nil {
		t.Fatalf("compile reordered catalog: %v", err)
	}
	ac, bc := a.Catalog(), b.Catalog()
	if ac == nil || bc == nil {
		t.Fatal("compiled registry has no catalog")
	}
	if !bytes.Equal(ac.CanonicalBytes(), bc.CanonicalBytes()) {
		t.Fatal("equivalent registries produced different canonical bytes")
	}
	if ac.Identity != bc.Identity {
		t.Fatalf("equivalent identities differ: %x vs %x", ac.Identity.CorpusHash, bc.Identity.CorpusHash)
	}
	for _, tc := range []struct {
		name string
		a, b *Face
		want FaceID
	}{
		{"first card", af, bf, 1},
		{"second card", ab, bb, 2},
		{"lexically first token", aa, ba, 3},
		{"lexically second token", az, bz, 4},
	} {
		if got := tc.a.CompiledID(); got != tc.want {
			t.Errorf("%s first ID = %d, want %d", tc.name, got, tc.want)
		}
		if got := tc.b.CompiledID(); got != tc.want {
			t.Errorf("%s reordered ID = %d, want %d", tc.name, got, tc.want)
		}
	}
	if ac.Identity.Schema != CompiledCatalogSchema {
		t.Fatalf("schema = %d, want %d", ac.Identity.Schema, CompiledCatalogSchema)
	}
	assertCatalogSpansInRange(t, ac)
}

func TestCompiledCatalogBindsRecursiveAbilitiesAndUnknownText(t *testing.T) {
	r, face, _, _, _ := compiledCatalogFixture(false, false)
	if err := r.CompileMetadata(); err != nil {
		t.Fatal(err)
	}
	c := r.Catalog()
	fr := c.Faces[face.CompiledID()-1]
	if fr.Abilities.Count != 2 {
		t.Fatalf("face ability count = %d, want 2", fr.Abilities.Count)
	}
	rootID := c.FaceAbilities[fr.Abilities.Start]
	root := c.Abilities[rootID-1]
	if root.Kind != SAKindSpell || root.API != APIDraw || root.Sub == 0 {
		t.Fatalf("compiled root = %+v", root)
	}
	child := c.Abilities[root.Sub-1]
	if child.Kind != SAKindDrawback || child.API != APIUnknown {
		t.Fatalf("compiled child = %+v", child)
	}
	if got, ok := c.String(child.UnknownAPI); !ok || got != "FutureAPI" {
		t.Fatalf("unknown API text = %q, %v", got, ok)
	}
	unknownID := c.FaceAbilities[fr.Abilities.Start+1]
	unknown := c.Abilities[unknownID-1]
	if unknown.Kind != SAKindUnknown || unknown.UnknownKind == 0 {
		t.Fatalf("unknown kind row = %+v", unknown)
	}
	if got, _ := c.String(unknown.UnknownKind); got != "FX" {
		t.Fatalf("unknown kind text = %q", got)
	}
	tr := c.Triggers[fr.Triggers.Start]
	if tr.Mode != TriggerModeUnknown || tr.UnknownMode == 0 || tr.Effect != root.Sub {
		t.Fatalf("unknown trigger row = %+v", tr)
	}
	if interest, ok := face.CompiledTriggerInterests(); !ok || interest != TriggerInterestAny {
		t.Fatalf("compiled interests = %x, %v; want catch-all", interest, ok)
	}
	if face.Abilities[0].CompiledKind() != SAKindSpell || face.Abilities[0].CompiledAPI() != APIDraw {
		t.Fatal("root ability binding missing")
	}
	if face.Abilities[0].Sub.CompiledKind() != SAKindDrawback || face.Abilities[0].Sub.CompiledAPI() != APIUnknown {
		t.Fatal("recursive ability binding missing")
	}
}

func TestCompiledCatalogIdentityChangesWithParameter(t *testing.T) {
	a, _, _, _, _ := compiledCatalogFixture(false, false)
	b, _, _, _, _ := compiledCatalogFixture(false, true)
	if err := a.CompileMetadata(); err != nil {
		t.Fatal(err)
	}
	if err := b.CompileMetadata(); err != nil {
		t.Fatal(err)
	}
	if a.Catalog().Identity.CorpusHash == b.Catalog().Identity.CorpusHash {
		t.Fatal("changing one parameter value did not change the corpus hash")
	}
}

func TestWholeCorpusCompilesMetadata(t *testing.T) {
	r := compiledCorpus(t)
	if err := r.CompileMetadata(); err != nil {
		t.Fatalf("CompileMetadata: %v", err)
	}
	c := r.Catalog()
	if c == nil || len(c.Faces) < 35_000 || len(c.Abilities) < 50_000 {
		t.Fatalf("catalog rows: faces=%d abilities=%d", len(c.Faces), len(c.Abilities))
	}
	t.Logf("catalog rows: faces=%d abilities=%d triggers=%d statics=%d replacements=%d strings=%d bytes=%d",
		len(c.Faces), len(c.Abilities), len(c.Triggers), len(c.Statics), len(c.Replacements), len(c.Strings), len(c.CanonicalBytes()))
}

func assertCatalogSpansInRange(t *testing.T, c *CompiledCatalog) {
	t.Helper()
	check := func(name string, span Span, n int) {
		t.Helper()
		if uint64(span.Start)+uint64(span.Count) > uint64(n) {
			t.Errorf("%s span %+v exceeds length %d", name, span, n)
		}
	}
	for i, f := range c.Faces {
		check("face types", f.Types, len(c.TypeTokens))
		check("face keywords", f.Keywords, len(c.Keywords))
		check("face abilities", f.Abilities, len(c.FaceAbilities))
		check("face mana abilities", f.ManaAbilities, len(c.ManaAbilityIDs))
		check("face triggers", f.Triggers, len(c.Triggers))
		check("face statics", f.Statics, len(c.Statics))
		check("face replacements", f.Replacements, len(c.Replacements))
		check("face SVars", f.SVars, len(c.SVars))
		if f.SpellAbility != 0 && int(f.SpellAbility) > len(c.Abilities) {
			t.Errorf("face %d spell ability %d exceeds %d abilities", i, f.SpellAbility, len(c.Abilities))
		}
	}
	for _, a := range c.Abilities {
		check("ability params", a.Params, len(c.Params))
		if a.Sub != 0 && int(a.Sub) > len(c.Abilities) {
			t.Errorf("ability sub %d exceeds %d abilities", a.Sub, len(c.Abilities))
		}
	}
	for _, tr := range c.Triggers {
		check("trigger params", tr.Params, len(c.Params))
	}
	for _, st := range c.Statics {
		check("static params", st.Params, len(c.Params))
	}
	for _, repl := range c.Replacements {
		check("replacement params", repl.Params, len(c.Params))
	}
}
