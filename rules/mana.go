package rules

import (
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/adams-shaun/gorge/cards"
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
	// Dyn is the non-literal amount token of a Draw part (Forge's
	// Draw<X/Spec>): N is unused and the count is resolved at payment from
	// the resolving source's SVar table by the named token (SVar:X for
	// Draw<X/...>), the way fixLifeXCost resolves an SVar-valued PayLife<X>.
	// A part whose SVar is absent or unresolvable fails closed -- the cost is
	// unpayable, never a silent zero draw. Empty for an ordinary literal
	// Draw<N/Spec>.
	//
	// On a TapPermanent part Dyn carries Forge's dynamic tapXType heads
	// instead: "X" (tapXType<X/Spec> -- the tap count announces the cast's
	// {X}, CR 601.2b: when the cost carries no other announce-bearing part
	// the tap election IS the announcement and binds Count$xPaid through the
	// pay-time CastInfo; when it does, the part settles exactly the announced
	// X) and "Any" (tapXType<Any/Spec> -- a free count that binds nothing;
	// paying the cost taps at least one matching permanent, so a spec no
	// candidate satisfies leaves the cost unpayable). N is unused for both.
	Dyn string
	// LibraryPos is the library slot a PutToLib cost part places the moved
	// card(s) at, read from Forge's <N/Pos/Spec> middle field: -1 is the
	// bottom (Forge CostPutCardToLib's "-1"), 0 is the top (its absent/
	// "0" default). It is unused by every other cost head, whose zero value
	// is inert.
	LibraryPos int32
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
	Exile           []CostPart
	Reveal          []CostPart
	Behold          []CostPart
	TapPermanent    []CostPart
	Blight          []CostPart
	Forage          bool
	// Draw carries Draw<N/Spec> components: paying one draws N cards for the
	// player(s) the spec names (default the payer). payMana never charges it;
	// the mid-resolution unless-pay path pays it (payUnlessCost), and the
	// cast/activation flow pays the payer's own parts beside the other
	// non-mana components.
	Draw   []CostPart
	Energy []CostPart
	// LifeX carries the announced PayLife<X> part (Toxic Deluge's "pay X
	// life"): the cast announces X (CR 601.2b, bounded by the payer's life
	// total) and the settle pays it as one LifeChange per part beside
	// payMana's fixed Life charge. PayLife<N> is the fixed part and lives in
	// Life above; a malformed PayLife<...> value still degrades to the
	// reported one-generic fallback.
	LifeX []CostPart
	// DamageYou carries DamageYou<N> parts -- the payer takes N damage from
	// the source. The corpus's only shape is an UnlessCost$ DamageYou<N>
	// (the Vexing Devil family), which the unless-pay arm pays through
	// effects.ParseDamageUnlessCost; the head is modelled here so ParseCost
	// stops substituting generic mana for it, and a plain Cost$ part is
	// settled by the cast flow like every other damage payment.
	DamageYou []CostPart
	// Return carries Return<N/Spec> tokens: a permanent (usually the source
	// itself, Spec CARDNAME) returned to its OWNER's hand as the payment
	// (Forge CostReturn.moveToHand; CR 118.2a lists returning a permanent to
	// its owner's hand among the payment actions).
	Return []CostPart
	// PutToLib carries PutCardToLibFrom<Zone><N/Pos/Spec> tokens: the payer
	// moves N cards matching Spec from Zone (Spec's Forge head names Hand,
	// Grave or Battlefield) to their OWN library at position Pos (-1 bottom,
	// 0 top). It is Forge's CostPutCardToLib family -- the printed activation
	// costs of Leashling, Ardent Dustspeaker, Battlefield Scrounger,
	// Timestream Navigator and Penance/Tainted Specter's UnlessCost$ -- and
	// is DISTINCT from the cumulative-upkeep PutCardToLibFromSameGrave action
	// (rules/cumulative.go), which is a keyword-expansion action rather than
	// a parsed cost token. Zone is the origin, LibraryPos the position.
	PutToLib []CostPart
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

// nonManaCost matches Sac<N/Spec>, Discard<N/Spec>, SubCounter<N/Kind> and
// Draw<N/Spec> tokens. Forge
// appends a human-readable "/description" after the spec and separates OR
// alternatives with ";"; the description may itself contain spaces (e.g.
// "Sac<1/Artifact;Creature/artifact or creature>"), which is why
// splitCostTokens keeps the whole <...> group atomic before nonManaCost ever
// sees it. The captured group only runs up to the first "/", so the trailing
// description is dropped right here; the ";" alternation is folded to ","
// (MatchesSpec's own separator) at the parse site. Ruling FL-54.
var nonManaCost = regexp.MustCompile(`^(Sac|SubCounter|Discard|Draw)<(\d+)/([^/>]+)(?:/[^>]*)?>$`)

// drawDynCost matches Forge's non-literal Draw amount, Draw<X/Spec> -- the
// Champion of Wits family's "you may draw cards equal to its power. If you
// do, discard two cards" (Cost$ Draw<X/You> with SVar:X:Count$CardPower).
// The first field is the SVar token the payment resolves against the
// source's own SVar table; the second is the player spec ("You"). The
// trailing ";" OR alternation folds to "," like every other non-mana head.
// The literal form Draw<N/Spec> stays nonManaCost's.
var drawDynCost = regexp.MustCompile(`^Draw<([A-Za-z][A-Za-z0-9]*)/([^/>]+)(?:/[^>]*)?>$`)

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

// dynTapCost matches Forge's dynamic tap-any-number tapXType tokens -- the
// heads the literal choiceCost regex above cannot read:
//
//   - tapXType<X/Spec> (Myr Battlesphere's "tap X untapped Myr", Burn at the
//     Stake's spell-cost form, Necron Overlord's "{X}, tap X artifacts"): the
//     count IS the cast's {X}. When the cost carries another announce-bearing
//     part (a printed {X}, PayEnergy<X>, Sac<X>, ...) xAsk announces it and
//     the part settles exactly that value; when it does not, the tap election
//     itself announces (CR 601.2b) and the paid count binds Count$xPaid
//     through the pay-time CastInfo. A triggered ability carrying the head
//     pays through the triggered-cost window (rules/cumulative.go), whose
//     tap election is the payment and whose empty answer is the decline.
//
//   - tapXType<Any/Spec> (Mossbridge Troll): a free count that binds no X.
//     Paying the cost still taps at least one matching permanent, so a spec
//     the filter cannot admit any candidate for (Mossbridge's
//     withTotalPowerGE10 group predicate fails closed) leaves the ability
//     unpayable rather than offering a zero-tap payment.
//
// The trailing "/description" is dropped and ";" alternations fold to ","
// like every other non-mana head.
var dynTapCost = regexp.MustCompile(`^tapXType<(X|Any)/([^/>]+)(?:/[^>]*)?>$`)
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

// putCardToLibCost matches Forge's PutCardToLibFrom<Zone><N/Pos/Spec> cost
// tokens -- moving N cards matching Spec from the payer's Hand, Graveyard or
// Battlefield to the top (Pos "0") or bottom (Pos "-1") of their own library
// as the payment (Forge CostPutCardToLib). The zone is the middle of the
// head, never a parameter, and the second field is the library position:
// Leashling/Penance/Tainted Specter place on TOP (Pos 0), the Born of the
// Gods reflective-mage family (Ardent Dustspeaker) and Battlefield Scrounger
// and Timestream Navigator put on the BOTTOM (Pos -1). The trailing
// "/description" is dropped and ";" alternations fold to "," like every
// other non-mana head. It is deliberately a POSITIVE zone list: a future
// Forge zone name this regex does not name falls through to the
// unrecognised-symbol fallback (the head is reported in Cost.Unknown, never
// silently modelled as a different zone). The separate
// PutCardToLibFromSameGrave cumulative-upkeep spelling is NOT matched here --
// it is a keyword action, not a cost token.
var putCardToLibCost = regexp.MustCompile(`^PutCardToLibFrom(Hand|Grave|Battlefield)<(\d+)/(-?\d+)/([^/>]+)(?:/[^>]*)?>$`)

// exileBattlefieldCost matches Forge's bare Exile<N/Spec> token -- exiling a
// matching permanent from the BATTLEFIELD as the payment (Karn's Sylex's
// "{X}, {T}, Exile Karn's Sylex", Mechtitan Core's "Exile CARDNAME and four
// other artifact creatures", Zombie Assassin's "{T}, Exile two cards from
// your graveyard and CARDNAME"). The zone-qualified forms are the separate
// exileCost heads above (ExileFromHand/ExileFromGrave); the trailing
// "/description" is dropped and ";" alternations fold to "," like every
// other non-mana head.
var exileBattlefieldCost = regexp.MustCompile(`^Exile<(\d+)/([^/>]+)(?:/[^>]*)?>$`)

// payLifeXCost matches Forge's announced life payment PayLife<X> (Toxic
// Deluge's "pay X life", Necrodominance's end-step body): the cast announces
// X like a printed {X} and the settle pays that much life, so the value is
// bounded by the payer's life total at the X ask. The fixed form is the
// lifeCost head above.
var payLifeXCost = regexp.MustCompile(`^PayLife<X>$`)

// subCounterXCost matches Forge's announced counter removal
// SubCounter<X/Kind> (Chandra, Awakened Inferno's "remove X loyalty
// counters"): the kind is read and the count is the cast's announced X,
// bounded by the counters the source actually has. The fixed form is the
// nonManaCost head above.
var subCounterXCost = regexp.MustCompile(`^SubCounter<X/([^/>]+)(?:/[^>]*)?>$`)

// damageYouCost matches Forge's DamageYou<N> token -- the payer takes N
// damage from the source as the payment (Forge CostDamage). The corpus's
// only shape is an UnlessCost$ (Vexing Devil's "have it deal 4 damage to
// them"), which the unless-pay arm prices through
// effects.ParseDamageUnlessCost; this head keeps a plain Cost$ spelling out
// of Cost.Unknown.
var damageYouCost = regexp.MustCompile(`^DamageYou<(\d+)(?:/[^>]*)?>$`)

var costBraces = strings.NewReplacer("{", " ", "}", " ")

// ParseCost accepts both Forge's space-separated form ("2 U U") and the
// bracketed oracle form ("{2}{U}{U}"). "no cost" and "" are free.
func ParseCost(s string) Cost {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "no cost") {
		return Cost{}
	}
	s = costBraces.Replace(s)
	var c Cost
	for toks := (costTokenIter{s: s}); ; {
		sym, ok := toks.next()
		if !ok {
			break
		}
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
		case sym == "Forage":
			c.Forage = true
		case strings.EqualFold(sym, "Mandatory"):
			// Forge's mandatory-payment marker (Cost$ Mandatory tapXType<X/...>,
			// Mandatory Sac<...>, Mandatory PayEnergy<...> -- 31 raw cost
			// occurrences): a payment-mode marker, not a payment. It priced one
			// phantom generic mana before, so a Mandatory tapXType trigger cost
			// was silently bought for {1}; skip the token and model the rest.
			continue
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
			if m := dynTapCost.FindStringSubmatch(sym); m != nil {
				// The dynamic tapXType heads (see the regex's doc): a TapPermanent
				// part whose count the tap election resolves at payment -- "X"
				// announcing the cast's {X}, "Any" free. N is unused.
				c.TapPermanent = append(c.TapPermanent, CostPart{Dyn: m[1], Spec: strings.ReplaceAll(m[2], ";", ",")})
				continue
			}
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
				case "Draw":
					c.Draw = append(c.Draw, part)
				default:
					c.SubCounter = append(c.SubCounter, part)
				}
				continue
			}
			if m := drawDynCost.FindStringSubmatch(sym); m != nil {
				// The dynamic-amount Draw cost (Draw<X/Spec>): the count is not
				// a literal but the source's SVar named by m[1], resolved at
				// payment. Recorded with Dyn set and N unused -- the part is a
				// real modelled cost, so the unrecognised-symbol fallback that
				// used to substitute one generic mana (and report the head via
				// reportUnknown, the census's cost:Draw label) never runs.
				spec := strings.ReplaceAll(m[2], ";", ",")
				c.Draw = append(c.Draw, CostPart{Spec: spec, Dyn: m[1]})
				continue
			}
			if m := exileBattlefieldCost.FindStringSubmatch(sym); m != nil {
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
				c.Exile = append(c.Exile, CostPart{N: int32(n), Spec: spec, Zone: state.ZBattlefield})
				continue
			}
			if m := payLifeXCost.FindStringSubmatch(sym); m != nil {
				// The announced form: the cast announces X (bounded by the
				// payer's life at the X ask) and the settle pays that much life.
				// No generic substitution, no Unknown entry.
				c.LifeX = append(c.LifeX, CostPart{Spec: "X", Announced: true})
				continue
			}
			if m := subCounterXCost.FindStringSubmatch(sym); m != nil {
				// The announced form: the kind is read; the count is the cast's
				// announced X (bounded by the source's counters at the X ask).
				spec := strings.ReplaceAll(m[1], ";", ",")
				c.SubCounter = append(c.SubCounter, CostPart{Spec: spec, Announced: true})
				continue
			}
			if m := damageYouCost.FindStringSubmatch(sym); m != nil {
				n, err := strconv.ParseInt(m[1], 10, 64)
				if err != nil || n < 0 || n > int64(math.MaxInt32) {
					// Same safe fallback as every other malformed cost token --
					// and REPORT it: the head is recognised, this instance is
					// not modelled.
					c.Generic = addClampedGeneric(c.Generic, 1)
					c.reportUnknown(sym)
					continue
				}
				c.DamageYou = append(c.DamageYou, CostPart{N: int32(n)})
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
			if m := putCardToLibCost.FindStringSubmatch(sym); m != nil {
				n, err := strconv.ParseInt(m[2], 10, 64)
				if err != nil || n < 0 || n > int64(math.MaxInt32) {
					// Same safe fallback as every other malformed cost token --
					// and REPORT it: the head is recognised, this instance is
					// not modelled.
					c.Generic = addClampedGeneric(c.Generic, 1)
					c.reportUnknown(sym)
					continue
				}
				pos, err := strconv.ParseInt(m[3], 10, 32)
				if err != nil || (pos != 0 && pos != -1) {
					// Only Forge's two modelled positions are real here: 0 (top)
					// and -1 (bottom). Any other value is a recognised head whose
					// instance this build cannot place, so it degrades and reports
					// rather than silently landing somewhere.
					c.Generic = addClampedGeneric(c.Generic, 1)
					c.reportUnknown(sym)
					continue
				}
				part := CostPart{N: int32(n), Spec: strings.ReplaceAll(m[4], ";", ","),
					LibraryPos: int32(pos)}
				switch m[1] {
				case "Hand":
					part.Zone = state.ZHand
				case "Grave":
					part.Zone = state.ZGraveyard
				default: // Battlefield
					part.Zone = state.ZBattlefield
				}
				c.PutToLib = append(c.PutToLib, part)
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
	for toks := (costTokenIter{s: s}); ; {
		tok, ok := toks.next()
		if !ok {
			return out
		}
		out = append(out, tok)
	}
}

type costTokenIter struct {
	s   string
	pos int
}

func (it *costTokenIter) next() (string, bool) {
	for it.pos < len(it.s) {
		r, size := utf8.DecodeRuneInString(it.s[it.pos:])
		if !unicode.IsSpace(r) {
			break
		}
		it.pos += size
	}
	if it.pos == len(it.s) {
		return "", false
	}
	start, depth := it.pos, 0
	for it.pos < len(it.s) {
		r, size := utf8.DecodeRuneInString(it.s[it.pos:])
		if r == '<' {
			depth++
		} else if r == '>' && depth > 0 {
			depth--
		} else if unicode.IsSpace(r) && depth == 0 {
			tok := it.s[start:it.pos]
			it.pos += size
			return tok, true
		}
		it.pos += size
	}
	return it.s[start:], true
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
	if len(d.PutToLib) > 0 {
		c.PutToLib = append(append([]CostPart(nil), c.PutToLib...), d.PutToLib...)
	}
	if len(d.Draw) > 0 {
		c.Draw = append(append([]CostPart(nil), c.Draw...), d.Draw...)
	}
	if len(d.LifeX) > 0 {
		c.LifeX = append(append([]CostPart(nil), c.LifeX...), d.LifeX...)
	}
	if len(d.DamageYou) > 0 {
		c.DamageYou = append(append([]CostPart(nil), c.DamageYou...), d.DamageYou...)
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
	return e.parseCost(o.Face().ManaCost)
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
	return e.offerCostForUsing(e.collectCostStatics(), p, id, base, scope)
}

func (e *Engine) offerCostForUsing(statics costStaticViews, p state.PlayerID, id state.ObjID, base Cost, scope costScope) Cost {
	return e.composedOfferCost(p, id, base, e.costModifiersWithTargetsUsing(statics, p, id, scope, nil, false), scope)
}

// composedOfferCost is offerCostFor with the modifier collection factored
// out, so offerCastable can evaluate the composition once and reuse it for
// both the per-face enumeration and the composed castable check.
func (e *Engine) composedOfferCost(p state.PlayerID, id state.ObjID, base Cost, mods costMods, scope costScope) Cost {
	c := mods.apply(base)
	if scope.kind != "Ability" {
		c = e.commanderTaxFor(p, id, c)
	}
	return c
}

// offerCastable is THE offer-side gate every cast/activation option is gated
// on. base is the RAW (pre-modifier) cost beginCast stores in pendingCast
// for this exact option and scope the costScope its modifiers are collected
// with, so the gate composes the very charge the payment will make: the
// scope's CR 601.2f modifiers over base, then (for a spell) the CR 903.8
// commander tax, never reduced by either.
//
// The mana feasibility question is posed over the still-unresolved flexible
// pip faces (costMods.feasibleAny): with a SetCost/MinMana floor in the
// composition, CR 202.4b's generic-face mana value of an unresolved twobrid
// pip can overprice the cheaper face the announcement resolves it to, and a
// composed-payable offer would then have no legal announcement. castable on
// the composed cost runs on top, supplying the non-mana parts (Sac/Discard/
// SubCounter/Tap) the enumeration does not model; composed and per-face
// feasibility are conjunctive, and the stricter composed answer can only
// withhold a legal offer (the safe direction), never offer an illegal one.
func (e *Engine) offerCastable(p state.PlayerID, id state.ObjID, base Cost, scope costScope, ability bool) bool {
	return e.offerCastableUsing(e.collectCostStatics(), p, id, base, scope, ability, nil)
}

// fixLifeXCost resolves an announced PayLife<X> cost part whose source face
// defines SVar:X with a body that is NOT Count$xPaid. Forge's SVar:X is the
// definition that body gives the cost's X, and exactly two shapes exist in
// the corpus:
//
//   - Count$xPaid (Toxic Deluge, Krumar Initiate's "Cost$ X B T PayLife<X>",
//     every SP face carrying the token): "the announced X" -- the payer
//     announces X freely (bounded by life, xAsk) and the life part settles at
//     the same value the printed {X} folds at. The cost passes through
//     unchanged.
//   - any other resolvable body (Murderous Betrayal's
//     SVar:X:Count$YourLifeTotal/HalfUp, Tornado's
//     SVar:X:Count$CardCounters.VELOCITY/Times.3): the value is FIXED -- the
//     payer announces nothing, and the settle pays exactly the evaluated
//     amount. Each such part is folded into Cost.Life, the fixed additional
//     cost both the payable gates (resolveManaWith's life check) and the
//     settle (payMana's LifeChange) already price and charge -- the same
//     place a RaiseCost's fixed life raise lands, so CR 601.2f's ordering
//     treats it as an additional cost that no reduction ever touches.
//
// The verdict is three-way, and ok=false is WITHHOLD: the SVar:X body is
// present, not Count$xPaid, and either unresolvable (War Room's commander
// colour identity) or negative -- the ability is not offered at all (the
// fail-closed direction ParseUnlessCost and manaAbilityPayable already take)
// rather than offered with an arbitrary announcement the payer cannot be
// held to. A cost pairing the non-xPaid body with a printed {X} or another
// announced part sharing the X (Sac<X/Spec>, PayEnergy<X>,
// SubCounter<X/Kind>) is also withheld: the corpus carries no such face, and
// what the fixed body would mean for a shared announcement is undefined
// here. The idempotence contract matters: offerCastable converts at the
// gate, the payment sites convert again, and a converted cost (no LifeX
// left) early-returns unchanged.
func (e *Engine) fixLifeXCost(p state.PlayerID, id state.ObjID, c Cost) (Cost, bool) {
	if len(c.LifeX) == 0 {
		return c, true
	}
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return c, true
	}
	body, present := o.Face().SVars["X"]
	if !present || strings.EqualFold(strings.TrimSpace(body), "Count$xPaid") {
		return c, true
	}
	if c.X > 0 || costAnnouncesSacX(c) {
		return c, false
	}
	for _, part := range c.Energy {
		if part.Spec == "X" {
			return c, false
		}
	}
	for _, part := range c.SubCounter {
		if part.Announced {
			return c, false
		}
	}
	ctx := &effects.Ctx{Source: id, Controller: p, SVars: o.Face().SVars}
	n, resolvable := effects.EvalCountOK(e, ctx, body)
	if !resolvable || n < 0 {
		return c, false
	}
	out := c
	out.LifeX = nil
	for range len(c.LifeX) {
		out.Life = addClampedGeneric(out.Life, int64(n))
	}
	return out, true
}

// drawCostCount resolves one Draw cost part's count at payment time. A
// literal part (Dyn == "") is simply N. A dynamic part (Forge's
// Draw<X/Spec>, Champion of Wits' "draw cards equal to its power") reads the
// source face's SVar table: the body named by part.Dyn (SVar:X for
// Draw<X/...>) is evaluated exactly the way fixLifeXCost evaluates its
// PayLife<X> body, with the source object bound as the count context so
// Count$CardPower reads the drawing permanent's own power. ok=false means
// the source face, the SVar, or the body is unavailable -- the cost is
// unpayable (the fail-closed direction), never a silent zero draw.
func (e *Engine) drawCostCount(id state.ObjID, you state.PlayerID, part CostPart) (int32, bool) {
	if part.Dyn == "" {
		return part.N, true
	}
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return 0, false
	}
	body, present := o.Face().SVars[part.Dyn]
	if !present {
		return 0, false
	}
	ctx := &effects.Ctx{Source: id, Controller: you, SVars: o.Face().SVars}
	n, resolvable := effects.EvalCountOK(e, ctx, body)
	if !resolvable || n < 0 {
		return 0, false
	}
	return n, true
}

