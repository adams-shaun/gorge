package forge

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// refAtom is one atom word: the subjects it can denote and its node.
type refAtom struct {
	subj  refSubj
	build func() *ir.Ref
}

func refEvObj(role ir.EventRole, lki bool) func() *ir.Ref {
	return func() *ir.Ref { return &ir.Ref{Kind: ir.RefEventObject, Role: role, LKI: lki} }
}

func refEvPlayer(role ir.EventRole) func() *ir.Ref {
	return func() *ir.Ref { return &ir.Ref{Kind: ir.RefEventPlayer, Role: role} }
}

func refMemLKI(m ir.Memory, lki bool) func() *ir.Ref {
	return func() *ir.Ref { return &ir.Ref{Kind: ir.RefMemory, Memory: m, LKI: lki} }
}

// refTopOfLibrary is the top card of your library.
func refTopOfLibrary() *ir.Ref {
	return &ir.Ref{Kind: ir.RefTopOfLibrary, N: &ir.Value{Kind: ir.VConst, N: 1}}
}

// refAtoms are the reference words read alone. ParentTarget reads as target
// slot 0 and keeps its spelling in Src so a later pass can re-slot it.
var refAtoms = map[string]refAtom{
	"Self":                      {rsObject, refSelf},
	"You":                       {rsPlayer, refYou},
	"Targeted":                  {rsEither, refKind(ir.RefTarget)},
	"ParentTarget":              {rsEither, refKind(ir.RefTarget)},
	"TargetedPlayer":            {rsPlayer, refKind(ir.RefTarget)},
	"Enchanted":                 {rsObject, refAttachedToSelf},
	"Equipped":                  {rsObject, refAttachedToSelf},
	"TriggeredCard":             {rsObject, refEvObj(ir.RoleCard, false)},
	"TriggeredCardLKICopy":      {rsObject, refEvObj(ir.RoleCard, true)},
	"TriggeredNewCardLKICopy":   {rsObject, refEvObj(ir.RoleCard, true)},
	"TriggeredSource":           {rsObject, refEvObj(ir.RoleSource, false)},
	"TriggeredSourceLKICopy":    {rsObject, refEvObj(ir.RoleSource, true)},
	"TriggeredAttacker":         {rsObject, refEvObj(ir.RoleAttacker, false)},
	"TriggeredAttackerLKICopy":  {rsObject, refEvObj(ir.RoleAttacker, true)},
	"TriggeredBlocker":          {rsObject, refEvObj(ir.RoleBlocker, false)},
	"TriggeredBlockerLKICopy":   {rsObject, refEvObj(ir.RoleBlocker, true)},
	"TriggeredTarget":           {rsEither, refEvObj(ir.RoleTarget, false)},
	"TriggeredTargetLKICopy":    {rsEither, refEvObj(ir.RoleTarget, true)},
	"TriggeredPlayer":           {rsPlayer, refEvPlayer(ir.RoleNone)},
	"TriggeredActivator":        {rsPlayer, refEvPlayer(ir.RoleActivator)},
	"TriggeredDefendingPlayer":  {rsPlayer, refKind(ir.RefDefendingPlayer)},
	"ReplacedCard":              {rsObject, refEvObj(ir.RoleReplaced, false)},
	"ReplacedTarget":            {rsEither, refEvObj(ir.RoleReplaced, false)},
	"ReplacedPlayer":            {rsPlayer, refEvPlayer(ir.RoleReplaced)},
	"Remembered":                {rsEither, refMem(ir.MemRemembered)},
	"RememberedLKI":             {rsObject, refMemLKI(ir.MemRemembered, true)},
	"Imprinted":                 {rsObject, refMem(ir.MemImprinted)},
	"ChosenCard":                {rsObject, refMem(ir.MemChosenCard)},
	"ExiledWith":                {rsObject, refMem(ir.MemExiledWithSource)},
	"ExiledWithSource":          {rsObject, refMem(ir.MemExiledWithSource)},
	"DelayTriggerRemembered":    {rsEither, refMem(ir.MemDelayedRemembered)},
	"DelayTriggerRememberedLKI": {rsEither, refMemLKI(ir.MemDelayedRemembered, true)},
	"ChosenPlayer":              {rsPlayer, refMem(ir.MemChosenPlayer)},
	"TopOfLibrary":              {rsObject, refTopOfLibrary},
	"ActivePlayer":              {rsPlayer, refKind(ir.RefActivePlayer)},
}

// refProjections are the suffixes read on an object atom, in match order.
var refProjections = []struct {
	suffix string
	kind   ir.RefKind
}{
	{"Controller", ir.RefControllerOf},
	{"Owner", ir.RefOwnerOf},
}

// refStorePrefixes start the words that name a per-source store.
var refStorePrefixes = []string{"DelayTriggerRemembered", "DirectRemembered", "Remembered", "Imprinted", "Chosen", "ExiledWith"}

// lowerRefAtom builds the node of an atom word, with Src set.
func lowerRefAtom(word string) (*ir.Ref, refSubj, bool) {
	a, ok := refAtoms[word]
	if !ok {
		return nil, 0, false
	}
	r := a.build()
	setRefSrc(r, word)
	return r, a.subj, true
}

// lowerRefWord reads one word: an atom, a projection of an object atom, or
// "<A>And<B>" of two atoms.
func lowerRefWord(word string) (*ir.Ref, refSubj, *Refusal) {
	if r, subj, ok := lowerRefAtom(word); ok {
		return r, subj, nil
	}
	if namesAbility(word) {
		return nil, 0, refuse(RefuseNeedsAbilityRef, word)
	}
	for _, p := range refProjections {
		base, ok := strings.CutSuffix(word, p.suffix)
		if !ok {
			continue
		}
		if of, subj, ok := lowerRefAtom(base); ok && subj&rsObject != 0 {
			r := &ir.Ref{Kind: p.kind, Of: of, Src: word}
			return r, rsPlayer, nil
		}
	}
	if r, subj, ok := lowerRefAnd(word); ok {
		return r, subj, nil
	}
	for _, pre := range refStorePrefixes {
		if strings.HasPrefix(word, pre) {
			return nil, 0, refuse(RefuseNeedsMemoryKind, word)
		}
	}
	return nil, 0, refuse(RefuseUnknownReferent, word)
}

// lowerRefAnd reads "<A>And<B>" with both sides atoms as their union; it can
// denote only what both sides can.
func lowerRefAnd(word string) (*ir.Ref, refSubj, bool) {
	for i := strings.Index(word, "And"); i > 0; {
		a, sa, okA := lowerRefAtom(word[:i])
		b, sb, okB := lowerRefAtom(word[i+len("And"):])
		if okA && okB && sa&sb != 0 {
			return &ir.Ref{Kind: ir.RefUnion, Union: []*ir.Ref{a, b}, Src: word}, sa & sb, true
		}
		j := strings.Index(word[i+1:], "And")
		if j < 0 {
			break
		}
		i += 1 + j
	}
	return nil, 0, false
}

// namesAbility reports whether word names an ability or a spell ability on
// the stack rather than an object or a player.
func namesAbility(word string) bool {
	return word == "Parent" || strings.Contains(word, "SpellAbility") || strings.Contains(word, "SA")
}
