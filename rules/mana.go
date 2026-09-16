package rules

import (
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

// CostPart is one non-mana cost component: Sac<N/Spec> (sacrifice N
// permanents matching Spec), Discard<N/Spec> (discard N matching cards), or
// SubCounter<N/Kind> (remove N counters of Kind from the source).
type CostPart struct {
	N    int32
	Spec string
	// Zone is the zone an Exile cost part pays from: ZHand for an
	// ExileFromHand token (the default zero value) or ZGraveyard for an
	// ExileFromGrave token. Sac/Discard/SubCounter parts never read it.
	Zone state.Zone
	// Announced marks the variable-count form of a Sac part (Sac<X/Spec> --
	// Dargo's "sacrifice any number"): the player announces the count as the
	// cast's X (CR 601.2b) and exactly that many permanents matching Spec are
	// sacrificed; a ReduceCost static reading the paid X composes with it.
	// N is unused for an Announced part.
	Announced bool
}

// ManaPair is one two-colour hybrid symbol: both A and B are WUBRG letters,
// and either one spells the pip (CR 107.4e).
type ManaPair struct{ A, B byte }

// Cost is a parsed cost. X counts how many "X" symbols appeared (almost
// always 0 or 1; WithX folds a chosen value into Generic once per symbol).
// Life, Tap, Sac, Discard and SubCounter are non-mana components a cast or
// activation must satisfy separately from mana payment; Exile<N/Spec> (from
// ExileFromHand/ExileFromGrave tokens) exiles matching cards as the payment;
// AddCounter<N/LOYALTY> is a free non-mana component (a planeswalker's [+N]
// loyalty gain) settled beside SubCounter by the ability branch in
// rules/cast.go. Life is paid through payMana's LifeChange event; Tap, Sac,
// Discard, SubCounter and Exile are settled by the cast-flow stages in
// rules/cast.go. Pay and CanPay remain pool-only helpers.
//
// Hybrid and Phyrexian symbols are no longer flattened to generic. A hybrid
// pip (GW) is recorded in Hybrid as the pair of colours it accepts; a
// Phyrexian pip (UP) is recorded in Phyrexian as its colour (CR 107.4f: pay
// that colour OR two life). Both are “announcement” costs: which half of a
// hybrid and whether a Phyrexian pip is paid with life is a player choice at
// cast time (CR 601.2b), not something a parser decides. Because a Phyrexian
// pip may be paid with life, the mana-only Pay/CanPay below cannot fully own
// it; rule/cast.go's payment stage resolves the announced choice and spends
// against both the pool and the payer's life (see Cost.payable).
type Cost struct {
	Colored      state.Mana
	Generic      int32
	Life         int32
	X            int
	Hybrid       []ManaPair
	Phyrexian    []byte
	Tap          bool
	Sac          []CostPart
	Discard      []CostPart
	SubCounter   []CostPart
	AddCounter   []CostPart
	Exile        []CostPart
	Reveal       []CostPart
	Behold       []CostPart
	TapPermanent []CostPart
	Blight       []CostPart
	Forage       bool
	// Energy carries PayEnergy<N> tokens: N energy counters (a player
	// counter, kind ENERGY) removed from the payer (CR 118.2d's energy
	// payment). The dynamic form PayEnergy<X> is the SAME X the cast
	// announces (Forge CostPayEnergy.getMaxAmountX bounds it by the payer's
	// energy count) and is recorded as a part with Spec "X" (N 0) -- xAsk
	// bounds the announced value, the settle spends exactly that many.
	Energy []CostPart
	// Return carries Return<N/Spec> tokens: a permanent (usually the source
	// itself, Spec CARDNAME) returned to its OWNER's hand as the payment
	// (Forge CostReturn.moveToHand; CR 118.2a lists returning a permanent to
	// its owner's hand among the payment actions).
	Return []CostPart

	// Unknown lists the HEAD (the text before any "<...>") of every cost
	// token this parse did not model, in order of appearance, deduplicated.
	// A token lands here exactly when ParseCost could not give it real
	// semantics and priced it as one generic mana (or one life-equivalent
	// of nothing) instead: the final unrecognised-symbol fallback AND the
	// malformed/out-of-range instances of otherwise-recognised heads (an
	// unparseable or int-overflow "PayLife<...>", "Sac<...>",
	// "AddCounter<...>" value — the head is known, that INSTANCE is not
	// modelled). Payment behaviour is unchanged by this field: it is a pure
	// report, read by the parameter census (rules/paramcensus_test.go) so
	// the repo-deck ratchet can name cost tokens a card's script carries
	// that the engine silently substitutes generic mana for (e.g. Chthonian
	// Nightmare's "PayEnergy<X> ... Return<1/CARDNAME>").
	Unknown []string
}

// nonManaCost matches Sac<N/Spec>, Discard<N/Spec>, and SubCounter<N/Kind> tokens. Forge
// appends a human-readable "/description" after the spec and separates OR
// alternatives with ";"; the description may itself contain spaces (e.g.
// "Sac<1/Artifact;Creature/artifact or creature>"), which is why
// splitCostTokens keeps the whole <...> group atomic before nonManaCost ever
// sees it. The captured group only runs up to the first "/", so the trailing
// description is dropped right here; the ";" alternation is folded to ","
// (MatchesSpec's own separator) at the parse site. Ruling FL-54.
var nonManaCost = regexp.MustCompile(`^(Sac|SubCounter|Discard)<(\d+)/([^/>]+)(?:/[^>]*)?>$`)

// sacXCost matches the announced-count sacrifice form Sac<X/Spec> (Dargo, the
// Shipwrecker's "sacrifice any number of artifacts and/or creatures"): the
// player announces X (0..candidates, CR 601.2b) and exactly X permanents are
// sacrificed. The part is recorded with Spec "X" (N 0) -- the same announced
// convention PayEnergy<X> uses -- and xAsk/sacAsk consume it; a ReduceCost
// static that reads the paid X (Dargo's SVar X:Count$xPaid) resolves through
// costModifiers' SVar-aware amount read.
var sacXCost = regexp.MustCompile(`^Sac<X/([^/>]+)(?:/[^>]*)?>$`)

// exileCost matches Forge's ExileFromHand<N/Spec> and ExileFromGrave<N/Spec>
// tokens -- exiling a matching card from the named zone as a cost payment
// (CR 118.8 lists exiling a card from one's hand among the payment actions;
// the graveyard form is the encore family's "exile this card from your
// graveyard"). As with the other non-mana tokens the trailing
// "/description" is dropped here and ";" alternations fold to ",".
// ExileFromHand evoke costs (the MH3 evoke family: Fury, Grief, ...) and the
// AlternateAdditionalCost ExileFromGrave line are the corpus users.
var exileCost = regexp.MustCompile(`^ExileFrom(Hand|Grave)<(\d+)/([^/>]+)(?:/[^>]*)?>$`)

// addCounterCost matches Forge's AddCounter<N/LOYALTY> token -- the
// planeswalker loyalty cost, and deliberately ONLY it (CR 107.4: the [+N]
// symbol): adding loyalty counters is not a payment at all, so an AddCounter
// part is a FREE cost component -- [+2] costs no mana, and AddCounter<0/LOYALTY>
// (the [0] abilities, 53 raw lines) costs nothing either. The settle is
// rules/cast.go's ability branch beside the SubCounter settle. The corpus's
// other 8 AddCounter tokens (Devoted Druid's M1M1 untap, Wall of Roots' M0M1
// mana ability, two UnlessCost$ SVars) are NOT matched by this regex and keep
// today's one-generic fallback, per the brief's scope boundary -- their
// counter semantics (M1M1/M0M1 kinds, mid-resolution UnlessCost payers) are
// their own work.
var addCounterCost = regexp.MustCompile(`^AddCounter<(\d+)/(LOYALTY)(?:/[^>]*)?>$`)

// lifeCost matches Forge's fixed life-payment token. Dynamic values such as
// PayLife<X> retain the ordinary malformed-token fallback below: this engine
// has no source from which to resolve their value.
var lifeCost = regexp.MustCompile(`^PayLife<(\d+)>$`)

var choiceCost = regexp.MustCompile(`^(Reveal|Behold|tapXType)<(\d+)/([^/>]+)(?:/[^>]*)?>$`)
var blightCost = regexp.MustCompile(`^Blight<(\d+)>$`)

// payEnergyCost matches Forge's PayEnergy<N> and PayEnergy<X> tokens --
// removing N energy counters from the payer (CR 118.2d; Forge
// CostPayEnergy.canPay reads the payer's ENERGY counter total, and its
// getMaxAmountX bounds a dynamic PayEnergy<X> by that same total). The
// trailing "/description" Forge may append is dropped like every other
// head. The X form is recorded as a part with Spec "X": xAsk bounds the
// announced value by the payer's energy count and the settle spends exactly
// that many, so the announcement and the spend cannot disagree.
var payEnergyCost = regexp.MustCompile(`^PayEnergy<([0-9]+|X)(?:/[^>]*)?>$`)

// returnCost matches Forge's Return<N/Spec> tokens -- a permanent matching
// Spec returned to its OWNER's hand as the payment (Forge CostReturn's
// moveToHand; its payCostFromSource branch is a Spec of CARDNAME, the source
// itself -- Chthonian Nightmare's "Return Chthonian Nightmare to its owner's
// hand"). N is almost always 1 (94 corpus files carry the token; every
// parsed one is 1). The trailing description is dropped, ";"
// alternations fold to "," like every other non-mana head.
var returnCost = regexp.MustCompile(`^Return<(\d+)/([^/>]+)(?:/[^>]*)?>$`)

// ParseCost accepts both Forge's space-separated form ("2 U U") and the
// bracketed oracle form ("{2}{U}{U}"). "no cost" and "" are free.
func ParseCost(s string) Cost {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "no cost") {
		return Cost{}
	}
	s = strings.NewReplacer("{", " ", "}", " ").Replace(s)
	var c Cost
	for _, sym := range splitCostTokens(s) {
		switch {
		case sym == "T":
			c.Tap = true
		case sym == "X":
			c.X++
		case sym == "Forage":
			c.Forage = true
		case len(sym) == 1 && strings.ContainsAny(sym, "WUBRGC"):
			c.Colored[state.ManaIndex(sym[0])]++
		case isHybrid(sym):
			c.Hybrid = append(c.Hybrid, hybridPair(sym))
		case isPhyrexian(sym):
			c.Phyrexian = append(c.Phyrexian, phyrexianColor(sym))
		default:
			if m := choiceCost.FindStringSubmatch(sym); m != nil {
				n, err := strconv.ParseInt(m[2], 10, 64)
				if err != nil || n <= 0 || n > int64(math.MaxInt32) {
					// Same safe fallback as every other malformed cost token --
					// and REPORT it: the head is recognised, this instance is
					// not modelled.
					c.Generic = addClampedGeneric(c.Generic, 1)
					c.reportUnknown(sym)
					continue
				}
				part := CostPart{N: int32(n), Spec: strings.ReplaceAll(m[3], ";", ",")}
				switch m[1] {
				case "Reveal":
					c.Reveal = append(c.Reveal, part)
				case "Behold":
					c.Behold = append(c.Behold, part)
				default:
					c.TapPermanent = append(c.TapPermanent, part)
				}
				continue
			}
			if m := blightCost.FindStringSubmatch(sym); m != nil {
				n, err := strconv.ParseInt(m[1], 10, 64)
				if err != nil || n <= 0 || n > int64(math.MaxInt32) {
					// Same safe fallback as every other malformed cost token --
					// and REPORT it: the head is recognised, this instance is
					// not modelled.
					c.Generic = addClampedGeneric(c.Generic, 1)
					c.reportUnknown(sym)
					continue
				}
				c.Blight = append(c.Blight, CostPart{N: int32(n), Spec: "Creature.YouCtrl"})
				continue
			}
			if m := lifeCost.FindStringSubmatch(sym); m != nil {
				n, err := strconv.ParseInt(m[1], 10, 64)
				if err != nil || n < 0 || n > int64(math.MaxInt32) {
					// Keep an out-of-range PayLife token on the same safe fallback
					// as every other malformed cost token -- and REPORT it: the
					// head is recognised, this instance is not modelled.
					c.Generic = addClampedGeneric(c.Generic, 1)
					c.reportUnknown(sym)
					continue
				}
				c.Life = addClampedGeneric(c.Life, n)
				continue
			}
			if m := nonManaCost.FindStringSubmatch(sym); m != nil {
				n, err := strconv.ParseInt(m[2], 10, 64)
				if err != nil || n < 0 || n > int64(math.MaxInt32) {
					// A malformed Sac/Discard/SubCounter token degrades the same way
					// an unrecognised mana token does: one generic mana,
					// never a hard parse error -- and is reported (the head is
					// recognised, this instance is not modelled).
					c.Generic = addClampedGeneric(c.Generic, 1)
					c.reportUnknown(sym)
					continue
				}
				// Fold Forge's ";" OR alternation into the "," MatchesSpec
				// already uses, so "Artifact;Creature" matches either.
				spec := strings.ReplaceAll(m[3], ";", ",")
				part := CostPart{N: int32(n), Spec: spec}
				switch m[1] {
				case "Sac":
					c.Sac = append(c.Sac, part)
				case "Discard":
					c.Discard = append(c.Discard, part)
				default:
					c.SubCounter = append(c.SubCounter, part)
				}
				continue
			}
			if m := exileCost.FindStringSubmatch(sym); m != nil {
				n, err := strconv.ParseInt(m[2], 10, 64)
				if err != nil || n < 0 || n > int64(math.MaxInt32) {
					// Same safe fallback as every other malformed cost token --
					// and REPORT it: the head is recognised, this instance is
					// not modelled.
					c.Generic = addClampedGeneric(c.Generic, 1)
					c.reportUnknown(sym)
					continue
				}
				spec := strings.ReplaceAll(m[3], ";", ",")
				part := CostPart{N: int32(n), Spec: spec}
				if m[1] == "Grave" {
					part.Zone = state.ZGraveyard
				}
				c.Exile = append(c.Exile, part)
				continue
			}
			if m := addCounterCost.FindStringSubmatch(sym); m != nil {
				n, err := strconv.ParseInt(m[1], 10, 64)
				if err != nil || n < 0 || n > int64(math.MaxInt32) {
					// Same degrade-to-one-generic fallback as the other
					// malformed tokens -- and reported for the same reason.
					c.Generic = addClampedGeneric(c.Generic, 1)
					c.reportUnknown(sym)
					continue
				}
				spec := strings.ReplaceAll(m[2], ";", ",")
				c.AddCounter = append(c.AddCounter, CostPart{N: int32(n), Spec: spec})
				continue
			}
			if m := sacXCost.FindStringSubmatch(sym); m != nil {
				spec := strings.ReplaceAll(m[1], ";", ",")
				c.Sac = append(c.Sac, CostPart{Spec: spec, Announced: true})
				continue
			}
			if m := payEnergyCost.FindStringSubmatch(sym); m != nil {
				if m[1] == "X" {
					// The dynamic form: the SAME X the cast announces.
					c.Energy = append(c.Energy, CostPart{Spec: "X"})
					continue
				}
				n, err := strconv.ParseInt(m[1], 10, 64)
				if err != nil || n < 0 || n > int64(math.MaxInt32) {
					// Same safe fallback as every other malformed cost token --
					// and REPORT it: the head is recognised, this instance is
					// not modelled.
					c.Generic = addClampedGeneric(c.Generic, 1)
					c.reportUnknown(sym)
					continue
				}
				c.Energy = append(c.Energy, CostPart{N: int32(n)})
				continue
			}
			if m := returnCost.FindStringSubmatch(sym); m != nil {
				n, err := strconv.ParseInt(m[1], 10, 64)
				if err != nil || n < 0 || n > int64(math.MaxInt32) {
					// Same safe fallback as every other malformed cost token --
					// and REPORT it: the head is recognised, this instance is
					// not modelled.
					c.Generic = addClampedGeneric(c.Generic, 1)
					c.reportUnknown(sym)
					continue
				}
				spec := strings.ReplaceAll(m[2], ";", ",")
				c.Return = append(c.Return, CostPart{N: int32(n), Spec: spec})
				continue
			}
			// Try to parse as a numeric token. Negative and out-of-range values
			// fall through to the +1 generic fallback.
			if n, err := strconv.ParseInt(sym, 10, 64); err == nil && n >= 0 && n <= int64(math.MaxInt32) {
				c.Generic = addClampedGeneric(c.Generic, n)
				continue
			}
			// An unrecognised symbol (including a malformed hybrid/Phyrexian
			// token) degrades to one generic mana, never a hard parse error -- and
			// is REPORTED as unmodelled (Cost.Unknown), so the parameter census
			// can name it instead of the substitution staying silent.
			c.reportUnknown(sym)
			c.Generic = addClampedGeneric(c.Generic, 1)
		}
	}
	return c
}

