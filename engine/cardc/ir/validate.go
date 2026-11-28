package ir

import "fmt"

// wordClass is the closed list a predicate's Word must come from.
type wordClass uint8

const (
	wcNone wordClass = iota
	wcCardType
	wcSupertype
	wcSubtype
	wcKeyword
	wcCounterKind
	wcName // free text
)

var wordClassNames = [...]string{
	wcNone:        "none",
	wcCardType:    "card type",
	wcSupertype:   "supertype",
	wcSubtype:     "subtype",
	wcKeyword:     "keyword",
	wcCounterKind: "counter kind",
	wcName:        "name",
}

// has reports whether w is in c's list; a name is any text.
func (c wordClass) has(w string) bool {
	switch c {
	case wcCardType:
		return TypeWord(w) == WordCardType
	case wcSupertype:
		return TypeWord(w) == WordSupertype
	case wcSubtype:
		return TypeWord(w) == WordSubtype
	case wcKeyword:
		return IsKeyword(w)
	case wcCounterKind:
		return IsCounterKind(w)
	case wcName:
		return true
	}
	return false
}

// refUse says whether a predicate takes a Ref, and of which kind.
type refUse uint8

const (
	refNo     refUse = iota
	refAny           // any reference
	refMemory        // only RefMemory, and only instead of Word/Colors
)

// operands is one predOperands row: the operands a predicate may carry. Every
// operand is optional; set marks a row as written.
type operands struct {
	set    bool
	word   wordClass
	ref    refUse
	colors bool
	cmp    bool // Cmp with Value
	args   bool // an inner filter with its own subject
	zones  bool // Zones as an origin, not the scope
}

var (
	opNone  = operands{set: true}
	opRef   = operands{set: true, ref: refAny}
	opCmp   = operands{set: true, cmp: true}
	opZones = operands{set: true, zones: true}
)

// predOperands says which operands each Pred takes.
var predOperands = [predCount]operands{
	PAny:                 opNone,
	PCardType:            {set: true, word: wcCardType},
	PSubtype:             {set: true, word: wcSubtype, ref: refMemory},
	PSupertype:           {set: true, word: wcSupertype},
	PColor:               {set: true, colors: true, ref: refMemory},
	PColorless:           opNone,
	PMulticolored:        opNone,
	PPermanentCard:       opNone,
	PManaValue:           opCmp,
	PPower:               opCmp,
	PToughness:           opCmp,
	PControlledBy:        opRef,
	POwnedBy:             opRef,
	PIs:                  opRef,
	PToken:               opNone,
	PTapped:              opNone,
	PAttacking:           opRef,
	PBlocking:            opRef,
	PHasKeyword:          {set: true, word: wcKeyword},
	PCounters:            {set: true, word: wcCounterKind, cmp: true},
	POpponentOf:          opRef,
	PPlayerIs:            opRef,
	PAttackedThisTurn:    opNone,
	PControls:            {set: true, args: true},
	PLifeTotal:           opCmp,
	PLostLifeThisTurn:    opNone,
	PCardsInGraveyard:    opCmp,
	PCardsInHand:         opCmp,
	PControlsMoreThan:    {set: true, ref: refAny, args: true},
	PName:                {set: true, word: wcName, ref: refMemory},
	PKicked:              opNone,
	PSurged:              opNone,
	PEmblem:              opNone,
	PMonocolored:         opNone,
	PBlocked:             opNone,
	PUnblocked:           opNone,
	PBlockedBy:           opRef,
	PFaceDown:            opNone,
	PEquipped:            opNone,
	PEnchanted:           opNone,
	PModified:            opNone,
	PHasAnyCounter:       opNone,
	PIsCommander:         opNone,
	PTopOfLibrary:        opNone,
	PWasCast:             {set: true, zones: true, ref: refAny},
	PEnteredThisTurn:     opZones,
	PDealtDamageThisTurn: opNone,
	PSuspended:           opNone,
	PHasXInCost:          opNone,
	PSharesName:          opRef,
}

// ValidateFilter checks f's shape: each combinator has its args and no
// predicate operands, each predicate carries only the operands predOperands
// allows with each Word from its closed list, and every node shares the
// root's Subject (zero inherits it) and scope Zones, except that an FAny
// alternative with its own Subject or Zones is a root of its own. An inner filter (the Args
// of PControls and PControlsMoreThan, a Ref's Filter) is a root of its own.
func ValidateFilter(f *Filter) error {
	if f == nil {
		return fmt.Errorf("ir: nil filter")
	}
	return validateNode(f, f)
}

