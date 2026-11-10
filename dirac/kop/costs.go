package kop

// Colour indices (pips arrays, mana pools).
const (
	W = iota
	U
	B
	R
	G
	C // colourless
	NumManaKinds
)

// CostFlags are the non-mana parts of a cost.
type CostFlags uint32

// CostFlags bits are allocated by explicit bit number (1 << n): take the
// next free bit; values are pinned by testdata/ordinals.tsv. All 32 bits of
// the uint32 are taken (bits 0-13 by the iota block below, 14-28 by the
// explicit constants after it, 29-31 by the hand-placed ones); a new flag
// needs a wider word or a sub-variant sharing a bit (CostSubCtrPick).
const (
	CostTap         CostFlags = 1 << iota // {T}
	CostUntap                             // {Q}
	CostSacSelf                           // sacrifice this
	CostSac                               // sacrifice CostN permanents matching CostFilter
	CostRemoveCtr                         // remove CostN counters of kind CostCtr from this
	CostAddCtr                            // put CostN counters of kind CostCtr on this
	CostDiscard                           // discard CostN cards
	CostPayLife                           // pay CostN life
	CostTapX                              // tap CostN untapped creatures matching CostFilter you control
	CostExileGrave                        // exile CostN cards from your graveyard
	CostSorcery                           // activate only as a sorcery
	CostOncePerTurn                       // activate only once each turn
	CostLoyalty                           // a planeswalker loyalty ability: sorcery speed, one loyalty ability per permanent per turn; the loyalty change is CostAddCtr/CostRemoveCtr with Ctr = CtrLoyalty
	CostOncePerGame                       // activate only once (per object: a permanent that leaves and returns is a new object)
)

// ManaCost is a packed mana cost: generic + WUBRG pips + colourless pips + X.
type ManaCost struct {
	Generic uint8
	Pips    [NumManaKinds]uint8 // W U B R G C
	X       uint8               // number of {X} symbols
	// XRed is the part of a generic cost reduction the printed generic part
	// could not absorb (CR 601.2f): it pays into {X} once X is chosen, so a
	// cost with XRed > 0 has Generic == 0. Only a cost with X > 0 reads it.
	XRed uint8
	// Hyb is up to four hybrid pips ({B/G}; gorge's planHybridLimit), each
	// the mask of its two colours, 0 = none (Hyb[0] first). The caster announces each pip's
	// colour before targets (CR 601.2b); a payment only ever sees the
	// resolved pips.
	Hyb [4]ColorMask
	// HybOrd records each hybrid pip's printed colour order, one bit per
	// pip (bit i for Hyb[i]): set when the pip's first printed colour is the
	// higher of its two WUBRG indexes, as {G/W} prints G first. The mask
	// alone cannot say which face the caster announces first, and gorge's
	// pip ask offers Hybrid[i].A first, so the lowering records it. 0 = the
	// mask's low colour is first.
	HybOrd uint8
	// Tb holds up to five monocoloured hybrid pips ({2/W}, CR 107.4e), three
	// bits each from bit 0 in printed order: colour+1 (W=1 .. G=5), 0 = no
	// pip. Hyb[0] is HybTwobrid while Tb != 0 (and Hyb[1] is 0), so every
	// flexible-pip gate that tests Hyb[0] routes a twobrid cost to its cold
	// branch. The caster announces each pip's face, its colour or two
	// generic (CR 601.2b); a payment only ever sees the resolved pips.
	Tb uint16
}

// Phyrexian reports whether m's Tb pips are Phyrexian.
func (m ManaCost) Phyrexian() bool { return m.Hyb[0] == HybTwobrid && m.Hyb[1] == HybPhyrexian }

// HasColour reports whether m prints a symbol of colour c (W U B R G = 0..4):
// a coloured pip, or a hybrid, monocoloured hybrid or Phyrexian pip of it
// (CR 107.4e, 107.4f).
func (m ManaCost) HasColour(c int) bool { return m.Chroma(c) != 0 }

// Chroma counts the mana symbols of m that are colour c (W U B R G = 0..4), one
// per symbol: a hybrid or Phyrexian pip counts once for either of its
// colours (gorge's ChromaSource read, Forge's getTotalChroma).
func (m ManaCost) Chroma(c int) int {
	n := int(m.Pips[c])
	if m.Hyb[0] == HybTwobrid {
		for i := 0; i < m.TbN(); i++ {
			if m.TbCol(i) == c {
				n++
			}
		}
		return n
	}
	for _, h := range m.Hyb {
		if h&(1<<uint(c)) != 0 {
			n++
		}
	}
	return n
}

