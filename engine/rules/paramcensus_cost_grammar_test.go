package rules

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

// cr-g180: the cost-gap census prices a Cost$/UnlessCost$ value with the
// grammar the runtime pays it with, not plain ParseCost. A RaiseCost static's
// Cost$ is read by raiseExtraResolve (its real-gap channel is
// extra.Withheld); an UnlessCost$ by effects.UnlessCostResolved's token folds
// then ParseUnlessCost; a Ward UnlessCost$ by beginWardPayment's arms
// (wardCostShapeKnown). Any other Cost$ stays ParseCost.

var (
	censusUnlessEnergyTok = regexp.MustCompile(`^PayEnergy<([^>]*)>$`)
	censusUnlessLifeXTok  = regexp.MustCompile(`^PayLife<X>$`)
	censusUnlessAmountTok = regexp.MustCompile(`^(DamageYou|Draw)<X(/[^>]*)?>$`)
)

// censusUnlessShapeResolved rewrites the amount-bearing tokens
// UnlessCostResolved folds from the face's SVar table (DamageYou<X>,
// Draw<X/..>, PayLife<X>, PayEnergy<Name>) to an amount of 1: the census
// needs the SHAPE the runtime pays, not the amount. A token whose SVar is
// absent stays raw, exactly as the runtime leaves an unbound amount.
func censusUnlessShapeResolved(f *cards.Face, raw string) string {
	fields := strings.Fields(raw)
	for i, tok := range fields {
		if strings.EqualFold(tok, "Mandatory") {
			fields[i] = ""
			continue
		}
		if m := censusUnlessEnergyTok.FindStringSubmatch(tok); m != nil {
			if _, ok := f.SVars[m[1]]; ok && m[1] != "X" && !isDigitRun(m[1]) {
				fields[i] = "PayEnergy<1>"
			}
			continue
		}
		if _, ok := f.SVars["X"]; !ok {
			continue
		}
		if censusUnlessLifeXTok.MatchString(tok) {
			fields[i] = "PayLife<1>"
		} else if m := censusUnlessAmountTok.FindStringSubmatch(tok); m != nil {
			fields[i] = m[1] + "<1" + m[2] + ">"
		}
	}
	return strings.Join(strings.Fields(strings.Join(fields, " ")), " ")
}

// censusCostUnknown is the cost tokens the census labels for one Cost$ or
// UnlessCost$ value. owner is "api:<API>" or "stat:<Mode>".
func censusCostUnknown(f *cards.Face, owner, key, value string) []string {
	switch key {
	case "Cost":
		if owner == "stat:RaiseCost" {
			// Shape only: a count whose SVar body is present resolves to an
			// amount of 1 (an SVar body the evaluator cannot read is already
			// a count:<head> gap), as Tectonic Split's Sac<X/Land> over
			// SVar:X:Count$Valid needs. "SVar$Name" is the indirection to
			// another SVar (March's Y -> Exiled) and follows it; a dangling
			// one stays unresolved, so the head is withheld.
			var eval func(body string, depth int) (int32, bool)
			eval = func(body string, depth int) (int32, bool) {
				name, ok := strings.CutPrefix(strings.TrimSpace(body), "SVar$")
				if !ok {
					return 1, true
				}
				next, ok := f.SVars[name]
				if !ok || depth > 8 {
					return 0, false
				}
				return eval(next, depth+1)
			}
			var heads []string
			for _, w := range raiseExtraResolve(value, f.SVars, faceAnnounces(f.SpellAbility()), func(body string) (int32, bool) { return eval(body, 0) }).extra.Withheld {
				// Withheld records "Head<name>"; the census labels the head.
				head, _, _ := strings.Cut(w, "<")
				heads = append(heads, head)
			}
			return heads
		}
	case "UnlessCost":
		if unlessNamesFaceSVar(f, key, value) {
			return nil
		}
		if owner == "api:Ward" && wardCostShapeKnown(strings.TrimSpace(value)) {
			return nil
		}
		// DamageYou<N> is the unless-pay arm's own damage path
		// (effects.ParseDamageUnlessCost), not ParseUnlessCost's.
		for _, v := range []string{value, censusUnlessShapeResolved(f, value)} {
			if _, ok := ParseUnlessCost(v); ok {
				return nil
			}
			if _, ok := effects.ParseDamageUnlessCost(v); ok {
				return nil
			}
		}
	}
	return ParseCost(value).Unknown
}

func censusCostLabels(t *testing.T, f *cards.Face) []string {
	t.Helper()
	_, d := measureParamCensus(t, nil)
	if d == nil {
		t.Fatal("no derived read sets")
	}
	var out []string
	for _, l := range cardCensusLabels(&cards.Card{Faces: []*cards.Face{f}}, d, nil) {
		if strings.HasPrefix(l, "cost:") {
			out = append(out, l)
		}
	}
	return out
}

