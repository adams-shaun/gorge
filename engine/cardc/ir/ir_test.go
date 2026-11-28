package ir

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// enumInfo describes one enum for the table-driven tests. Adding an enum means
// adding a row here.
type enumInfo struct {
	typ   reflect.Type
	count int
	names []string
}

var enums = []enumInfo{
	{reflect.TypeFor[Layout](), int(layoutCount), layoutNames[:]},
	{reflect.TypeFor[ManaKind](), int(manaKindCount), manaKindNames[:]},
	{reflect.TypeFor[Zone](), int(zoneCount), zoneNames[:]},
	{reflect.TypeFor[CmpOp](), int(cmpOpCount), cmpOpNames[:]},
	{reflect.TypeFor[RefKind](), int(refKindCount), refKindNames[:]},
	{reflect.TypeFor[ReceiptKind](), int(receiptKindCount), receiptKindNames[:]},
	{reflect.TypeFor[Subject](), int(subjectCount), subjectNames[:]},
	{reflect.TypeFor[FilterOp](), int(filterOpCount), filterOpNames[:]},
	{reflect.TypeFor[Pred](), int(predCount), predNames[:]},
	{reflect.TypeFor[ValueKind](), int(valueKindCount), valueKindNames[:]},
	{reflect.TypeFor[CondKind](), int(condKindCount), condKindNames[:]},
	{reflect.TypeFor[AbilityKind](), int(abilityKindCount), abilityKindNames[:]},
	{reflect.TypeFor[EventKind](), int(eventKindCount), eventKindNames[:]},
	{reflect.TypeFor[Verb](), int(verbCount), verbNames[:]},
	{reflect.TypeFor[Duration](), int(durationCount), durationNames[:]},
	{reflect.TypeFor[CostKind](), int(costKindCount), costKindNames[:]},
	{reflect.TypeFor[EffectKind](), int(effectKindCount), effectKindNames[:]},
}

var structTypes = []reflect.Type{
	reflect.TypeFor[Card](), reflect.TypeFor[Face](), reflect.TypeFor[PT](),
	reflect.TypeFor[Refusal](), reflect.TypeFor[ManaSym](), reflect.TypeFor[Compare](),
	reflect.TypeFor[Ref](), reflect.TypeFor[Target](), reflect.TypeFor[Filter](),
	reflect.TypeFor[Value](), reflect.TypeFor[Cond](), reflect.TypeFor[Ability](),
	reflect.TypeFor[Keyword](), reflect.TypeFor[Trigger](), reflect.TypeFor[Program](),
	reflect.TypeFor[Step](), reflect.TypeFor[Cost](), reflect.TypeFor[CostPart](),
	reflect.TypeFor[Effect](), reflect.TypeFor[Enter](),
}

func enumFor(t reflect.Type) (enumInfo, bool) {
	for _, e := range enums {
		if e.typ == t {
			return e, true
		}
	}
	return enumInfo{}, false
}

var kebab = regexp.MustCompile(`^[a-z]+(-[a-z]+)*$`)

func TestEnumNamesComplete(t *testing.T) {
	for _, e := range enums {
		if len(e.names) != e.count {
			t.Errorf("%v: name table has %d entries, sentinel says %d", e.typ, len(e.names), e.count)
		}
		if e.names[0] != "none" {
			t.Errorf("%v: zero value is %q, want none", e.typ, e.names[0])
		}
		seen := map[string]int{}
		for i := 0; i < e.count && i < len(e.names); i++ {
			n := e.names[i]
			if n == "" || !kebab.MatchString(n) {
				t.Errorf("%v value %d: name %q is empty or not lower-kebab", e.typ, i, n)
			}
			if j, dup := seen[n]; dup {
				t.Errorf("%v: name %q for both %d and %d", e.typ, n, j, i)
			}
			seen[n] = i
			// String and Parse agree with the table.
			v := reflect.New(e.typ).Elem()
			v.SetUint(uint64(i))
			if got := v.Interface().(interface{ String() string }).String(); got != n {
				t.Errorf("%v(%d).String() = %q, want %q", e.typ, i, got, n)
			}
			out := reflect.ValueOf(parseFuncs[e.typ]).Call([]reflect.Value{reflect.ValueOf(n)})
			if !out[1].Bool() || out[0].Uint() != uint64(i) {
				t.Errorf("Parse%v(%q) = %v, %v", e.typ.Name(), n, out[0], out[1])
			}
		}
		// A value past the sentinel has no name and does not parse.
		out := reflect.ValueOf(parseFuncs[e.typ]).Call([]reflect.Value{reflect.ValueOf("no-such-name")})
		if out[1].Bool() {
			t.Errorf("Parse%v accepted an unknown name", e.typ.Name())
		}
	}
}

