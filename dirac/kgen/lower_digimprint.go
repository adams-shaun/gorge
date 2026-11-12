package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// lower_digimprint.go: DigUntil's ImprintFound$ + RememberRevealed$ (Jodah, the
// Unifier): "exile cards from the top of your library until you
// exile a legendary nonland card with lesser mana value. You may cast that
// card without paying its mana cost. Put the rest on the bottom of your
// library in a random order." gorge effDigUntil imprints the found card and
// appends every revealed card (the found one included) to the remembered
// set; the chain then plays Defined$ Imprinted and moves Card.IsRemembered
// from exile to the library bottom with RandomOrder$ (zone_changeall.go: the
// exile scan order Fisher-Yates'd through the engine rng, then placed).
//
// The kernel's remembered set holds only the found card (RememberFound),
// which is what the Play reads, and the DigUntil marks where its cards start
// in the exile (DugMark): the bottom move (OpDiscover C=DiscoverDugBottom)
// takes every card from there still in exile and not queued to be cast, the
// same set as gorge's remembered cards left in exile. Only the exact chain
// is read: one found card, both piles to exile, then that Play, that move
// and a Cleanup.

func (lc *lowerCtx) digUntilImprint(ps *params, in lInstr, found, rev int) error {
	if !ps.is("ImprintFound", "True") || !ps.is("RememberRevealed", "True") {
		return refuse("DigUntil ImprintFound$/RememberRevealed$ apart")
	}
	if _, ok := ps.get("RememberFound"); ok {
		return refuse("DigUntil RememberFound$ beside ImprintFound$")
	}
	if in.A.Kind != vConst || in.A.N != 1 || found != zExile || rev != zExile || in.CS != "revealed=Exile" {
		return refuse("DigUntil ImprintFound$ shape")
	}
	if in.Obj.Kind != rYou {
		return refuse("DigUntil ImprintFound$ of another player's library")
	}
	n := lc.curNode.Sub
	if n == nil || n.API != "Play" || n.Params["Defined"] != "Imprinted" {
		return refuse("DigUntil ImprintFound$ without a Play of the imprinted card")
	}
	m := n.Sub
	if m == nil || m.API != "ChangeZoneAll" || m.Params["ChangeType"] != "Card.IsRemembered" || m.Params["Origin"] != "Exile" ||
		m.Params["Destination"] != "Library" || m.Params["RandomOrder"] != "True" || m.Params["LibraryPosition"] != "-1" {
		return refuse("DigUntil ImprintFound$ without the random-order bottom of the revealed cards")
	}
	if c := m.Sub; c == nil || c.API != "Cleanup" || c.Sub != nil {
		return refuse("DigUntil ImprintFound$ chain tail")
	}
	in.CS += "|RememberFound|DugMark"
	lc.digImprinted = true
	lc.emit(in)
	return nil
}

// digImprintFound lowers DigUntil's ImprintFound$ alone (Black Widow, Super
// Spy): the found cards join the imprint (the kernel's chosen set,
// DuImprintFound), revealed and found piles both to exile. Only the exact
// shape gorge's effDigUntil reads refuses: both piles to exile, a constant
// amount, the resolving controller's side of the walk, no random order and
// no RevealedLibraryPosition$ (the shape check on the assembled CS).
func (lc *lowerCtx) digImprintFound(ps *params, in lInstr, found, rev int) error {
	rem := false
	if v, ok := ps.get("RememberFound"); ok {
		// Demonlord Belzenlok: the found card joins both the remembered
		// set (the repeat's gate and its hand move) and the chosen set
		// (ImprintFound$ / Imprinted$Amount)
		if v != "True" {
			return refuse("DigUntil RememberFound$ %q beside ImprintFound$", v)
		}
		rem = true
	}
	if in.A.Kind != vConst || in.A.N < 1 || in.A.N > 127 || found != zExile || rev != zExile || in.CS != "revealed=Exile" {
		return refuse("DigUntil ImprintFound$ shape")
	}
	switch in.Obj.Kind {
	case rYou, rEachOpp, rTriggeredPlayer:
	default:
		return refuse("DigUntil ImprintFound$ of %v", in.Obj)
	}
	in.CS += "|ImprintFound"
	if rem {
		in.CS += "|RememberFound"
	}
	lc.digImprintFoundMode = true
	lc.digImprintChosen = true
	lc.emit(in)
	return nil
}

// digRememberedRevealed lowers DigUntil's RememberRevealed$ alone (Dream
// Harvest, Improvisation Capstone): every scanned card — found and
// revealed, in scan order — joins the resolution's remembered set (the
// kernel appends the scan window, DuRememberRevealed), both piles to exile.
// MinTotalCMC$ bounds the scan by the matches' cumulative mana value
// (DuMinTotalCMC, in.B). Only the exact shape gorge's effDigUntil reads
// refuses; gorge's RememberRevealed arm with RememberFound$ REPLACES the
// remembered set at the end (digRemembered), so that pair stays refused.
func (lc *lowerCtx) digRememberedRevealed(ps *params, in lInstr, found, rev int) error {
	if _, ok := ps.get("RememberFound"); ok {
		return refuse("DigUntil RememberFound$ beside RememberRevealed$")
	}
	if found == zNone && rev == zLibrary && in.CS == "revealed=Library|FoundWithRevealed" {
		return lc.digRememberedLibraryReveal(in)
	}
	if in.A.Kind != vConst || in.A.N < 1 || in.A.N > 127 || found != zExile || rev != zExile || in.CS != "revealed=Exile" {
		return refuse("DigUntil RememberRevealed$ shape")
	}
	switch in.Obj.Kind {
	case rYou, rEachOpp, rTriggeredPlayer:
	default:
		return refuse("DigUntil RememberRevealed$ of %v", in.Obj)
	}
	if v, ok := ps.get("MinTotalCMC"); ok {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 1 || n > 127 {
			return refuse("DigUntil MinTotalCMC$ %q", v)
		}
		in.B = cst(n)
		in.CS += "|MinTotalCMC"
	}
	in.CS += "|RememberRevealed"
	lc.emit(in)
	return nil
}