func TestCensusCostRaiseCostUsesRaiseGrammar(t *testing.T) {
	t.Parallel()
	const line = "ExileFromHand<Y/Card.Red/red cards>"
	mk := func(announce string) *cards.Face {
		sp := map[string]string{}
		if announce != "" {
			sp["Announce"] = announce
		}
		return &cards.Face{
			Name:  "raise-fixture",
			SVars: map[string]string{"Y": "SVar$Exiled"},
			Abilities: []*cards.SA{
				{Kind: "SP", API: "PermanentCreature", Params: sp},
			},
			Statics: []cards.Static{{Mode: "RaiseCost", Params: map[string]string{"Cost": line, "Type": "Spell"}}},
		}
	}
	if got := ParseCost(line).Unknown; !slices.Contains(got, "ExileFromHand") {
		t.Fatalf("precondition: plain ParseCost(%q).Unknown = %v, want ExileFromHand", line, got)
	}
	if got := censusCostLabels(t, mk("Exiled")); len(got) != 0 {
		t.Errorf("RaiseCost with Announce$ Exiled labelled %v, want none", got)
	}
	if got := censusCostLabels(t, mk("")); !slices.Equal(got, []string{"cost:ExileFromHand"}) {
		t.Errorf("RaiseCost with no Announce$ labelled %v, want [cost:ExileFromHand]", got)
	}
}

func TestCensusCostUnlessUsesUnlessGrammar(t *testing.T) {
	t.Parallel()
	mk := func(unless string, svars map[string]string) *cards.Face {
		return &cards.Face{
			Name:  "unless-fixture",
			SVars: svars,
			Abilities: []*cards.SA{
				{Kind: "SP", API: "Counter", Params: map[string]string{"UnlessCost": unless}},
			},
		}
	}
	for _, v := range []string{
		"AddCounter<1/P1P1/CARDNAME>",
		"ExileFromGrave<4/Card> ExileFromStack<1/Card.Self>",
	} {
		if got := ParseCost(v).Unknown; len(got) == 0 {
			t.Fatalf("precondition: plain ParseCost(%q) models every token", v)
		}
		if got := censusCostLabels(t, mk(v, nil)); len(got) != 0 {
			t.Errorf("UnlessCost$ %s labelled %v, want none", v, got)
		}
	}
	if got := censusCostLabels(t, mk("DamageYou<X>", map[string]string{"X": "Count$CardPower"})); len(got) != 0 {
		t.Errorf("DamageYou<X> with SVar:X labelled %v, want none", got)
	}
	if got := censusCostLabels(t, mk("DamageYou<X>", nil)); !slices.Equal(got, []string{"cost:DamageYou"}) {
		t.Errorf("DamageYou<X> without SVar:X labelled %v, want [cost:DamageYou]", got)
	}
	if got := censusCostLabels(t, mk("Foo<1>", nil)); !slices.Equal(got, []string{"cost:Foo"}) {
		t.Errorf("Foo<1> labelled %v, want [cost:Foo]", got)
	}
}

func TestCensusCostWardUsesWardGrammar(t *testing.T) {
	t.Parallel()
	mk := func(unless string) *cards.Face {
		return &cards.Face{
			Name: "ward-fixture",
			Abilities: []*cards.SA{
				{Kind: "AB", API: "Ward", Params: map[string]string{"UnlessCost": unless}},
			},
		}
	}
	if got := censusCostLabels(t, mk("Discard<1/Card>:2")); len(got) != 0 {
		t.Errorf("Ward Discard<1/Card>:2 labelled %v, want none", got)
	}
	if got := censusCostLabels(t, mk("Foo<1>")); !slices.Equal(got, []string{"cost:Foo"}) {
		t.Errorf("Ward Foo<1> labelled %v, want [cost:Foo]", got)
	}
}

func TestWardCostShapeKnown(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]bool{
		"PayLife<X/life equal to CARDNAME's power>": true,
		"AddCounterYou<5/POISON>":                   true,
		"Discard<1/Card>:2":                         true,
		"2":                                         true,
		"Foo<1>":                                    false,
		"Discard<1/Card>:Bogus<2>":                  false,
	} {
		if got := wardCostShapeKnown(raw); got != want {
			t.Errorf("wardCostShapeKnown(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestCensusCostKeepsGixDiscardX(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	c := searchCorpusCard(t, reg, "Gix, Yawgmoth Praetor")
	_, d := measureParamCensus(t, nil)
	if got := cardCensusLabels(c, d, nil); !slices.Contains(got, "cost:Discard") {
		t.Errorf("Gix labels = %v, want cost:Discard kept (activation Discard<X> is a real gap)", got)
	}
}