var parseFuncs = map[reflect.Type]any{
	reflect.TypeFor[Layout]():      ParseLayout,
	reflect.TypeFor[ManaKind]():    ParseManaKind,
	reflect.TypeFor[Zone]():        ParseZone,
	reflect.TypeFor[CmpOp]():       ParseCmpOp,
	reflect.TypeFor[RefKind]():     ParseRefKind,
	reflect.TypeFor[ReceiptKind](): ParseReceiptKind,
	reflect.TypeFor[Subject]():     ParseSubject,
	reflect.TypeFor[FilterOp]():    ParseFilterOp,
	reflect.TypeFor[Pred]():        ParsePred,
	reflect.TypeFor[ValueKind]():   ParseValueKind,
	reflect.TypeFor[CondKind]():    ParseCondKind,
	reflect.TypeFor[AbilityKind](): ParseAbilityKind,
	reflect.TypeFor[EventKind]():   ParseEventKind,
	reflect.TypeFor[Verb]():        ParseVerb,
	reflect.TypeFor[Duration]():    ParseDuration,
	reflect.TypeFor[CostKind]():    ParseCostKind,
	reflect.TypeFor[EffectKind]():  ParseEffectKind,
}

func TestBitsetsCanonical(t *testing.T) {
	if got := (ColorW | ColorB | ColorG).String(); got != "WBG" {
		t.Errorf("ColorSet string = %q", got)
	}
	for _, bad := range []string{"X", "UW", "WW", "w"} {
		if _, ok := ParseColorSet(bad); ok {
			t.Errorf("ParseColorSet accepted %q", bad)
		}
	}
	for c := ColorSet(0); c <= allColors; c++ {
		if back, ok := ParseColorSet(c.String()); !ok || back != c {
			t.Errorf("ColorSet %d does not round trip via %q", c, c.String())
		}
	}
	z := MakeZoneSet(ZoneGraveyard, ZoneHand)
	if z.String() != "hand,graveyard" {
		t.Errorf("ZoneSet string = %q", z.String())
	}
}

func TestIRHoldsNoMaps(t *testing.T) {
	var check func(path string, t reflect.Type, seen map[reflect.Type]bool) []string
	check = func(path string, ty reflect.Type, seen map[reflect.Type]bool) []string {
		switch ty.Kind() {
		case reflect.Map, reflect.Func, reflect.Chan, reflect.Interface, reflect.UnsafePointer:
			return []string{path + " is a " + ty.Kind().String()}
		case reflect.Ptr, reflect.Slice, reflect.Array:
			return check(path+"[]", ty.Elem(), seen)
		case reflect.Struct:
			if seen[ty] {
				return nil
			}
			seen[ty] = true
			var bad []string
			for i := 0; i < ty.NumField(); i++ {
				f := ty.Field(i)
				bad = append(bad, check(path+"."+f.Name, f.Type, seen)...)
			}
			return bad
		}
		return nil
	}
	for _, st := range structTypes {
		for _, msg := range check(st.Name(), st, map[reflect.Type]bool{}) {
			t.Error(msg)
		}
		// Every node ends with Src, except the two value structs.
		last := st.Field(st.NumField() - 1)
		hasSrc := last.Name == "Src" && last.Type.Kind() == reflect.String
		if exempt := st.Name() == "Refusal" || st.Name() == "ManaSym"; hasSrc == exempt {
			t.Errorf("%s: last field is %s; Src must end every node struct except Refusal and ManaSym", st.Name(), last.Name)
		}
	}
}

