package forge

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

var (
	refSelf = func() *ir.Ref { return &ir.Ref{Kind: ir.RefSelf} }
	refYou  = func() *ir.Ref { return &ir.Ref{Kind: ir.RefYou} }
)

func refKind(k ir.RefKind) func() *ir.Ref { return func() *ir.Ref { return &ir.Ref{Kind: k} } }

func refMem(m ir.Memory) func() *ir.Ref {
	return func() *ir.Ref { return &ir.Ref{Kind: ir.RefMemory, Memory: m} }
}

// refOpponents is every opponent of you.
func refOpponents() *ir.Ref {
	f := pred(ir.SubjPlayer, ir.POpponentOf)
	f.Ref = refYou()
	return &ir.Ref{Kind: ir.RefAll, Filter: f}
}

// refAttachedToSelf is the object the source is attached to.
func refAttachedToSelf() *ir.Ref { return &ir.Ref{Kind: ir.RefAttachedTo, Of: refSelf()} }

func withRef(n *ir.Filter, ref func() *ir.Ref) *ir.Filter {
	if ref != nil {
		n.Ref = ref()
	}
	return n
}

// --- bases --------------------------------------------------------------

// abilityBases name abilities, not objects.
var abilityBases = map[string]bool{
	"SpellAbility": true, "Activated": true, "Triggered": true, "Static": true, "Effect": true,
}

// referentBases name one context object rather than a kind of object.
var referentBases = map[string]bool{
	"TargetedCard": true, "Targeted": true, "Remembered": true, "TriggeredCard": true, "Imprinted": true,
	"CARDNAME": true, "Self": true, "Other": true, "Enchanted": true, "Equipped": true,
}

// referentBasePrefixes are the prefixes of the other context bases.
var referentBasePrefixes = []string{"Triggered", "Targeted", "Remembered", "Imprinted", "Chosen", "Defined", "Parent"}

// lowerBase reads an alternative's base word.
func lowerBase(tok string) (*ir.Filter, *Refusal) {
	n, r := lowerBaseWord(tok)
	if r != nil {
		return nil, r
	}
	setSrc(n, tok)
	return n, nil
}

func lowerBaseWord(tok string) (*ir.Filter, *Refusal) {
	obj := ir.SubjObject
	switch tok {
	case "Card":
		return pred(obj, ir.PAny), nil
	case "Permanent":
		return pred(obj, ir.PPermanentCard), nil
	case "Emblem":
		return pred(obj, ir.PEmblem), nil
	case "Spell":
		n := pred(ir.SubjSpell, ir.PAny)
		n.Zones = ir.MakeZoneSet(ir.ZoneStack)
		return n, nil
	case "Player":
		return pred(ir.SubjPlayer, ir.PAny), nil
	case "Opponent":
		return withRef(pred(ir.SubjPlayer, ir.POpponentOf), refYou), nil
	case "You":
		return withRef(pred(ir.SubjPlayer, ir.PPlayerIs), refYou), nil
	case "Any":
		return anyTarget(), nil
	}
	if n := typeWordNode(obj, tok); n != nil {
		return n, nil
	}
	switch {
	case abilityBases[tok]:
		return nil, refuse(RefuseBaseAbility, tok)
	case referentBases[tok] || hasAnyPrefix(tok, referentBasePrefixes):
		return nil, refuse(RefuseBaseReferent, tok)
	case strings.ContainsRune(tok, ' '):
		return nil, refuse(RefuseNestedFilter, tok)
	}
	return nil, refuse(RefuseUnknownWord, tok)
}

// anyTarget is "any target": a creature, planeswalker or battle on the
// battlefield, or a player (CR 115.4).
func anyTarget() *ir.Filter {
	perm := func(w string) *ir.Filter {
		n := pred(ir.SubjPermanent, ir.PCardType)
		n.Word, n.Zones = w, ir.MakeZoneSet(ir.ZoneBattlefield)
		return n
	}
	return anyRoot([]*ir.Filter{perm("Creature"), perm("Planeswalker"), perm("Battle"), pred(ir.SubjPlayer, ir.PAny)})
}

