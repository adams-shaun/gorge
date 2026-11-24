package gorgebe

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cardc/mtgish"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func i32(n int32) *int32 { return &n }

func kw(name string) *ir.Ability {
	return &ir.Ability{Kind: ir.AbKeyword, Keyword: &ir.Keyword{Name: name}}
}

func normalCard() *ir.Card {
	return &ir.Card{Name: "Zz Test", Layout: ir.LayoutNormal, Faces: []*ir.Face{{
		Name: "Zz Test",
		ManaCost: []ir.ManaSym{
			{Kind: ir.ManaGeneric, N: 2},
			{Kind: ir.ManaHybrid, Colors: ir.ColorW | ir.ColorU},
			{Kind: ir.ManaPhyrexian, Colors: ir.ColorG},
			{Kind: ir.ManaX},
		},
		HasManaCost:    true,
		ColorIndicator: ir.ColorU,
		Supertypes:     []string{"Legendary"},
		Types:          []string{"Artifact", "Creature"},
		Subtypes:       []string{"TimeLord", "Elf"},
		Power:          &ir.PT{Star: true, N: 1},
		Toughness:      &ir.PT{Star: true},
		Abilities:      []*ir.Ability{kw("First Strike"), kw("Flying")},
	}}}
}

func TestPrintSkeleton(t *testing.T) {
	got, err := Print(normalCard(), []string{"Oracle text."})
	if err != nil {
		t.Fatal(err)
	}
	want := "Name:Zz Test\nManaCost:2 WU GP X\nTypes:Legendary Artifact Creature Time Lord Elf\nPT:*+1/*\nColors:blue\n" +
		"K:First Strike\nK:Flying\nOracle:Oracle text.\n"
	if string(got) != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	pc, err := PrintCharacteristics(normalCard(), []string{"Oracle text."})
	if err != nil || strings.Contains(string(pc), "K:") {
		t.Errorf("PrintCharacteristics: %q, %v", pc, err)
	}

	l, d := int32(3), int32(4)
	tf := &ir.Card{Name: "Zz Front", Layout: ir.LayoutTransform, Faces: []*ir.Face{
		{Name: "Zz Front", Types: []string{"Creature"}, Subtypes: []string{"Human"}, Power: &ir.PT{N: 1}, Toughness: &ir.PT{N: -0 + 2}, HasManaCost: true, ManaCost: []ir.ManaSym{{Kind: ir.ManaTwoHybrid, Colors: ir.ColorR}}},
		{Name: "Zz Back", Types: []string{"Planeswalker"}, Loyalty: &l, Defense: &d},
	}}
	got, err = Print(tf, []string{"", "Back."})
	if err != nil {
		t.Fatal(err)
	}
	want = "AlternateMode:DoubleFaced\nName:Zz Front\nManaCost:2R\nTypes:Creature Human\nPT:1/2\nALTERNATE\n" +
		"Name:Zz Back\nManaCost:no cost\nTypes:Planeswalker\nLoyalty:3\nDefense:4\nOracle:Back.\n"
	if string(got) != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}

	zero := normalCard()
	zero.Faces[0].ManaCost = nil
	zero.Faces[0].Power.Star, zero.Faces[0].Power.N = false, 0
	zero.Faces[0].Toughness = &ir.PT{Star: true, N: -2}
	got, _ = Print(zero, []string{""})
	if !strings.Contains(string(got), "ManaCost:0\n") || !strings.Contains(string(got), "PT:0/*-2\n") {
		t.Errorf("zero cost / negative star: %s", got)
	}

	ab := normalCard()
	ab.Faces[0].Abilities = []*ir.Ability{
		{Kind: ir.AbKeyword, Keyword: &ir.Keyword{Name: "Ward", Cost: abGenericCost(1)}},
		{Kind: ir.AbTriggered, Trigger: abTrig(ir.EvEnters, abSelf(), nil), Body: abDraw()},
	}
	got, err = Print(ab, []string{""})
	if err != nil || !strings.Contains(string(got), "K:Ward:1\n") || !strings.Contains(string(got), "T:Mode$ ChangesZone") ||
		!strings.Contains(string(got), "SVar:TrigDraw:DB$ Draw") {
		t.Errorf("ward and trigger: %v\n%s", err, got)
	}

	un := func(name string, c *ir.Card, oracle []string, what string) {
		t.Helper()
		_, err := Print(c, oracle)
		var u *Unprintable
		if !errors.As(err, &u) || u.What != what {
			t.Errorf("%s: err %v, want unprintable %s", name, err, what)
		}
	}
	split := normalCard()
	split.Layout = ir.LayoutSplit
	un("split", split, []string{""}, "layout:split")
	trig := normalCard()
	trig.Faces[0].Abilities = []*ir.Ability{{Kind: ir.AbTriggered, Trigger: &ir.Trigger{Event: ir.EvUntaps}, Body: abDraw()}}
	un("trigger", trig, []string{""}, "event:untaps")
	ward := normalCard()
	ward.Faces[0].Abilities = []*ir.Ability{{Kind: ir.AbKeyword, Keyword: &ir.Keyword{Name: "Ward", Cost: &ir.Cost{}}}}
	un("ward", ward, []string{""}, "cost:empty")
	ref := normalCard()
	ref.Faces[0].Refusals = []ir.Refusal{{Key: "Action.Foo", Path: "p"}}
	un("refusal", ref, []string{""}, "refusal:Action.Foo")
	un("faces", normalCard(), nil, "faces")
}