// offerCastableUsing is offerCastable's core with the statics collected
// once (the walk shares one collection) and the mana pool optionally
// overridden: hyp nil is the ordinary real-pool gate, hyp non-nil prices the
// mana feasibility against the potential-action walk's hypothetical bound
// (the pool the seat would hold after floating every untapped source) while
// every non-mana read stays real. The two modes share one body, so the walk
// cannot drift from the offer it mirrors.
func (e *Engine) offerCastableUsing(statics costStaticViews, p state.PlayerID, id state.ObjID, base Cost, scope costScope, ability bool, hyp *state.Mana) bool {
	// The SVar-fixed PayLife<X> conversion (fixLifeXCost) shapes the cost the
	// gate prices into the exact cost the payment will store (beginCast and
	// beginActivation convert through the same helper), so an offered cost and
	// the charge can never disagree about the life part. A face whose SVar:X
	// body is present but unresolvable is WITHHELD here -- the fail-closed
	// direction the rest of the cost grammar takes -- rather than offered with
	// an arbitrary announcement.
	base, ok := e.fixLifeXCost(p, id, base)
	if !ok {
		return false
	}
	mods := e.costModifiersWithTargetsUsing(statics, p, id, scope, nil, false)
	tax := int32(0)
	if scope.kind != "Ability" {
		tax = e.commanderTaxAmount(p, id)
	}
	delve := int32(0)
	if e.HasKeyword(id, "Delve") {
		delve = int32(len(e.G.Zone(state.ZGraveyard, p)))
	}
	if !e.manaFeasiblePriced(p, id, ability, base, mods, tax, delve, hyp) {
		// A target-dependent reducer cannot be in the ordinary pre-target
		// snapshot, but it may make one legal target choice payable. Retry with
		// exactly those potential reductions; target-dependent raises/floors
		// remain absent until the actual target is known (see the helper's
		// contract).
		potential := e.costModifiersWithTargetsUsing(statics, p, id, scope, e.costPotentialTargets(p, id, scope), true)
		if !e.manaFeasiblePriced(p, id, ability, base, potential, tax, delve, hyp) {
			return false
		}
		mods = potential
	}
	// feasibleAny has established the mana half for a specific announced face
	// when a floor or Color$ reduction is face-sensitive. Do not re-check
	// that result against the unresolved Cost: Color$ W can make the W half
	// of {W/U} free, while applying it before that half is chosen sees no W
	// pip at all. The remaining cost parts are face-independent, so this
	// shared tail preserves every Sac/Discard/counter/tap legality check.
	return e.nonManaCastable(p, id, e.composedOfferCost(p, id, base, mods, scope), ability)
}