// reportUnknown records the head of one degraded cost token in Unknown,
// in order, deduplicated. Used by the final unrecognised-symbol fallback AND
// by the malformed-instance branches of the recognised heads: both shapes
// priced one generic without real semantics, so both are unmodelled.
func (c *Cost) reportUnknown(sym string) {
	if i := strings.IndexByte(sym, '<'); i > 0 {
		sym = sym[:i]
	}
	if !slices.Contains(c.Unknown, sym) {
		c.Unknown = append(c.Unknown, sym)
	}
}

// splitCostTokens splits a cost string on whitespace, but keeps each <...>
// group atomic so Forge's non-mana tokens -- whose trailing "/description"
// can contain spaces -- are not torn apart by a plain Fields split before
// nonManaCost can see them. Ruling FL-54.
func splitCostTokens(s string) []string {
	var out []string
	var cur strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '<':
			depth++
			cur.WriteRune(r)
		case r == '>':
			if depth > 0 {
				depth--
			}
			cur.WriteRune(r)
		case unicode.IsSpace(r) && depth == 0:
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// addClampedGeneric adds n to the int32 generic count, saturating at
// math.MaxInt32 on top and refusing to go below zero, so no accumulation of
// numeric tokens (nor a WithX fold) can ever wrap Generic negative. Task 20.
func addClampedGeneric(v int32, n int64) int32 {
	total := int64(v) + n
	if total > math.MaxInt32 {
		return math.MaxInt32
	}
	if total < 0 {
		return 0
	}
	return int32(total)
}

// isHybrid reports whether sym is a two-colour hybrid pip: either the
// slash form ("W/U") or the concatenated form ("GW", "WB"). A second
// character of 'P' is Phyrexian, not hybrid, and is handled by
// isPhyrexian. Both letters must be distinct WUBRG colours (a doubled
// letter, "WW", is not a hybrid — it is a script typo and degrades to
// generic).
func isHybrid(sym string) bool {
	var a, b byte
	if len(sym) == 3 && sym[1] == '/' {
		a, b = sym[0], sym[2]
	} else if len(sym) == 2 && sym[1] != 'P' {
		a, b = sym[0], sym[1]
	} else {
		return false
	}
	return a != b && strings.ContainsRune("WUBRG", rune(a)) && strings.ContainsRune("WUBRG", rune(b))
}

// hybridPair normalises a hybrid symbol to its two colours as a ManaPair.
// sym is guaranteed a hybrid by isHybrid.
func hybridPair(sym string) ManaPair {
	if len(sym) == 3 && sym[1] == '/' {
		return ManaPair{A: sym[0], B: sym[2]}
	}
	return ManaPair{A: sym[0], B: sym[1]}
}

// isPhyrexian reports whether sym is a Phyrexian pip: a WUBRG colour
// followed by 'P', either slash ("W/P") or concatenated ("UP", "BP").
// CR 107.4f: pay that colour OR two life.
func isPhyrexian(sym string) bool {
	if len(sym) == 3 && sym[1] == '/' {
		return strings.ContainsRune("WUBRG", rune(sym[0])) && sym[2] == 'P'
	}
	if len(sym) != 2 {
		return false
	}
	return sym[1] == 'P' && strings.ContainsRune("WUBRG", rune(sym[0]))
}

// phyrexianColor returns the colour letter of a Phyrexian pip. sym is
// guaranteed a Phyrexian pip by isPhyrexian.
func phyrexianColor(sym string) byte {
	return sym[0]
}

func (c Cost) CMC() int32 {
	return c.Colored.Total() + c.Generic + int32(len(c.Hybrid)) + int32(len(c.Phyrexian))
}

// WithX folds a chosen X value into Generic, once per X symbol the cost
// carried, then clears X: once a value is chosen, {X} is no longer a
// distinct requirement, it is simply that much more generic mana.
func (c Cost) WithX(x int32) Cost {
	c.Generic = addClampedGeneric(c.Generic, int64(c.X)*int64(x))
	c.X = 0
	return c
}

// Plus sums two costs (Kicker's own cost added to the card's printed cost):
// colours, generic and life add, X counts add, Tap ORs, and each side's
// non-mana parts concatenate.
func (c Cost) Plus(d Cost) Cost {
	for i := range c.Colored {
		c.Colored[i] += d.Colored[i]
	}
	c.Generic += d.Generic
	c.Life = addClampedGeneric(c.Life, int64(d.Life))
	c.X += d.X
	c.Tap = c.Tap || d.Tap
	if len(d.Hybrid) > 0 {
		c.Hybrid = append(append([]ManaPair(nil), c.Hybrid...), d.Hybrid...)
	}
	if len(d.Phyrexian) > 0 {
		c.Phyrexian = append(append([]byte(nil), c.Phyrexian...), d.Phyrexian...)
	}
	if len(d.Sac) > 0 {
		c.Sac = append(append([]CostPart(nil), c.Sac...), d.Sac...)
	}
	if len(d.Discard) > 0 {
		c.Discard = append(append([]CostPart(nil), c.Discard...), d.Discard...)
	}
	if len(d.SubCounter) > 0 {
		c.SubCounter = append(append([]CostPart(nil), c.SubCounter...), d.SubCounter...)
	}
	if len(d.AddCounter) > 0 {
		c.AddCounter = append(append([]CostPart(nil), c.AddCounter...), d.AddCounter...)
	}
	if len(d.Exile) > 0 {
		c.Exile = append(append([]CostPart(nil), c.Exile...), d.Exile...)
	}
	if len(d.Reveal) > 0 {
		c.Reveal = append(append([]CostPart(nil), c.Reveal...), d.Reveal...)
	}
	if len(d.Behold) > 0 {
		c.Behold = append(append([]CostPart(nil), c.Behold...), d.Behold...)
	}
	if len(d.TapPermanent) > 0 {
		c.TapPermanent = append(append([]CostPart(nil), c.TapPermanent...), d.TapPermanent...)
	}
	if len(d.Blight) > 0 {
		c.Blight = append(append([]CostPart(nil), c.Blight...), d.Blight...)
	}
	if len(d.Energy) > 0 {
		c.Energy = append(append([]CostPart(nil), c.Energy...), d.Energy...)
	}
	if len(d.Return) > 0 {
		c.Return = append(append([]CostPart(nil), c.Return...), d.Return...)
	}
	c.Forage = c.Forage || d.Forage
	return c
}

// commanderTaxFor composes base with the CR 903.8 commander tax: an
// additional generic {2} -- on top of everything else -- for each previous
// time id has been cast from the command zone this game. It is the ONE place
// a command-zone cast's extra cost is composed, and BOTH sides of the
// two-sided gate apply it to the same base: the legality offer (the
// command-zone walk in rules/legal.go calls commanderTaxFor(base) before
// castable) and the payment (rules/cast.go's beginCast applies
// commanderTaxFor to the cost it resolves). Because both derive the taxed
// cost from this same function over
// the same board, an offered command-zone cast and the cost it charges are
// structurally incapable of disagreeing -- the no-progress spin
// stalledCastLimit bounds is exactly what that disagreement used to cause.
//
// base is expected to be the already-reduced output of adjustedCost (kicker/
// surge/flashback recast it afterwards, but a card in the command zone is
// only ever cast here by its plain cost). Applying the tax to base means cost
// reductions land BEFORE it and never spill onto it: an additional cost is
// outside a plain "cost to cast" reduction, exactly how Kicker's own
// additional {N} composes in beginCast. Colored requirements are untouched.
//
// The Commander-format gate is explicit, not incidental: outside the format
// -- even when a card sits in the (usually empty) command zone -- base passes
// through unchanged. A card that is not its owner's commander, or a commander
// no longer in the command zone, is likewise untaxed.
func (e *Engine) commanderTaxFor(p state.PlayerID, id state.ObjID, base Cost) Cost {
	base.Generic += e.commanderTaxAmount(p, id)
	return base
}

// commanderTaxAmount is the CR 903.8 commander-tax generic amount for a
// command-zone commander cast by p: 2 per prior command-zone cast of id, 0
// for every other card, zone or format. It is the single O(1) tax read; both
// commanderTaxFor (the offer side) and beginCast's taxGeneric capture use it.
func (e *Engine) commanderTaxAmount(p state.PlayerID, id state.ObjID) int32 {
	if e.format != FormatCommander {
		return 0
	}
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZCommand {
		return 0
	}
	for k, cid := range e.G.Players[p].Commanders {
		if cid != id {
			continue
		}
		if n := e.G.Players[p].CmdCasts[k]; n > 0 {
			return 2 * n
		}
		return 0
	}
	return 0
}

// rawBaseCost is id's printed mana cost, without any cost modifier applied:
// the CR 601.2f "mana cost or alternative cost" basis onto which the chosen
// {X} and the RaiseCost/ReduceCost composition (manaToPay) are built. A
// missing object or a Face()-less one degrades to the zero Cost rather than
// panicking, matching adjustedCost's own guard.
func (e *Engine) rawBaseCost(p state.PlayerID, id state.ObjID) Cost {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return Cost{}
	}
	return ParseCost(o.Face().ManaCost)
}

