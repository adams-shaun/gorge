package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() {
	Register("TwoPiles", effTwoPiles)
}

// effTwoPiles implements Forge's TwoPilesEffect, the Fact or Fiction pile
// split (task twopiles1). The card set comes from DefinedCards$ ("Remembered"
// — the cards the upstream PeekAndReveal put in the resolution's
// Remembered — or "Targeted", the Split the Spoils/Boneyard Parley exiled
// targets); the Separator$ player splits it into pile A (a KChoose subset
// pick, Min 0 — "piles can be empty", Split the Spoils' oracle) and pile B
// (the offered-order rest); the Chooser$ player (absent → Defined$) picks
// which pile is the chosen one; the ChosenPile$ SVar body then runs with the
// chosen pile as the sub's Remembered set and UnchosenPile$ with the other —
// both are DB$ ChangeZone | Defined$ Remembered bodies, so the existing
// movement path emits the MoveZone events and nothing here touches state.
//
// Two asks, each answered in place: the split ask (pile A; pile B is the
// offered-order rest) and then the pile pick.
//
// The R-9 no-host stand-in: pile A is the FIRST card of the set and the
// chooser takes pile A — silent, byte-identical run to run, straight from
// the ordered card list. Botpolicy's KChoose default arm answers the split
// with option 0 (pile A = the first card) and the pick with option 0 (pile
// A), the same deterministic answers through the ordinary decision channel.
//
// The battlefield shape (Liliana of the Veil's -6, Do or Die, Fight or
// Flight; cards.TwoPilesBattlefieldShape) has no DefinedCards$: the card set
// is the permanents ValidCards$ matches among those the pool player controls
// (Defined$, else the targeted player), the Chooser$ defaults to that player,
// and the pile bodies run exactly as above.
//
// Scoped out (each ONE loud Note naming the unread parameter, nothing
// moves; cards.TwoPilesShapeSupported keeps coverage in step): DefinedPiles$, Zone$ other
// than that battlefield shape, FaceDown$, LeftRightPile$, a DefinedCards$
// value other than Remembered/Targeted, a ChosenPile$/UnchosenPile$ value
// that is not an SVar name of the resolving face (the TurnFaceUp/ToHand/
// ToGrave one-offs), and a Separator$/Chooser$ selector this resolver
// cannot evaluate. AILogic$/RememberChosen$/KeepRemembered$ and the
// description params are unread display/AI metadata.
func effTwoPiles(h Host, c *Ctx, sa *cards.SA) {
	for _, p := range []struct{ name, value string }{
		{"DefinedPiles", sa.ParamStr(cards.PKDefinedPiles)},
		{"LeftRightPile", sa.ParamStr(cards.PKLeftRightPile)},
	} {
		if v := strings.TrimSpace(p.value); v != "" {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unimplemented TwoPiles shape: " + p.name + "$ " + v})
			return
		}
	}
	spec := DefinedOf(sa).Cards.Text
	battlefield := false
	if spec == "" {
		if v := strings.TrimSpace(sa.ParamStr(cards.PKZone)); v != "" {
			if !cards.TwoPilesBattlefieldShape(sa) {
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
					Text: "unimplemented TwoPiles shape: Zone$ " + v})
				return
			}
			battlefield = true
		}
	}
	if !battlefield && spec != "Remembered" && spec != "Targeted" {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unimplemented TwoPiles shape: DefinedCards$ " + spec})
		return
	}
	if v := strings.TrimSpace(sa.ParamStr(cards.PKFaceDown)); v != "" && !strings.EqualFold(v, "One") {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unimplemented TwoPiles shape: FaceDown$ " + v})
		return
	}
	// The pile bodies must be SVar names on the resolving face: the core
	// carriers all point at DB$ ChangeZone/Play bodies, while the exotic
	// one-offs (TurnFaceUp, ToHand, ToGrave) name actions no SVar defines.
	// An unresolvable value emits its own Note and returns "".
	pileBodyFor := func(value, label string) string {
		v := strings.TrimSpace(value)
		if v == "" {
			return ""
		}
		if cards.ResolveSVar(c.SVars, v) == nil {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unimplemented TwoPiles shape: " + label + "$ " + v})
			return ""
		}
		return v
	}

	// The card set, in the deterministic Remembered/Targets order. Defined$
	// Remembered is Forge's host remembered list, which never contains the
	// event object the trigger fired on (rememberedExcludingCapture): a
	// Curator of Destinies ETB captures the entering Sphinx in its Ctx, and
	// reading the raw list offered the source itself as a pile card.
	var pool state.PlayerID
	var ids []state.ObjID
	g := h.Game()
	if battlefield {
		p, ok := twoPilesPoolPlayer(h, c, sa)
		if !ok {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unimplemented TwoPiles shape: the battlefield pool names no player this engine can resolve"})
			return
		}
		pool = p
		ids = twoPilesBattlefieldSet(h, c, sa, pool)
	} else {
		set := objectsOf(rememberedExcludingCapture(h, c))
		if spec == "Targeted" {
			set = objectsOf(c.Targets)
		}
		ids = make([]state.ObjID, 0, len(set))
		for _, t := range set {
			if o := g.Obj(t.Obj); o != nil {
				ids = append(ids, o.ID)
			}
		}
	}
	if len(ids) == 0 {
		// Forge returns when the card list is empty: no ask, no movement,
		// the chain continues (the fail-to-find shape must not wedge).
		return
	}

	// The players. Separator$ (absent → the Chooser$ player) splits;
	// Chooser$ (absent → Defined$) picks.
	sepSpec := strings.TrimSpace(sa.ParamStr(cards.PKSeparator))
	chooseSpec := strings.TrimSpace(sa.ParamStr(cards.PKChooser))
	if chooseSpec == "" {
		chooseSpec = DefinedRefOf(sa).Text
	}
	if sepSpec == "" {
		sepSpec = chooseSpec
	}
	var sep, chooser state.PlayerID
	ok := true
	if battlefield && chooseSpec == "" {
		// "that player's choice": the pool player picks the pile.
		chooser = pool
	} else {
		chooser, ok = twoPilesPlayer(h, c, chooseSpec)
	}
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unimplemented TwoPiles shape: Chooser$ " + chooseSpec + " names no player this engine can resolve"})
		return
	}
	if battlefield && sepSpec == "" {
		sep = chooser
	} else {
		sep, ok = twoPilesPlayer(h, c, sepSpec)
	}
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unimplemented TwoPiles shape: Separator$ " + sepSpec + " names no player this engine can resolve"})
		return
	}
	// The pile bodies: ChosenPile$ must name an SVar of the resolving face
	// (Forge's core carriers all do); UnchosenPile$ is optional — Brilliant
	// Ultimatum leaves the unchosen pile in exile. An unresolvable value
	// emitted its own Note inside pileBodyFor and stops the walk.
	chosenBody := pileBodyFor(sa.ParamStr(cards.PKChosenPile), "ChosenPile")
	if chosenBody == "" {
		return
	}
	unchosenBody := pileBodyFor(sa.ParamStr(cards.PKUnchosenPile), "UnchosenPile")
	if strings.TrimSpace(sa.ParamStr(cards.PKUnchosenPile)) != "" && unchosenBody == "" {
		return
	}

	runPile := func(name string, pile []state.ObjID) {
		if name == "" || len(pile) == 0 {
			return
		}
		sub := cards.ResolveSVar(c.SVars, name)
		if sub == nil {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unimplemented TwoPiles shape: " + name + " does not resolve"})
			return
		}
		// A FRESH Ctx carrying ONLY the pile (the Remembered aliasing
		// hazard cardflow.go documents): the pile body's ChangeZone reads
		// exactly these cards and its riders land on the fresh slice, never
		// on the outer resolution's Remembered. Child keeps the script
		// table (and the layer tables Resolve rebinds on entry anyway).
		k := c.Child(c.Source, c.Controller)
		sc := &k
		sc.Remembered = make([]state.Target, 0, len(pile))
		for _, id := range pile {
			sc.Remembered = append(sc.Remembered, state.Target{Obj: id})
		}
		Resolve(h, sc, noShuffleBody(sub))
	}

	// runPiles runs the bodies once the pick is settled: ChosenPile$ on the
	// chosen pile, UnchosenPile$ on the other.
	runPiles := func(pickB bool, pileA, pileB []state.ObjID) {
		if pickB {
			runPile(chosenBody, pileB)
			runPile(unchosenBody, pileA)
		} else {
			runPile(chosenBody, pileA)
			runPile(unchosenBody, pileB)
		}
	}

	// Pose the split ask (skipped for a one-card set — a decision
	// nobody could answer differently is never posed; pile A is that card).
	if len(ids) > 1 {
		d := &decision.Decision{Player: sep, Kind: decision.KChoose,
			Min: 0, Max: len(ids), Source: c.Source,
			ResumeKind: "twopiles_split", ResumeSA: sa,
			Prompt: "Separate these cards into two piles: pick the cards of the first pile (the rest form the second)"}
		for _, id := range ids {
			name := "a card"
			if o := g.Obj(id); o != nil && o.Face() != nil {
				name = o.Face().Name
			}
			d.Options = append(d.Options, decision.Option{Index: len(d.Options),
				Kind: "twopiles", Label: name, Obj: id, Player: sep})
		}
		// R-9 no-host stand-in: pile A is the first card of the set, silent
		// and byte-identical run to run. An answer names pile A, in answer
		// order; either way the walk continues to the pile pick.
		pileA := []state.ObjID{ids[0]}
		if ans, ok := AskTape(h, d); ok {
			pileA = counterAnswerObjs(ans)
		}

		pileA, pileB := splitSet(ids, pileA)
		pickB := twoPilesPosePick(h, c, sa, chooser)
		runPiles(pickB, pileA, pileB)
		return
	}
	// One-card set: the split ask is skipped, the chooser is still asked.
	pickB := twoPilesPosePick(h, c, sa, chooser)
	runPiles(pickB, ids[:1], nil)
}