// TbN is the number of unresolved twobrid pips.
func (m ManaCost) TbN() int {
	n := 0
	for t := m.Tb; t != 0; t >>= 3 {
		n++
	}
	return n
}

// TbCol is twobrid pip i's colour (W U B R G = 0..4).
func (m ManaCost) TbCol(i int) int { return int(m.Tb>>(3*uint(i))&7) - 1 }

// ResolveTb returns m with its first twobrid pip paid: as two generic
// (generic) or as its colour.
func (m ManaCost) ResolveTb(generic bool) ManaCost {
	switch {
	case !generic:
		m.Pips[m.TbCol(0)]++
	case m.Hyb[1] != HybPhyrexian:
		m.Generic += 2
		// a Phyrexian pip's life face costs no mana: its two life ride the
		// build (kernel mana_phyrexian.go)
	}
	m.Tb >>= 3
	if m.Tb == 0 {
		m.Hyb[0], m.Hyb[1] = 0, 0
	}
	return m
}

// XGeneric is the generic mana the X symbols add at X = x: x per symbol,
// less the reduction left over from the printed generic part, floored at
// zero last (CR 601.2f).
func (m ManaCost) XGeneric(x int) int {
	return max(0, x*int(m.X)-int(m.XRed))
}

// MV is the mana value (X counts 0 off the stack; a hybrid pip counts 1).
func (m ManaCost) MV() int {
	n := int(m.Generic)
	for _, p := range m.Pips {
		n += int(p)
	}
	if m.Hyb[0] == HybTwobrid {
		if m.Hyb[1] == HybPhyrexian {
			return n + m.TbN() // CR 202.3g: a Phyrexian pip counts one
		}
		return n + 2*m.TbN() // CR 202.3f
	}
	for _, h := range m.Hyb {
		if h != 0 {
			n++
		}
	}
	return n
}

// Resolve returns m with its first unresolved hybrid pip paid as colour c
// (W U B R G = 0..4).
func (m ManaCost) Resolve(c int) ManaCost {
	m.Pips[c]++
	copy(m.Hyb[:], m.Hyb[1:])
	m.Hyb[len(m.Hyb)-1] = 0
	m.HybOrd >>= 1
	return m
}

// Cost is a full cost.
type Cost struct {
	Mana   ManaCost
	Flags  CostFlags
	N      uint8 // count for Sac (0: the announced X, Sac<X/Spec>) / RemoveCtr / AddCtr / Discard / PayLife / TapX / ExileGrave
	Ctr    CounterKind
	Filter FilterID
	// Host names a cost part paid on the permanent that granted the ability
	// (an Equipment's or Aura's static): CostHostSac or CostHostExile. The CostFlags words are
	// full, so the part rides its own byte. Only a granted ability reads it.
	Host uint8
	// PutLibSelf: PutCardToLibFromGrave<1/-1/CARDNAME>, the source card in
	// its owner's graveyard moves to the bottom of that library as the
	// ability's cost (CR 118.3, 602.2b; Daniels, Temporal Agent). A table
	// byte, not a CostFlags bit (the words are full); only a graveyard
	// activated ability's own cost reads it.
	PutLibSelf bool
	// ExileHand: the CostExileGrave part (N cards matching Filter) is exiled
	// from the payer's hand, not their graveyard (ExileFromHand<1/Card>, CR
	// 118.3, 602.2b; gorge exAsk over the hand zone). A table byte, not a
	// CostFlags bit (the words are full). Read only for an exile-granted
	// ability (ExtGrantExile): its own cost beside nothing else.
	ExileHand bool
	// Exert: Exert<1/NICKNAME|CARDNAME>, the source is exerted as the cost is
	// paid (CR 118.3, 702.100a; gorge cost.parse's Exert part, rules
	// pay_ability.go). It taps nothing and leaves no counter: the payment
	// stamps the one-shot "doesn't untap during its controller's next untap
	// step" mark (pfSkipUntap). A table byte, not a CostFlags bit (the words
	// are full).
	Exert bool
	// DiscardHand: discard the whole hand as part of an activated ability's
	// cost (Discard<N/Hand>, CR 118.3, 602.2b; gorge pay.DiscardCandidates'
	// whole-hand spec discards every card in hand order, asks no choice and
	// an empty hand pays with nothing). A table byte, not a CostFlags bit
	// (the words are full).
	DiscardHand bool
	// XMin: the announced X's floor (CR 107.3, 601.2b; gorge Cost.XMin /
	// XMin$): the X ask offers nothing below it, and an offer that cannot
	// pay it is not made. 0 means no floor. A table byte, not a CostFlags
	// bit (the words are full); a uint8 as X is everywhere in the kernel.
	// Only a cost that announces X by a mana {X} or an announced sacrifice
	// (CostSac with N 0) carries it.
	XMin uint8
	// ExileBf: the CostSac part (N permanents the payer controls matching
	// Filter) is exiled, not sacrificed (Exile<N/Spec> from the battlefield,
	// CR 118.3, 602.2b, 406.2; gorge cost.parse's exileBattlefieldCost part,
	// exAsk over the battlefield zone). It is the sacrifice's pick with the
	// exile destination: no Sacrificed trigger, and a CantSacrifice static
	// does not withhold a candidate. A table byte, not a CostFlags bit (the
	// words are full). Only a printed activated ability's or a permanent
	// spell's own additional cost carries it.
	ExileBf bool
}