// offerCostFor is the CR 601.2f-composed cost an offer is gated on: the
// selected base cost (a spell's printed mana cost, or an alternative/
// flashback/surge/kicker cost) with RaiseCost then ReduceCost applied to
// Generic, and (for a spell) the CR 903.8 commander tax added last because an
// additional cost is never reduced. {X} is not yet chosen at offer time, so it
// contributes zero generic here and is not reduced; the offer stays
// conservative (a spell offering itself is withheld only when even X=0 is
// unpayable) while the actual charge (manaToPay) applies the modifiers after
// X is folded -- the two never disagree on a card with no {X} in its cost.
func (e *Engine) offerCostFor(p state.PlayerID, id state.ObjID, base Cost, ability bool) Cost {
	kind := "Spell"
	if ability {
		kind = "Ability"
	}
	c := base
	raise, reduce := e.costModifiers(p, id, kind)
	c.Generic += raise
	c.Generic -= reduce
	if c.Generic < 0 {
		c.Generic = 0
	}
	if !ability {
		c = e.commanderTaxFor(p, id, c)
	}
	return c
}

// AbilityCosts returns id's non-mana activated-ability costs after the same
// offer-time RaiseCost/ReduceCost composition legalActions applies. The order
// is the face's authored ability order. This is a projection helper: it emits
// no event and mutates no game state.
func (e *Engine) AbilityCosts(p state.PlayerID, id state.ObjID) []string {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return nil
	}
	var out []string
	for _, ab := range o.Face().Abilities {
		if ab.Kind != "AB" || ab.API == "Mana" {
			continue
		}
		out = append(out, formatCost(e.offerCostFor(p, id, ParseCost(ab.Params["Cost"]), true)))
	}
	return out
}