// costPotentialTargets returns the legal target candidates that can make a
// target-conditional reduction available at offer time. The final selection
// is still repriced before payment; this is only the "does SOME legal
// announcement exist" half of CR 601.2. Modal spells have no selected mode
// yet and therefore conservatively contribute no potential discount.
func (e *Engine) costPotentialTargets(p state.PlayerID, id state.ObjID, scope costScope) []state.Target {
	var sa *cards.SA
	if scope.kind == "Ability" {
		sa = scope.ab
	} else if o := e.G.Obj(id); o != nil && o.Face() != nil {
		sa = o.Face().SpellAbility()
	}
	if sa == nil || sa.Params["ValidTgts"] == "" || sa.Params["Choices"] != "" {
		return nil
	}
	var excludeSelf state.ObjID
	if scope.kind != "Ability" {
		excludeSelf = id
	}
	candidates := e.legalTargetCandidates(p, id, excludeSelf, sa)
	out := make([]state.Target, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.kind == "player" {
			out = append(out, state.Target{Player: candidate.player, IsPlayer: true})
		} else {
			out = append(out, state.Target{Obj: candidate.obj})
		}
	}
	return out
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
		if ab.Kind != "AB" || isManaAbilityAPI(ab.API) {
			continue
		}
		cost := e.parseCost(ab.Params["Cost"])
		// The ability's own ReduceCost$ (Otawara's Channel): the same
		// composition the offer gate and beginActivation's charge apply, so
		// the decision's displayed cost is the cost the payment will charge.
		if n := e.ownReduceCost(p, id, ab, nil); n > 0 && cost.Generic >= n {
			cost.Generic -= n
		} else if n > 0 {
			cost.Generic = 0
		}
		out = append(out, formatCost(e.offerCostFor(p, id, cost, abilityScope(ab))))
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
	for range c.LifeX {
		parts = append(parts, "PayLife<X>")
	}
	for _, part := range c.DamageYou {
		parts = append(parts, "DamageYou<"+strconv.FormatInt(int64(part.N), 10)+">")
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
			n := strconv.FormatInt(int64(part.N), 10)
			if part.Dyn != "" {
				n = part.Dyn
			}
			parts = append(parts, kind+"<"+n+"/"+part.Spec+">")
		}
	}
	appendCostParts("Sac", c.Sac)
	appendCostParts("Discard", c.Discard)
	appendCostParts("Draw", c.Draw)
	appendCostParts("SubCounter", c.SubCounter)
	appendCostParts("AddCounter", c.AddCounter)
	for _, part := range c.Exile {
		var head string
		switch part.Zone {
		case state.ZBattlefield:
			head = "Exile"
		case state.ZGraveyard:
			head = "ExileFromGrave"
		default:
			head = "ExileFromHand"
		}
		parts = append(parts, head+"<"+strconv.FormatInt(int64(part.N), 10)+"/"+part.Spec+">")
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
	appendCostParts("PayEnergy", c.Energy)
	for _, part := range c.PutToLib {
		zone := "Battlefield"
		switch part.Zone {
		case state.ZHand:
			zone = "Hand"
		case state.ZGraveyard:
			zone = "Grave"
		}
		parts = append(parts, "PutCardToLibFrom"+zone+"<"+strconv.FormatInt(int64(part.N), 10)+"/"+
			strconv.FormatInt(int64(part.LibraryPos), 10)+"/"+part.Spec+">")
	}
	return strings.Join(parts, " ")
}