// CostHostSac: sacrifice the granting host (Sac<1/OriginalHost>: Deconstruction
// Hammer's granted ability). The host must be the payer's own permanent on the
// battlefield (CR 701.21a); the one candidate is settled unasked.
const CostHostSac uint8 = 1

// CostHostExile: exile the granting host (Exile<1/OriginalHost>: The Dominion
// Bracelet's granted ability). The host must be the payer's own permanent on
// the battlefield; the one candidate is settled unasked, and it moves to
// exile without being sacrificed (no Sacrificed trigger).
const CostHostExile uint8 = 2

// CostSubCtrPick: remove Cost.N counters of kind Cost.Ctr
// (or any kind for CtrAll) from the source (Cost.Filter 0) or from the
// payer's permanents matching Cost.Filter. The CostSubAmong bit
// already means this; the alias names the source-anchored and wildcard
// forms. The two CostFlags words are full (bits 30-31 are), so
// this shares bit 31 rather than taking a new one.
const CostSubCtrPick CostFlags = CostSubAmong

// PayEvidence: collect evidence A (CR 701.59a): exile any
// number of the payer's graveyard cards with total mana value A or more.
// Only a ward cost today (CardDef.WardEvidence: Axebane Ferox; gorge
// kw_ward.go beginWardPayment's CollectEvidence arm). C's nibble is full at 15.
const PayEvidence = 15

// PayBSubBlight: a PaySac whose B.N carries this sentinel
// is a Blight cost (CR 701.68), not a sacrifice. C's nibble is full at 15,
// so the sentinel rides the existing Instr.B value.
const PayBSubBlight = 1

// PayBSubEvidence: a PayManaX whose B.N carries this
// sentinel is a trigger's own CollectEvidence<A> cost (CR 701.59a), not a
// pay-{X} cost: PayEvidence (the ward's) equals PayManaX in C's full nibble.
const PayBSubEvidence = 2

// PayBSubTrigNew: a PayExileGrave whose B.N carries
// this sentinel is a dies trigger's own ExileAnyGrave<1/Card.TriggeredNewCard>
// cost (the triggered card exiled from any graveyard), not a filter exile.
const PayBSubTrigNew = 3

// PayBSubExileHand: a PayExileGrave whose B.N carries this sentinel is a
// trigger's own ExileFromHand<N/Card> cost (N cards of the payer's hand
// exiled), not a graveyard exile.
const PayBSubExileHand = 9

// PayBSubTrigNewEvidence: a PayExileGrave whose B.N carries this sentinel is
// a dies trigger's own ExileAnyGrave<1/Card.TriggeredNewCard>
// CollectEvidence<A> cost (Lamplight Phoenix): the triggered card is exiled
// first, then the evidence is collected from the rest of the graveyard.
const PayBSubTrigNewEvidence = 10

// PayBSubEvidenceX: a PayManaX whose B.N carries this sentinel is a trigger's
// own CollectEvidence<X> cost (Incinerator of the Guilty), A the cap on the
// announced X: the payer announces X over 0..the graveyard's total mana
// value, then pays (collects evidence X).
const PayBSubEvidenceX = 11