// formatCost writes the parsed cost back in the whitespace-delimited Forge
// notation understood by the client. Parsing first is intentional: malformed
// tokens retain the engine's real conservative one-generic interpretation,
// and every recognised non-mana component remains visible so a client that
// cannot price it can still fail closed.
func formatCost(c Cost) string {
	var parts []string
	if c.Generic > 0 {
		parts = append(parts, strconv.FormatInt(int64(c.Generic), 10))
	}
	const faces = "WUBRGC"
	for i, face := range []byte(faces) {
		for n := int32(0); n < c.Colored[i]; n++ {
			parts = append(parts, string(face))
		}
	}
	for range c.X {
		parts = append(parts, "X")
	}
	for _, h := range c.Hybrid {
		parts = append(parts, string([]byte{h.A, '/', h.B}))
	}
	for _, p := range c.Phyrexian {
		parts = append(parts, string([]byte{p, 'P'}))
	}
	if c.Life > 0 {
		parts = append(parts, "PayLife<"+strconv.FormatInt(int64(c.Life), 10)+">")
	}
	if c.Tap {
		parts = append(parts, "T")
	}
	for _, part := range c.Energy {
		if part.Spec == "X" {
			parts = append(parts, "PayEnergy<X>")
		} else {
			parts = append(parts, "PayEnergy<"+strconv.FormatInt(int64(part.N), 10)+">")
		}
	}
	appendCostParts := func(kind string, costs []CostPart) {
		for _, part := range costs {
			parts = append(parts, kind+"<"+strconv.FormatInt(int64(part.N), 10)+"/"+part.Spec+">")
		}
	}
	appendCostParts("Sac", c.Sac)
	appendCostParts("Discard", c.Discard)
	appendCostParts("SubCounter", c.SubCounter)
	appendCostParts("AddCounter", c.AddCounter)
	for _, part := range c.Exile {
		from := "Hand"
		if part.Zone == state.ZGraveyard {
			from = "Grave"
		}
		parts = append(parts, "ExileFrom"+from+"<"+strconv.FormatInt(int64(part.N), 10)+"/"+part.Spec+">")
	}
	appendCostParts("Reveal", c.Reveal)
	appendCostParts("Behold", c.Behold)
	appendCostParts("tapXType", c.TapPermanent)
	for _, part := range c.Blight {
		parts = append(parts, "Blight<"+strconv.FormatInt(int64(part.N), 10)+">")
	}
	appendCostParts("Return", c.Return)
	if c.Forage {
		parts = append(parts, "Forage")
	}
	return strings.Join(parts, " ")
}