func hasAnyPrefix(s string, ps []string) bool {
	for _, p := range ps {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// typeWordNode reads a type word or non<TypeWord>; nil when tok is neither.
func typeWordNode(subj ir.Subject, tok string) *ir.Filter {
	if n := typeNode(subj, tok); n != nil {
		return n
	}
	if w, ok := strings.CutPrefix(tok, "non"); ok {
		if n := typeNode(subj, w); n != nil {
			return not(subj, n)
		}
	}
	return nil
}

func typeNode(subj ir.Subject, w string) *ir.Filter {
	var p ir.Pred
	switch ir.TypeWord(w) {
	case ir.WordCardType:
		p = ir.PCardType
	case ir.WordSupertype:
		p = ir.PSupertype
	case ir.WordSubtype:
		p = ir.PSubtype
	default:
		return nil
	}
	n := pred(subj, p)
	n.Word = w
	return n
}

// --- predicates ---------------------------------------------------------

// predAtom is one exact-word predicate row: a predicate with an optional
// reference, optionally negated.
type predAtom struct {
	p   ir.Pred
	ref func() *ir.Ref
	neg bool
	z   ir.Zone // an origin zone (PWasCast, PEnteredThisTurn)
}

// exactPreds are the predicate words read whole.
var exactPreds = map[string]predAtom{
	"Self":          {p: ir.PIs, ref: refSelf},
	"StrictlySelf":  {p: ir.PIs, ref: refSelf},
	"Other":         {p: ir.PIs, ref: refSelf, neg: true},
	"StrictlyOther": {p: ir.PIs, ref: refSelf, neg: true},
	"YouCtrl":       {p: ir.PControlledBy, ref: refYou},
	"YouDontCtrl":   {p: ir.PControlledBy, ref: refYou, neg: true},
	"OppCtrl":       {p: ir.PControlledBy, ref: refOpponents},
	"YouOwn":        {p: ir.POwnedBy, ref: refYou},
	"YouDontOwn":    {p: ir.POwnedBy, ref: refYou, neg: true},
	"OppOwn":        {p: ir.POwnedBy, ref: refOpponents},

	"ActivePlayerCtrl":                      {p: ir.PControlledBy, ref: refKind(ir.RefActivePlayer)},
	"DefenderCtrl":                          {p: ir.PControlledBy, ref: refKind(ir.RefDefendingPlayer)},
	"ControlledBy TriggeredDefendingPlayer": {p: ir.PControlledBy, ref: refKind(ir.RefDefendingPlayer)},
	"ControlledBy TriggeredPlayer":          {p: ir.PControlledBy, ref: refKind(ir.RefEventPlayer)},

	"Colorless":    {p: ir.PColorless},
	"nonColorless": {p: ir.PColorless, neg: true},
	"MultiColor":   {p: ir.PMulticolored},
	"MonoColor":    {p: ir.PMonocolored},

	"token":    {p: ir.PToken},
	"nonToken": {p: ir.PToken, neg: true},
	"tapped":   {p: ir.PTapped},
	"untapped": {p: ir.PTapped, neg: true},

	"attacking":       {p: ir.PAttacking},
	"blocking":        {p: ir.PBlocking},
	"blocked":         {p: ir.PBlocked},
	"unblocked":       {p: ir.PUnblocked},
	"attackingYou":    {p: ir.PAttacking, ref: refYou},
	"blockingSource":  {p: ir.PBlocking, ref: refSelf},
	"blockedBySource": {p: ir.PBlockedBy, ref: refSelf},
	"faceDown":        {p: ir.PFaceDown},
	"faceUp":          {p: ir.PFaceDown, neg: true},

	"equipped":    {p: ir.PEquipped},
	"enchanted":   {p: ir.PEnchanted},
	"modified":    {p: ir.PModified},
	"HasCounters": {p: ir.PHasAnyCounter},
	"kicked":      {p: ir.PKicked},
	"suspended":   {p: ir.PSuspended},
	"hasXCost":    {p: ir.PHasXInCost},
	"IsCommander": {p: ir.PIsCommander},
	"TopLibrary":  {p: ir.PTopOfLibrary},

	"EnchantedBy": {p: ir.PIs, ref: refAttachedToSelf},
	"EquippedBy":  {p: ir.PIs, ref: refAttachedToSelf},
	"AttachedBy":  {p: ir.PIs, ref: refAttachedToSelf},

	"ThisTurnEntered":        {p: ir.PEnteredThisTurn},
	"attackedThisTurn":       {p: ir.PAttackedThisTurn},
	"wasDealtDamageThisTurn": {p: ir.PDealtDamageThisTurn},

	"wasCast":                  {p: ir.PWasCast},
	"wasCastByYou":             {p: ir.PWasCast, ref: refYou},
	"wasCastFromYourHand":      {p: ir.PWasCast, z: ir.ZoneHand},
	"wasCastFromYourHandByYou": {p: ir.PWasCast, z: ir.ZoneHand, ref: refYou},
	"wasCastFromExile":         {p: ir.PWasCast, z: ir.ZoneExile},
	"wasCastFromGraveyard":     {p: ir.PWasCast, z: ir.ZoneGraveyard},
	"wasCastFromYourGraveyard": {p: ir.PWasCast, z: ir.ZoneGraveyard, ref: refYou},

	"IsRemembered":        {p: ir.PIs, ref: refMem(ir.MemRemembered)},
	"IsImprinted":         {p: ir.PIs, ref: refMem(ir.MemImprinted)},
	"IsTriggerRemembered": {p: ir.PIs, ref: refMem(ir.MemTriggerRemembered)},
	"ChosenCard":          {p: ir.PIs, ref: refMem(ir.MemChosenCard)},
	"nonChosenCard":       {p: ir.PIs, ref: refMem(ir.MemChosenCard), neg: true},
	"ExiledWithSource":    {p: ir.PIs, ref: refMem(ir.MemExiledWithSource)},
	"ChosenType":          {p: ir.PSubtype, ref: refMem(ir.MemChosenType)},
	"ChosenColor":         {p: ir.PColor, ref: refMem(ir.MemChosenColor)},
	"NamedCard":           {p: ir.PName, ref: refMem(ir.MemNamedCard)},
	"sameName":            {p: ir.PSharesName, ref: refSelf},
}

func (a predAtom) node(subj ir.Subject) *ir.Filter {
	n := withRef(pred(subj, a.p), a.ref)
	if a.z != ir.ZoneNone {
		n.Zones = ir.MakeZoneSet(a.z)
	}
	if a.neg {
		return not(subj, n)
	}
	return n
}

// colorWords are the colour words a predicate tests.
var colorWords = map[string]ir.ColorSet{
	"White": ir.ColorW, "Blue": ir.ColorU, "Black": ir.ColorB, "Red": ir.ColorR, "Green": ir.ColorG,
}

// zoneWords are Forge's zone spellings.
var zoneWords = map[string]ir.Zone{
	"Battlefield": ir.ZoneBattlefield, "Hand": ir.ZoneHand, "Library": ir.ZoneLibrary,
	"Graveyard": ir.ZoneGraveyard, "Exile": ir.ZoneExile, "Stack": ir.ZoneStack, "Command": ir.ZoneCommand,
}

// keywordNames maps Forge's keyword spelling to ir's keyword name.
var keywordNames = map[string]string{
	"Augment": "Augment", "Awaken": "Awaken", "Cascade": "Cascade", "Convoke": "Convoke",
	"Cumulative upkeep": "Cumulative upkeep", "Cumulative Upkeep": "Cumulative upkeep",
	"Cycling": "Cycling", "Deathtouch": "Deathtouch", "Decayed": "Decayed", "Defender": "Defender",
	"Disturb": "Disturb", "Double Strike": "Double strike", "Embalm": "Embalm", "Eternalize": "Eternalize",
	"Fading": "Fading", "First Strike": "First strike", "Flanking": "Flanking", "Flash": "Flash",
	"Flashback": "Flashback", "Flying": "Flying", "Forestwalk": "Forestwalk", "Foretell": "Foretell",
	"Freerunning": "Freerunning", "Haste": "Haste", "Hexproof": "Hexproof", "Horsemanship": "Horsemanship",
	"Indestructible": "Indestructible", "Infect": "Infect", "Islandwalk": "Islandwalk", "Kicker": "Kicker",
	"Level up": "Level up", "Level Up": "Level up", "Lifelink": "Lifelink", "Madness": "Madness",
	"Megamorph": "Megamorph", "Menace": "Menace", "Modular": "Modular", "Morph": "Morph",
	"Mountainwalk": "Mountainwalk", "Multikicker": "Multikicker", "Mutate": "Mutate", "Partner": "Partner",
	"Persist": "Persist", "Plainswalk": "Plainswalk", "Protection": "Protection", "Rampage": "Rampage",
	"Reach": "Reach", "Shadow": "Shadow", "Skulk": "Skulk", "Soulbond": "Soulbond", "Suspend": "Suspend",
	"Swampwalk": "Swampwalk", "Toxic": "Toxic", "Trample": "Trample", "Unearth": "Unearth",
	"Vigilance": "Vigilance", "Ward": "Ward", "Warp": "Warp",
}

// cmpOps are Forge's comparison suffixes.
var cmpOps = map[string]ir.CmpOp{
	"LE": ir.CmpLE, "GE": ir.CmpGE, "EQ": ir.CmpEQ, "LT": ir.CmpLT, "GT": ir.CmpGT, "NE": ir.CmpNE,
}

// numericPreds are the <stat><OP><int> families.
var numericPreds = []struct {
	prefix string
	p      ir.Pred
}{{"cmc", ir.PManaValue}, {"power", ir.PPower}, {"toughness", ir.PToughness}}

// needsReferencePrefixes are predicate words naming a context object or
// player outside the table.
var needsReferencePrefixes = []string{"ControlledBy ", "OwnedBy ", "Triggered", "Targeted", "Chosen", "Paired"}

// lowerPred reads one predicate token, its '!' stripped: the exact-word map
// first, then the prefix families in table order.
func lowerPred(tok string, subj ir.Subject) (*ir.Filter, *Refusal) {
	if a, ok := exactPreds[tok]; ok {
		return a.node(subj), nil
	}
	switch tok {
	case "Opponent":
		if subj != ir.SubjPlayer {
			return nil, refuse(RefuseUnknownPredicate, tok)
		}
		return withRef(pred(subj, ir.POpponentOf), refYou), nil
	case "Historic":
		return anyOf(subj, typeNode(subj, "Artifact"), typeNode(subj, "Legendary"), typeNode(subj, "Saga")), nil
	case "Outlaw":
		return anyOf(subj, typeNode(subj, "Assassin"), typeNode(subj, "Mercenary"), typeNode(subj, "Pirate"),
			typeNode(subj, "Rogue"), typeNode(subj, "Warlock")), nil
	}
	if n := typeWordNode(subj, tok); n != nil {
		return n, nil
	}
	if n := colorNode(subj, tok); n != nil {
		return n, nil
	}
	return lowerPrefixPred(tok, subj)
}

func colorNode(subj ir.Subject, tok string) *ir.Filter {
	if c, ok := colorWords[tok]; ok {
		n := pred(subj, ir.PColor)
		n.Colors = c
		return n
	}
	if w, ok := strings.CutPrefix(tok, "non"); ok {
		if c, ok := colorWords[w]; ok {
			n := pred(subj, ir.PColor)
			n.Colors = c
			return not(subj, n)
		}
	}
	return nil
}

// lowerPrefixPred reads the prefix families: numeric bounds, counters,
// keywords, entered-from, names; then the refusals by prefix.
func lowerPrefixPred(tok string, subj ir.Subject) (*ir.Filter, *Refusal) {
	for _, np := range numericPreds {
		if rest, ok := strings.CutPrefix(tok, np.prefix); ok && len(rest) >= 2 {
			if op, ok := cmpOps[rest[:2]]; ok {
				return boundNode(subj, np.p, op, rest[2:], tok)
			}
		}
	}
	if rest, ok := strings.CutPrefix(tok, "counters_"); ok {
		return countersNode(subj, rest, tok)
	}
	for _, kp := range []struct {
		prefix string
		neg    bool
	}{{"without", true}, {"with", false}, {"hasKeyword", false}} {
		if rest, ok := strings.CutPrefix(tok, kp.prefix); ok && rest != "" {
			return keywordNode(subj, rest, kp.neg, tok)
		}
	}
	if rest, ok := strings.CutPrefix(tok, "ThisTurnEnteredFrom_"); ok {
		z, ok := zoneWords[rest]
		if !ok {
			return nil, refuse(RefuseUnknownPredicate, tok)
		}
		n := pred(subj, ir.PEnteredThisTurn)
		n.Zones = ir.MakeZoneSet(z)
		return n, nil
	}
	if rest, ok := strings.CutPrefix(tok, "named"); ok && rest != "" {
		n := pred(subj, ir.PName)
		n.Word = strings.NewReplacer(";", ",", "_", " ").Replace(rest)
		return n, nil
	}
	switch {
	case hasAnyPrefix(tok, needsReferencePrefixes):
		return nil, refuse(RefuseNeedsReference, tok)
	case strings.ContainsRune(tok, ' ') || strings.Contains(tok, "Valid"):
		return nil, refuse(RefuseNestedFilter, tok)
	}
	return nil, refuse(RefuseUnknownPredicate, tok)
}

// boundNode reads a numeric bound; a bound that is not an integer names an
// SVar.
func boundNode(subj ir.Subject, p ir.Pred, op ir.CmpOp, bound, tok string) (*ir.Filter, *Refusal) {
	v, err := strconv.ParseInt(bound, 10, 32)
	if err != nil {
		return nil, refuse(RefuseBoundNamesSVar, tok)
	}
	n := pred(subj, p)
	n.Cmp, n.Value = op, &ir.Value{Kind: ir.VConst, N: int32(v)}
	return n, nil
}

// countersNode reads counters_<OP><int>_<KIND> (rest is after "counters_").
func countersNode(subj ir.Subject, rest, tok string) (*ir.Filter, *Refusal) {
	if len(rest) < 2 {
		return nil, refuse(RefuseUnknownPredicate, tok)
	}
	op, ok := cmpOps[rest[:2]]
	if !ok {
		return nil, refuse(RefuseUnknownPredicate, tok)
	}
	bound, kind, ok := strings.Cut(rest[2:], "_")
	if !ok || kind == "" {
		return nil, refuse(RefuseUnknownPredicate, tok)
	}
	n, r := boundNode(subj, ir.PCounters, op, bound, tok)
	if r != nil {
		return nil, r
	}
	w := counterKind(kind)
	if !ir.IsCounterKind(w) {
		return nil, refuse(RefuseUnknownCounter, tok)
	}
	n.Word = w
	return n, nil
}

// counterKind is Forge's counter spelling in ir's form.
func counterKind(s string) string {
	switch s {
	case "P1P1":
		return "+1/+1"
	case "M1M1":
		return "-1/-1"
	}
	return strings.ToLower(s)
}

func keywordNode(subj ir.Subject, kw string, neg bool, tok string) (*ir.Filter, *Refusal) {
	name, ok := keywordNames[kw]
	if !ok {
		return nil, refuse(RefuseUnknownKeyword, tok)
	}
	n := pred(subj, ir.PHasKeyword)
	n.Word = name
	if neg {
		return not(subj, n), nil
	}
	return n, nil
}

// lowerZonePred reads inZone<Zone>. ok reports a zone predicate; a negated or
// unknown one is a refusal.
func lowerZonePred(body, tok string, neg bool) (ir.ZoneSet, bool, *Refusal) {
	rest, ok := strings.CutPrefix(body, "inZone")
	if !ok {
		return 0, false, nil
	}
	z, ok := zoneWords[rest]
	if neg || !ok {
		return 0, false, refuse(RefuseUnknownPredicate, tok)
	}
	return ir.MakeZoneSet(z), true, nil
}