// manaCostBeyondTap reports whether paying this cost takes anything beyond
// a bare tap: any mana pip (generic, coloured, X, snow, hybrid, Phyrexian,
// twobrid) or any non-mana component (HasNonMana). A bare-tap mana ability
// (every plain land's) and a tapless free one report false — exactly the
// windows the client's empty-priority-window floor may pass away unseen.
func manaCostBeyondTap(c Cost) bool {
	c.Tap = false
	if c.Generic > 0 || c.X > 0 || c.Snow > 0 || c.Colored != (state.Mana{}) {
		return true
	}
	if len(c.Hybrid) > 0 || len(c.Phyrexian) > 0 || len(c.Twobrid) > 0 || len(c.HybridPhyrexian) > 0 {
		return true
	}
	return c.HasNonMana()
}

// manaActivationCostMarker is the wire marker fb-led1 adds to a priority
// window's "activate" option (decision.Option.Cost): the formatCost rendering
// of the FIRST available mana ability (face order, deterministic) whose cost
// is more than a bare tap, "" when every available ability is a bare tap —
// so a plain land's option carries no field and every existing option list
// serialises byte-identically. The engine already gates the offer through
// manaAbilityPayable, which parses and prices the whole cost, so the marker
// costs no new rules knowledge at the offer site; it exists purely so a
// client that (rightly) does not count a bare tap as a play can still tell
// the Lion's Eye Diamond window it must show the player.
func manaActivationCostMarker(abilities []*cards.SA) string {
	for _, ma := range abilities {
		c := ParseCost(ma.Params["Cost"])
		if manaCostBeyondTap(c) {
			return formatCost(c)
		}
	}
	return ""
}

