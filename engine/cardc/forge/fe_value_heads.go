package forge

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// count reads a Count$ head (its operator already cut off): the exact heads
// first, then the prefix families in table order, then the two-branch state
// heads.
func (rd *valueReader) count(head string) (*ir.Value, *Refusal) {
	if mk, ok := exactHeads[head]; ok {
		return mk(), nil
	}
	for _, fam := range headFamilies {
		if rest, ok := strings.CutPrefix(head, fam.prefix); ok {
			return fam.read(rd, rest)
		}
	}
	if word, ok := twoBranchHead(head); ok {
		return nil, refuse(RefuseNeedsCondition, word)
	}
	return nil, refuse(RefuseUnknownHead, headWord(head))
}

func of(k ir.ValueKind, ref func() *ir.Ref) func() *ir.Value {
	return func() *ir.Value {
		v := &ir.Value{Kind: k}
		if ref != nil {
			v.Ref = ref()
		}
		return v
	}
}

// exactHeads are the Count$ heads that take no argument.
var exactHeads = map[string]func() *ir.Value{
	"xPaid":                 of(ir.VX, nil),
	"CardPower":             of(ir.VPower, refSelf),
	"CardToughness":         of(ir.VToughness, refSelf),
	"CardManaCost":          of(ir.VManaValue, refSelf),
	"YourLifeTotal":         of(ir.VLife, refYou),
	"YourStartingLife":      of(ir.VStartingLife, refYou),
	"LifeYouGainedThisTurn": of(ir.VLifeGainedThisTurn, refYou),
	"ChosenNumber":          of(ir.VChosenNumber, nil),
	"RememberedSize":        of(ir.VCountReceipt, refMem(ir.MemRemembered)),
	"RememberedNumber":      of(ir.VReceiptAmount, refMem(ir.MemRemembered)),
	"TriggerRememberAmount": of(ir.VEventAmount, nil),
	"CardsInYourHand":       cardsInYourHand,
	"Domain":                of(ir.VDomain, refYou),
	"Party":                 of(ir.VPartySize, refYou),
	"Converge":              of(ir.VConverge, nil),
	"AttackersDeclared":     of(ir.VAttackersDeclared, nil),
	"TimesKicked":           of(ir.VTimesKicked, nil),
	"YouDrewThisTurn":       of(ir.VCardsDrawnThisTurn, refYou),
}

func cardsInYourHand() *ir.Value {
	f := withRef(pred(ir.SubjObject, ir.PControlledBy), refYou)
	f.Zones = ir.MakeZoneSet(ir.ZoneHand)
	return &ir.Value{Kind: ir.VCount, Filter: f}
}

// headFamily is one Count$ head family that takes an argument.
type headFamily struct {
	prefix string
	read   func(rd *valueReader, rest string) (*ir.Value, *Refusal)
}

// headFamilies are the Count$ heads that take an argument, by prefix. It is
// set in init: compare reads its operands through the reader that reads it.
var headFamilies []headFamily

func init() { headFamilies = headFamilyTable() }

func headFamilyTable() []headFamily {
	return []headFamily{
		{"Valid ", validIn(ir.ZoneBattlefield)},
		{"ValidBattlefield ", validIn(ir.ZoneBattlefield)},
		{"ValidGraveyard ", validIn(ir.ZoneGraveyard)},
		{"ValidHand ", validIn(ir.ZoneHand)},
		{"ValidLibrary ", validIn(ir.ZoneLibrary)},
		{"ValidExile ", validIn(ir.ZoneExile)},
		{"CardCounters.", cardCounters},
		{"Devotion.", devotion},
		{"DevotionDual.", devotion},
		{"Compare ", compare},
		{"ThisTurnCast_", castThisTurn},
		{"ThisTurnEntered_", enteredThisTurn},
	}
}

// validIn counts the objects in zone matching the filter.
func validIn(zone ir.Zone) func(*valueReader, string) (*ir.Value, *Refusal) {
	return func(_ *valueReader, rest string) (*ir.Value, *Refusal) {
		f, r := LowerFilter(rest)
		if r != nil {
			return nil, r
		}
		setScope(f, ir.MakeZoneSet(zone))
		return &ir.Value{Kind: ir.VCount, Filter: f}, nil
	}
}