// twoPilesPoolPlayer is the player whose permanents the battlefield shape
// separates: Defined$ when the script names one, else the first player the
// resolution targeted (Liliana's "target player").
func twoPilesPoolPlayer(h Host, c *Ctx, sa *cards.SA) (state.PlayerID, bool) {
	if spec := DefinedRefOf(sa).Text; spec != "" {
		return twoPilesPlayer(h, c, spec)
	}
	for _, t := range c.Targets {
		if t.IsPlayer && int(t.Player) < len(h.Game().Players) && !h.Game().Players[t.Player].Lost {
			return t.Player, true
		}
	}
	return 0, false
}

// twoPilesBattlefieldSet is the pool player's battlefield permanents the
// ValidCards$ spec matches, in battlefield order (deterministic).
func twoPilesBattlefieldSet(h Host, c *Ctx, sa *cards.SA, pool state.PlayerID) []state.ObjID {
	g := h.Game()
	spec := twoPilesValidCards(sa)
	var ids []state.ObjID
	for _, id := range g.Zone(state.ZBattlefield, pool) {
		if o := g.Obj(id); o != nil && o.Controller == pool &&
			MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
			ids = append(ids, id)
		}
	}
	return ids
}

// noShuffleBody is the pile body a pile movement actually resolves: the SVar
// SA with NoShuffle$ True forced on, unless the script already spoke about
// the shuffle itself.
//
// Every core carrier's pile bodies are library-origin movements (Fact or
// Fiction's DBHand/DBGrave, Sphinx of Uthuun, Steam Augury, Epiphany at the
// Drownyard, Intrude on the Mind, Unesh, Jace Architect of Thought's
// DBLibraryBottom), and the shared Origin$ Library tail
// (moveDefinedLibraryObjects / the ChangeZoneAll equivalent) fires the
// default CR 701.23d *search* shuffle on the owner's library unless the SA
// opts out. A pile split is not a search: not one of these oracle texts says
// to shuffle, so an unsuppressed tail would shuffle the caster's remaining
// library once per pile body — measured as two extra Secret Shuffle events
// on the real Fact or Fiction flow before this suppression.
//
// The opt-out is only forced when the body itself is silent: Phyrexian
// Portal's ChosenPile$ DBHand carries an explicit Shuffle$ True ("then
// shuffle the rest of that pile into your library" is in its oracle text),
// and a script that named either flag keeps what it named. Suppressing the
// shuffle cannot disturb placement: placeLibraryObjects (the UnchosenPile$
// DBLibraryBottom / LibraryPosition$ -1 shape) runs after the shuffle point,
// so the bottom-of-library pile still lands in order.
func noShuffleBody(sub *cards.SA) *cards.SA {
	if sub == nil {
		return nil
	}
	if strings.TrimSpace(sub.ParamStr(cards.PKShuffle)) != "" || strings.TrimSpace(sub.ParamStr(cards.PKNoShuffle)) != "" {
		return sub
	}
	cp := *sub
	cp.Params = make(map[string]string, len(sub.Params)+1)
	for k, v := range sub.Params {
		cp.Params[k] = v
	}
	cp.SetParam(cards.PKNoShuffle, "True")
	return &cp
}