// PayBSubSacHost: a PaySacSelf whose B.N carries this sentinel is a granted
// trigger's own Sac<1/OriginalHost> cost: the Equipment or Aura whose static
// grants the trigger is sacrificed, not the creature carrying it (CR 701.21a).
const PayBSubSacHost = 7

// PayBSubStackSelf: a PayExileGrave whose B.N carries this sentinel also
// exiles the resolving spell itself from the stack, after the graveyard
// cards (UnlessCost$ ExileFromGrave<N/Spec> ExileFromStack<1/Card.Self>;
// Sphinx's Approach, gorge pay/unlessspecial.go's stack exile part). B's
// other PayExileGrave sentinels are 3, 8, 9 and 10.
const PayBSubStackSelf = 13

// PayBSubChain: a PayExileGrave whose B.N carries this sentinel is the second
// half of a trigger's own "{N} and ExileFromGrave<1/Spec>" cost. It follows the
// PayMana instruction that holds the pay or decline election and never poses an
// election of its own: the mana half is charged from the pool once its card is
// exiled (gorge's triggered-cost window settles the exile pick, then the charge).
const PayBSubChain = 8

// ReduceAbUnlock (bit 1 of a StatReduceAbility static's B): the reduction
// reaches Room unlock special actions rather than activated abilities
// (Inquisitive Glimmer's ValidSpell$ Static.Unlock).
// Bit 0 stays the Exhaust-only flag; bit 2 Power-up, bit 3 Equip; bits 4..7
// the MinMana floor.
const ReduceAbUnlock = 1 << 1

// ReduceAbPowerUp (bit 2 of a StatReduceAbility static's B): the reduction
// reaches only an activated ability with CostPowerUp, a Power-up ability
// (CR 702.193a; Hulk, Gamma Goliath's ValidSpell$ Activated.PowerUp).
// Bit 0 stays the Exhaust-only flag; bit 1 the unlock arm; bits 4..7 the
// MinMana floor.
const ReduceAbPowerUp = 1 << 2

// ReduceAbEquip (bit 3 of a StatReduceAbility static's B): the reduction
// reaches only an activated ability with AfEquip, an Equip ability (Auriok
// Steelshaper's ValidSpell$ Activated.Equip). With StTargetsFilter2 it lowers
// only an equip whose target matches Filter2, read with the static's source as
// Self (Cloud, Planet's Champion's ValidTarget$ Card.Self): the offer prices the
// cheapest legal target, the target menu each candidate, the charge the chosen
// one. Bit 0 stays the Exhaust-only flag; bit 1 the unlock arm; bit 2 Power-up;
// bits 4..7 the MinMana floor.
const ReduceAbEquip = 1 << 3

// ReduceAbMinShift is the bit position of the MinMana$ floor (0..15) inside a
// StatReduceAbility static's B; bit 0 stays the Exhaust-only flag
// (gorge pay.ApplyTo's per-reduction Floor).
const ReduceAbMinShift = 4

// ReduceColorBits is the width of one colour's count in Static.B under StReduceColor.
const ReduceColorBits = 2

// CostExileCtrlOrGrave (beside CostExileGrave): the exiled
// cards may be other permanents the payer controls as well as cards in their
// graveyard (gorge cost.parse's ExileCtrlOrGrave ZoneSet part, Craft's
// material, CR 702.167a; rules exAsk offers the battlefield candidates, then
// the graveyard's). Bit 30, clear of the iota and << 1 chains.
const CostExileCtrlOrGrave CostFlags = 1 << 30

// CostSubAmong: remove Cost.N counters of kind Cost.Ctr from
// among the permanents matching Cost.Filter that the payer controls, the
// total split however the payer chooses, one counter per pick (gorge
// RemoveAnyCounter<N/KIND/Target>, rules cast_subcounter.go
// amongCounterAsk; Hopeful Initiate). The top bit, clear of the iota and
// << 1 chains.
const CostSubAmong CostFlags = 1 << 31

// CostLoyaltyX: an announced-X counter removal, X announced 0..the counters
// of Cost.Ctr the source has and that many removed. A planeswalker's [-X]
// (SubCounter<X/LOYALTY>; CR 606.4) has CostLoyalty beside it and Cost.Ctr
// CtrLoyalty; a creature's SubCounter<X/KIND> (CR 107.3, 118.3) rides alone
// with that kind. Cost.N is 0. Bit 29, clear of the iota and << 1 chains.
const CostLoyaltyX CostFlags = 1 << 29

