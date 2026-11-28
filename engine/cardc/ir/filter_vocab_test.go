package ir

import (
	"slices"
	"strings"
	"testing"
)

func TestPredOperandsCoverEveryPred(t *testing.T) {
	if len(predOperands) != int(predCount) {
		t.Fatalf("predOperands has %d rows, predCount is %d", len(predOperands), predCount)
	}
	for p := PAny; p < predCount; p++ {
		if !predOperands[p].set {
			t.Errorf("Pred %s has no predOperands row", p)
		}
	}
	if predOperands[PNone].set {
		t.Error("PNone must have no predOperands row")
	}
}

func TestWordListsSortedAndDisjoint(t *testing.T) {
	lists := map[string][]string{
		"cardTypes": cardTypeWords, "supertypes": supertypeWords, "subtypes": subtypeWords,
		"keywords": keywordWords, "counterKinds": counterKindWords,
	}
	for name, l := range lists {
		if len(l) == 0 {
			t.Errorf("%s is empty", name)
		}
		if !slices.IsSorted(l) {
			t.Errorf("%s is not sorted", name)
		}
		for i := 1; i < len(l); i++ {
			if l[i] == l[i-1] {
				t.Errorf("%s holds %q twice", name, l[i])
			}
		}
	}
	if len(cardTypeWords) != 15 || len(supertypeWords) != 5 {
		t.Errorf("CR 205.2a has 15 card types and CR 205.4a 5 supertypes; got %d and %d", len(cardTypeWords), len(supertypeWords))
	}
	if len(subtypeWords) < 600 {
		t.Errorf("subtypes has %d words, want the ~630 of the type vocabulary", len(subtypeWords))
	}
	typeLists := [][]string{cardTypeWords, supertypeWords, subtypeWords}
	for i, a := range typeLists {
		for _, w := range a {
			for j, b := range typeLists {
				if j != i && slices.Contains(b, w) {
					t.Errorf("%q is in two type lists", w)
				}
			}
		}
	}
	// Lookups agree with the lists.
	for _, c := range []struct {
		w    string
		want WordKind
	}{
		{"Creature", WordCardType}, {"Legendary", WordSupertype}, {"Goblin", WordSubtype},
		{"Forest", WordSubtype}, {"creature", WordNone}, {"Flying", WordNone}, {"", WordNone},
	} {
		if got := TypeWord(c.w); got != c.want {
			t.Errorf("TypeWord(%q) = %s, want %s", c.w, got, c.want)
		}
	}
	if !IsKeyword("First strike") || IsKeyword("First Strike") || IsKeyword("Goblin") {
		t.Error("IsKeyword must match the rules name exactly")
	}
	if !IsCounterKind("+1/+1") || !IsCounterKind("loyalty") || IsCounterKind("Loyalty") {
		t.Error("IsCounterKind must match the lower-case printed name exactly")
	}
}