func validateNode(root, f *Filter) error {
	if f == nil {
		return fmt.Errorf("ir: nil filter")
	}
	label := f.Op.String()
	if f.Op == FPred && f.Pred != PNone {
		label = f.Pred.String()
	}
	if err := checkScope(root, f, label); err != nil {
		return err
	}
	switch f.Op {
	case FAll, FAny, FNot:
		return validateCombinator(root, f, label)
	case FPred:
		return validatePred(f, label)
	}
	return fmt.Errorf("ir: %s: no op", label)
}

// checkScope holds a non-root node to the root's Subject and scope Zones.
func checkScope(root, f *Filter, label string) error {
	if f == root {
		return nil
	}
	if f.Subject != SubjNone && f.Subject != root.Subject {
		return fmt.Errorf("ir: %s: subject %s differs from the root's %s", label, f.Subject, root.Subject)
	}
	originZones := f.Op == FPred && f.Pred < predCount && predOperands[f.Pred].zones
	if !originZones && f.Zones != 0 && f.Zones != root.Zones {
		return fmt.Errorf("ir: %s: takes no zones (%s is not the root's scope %q)", label, f.Zones, root.Zones)
	}
	return nil
}

func validateCombinator(root, f *Filter, label string) error {
	if f.Op == FNot && len(f.Args) != 1 {
		return fmt.Errorf("ir: %s: needs exactly one arg, has %d", label, len(f.Args))
	}
	if len(f.Args) == 0 {
		return fmt.Errorf("ir: %s: needs at least one arg", label)
	}
	if f.Pred != PNone || f.Word != "" || f.Colors != 0 || f.Ref != nil || f.Cmp != CmpNone || f.Value != nil {
		return fmt.Errorf("ir: %s: carries predicate operands", label)
	}
	for _, a := range f.Args {
		next := root
		if f.Op == FAny && opensScope(root, a) {
			next = a
		}
		if err := validateNode(next, a); err != nil {
			return err
		}
	}
	return nil
}

// opensScope reports whether a, an alternative of an FAny, carries a Subject
// or scope Zones of its own: "any target" is a permanent on the battlefield or
// a player (CR 115.4), so each alternative is a root of its own.
func opensScope(root, a *Filter) bool {
	if a == nil {
		return false
	}
	return (a.Subject != SubjNone && a.Subject != root.Subject) || (a.Zones != 0 && a.Zones != root.Zones)
}

func validatePred(f *Filter, label string) error {
	if f.Pred == PNone {
		return fmt.Errorf("ir: %s: needs a predicate", label)
	}
	if f.Pred >= predCount || !predOperands[f.Pred].set {
		return fmt.Errorf("ir: %s: unknown predicate", label)
	}
	row := predOperands[f.Pred]
	if f.Word != "" {
		if row.word == wcNone {
			return fmt.Errorf("ir: %s: takes no word", label)
		}
		if !row.word.has(f.Word) {
			return fmt.Errorf("ir: %s: %q is not a %s", label, f.Word, wordClassNames[row.word])
		}
	}
	if f.Colors != 0 && !row.colors {
		return fmt.Errorf("ir: %s: takes no colors", label)
	}
	if err := checkPredRef(f, row, label); err != nil {
		return err
	}
	if (f.Cmp != CmpNone || f.Value != nil) && !row.cmp {
		return fmt.Errorf("ir: %s: takes no comparison", label)
	}
	if (f.Cmp != CmpNone) != (f.Value != nil) {
		return fmt.Errorf("ir: %s: cmp and value go together", label)
	}
	if len(f.Args) > 0 && !row.args {
		return fmt.Errorf("ir: %s: takes no args", label)
	}
	for _, a := range f.Args {
		if err := ValidateFilter(a); err != nil {
			return err
		}
	}
	return nil
}

func checkPredRef(f *Filter, row operands, label string) error {
	if f.Ref == nil {
		return nil
	}
	switch row.ref {
	case refNo:
		return fmt.Errorf("ir: %s: takes no ref", label)
	case refMemory:
		if f.Word != "" || f.Colors != 0 {
			return fmt.Errorf("ir: %s: takes a word or a memory ref, not both", label)
		}
		if f.Ref.Kind != RefMemory {
			return fmt.Errorf("ir: %s: ref must be a memory ref, not %s", label, f.Ref.Kind)
		}
	}
	return validateRefFilters(f.Ref)
}

// validateRefFilters validates every filter reachable through r's references.
func validateRefFilters(r *Ref) error {
	if r == nil {
		return nil
	}
	if r.Filter != nil {
		if err := ValidateFilter(r.Filter); err != nil {
			return err
		}
	}
	if err := validateRefFilters(r.Of); err != nil {
		return err
	}
	for _, u := range r.Union {
		if err := validateRefFilters(u); err != nil {
			return err
		}
	}
	return nil
}