// HasNonMana reports whether paying this cost takes more than mana.
// AddCounter counts (the part is settled by the cast flow beside SubCounter,
// even though it takes no payment), so a caller using this to skip the
// cast-flow stages is told the truth.
func (c Cost) HasNonMana() bool {
	return c.Life > 0 || c.Tap || len(c.Sac) > 0 || len(c.Discard) > 0 || len(c.SubCounter) > 0 || len(c.AddCounter) > 0 || len(c.Exile) > 0 || len(c.Reveal) > 0 || len(c.Behold) > 0 || len(c.TapPermanent) > 0 || len(c.Blight) > 0 || c.Forage || len(c.Energy) > 0 || len(c.Return) > 0
}

// costModifiers reports the RaiseCost and ReduceCost generic-mana amounts
// that apply, per CR 601.2f, to a cost paid by p for the object id, with the
// announced {X} not yet chosen (bound 0 -- an offer-time read).
func (e *Engine) costModifiers(p state.PlayerID, id state.ObjID, kind string) (raise, reduce int32) {
	return e.costModifiersX(p, id, kind, 0)
}

// costModifiersX is costModifiers with the announced {X} value bound, for the
// in-cast recomputation manaToPay does when the cost announces a variable
// sacrifice count (Sac<X/Spec>): Dargo's {2}-less-per-sacrifice ReduceCost
// statics read the paid X, which does not exist at offer time. kind is
// "Spell" (a cast) or "Ability" (an activation): a static whose Type$ names
// the other kind is skipped, and one naming neither applies to both. This
// engine's RaiseCost/ReduceCost only ever touch the Generic component (never
// Colored). Increases and reductions are returned separately so the caller
// can apply them in the CR 601.2f order (increases before reductions).
func (e *Engine) costModifiersX(p state.PlayerID, id state.ObjID, kind string, x int32) (raise, reduce int32) {
	for _, mode := range []string{"RaiseCost", "ReduceCost"} {
		for _, sv := range e.activeStatics(mode) {
			if !e.costActorMatches(sv, p) {
				continue
			}
			if ty, ok := sv.Params["Type"]; ok && ty != "" && ty != kind {
				continue
			}
			if !effects.MatchesSpecCtx(e.G, sv.Params["ValidCard"], id, e.specCtx(sv.Source, p)) {
				continue
			}
			// An unresolvable amount degrades asymmetrically: an unexplained
			// RAISE prices the cost up (conservative), an unexplained REDUCE
			// applies nothing (fail closed -- an invented {1} discount is a
			// game-action error, an invented {1} surcharge is only an offer
			// the seat may decline).
			amt := e.staticAmount(sv, p, x, costModifierDef(mode))
			if mode == "RaiseCost" {
				raise += amt
			} else {
				reduce += amt
			}
		}
		// The spell BEING CAST prices its own face statics: a self-scoped
		// ReduceCost (Dargo's "costs {2} less for each permanent sacrificed
		// this way", ValidCard$ Card.Self) sits on the spell's own face, and
		// the spell's statics function while it is on the stack (CR 604.3's
		// battlefield rule does not reach a spell's own cast composition --
		// CR 601.2f's total cost is composed from the announced spell's own
		// static). A battlefield scan can never see a hand/stack card, so the
		// offer gate prices the un-reduced cost and the in-cast recompute
		// (manaToPay, with the announced X) applies the real reduction.
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Zone == state.ZStack {
			for _, st := range o.Face().Statics {
				if st.Mode != mode {
					continue
				}
				sv := staticView{Source: id, Controller: p, Params: st.Params}
				if !e.costActorMatches(sv, p) {
					continue
				}
				if ty, ok := sv.Params["Type"]; ok && ty != "" && ty != kind {
					continue
				}
				if !effects.MatchesSpecCtx(e.G, sv.Params["ValidCard"], id, e.specCtx(sv.Source, p)) {
					continue
				}
				amt := e.staticAmount(sv, p, x, costModifierDef(mode))
				if mode == "RaiseCost" {
					raise += amt
				} else {
					reduce += amt
				}
			}
		}
	}
	return
}