// dynTapParts returns the dynamic (X/Any) TapPermanent parts of a cost --
// the tapXType heads the tap election pays (dynTapCost's doc). The literal
// tapXType<N/Spec> parts are ordinary fixed payments and never appear here.
func dynTapParts(c Cost) []CostPart {
	var out []CostPart
	for _, part := range c.TapPermanent {
		if part.Dyn != "" {
			out = append(out, part)
		}
	}
	return out
}

// costCarriesDynTap reports whether a cost carries a dynamic tapXType part:
// the trigger-cost window's arm condition and the announce carve-outs use
// it, so a cost whose only X is a tapXType<X/Spec> election is treated the
// way a printed {X} is everywhere the engine asks "does this announce an X".
func costCarriesDynTap(c Cost) bool {
	return len(dynTapParts(c)) > 0
}

// withoutDynTaps strips the dynamic tapXType parts from a cost, keeping the
// literal ones (so a composed cost carrying both keeps its unpriceable half
// unpriceable). The triggered-cost window prices the non-tap rest of a
// dyn-tap cost with it.
func withoutDynTaps(c Cost) Cost {
	if !costCarriesDynTap(c) {
		return c
	}
	kept := make([]CostPart, 0, len(c.TapPermanent))
	for _, part := range c.TapPermanent {
		if part.Dyn == "" {
			kept = append(kept, part)
		}
	}
	c.TapPermanent = kept
	return c
}

// costAnnouncesCastX reports whether paying this cost announces a value for
// {X} through an announce-bearing part OTHER than a tapXType<X/Spec>
// election: a printed {X} mana symbol, a PayEnergy<X> part, an announced
// Sac<X/Spec>, SubCounter<X/Kind> or PayLife<X> part. This mirrors xAsk's
// own guard (the two must agree: the tap ask defers its X-form parts exactly
// when this is true, and xAsk bounds the announced X by the tap candidates).
// It deliberately does not fold into legal.go's costAnnouncesX, which omits
// the announced-Sac clause -- the offer gate's carve-out and this defer
// decision answer different questions and changing the offer gate's answer
// for Sac<X> costs is not this work.
func costAnnouncesCastX(c Cost) bool {
	if c.X > 0 {
		return true
	}
	for _, part := range c.Energy {
		if part.Spec == "X" {
			return true
		}
	}
	for _, part := range c.Sac {
		if part.Announced {
			return true
		}
	}
	for _, part := range c.SubCounter {
		if part.Announced {
			return true
		}
	}
	return len(c.LifeX) > 0
}

