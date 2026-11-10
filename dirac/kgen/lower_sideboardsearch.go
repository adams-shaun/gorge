package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/effects"
)

// ChangeZone from outside the game (CR 400.11): Origin$ Sideboard (North Wind
// Avatar's "put a card you own from outside the game into your hand") and
// Origin$ Library,Sideboard (Turtles Forever: search your library and/or
// outside the game for exactly four legendary creature cards with different
// names, an opponent chooses two for your hand, the rest are shuffled into
// your library). gorge searches the library's cards first, then the
// sideboard's (effSearchLibrary over the origin zones); the kernel reads the
// same pool through the sideboard flags of OpSearchEx (zone_sideboardsearch.go,
// kop.SrSideboard) and OpChangeZoneAllEx's Library,Sideboard sweep. Only the
// shapes below lower; every other parameter of such an origin refuses by name.

// sideboard origin spellings of a ChangeZone/ChangeZoneAll Origin$.
const (
	sideNone  = iota
	sideOnly  // Origin$ Sideboard
	libAndSid // Origin$ Library,Sideboard
)

// sideboardOrigin classifies the node's Origin$ without marking it read.
func sideboardOrigin(ps *params) int {
	switch strings.TrimSpace(ps.m["Origin"]) {
	case "Sideboard":
		return sideOnly
	case "Library,Sideboard":
		return libAndSid
	}
	return sideNone
}

// sideboardSearch lowers a ChangeZone whose origin names the sideboard.
func (lc *lowerCtx) sideboardSearch(ps *params, kind int) error {
	from := strings.TrimSpace(ps.str("Origin"))
	_, hasDest := ps.m["Destination"]
	switch {
	case kind == libAndSid && !hasDest:
		return lc.sideboardFind(ps, from)
	case kind == libAndSid && ps.str("Destination") == "Hand" && everyRemembered(ps.m["ChangeType"]):
		return lc.sideboardRememberedPick(ps, from)
	case kind == sideOnly && ps.str("Destination") == "Hand":
		return lc.sideboardToHand(ps, from)
	}
	if kind == sideOnly {
		return refuse("ChangeZone Origin$ Sideboard to %s", ps.str("Destination"))
	}
	return refuse("ChangeZone Origin$ Library,Sideboard to %s ChangeType$ %q", ps.str("Destination"), ps.str("ChangeType"))
}

// sideboardFilter reads ChangeType$ and ChangeNum$ of a sideboard search: a
// plain object filter (no EACH list) and a constant count.
func (lc *lowerCtx) sideboardFilter(ps *params, from string) (*lFilter, lValue, string, error) {
	ct, ok := ps.get("ChangeType")
	if !ok || strings.TrimSpace(ct) == "" {
		return nil, lValue{}, "", refuse("ChangeZone Origin$ %s without ChangeType$", from)
	}
	f, err := lc.pickFilter(ct)
	if err != nil {
		return nil, lValue{}, "", err
	}
	if f.Each {
		return nil, lValue{}, "", refuse("ChangeZone Origin$ %s with an EACH ChangeType$", from)
	}
	n, err := lc.amount(ps, "ChangeNum", 1)
	if err != nil {
		return nil, lValue{}, "", err
	}
	if !n.isConst() || n.N < 1 {
		return nil, lValue{}, "", refuse("ChangeZone Origin$ %s with ChangeNum$ %v", from, n)
	}
	return f, n, ct, nil
}

// sideboardToHand is Origin$ Sideboard, Destination$ Hand, Hidden$ True: the
// fetching player picks ChangeNum$ cards of their sideboard (Mandatory$, or a
// quantity-only filter, makes the pick compulsory: CR 701.23d), and nothing is
// shuffled because the library is not searched.
func (lc *lowerCtx) sideboardToHand(ps *params, from string) error {
	if !ps.is("Hidden", "True") {
		return refuse("ChangeZone Origin$ Sideboard without Hidden$ True")
	}
	ps.consume("Destination")
	f, n, ct, err := lc.sideboardFilter(ps, from)
	if err != nil {
		return err
	}
	flags := []string{"Side", "SideOnly"}
	if ps.is("Mandatory", "True") || !effects.SearchStatesQuality(ct) {
		flags = append(flags, "Mandatory")
	}
	lc.emit(lInstr{Op: "Search", Obj: lRef{Kind: rYou}, A: n, F: f, C: zHand, CS: strings.Join(flags, "|"),
		Note: "a pick from the controller's sideboard (outside the game)"})
	return nil
}