// costModifierDef is the degrade-to value an unresolvable cost-modifier
// Amount$ falls back to: RaiseCost surcharges {1} (the conservative reading
// parseAmount always applied), ReduceCost discounts nothing.
func costModifierDef(mode string) int32 {
	if mode == "RaiseCost" {
		return 1
	}
	return 0
}

// staticAmount resolves one cost-modifier static's Amount$ parameter. A plain
// integer is that integer (parseAmount's shape). Anything else is a face-SVar
// reference in one of two spellings -- a bare name (Herald of War's
// "Amount$ X" over SVar:X:Count$CardCounters.P1P1) or Forge's
// "SVar$Name[/Op]" (Dargo's "Amount$ Y" over SVar:Y:SVar$X/Times.2) --
// resolved through the shared count evaluator with the announced {X} bound
// to the context (Count$xPaid reads it) and the /Op suffix applied through
// the SAME arithmetic effects.ApplyCountOp wraps. An unresolvable amount
// (no SVar, a name the face does not carry, a negative result) degrades to
// def exactly as parseAmount did, never a hard failure.
func (e *Engine) staticAmount(sv staticView, p state.PlayerID, x int32, def int32) int32 {
	raw := strings.TrimSpace(sv.Params["Amount"])
	if raw == "" {
		return def
	}
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil && n >= 0 && n <= math.MaxInt32 {
		return int32(n)
	}
	n, ok := e.evalAmountRef(sv, p, x, raw, 0)
	if !ok || n < 0 || n > math.MaxInt32 {
		return def
	}
	return n
}