// ---- reflective fixtures

// fill populates v so every field of every struct type is non-zero. To bound
// size, a struct type appears once on a path, plus once more through a field
// of its own type (Ref.Of, Filter.Args, ...). owner is the enclosing struct.
func fill(v reflect.Value, owner reflect.Type, onPath map[reflect.Type]int) {
	allowed := func(et reflect.Type) bool {
		return onPath[et] == 0 || (onPath[et] == 1 && owner == et)
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			fill(v.Field(i), v.Type(), onPath)
		}
	case reflect.Ptr:
		et := v.Type().Elem()
		if et.Kind() == reflect.Struct && !allowed(et) {
			return
		}
		onPath[et]++
		p := reflect.New(et)
		fill(p.Elem(), owner, onPath)
		v.Set(p)
		onPath[et]--
	case reflect.Slice:
		et := v.Type().Elem()
		if et.Kind() == reflect.Ptr && !allowed(et.Elem()) {
			return
		}
		s := reflect.MakeSlice(v.Type(), 1, 1)
		fill(s.Index(0), owner, onPath)
		v.Set(s)
	case reflect.String:
		v.SetString("s-" + v.Type().Name())
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int32:
		v.SetInt(3)
	case reflect.Uint8, reflect.Uint16:
		switch {
		case v.Type() == reflect.TypeFor[ColorSet]():
			v.SetUint(uint64(ColorW | ColorB))
		case v.Type() == reflect.TypeFor[ZoneSet]():
			v.SetUint(uint64(MakeZoneSet(ZoneHand, ZoneStack)))
		default:
			if e, ok := enumFor(v.Type()); ok {
				if e.count > 1 {
					v.SetUint(1)
				}
			} else {
				v.SetUint(2) // Ref.Slot
			}
		}
	default:
		panic("fill: unhandled kind " + v.Kind().String())
	}
}

func filled[T any]() *T {
	p := new(T)
	rt := reflect.TypeFor[T]()
	fill(reflect.ValueOf(p).Elem(), rt, map[reflect.Type]int{rt: 1})
	return p
}

// visit calls f on every value reachable from v.
func visit(v reflect.Value, f func(reflect.Value)) {
	f(v)
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			visit(v.Field(i), f)
		}
	case reflect.Ptr:
		if !v.IsNil() {
			visit(v.Elem(), f)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			visit(v.Index(i), f)
		}
	}
}

func mustDecode(t *testing.T, b []byte) *Card {
	t.Helper()
	c, err := DecodeCard(b)
	if err != nil {
		t.Fatalf("DecodeCard: %v\n%s", err, b)
	}
	return c
}