func TestRespell(t *testing.T) {
	for in, want := range map[string]string{
		"AssemblyWorker": "Assembly-Worker", "Ctan": "C'tan", "MindlessOne": "Mindless-One",
		"Shiar": "Shi'ar", "PowerPlant": "Power-Plant", "Urzas": "Urza's", "TimeLord": "Time Lord",
		"Elf": "Elf",
	} {
		if got := Respell(in); got != want {
			t.Errorf("Respell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseScriptRoundTrip(t *testing.T) {
	c := normalCard()
	c.Faces[0].Types = []string{"Creature"}
	c.Faces[0].Supertypes, c.Faces[0].Subtypes = nil, []string{"Elf"}
	text, err := Print(c, []string{""})
	if err != nil {
		t.Fatal(err)
	}
	cc, diags := ParseScript("cardc/zz.txt", text)
	if len(diags) != 0 {
		t.Fatalf("diags: %v", diags)
	}
	f := cc.Faces[0]
	if f.Name != "Zz Test" || f.ManaCost != "2 WU GP X" || !reflect.DeepEqual(f.Types, []string{"Creature", "Elf"}) ||
		f.PT != "*+1/*" || f.Colors != "blue" || !reflect.DeepEqual(f.Keywords, []string{"First Strike", "Flying"}) {
		t.Errorf("face %+v", f)
	}
}

func TestReadFallbacks(t *testing.T) {
	m, err := ReadFallbacks(filepath.Join(t.TempDir(), "absent"))
	if err != nil || len(m) != 0 {
		t.Errorf("missing dir: %v, %v", m, err)
	}
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("lightning_bolt.txt", "name Lightning Bolt\nclass cardc-gap\nissue 12\n")
	m, err = ReadFallbacks(dir)
	if err != nil || m["Lightning Bolt"] != (Fallback{"Lightning Bolt", "cardc-gap", 12}) {
		t.Errorf("good file: %v, %v", m, err)
	}
	write("bad_class.txt", "name Bad Class\nclass nope\nissue 0\n")
	if _, err := ReadFallbacks(dir); err == nil || !strings.Contains(err.Error(), "bad_class.txt") {
		t.Errorf("bad class: %v", err)
	}
	write("bad_class.txt", "name Bad Class\nclass equivalent\nissue 0\n")
	write("wrong_stem.txt", "name Other Name\nclass equivalent\nissue 0\n")
	if _, err := ReadFallbacks(dir); err == nil || !strings.Contains(err.Error(), "wrong_stem.txt") {
		t.Errorf("stem mismatch: %v", err)
	}
	if got := Stem("Jace, the Mind Sculptor"); got != "jace_the_mind_sculptor" {
		t.Errorf("Stem = %q", got)
	}
}

func bears() *ir.Card {
	return &ir.Card{Name: "Grizzly Bears", Layout: ir.LayoutNormal, Faces: []*ir.Face{{
		Name: "Grizzly Bears", HasManaCost: true,
		ManaCost: []ir.ManaSym{{Kind: ir.ManaGeneric, N: 1}, {Kind: ir.ManaColored, Colors: ir.ColorG}},
		Types:    []string{"Creature"}, Subtypes: []string{"Bear"},
		Power: &ir.PT{N: 2}, Toughness: &ir.PT{N: 2},
	}}}
}

func elves() *ir.Card {
	return &ir.Card{Name: "Llanowar Elves", Layout: ir.LayoutNormal, Faces: []*ir.Face{{
		Name: "Llanowar Elves", HasManaCost: true,
		ManaCost: []ir.ManaSym{{Kind: ir.ManaColored, Colors: ir.ColorG}},
		Types:    []string{"Creature"}, Subtypes: []string{"Elf", "Druid"},
		Power: &ir.PT{N: 1}, Toughness: &ir.PT{N: 1},
		Abilities: []*ir.Ability{{Kind: ir.AbTriggered}},
	}}}
}

func TestBuildOrigins(t *testing.T) {
	forge := testutil.CorpusRegistry(t)
	gb, _ := forge.Lookup("Grizzly Bears")
	le, _ := forge.Lookup("Llanowar Elves")
	if gb == nil || le == nil {
		t.Fatal("corpus lacks the test cards")
	}
	t.Run("served", func(t *testing.T) {
		r, err := Build(forge, []*ir.Card{bears(), elves(), bears()}, nil)
		if err != nil {
			t.Fatal(err)
		}
		es := r.Entries()
		if len(es) != 2 || es[0].Origin != OriginCardc || es[0].Reason != "" || es[0].Card == gb || es[0].Forge != gb {
			t.Fatalf("bears entry %+v", es[0])
		}
		if es[1].Origin != OriginForge || es[1].Reason != "unprintable:event:none" || es[1].Card != le {
			t.Errorf("elves entry %+v", es[1])
		}
		if r.Served() != 1 {
			t.Errorf("Served = %d", r.Served())
		}
		deck := r.Deck([]*cards.Card{gb, le, gb, gb, gb})
		if len(deck) != 5 || deck[0] != es[0].Card || deck[1] != le || deck[2] != es[0].Card || deck[4] != es[0].Card {
			t.Errorf("Deck did not swap exactly the served card: %v", deck)
		}
	})
	t.Run("fallback", func(t *testing.T) {
		fb := map[string]Fallback{"Grizzly Bears": {Name: "Grizzly Bears", Class: "cardc-gap"}}
		r, err := Build(forge, []*ir.Card{bears()}, fb)
		if err != nil {
			t.Fatal(err)
		}
		e := r.Entries()[0]
		if e.Origin != OriginForge || e.Reason != "fallback:cardc-gap" || e.Card != gb {
			t.Errorf("entry %+v", e)
		}
	})
	t.Run("refused", func(t *testing.T) {
		c := bears()
		c.Faces[0].Refusals = []ir.Refusal{{Key: "Action.Foo"}}
		r, err := Build(forge, []*ir.Card{c}, nil)
		if err != nil || r.Entries()[0].Reason != "refused:Action.Foo" {
			t.Errorf("%v %+v", err, r.Entries()[0])
		}
	})
	t.Run("unknown", func(t *testing.T) {
		if _, err := Build(forge, []*ir.Card{{Name: "Zz No Such Card"}}, nil); err == nil ||
			!strings.Contains(err.Error(), `no Forge card "Zz No Such Card"`) {
			t.Errorf("err %v", err)
		}
	})
}

// sliceCards compiles the fdn and legacy slices from the mtgish data, skipping
// (ENV) when it is absent.
func sliceCards(t *testing.T) []*ir.Card {
	t.Helper()
	p := mtgish.DataPath()
	if p == "" {
		t.Skip(mtgish.SkipMessage)
	}
	if _, err := os.Stat(p); err != nil {
		t.Skip(mtgish.SkipMessage)
	}
	root, err := mtgish.ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	all, err := mtgish.LoadAll(p)
	if err != nil {
		t.Fatal(err)
	}
	ix := mtgish.NewIndex(all)
	var out []*ir.Card
	seen := map[string]bool{}
	for _, s := range []string{"fdn", "legacy"} {
		names, err := mtgish.SliceNames(root, s)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range names {
			c, ok := ix.Lookup(n)
			if !ok {
				t.Fatalf("slice %s: %q does not resolve", s, n)
			}
			ic := mtgish.Compile(c)
			if !seen[ic.Name] {
				seen[ic.Name] = true
				out = append(out, ic)
			}
		}
	}
	return out
}

func oracleOf(fc *cards.Card) []string {
	o := make([]string, len(fc.Faces))
	for i, f := range fc.Faces {
		o[i] = f.Oracle
	}
	return o
}

func TestSkeletonMatchesForge(t *testing.T) {
	forge := testutil.CorpusRegistry(t)
	slice := sliceCards(t)
	faces := 0
	for _, c := range slice {
		fc, ok := forge.Lookup(c.Name)
		if !ok {
			t.Errorf("%s: no Forge card", c.Name)
			continue
		}
		text, err := PrintCharacteristics(c, oracleOf(fc))
		if err != nil {
			t.Errorf("%s: %v", c.Name, err)
			continue
		}
		pc, diags := ParseScript("cardc/x.txt", text)
		if len(diags) != 0 {
			t.Errorf("%s: diags %v\n%s", c.Name, diags, text)
			continue
		}
		if len(pc.Faces) != len(fc.Faces) {
			t.Errorf("%s: %d faces, Forge %d", c.Name, len(pc.Faces), len(fc.Faces))
			continue
		}
		for i, f := range pc.Faces {
			g := fc.Faces[i]
			faces++
			if f.Name != g.Name || f.ManaCost != g.ManaCost || !reflect.DeepEqual(f.Types, g.Types) || f.PT != g.PT ||
				f.Loyalty != g.Loyalty || f.Defense != g.Defense || f.Colors != g.Colors || f.Oracle != g.Oracle {
				t.Errorf("%s face %d:\n cardc %q %q %v %q %q %q %q\n forge %q %q %v %q %q %q %q", c.Name, i,
					f.Name, f.ManaCost, f.Types, f.PT, f.Loyalty, f.Defense, f.Colors,
					g.Name, g.ManaCost, g.Types, g.PT, g.Loyalty, g.Defense, g.Colors)
			}
		}
	}
	t.Logf("%d cards, %d faces agree", len(slice), faces)
}

// chainOf is the APIs down sub's Sub chain joined by ">", Cleanup dropped.
func chainOf(sub *cards.SA) string {
	var apis []string
	for ; sub != nil; sub = sub.Sub {
		if sub.API != "Cleanup" {
			apis = append(apis, sub.API)
		}
	}
	return strings.Join(apis, ">")
}

// faceShape is f's ability shape: one string per keyword, ability, trigger,
// static and replacement, sorted.
func faceShape(f *cards.Face) []string {
	var out []string
	for _, k := range f.Keywords {
		out = append(out, "K:"+k)
	}
	for _, a := range f.Abilities {
		out = append(out, "A:"+a.Kind+":"+chainOf(a))
	}
	for _, tr := range f.Triggers {
		out = append(out, "T:"+tr.Mode+":"+chainOf(tr.Effect))
	}
	for _, st := range f.Statics {
		out = append(out, "S:"+st.Mode)
	}
	for _, r := range f.Repls {
		out = append(out, "R:"+r.Event+":"+chainOf(r.With))
	}
	sort.Strings(out)
	return out
}

// servedFloor is the number of cards of the fdn and legacy slices cardc serves.
const servedFloor = 255

func TestServedCardsMatchForge(t *testing.T) {
	forge := testutil.CorpusRegistry(t)
	slice := sliceCards(t)
	r, err := Build(forge, slice, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Served() < servedFloor {
		t.Errorf("served %d, want at least %d", r.Served(), servedFloor)
	}
	for _, e := range r.Entries() {
		if e.Origin != OriginCardc {
			continue
		}
		if len(e.Card.Faces) != len(e.Forge.Faces) {
			t.Errorf("%s: face count", e.Name)
			continue
		}
		for i, f := range e.Card.Faces {
			got, want := faceShape(f), faceShape(e.Forge.Faces[i])
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s face %d: shape differs:\n cardc %q\n forge %q", e.Name, i, got, want)
			}
		}
	}
	t.Logf("%d of %d served by cardc", r.Served(), len(r.Entries()))
}

// TestServedAPIsSupported: every API a served face's chains name is one the
// engine implements.
func TestServedAPIsSupported(t *testing.T) {
	forge := testutil.CorpusRegistry(t)
	r, err := Build(forge, sliceCards(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	sup := effects.Supported()
	check := func(name string, sa *cards.SA) {
		for ; sa != nil; sa = sa.Sub {
			if !sup["api:"+sa.API] {
				t.Errorf("%s: API %q is not implemented", name, sa.API)
			}
		}
	}
	for _, e := range r.Entries() {
		if e.Origin != OriginCardc {
			continue
		}
		for _, f := range e.Card.Faces {
			for _, a := range f.Abilities {
				check(e.Name, a)
			}
			for _, tr := range f.Triggers {
				check(e.Name, tr.Effect)
			}
			for _, rp := range f.Repls {
				check(e.Name, rp.With)
			}
		}
	}
}
