package kgen

import (
	"errors"
	"math/bits"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// clauseFromIR maps one object alternative, its base node and the conjuncts
// after it in token order, to the lClause parseClause builds. Each conjunct
// calls the lClause setter the string reader calls for its word, so the
// setters' capacity refusals pass through unchanged.
func clauseFromIR(alt *ir.Filter, knownSub func(string) bool) (lClause, error) {
	var c lClause
	base, conj := alt, []*ir.Filter(nil)
	if alt.Op == ir.FAll {
		if len(alt.Args) < 2 {
			return c, errNoTypedReading
		}
		base, conj = alt.Args[0], alt.Args[1:]
	}
	if alt.Subject == ir.SubjPlayer {
		return c, errNoTypedReading
	}
	if err := c.baseFromIR(base, knownSub); err != nil {
		return c, err
	}
	if alt.Zones != base.Zones && (base.Zones != 0 || alt.Zones != ir.MakeZoneSet(ir.ZoneBattlefield)) {
		// inZoneBattlefield is the string reader's no-op; any other zone
		// word lowers to a PredWord7 read this table does not map
		return c, errNoTypedReading
	}
	for _, n := range conj {
		if err := c.conjFromIR(n, knownSub); err != nil {
			if historicIn(conj) {
				return c, errNoTypedReading // the string reader retries it as three clauses (historicClauses)
			}
			return c, err
		}
	}
	return c, nil
}

// historicIn reports whether a conjunct is the Historic union.
func historicIn(conj []*ir.Filter) bool {
	for _, n := range conj {
		if n.Op == ir.FAny && unionWords(n, ir.PCardType, "Artifact", ir.PSupertype, "Legendary", ir.PSubtype, "Saga") {
			return true
		}
	}
	return false
}

// baseFromIR maps an alternative's base node: Card, Permanent, Emblem,
// Spell, a modeled card type or supertype, Snow, or a pool subtype.
func (c *lClause) baseFromIR(n *ir.Filter, knownSub func(string) bool) error {
	if n.Op != ir.FPred || n.Ref != nil || n.Cmp != ir.CmpNone || n.Colors != 0 {
		return errNoTypedReading
	}
	switch {
	case n.Pred == ir.PAny && n.Subject == ir.SubjSpell:
		c.SpellBase = true
		return nil
	case n.Zones != 0:
		return errNoTypedReading
	case n.Pred == ir.PAny:
		return nil // Card
	case n.Pred == ir.PPermanentCard:
		c.Types, c.BasePermanent = tPermanent, true
		return nil
	case n.Pred == ir.PEmblem:
		c.Types = tEmblem
		return nil
	case n.Pred == ir.PSupertype && n.Word == "Snow":
		return c.setCmp(cmpWord7, 0, int(kop.W7Snow), "", "")
	case n.Pred != ir.PCardType && n.Pred != ir.PSupertype && n.Pred != ir.PSubtype:
		return errNoTypedReading
	}
	if b, ok := typeBits[n.Word]; ok {
		c.Types = b
		return nil
	}
	if knownSub(n.Word) {
		c.Subtype = n.Word
		return nil
	}
	return errNoTypedReading
}

// conjFromIR maps one conjunct: a predicate, its negation, or the Historic /
// Outlaw unions.
func (c *lClause) conjFromIR(n *ir.Filter, knownSub func(string) bool) error {
	neg := false
	if n.Op == ir.FNot && len(n.Args) == 1 {
		neg, n = true, n.Args[0]
	}
	if n.Op == ir.FAny {
		return c.unionFromIR(n, neg)
	}
	if n.Op != ir.FPred {
		return errNoTypedReading
	}
	rd, ok := conjReaders[n.Pred]
	if !ok {
		return errNoTypedReading
	}
	return rd(c, n, neg, knownSub)
}

// conjReader maps one predicate node (neg: under a FNot) onto the clause.
type conjReader func(c *lClause, n *ir.Filter, neg bool, knownSub func(string) bool) error

// conjReaders is the typed reader's predicate table.
var conjReaders map[ir.Pred]conjReader

func init() {
	conjReaders = map[ir.Pred]conjReader{
		ir.PCardType: (*lClause).typeFromIR, ir.PSupertype: (*lClause).typeFromIR, ir.PSubtype: (*lClause).subtypeFromIR,
		ir.PColor: (*lClause).colorFromIR, ir.PColorless: colorlessFromIR,
		ir.PIs: (*lClause).isFromIR, ir.PControlledBy: whoFromIR, ir.POwnedBy: whoFromIR,
		ir.PPower: cmpFromIR, ir.PToughness: cmpFromIR, ir.PManaValue: cmpFromIR, ir.PCounters: cmpFromIR,
		ir.PHasKeyword: (*lClause).keywordFromIR, ir.PName: nameFromIR, ir.PSharesName: sharesNameFromIR,
		ir.PAttacking: attackingFromIR, ir.PBlocking: blockingFromIR, ir.PFaceDown: faceDownFromIR,
		ir.PAttackedThisTurn: attackedFromIR, ir.PWasCast: wasCastFromIR,
		ir.PToken: flagPair(fToken, fNonToken), ir.PTapped: flagPair(fTapped, fUntapped),
		ir.PKicked: flagPair(fKicked, 0), ir.PUnblocked: unblockedFromIR,
		ir.PEnteredThisTurn:     flagOrNotF2(fEnteredThisTurn, kop.F2EnteredThisTurn),
		ir.PDealtDamageThisTurn: flagOrNotF2(fWasDamaged, kop.F2WasDamaged),
		ir.PMulticolored:        bareCmp(cmpMultiColor, -1), ir.PMonocolored: bareCmp(cmpMonoColor, -1),
		ir.PBlocked: bareCmp(cmpBlocked, -1), ir.PEquipped: bareCmp(cmpEquipped, -1),
		ir.PModified: bareCmp(cmpModified, -1), ir.PHasXInCost: bareCmp(cmpHasX, -1),
		ir.PEnchanted: bareCmp(cmpEnchanted, 1), ir.PHasAnyCounter: hasCountersFromIR,
	}
}

// plainPred reports whether n carries no operand beyond its Pred.
func plainPred(n *ir.Filter) bool {
	return n.Ref == nil && n.Cmp == ir.CmpNone && n.Value == nil && n.Word == "" && n.Colors == 0 && n.Zones == 0 && len(n.Args) == 0
}

// flagPair sets on (or off, under a FNot) for a plain predicate; a zero
// flag has no mapping.
func flagPair(on, off uint64) conjReader {
	return func(c *lClause, n *ir.Filter, neg bool, _ func(string) bool) error {
		f := on
		if neg {
			f = off
		}
		if f == 0 || !plainPred(n) {
			return errNoTypedReading
		}
		c.Flags |= f
		return nil
	}
}

// flagOrNotF2 is a flag word whose '!' spelling is a negated Flags2 bit
// (notF2Bit).
func flagOrNotF2(on uint64, f2 kop.FilterFlags2) conjReader {
	return func(c *lClause, n *ir.Filter, neg bool, _ func(string) bool) error {
		if !plainPred(n) {
			return errNoTypedReading
		}
		if neg {
			return c.setCmp(cmpNotF2, 0, bits.TrailingZeros32(uint32(f2)), "", "")
		}
		c.Flags |= on
		return nil
	}
}

// bareCmp is an operator-less comparison word; negN is the CmpN of its '!'
// spelling, -1 when that spelling has no mapping.
func bareCmp(stat uint8, negN int) conjReader {
	return func(c *lClause, n *ir.Filter, neg bool, _ func(string) bool) error {
		if !plainPred(n) || neg && negN < 0 {
			return errNoTypedReading
		}
		v := 0
		if neg {
			v = negN
		}
		return c.setCmp(stat, 0, v, "", "")
	}
}

func hasCountersFromIR(c *lClause, n *ir.Filter, neg bool, _ func(string) bool) error {
	if !plainPred(n) {
		return errNoTypedReading
	}
	if neg {
		return c.setCmp(cmpNoCounters, 0, 0, "", "")
	}
	return c.setCmp(cmpHasCounters, 0, 0, "", "")
}

func unblockedFromIR(c *lClause, n *ir.Filter, neg bool, _ func(string) bool) error {
	if neg || !plainPred(n) {
		return errNoTypedReading
	}
	c.Unblocked = true
	return nil
}

// typeFromIR maps a card-type or supertype conjunct: its negation is
// NotTypes (non<Type>, !<Type>); Basic is the fBasic word, Legendary the
// supertype read, a card type the narrowing the type property makes.
func (c *lClause) typeFromIR(n *ir.Filter, neg bool, knownSub func(string) bool) error {
	if n.Ref != nil || n.Word == "" {
		return errNoTypedReading
	}
	b, ok := typeBits[n.Word]
	if !ok {
		if !neg && n.Word == "Snow" {
			return c.setCmp(cmpWord7, 0, int(kop.W7Snow), "", "")
		}
		return errNoTypedReading
	}
	switch {
	case neg && b&(tAnyCard|tSupertype) != 0:
		c.NotTypes |= b
		return nil
	case neg:
		return errNoTypedReading
	case n.Word == "Basic":
		c.Flags |= fBasic
		return nil
	case n.Word == "Legendary":
		c.legendaryFromIR()
		return nil
	case b&tAnyCard != 0:
		return c.typePropFromIR(b)
	}
	return errNoTypedReading
}

// legendaryFromIR is the Legendary property beside the head's types.
func (c *lClause) legendaryFromIR() {
	switch c.Types {
	case 0, tLegendary:
		c.Types = tLegendary
	case tPermanent:
		c.Types = tLegendary
		c.NotTypes |= tInstant | tSorcery
	default:
		c.Flags |= fLegendary
	}
}

// typePropFromIR is a card-type property narrowing the clause's types.
func (c *lClause) typePropFromIR(b uint16) error {
	if c.Types&tLegendary != 0 {
		c.Types &^= tLegendary
		c.Flags |= fLegendary
	}
	if c.TypesAll != 0 {
		if b&(b-1) != 0 {
			if c.TypesAll&b != c.TypesAll {
				return errNoTypedReading
			}
			return nil
		}
		c.TypesAll |= b
		c.Types |= b
		return nil
	}
	switch {
	case c.Types == 0, c.Types&b == c.Types:
		if c.Types == 0 {
			c.Types = b
		}
	case c.Types&b == b:
		c.Types = b
	case c.Types&(c.Types-1) == 0 && b&(b-1) == 0:
		c.TypesAll = c.Types | b
		c.Types |= b
	default:
		return errNoTypedReading
	}
	return nil
}

// subtypeFromIR maps a subtype conjunct (a pool subtype word, or the chosen
// type) and its negation.
func (c *lClause) subtypeFromIR(n *ir.Filter, neg bool, knownSub func(string) bool) error {
	if n.Ref != nil {
		if !neg && isMemRef(n.Ref, ir.MemChosenType) && n.Word == "" {
			c.Flags |= fChosenType
			return nil
		}
		return errNoTypedReading
	}
	if n.Word == "" || !knownSub(n.Word) {
		return errNoTypedReading
	}
	if neg {
		return c.addNotSub(n.Word)
	}
	return c.addSubtype(n.Word)
}

// colorLetters are the kernel filter's colour letters by ir colour bit.
var colorLetters = [...]struct {
	set ir.ColorSet
	l   byte
}{{ir.ColorW, 'W'}, {ir.ColorU, 'U'}, {ir.ColorB, 'B'}, {ir.ColorR, 'R'}, {ir.ColorG, 'G'}}

// colorFromIR maps a colour conjunct (one colour, or the chosen colour).
func (c *lClause) colorFromIR(n *ir.Filter, neg bool, _ func(string) bool) error {
	if n.Ref != nil {
		if !neg && isMemRef(n.Ref, ir.MemChosenColor) && n.Colors == 0 {
			c.Flags |= fChosenColor
			return nil
		}
		return errNoTypedReading
	}
	for _, cl := range colorLetters {
		if n.Colors != cl.set {
			continue
		}
		if neg {
			c.addNotColor(cl.l)
			return nil
		}
		if c.Colors != "" {
			return errTwoColours
		}
		c.Colors = string(cl.l)
		return nil
	}
	return errNoTypedReading
}

var errTwoColours = errors.New("two colours in one clause")

// addNotColor adds one colour letter to NotColors unless it is there.
func (c *lClause) addNotColor(l byte) {
	for i := 0; i < len(c.NotColors); i++ {
		if c.NotColors[i] == l {
			return
		}
	}
	c.NotColors += string(l)
}

// colorlessFromIR: Colorless is none of the five colours, its negation
// (nonColorless) any of them.
func colorlessFromIR(c *lClause, n *ir.Filter, neg bool, _ func(string) bool) error {
	if !plainPred(n) {
		return errNoTypedReading
	}
	if neg {
		if c.Colors != "" {
			return errTwoColours
		}
		c.Colors = "WUBRG"
		return nil
	}
	for _, cl := range colorLetters {
		c.addNotColor(cl.l)
	}
	return nil
}

// isFromIR maps the identity conjuncts: Self / Other, the attached-by
// relation, and the remembered / chosen / exiled-with stores.
func (c *lClause) isFromIR(n *ir.Filter, neg bool, _ func(string) bool) error {
	r := n.Ref
	if r == nil || n.Word != "" || n.Cmp != ir.CmpNone || n.Zones != 0 {
		return errNoTypedReading
	}
	var on, off uint64
	switch {
	case isRefKind(r, ir.RefSelf):
		on, off = fSelf, fOther
	case r.Kind == ir.RefAttachedTo && isRefKind(r.Of, ir.RefSelf) && r.Filter == nil:
		// EnchantedBy, EquippedBy and AttachedBy read as one node: the
		// corpus's most common spelling (EnchantedBy) is mapped
		on, off = fEnchantedBy, fNotAttachedBy
	case isMemRef(r, ir.MemRemembered):
		on, off = fRemembered, fNotRemembered
	case isMemRef(r, ir.MemChosenCard):
		on, off = fChosenCard, fNotChosenCard
	case isMemRef(r, ir.MemExiledWithSource):
		on = fExiledWith
	}
	f := on
	if neg {
		f = off
	}
	if f == 0 {
		return errNoTypedReading
	}
	c.Flags |= f
	return nil
}

func isMemRef(r *ir.Ref, m ir.Memory) bool {
	return r != nil && r.Kind == ir.RefMemory && r.Memory == m && r.Of == nil && r.Filter == nil
}

// isOpponentsRef reports whether r is every opponent of you (OppCtrl).
func isOpponentsRef(r *ir.Ref) bool {
	if r == nil || r.Kind != ir.RefAll || r.Of != nil || r.Filter == nil {
		return false
	}
	f := r.Filter
	return isPredNode(f, ir.POpponentOf) && f.Subject == ir.SubjPlayer && isRefKind(f.Ref, ir.RefYou)
}

// whoFromIR maps a controller / owner conjunct.
func whoFromIR(c *lClause, n *ir.Filter, neg bool, _ func(string) bool) error {
	if n.Ref == nil || n.Word != "" || n.Cmp != ir.CmpNone || n.Zones != 0 {
		return errNoTypedReading
	}
	dst := &c.Ctrl
	if n.Pred == ir.POwnedBy {
		dst = &c.Owner
	}
	switch {
	case isRefKind(n.Ref, ir.RefYou) && !neg:
		return c.setWho(dst, whoYou)
	case isRefKind(n.Ref, ir.RefYou), isOpponentsRef(n.Ref) && !neg:
		return c.setWho(dst, whoOpp)
	case neg || n.Pred != ir.PControlledBy:
		return errNoTypedReading
	case isRefKind(n.Ref, ir.RefActivePlayer):
		return c.setCmp(cmpActiveCtrl, 0, 0, "", "")
	case isRefKind(n.Ref, ir.RefDefendingPlayer):
		return c.setWho(dst, whoDefending)
	}
	return errNoTypedReading
}

// cmpOpsFromIR maps ir's comparison operators to the kernel's.
var cmpOpsFromIR = [...]uint8{ir.CmpEQ: opEQ, ir.CmpNE: opNE, ir.CmpLT: opLT, ir.CmpLE: opLE, ir.CmpGT: opGT, ir.CmpGE: opGE}

// cmpFromIR maps a constant power / toughness / mana value / counter bound.
func cmpFromIR(c *lClause, n *ir.Filter, neg bool, _ func(string) bool) error {
	if neg || n.Ref != nil || n.Value == nil || n.Value.Kind != ir.VConst || n.Zones != 0 ||
		int(n.Cmp) >= len(cmpOpsFromIR) || cmpOpsFromIR[n.Cmp] == 0 {
		return errNoTypedReading
	}
	op, v := cmpOpsFromIR[n.Cmp], int(n.Value.N)
	switch n.Pred {
	case ir.PPower:
		return c.setCmp(cmpPower, op, v, "", "")
	case ir.PToughness:
		return c.setCmp(cmpToughness, op, v, "", "")
	case ir.PManaValue:
		return c.setCmp(cmpCMC, op, v, "", "")
	}
	kind := forgeCounterWord(n.Word)
	if kind == "" {
		return errNoTypedReading
	}
	return c.setCmp(cmpCounters, op, v, "", kind)
}

// forgeCounterWord is ir's counter kind in Forge's counters_ spelling.
func forgeCounterWord(w string) string {
	switch w {
	case "":
		return ""
	case "+1/+1":
		return "P1P1"
	case "-1/-1":
		return "M1M1"
	}
	return strings.ToUpper(w)
}

// nameFromIR maps named<Name> (a literal) and the NamedCard store.
func nameFromIR(c *lClause, n *ir.Filter, neg bool, _ func(string) bool) error {
	if neg || n.Cmp != ir.CmpNone || n.Zones != 0 {
		return errNoTypedReading
	}
	if n.Ref != nil {
		if !isMemRef(n.Ref, ir.MemNamedCard) || n.Word != "" {
			return errNoTypedReading
		}
		c.Flags |= fNamedCard
		return nil
	}
	if n.Word == "" {
		return errNoTypedReading
	}
	c.Flags |= fNamedAsSource
	c.Name = n.Word
	return nil
}

// sharesNameFromIR maps sameName (the source's name); beside a Remembered
// word the string reader refuses, so the read falls back to it.
func sharesNameFromIR(c *lClause, n *ir.Filter, neg bool, _ func(string) bool) error {
	if !isRefKind(n.Ref, ir.RefSelf) || n.Word != "" || c.Flags&(fRemembered|fNotRemembered) != 0 {
		return errNoTypedReading
	}
	v := 0
	if neg {
		v = 1
	}
	return c.setCmp(cmpSameNameSelf, 0, v, "", "")
}

func attackingFromIR(c *lClause, n *ir.Filter, neg bool, _ func(string) bool) error {
	switch {
	case plainPred(n) && neg:
		c.Flags |= fNotAttacking
		return nil
	case plainPred(n):
		c.Flags |= fAttacking
		return nil
	case !neg && isRefKind(n.Ref, ir.RefYou) && n.Zones == 0 && n.Word == "":
		c.Flags |= fAttacking
		return c.setCmp(cmpAttackingYou, 0, 0, "", "")
	}
	return errNoTypedReading
}

func blockingFromIR(c *lClause, n *ir.Filter, neg bool, _ func(string) bool) error {
	if !plainPred(n) {
		return errNoTypedReading
	}
	if neg {
		return c.setCmp(cmpWord7, 0, int(kop.W7NotBlocking), "", "")
	}
	c.Flags |= fBlocking
	return nil
}

// faceDownFromIR: faceDown, and its negation read as faceUp (a faceUp beside
// withDisguise is the string reader's).
func faceDownFromIR(c *lClause, n *ir.Filter, neg bool, _ func(string) bool) error {
	if !plainPred(n) {
		return errNoTypedReading
	}
	if !neg {
		return c.setCmp(cmpFaceDown, 0, 0, "", "")
	}
	if c.CmpStat == cmpWord7 {
		return errNoTypedReading
	}
	return c.setCmp(cmpWord7, 0, int(kop.W7FaceUp), "", "")
}

func attackedFromIR(c *lClause, n *ir.Filter, neg bool, _ func(string) bool) error {
	if !plainPred(n) {
		return errNoTypedReading
	}
	v := kop.PermMarkAttacked
	if neg {
		v |= kop.PermMarkNot
	}
	return c.setCmp(cmpPermMark, 0, v, "", "")
}

// wasCastFromIR maps wasCastByYou; the provenance words compose with each
// other in the string reader and stay there.
func wasCastFromIR(c *lClause, n *ir.Filter, neg bool, _ func(string) bool) error {
	if neg || n.Zones != 0 || !isRefKind(n.Ref, ir.RefYou) || n.Word != "" {
		return errNoTypedReading
	}
	c.Flags |= fCastByYou
	return nil
}

// unionFromIR maps the Historic union (artifact, legendary or Saga) and the
// Outlaw union (its five creature types) and its negation.
func (c *lClause) unionFromIR(n *ir.Filter, neg bool) error {
	switch {
	case !neg && unionWords(n, ir.PCardType, "Artifact", ir.PSupertype, "Legendary", ir.PSubtype, "Saga"):
		if c.CmpStat != cmpNone {
			return errNoTypedReading // historicClauses' split
		}
		return c.setCmp(cmpHistoric, 0, 0, "", "")
	case unionWords(n, ir.PSubtype, "Assassin", ir.PSubtype, "Mercenary", ir.PSubtype, "Pirate", ir.PSubtype, "Rogue", ir.PSubtype, "Warlock"):
		v := 0
		if neg {
			v = 1
		}
		return c.setCmp(cmpOutlaw, 0, v, "", "")
	}
	return errNoTypedReading
}

// unionWords reports whether n's arms are exactly the (Pred, Word) pairs of
// pw, in order.
func unionWords(n *ir.Filter, pw ...any) bool {
	if len(n.Args)*2 != len(pw) {
		return false
	}
	for i, a := range n.Args {
		p, _ := pw[2*i].(ir.Pred)
		w, _ := pw[2*i+1].(string)
		if !isPredNode(a, p) || a.Word != w || a.Ref != nil {
			return false
		}
	}
	return true
}

// kwWordFromIR is ir's keyword name in the with<Keyword> spelling the string
// reader reads, for the two-word names.
var kwWordFromIR = map[string]string{"First strike": "FirstStrike", "Double strike": "DoubleStrike"}

// kwWithBits are the keywords with<K> reads as PredHasKeyword; kwLacksBits
// those !with<K> / without<K> read as its negation.
var (
	kwWithBits = map[string]bool{"Defender": true, "Deathtouch": true, "Trample": true, "Haste": true, "Menace": true,
		"Vigilance": true, "Reach": true, "Lifelink": true, "Indestructible": true, "FirstStrike": true,
		"DoubleStrike": true, "Shadow": true, "Fear": true}
	kwLacksBits = map[string]bool{"Defender": true, "Deathtouch": true, "Trample": true, "Haste": true, "Menace": true,
		"Vigilance": true, "Reach": true, "Lifelink": true, "Indestructible": true, "FirstStrike": true,
		"DoubleStrike": true, "Shadow": true}
	kwWord7 = map[string]int16{"Warp": kop.W7WithWarp, "Flashback": kop.W7WithFlashback, "Infect": kop.W7WithInfect,
		"Devoid": kop.W7WithDevoid, "Skulk": kop.W7WithSkulk}
)

// keywordFromIR maps a has-keyword conjunct and its negation.
func (c *lClause) keywordFromIR(n *ir.Filter, neg bool, _ func(string) bool) error {
	if n.Ref != nil || n.Word == "" || n.Cmp != ir.CmpNone || n.Zones != 0 {
		return errNoTypedReading
	}
	w := n.Word
	if f, ok := kwWordFromIR[w]; ok {
		w = f
	}
	switch {
	case w == "Flying" && neg:
		c.Flags |= fWithoutFlying
		return nil
	case w == "Flying":
		c.Flags |= fWithFlying
		return nil
	case w == "Flanking" && neg:
		return c.setCmp(cmpWord7, 0, int(kop.W7WithoutFlanking), "", "")
	case neg && kwLacksBits[w]:
		return c.setCmp(cmpLacksKw, 0, withKwBit("with"+w), "", "")
	case neg:
		return errNoTypedReading
	case w == "Flash":
		return c.setCmp(cmpHasKw, 0, bits.TrailingZeros32(uint32(kop.KwFlash)), "", "")
	case kwWithBits[w]:
		return c.setCmp(cmpHasKw, 0, withKwBit("with"+w), "", "")
	}
	if k, ok := kwWord7[w]; ok {
		return c.setCmp(cmpWord7, 0, int(k), "", "")
	}
	return errNoTypedReading
}