// twoPilesPosePick poses the two-option pile pick (KChoose Min 1 Max 1,
// options "pile-a"/"pile-b" — no new decision.Kind). pickB is the pick: the
// answer (only "pile-b" picks pile B), or the no-answer stand-in's pile A.
func twoPilesPosePick(h Host, c *Ctx, sa *cards.SA, chooser state.PlayerID) (pickB bool) {
	d := &decision.Decision{Player: chooser, Kind: decision.KChoose,
		Min: 1, Max: 1, Source: c.Source,
		ResumeKind: "twopiles_pick",
		ResumeSA:   sa,
		Prompt:     "Choose a pile"}
	d.Options = append(d.Options,
		decision.Option{Index: 0, Kind: "pile-a", Label: "First pile", Player: chooser},
		decision.Option{Index: 1, Kind: "pile-b", Label: "Second pile", Player: chooser})
	if ans, ok := AskTape(h, d); ok {
		return len(ans) > 0 && ans[0].Kind == "pile-b"
	}
	return false
}

// twoPilesPlayer resolves one Separator$/Chooser$ selector to a single
// seat. "You" is the controller; everything else goes through the shared
// knownDefinedTargets resolver (Opponent → the first alive opponent in seat
// order, ChosenPlayer/TriggeredPlayer when bound) and takes the first player
// entry, deterministic in that order. An unresolvable spec reports false and
// the caller goes loud.
func twoPilesPlayer(h Host, c *Ctx, spec string) (state.PlayerID, bool) {
	if spec == "You" {
		return c.Controller, true
	}
	ts, ok := knownDefinedTargets(h, c, spec)
	if !ok {
		return 0, false
	}
	for _, t := range ts {
		if t.IsPlayer && int(t.Player) < len(h.Game().Players) && !h.Game().Players[t.Player].Lost {
			return t.Player, true
		}
	}
	return 0, false
}

// splitSet splits the offered-order card set on the answered pile-A ids:
// pile A keeps the ANSWER order, pile B the offered order minus A.
func splitSet(ids, pileA []state.ObjID) (a, b []state.ObjID) {
	a = make([]state.ObjID, 0, len(pileA))
	for _, id := range pileA {
		a = append(a, id)
	}
	return a, restOf(ids, idSet(pileA))
}

// idSet is the membership set a pile split reads.
func idSet(ids []state.ObjID) map[state.ObjID]bool {
	in := make(map[state.ObjID]bool, len(ids))
	for _, id := range ids {
		in[id] = true
	}
	return in
}

// restOf is the offered-order complement of the picked set. A stray answer
// id that is no longer in the card set is simply dropped.
func restOf(ids []state.ObjID, in map[state.ObjID]bool) []state.ObjID {
	out := make([]state.ObjID, 0, len(ids))
	for _, id := range ids {
		if !in[id] {
			out = append(out, id)
		}
	}
	return out
}