func TestCodecRoundTrip(t *testing.T) {
	base := filled[Card]()
	// The filled card reaches every struct type.
	reached := map[reflect.Type]bool{}
	visit(reflect.ValueOf(base).Elem(), func(v reflect.Value) { reached[v.Type()] = true })
	for _, st := range structTypes {
		// Compare is a standalone node no other node holds; it is covered below.
		if !reached[st] && st != reflect.TypeFor[Compare]() {
			t.Errorf("fixture never builds a %s", st.Name())
		}
	}
	roundTrip := func(label string, c *Card) {
		t.Helper()
		back := mustDecode(t, EncodeCard(c))
		if !reflect.DeepEqual(c, back) { // includes every Src
			t.Errorf("%s: round trip changed the card\nbefore %s\nafter  %s", label, c, back)
		}
		if !Equal(c, back) {
			t.Errorf("%s: round trip not Equal", label)
		}
	}
	roundTrip("filled", base)
	// Every value of every enum, in every field of that type.
	for _, e := range enums {
		for i := 0; i < e.count; i++ {
			c := mustDecode(t, EncodeCard(base))
			n := 0
			visit(reflect.ValueOf(c).Elem(), func(v reflect.Value) {
				if v.Type() == e.typ && v.CanSet() {
					v.SetUint(uint64(i))
					n++
				}
			})
			if n == 0 {
				t.Fatalf("fixture has no %v field", e.typ)
			}
			roundTrip(e.typ.Name()+"="+e.names[i], c)
		}
	}
	// Compare, with every operator, through its own JSON.
	for op := CmpOp(0); op < cmpOpCount; op++ {
		cmp := filled[Compare]()
		cmp.Op = op
		b, err := json.Marshal(cmp)
		if err != nil {
			t.Fatal(err)
		}
		var back Compare
		if err := json.Unmarshal(b, &back); err != nil || !reflect.DeepEqual(cmp, &back) {
			t.Errorf("Compare %v does not round trip: %v", op, err)
		}
	}
	// Every color set and a few zone sets.
	for cs := ColorSet(0); cs <= allColors; cs++ {
		c := &Card{Faces: []*Face{{ColorIndicator: cs}}}
		roundTrip("colors "+cs.String(), c)
	}
	for _, zs := range []ZoneSet{0, MakeZoneSet(ZoneBattlefield), MakeZoneSet(ZoneNone+1, ZoneCommand, ZoneExile)} {
		roundTrip("zones "+zs.String(), &Card{Faces: []*Face{{Abilities: []*Ability{{Zones: zs}}}}})
	}
	// Optional numbers: a pointer to zero survives, and differs from nil.
	zero := int32(0)
	withZero := &Card{Faces: []*Face{{Loyalty: &zero}}}
	roundTrip("loyalty 0", withZero)
	if Equal(withZero, &Card{Faces: []*Face{{}}}) {
		t.Error("Loyalty=&0 must differ from Loyalty=nil")
	}
	// Empty and nil are one thing.
	e1 := mustDecode(t, EncodeCard(&Card{Faces: []*Face{{Types: []string{}}}}))
	if !Equal(e1, &Card{Faces: []*Face{{}}}) || e1.String() != (&Card{Faces: []*Face{{}}}).String() {
		t.Error("empty slice and nil slice must be equal and print alike")
	}
}

func TestCodecDeterministic(t *testing.T) {
	c := filled[Card]()
	a, b := EncodeCard(c), EncodeCard(c)
	if string(a) != string(b) {
		t.Error("two encodings of one card differ")
	}
	// Canonical: re-encoding a decoded card gives the same bytes.
	if again := EncodeCard(mustDecode(t, a)); string(again) != string(a) {
		t.Errorf("decode-then-encode changed the bytes\n%s\n%s", a, again)
	}
	// Empty is omitted.
	if got := string(EncodeCard(&Card{Faces: []*Face{}})); got != "{}" {
		t.Errorf("empty card encodes as %s", got)
	}
}