// evalAmountRef resolves one Amount$ reference recursively: a bare face-SVar
// name, Forge's "SVar$Name[/Op]" spelling, or a Count$ body (whose own /Op
// EvalCount already applies -- never double-applied here). depth bounds the
// SVar-to-SVar chase so a cyclic table terminates deterministically.
func (e *Engine) evalAmountRef(sv staticView, p state.PlayerID, x int32, expr string, depth int) (int32, bool) {
	if depth > 4 {
		return 0, false
	}
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, false
	}
	if inner, ok := strings.CutPrefix(expr, "SVar$"); ok {
		// Forge's SVar-reference form: the op after the "/" belongs to THIS
		// reference, applied after the named SVar's own value resolves.
		ref, op, _ := strings.Cut(inner, "/")
		n, ok := e.evalAmountRef(sv, p, x, strings.TrimSpace(ref), depth+1)
		if !ok {
			return 0, false
		}
		return effects.ApplyCountOp(n, op), true
	}
	if strings.HasPrefix(expr, "Count$") {
		// The count grammar owns its own /Op -- hand the whole body over.
		o := e.G.Obj(sv.Source)
		if o == nil || o.Face() == nil {
			return 0, false
		}
		f := o.Face()
		ctx := &effects.Ctx{Source: sv.Source, Controller: p, SVars: f.SVars, X: x}
		return effects.EvalCount(e, ctx, expr), true
	}
	o := e.G.Obj(sv.Source)
	if o == nil || o.Face() == nil {
		return 0, false
	}
	f := o.Face()
	if body, ok := f.SVars[expr]; ok {
		return e.evalAmountRef(sv, p, x, body, depth+1)
	}
	return 0, false
}

// costActorMatches is the cost-modifier actor gate: a RaiseCost/ReduceCost
// static with an Activator$ or Caster$ parameter scopes to whose cost it
// modifies. With neither it applies regardless of actor.
func (e *Engine) costActorMatches(sv staticView, actor state.PlayerID) bool {
	if _, ok := sv.Params["Activator"]; ok {
		return e.actorMatches(sv, "Activator", actor)
	}
	if _, ok := sv.Params["Caster"]; ok {
		return e.actorMatches(sv, "Caster", actor)
	}
	return true
}

// Priceable reports whether payMana can actually charge every part of this
// cost. payMana charges Colored and Generic from the pool and fixed Life from
// the payer, but an {X} component that has not been folded into Generic, or a
// Tap/Sac/Discard/SubCounter component, still needs cast-flow handling. Such a cost
// is unpriceable by the mid-resolution payment API and must be DECLINED,
// never silently priced at zero. This is the predicate I-5 routes through:
// every component ParseCost collapses into a shape payMana cannot charge --
// an SVar-sourced X, a cast-time-chosen X, or a Tap/Sac/Discard/SubCounter part --
// travels through the same predicate rather than a `if cost == "X"` special
// case.
//
// Cost.Pay remains pool-only, while Priceable is the "is this chargeable by
// payMana with a payer" question the mid-resolution unless-pay answer asks
// before trusting the pool and life total.
func (c Cost) Priceable() bool {
	return c.X == 0 && !c.Tap && len(c.Sac) == 0 && len(c.Discard) == 0 && len(c.SubCounter) == 0 &&
		len(c.Exile) == 0 && len(c.Reveal) == 0 && len(c.Behold) == 0 && len(c.TapPermanent) == 0 &&
		len(c.Blight) == 0 && !c.Forage && len(c.Hybrid) == 0 && len(c.Phyrexian) == 0 &&
		len(c.Energy) == 0 && len(c.Return) == 0
}

