package cost

import (
	"math"
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// CostPart is one non-mana cost component: Sac<N/Spec> (sacrifice N
// permanents matching Spec), Discard<N/Spec> (discard N matching cards), or
// SubCounter<N/Kind> (remove N counters of Kind from the source), or
// announced-count ExileFromGrave<X/Spec>.
type CostPart struct {
	N    int32
	Spec string
	// ChooseCard marks Close Encounter's choice-only cost, never a reveal arm.
	ChooseCard bool
	// Referent is a source-relative object name bound by a granted ability.
	// For Exile<N/OriginalHost>, this is the grantor rather than the recipient
	// permanent that carries the activated ability.
	Referent state.ObjID
	// Zone is the single zone an Exile cost part pays from. Zero means hand.
	// ZoneSet is a bitset of alternative zones for ExileCtrlOrGrave; when
	// nonzero it takes precedence over Zone.
	Zone    state.Zone
	ZoneSet uint8
	// Announced marks a variable-count Sac<X/Spec> or ExileFromGrave<X/Spec>
	// part: the player announces the count as the cast's X (CR 601.2b)
	// and exactly that many matching objects are paid. N is unused for an Announced part.
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
	// Target is the removal-target filter of a SubCounter part's third field
	// (SubCounter<N|X/Kind/Target[/desc]>): the permanent WHOSE counters the
	// payment removes (Ghave, Guru of Spores' "remove a +1/+1 counter from a
	// creature you control"), matched with MatchesSpecFrom against the payer's
	// battlefield exactly like a Sac part's spec. Empty (and Forge's
	// CARDNAME/NICKNAME spellings, engine/rules/cast.go's subCounterTargetsSource)
	// anchors the removal on the SOURCE itself -- the reading every two-field
	// SubCounter<N/Kind> token has always had. Forge writes the restriction
	// space-free and the display description prose, so a space-bearing third
	// field is a description, not a filter.
	Target string
	// Desc is Forge's trailing human-readable description field
	// (the ".../another creature" in Sac<1/Creature.Other/another creature>),
	// captured verbatim so a player-facing prompt can render prose instead of
	// the raw token (engine/rules/mana.go's costPhrase). It is DISPLAY ONLY: no
	// payment path reads it, so a part with an empty Desc behaves exactly as
	// before. Every cost head that can carry a description captures it here;
	// a head whose regex has no description group (PayLife<N>, Blight<N>)
	// leaves it empty.
	Desc string
	// MinPower is the SET-level floor a withTotalPowerGE<N> group predicate
	// places on a TapPermanent part (Mossbridge Troll's "total power 10 or
	// greater", Crew's "total power N or greater"): the predicate constrains
	// the TAPPED SET's total power, not each candidate on its own, so it is
	// not a filter the per-object matcher can evaluate. stripGroupPowerFloor
	// moves it out of the spec at parse time -- into this field, so the
	// offer gate (engine/rules/cast.go nonManaCastable) and the tap election
	// (Decision.MinSum over Option.Value = the candidate's current power)
	// enforce it against the set the payment actually taps. It is stripped
	// for the Any form and the literal N form; an X-form part keeps the
	// predicate in its spec, which fails closed as before (a floor whose
	// count the announcement fixes first would need re-pricing at settle --
	// no corpus carrier combines them).
	MinPower int32
	// ThenExile marks a Behold part spelled BeholdExile<N/Spec> (the
	// Lorwyn Champion cycle's "behold a Kithkin and exile it"): the beheld
	// object -- a permanent the payer controls or a card revealed from their
	// hand -- is then exiled as part of the same payment, linked to the
	// paying source (ExiledWith) so "return the exiled card" reads it.
	ThenExile bool
	// Each marks a GainLife part spelled GainLife<N/Player.../*>, Forge's
	// "each" marker on the trailing third field (Reverent Silence's "each
	// other player gains 6 life", Skyshroud Cutter's 5). It is preserved as
	// parsed data -- never re-derived from the Spec text at payment -- so the
	// prose and any future per-part reading agree with the token. It is
	// unused by every other head. Note this is a DESCRIPTION of the printed
	// wording, not the payment semantics: the payment pays every player the
	// Spec matches relative to the payer (which for Player.Other/* is exactly
	// the oracle), with or without the marker.
	Each bool
	// AtLeast marks Forge's open-ended "N+" count on an Exile part
	// (Exile<1+/...>, ExileFromGrave<1+/...>): the payer exiles N OR MORE
	// matching objects -- "exile one or more other artifacts you control"
	// (Fabrication Foundry), "exile any number of black cards from your
	// graveyard" (Baron Helmut Zemo; Forge writes "any number" with a
	// set-level floor as 1+, since an empty set cannot reach the floor).
	// N is the minimum. The count binds nothing: the elected set's size is
	// the player's choice, bounded only by Group.
	AtLeast bool
	// Group is the SET-level constraint a withTotal<Prop><Cmp><Rhs> group
	// predicate places on an AtLeast part's elected set (the sum of the
	// elected objects' mana values, or of their mana symbols of one colour).
	// Like MinPower it is moved out of the spec at parse time -- a
	// per-object matcher cannot evaluate a property of the whole set -- and
	// the offer gate, the X announcement and the election (Decision.MinSum/
	// MaxSum over Option.Value) enforce it. The zero value is no constraint.
	Group GroupTotal
}

// GroupTotal is one parsed withTotal<Prop><Cmp><Rhs> group predicate (see
// CostPart.Group): Prop names the summed per-object quantity, Cmp the
// comparison against the right-hand side, which is the literal N or -- when
// Var is set -- the announced value of that name (Fabrication Foundry's
// withTotalCMCEQX with Announce$ X). Raw is the predicate's own spelling,
// kept for FormatCost so a formatted cost re-parses to the same part.
type GroupTotal struct {
	Prop  GroupProp
	Color byte // the WUBRG letter a GroupManaSymbols sum counts
	Cmp   GroupCmp
	N     int32
	Var   string
	Raw   string
}

// GroupProp is the per-object quantity a GroupTotal sums.
type GroupProp uint8

const (
	// GroupNone is the zero GroupTotal: no set-level constraint.
	GroupNone GroupProp = iota
	// GroupCMC sums mana values (withTotalCMC...).
	GroupCMC
	// GroupManaSymbols sums the mana symbols of one colour among the
	// objects' mana costs (withTotalManaSymbols_<Color>_...): a hybrid or
	// Phyrexian symbol of that colour counts as one (CR 107.4e/f, 202.2a).
	GroupManaSymbols
)

// GroupCmp is a GroupTotal's comparison.
type GroupCmp uint8

const (
	// GroupEQ: the total equals the right-hand side.
	GroupEQ GroupCmp = iota + 1
	// GroupGE: the total is at least the right-hand side.
	GroupGE
)

// Active reports whether g carries a constraint.
func (g GroupTotal) Active() bool { return g.Prop != GroupNone }

// Holds reports whether total satisfies g against the resolved right-hand
// side rhs (the literal N, or the announced value Var names).
func (g GroupTotal) Holds(total, rhs int32) bool {
	switch g.Cmp {
	case GroupEQ:
		return total == rhs
	case GroupGE:
		return total >= rhs
	}
	return false
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
	Colored state.Mana
	Generic int32
	Life    int32
	X       int
	// XMin is the LOWER BOUND an XMin<N> cost token places on the announced
	// X ("X can't be 0"): XMin1 means the cost's {X} must be at least 1.
	// It is not a payment -- it adds no generic mana and reports no Unknown
	// -- only a floor for xAsk's option list and the offer gate's minimum-X
	// price. XMin is a property of the shared announced X, so Plus takes the
	// max of the two bounds and WithX (the announcement) clears it.
	XMin            int32
	Hybrid          []ManaPair
	Phyrexian       []byte
	Twobrid         []Twobrid
	HybridPhyrexian []HybridPhyrexian
	Snow            int32
	// Waterbend carries the amount a Waterbend<N> cost token lets tapped
	// artifacts and creatures help pay (CR 701.67a): the {N} is already
	// folded into Generic by the same token (so Waterbend adds no charge of
	// its own, it is the CAP on how much of that generic the taps may
	// cover), and each untapped artifact or creature the payer taps while
	// paying the cost pays for {1}. WaterbendX marks a Waterbend<X> part,
	// whose amount is the announced {X} (the token also contributes that X
	// to X, exactly like a RaiseCost Waterbend<X>). The field is the shared
	// reading both an ability's own Cost$ (Giant Koi) and a spell's
	// additional part (the optional-cost Waterbend<4> family) expose, so the
	// offer gate can credit the taps and the payment can offer them.
	Waterbend  int32
	WaterbendX bool
	Tap        bool
	Untap      bool
	Sac        []CostPart
	Discard    []CostPart
	SubCounter []CostPart
	AddCounter []CostPart
	Exile      []CostPart
	// ExileFromTop carries Forge's ExileFromTop<N/Card> parts -- exiling the
	// top N cards of the payer's OWN library as a cast/activation cost (Storm
	// Elemental, Phyrexian Devourer, Arc-Slogger, Whirling Catapult). It is a
	// DISTINCT slice from Exile on purpose: state.ZLibrary is the zero Zone,
	// and CostPart.Zone's zero value means "the hand" to every hand/grave
	// exile path, so a library part recorded in Exile would be read back as a
	// hand exile. The library is ordered, so the payment takes the actual top
	// N cards and never poses a chooser.
	ExileFromTop []CostPart
	Reveal       []CostPart
	// RevealOrChoose carries Forge's either-or `RevealOrChoose<N/Spec>` cost
	// (Monstrous Emergence, Dragon's Fire): reveal N hand cards matching Spec
	// OR choose N permanents matching Spec you control. It is deliberately a
	// DISTINCT slice from Reveal: the reveal arm is a real hand-card reveal
	// (public Note, the card rides the Revealed paid list), while the choose
	// arm elects a permanent already on the battlefield -- a different
	// provenance that must not be read as a hand reveal. Both arms' elected
	// objects land in the same paid list the `Revealed$<Property>` refs read
	// (Forge's CostReveal owns both arms), but only the hand arm is announced
	// as a reveal. Spec serves both arms (the card's own chooser reads one
	// type for the hand card and the permanent).
	// ChooseCard parts share the paid-object election but carry a distinct
	// cost kind so formatting/reparsing cannot turn them into hand reveals.
	RevealOrChoose []CostPart
	// RevealChosen carries RevealChosen<Player> and RevealChosen<Type/...>
	// components (Stalking Leonin, Guardian Archon, Emissary of Grudges, A
	// Killer Among Us): the payer publicly reveals a designation that was
	// chosen SECRETLY earlier in the game (a player, or a creature type).
	// Unlike Reveal<N/Spec> there is NO hand choice and no mana -- the whole
	// part is free -- so the payability gate is the designation's presence on
	// the ability's source, and the payment is one public Note. Spec is
	// "Player" or "Type".
	RevealChosen []CostPart
	// ChooseType counts ChooseCreatureType<N> parts: "as an additional cost
	// to cast this spell, choose a creature type" (Caller of the Hunt, and
	// Celestial Reunion's optional cost). The choice is a payment with no
	// resource (CR 601.2h); the cast flow asks it (chooseTypeCostAsk) before
	// any part that reads Card.ChosenType, and records it on the spell
	// through the existing Choose "type" event, which the permanent the spell
	// becomes keeps (events.Apply's Move keeps Chosen* across stack ->
	// battlefield).
	ChooseType   int32
	Behold       []CostPart
	TapPermanent []CostPart
	// UntapPermanent carries untapYType<N/Spec> parts -- untapping N
	// permanents matching Spec as a cost (Benthic Explorers' "{T}, Untap a
	// tapped land an opponent controls: Add one mana of any type that land
	// could produce"). It is DISTINCT from Cost.Untap (the {Q} symbol,
	// untapping the SOURCE itself): this head untaps OTHER permanents the
	// spec names, so the settle elects them like the literal tapXType form.
	// The trailing "/description" is captured into CostPart.Desc.
	UntapPermanent []CostPart
	Blight         []CostPart
	// Exert carries Exert<1/CARDNAME> parts (CR 701.39: the source will not
	// untap during its controller's next untap step). Paid by one
	// events.Exert on the source -- the same event the declare-attackers
	// exert election emits, so "whenever you exert" triggers and the untap
	// skip read one fold. Only the source-anchored form is modelled; any
	// other spelling keeps the reported one-generic fallback.
	Exert  []CostPart
	Forage bool
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
	// LifeHalfUp marks a LifeTotalHalfUp token (Temporal Extortion's "may pay
	// half their life, rounded up"): the amount is the PAYER's life total at
	// pay time, half rounded up, so the strict parser cannot fold it — the
	// unless path's gate and charge sites resolve it against the payer
	// (unlessFoldDynamicLife) before the mana/life machinery reads Life.
	// A payer at zero life cannot pay (nothing to halve — fail closed).
	LifeHalfUp bool
	// DamageYou carries DamageYou<N> parts -- the payer takes N damage from
	// the source. The corpus's only shape is an UnlessCost$ DamageYou<N>
	// (the Vexing Devil family), which the unless-pay arm pays through
	// effects.ParseDamageUnlessCost; the head is modelled here so ParseCost
	// stops substituting generic mana for it, and a plain Cost$ part is
	// settled by the cast flow like every other damage payment.
	DamageYou []CostPart
	// GainLife carries GainLife<N/Player...> parts -- the PAYER has the named
	// player(s) gain N life as the payment (Forge CostGainLife; the
	// Invigorate/Reverent Silence/Skyshroud Cutter AlternativeCost family,
	// "rather than pay this spell's mana cost, you may have an opponent gain
	// N life"). It is a payment of the cost, not an effect on the payer:
	// each part emits one POSITIVE LifeChange per player the part's Spec
	// matches relative to the payer, routed through the ordinary
	// applyLifeReplacements machinery (CR 616 GainLife replacements apply).
	// Spec keeps Forge's raw player word (Player.Opponent / Player.Other);
	// Each is the trailing `/*` marker (see CostPart.Each) for the
	// "each other player" spelling. Before the head existed the token hit
	// the unrecognised-symbol fallback -- a phantom generic pip and the
	// whole alternative withheld (altCostParse fails closed on Unknown).
	GainLife []CostPart
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
	// (engine/rules/cumulative.go), which is a keyword-expansion action rather than
	// a parsed cost token. Zone is the origin, LibraryPos the position.
	PutToLib []CostPart
	// MoveToGrave carries ExiledMoveToGrave<N/Spec> tokens: N cards matching
	// Spec move out of EXILE into their OWNER's graveyard as the payment
	// (Forge CostExiledMoveToGrave; the Eldrazi processor family's "put a
	// card an opponent owns from exile into that player's graveyard" and
	// Shelob, Dread Weaver's "put a creature card exiled with Shelob into its
	// owner's graveyard"). The origin is always every player's exile zone
	// -- exiled cards live in their OWNER's exile zone (engine/events/apply.go's
	// zoneOwner), so a candidate scan iterates the players -- and the
	// destination is the owner's graveyard, which is why this is its own
	// slice rather than an Exile part (whose Zone is the ORIGIN and whose
	// destination is always exile). The specs in measured use are
	// Card.OppOwn (12 lines) and Card/Creature.ExiledWithSource (4 lines);
	// both read through the ordinary filter grammar (engine/effects/filter.go's
	// OppOwn and ExiledWithSource predicates, bound to the PAYER and the
	// ability's source).
	MoveToGrave []CostPart
	// Mill carries Mill<N> cost components paid from the payer's library.
	Mill []CostPart
	// Evidence carries CollectEvidence<N> and CollectEvidence<NAME> tokens
	// (alltargeted1): the payer exiles cards from their OWN graveyard whose
	// total mana value reaches N or greater (CR 701.30b's collect evidence;
	// the same action the Ward payment path pays). N is a literal, or a name
	// (Urgent Necropsy's X) resolved through the source face's SVar table
	// AFTER the CR 601.2c targets exist -- the corpus carrier's body reads
	// AllTargeted$CardManaCost over the whole root/sub-ability target union,
	// so the resolution needs the pre-asked sub-ability targets too. The
	// payment is the evidenceAsk stage (cast.go), which runs after the
	// target stages for exactly that reason; at OFFER time the amount is
	// unknowable, so nonManaCastable deliberately does not gate on this
	// part (fail-open there) and the payment stage aborts (CR 733.1) when
	// the graveyard cannot reach the resolved total.
	Evidence []CostPart
	// RollDice carries RollDice<N/Sides/XVar> free cost components. The payment
	// rolls each die and publishes its canonical effects.DieRollNote; Dyn names
	// the ability X binding (currently only X is modelled).
	RollDice []CostPart
	// Withheld lists the parts of a RaiseCost static's Cost$ that the
	// payment machinery cannot pay (a head no stage settles, a named count
	// with no resolvable reading). It is never produced by ParseCost: only
	// the RaiseCost bridge (engine/rules/raise_cost_extra.go) writes it, and
	// nonManaCastable refuses any cost that carries it, so the spell or
	// ability is withheld at the offer gate instead of being offered with
	// the additional cost silently dropped.
	Withheld []string
	// Unknown lists the HEAD (the text before any "<...>") of every cost
	// token this parse did not model, in order of appearance, deduplicated.
	// A token lands here exactly when ParseCost could not give it real
	// semantics and priced it as one generic mana (or one life-equivalent
	// of nothing) instead: the final unrecognised-symbol fallback AND the
	// malformed/out-of-range instances of otherwise-recognised heads (an
	// unparseable or int-overflow "PayLife<...>", "Sac<...>",
	// "AddCounter<...>" value — the head is known, that INSTANCE is not
	// modelled). Payment behaviour is unchanged by this field: it is a pure
	// report, read by the parameter census (engine/rules/paramcensus_test.go) so
	// the repo-deck ratchet can name cost tokens a card's script carries
	// that the engine silently substitutes generic mana for (e.g. Chthonian
	// Nightmare's "PayEnergy<X> ... Return<1/CARDNAME>").
	Unknown []string
}

// IsWholeHandRevealSpec reports whether a Reveal cost part's type slot names
// the WHOLE hand rather than a card filter. Forge spells "reveal your hand"
// as Reveal<N/Hand> (Land Grant's free-cast cost, Sasaya, Orochi Ascendant's
// flip cost) -- the count is display noise, the same reading
// discardCandidates gives Discard<1/Hand> and Discard<0/Hand> -- and no card
// ever matches the bare word "Hand" as a filter, so reading it as one leaves
// the cost permanently unpayable and every carrier unplayable. Revealing an
// empty hand is legal (CR 701.20a), so a whole-hand reveal is payable with
// ANY hand, including an empty one.
func IsWholeHandRevealSpec(spec string) bool {
	return strings.EqualFold(spec, "Hand")
}

// IsSameColorRevealSpec reports whether a Reveal cost part's type slot is the
// RELATIONAL "SameColor" (Reveal<2/SameColor>, Illuminated Folio's "Reveal
// two cards from your hand that share a color"). SameColor names a relation
// BETWEEN the revealed cards, not a card property: no card's type line
// carries it, so reading it as one left the ability permanently unoffered.
// The reading lives beside the offer gate and the payment ask
// (engine/rules/cast.go's nonManaCastable / revealCostAsk) as a decision.SetPropShared
// constraint over each candidate's DERIVED colour tokens -- see
// sameColorRevealSets in rules/setprops.go.
func IsSameColorRevealSpec(spec string) bool {
	return strings.EqualFold(spec, "SameColor")
}

// IsWholeZoneExileSpec reports whether an Exile cost part's type slot names
// the WHOLE zone rather than a card filter. Forge spells "exile your hand"
// (Herigast, Erupting Nullkite) as ExileFromHand<N/All> and "exile all cards
// from their graveyard" (Grip of Amnesia) as ExileFromGrave<N/All> -- the
// count is display noise, the same reading isWholeHandRevealSpec gives
// Reveal<N/Hand> and discardCandidates gives Discard<N/Hand>. No card ever
// matches the bare word "All" as a filter, so reading it as one leaves the
// cost permanently unpayable and every carrier's pay window decline-only.
// Unlike the whole-hand reveal, an ALL-zone exile is NOT payable empty: the
// token still demands part.N cards (the corpus writes 1), so a zone holding
// fewer than part.N cards cannot pay.
func IsWholeZoneExileSpec(spec string) bool {
	return strings.EqualFold(spec, "All")
}

// MillCostTotal returns the total number of cards every Mill<N> cost
// component requires (0 when there is no Mill part). The total is int64 so
// the sum of arbitrary parts cannot overflow int on a 32-bit build; ok is
// false for a negative requirement, which can never be paid.
func MillCostTotal(parts []CostPart) (int64, bool) {
	var total int64
	for _, part := range parts {
		if part.N < 0 {
			return 0, false
		}
		total += int64(part.N)
	}
	return total, true
}

// AddClampedGeneric adds n to the int32 generic count, saturating at
// math.MaxInt32 on top and refusing to go below zero, so no accumulation of
// numeric tokens (nor a WithX fold) can ever wrap Generic negative. Task 20.
func AddClampedGeneric(v int32, n int64) int32 {
	total := int64(v) + n
	if total > math.MaxInt32 {
		return math.MaxInt32
	}
	if total < 0 {
		return 0
	}
	return int32(total)
}

func (c Cost) CMC() int32 {
	// A monocolour hybrid's mana value is its generic face, not one: {2/W}
	// has mana value 2 (CR 202.4b). This matters to SetCost/MinMana floors
	// before its payment face is announced.
	twobrid := int32(0)
	for _, t := range c.Twobrid {
		twobrid = AddClampedGeneric(twobrid, int64(t.Generic))
	}
	return c.Colored.Total() + c.Generic + int32(len(c.Hybrid)) + int32(len(c.Phyrexian)) +
		twobrid + int32(len(c.HybridPhyrexian)) + c.Snow
}

// WithX folds a chosen X value into Generic, once per X symbol the cost
// carried, then clears X: once a value is chosen, {X} is no longer a
// distinct requirement, it is simply that much more generic mana.
func (c Cost) WithX(x int32) Cost {
	c.Generic = AddClampedGeneric(c.Generic, int64(c.X)*int64(x))
	c.X = 0
	// The lower bound is consumed by the announcement: once an X is chosen
	// the bound has served its purpose, and clearing it keeps a later
	// WithX from double-charging the floor as if it were generic mana.
	c.XMin = 0
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
	c.Life = AddClampedGeneric(c.Life, int64(d.Life))
	c.X += d.X
	// The shared announced X takes the higher of the two lower bounds
	// (Thieving Skydiver: the printed cost carries none, the kicked Kicker
	// part carries XMin1, so the composed cast's X must be at least 1).
	if d.XMin > c.XMin {
		c.XMin = d.XMin
	}
	c.Tap = c.Tap || d.Tap
	c.Untap = c.Untap || d.Untap
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
	c.Snow = AddClampedGeneric(c.Snow, int64(d.Snow))
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
	if len(d.ExileFromTop) > 0 {
		c.ExileFromTop = append(append([]CostPart(nil), c.ExileFromTop...), d.ExileFromTop...)
	}
	if len(d.MoveToGrave) > 0 {
		c.MoveToGrave = append(append([]CostPart(nil), c.MoveToGrave...), d.MoveToGrave...)
	}
	if len(d.Mill) > 0 {
		c.Mill = append(append([]CostPart(nil), c.Mill...), d.Mill...)
	}
	if len(d.Evidence) > 0 {
		c.Evidence = append(append([]CostPart(nil), c.Evidence...), d.Evidence...)
	}
	if len(d.Reveal) > 0 {
		c.Reveal = append(append([]CostPart(nil), c.Reveal...), d.Reveal...)
	}
	if len(d.RevealOrChoose) > 0 {
		c.RevealOrChoose = append(append([]CostPart(nil), c.RevealOrChoose...), d.RevealOrChoose...)
	}
	if len(d.RevealChosen) > 0 {
		c.RevealChosen = append(append([]CostPart(nil), c.RevealChosen...), d.RevealChosen...)
	}
	c.ChooseType = AddClampedGeneric(c.ChooseType, int64(d.ChooseType))
	if len(d.Behold) > 0 {
		c.Behold = append(append([]CostPart(nil), c.Behold...), d.Behold...)
	}
	if len(d.TapPermanent) > 0 {
		c.TapPermanent = append(append([]CostPart(nil), c.TapPermanent...), d.TapPermanent...)
	}
	if len(d.UntapPermanent) > 0 {
		c.UntapPermanent = append(append([]CostPart(nil), c.UntapPermanent...), d.UntapPermanent...)
	}
	if len(d.Blight) > 0 {
		c.Blight = append(append([]CostPart(nil), c.Blight...), d.Blight...)
	}
	if len(d.Exert) > 0 {
		c.Exert = append(append([]CostPart(nil), c.Exert...), d.Exert...)
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
	if len(d.GainLife) > 0 {
		c.GainLife = append(append([]CostPart(nil), c.GainLife...), d.GainLife...)
	}
	c.Forage = c.Forage || d.Forage
	c.Waterbend = AddClampedGeneric(c.Waterbend, int64(d.Waterbend))
	c.WaterbendX = c.WaterbendX || d.WaterbendX
	if len(d.Withheld) > 0 {
		c.Withheld = append(append([]string(nil), c.Withheld...), d.Withheld...)
	}
	return c
}

// DynTapParts returns the dynamic (X/Any) TapPermanent parts of a cost --
// the tapXType heads the tap election pays (dynTapCost's doc). The literal
// tapXType<N/Spec> parts are ordinary fixed payments and never appear here.
func DynTapParts(c Cost) []CostPart {
	var out []CostPart
	for _, part := range c.TapPermanent {
		if part.Dyn != "" {
			out = append(out, part)
		}
	}
	return out
}

// CostCarriesDynTap reports whether a cost carries a dynamic tapXType part:
// the trigger-cost window's arm condition and the announce carve-outs use
// it, so a cost whose only X is a tapXType<X/Spec> election is treated the
// way a printed {X} is everywhere the engine asks "does this announce an X".
func CostCarriesDynTap(c Cost) bool {
	return len(DynTapParts(c)) > 0
}

// WithoutDynTaps strips the dynamic tapXType parts from a cost, keeping the
// literal ones (so a composed cost carrying both keeps its unpriceable half
// unpriceable). The triggered-cost window prices the non-tap rest of a
// dyn-tap cost with it.
func WithoutDynTaps(c Cost) Cost {
	if !CostCarriesDynTap(c) {
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

// CostAnnouncesCastX reports whether paying this cost announces a value for
// {X} through an announce-bearing part OTHER than a tapXType<X/Spec>
// election: a printed {X} mana symbol, a PayEnergy<X> part, an announced
// Sac<X/Spec>, SubCounter<X/Kind> or PayLife<X> part. This mirrors xAsk's
// own guard (the two must agree: the tap ask defers its X-form parts exactly
// when this is true, and xAsk bounds the announced X by the tap candidates).
// It deliberately does not fold into legal.go's costAnnouncesX, which omits
// the announced-Sac clause -- the offer gate's carve-out and this defer
// decision answer different questions and changing the offer gate's answer
// for Sac<X> costs is not this work.
func CostAnnouncesCastX(c Cost) bool {
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
	for _, part := range c.Blight {
		if part.Announced {
			return true
		}
	}
	for _, part := range c.Exile {
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
	return c.Life > 0 || c.Tap || c.Untap || len(c.Sac) > 0 || len(c.Discard) > 0 || len(c.SubCounter) > 0 || len(c.AddCounter) > 0 || len(c.Exile) > 0 || len(c.ExileFromTop) > 0 || len(c.Reveal) > 0 || len(c.RevealChosen) > 0 || len(c.Behold) > 0 || len(c.TapPermanent) > 0 || len(c.UntapPermanent) > 0 || len(c.Blight) > 0 || c.Forage || len(c.Energy) > 0 || len(c.Return) > 0 || len(c.PutToLib) > 0 || len(c.Draw) > 0 || len(c.LifeX) > 0 || len(c.DamageYou) > 0 || len(c.GainLife) > 0 || len(c.MoveToGrave) > 0 || len(c.Mill) > 0 || len(c.Exert) > 0 || c.ChooseType > 0
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
	return c.X == 0 && !c.Tap && !c.Untap && len(c.Sac) == 0 && len(c.Discard) == 0 && len(c.SubCounter) == 0 &&
		len(c.Draw) == 0 && len(c.Exile) == 0 && len(c.ExileFromTop) == 0 && len(c.Reveal) == 0 && len(c.RevealChosen) == 0 && len(c.Behold) == 0 &&
		len(c.TapPermanent) == 0 && len(c.UntapPermanent) == 0 && len(c.Blight) == 0 && !c.Forage &&
		len(c.Hybrid) == 0 && len(c.Phyrexian) == 0 && len(c.Twobrid) == 0 && len(c.HybridPhyrexian) == 0 &&
		len(c.Energy) == 0 && len(c.Return) == 0 && len(c.PutToLib) == 0 && len(c.LifeX) == 0 && len(c.DamageYou) == 0 &&
		len(c.GainLife) == 0 &&
		len(c.MoveToGrave) == 0 && len(c.Mill) == 0
}

// PureManaLife reports whether the cost is, WHOLE, a mana/life half: every
// part payMana charges (Priceable) and no component, announcement or report
// field any other stage settles. It is the strict sibling of Priceable for
// gates that settle ONE named component beside the mana/life half and must
// reject every other field explicitly -- a half-paid commitment the settle
// cannot honour is the one shape worse than the decline. Priceable reads
// only the components its own charging skips, so it lets the free AddCounter
// part (a planeswalker activation's [+N]), an Evidence part, an Exert part,
// a ChooseType count, a RevealOrChoose part, a dynamic LifeHalfUp, a
// RollDice part and the Withheld/Unknown report fields through; a gate
// settling, say, Forage beside "the rest" would then silently drop them.
func (c Cost) PureManaLife() bool {
	return c.Priceable() &&
		len(c.AddCounter) == 0 && len(c.Evidence) == 0 && len(c.Exert) == 0 &&
		c.ChooseType == 0 && len(c.RevealOrChoose) == 0 && !c.LifeHalfUp &&
		len(c.RollDice) == 0 && len(c.Withheld) == 0 && len(c.Unknown) == 0
}

// EnergyCostTotal returns the fixed energy a cost's PayEnergy<N> parts demand:
// the SUM of every fixed part, so a composed cost carrying the part several
// times (a replicated cast re-pays its PayEnergy cost once per payment) draws
// the pool down once per part rather than each spending the whole total
// independently. A dynamic PayEnergy<X> part contributes nothing here -- its
// amount is the announced X, bounded at the X ask by the payer's energy total
// (createEnergyCostX's rule) and charged as that value.
func (c Cost) EnergyCostTotal() int32 {
	total := int32(0)
	for _, part := range c.Energy {
		if part.Spec == "X" {
			continue
		}
		total += part.N
	}
	return total
}

// EnergyCostX reports whether the cost carries a dynamic PayEnergy<X> part
// whose amount is the announced X rather than a fixed N.
func (c Cost) EnergyCostX() bool {
	for _, part := range c.Energy {
		if part.Spec == "X" {
			return true
		}
	}
	return false
}

// WithoutEnergy returns the cost with its energy parts stripped, so a caller
// can judge the remaining mana/life/components by the ordinary rules (an
// energy part is charged by chargeEnergyCost, never by payMana).
func (c Cost) WithoutEnergy() Cost {
	if len(c.Energy) == 0 {
		return c
	}
	c.Energy = nil
	return c
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
// HasPips reports whether costPips would return any pip. Every pip source
// costPips reads is listed here; a cost without one resolves against only
// its life and generic totals (resolveManaWith), whatever the rider, the
// B-life grant and the conversion set are.
func (c Cost) HasPips() bool {
	return c.Colored != (state.Mana{}) || len(c.Hybrid) > 0 || len(c.Twobrid) > 0 || len(c.Phyrexian) > 0 ||
		len(c.HybridPhyrexian) > 0 || c.Snow > 0
}

// PipLetters is costPips' fixed exact-colour order (colourless last).
var PipLetters = [...]byte{'W', 'U', 'B', 'R', 'G', 'C'}

// HasManaPayment reports whether a cost's mana component is non-empty, per CR
// 601.2g's "if the total cost includes a mana payment".
func (c Cost) HasManaPayment() bool {
	return c.Colored.Total() > 0 || c.Generic > 0
}

// DropAnnouncePrefix removes the first n announcement pips (in announcePip
// order: two-colour hybrids, then monocolour hybrids, then Phyrexian, then
// hybrid-Phyrexian) from the cost, leaving the rest as the cost's live
// choices. It is how feasibleAny's per-level walk consumes one announcement
// pip at a time after folding that pip's resolved face into the cost, so a
// leaf never sees a pip slot twice.
func (c Cost) DropAnnouncePrefix(n int) Cost {
	drop := n
	if drop < len(c.Hybrid) {
		c.Hybrid = c.Hybrid[drop:]
		drop = 0
	} else {
		drop -= len(c.Hybrid)
		c.Hybrid = nil
	}
	if drop > 0 {
		if drop < len(c.Twobrid) {
			c.Twobrid = c.Twobrid[drop:]
			drop = 0
		} else {
			drop -= len(c.Twobrid)
			c.Twobrid = nil
		}
	}
	if drop > 0 {
		if drop < len(c.Phyrexian) {
			c.Phyrexian = c.Phyrexian[drop:]
			drop = 0
		} else {
			drop -= len(c.Phyrexian)
			c.Phyrexian = nil
		}
	}
	if drop > 0 {
		if drop < len(c.HybridPhyrexian) {
			c.HybridPhyrexian = c.HybridPhyrexian[drop:]
		} else {
			c.HybridPhyrexian = nil
		}
	}
	return c
}

// hybrids, the monocolour hybrids, the Phyrexian pips and the
// hybrid-Phyrexian pips (snow pips have nothing to announce).
func (c Cost) AnnPipCount() int { return c.AnnPipCountP() }

// AnnPipCountP is annPipCount without the receiver copy.
func (c *Cost) AnnPipCountP() int {
	return len(c.Hybrid) + len(c.Twobrid) + len(c.Phyrexian) + len(c.HybridPhyrexian)
}

// ManaPipCount is an upper bound on the mana units a cost can consume. Any
// minimal covering selection of window sources uses at most this many
// sources (every window source contributes at least one unit), so the
// reachability search below never has to consider picking more.
func (c Cost) ManaPipCount() int {
	n := int(c.Generic) + int(c.Colored.Total()) + int(c.Snow)
	n += len(c.Hybrid) + len(c.Phyrexian) + len(c.Twobrid) + len(c.HybridPhyrexian)
	return n
}

// PoolUnitsFloor is a lower bound on the pool units every successful
// resolveManaWith payment of c spends: its Generic plus one unit per strict
// W/U/R/G/C pip. Each such pip is paid only by a colour alternative (the
// anyColor/anyType riders and a conversion widen WHICH colour, never
// whether a unit is taken), and takeUnit removes exactly one unit of the
// remainder; the search then needs the remainder to cover a generic
// requirement that only ever grows from c.Generic. A {B} pip is left out
// (PayLifeInsteadOf:B may pay it with life), as is every hybrid, Phyrexian
// and snow pip, so the bound holds whatever the payer's grants are. A pool
// holding fewer units than this can pay nothing, which is exactly the
// answer the search would give.
func (c *Cost) PoolUnitsFloor() int64 {
	n := int64(c.Generic)
	for _, letter := range PipLetters {
		if letter == 'B' {
			continue
		}
		if k := c.Colored[state.ManaIndex(letter)]; k > 0 {
			n += int64(k)
		}
	}
	return n
}

// SubCounterTargetsSource reports whether a SubCounter part's removal-target
// field names the paying source itself: the empty field (the original
// two-field SubCounter<N/Kind> token, which has always removed from the
// source) and Forge's payCostFromSource spellings CARDNAME/NICKNAME. Any
// other value is a filter matched against the payer's battlefield.
func SubCounterTargetsSource(target string) bool {
	return subCounterTargetsSourceSet.Has(strings.ToUpper(strings.TrimSpace(target)))
}

var subCounterTargetsSourceSet = state.NewNameSet(
	"",
	"CARDNAME",
	"NICKNAME",
)