// sideboardFind is the first node of Turtles Forever: Origin$ Library,Sideboard
// with no Destination$. The searcher finds exactly ChangeNum$ cards with
// different names, or none (Exactly$), revealed and remembered; nothing
// moves and nothing is shuffled (gorge applyLibrarySearch: a search with no
// Destination$ remembers the found cards where they lie).
func (lc *lowerCtx) sideboardFind(ps *params, from string) error {
	for _, k := range []string{"RememberChanged", "Exactly", "Reveal", "DifferentNames"} {
		if !ps.is(k, "True") {
			return refuse("ChangeZone Origin$ %s without Destination$ needs %s$ True", from, k)
		}
	}
	if !ps.is("Shuffle", "False") {
		return refuse("ChangeZone Origin$ %s without Destination$ needs Shuffle$ False", from)
	}
	f, n, ct, err := lc.sideboardFilter(ps, from)
	if err != nil {
		return err
	}
	if !effects.SearchStatesQuality(ct) {
		return refuse("Exactly$ search of %q states no quality", ct)
	}
	lc.emit(lInstr{Op: "Search", Obj: lRef{Kind: rYou}, A: n, F: f, C: zNone, CS: "Side|Exactly|DifferentNames|Remember|NoShuffle",
		Note: "find exactly A cards of the library and the sideboard with different names, remembered where they lie"})
	return nil
}

// sideboardRememberedPick is the second node of Turtles Forever: an opponent
// (Chooser$ Opponent) takes ChangeNum$ of the remembered cards, wherever they
// lie in the library or the sideboard, into the searcher's hand (Mandatory$),
// forgetting each moved card; no shuffle (Shuffle$ False).
func (lc *lowerCtx) sideboardRememberedPick(ps *params, from string) error {
	if ps.str("Chooser") != "Opponent" {
		return refuse("ChangeZone Origin$ %s Chooser$ %q", from, ps.str("Chooser"))
	}
	for _, k := range []string{"Hidden", "NoLooking", "Mandatory", "ForgetChanged"} {
		if !ps.is(k, "True") {
			return refuse("ChangeZone Origin$ %s to Hand needs %s$ True", from, k)
		}
	}
	if !ps.is("Shuffle", "False") {
		return refuse("ChangeZone Origin$ %s to Hand needs Shuffle$ False", from)
	}
	ps.consume("Destination")
	f, n, _, err := lc.sideboardFilter(ps, from)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Search", Obj: lRef{Kind: rYou}, A: n, F: f, C: zHand, CS: "Side|ChooserOpp|Mandatory|ForgetChanged|NoShuffle",
		Note: "an opponent takes A of the remembered cards (library or sideboard) into your hand"})
	return nil
}

// originLibrarySideboard lowers a ChangeZoneAll over Origin$ Library,Sideboard
// to the Library: the remembered cards are put on top of their owner's library
// in sweep order (every library's matches, then every sideboard's), and the
// library is shuffled only if one moved (gorge changeZoneAllShuffleLibraries:
// the owners of the placements; ChangeType$ Card.IsRemembered is no
// "your own cards" sweep, so a sweep that moves nothing shuffles nobody).
func (lc *lowerCtx) originLibrarySideboard(ps *params) error {
	if ps.str("Destination") != "Library" || !everyRemembered(ps.m["ChangeType"]) || !ps.is("Shuffle", "True") {
		return refuse("ChangeZoneAll Origin$ Library,Sideboard outside Destination$ Library, ChangeType$ IsRemembered, Shuffle$ True")
	}
	for _, k := range sortedParamKeys(ps.m) {
		switch k {
		case "Origin", "Destination", "ChangeType", "Shuffle", "SubAbility":
		default:
			if !ignoredParams[k] {
				return refuse("ChangeZoneAll Origin$ Library,Sideboard with %s$", k)
			}
		}
	}
	f, err := lc.filter(ps.str("ChangeType"))
	if err != nil {
		return err
	}
	if len(f.Clauses) == 0 || f.Players != 0 {
		return refuse("ChangeZoneAll Origin$ Library,Sideboard filter shape")
	}
	lc.emit(lInstr{Op: "ChangeZoneAll", F: f, C: zLibrary, CS: "from=Library,Sideboard"})
	moved := lCond{Kind: cValue, A: lValue{Kind: vRememberedCount}, Op: opGE, B: cst(1)}
	lc.emit(lInstr{Op: "SkipIf", Cond: moved, Skip: 1, Note: "gorge shuffles only the libraries a card was placed into"})
	lc.emit(lInstr{Op: "Shuffle", Obj: lRef{Kind: rYou}})
	return nil
}