// PayBRemember: a PayDiscard unless-cost whose B.N
// carries this sentinel remembers the cards it discards (gorge
// effImmediateTrigger: RememberDiscarded$ True on a gated ImmediateTrigger
// appends the settled UnlessCost$ Discard<> cards to the remembered set the
// reflexive ability inherits; Ill-Timed Explosion). C's nibble is full, so
// the sentinel rides the existing Instr.B value.
const PayBRemember = 4

// PayBReveal: a PayDiscard unless-cost whose B.N carries this sentinel reveals
// A hand cards matching F instead of discarding them; nothing moves (gorge
// AdvanceUnless's "revealcost"). It is the as-enters election of a reveal land
// (dirac/repl_revealland.go), and C's nibble is full.
const PayBReveal = 5

// PayBBehold: a PayDiscard unless-cost whose B.N carries this sentinel beholds
// A permanents the payer controls or hand cards matching F (CR 701.4a); nothing
// moves. Theorist's Sanctum's as-enters election (dirac/repl_revealland.go).
const PayBBehold = 6

// PayBSubWaterbend: a PayTapX unless-cost whose B.N carries this sentinel is
// an UnlessCost$ Waterbend<A> (CR 701.67a; Waterbending Lesson): the payer's
// pay / decline, then the taps for what the floating pool cannot cover
// (dirac/cost_unlesswaterbend.go).
const PayBSubWaterbend = 12

// PayKind is the cost an OpUnlessPaid / OpIfPaid offers (C & 0xF).
const (
	PayMana    = 1 // A generic + B.N packed pips (3 bits per colour, WUBRG)
	PayDiscard = 2 // discard A cards
	PaySac     = 3 // sacrifice A permanents matching F
	PayLife    = 4 // pay A life
	PayEnergy  = 5 // pay A energy (CR 107.14)
	PayReturn  = 6 // return A permanents matching F the payer controls to their owner's hand
	PayForage  = 7 // forage (CR 701.61a): exile three graveyard cards or sacrifice a Food matching F (a trigger's own cost)
	PaySacSelf = 8 // sacrifice the resolving ability's source (Sac<1/CARDNAME>): payable while the payer controls it
	PayDraw    = 9 // draw A cards (Draw<N/You>, a trigger's own cost; always payable)
	// appended by unless-cost Discard<N/Random|Hand>
	PayDiscardRandom = 10 // discard A cards at random (the engine's RNG picks)
	PayDiscardHand   = 11 // discard the whole hand (empty hand pays with nothing)
	// appended by (parts B and C)
	PaySubCounter = 12 // remove A counters of kind B.N from the resolving ability's source (a trigger's own cost / an UnlessPayer$ You)
	PayTapX       = 13 // tap A untapped permanents the payer controls matching F (a trigger's own literal tapXType cost; the tap election is the payment)
	// appended by
	PayExileGrave = 14 // exile A cards matching F from the payer's graveyard (a trigger's own ExileFromGrave<N/Spec> cost; gorge's "exile_cost" pick)
	// appended by (Shanna): a trigger's own "pay {X}" announced first
	PayManaX = 15 // pay {X}, X announced 0..min(A, what the payer can produce, 64) before the pay election (A: the XMax$ cap; ChTrigX); the paid X is the body's Count$xPaid
)

// Cloak/ManifestDread flags (OpCloak/OpManifestDread B.N).
const (
	CfTapped          = 1 << iota // the cloaked cards enter tapped (gorge Tapped$)
	CfShuffle                     // pile shuffle; no library shuffle (gorge Shuffle$)
	CfRemember                    // the moved cards join the remembered set (RememberCloaked$/RememberManifested$)
	CfChosen                      // cloak the resolution's chosen card from where it sits, under the resolving controller (gorge effCloak's Choices$ pick, Vannifar)
	CfCloakCtrl                   // Obj names only the library's owner: the cloaked cards enter under the resolving controller (gorge's cloak(t.Obj, c.Controller), Etrata)
	CfCloakRemembered             // Defined$ Remembered: cloak each remembered card from where it sits, under the resolving controller (Expose the Culprit)
)

// CostTapHost: tap the granting host (an Equipment's granted ability).
const CostTapHost CostFlags = 1 << 14

// CostDiscardSelf: discard the source card (a hand ability: cycling, CR
// 702.29a; gorge's Discard<1/CARDNAME> cost, DiscardCostCycling).
const CostDiscardSelf CostFlags = 1 << 15