func TestValidateFilter(t *testing.T) {
	pred := func(p Pred) *Filter { return &Filter{Op: FPred, Pred: p} }
	word := func(p Pred, w string) *Filter { f := pred(p); f.Word = w; return f }
	withRef := func(f *Filter, r *Ref) *Filter { f.Ref = r; return f }
	mem := func(m Memory) *Ref { return &Ref{Kind: RefMemory, Memory: m} }
	two := &Value{Kind: VConst, N: 2}
	cmp := func(p Pred) *Filter { f := pred(p); f.Cmp, f.Value = CmpGE, two; return f }
	zones := func(f *Filter, z ...Zone) *Filter { f.Zones = MakeZoneSet(z...); return f }
	subj := func(f *Filter, s Subject) *Filter { f.Subject = s; return f }
	all := func(args ...*Filter) *Filter { return &Filter{Op: FAll, Args: args} }

	valid := map[string]*Filter{
		"card type":         word(PCardType, "Creature"),
		"supertype":         word(PSupertype, "Legendary"),
		"subtype":           word(PSubtype, "Goblin"),
		"subtype memory":    withRef(pred(PSubtype), mem(MemChosenType)),
		"color":             {Op: FPred, Pred: PColor, Colors: ColorR},
		"color memory":      withRef(pred(PColor), mem(MemChosenColor)),
		"name free text":    word(PName, "Any Name At All"),
		"name memory":       withRef(pred(PName), mem(MemNamedCard)),
		"keyword":           word(PHasKeyword, "First strike"),
		"counter":           func() *Filter { f := word(PCounters, "+1/+1"); f.Cmp, f.Value = CmpGE, two; return f }(),
		"mana value":        cmp(PManaValue),
		"emblem":            pred(PEmblem),
		"blocked by":        withRef(pred(PBlockedBy), &Ref{Kind: RefSelf}),
		"blocking a ref":    withRef(pred(PBlocking), &Ref{Kind: RefSelf}),
		"attacking bare":    pred(PAttacking),
		"was cast":          withRef(zones(pred(PWasCast), ZoneHand), &Ref{Kind: RefYou}),
		"entered":           zones(pred(PEnteredThisTurn), ZoneGraveyard),
		"shares name":       withRef(pred(PSharesName), &Ref{Kind: RefTarget}),
		"is memory":         withRef(pred(PIs), mem(MemRemembered)),
		"all of two":        all(word(PCardType, "Creature"), pred(PToken)),
		"any of one":        {Op: FAny, Args: []*Filter{pred(PEmblem)}},
		"not":               {Op: FNot, Args: []*Filter{pred(PToken)}},
		"subject inherited": subj(all(pred(PTapped), subj(pred(PToken), SubjPermanent)), SubjPermanent),
		"zones scope":       zones(all(zones(word(PCardType, "Land"), ZoneBattlefield), pred(PTapped)), ZoneBattlefield),
		"controls own subject": subj(func() *Filter {
			f := pred(PControls)
			f.Args = []*Filter{subj(word(PCardType, "Artifact"), SubjPermanent)}
			return f
		}(), SubjPlayer),
		"controlled by player filter": subj(withRef(pred(PControlledBy),
			&Ref{Kind: RefAll, Filter: subj(pred(PAttackedThisTurn), SubjPlayer)}), SubjPermanent),
	}
	for name, f := range valid {
		if err := ValidateFilter(f); err != nil {
			t.Errorf("%s: ValidateFilter = %v, want nil", name, err)
		}
	}

	invalid := []struct {
		name, want string
		f          *Filter
	}{
		{"nil", "nil filter", nil},
		{"no op", "no op", &Filter{}},
		{"empty all", "all: needs at least one arg", &Filter{Op: FAll}},
		{"empty any", "any: needs at least one arg", &Filter{Op: FAny}},
		{"all with pred", "all: carries predicate operands", &Filter{Op: FAll, Args: []*Filter{pred(PToken)}, Word: "x"}},
		{"not two args", "not: needs exactly one arg", &Filter{Op: FNot, Args: []*Filter{pred(PToken), pred(PTapped)}}},
		{"nil arg", "nil filter", all(nil)},
		{"pred none", "pred: needs a predicate", pred(PNone)},
		{"word on token", "token: takes no word", word(PToken, "Creature")},
		{"card type off list", "card-type: \"Goblin\" is not a card type", word(PCardType, "Goblin")},
		{"lower-case type", "card-type: \"creature\" is not a card type", word(PCardType, "creature")},
		{"supertype off list", "supertype: \"Creature\" is not a supertype", word(PSupertype, "Creature")},
		{"subtype off list", "subtype: \"Legendary\" is not a subtype", word(PSubtype, "Legendary")},
		{"keyword off list", "has-keyword: \"First Strike\" is not a keyword", word(PHasKeyword, "First Strike")},
		{"counter off list", "counters: \"Loyalty\" is not a counter kind", word(PCounters, "Loyalty")},
		{"ref on emblem", "emblem: takes no ref", withRef(pred(PEmblem), &Ref{Kind: RefSelf})},
		{"word and ref", "subtype: takes a word or a memory ref, not both", withRef(word(PSubtype, "Goblin"), mem(MemChosenType))},
		{"non-memory ref on name", "name: ref must be a memory ref", withRef(pred(PName), &Ref{Kind: RefTarget})},
		{"colors on name", "name: takes no colors", &Filter{Op: FPred, Pred: PName, Colors: ColorU}},
		{"cmp on tapped", "tapped: takes no comparison", cmp(PTapped)},
		{"cmp without value", "mana-value: cmp and value go together", &Filter{Op: FPred, Pred: PManaValue, Cmp: CmpEQ}},
		{"value without cmp", "power: cmp and value go together", &Filter{Op: FPred, Pred: PPower, Value: two}},
		{"args on tapped", "tapped: takes no args", &Filter{Op: FPred, Pred: PTapped, Args: []*Filter{pred(PToken)}}},
		{"zones on nested emblem", "emblem: takes no zones", all(zones(pred(PEmblem), ZoneExile))},
		{"subject mismatch", "token: subject player differs from the root's permanent", subj(all(subj(pred(PToken), SubjPlayer)), SubjPermanent)},
		{"bad word deep in ref", "card-type: \"Elf\" is not a card type", withRef(pred(PControlledBy),
			&Ref{Kind: RefAll, Filter: word(PCardType, "Elf")})},
		{"bad word under controls", "card-type: \"Elf\" is not a card type", &Filter{Op: FPred, Pred: PControls, Args: []*Filter{word(PCardType, "Elf")}}},
	}
	for _, c := range invalid {
		err := ValidateFilter(c.f)
		if err == nil {
			t.Errorf("%s: ValidateFilter = nil, want an error", c.name)
			continue
		}
		if !strings.HasPrefix(err.Error(), "ir: ") || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: ValidateFilter = %q, want it to name %q", c.name, err, c.want)
		}
	}
}

func TestRefMemoryPrintsAndCompares(t *testing.T) {
	a := &Ref{Kind: RefMemory, Memory: MemImprinted}
	if got := a.String(); got != "(ref memory imprinted)" {
		t.Errorf("print = %s, want (ref memory imprinted)", got)
	}
	if Equal(a, &Ref{Kind: RefMemory, Memory: MemRemembered}) {
		t.Error("refs naming different memories must differ")
	}
}