// HasNonMana reports whether paying this cost takes more than mana.
// AddCounter counts (the part is settled by the cast flow beside SubCounter,
// even though it takes no payment), so a caller using this to skip the
// cast-flow stages is told the truth.
func (c Cost) HasNonMana() bool {
	return c.Life > 0 || c.Tap || len(c.Sac) > 0 || len(c.Discard) > 0 || len(c.SubCounter) > 0 || len(c.AddCounter) > 0 || len(c.Exile) > 0 || len(c.Reveal) > 0 || len(c.Behold) > 0 || len(c.TapPermanent) > 0 || len(c.Blight) > 0 || c.Forage || len(c.Energy) > 0 || len(c.Return) > 0 || len(c.PutToLib) > 0 || len(c.Draw) > 0 || len(c.LifeX) > 0 || len(c.DamageYou) > 0
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
		len(c.Draw) == 0 && len(c.Exile) == 0 && len(c.Reveal) == 0 && len(c.Behold) == 0 &&
		len(c.TapPermanent) == 0 && len(c.Blight) == 0 && !c.Forage &&
		len(c.Hybrid) == 0 && len(c.Phyrexian) == 0 && len(c.Twobrid) == 0 && len(c.HybridPhyrexian) == 0 &&
		len(c.Energy) == 0 && len(c.Return) == 0 && len(c.PutToLib) == 0 && len(c.LifeX) == 0 && len(c.DamageYou) == 0
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

// pipRider carries the payer-side may-play riders a cost's pip alternatives
// expand under: anyColor is MayPlayIgnoreColor$ ("mana of any color",
// CR 401.5), anyType is MayPlayIgnoreType$ ("mana of any type", Rakdos, the
// Muscle). The zero value is the plain exact-colour payment.
type pipRider struct {
	anyColor bool
	anyType  bool
}

// costPips expands a cost's mana part into a flat pip list, in a fixed order
// (exact colours first — colourless included — then two-colour hybrids, then
// monocolour hybrids, then Phyrexians, then hybrid-Phyrexians, then snow).
// The order is a deterministic exploration order for resolveMana's search,
// not a payment schedule: the backtracking search tries alternatives in
// list order and each pip's alternatives in their own order, so the chosen
// assignment is stable run to run. Snow pips come last so the search prefers
// spending ordinary mana before touching a snow unit for generic.
//
// Two payer-side grants widen the alternatives: when bLifeOK is set, every
// plain {B} pip additionally accepts 2 life (K'rrik, Son of Yawgmoth's "For
// each {B} in a cost, you may pay 2 life rather than pay that mana"); when
// rider.anyColor is set, every coloured pip (plain, hybrid, twobrid,
// Phyrexian or hybrid-Phyrexian) is payable by ANY colour in the pool — the
// may-play grant's MayPlayIgnoreColor$ rider, "you may spend mana as though
// it were mana of any color to cast it" (CR 401.5). A {C} pip stays
// colourless-only under anyColor: CR 107.4c's "any color" never includes
// colourless. rider.anyType (MayPlayIgnoreType$, Rakdos, the Muscle's "mana
// of any type can be spent to cast those spells") is the wider reading: under
// it EVERY pip — coloured and {C} alike — accepts all six mana types
// (anyTypeAlts), since "any type" is every mana type, colourless included.
func (c Cost) costPips(bLifeOK bool, rider pipRider) []pip {
	var out []pip
	// The coloured slots including the colourless one: a plain {C} pip is a
	// strict colourless requirement generic must not satisfy by stealing the
	// pool's only colourless, so it is reserved like any coloured pip.
	for _, letter := range []byte{'W', 'U', 'B', 'R', 'G', 'C'} {
		for n := c.Colored[state.ManaIndex(letter)]; n > 0; n-- {
			alts := []pipAlt{{color: letter}}
			if rider.anyType {
				alts = anyTypeAlts()
			} else if rider.anyColor && letter != 'C' {
				alts = anyColorAlts()
			}
			if bLifeOK && letter == 'B' {
				alts = append(alts, pipAlt{life: 2})
			}
			out = append(out, pip{alts: alts})
		}
	}
	for _, pair := range c.Hybrid {
		alts := []pipAlt{{color: pair.A}, {color: pair.B}}
		if rider.anyType {
			alts = anyTypeAlts()
		} else if rider.anyColor {
			alts = anyColorAlts()
		}
		out = append(out, pip{alts: alts})
	}
	for _, t := range c.Twobrid {
		var alts []pipAlt
		switch {
		case rider.anyType:
			alts = anyTypeAlts()
		case rider.anyColor:
			alts = anyColorAlts()
		default:
			alts = []pipAlt{{color: t.Col}}
		}
		if t.Generic > 0 {
			alts = append(alts, pipAlt{generic: t.Generic})
		}
		out = append(out, pip{alts: alts})
	}
	for _, letter := range c.Phyrexian {
		alts := []pipAlt{{color: letter}, {life: 2}}
		if rider.anyType {
			alts = append(anyTypeAlts(), pipAlt{life: 2})
		} else if rider.anyColor {
			alts = append(anyColorAlts(), pipAlt{life: 2})
		}
		out = append(out, pip{alts: alts})
	}
	for _, hp := range c.HybridPhyrexian {
		alts := []pipAlt{{color: hp.A}, {color: hp.B}, {life: 2}}
		if rider.anyType {
			alts = append(anyTypeAlts(), pipAlt{life: 2})
		} else if rider.anyColor {
			alts = append(anyColorAlts(), pipAlt{life: 2})
		}
		out = append(out, pip{alts: alts})
	}
	for n := c.Snow; n > 0; n-- {
		out = append(out, pip{alts: []pipAlt{{snow: true}}})
	}
	return out
}

// anyColorAlts is the colour alternatives a coloured pip accepts under the
// may-play ignore-colour rider (MayPlayIgnoreColor$ True, CR 401.5): any of
// the five colours, tried in fixed WUBRG order. A {C} pip never reaches this
// helper: CR 107.4c's "any color" never includes colourless.
func anyColorAlts() []pipAlt {
	return []pipAlt{{color: 'W'}, {color: 'U'}, {color: 'B'}, {color: 'R'}, {color: 'G'}}
}

// anyTypeAlts is the MayPlayIgnoreType$ alternative set: ALL six mana types
// (Rakdos, the Muscle's "mana of any type can be spent to cast those spells"
// — "any type" is every mana type, colourless included, unlike "any color"
// which CR 107.4c keeps away from {C}). Used for every pip kind under the
// anyType rider; coloured pips, {C} pips, hybrids and Phyrexians all widen
// to it.
func anyTypeAlts() []pipAlt {
	return []pipAlt{{color: 'W'}, {color: 'U'}, {color: 'B'}, {color: 'R'}, {color: 'G'}, {color: 'C'}}
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
//
// conv, when non-nil, is the stat:ManaConvert conversion set this payment is
// resolved under (rules/mana_convert.go): it WIDENS what a pip's colour
// alternatives accept -- the payer's converted mana may be spent as though
// it were another colour -- and onlyC may also NARROW it ("spend other mana
// only as though it were colorless"). A nil conv is the plain exact-colour
// match every pre-existing caller keeps, so games with no ManaConvert static
// on the battlefield resolve byte-identically.
func (c Cost) resolveMana(pool, snow state.Mana, life int32, conv *manaConv) (manaPayment, bool) {
	return c.resolveManaWith(pool, snow, life, false, pipRider{}, conv)
}

// resolveManaWith is resolveMana with the two payer-side grants applied:
// when bLifeOK is set, every plain {B} pip additionally accepts 2 life
// (K'rrik, Son of Yawgmoth's "For each {B} in a cost, you may pay 2 life
// rather than pay that mana"); when anyColor is set, every coloured pip
// (plain, hybrid, twobrid, Phyrexian or hybrid-Phyrexian) is payable by ANY
// colour in the pool -- the may-play grant's MayPlayIgnoreColor$ rider, "you
// may spend mana as though it were mana of any color to cast it" (CR 401.5).
// A {C} pip stays colourless-only under anyColor: CR 107.4c's "any color"
// never includes colourless. Both grants keep main search's deterministic
// first-alternative preference; the expanded alternatives are tried in fixed
// WUBRG order (see anyColorAlts).
func (c Cost) resolveManaWith(pool, snow state.Mana, life int32, bLifeOK bool, rider pipRider, conv *manaConv) (manaPayment, bool) {
	if life < c.Life {
		return manaPayment{}, false
	}
	pips := c.costPips(bLifeOK, rider)
	rem := pool
	sn := snow
	life -= c.Life
	lifeSpent := c.Life
	// pipExact reports whether col is one of the pip's colour alternatives
	// (a strict colour pip lists its colour once; a hybrid lists two).
	pipExact := func(p pip, col byte) bool {
		for _, alt := range p.alts {
			if alt.color == col {
				return true
			}
		}
		return false
	}
	// pipAccepts reports whether one unit of pool colour col (manaLetters
	// index) may pay pip p under conv. Exact colours always match; conv
	// widens (wild/to) and narrows (onlyC) around that base. Non-colour
	// alternatives (generic, life, snow) are unaffected by conv.
	pipAccepts := func(p pip, col byte, di int) bool {
		isC := len(p.alts) == 1 && p.alts[0].color == 'C'
		if conv == nil {
			return pipExact(p, col)
		}
		if conv.onlyC[di] {
			// The <-C restriction: this pool colour may be spent ONLY as
			// colorless mana -- a {C} pip or generic (generic is handled
			// outside the pip loop and takes any mana), never a coloured
			// or hybrid pip, not even its own colour's.
			return isC
		}
		if pipExact(p, col) {
			return true
		}
		if conv.wild[di] {
			// "spend as mana of any color" widens to every coloured or
			// hybrid pip; the colourless-specific {C} pip is a TYPE, not a
			// colour (CR 107.4c), so it is covered only by the
			// AnyType->AnyType wording (conv.wildC).
			if isC {
				return conv.wildC
			}
			return true
		}
		for _, alt := range p.alts {
			if alt.color != 0 && conv.to[di][state.ManaIndex(alt.color)] {
				return true
			}
		}
		return false
	}
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
		p := pips[i]
		// Try each alternative in order: for a hybrid this prefers A over B;
		// for a single-colour pip there is one colour alternative, then the
		// pip's life face for a Phyrexian pip.
		for _, alt := range p.alts {
			switch {
			case alt.color != 0:
				di := state.ManaIndex(alt.color)
				if rem[di] > 0 && pipAccepts(p, alt.color, di) {
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
		// A conversion may let OTHER pool colours pay this pip too -- e.g.
		// "spend white mana as though it were red" offers the pool's white
		// mana for a red pip, which the exact-colour alternatives above
		// cannot see. The walk order is manaLetters (WUBRGC), so the
		// assignment stays deterministic; converted mana is tried only after
		// every exact alternative, so a conversion never displaces an exact
		// payment.
		if conv != nil {
			for di := range manaLetters {
				col := manaLetters[di][0]
				if !pipExact(p, col) && rem[di] > 0 && pipAccepts(p, col, di) {
					before, beforeSnow := rem, sn
					takeUnit(&rem, &sn, di)
					if rec(i+1, generic) {
						return true
					}
					rem, sn = before, beforeSnow
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
	_, ok := c.resolveMana(pool, snow, life, nil)
	return ok
}

func (c Cost) CanPay(p state.Mana) bool {
	// Pool-only feasibility, no life and no snow offered: a hybrid must be
	// paid by one of its colours in the pool, a Phyrexian pip by its colour,
	// a monocolour hybrid by its colour (its generic face is not offered
	// here) and a {S} pip is unpayable. This is the pure pricing question the
	// corpus invariants ask, and it never treats a hybrid as generic nor lets
	// colourless `pay` it.
	_, ok := c.resolveMana(p, state.Mana{}, 0, nil)
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
	pay, ok := c.resolveMana(p, state.Mana{}, 0, nil)
	if !ok {
		return p, false
	}
	return pay.pool, true
}

// ParseUnlessCost strictly parses an UnlessCost$ value for the mid-resolution
// unless-pay path. Unlike ParseCost — which degrades every token it does not
// know to one generic mana, silently buying a dynamic or unmodelled cost for
// {1} — this parser is total and strict: every token must be a mana symbol
// (a WUBRGC letter or a numeric generic), a fixed PayLife<N>, or a
// Sac<N/Spec>, Discard<N/Spec>, SubCounter<N/Kind>, Draw<N/Spec> or
// Reveal<N/Spec> component. Anything else — X, Y, Z (whose value is a cast
// choice or an SVar the unless-pay answer does not carry), DamageYou<N>,
// PayEnergy<N>, Return<...>, ExileFromGrave<...>, Behold<...>,
// tapXType<...>, LifeTotalHalfUp, DefinedCost_*, CopyCost, or any prose —
// reports ok=false, and the unless-pay arm treats that as a hard decline
// (the conservative read: a payer who "pays" a cost the engine cannot price
// has not paid it). A Reveal component is choice-bearing like Sac/Discard
// and pays through the beginUnlessPayment continuation, never synchronously.
func ParseUnlessCost(s string) (Cost, bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "no cost") {
		return Cost{}, true
	}
	s = costBraces.Replace(s)
	var c Cost
	for toks := (costTokenIter{s: s}); ; {
		sym, ok := toks.next()
		if !ok {
			break
		}
		switch {
		case sym == "T" || sym == "X":
			// An unfolded X is never priceable here: payMana does not charge
			// it, so a "pay" from an empty pool would satisfy it for free.
			return Cost{}, false
		case len(sym) == 1 && strings.ContainsAny(sym, "WUBRGC"):
			c.Colored[state.ManaIndex(sym[0])]++
		default:
			if n, err := strconv.Atoi(sym); err == nil && n >= 0 {
				c.Generic = addClampedGeneric(c.Generic, int64(n))
				continue
			}
			if m := lifeCost.FindStringSubmatch(sym); m != nil {
				n, err := strconv.ParseInt(m[1], 10, 64)
				if err != nil || n < 0 || n > int64(math.MaxInt32) {
					return Cost{}, false
				}
				c.Life = addClampedGeneric(c.Life, n)
				continue
			}
			if m := nonManaCost.FindStringSubmatch(sym); m != nil {
				n, err := strconv.ParseInt(m[2], 10, 64)
				if err != nil || n < 0 || n > int64(math.MaxInt32) {
					return Cost{}, false
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
				case "Draw":
					c.Draw = append(c.Draw, part)
				default:
					c.SubCounter = append(c.SubCounter, part)
				}
				continue
			}
			// Reveal<N/Spec> is the hideaway-family choice-bearing unless cost
			// (Primal Beyond, Port Town, Xyru Specter's Challenge): the payer
			// reveals N hand cards matching the spec. It parses like the other
			// component heads and PAYS through the beginUnlessPayment
			// continuation (payUnlessCost refuses it, exactly like Sac/Discard).
			// The other choiceCost heads, Behold<...> and tapXType<...>, stay
			// hard declines — no corpus UnlessCost$ carries either.
			if m := choiceCost.FindStringSubmatch(sym); m != nil {
				if m[1] != "Reveal" {
					return Cost{}, false
				}
				n, err := strconv.ParseInt(m[2], 10, 64)
				if err != nil || n <= 0 || n > int64(math.MaxInt32) {
					return Cost{}, false
				}
				c.Reveal = append(c.Reveal, CostPart{N: int32(n), Spec: strings.ReplaceAll(m[3], ";", ",")})
				continue
			}
			// Every other token — a dynamic amount, an unmodelled cost verb,
			// or prose — makes the whole cost unpriceable.
			return Cost{}, false
		}
	}
	return c, true
}

// payerGrantsPayLifeInsteadOfB reports whether p's side of the battlefield
// carries a Continuous static granting PayLifeInsteadOf:B to p (K'rrik's
// "Affected$ You | AddKeyword$ PayLifeInsteadOf:B"). Every mana payment and
// every cast/activation offer gate consults it, so a plain {B} pip is
// payable with 2 life anywhere K'rrik is in play under its controller.
func (e *Engine) payerGrantsPayLifeInsteadOfB(p state.PlayerID) bool {
	for _, sv := range e.activeStatics("Continuous") {
		if !slices.Contains(cards.SplitKeywordList(sv.Params["AddKeyword"]), "PayLifeInsteadOf:B") {
			continue
		}
		if effects.MatchesPlayerSpec(e.G, sv.Params["Affected"], p, sv.Controller) {
			return true
		}
	}
	return false
}

// payerGrantsIgnoreColor reports whether an active may-play grant of p's
// carrying MayPlayIgnoreColor$ True selects the card id being cast: the
// grant is p's, its AffectedZone names the card's CURRENT zone (so a card
// being cast the ordinary way from hand never inherits an exile grant), and
// the Affected$ spec matches the card. The IsRemembered predicate inside a
// grant's spec is matched against the CONTINUOUS EFFECT's Remembered set
// (the cards the delivering Effect captured), through the SpecContext the
// ordinary filter grammar already carries -- the same direct-list reading
// restrictionApplies uses for Effect-delivered CantTarget/CantRegenerate.
func (e *Engine) payerGrantsIgnoreColor(p state.PlayerID, id state.ObjID) bool {
	return e.payerGrantsMayPlayRider(p, id, func(ce ContinuousEffect) bool { return ce.MayPlayIgnoreColor })
}

// payerGrantsIgnoreType is payerGrantsIgnoreColor for the MayPlayIgnoreType$
// rider (Rakdos, the Muscle's "mana of any type can be spent to cast those
// spells"): the same zone/Affects/remembered-set reading, keyed on the wider
// rider.
func (e *Engine) payerGrantsIgnoreType(p state.PlayerID, id state.ObjID) bool {
	return e.payerGrantsMayPlayRider(p, id, func(ce ContinuousEffect) bool { return ce.MayPlayIgnoreType })
}

// payerGrantsMayPlayRider is the shared body of the two may-play payment
// riders: it walks the active may-play grants of p's and reports whether one
// carrying the asked rider selects the card id being cast.
func (e *Engine) payerGrantsMayPlayRider(p state.PlayerID, id state.ObjID, rider func(ContinuousEffect) bool) bool {
	o := e.G.Obj(id)
	if o == nil {
		return false
	}
	for _, ce := range e.active() {
		if !ce.MayPlay || !rider(ce) || ce.Controller != p {
			continue
		}
		if ce.MayPlayPlayerTurn && e.G.Active != p {
			continue
		}
		zones, all, ok := effects.ParseZones(ce.AffectedZone)
		if !ok && !all {
			continue
		}
		if !all && !slices.Contains(zones, o.Zone) {
			continue
		}
		sc := effects.SpecContext{You: ce.Controller, Source: ce.Source,
			Remembered: rememberedTargets(ce.Remembered), Resolving: true}
		if effects.MatchesSpecCtx(e.G, ce.Affects, id, sc) {
			return true
		}
	}
	return false
}

// rememberedTargets lifts a ContinuousEffect's Remembered object ids into
// the []state.Target shape the filter grammar's SpecContext carries.
func rememberedTargets(ids []state.ObjID) []state.Target {
	if len(ids) == 0 {
		return nil
	}
	out := make([]state.Target, 0, len(ids))
	for _, id := range ids {
		out = append(out, state.Target{Obj: id})
	}
	return out
}