// CostForage: forage (CR 701.61a): exile three cards from your graveyard or
// sacrifice a Food (Cost.Filter: the Food you control). The either-or is
// chosen at payment (gorge forageAsk), then the three cards one pick each.
// Never combined with another counted part (Cost.N stays free).
const CostForage CostFlags = 1 << 16

// CostPayEnergy: pay N energy counters (CR 107.14, 122.1; gorge
// cost.Energy / Forge CostPayEnergy): payable when the controller has N.
const CostPayEnergy CostFlags = 1 << 17

// CostReturnSelf: return this permanent to its owner's hand (gorge
// Return<1/CARDNAME>, Forge CostReturn payCostFromSource).
const CostReturnSelf CostFlags = 1 << 18

// CostReturn: return N permanents you control matching Filter to their
// owner's hand, chosen at payment (gorge Return<N/Spec>, returnAsk).
const CostReturn CostFlags = 1 << 19

// CostExileSelf: exile this permanent (gorge Exile<1/CARDNAME>, exAsk's
// source part; Forge CostExile payCostFromSource).
const CostExileSelf CostFlags = 1 << 20

// CostUntapSelf: {Q}, untap this permanent (CR 107.6; gorge cost.Untap:
// payable while it is tapped and, a creature, not summoning sick; paid by
// an untap, a stun counter removed instead).
const CostUntapSelf CostFlags = 1 << 21

// CostTapPower: tap any number of untapped creatures you control matching
// Filter with total power N or greater (CR 702.122a Crew, 702.171a Saddle;
// gorge tapXType<Any/Spec+withTotalPowerGE<N>>, CostPart.MinPower). Paid
// by an any-number pick at bsTapCost closed by Done (gorge's KChoose
// "tapcost", Min 1, MinSum N, each option valued at its power); never
// beside CostTapX (the two share the pick record's pkTapX kind).
const CostTapPower CostFlags = 1 << 22

// CostCrew / CostSaddle mark a CostTapPower part that pays Crew (CR
// 702.122) or Saddle (CR 702.171): only then does a tapped creature's
// TapPowerValue static add to its power (graveexile). Station is neither.
// CostPowerUp marks a PowerUp$ ability (CR 702.193): once per object stint,
// and its mana cost is reduced by the permanent's mana cost on its entry
// turn (kw_powerup.go).
const (
	CostCrew    CostFlags = 1 << 23
	CostSaddle  CostFlags = 1 << 24
	CostPowerUp CostFlags = 1 << 25
)

// CostWaterbend: Waterbend<N> (CR 701.67): N generic in Cost.Mana may be
// paid by tapping that many untapped artifacts and/or creatures instead
// (each pays {1}); an offer credit reduces the mana the ability is offered
// for, and the picks are tapped at payment. An activated ability's own cost
// only, never beside {T} (gorge taps the source twice), a counted part or an
// {X} (kgen refuses all three). Waterbend<X> is the one exception to the last:
// N 0 with Mana exactly {X} means the announced X is the amount (the CostSac
// N 0 convention), each pick paying one generic of the folded X.
const CostWaterbend CostFlags = 1 << 26

// CostBlight: Blight<N> (CR 701.68): put Cost.N -1/-1 counters on a
// creature Cost.Filter you control, the source included (gorge cost.parse's
// Blight part).
const CostBlight CostFlags = 1 << 27

// CostDiscardFiltered: the CostDiscard part counts and pays only the hand
// cards matching Cost.Filter (a filtered discard cost, Discard<N/Spec> with
// a spec beyond Card/Random/Hand; CR 118.3, 601.2h; gorge pay
// DiscardCandidates over MatchesSpecFrom). A named<self> clause lowers to
// Pred = PredNamed, PredN = the card's own id (grandeur, CR 702.36).
// Alone among the counted parts (Cost.N / Cost.Filter), a battlefield
// activated ability's own cost only; kgen refuses the rest.
const CostDiscardFiltered CostFlags = 1 << 28

// EvidenceTargetsMV: an AbSpell's Ability.Evidence of this value is a
// mandatory CollectEvidence<X> additional cost, X the total mana value of
// the spell's targets (gorge cast_targets.go evidenceAmount, SVar:X
// AllTargeted$CardManaCost; Urgent Necropsy). Not a literal threshold.
const EvidenceTargetsMV = 255