// setScope gives f, and each alternative of an FAny root with no zones of its
// own, the scope zones. A player filter has no zone.
func setScope(f *ir.Filter, zones ir.ZoneSet) {
	if f.Subject == ir.SubjPlayer {
		return
	}
	if f.Op == ir.FAny {
		for _, a := range f.Args {
			if a.Zones == 0 && a.Subject != ir.SubjPlayer {
				a.Zones = zones
			}
		}
	}
	f.Zones = zones
}

func cardCounters(_ *valueReader, kind string) (*ir.Value, *Refusal) {
	w := counterKind(kind)
	if !ir.IsCounterKind(w) {
		return nil, refuse(RefuseUnknownCounter, "CardCounters."+kind)
	}
	return &ir.Value{Kind: ir.VCountersAmong, Ref: refSelf(), Word: w}, nil
}

func devotion(_ *valueReader, rest string) (*ir.Value, *Refusal) {
	var cs ir.ColorSet
	for _, w := range strings.Split(rest, ".") {
		c, ok := colorWords[w]
		switch {
		case w == "Chosen":
			return nil, refuse(RefuseNeedsReference, "Devotion.Chosen")
		case !ok:
			return nil, refuse(RefuseUnknownHead, "Devotion."+w)
		}
		cs |= c
	}
	return &ir.Value{Kind: ir.VDevotion, Ref: refYou(), Colors: cs}, nil
}

// compare reads "<a> <OP><b>.<then>.<else>".
func compare(rd *valueReader, rest string) (*ir.Value, *Refusal) {
	i := strings.LastIndexByte(rest, ' ')
	if i < 0 || len(rest)-i-1 < 2 {
		return nil, refuse(RefuseUnknownHead, "Compare")
	}
	tail := rest[i+1:]
	cmp, ok := cmpOps[tail[:2]]
	parts := strings.Split(tail[2:], ".")
	if !ok || len(parts) != 3 {
		return nil, refuse(RefuseUnknownHead, "Compare")
	}
	vals := make([]*ir.Value, 0, 4)
	for _, s := range append([]string{rest[:i]}, parts...) {
		v, r := rd.text(s)
		if r != nil {
			return nil, r
		}
		vals = append(vals, v)
	}
	test := &ir.Cond{Kind: ir.CCompare, L: vals[0], R: vals[1], Cmp: cmp}
	return &ir.Value{Kind: ir.VIf, Test: test, Args: vals[2:]}, nil
}

func castThisTurn(_ *valueReader, rest string) (*ir.Value, *Refusal) {
	f, r := LowerFilter(rest)
	if r != nil {
		return nil, r
	}
	return &ir.Value{Kind: ir.VCastThisTurn, Filter: f}, nil
}

// enteredThisTurn reads "<Zone>[_from_<Zone>]_<filter>".
func enteredThisTurn(_ *valueReader, rest string) (*ir.Value, *Refusal) {
	to, rest, _ := strings.Cut(rest, "_")
	dest, ok := zoneWords[to]
	if !ok {
		return nil, refuse(RefuseUnknownHead, "ThisTurnEntered_"+to)
	}
	var origin ir.ZoneSet
	if after, ok := strings.CutPrefix(rest, "from_"); ok {
		from, filter, _ := strings.Cut(after, "_")
		z, ok := zoneWords[from]
		if !ok {
			return nil, refuse(RefuseUnknownHead, "ThisTurnEntered_from_"+from)
		}
		origin, rest = ir.MakeZoneSet(z), filter
	}
	f, r := LowerFilter(rest)
	if r != nil {
		return nil, r
	}
	setScope(f, ir.MakeZoneSet(dest))
	return &ir.Value{Kind: ir.VEnteredThisTurn, Filter: f, Zones: origin}, nil
}

// twoBranchHead reports a "<State>.<a>.<b>" head and its state word.
func twoBranchHead(head string) (string, bool) {
	parts := strings.Split(head, ".")
	if len(parts) != 3 || !isName(parts[0]) || parts[1] == "" || parts[2] == "" {
		return "", false
	}
	return parts[0], true
}

// headWord is a head's leading word, before its argument.
func headWord(head string) string {
	if i := strings.IndexAny(head, " ._"); i >= 0 {
		return head[:i]
	}
	return head
}
