package rules

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/adams-shaun/gorge/state"
)

// CostPart is one non-mana cost component: Sac<N/Spec> (sacrifice N
// permanents matching Spec), Discard<N/Spec> (discard N matching cards), or
// SubCounter<N/Kind> (remove N counters of Kind from the source).
type CostPart struct {
	N    int32
	Spec string
}

// ManaPair is one two-face hybrid symbol: each face is a WUBRGC mana symbol,
// and either one spells the pip (CR 107.4e).
type ManaPair struct{ A, B byte }

// Twobrid is one monocolour hybrid symbol (CR 107.4e, Forge's `2B` shape):
// it may be paid with Generic generic mana OR with one mana of colour Col.
type Twobrid struct {
	Generic int32
	Col     byte
}

// HybridPhyrexian is one three-part symbol (Forge's `GWP` shape, CR 107.4f):
// it may be paid with one mana of colour A, one of colour B, or two life.
type HybridPhyrexian struct{ A, B byte }

// Cost is a parsed cost. X counts how many "X" symbols appeared (almost
// always 0 or 1; WithX folds a chosen value into Generic once per symbol).
// Life, Tap, Sac, Discard and SubCounter are non-mana components a cast or
// activation must satisfy separately from mana payment; AddCounter<N/LOYALTY>
// is a free non-mana component (a planeswalker's [+N] loyalty gain) settled
// beside SubCounter by the ability branch in rules/cast.go. Life is paid through
// payMana's LifeChange event; Tap, Sac, Discard and SubCounter are settled by
// the cast-flow stages in rules/cast.go. Pay and CanPay remain pool-only helpers.
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
	Colored         state.Mana
	Generic         int32
	Life            int32
	X               int
	Hybrid          []ManaPair
	Phyrexian       []byte
	Twobrid         []Twobrid
	HybridPhyrexian []HybridPhyrexian
	Snow            int32
	Tap             bool
	Sac             []CostPart
	Discard         []CostPart
	SubCounter      []CostPart
	AddCounter      []CostPart
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
		case sym == "S":
			// CR 107.4h: snow mana. One pip, payable only by a mana a snow
			// permanent produced (rules/mana.go's resolveMana pays it from the
			// parallel snow tally, never plain pool mana).
			c.Snow++
		case len(sym) == 1 && strings.ContainsAny(sym, "WUBRGC"):
			c.Colored[state.ManaIndex(sym[0])]++
		case isHybrid(sym):
			c.Hybrid = append(c.Hybrid, hybridPair(sym))
		case isPhyrexian(sym):
			c.Phyrexian = append(c.Phyrexian, phyrexianColor(sym))
		case isTwobrid(sym):
			c.Twobrid = append(c.Twobrid, twobridPair(sym))
		case isHybridPhyrexian(sym):
			c.HybridPhyrexian = append(c.HybridPhyrexian, hybridPhyrexianPair(sym))
		default:
			if m := lifeCost.FindStringSubmatch(sym); m != nil {
				n, err := strconv.ParseInt(m[1], 10, 64)
				if err != nil || n < 0 || n > int64(math.MaxInt32) {
					// Keep an out-of-range PayLife token on the same safe fallback
					// as every other malformed cost token.
					c.Generic = addClampedGeneric(c.Generic, 1)
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
					// never a hard parse error.
					c.Generic = addClampedGeneric(c.Generic, 1)
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
			if m := addCounterCost.FindStringSubmatch(sym); m != nil {
				n, err := strconv.ParseInt(m[1], 10, 64)
				if err != nil || n < 0 || n > int64(math.MaxInt32) {
					// Same degrade-to-one-generic fallback as the other
					// malformed tokens.
					c.Generic = addClampedGeneric(c.Generic, 1)
					continue
				}
				spec := strings.ReplaceAll(m[2], ";", ",")
				c.AddCounter = append(c.AddCounter, CostPart{N: int32(n), Spec: spec})
				continue
			}
			// Try to parse as a numeric token. Negative and out-of-range values
			// fall through to the +1 generic fallback.
			if n, err := strconv.ParseInt(sym, 10, 64); err == nil && n >= 0 && n <= int64(math.MaxInt32) {
				c.Generic = addClampedGeneric(c.Generic, n)
				continue
			}
			// An unrecognised symbol (including a malformed hybrid/Phyrexian
			// token) degrades to one generic mana, never a hard parse error.
			c.Generic = addClampedGeneric(c.Generic, 1)
		}
	}
	return c
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
// isPhyrexian. Both faces must be distinct WUBRGC mana symbols (a doubled
// letter, "WW", is not a hybrid — it is a script typo and degrades to
// generic). This includes colourless hybrid, such as {C/W}.
func isHybrid(sym string) bool {
	var a, b byte
	if len(sym) == 3 && sym[1] == '/' {
		a, b = sym[0], sym[2]
	} else if len(sym) == 2 && sym[1] != 'P' {
		a, b = sym[0], sym[1]
	} else {
		return false
	}
	return a != b && strings.ContainsRune("WUBRGC", rune(a)) && strings.ContainsRune("WUBRGC", rune(b))
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

// isTwobrid reports whether sym is a monocolour hybrid pip (CR 107.4e):
// a positive number followed by one colour — Forge's concatenated `2B`, or
// the slash form `2/B`. It may be paid with that many generic mana OR one
// mana of the colour. A `2/C` (the colourless-hybrid shape the client
// knows) parses the same way; the corpus carries none (measured), so the
// shape is parse-for-parity, not corpus-driven.
func isTwobrid(sym string) bool {
	a, b, ok := splitHybridSlash(sym)
	if ok {
		return isDigitRun(a) && len(b) == 1 && strings.ContainsRune("WUBRGC", rune(b[0]))
	}
	// Concatenated form: leading digits then exactly one colour letter.
	i := 0
	for i < len(sym) && sym[i] >= '0' && sym[i] <= '9' {
		i++
	}
	return i > 0 && i == len(sym)-1 && strings.ContainsRune("WUBRGC", rune(sym[i]))
}

// twobridPair normalises a monocolour hybrid symbol. sym is guaranteed by
// isTwobrid.
func twobridPair(sym string) Twobrid {
	if a, b, ok := splitHybridSlash(sym); ok {
		n, _ := strconv.ParseInt(a, 10, 64)
		if n < 0 {
			n = 0
		}
		if n > int64(math.MaxInt32) {
			n = int64(math.MaxInt32)
		}
		return Twobrid{Generic: int32(n), Col: b[0]}
	}
	i := 0
	for i < len(sym) && sym[i] >= '0' && sym[i] <= '9' {
		i++
	}
	n, _ := strconv.ParseInt(sym[:i], 10, 64)
	if n > int64(math.MaxInt32) {
		n = int64(math.MaxInt32)
	}
	return Twobrid{Generic: int32(n), Col: sym[i]}
}

// isHybridPhyrexian reports whether sym is a three-part hybrid-Phyrexian
// pip (CR 107.4f): two distinct WUBRG colours and P. Forge writes the usual
// `GWP`/`G/W/P` spelling and also the P-first `PRG` spelling on Lukka, Bound
// to Ruin; both mean a choice of either colour or two life. Measured corpus
// population: 4 ManaCost files (Ajani Sleeper Agent, Lukka Bound to Ruin,
// Nahiri the Unforgiving, Tamiyo Compleated Sage).
func isHybridPhyrexian(sym string) bool {
	if a, b, ok := splitHybridSlash(sym); ok {
		if len(a) != 1 || len(b) != 3 || b[1] != '/' {
			return false
		}
		if a[0] == 'P' {
			return b[0] != b[2] && strings.ContainsRune("WUBRG", rune(b[0])) &&
				strings.ContainsRune("WUBRG", rune(b[2]))
		}
		return b[2] == 'P' && a[0] != b[0] && strings.ContainsRune("WUBRG", rune(a[0])) &&
			strings.ContainsRune("WUBRG", rune(b[0]))
	}
	if len(sym) != 3 {
		return false
	}
	if sym[0] == 'P' {
		return sym[1] != sym[2] && strings.ContainsRune("WUBRG", rune(sym[1])) &&
			strings.ContainsRune("WUBRG", rune(sym[2]))
	}
	return sym[2] == 'P' && sym[0] != sym[1] && strings.ContainsRune("WUBRG", rune(sym[0])) &&
		strings.ContainsRune("WUBRG", rune(sym[1]))
}

// hybridPhyrexianPair normalises a hybrid-Phyrexian symbol to its two
// colours. sym is guaranteed by isHybridPhyrexian.
func hybridPhyrexianPair(sym string) HybridPhyrexian {
	if a, b, ok := splitHybridSlash(sym); ok {
		if a[0] == 'P' {
			return HybridPhyrexian{A: b[0], B: b[2]}
		}
		return HybridPhyrexian{A: a[0], B: b[0]}
	}
	if sym[0] == 'P' {
		return HybridPhyrexian{A: sym[1], B: sym[2]}
	}
	return HybridPhyrexian{A: sym[0], B: sym[1]}
}

// splitHybridSlash reports whether sym is an `a/b` slash form and splits it.
// The b side may itself carry slashes (`G/W/P`), so it returns the whole
// remainder after the first slash.
func splitHybridSlash(sym string) (a, b string, ok bool) {
	i := strings.IndexByte(sym, '/')
	if i <= 0 || i == len(sym)-1 {
		return "", "", false
	}
	return sym[:i], sym[i+1:], true
}

// isDigitRun reports whether s is a non-empty run of decimal digits.
func isDigitRun(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func (c Cost) CMC() int32 {
	// A monocolour hybrid's mana value is its generic face, not one: {2/W}
	// has mana value 2 (CR 202.4b). This matters to SetCost/MinMana floors
	// before its payment face is announced.
	twobrid := int32(0)
	for _, t := range c.Twobrid {
		twobrid = addClampedGeneric(twobrid, int64(t.Generic))
	}
	return c.Colored.Total() + c.Generic + int32(len(c.Hybrid)) + int32(len(c.Phyrexian)) +
		twobrid + int32(len(c.HybridPhyrexian)) + c.Snow
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
	if len(d.Twobrid) > 0 {
		c.Twobrid = append(append([]Twobrid(nil), c.Twobrid...), d.Twobrid...)
	}
	if len(d.HybridPhyrexian) > 0 {
		c.HybridPhyrexian = append(append([]HybridPhyrexian(nil), c.HybridPhyrexian...), d.HybridPhyrexian...)
	}
	c.Snow = addClampedGeneric(c.Snow, int64(d.Snow))
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
func (e *Engine) offerCostFor(p state.PlayerID, id state.ObjID, base Cost, scope costScope) Cost {
	c := e.costModifiers(p, id, scope).apply(base)
	if scope.kind != "Ability" {
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
		out = append(out, formatCost(e.offerCostFor(p, id, ParseCost(ab.Params["Cost"]), abilityScope(ab))))
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
	for _, t := range c.Twobrid {
		parts = append(parts, strconv.FormatInt(int64(t.Generic), 10)+"/"+string(t.Col))
	}
	for _, p := range c.Phyrexian {
		parts = append(parts, string([]byte{p, 'P'}))
	}
	for _, hp := range c.HybridPhyrexian {
		parts = append(parts, string([]byte{hp.A, '/', hp.B, '/', 'P'}))
	}
	for n := c.Snow; n > 0; n-- {
		parts = append(parts, "S")
	}
	if c.Life > 0 {
		parts = append(parts, "PayLife<"+strconv.FormatInt(int64(c.Life), 10)+">")
	}
	if c.Tap {
		parts = append(parts, "T")
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
	return strings.Join(parts, " ")
}

// HasNonMana reports whether paying this cost takes more than mana.
// AddCounter counts (the part is settled by the cast flow beside SubCounter,
// even though it takes no payment), so a caller using this to skip the
// cast-flow stages is told the truth.
func (c Cost) HasNonMana() bool {
	return c.Life > 0 || c.Tap || len(c.Sac) > 0 || len(c.Discard) > 0 || len(c.SubCounter) > 0 || len(c.AddCounter) > 0
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
		len(c.Hybrid) == 0 && len(c.Phyrexian) == 0 && len(c.Twobrid) == 0 && len(c.HybridPhyrexian) == 0
}

// pip is one flexible mana demand inside a cost's mana part, as a list of
// alternative payments tried in order. The alternative kinds are exactly the
// mana symbols CR 107.4 knows: one unit of a colour, N generic mana (a
// monocolour hybrid's "2" face), two life (a Phyrexian face), and snow mana
// (a {S} pip, payable only by a mana a snow permanent produced). A pip with
// one colour listed twice is just a strict colour pip.
type pip struct {
	alts []pipAlt
}

type pipAlt struct {
	color   byte  // one unit of this colour (0 = not a colour alternative)
	generic int32 // this many generic mana (0 = not a generic alternative)
	life    int32 // two life (0 = not a life alternative)
	snow    bool  // one snow mana unit
}

// costPips expands a cost's mana part into a flat pip list, in a fixed order
// (exact colours first — colourless included — then two-colour hybrids, then
// monocolour hybrids, then Phyrexians, then hybrid-Phyrexians, then snow).
// The order is a deterministic exploration order for resolveMana's search,
// not a payment schedule: the backtracking search tries alternatives in
// list order and each pip's alternatives in their own order, so the chosen
// assignment is stable run to run. Snow pips come last so the search prefers
// spending ordinary mana before touching a snow unit for generic.
func (c Cost) costPips() []pip {
	var out []pip
	// The coloured slots including the colourless one: a plain {C} pip is a
	// strict colourless requirement generic must not satisfy by stealing the
	// pool's only colourless, so it is reserved like any coloured pip.
	for _, letter := range []byte{'W', 'U', 'B', 'R', 'G', 'C'} {
		for n := c.Colored[state.ManaIndex(letter)]; n > 0; n-- {
			out = append(out, pip{alts: []pipAlt{{color: letter}}})
		}
	}
	for _, pair := range c.Hybrid {
		out = append(out, pip{alts: []pipAlt{{color: pair.A}, {color: pair.B}}})
	}
	for _, t := range c.Twobrid {
		alts := []pipAlt{{color: t.Col}}
		if t.Generic > 0 {
			alts = append(alts, pipAlt{generic: t.Generic})
		}
		out = append(out, pip{alts: alts})
	}
	for _, letter := range c.Phyrexian {
		out = append(out, pip{alts: []pipAlt{{color: letter}, {life: 2}}})
	}
	for _, hp := range c.HybridPhyrexian {
		out = append(out, pip{alts: []pipAlt{{color: hp.A}, {color: hp.B}, {life: 2}}})
	}
	for n := c.Snow; n > 0; n-- {
		out = append(out, pip{alts: []pipAlt{{snow: true}}})
	}
	return out
}

// manaPayment is what resolveMana found: the pool and snow tally after every
// pip and the generic requirement were paid, and the life the fixed Life
// component plus any Phyrexian face spent. Snow units are always consumed
// alongside their pool slot (Snow[i] never exceeds Pool[i]).
type manaPayment struct {
	pool      state.Mana
	snow      state.Mana
	lifeSpent int32
}

// takeUnit consumes one mana unit from slot i of rem/sn, preferring a
// NON-snow unit when one exists so a snow unit stays available for a later
// {S} pip; the backtracking search undoes the choice if the rest of the
// cost cannot be paid that way.
func takeUnit(rem, sn *state.Mana, i int) {
	if (*rem)[i] > (*sn)[i] {
		(*rem)[i]--
		return
	}
	(*rem)[i]--
	(*sn)[i]--
}

// resolveMana finds a concrete payment of the cost's mana and fixed-life
// parts from pool, the pool's parallel snow tally and the payer's life,
// preferring to spend coloured pool mana over life for a Phyrexian pip and
// the first alternative of each pip, so the assignment is deterministic. It
// returns the payment with the coloured pips and generic requirement spent,
// and whether the whole cost is payable. The generic requirement is paid
// last from whatever the pips left, so coloured mana is never spent on
// generic while a pip still needs it; a monocolour hybrid's generic face
// competes in the backtracking search as the pip's later alternative (its
// generic amount joins the requirement for the rest of the search).
func (c Cost) resolveMana(pool, snow state.Mana, life int32) (manaPayment, bool) {
	if life < c.Life {
		return manaPayment{}, false
	}
	pips := c.costPips()
	rem := pool
	sn := snow
	life -= c.Life
	lifeSpent := c.Life
	// finalGeneric is the successful search path's generic requirement: the
	// cost's own Generic plus every monocolour-hybrid pip that paid its
	// generic face on that path. The closing deduction spends exactly it.
	finalGeneric := c.Generic
	var rec func(i int, generic int32) bool
	rec = func(i int, generic int32) bool {
		if i == len(pips) {
			if rem.Total() < generic {
				return false
			}
			finalGeneric = generic
			return true
		}
		for _, alt := range pips[i].alts {
			switch {
			case alt.color != 0:
				di := state.ManaIndex(alt.color)
				if rem[di] > 0 {
					before, beforeSnow := rem, sn
					takeUnit(&rem, &sn, di)
					if rec(i+1, generic) {
						return true
					}
					rem, sn = before, beforeSnow
				}
			case alt.generic > 0:
				// A monocolour hybrid's generic face: this pip joins the
				// generic requirement (tried after the colour face, so a
				// colour unit is preferred when the search can still pay).
				if rec(i+1, generic+alt.generic) {
					return true
				}
			case alt.life > 0:
				if life >= 2 {
					life -= 2
					lifeSpent += 2
					if rec(i+1, generic) {
						return true
					}
					lifeSpent -= 2
					life += 2
				}
			case alt.snow:
				// A {S} pip consumes an actual SNOW unit: both the pool slot
				// and the parallel snow tally, so the unit that leaves is the
				// unit that was snow (never a plain unit misattributed into
				// the tally).
				for di, s := range sn {
					if s > 0 {
						before, beforeSnow := rem, sn
						rem[di]--
						sn[di]--
						if rec(i+1, generic) {
							return true
						}
						rem, sn = before, beforeSnow
					}
				}
			}
		}
		return false
	}
	if !rec(0, c.Generic) {
		return manaPayment{}, false
	}
	// The search found a pip assignment that leaves enough total mana; deduct
	// the generic requirement from that remainder, preferring colourless then
	// colours in fixed WUBRG order so payment is deterministic. Generic can
	// be paid by any leftover mana, so a total >= Generic always suffices.
	need := finalGeneric
	for _, i := range [...]int{state.MC, state.MW, state.MU, state.MB, state.MR, state.MG} {
		for need > 0 && rem[i] > 0 {
			takeUnit(&rem, &sn, i)
			need--
		}
	}
	return manaPayment{pool: rem, snow: sn, lifeSpent: lifeSpent}, true
}

// payable reports whether the cost's mana and fixed-life parts can be paid
// by pool, its parallel snow tally and the payer's current life (a Phyrexian
// pip may additionally be paid with two life; a {S} pip only by snow mana).
// This is the offering gate's feasibility question, and the real answer to
// "is there ANY way this cost can be paid right now" -- the same resolveMana
// the payment stage uses, so an offered cost and the cost it charges can
// never disagree.
func (c Cost) payable(pool, snow state.Mana, life int32) bool {
	_, ok := c.resolveMana(pool, snow, life)
	return ok
}

func (c Cost) CanPay(p state.Mana) bool {
	// Pool-only feasibility, no life and no snow offered: a hybrid must be
	// paid by one of its colours in the pool, a Phyrexian pip by its colour,
	// a monocolour hybrid by its colour (its generic face is not offered
	// here) and a {S} pip is unpayable. This is the pure pricing question the
	// corpus invariants ask, and it never treats a hybrid as generic nor lets
	// colourless `pay` it.
	_, ok := c.resolveMana(p, state.Mana{}, 0)
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
	// Pool-only: no life and no snow are offered, so a Phyrexian pip is paid
	// by its colour (the cast flow's payMana handles the life half and passes
	// a fully resolved cost here). resolveMana already reserves the coloured
	// pips and deducts generic, so the returned pool is fully spent. A failed
	// search returns the input pool untouched.
	pay, ok := c.resolveMana(p, state.Mana{}, 0)
	if !ok {
		return p, false
	}
	return pay.pool, true
}