func TestCodecUsesNames(t *testing.T) {
	b, err := json.Marshal(&Ref{Kind: RefControllerOf})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"kind":"controller-of"}` {
		t.Errorf("Ref encodes as %s", b)
	}
	// Unknown names are rejected, naming the culprit; ordinals are not names.
	for in, want := range map[string]string{
		`{"layout":"bogus-layout"}`: "bogus-layout",
		`{"layout":1}`:              "Layout",
		`{"faces":[{"abilities":[{"kind":"sorcery-ish"}]}]}`:            "sorcery-ish",
		`{"faces":[{"colorIndicator":"WX"}]}`:                           "WX",
		`{"faces":[{"abilities":[{"zones":["nowhere"]}]}]}`:             "nowhere",
		`{"faces":[{"abilities":[{"zones":["none"]}]}]}`:                "none",
		`{"faces":[{"abilities":[{"body":{"steps":[{"verb":"v"}]}}]}]}`: "v",
		`{"nosuchfield":1}`: "nosuchfield",
	} {
		_, err := DecodeCard([]byte(in))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("DecodeCard(%s) error = %v, want one naming %q", in, err, want)
		}
	}
	if _, err := DecodeCard([]byte(`{} {}`)); err == nil {
		t.Error("trailing data accepted")
	}
}

type mutation struct {
	desc    string
	srcOnly bool
	apply   func()
}

// mutations lists one-field edits of the tree under v. path names the field
// the edit is in (Type.Field), so callers can dedupe equivalent edits.
func mutations(v reflect.Value, path string, out *[]mutation) {
	add := func(desc string, src bool, f func()) {
		*out = append(*out, mutation{path + ": " + desc, src, f})
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			fv := v.Field(i)
			if f.Name == "Src" {
				add("Src", true, func() { fv.SetString(fv.String() + "!") })
				continue
			}
			mutations(fv, v.Type().Name()+"."+f.Name, out)
		}
	case reflect.Ptr:
		if v.IsNil() {
			return
		}
		add("set nil", false, func() { v.Set(reflect.Zero(v.Type())) })
		mutations(v.Elem(), path, out)
	case reflect.Slice:
		if v.Len() == 0 {
			return
		}
		add("drop last", false, func() { v.Set(v.Slice(0, v.Len()-1)) })
		for i := 0; i < v.Len(); i++ {
			mutations(v.Index(i), path+"[]", out)
		}
	case reflect.String:
		add("string", false, func() { v.SetString(v.String() + "x") })
	case reflect.Bool:
		add("flip", false, func() { v.SetBool(!v.Bool()) })
	case reflect.Int32:
		add("int", false, func() { v.SetInt(v.Int() + 1) })
	case reflect.Uint8, reflect.Uint16:
		add("uint", false, func() { v.SetUint(v.Uint() + 1) })
	}
}

func TestPrintMatchesEqual(t *testing.T) {
	base := filled[Card]()
	baseJSON := EncodeCard(base)
	baseStr := base.String()
	var all []mutation
	mutations(reflect.ValueOf(mustDecode(t, baseJSON)).Elem(), "Card", &all)
	// One edit per (type, field, kind of edit) keeps the run short.
	var picks []int
	seenEdit := map[string]bool{}
	for i, m := range all {
		if !seenEdit[m.desc] {
			seenEdit[m.desc] = true
			picks = append(picks, i)
		}
	}
	if len(picks) < 100 {
		t.Fatalf("only %d distinct mutations; fixture too small", len(picks))
	}
	for _, k := range picks {
		c := mustDecode(t, baseJSON)
		var ms []mutation
		mutations(reflect.ValueOf(c).Elem(), "Card", &ms)
		m := ms[k]
		m.apply()
		eq, same := Equal(base, c), baseStr == c.String()
		if m.srcOnly {
			if !eq || !same {
				t.Fatalf("%s: Src-only change: Equal=%v print-same=%v", m.desc, eq, same)
			}
			continue
		}
		if eq || same {
			t.Fatalf("%s: Equal=%v print-same=%v, want both false\n%s\n%s", m.desc, eq, same, baseStr, c)
		}
	}

	// Spot checks of the documented grammar.
	str := func(s stringer, want string) {
		t.Helper()
		if got := s.String(); got != want {
			t.Errorf("print = %s, want %s", got, want)
		}
	}
	two := &Value{Kind: VConst, N: 2}
	str(&Ref{Kind: RefSelf}, `(ref self)`)
	str(&Ref{Kind: RefTarget}, `(ref target 0)`)
	str(&Ref{Kind: RefControllerOf, Of: &Ref{Kind: RefTarget, Slot: 1}}, `(ref controller-of (ref target 1))`)
	str(&Filter{Op: FAll, Args: []*Filter{
		{Op: FPred, Pred: PCardType, Word: "Creature"},
		{Op: FNot, Args: []*Filter{{Op: FPred, Pred: PToken}}},
	}}, `(filter all (filter pred card-type "Creature") (filter not (filter pred token)))`)
	str(&Value{Kind: VAdd, Args: []*Value{two, {Kind: VX}}}, `(value add (value const 2) (value x))`)
	str(&Value{Kind: VConst}, `(value const 0)`)
	str(&Step{Verb: VerbDraw, N: two}, "(step draw :n (value const 2))")

	// Layouts that must not collide.
	distinct := []stringer{
		&Ref{Kind: RefChoose, N: two}, &Ref{Kind: RefChoose, Of: &Ref{Kind: RefYou}},
		&Ref{Kind: RefChoose, Union: []*Ref{{Kind: RefYou}}},
		&Ref{Kind: RefChoose, Union: []*Ref{nil}}, &Ref{Kind: RefChoose},
		&Filter{Op: FAll, Args: []*Filter{{}}}, &Filter{Op: FAll, Args: []*Filter{nil}},
		&Filter{Op: FAll, Word: "nil"}, &Filter{Op: FAll, Pred: PName},
		&Filter{Op: FAll, Word: "x"}, &Filter{Op: FAll, Value: &Value{Kind: VX}},
		&Value{Kind: VX, N: 1}, &Value{Kind: VX, Args: []*Value{{Kind: VConst, N: 1}}},
		&Value{Kind: VX, Word: "1"}, &Value{Kind: VX, Ref: &Ref{Kind: RefYou}}, &Value{Kind: VX, Ref2: &Ref{Kind: RefYou}}, &Value{Kind: VX},
		&Cond{Kind: CCompare, L: two}, &Cond{Kind: CCompare, R: two},
		&Step{Verb: VerbNone, Then: &Program{}}, &Step{Verb: VerbNone, Else: &Program{}},
		&Compare{Op: CmpEQ}, &Compare{Op: CmpEQ, V: &Value{}}, &Compare{},
	}
	seen := map[string]int{}
	for i, d := range distinct {
		if j, dup := seen[d.String()]; dup {
			t.Errorf("entries %d and %d both print as %s", j, i, d)
		}
		seen[d.String()] = i
	}
}

type stringer interface{ String() string }

func TestEqualGeneric(t *testing.T) {
	var nilRef *Ref
	if !Equal(nilRef, (*Ref)(nil)) || Equal(nilRef, &Ref{}) || Equal(&Ref{}, nilRef) {
		t.Error("nil handling")
	}
	if Equal(&Ref{}, &Filter{}) || Equal(&Ref{}, Ref{}) || Equal(nil, nil) || Equal((*Ref)(nil), (*Filter)(nil)) {
		t.Error("different dynamic types must be unequal")
	}
	if !Equal(&Ref{Src: "a"}, &Ref{Src: "b"}) {
		t.Error("Src must be ignored")
	}
	if !Equal(&ManaSym{Kind: ManaX}, &ManaSym{Kind: ManaX}) || Equal(&Refusal{Key: "a"}, &Refusal{Key: "b"}) {
		t.Error("value structs")
	}
}

// TestTargetsLiveOnPrograms guards the 11-24 decision: targets are scoped per
// Program (as in mtgish's Actions.Targeted), never per Ability. A ticket that
// needs ability-wide targets stops and reports instead of editing this test.
func TestTargetsLiveOnPrograms(t *testing.T) {
	if _, ok := reflect.TypeFor[Ability]().FieldByName("Targets"); ok {
		t.Error("ir.Ability must not have a Targets field; targets live on ir.Program")
	}
	if _, ok := reflect.TypeFor[Program]().FieldByName("Targets"); !ok {
		t.Error("ir.Program must have a Targets field")
	}
}

// TestRefUpTo: UpTo is part of a Ref's meaning, so Equal, the printer and the codec all see it.
func TestRefUpTo(t *testing.T) {
	a := &Ref{Kind: RefChoose, N: &Value{Kind: VConst, N: 2}}
	b := &Ref{Kind: RefChoose, N: &Value{Kind: VConst, N: 2}, UpTo: true}
	if Equal(a, b) || a.String() == b.String() {
		t.Errorf("UpTo ignored: Equal=%v\n%s\n%s", Equal(a, b), a, b)
	}
	if got := b.String(); !strings.Contains(got, ":up-to") {
		t.Errorf("print %s", got)
	}
	j, err := json.Marshal(b)
	if err != nil || !strings.Contains(string(j), `"upTo":true`) {
		t.Errorf("json %s %v", j, err)
	}
	var back Ref
	if err := json.Unmarshal(j, &back); err != nil || !back.UpTo {
		t.Errorf("round trip %+v %v", back, err)
	}
}