// digRememberedLibraryReveal is digRememberedRevealed's library form (Sanar,
// Innovative First-Year): found and revealed cards all stay in the library in
// place (the kernel's stay-in-place arm, gorge's absent
// RevealedLibraryPosition$) and every scanned card is remembered. The amount
// is a constant or the distinct colours among the controller's permanents
// (Count$Valid Permanent.YouCtrl$Colors); nothing else is accepted, and
// neither a position nor a random order (the CS check already refused them).
func (lc *lowerCtx) digRememberedLibraryReveal(in lInstr) error {
	constOK := in.A.Kind == vConst && in.A.N >= 1 && in.A.N <= 127
	colorsOK := in.A.Kind == vValidAgg && in.A.N == kop.AggColors
	if !constOK && !colorsOK {
		return refuse("DigUntil RememberRevealed$ shape")
	}
	switch in.Obj.Kind {
	case rYou, rEachOpp, rTriggeredPlayer:
	default:
		return refuse("DigUntil RememberRevealed$ of %v", in.Obj)
	}
	lc.digRememberedLibrary = true
	in.CS += "|RememberRevealed"
	lc.emit(in)
	return nil
}

// digChosenLibraryPick is Sanar's per-colour "ChangeZone | ChooseFromDefined$
// Remembered.<Colour> | Origin$ Library | Destination$ Exile | Imprint$
// True | ChangeNum$ 1": a hidden pick (not a search) of up to one remembered
// card of that colour out of the library into exile, imprinted into the
// kernel's chosen set (SrImprint). Only this exact shape lowers.
func (lc *lowerCtx) digChosenLibraryPick(ps *params, raw string) error {
	colour := strings.TrimPrefix(strings.TrimSpace(raw), "Remembered.")
	switch colour {
	case "White", "Blue", "Black", "Red", "Green":
	default:
		return refuse("ChooseFromDefined$ %q", raw)
	}
	if strings.TrimSpace(ps.str("Origin")) != "Library" || strings.TrimSpace(ps.str("Destination")) != "Exile" ||
		!ps.is("Imprint", "True") {
		return refuse("ChooseFromDefined$ shape")
	}
	n := 1
	if v, ok := ps.get("ChangeNum"); ok {
		if n, _ = strconv.Atoi(strings.TrimSpace(v)); n != 1 {
			return refuse("ChooseFromDefined$ ChangeNum$ %q", v)
		}
	}
	ps.consume("ChooseFromDefined", "Origin", "Destination", "Imprint", "ChangeNum", "LockInText")
	f, err := lc.filter("Card.IsRemembered+" + colour)
	if err != nil {
		return err
	}
	lc.digImprintChosen = true
	lc.emit(lInstr{Op: "ChooseMove", Obj: lRef{Kind: rYou}, A: cst(n), F: f, C: zExile, CS: "from=Library|Imprint|Mandatory"})
	return nil
}

func (lc *lowerCtx) digImprintBottom(ps *params) error {
	for _, k := range []string{"ChangeType", "Origin", "Destination", "RandomOrder", "LibraryPosition"} {
		ps.consume(k)
	}
	lc.emit(lInstr{Op: "DigBottom"})
	return nil
}

// digImprintFoundPut is PutCounter under a DigUntil ImprintFound$ (Black
// Widow, Super Spy's "You may put a +1/+1 counter on NICKNAME. If you
// don't, you may cast the exiled nonland card..."). The ask is gorge
// putCounterOptionalDeclined; on a decline the whole put is skipped
// (counters_put.go:73), so RememberPut$ remembers nothing and the Effect's
// ConditionDefined$ Remembered EQ0 (node's condition) lets it run; on an
// accept RememberPut$ remembers the recipient and the condition skips the
// Effect.
func (lc *lowerCtx) digImprintFoundPut(ps *params) error {
	if v, ok := ps.get("Optional"); !ok || strings.TrimSpace(v) != "True" {
		return refuse("PutCounter without Optional$ True under ImprintFound$")
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	if obj.Kind != rSelf {
		return refuse("ImprintFound PutCounter of %v", obj)
	}
	n, err := lc.amount(ps, "CounterNum", 1)
	if err != nil {
		return err
	}
	if ps.str("CounterTypes") != "" {
		return refuse("ImprintFound PutCounter CounterTypes$")
	}
	ct := ps.str("CounterType")
	if ct == "" {
		return refuse("ImprintFound PutCounter without CounterType")
	}
	in := lInstr{Op: "PutCounter", Obj: obj, A: n, CS: ct}
	if err := putCounterRememberPut(ps, &in); err != nil {
		return err
	}
	lc.emit(lInstr{Op: "MayAsk", C: kop.MayGuardPut, Skip: 1, Note: "you may"})
	lc.emit(in)
	return nil
}