// pip is one coloured-or-flexible demand inside a cost's mana part: the set
// of acceptable colours, and whether the pip may alternatively be paid with
// two life (a Phyrexian pip). n is always 1 for the pips this engine builds
// from a cost; the field exists so a caller that expands a multi-count
// coloured requirement can reuse the same struct.
type pip struct {
	colors [2]byte
	n      int
	lifeOK bool
}

// costPips expands a cost's Colored, Hybrid and Phyrexian parts into a flat
// pip list, in that order (exact colours first, then hybrids, then
// Phyrexian). A coloured pip accepts exactly its own colour; a hybrid accepts
// either of its pair; a Phyrexian pip accepts its colour or two life.
func (c Cost) costPips() []pip {
	var out []pip
	// The coloured slots including the colourless one: a plain {C} pip is a
	// strict colourless requirement generic must not satisfy by stealing the
	// pool's only colourless, so it is reserved like any coloured pip.
	for _, letter := range []byte{'W', 'U', 'B', 'R', 'G', 'C'} {
		for n := c.Colored[state.ManaIndex(letter)]; n > 0; n-- {
			out = append(out, pip{colors: [2]byte{letter, letter}, n: 1})
		}
	}
	for _, pair := range c.Hybrid {
		out = append(out, pip{colors: [2]byte{pair.A, pair.B}, n: 1})
	}
	for _, letter := range c.Phyrexian {
		out = append(out, pip{colors: [2]byte{letter, letter}, n: 1, lifeOK: true})
	}
	return out
}

// resolveMana finds a concrete payment of the cost's mana and fixed-life
// parts from pool and the payer's life, preferring to spend coloured pool mana
// over life for a Phyrexian pip and preferring the first colour of a hybrid
// pair, so the assignment is deterministic. It returns the pool with the
// coloured pips spent, total life spent (the fixed Life component plus two
// for each Phyrexian pip paid by life), and whether the whole cost is payable.
// The generic requirement is paid last from whatever the pips left, so
// coloured mana is never spent on generic while a pip still needs it.
func (c Cost) resolveMana(pool state.Mana, life int32) (state.Mana, int32, bool) {
	if life < c.Life {
		return pool, 0, false
	}
	pips := c.costPips()
	rem := pool
	life -= c.Life
	lifeSpent := c.Life
	var rec func(i int) bool
	rec = func(i int) bool {
		if i == len(pips) {
			return rem.Total() >= c.Generic
		}
		p := pips[i]
		// Try each acceptable colour, in the order given. For a hybrid this
		// prefers A over B; for a single-colour pip A==B so it is just once.
		for _, col := range p.colors {
			di := state.ManaIndex(col)
			if rem[di] > 0 {
				rem[di]--
				if rec(i + 1) {
					return true
				}
				rem[di]++
			}
		}
		if p.lifeOK && life >= 2 {
			life -= 2
			lifeSpent += 2
			if rec(i + 1) {
				return true
			}
			lifeSpent -= 2
			life += 2
		}
		return false
	}
	if !rec(0) {
		return pool, 0, false
	}
	// The search found a pip assignment that leaves enough total mana; deduct
	// the generic requirement from that remainder, preferring colourless then
	// colours in fixed WUBRG order so payment is deterministic. Generic can
	// be paid by any leftover mana, so a total >= Generic always suffices.
	need := c.Generic
	for _, i := range [...]int{state.MC, state.MW, state.MU, state.MB, state.MR, state.MG} {
		for need > 0 && rem[i] > 0 {
			rem[i]--
			need--
		}
	}
	return rem, lifeSpent, true
}

// payable reports whether the cost's mana and fixed-life parts can be paid
// by pool and the payer's current life (a Phyrexian pip may additionally be
// paid with two life). This is the offering gate's feasibility question, and
// the real answer to "is there ANY way this cost can be paid right now" -- the
// same resolveMana the payment stage uses, so an offered cost and the cost it
// charges can never disagree.
func (c Cost) payable(pool state.Mana, life int32) bool {
	_, _, ok := c.resolveMana(pool, life)
	return ok
}

func (c Cost) CanPay(p state.Mana) bool {
	// Pool-only feasibility, no life offered: a hybrid must be paid by one of
	// its colours in the pool, a Phyrexian pip by its colour. This is the
	// pure pricing question the corpus invariants ask, and it never treats a
	// hybrid as generic nor lets colourless `pay` it.
	_, _, ok := c.resolveMana(p, 0)
	return ok
}

// Pay spends the cost from a pool and returns what is left. Coloured
// requirements come out first so generic can never strand a colour the cost
// still needs; hybrid pips take one of their pair and Phyrexian pips their
// colour (pool-only -- the cast flow's payMana handles the life half and
// passes a fully-resolved cost here). Mana-only: non-mana parts
// (Tap/Sac/Discard/SubCounter) are the cast flow's own job (rules/cast.go), never
// this function's.
func (c Cost) Pay(p state.Mana) (state.Mana, bool) {
	// Pool-only: no life is offered, so a Phyrexian pip is paid by its colour
	// (the cast flow's payMana handles the life half and passes a fully
	// resolved cost here). resolveMana already reserves the coloured pips and
	// deducts generic, so the returned pool is fully spent.
	out, _, ok := c.resolveMana(p, 0)
	return out, ok
}
