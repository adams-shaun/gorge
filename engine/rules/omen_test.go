package rules

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// omen_test.go pins the CR 720 Omen spell face end to end. The main-path and
// timing tests run on the REAL corpus cards (Twinmaw Stormbrood // Charring
// Bite and Runescale Stormbrood // Chilling Screech); the counter and
// reversal edges run on inline fixtures (never Forge text, per the licensing
// rule). Every test ends in a replayCheck.

// omenFixtureSrc is an authored Omen card: a Sorcery main face over a
// Sorcery Omen face that deals damage to a creature. The front is deliberately
// castable so the reversal test can price it under a tax.
const omenFixtureSrc = "Name:Dusk Herald\nManaCost:1 B\nTypes:Sorcery\n" +
	"AlternateMode:Omen\nOracle:front\nALTERNATE\nName:Grim Tidings\nManaCost:1 B\n" +
	"Types:Sorcery Omen\nA:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 2 | SpellDescription$ Grim Tidings deals 2 damage to target creature.\nOracle:back\n"

const omenCancelSrc = "Name:Omen Cancel\nManaCost:1 U U\nTypes:Instant\n" +
	"A:SP$ Counter | TargetType$ Spell | TgtPrompt$ Select target spell | ValidTgts$ Card\nOracle:x\n"

const omenBearSrc = "Name:Omen Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// omenTaxSrc: an Omen card whose main face is a {1}{B} Sorcery and whose Omen
// face is the same price, so a "noncreature spells cost {1} more" tax can
// force the Omen cast to reverse after it is proposed.
const omenTaxSrc = "Name:Dusk Ledger\nManaCost:1 B\nTypes:Sorcery\n" +
	"AlternateMode:Omen\nOracle:front\nALTERNATE\nName:Grim Ledger\nManaCost:1 B\n" +
	"Types:Sorcery Omen\nA:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 2 | SpellDescription$ Grim Ledger deals 2 damage to target creature.\nOracle:back\n"

// omenTaxWardenSrc: Thalia's shape -- noncreature spells cost {1} more.
const omenTaxWardenSrc = "Name:Omen Tax Warden\nManaCost:1 W\nTypes:Creature Human Soldier\nPT:2/1\n" +
	"S:Mode$ RaiseCost | ValidCard$ Card.nonCreature | Type$ Spell | Amount$ 1 | Description$ Noncreature spells cost {1} more to cast.\nOracle:x\n"

// omenOption returns the cast option for id whose Mode is exactly mode, or
// nil when none is offered.
func omenOption(t *testing.T, e *Engine, id state.ObjID, mode string) *decision.Option {
	t.Helper()
	opts := castOptions(t, e)
	for i := range opts {
		o := &opts[i]
		if o.Obj == id && o.Mode == mode {
			return o
		}
	}
	return nil
}

// omenCorpusEngine deals seat 0 a 40-card deck led by Twinmaw Stormbrood and
// all Forests, and seat 1 a deck of Grizzly Bears, one of which is moved onto
// the battlefield as Charring Bite's target. The returned id is the Twinmaw
// in seat 0's hand. Corpus cards only -- no Forge script text is committed
// here.
func omenCorpusEngine(t *testing.T, reg *cards.Registry) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	twinmaw := searchCorpusCard(t, reg, "Twinmaw Stormbrood")
	forest := searchCorpusCard(t, reg, "Forest")
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	deck := []*cards.Card{twinmaw}
	for len(deck) < 40 {
		deck = append(deck, forest)
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = bear
	}
	cfg := seatZeroStart(Config{Seed: 7201, Names: []string{"omen", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	oppBear := e.G.Zone(state.ZLibrary, 1)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: oppBear, From: state.ZLibrary, To: state.ZBattlefield})
	e.pending = nil
	id := searchMoveByName(t, e, "Twinmaw Stormbrood", state.ZHand)
	return e, cfg, id, oppBear
}

// TestOmenFaceOfferedFromHandAndShuffledOnResolve is the main-path pin on the
// real corpus card: the hand offers the Omen face as a separate cast option;
// casting it poses its target ask, resolves, and shuffles the card into its
// owner's library as its main face (CR 720.3d, CR 720.4), never the
// graveyard. The shuffle is logged once, after the move, and the front face
// cast option remains available throughout.
func TestOmenFaceOfferedFromHandAndShuffledOnResolve(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg, id, oppBear := omenCorpusEngine(t, reg)
	addMana(t, e, 0, "RR")
	plain := omenOption(t, e, id, "")
	alt := omenOption(t, e, id, "omen_alt")
	if plain != nil {
		t.Fatalf("the {5}{W} front must not be offered with only {R}{R}: %+v", plain)
	}
	if alt == nil || alt.Label != "Cast Charring Bite" {
		t.Fatalf("omen_alt offer missing/renamed: %+v", castOptions(t, e))
	}
	start := len(e.L.Events)

	submitChoices(t, e, alt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("Charring Bite target ask: %+v", d)
	}
	tgt := -1
	for _, o := range d.Options {
		if o.Obj == oppBear {
			tgt = o.Index
		}
	}
	if tgt < 0 {
		t.Fatalf("no option targeting the bear: %+v", d.Options)
	}
	submitChoices(t, e, tgt)
	passUntilStackEmpty(t, e, 20)

	o := e.G.Obj(id)
	if o.Zone != state.ZLibrary {
		t.Fatalf("resolved Omen zone=%s, want ZLibrary", o.Zone)
	}
	if int(o.FaceIdx) != 0 {
		t.Fatalf("resolved Omen faceIdx=%d, want 0 (CR 720.4)", o.FaceIdx)
	}
	if o.Owner != 0 {
		t.Fatalf("resolved Omen owner=%d, want its owner 0", o.Owner)
	}
	if got := e.G.Zone(state.ZGraveyard, 0); len(got) != 0 {
		t.Fatalf("resolved Omen left seat 0's graveyard non-empty: %v", got)
	}
	shuffles := 0
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.Shuffle && ev.Player == 0 {
			shuffles++
		}
	}
	if shuffles != 1 {
		t.Fatalf("shuffle events for the resolving Omen = %d, want 1", shuffles)
	}
	if hasNote(e, "cast aborted") {
		t.Fatal("the offered Omen cast aborted")
	}
	replayCheck(t, e, cfg)
}

// TestOmenUsesOmenTimingAndCost pins CR 720.2's timing split. A Sorcery Omen
// face is not offered on an opponent's turn even with its cost funded, while
// the Instant Omen face is offered mid-stack with exactly {1}{U} while the
// {3}{R} main face is unaffordable.
func TestOmenUsesOmenTimingAndCost(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)

	// Sorcery Omen (Twinmaw Stormbrood // Charring Bite): offered on the
	// controller's own main phase, never on the opponent's.
	t.Run("sorcery", func(t *testing.T) {
		e, cfg, id, _ := omenCorpusEngine(t, reg)
		addMana(t, e, 0, "WWRR")
		if got := omenOption(t, e, id, "omen_alt"); got == nil {
			t.Fatalf("the Sorcery Omen face must be offered on its controller's turn: %+v", castOptions(t, e))
		}
		driveToStep(t, e, 2, 1, state.StepMain1)
		passOnce(t, e)
		addMana(t, e, 0, "WWRR")
		if got := omenOption(t, e, id, "omen_alt"); got != nil {
			t.Fatalf("the Sorcery Omen face was offered on the opponent's turn: %+v", got)
		}
		replayCheck(t, e, cfg)
	})

	// Instant Omen (Runescale Stormbrood // Chilling Screech): the front costs
	// {3}{R} and is not offered with {U}{U}{R}; the Omen face costs {1}{U} and
	// is offered while a spell is on the stack.
	t.Run("instant", func(t *testing.T) {
		runescale := searchCorpusCard(t, reg, "Runescale Stormbrood")
		forest := searchCorpusCard(t, reg, "Forest")
		bear := searchCorpusCard(t, reg, "Grizzly Bears")
		bolt := searchCorpusCard(t, reg, "Lightning Bolt")
		deck := []*cards.Card{runescale, bolt}
		for len(deck) < 40 {
			deck = append(deck, forest)
		}
		opp := make([]*cards.Card, 40)
		for i := range opp {
			opp[i] = bear
		}
		cfg := seatZeroStart(Config{Seed: 7202, Names: []string{"omen", "opponent"},
			Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
		e := New(cfg)
		e.Advance()
		toMain1(t, e)
		oppBear := e.G.Zone(state.ZLibrary, 1)[0]
		e.emit(events.Event{Kind: events.MoveZone, Obj: oppBear, From: state.ZLibrary, To: state.ZBattlefield})
		e.pending = nil
		id := searchMoveByName(t, e, "Runescale Stormbrood", state.ZHand)
		boltID := searchMoveByName(t, e, "Lightning Bolt", state.ZHand)
		addMana(t, e, 0, "UUR")
		if got := omenOption(t, e, id, ""); got != nil {
			t.Fatalf("the {3}{R} front was offered with only {U}{U}{R}: %+v", got)
		}

		// Put a mana-value-1 spell on the stack: Chilling Screech's only legal
		// target shape.
		submitChoices(t, e, castOptionFor(t, e, boltID).Index)
		d := e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("Lightning Bolt target ask: %+v", d)
		}
		tgt := -1
		for _, o := range d.Options {
			if o.Obj == oppBear {
				tgt = o.Index
			}
		}
		if tgt < 0 {
			t.Fatalf("no option targeting the bear: %+v", d.Options)
		}
		submitChoices(t, e, tgt)
		alt := omenOption(t, e, id, "omen_alt")
		if alt == nil || alt.Label != "Cast Chilling Screech" {
			t.Fatalf("the Instant Omen face must be offered while a spell is on the stack: %+v", castOptions(t, e))
		}
		if got := omenOption(t, e, id, ""); got != nil {
			t.Fatalf("the {3}{R} front was offered with only {U}{R} left: %+v", got)
		}
		replayCheck(t, e, cfg)
	})
}

// TestOmenCounteredGoesToGraveyard pins the fizzle boundary: a countered Omen
// spell NEVER resolved, so CR 720.3d's shuffle does not apply -- it goes to
// its owner's graveyard as its main face, and nothing is shuffled.
func TestOmenCounteredGoesToGraveyard(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 7203, omenFixtureSrc, omenCancelSrc, omenBearSrc)
	cancelID := addToHand(t, e, 0, omenCancelSrc)
	bearID := putCreature(t, e, 0, omenBearSrc)
	addMana(t, e, 0, "BBUUU")
	alt := omenOption(t, e, id, "omen_alt")
	if alt == nil {
		t.Fatalf("omen_alt offer missing: %+v", castOptions(t, e))
	}
	start := len(e.L.Events)
	submitChoices(t, e, alt.Index)
	subMid := e.Pending()
	if subMid == nil || subMid.Kind != decision.KTarget {
		t.Fatalf("Grim Tidings target ask: %+v", subMid)
	}
	tgt := -1
	for _, o := range subMid.Options {
		if o.Obj == bearID {
			tgt = o.Index
		}
	}
	if tgt < 0 {
		t.Fatalf("no option targeting the bear: %+v", subMid.Options)
	}
	submitChoices(t, e, tgt)

	// The Omen spell sits on the stack, paid for. Counter it before anyone
	// passes priority into resolution.
	addMana(t, e, 0, "UU")
	submitChoices(t, e, castOptionFor(t, e, cancelID).Index)
	subMid = e.Pending()
	if subMid == nil || subMid.Kind != decision.KTarget {
		t.Fatalf("no target decision for the counter: %+v", subMid)
	}
	tgt = -1
	for _, o := range subMid.Options {
		if o.Obj == id {
			tgt = o.Index
		}
	}
	if tgt < 0 {
		t.Fatalf("no option targeting the Omen spell: %+v", subMid.Options)
	}
	submitChoices(t, e, tgt)
	passUntilStackEmpty(t, e, 20)

	o := e.G.Obj(id)
	if o.Zone != state.ZGraveyard {
		t.Fatalf("countered Omen zone=%s, want ZGraveyard", o.Zone)
	}
	if int(o.FaceIdx) != 0 {
		t.Fatalf("countered Omen faceIdx=%d, want 0", o.FaceIdx)
	}
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.Shuffle && ev.Player == 0 {
			t.Fatalf("a countered Omen shuffled its owner's library: %+v", ev)
		}
	}
	replayCheck(t, e, cfg)
}

// TestOmenCardIsMainFaceOffTheStack pins CR 720.4 in both directions: a
// resolved Omen rests in the library as its main face (FaceIdx 0), and a
// proposed Omen cast reversed under CR 733.1 returns to hand as its main face
// -- the proposal's own flip is undone. The reversal is a cost abort: the
// offer prices the Omen face under the tax, so the only way to begin this
// cast is directly.
func TestOmenCardIsMainFaceOffTheStack(t *testing.T) {
	t.Parallel()

	t.Run("resolved into the library", func(t *testing.T) {
		reg := searchTestRegistry(t)
		e, cfg, id, oppBear := omenCorpusEngine(t, reg)
		addMana(t, e, 0, "RR")
		alt := omenOption(t, e, id, "omen_alt")
		if alt == nil {
			t.Fatalf("omen_alt offer missing: %+v", castOptions(t, e))
		}
		submitChoices(t, e, alt.Index)
		d := e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("Charring Bite target ask: %+v", d)
		}
		for _, o := range d.Options {
			if o.Obj == oppBear {
				submitChoices(t, e, o.Index)
				passUntilStackEmpty(t, e, 20)
				got := e.G.Obj(id)
				if got.Zone != state.ZLibrary || int(got.FaceIdx) != 0 || got.Face().Name != "Twinmaw Stormbrood" {
					t.Fatalf("resolved Omen card: zone=%s face=%d %q, want library as the main face Twinmaw Stormbrood",
						got.Zone, got.FaceIdx, got.Face().Name)
				}
				replayCheck(t, e, cfg)
				return
			}
		}
		t.Fatalf("no option targeting the bear: %+v", d.Options)
	})

	t.Run("reversal restores the main face", func(t *testing.T) {
		e, cfg, id := newFixtureDeck(t, 7204, omenTaxSrc, omenTaxWardenSrc)
		putCreature(t, e, 0, omenTaxWardenSrc)
		addMana(t, e, 0, "BB")
		if got := omenOption(t, e, id, "omen_alt"); got != nil {
			t.Fatalf("the taxed Omen face ({2}{B}) was offered with only {B}{B}: %+v", got)
		}
		opt := decision.Option{Kind: "cast", Obj: id, Mode: "omen_alt"}
		e.beginCast(0, opt)
		if e.cast != nil || !hasNote(e, "cast aborted: cost no longer payable") {
			t.Fatalf("the taxed Omen cast must reverse: cast=%+v", e.cast)
		}
		o := e.G.Obj(id)
		if o.Zone != state.ZHand || int(o.FaceIdx) != 0 {
			t.Fatalf("reversal must restore the card as its main face: zone=%s face=%d", o.Zone, o.FaceIdx)
		}
		e.pending = nil
		e.priorityRound()
		replayCheck(t, e, cfg)
	})
}

// omenCensusDeck builds seat 0 a deck led by one Omen card plus the cards the
// census board needs: a Lightning Bolt (the cmc<=2 spell Chilling Screech
// counters) and a Grizzly Bears (the creature Dynamic Soar pumps), all over
// basic Forests so the library searches find lands.
func omenCensusDeck(t *testing.T, reg *cards.Registry, omen *cards.Card) []*cards.Card {
	t.Helper()
	deck := []*cards.Card{omen, searchCorpusCard(t, reg, "Lightning Bolt"), searchCorpusCard(t, reg, "Grizzly Bears")}
	forest := searchCorpusCard(t, reg, "Forest")
	for len(deck) < 40 {
		deck = append(deck, forest)
	}
	return deck
}

// omenCensusEngine builds the shared census board: the Omen card in seat 0's
// hand, a creature of seat 0's on the battlefield (Dynamic Soar's
// Creature.YouCtrl target), and a Grizzly Bears of seat 1's on the
// battlefield (the other faces' target). It also leaves Lightning Bolt in
// seat 0's hand so the instant that needs a spell on the stack can cast it.
func omenCensusEngine(t *testing.T, reg *cards.Registry, omen *cards.Card) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = bear
	}
	cfg := seatZeroStart(Config{Seed: 7205, Names: []string{"omen", "opponent"},
		Decks: [][]*cards.Card{omenCensusDeck(t, reg, omen), opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	oppBear := e.G.Zone(state.ZLibrary, 1)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: oppBear, From: state.ZLibrary, To: state.ZBattlefield})
	e.pending = nil
	searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
	id := searchMoveByName(t, e, omen.Faces[0].Name, state.ZHand)
	return e, cfg, id, oppBear
}

// omenDrain resolves the stack, answering every mid-resolution ask with the
// package's bot policy (so searches, X values, discards and targets all get a
// legal answer) and passing priority. Unlike passUntilStackEmpty it is happy
// with the full range of asks the Omen faces pose.
func omenDrain(t *testing.T, e *Engine, limit int) {
	t.Helper()
	for n := 0; n < limit && !e.G.Over && len(e.G.Stack) > 0; n++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining the Omen stack (depth %d)", len(e.G.Stack))
		}
		if d.Kind == decision.KPriority {
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("priority decision with no pass option: %+v", d)
			}
			submitChoices(t, e, idx)
			continue
		}
		in := newTestBot(uint64(n+1)).answer(e, d)
		if err := e.Submit(in); err != nil {
			t.Fatalf("submit bot answer to %+v: %v", d, err)
		}
	}
	if len(e.G.Stack) > 0 {
		t.Fatalf("Omen stack did not drain within %d decisions", limit)
	}
}

// omenCorpusNames lists the main-face names of every corpus card whose script
// carries an AlternateMode:Omen line. It walks the fetched cardsfolder rather
// than reg.AllCards(): the shared corpus is an imaged registry and
// materializing all ~34k cards is exactly the scan-set regression the lazy
// registry exists to avoid.
func omenCorpusNames(t *testing.T) []string {
	t.Helper()
	root := filepath.Join(testutil.CorpusDirOrSkip(t), "cardsfolder")
	var names []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".txt") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		omen := false
		name := ""
		for _, line := range strings.Split(string(b), "\n") {
			switch {
			case line == "AlternateMode:Omen":
				omen = true
			case strings.HasPrefix(line, "Name:") && name == "":
				name = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
			}
		}
		if omen && name != "" {
			names = append(names, name)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}
	sort.Strings(names)
	return names
}

// omenPreExistingGapNotes are the exact gap Notes an Omen face may emit for a
// primitive unrelated to the Omen cast itself. Divining Dive's GenericChoice
// carries Secretly$ True, which api:GenericChoice has never read: the choice
// is private to its chooser at the decision layer, but the ModeChosen marker
// is public, so honouring Secretly$ is a view-redaction change outside this
// ticket. The census still fails on every OTHER gap Note, and the report names
// this one.
var omenPreExistingGapNotes = map[string]bool{
	"GenericChoice ignores unread parameter(s) Secretly$": true,
}

func isOmenPreExistingGapNote(text string) bool { return omenPreExistingGapNotes[text] }

// TestEveryOmenFaceCastsInCorpus is the corpus census: every card in the
// fetched corpus whose back face is an Omen spell offers that face from hand
// (with a funded pool and a legal board), casts it, resolves it, and shuffles
// the card into its owner's library as its main face -- without a single
// unread-parameter or unimplemented Note, except the pre-existing
// GenericChoice Secretly$ gap named above. A face that fails here names the
// primitive gap in its failure message.
func TestEveryOmenFaceCastsInCorpus(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	names := omenCorpusNames(t)
	if len(names) == 0 {
		t.Fatal("no Omen cards in the fetched corpus")
	}
	for _, name := range names {
		omen := searchCorpusCard(t, reg, name)
		t.Run(omen.Faces[0].Name, func(t *testing.T) {
			e, cfg, id, oppBear := omenCensusEngine(t, reg, omen)
			addMana(t, e, 0, "WUBRGCWUBRGCWUBRGCWUBRGC")
			face := omenSpellFace(e.G.Obj(id))
			if face == nil {
				t.Fatalf("%s: omenSpellFace returned nil for the hand card", omen.Faces[0].Name)
			}
			// Chilling Screech's only legal target shape is a spell with mana
			// value 2 or less: put one on the stack first.
			if face.Name == "Chilling Screech" {
				boltID := searchMoveByName(t, e, "Lightning Bolt", state.ZHand)
				submitChoices(t, e, castOptionFor(t, e, boltID).Index)
				d := e.Pending()
				if d == nil || d.Kind != decision.KTarget {
					t.Fatalf("Lightning Bolt target ask: %+v", d)
				}
				tgt := -1
				for _, o := range d.Options {
					if o.Obj == oppBear {
						tgt = o.Index
					}
				}
				if tgt < 0 {
					t.Fatalf("no option targeting the bear: %+v", d.Options)
				}
				submitChoices(t, e, tgt)
			}
			alt := omenOption(t, e, id, "omen_alt")
			if alt == nil {
				t.Fatalf("%s: Omen face %q not offered; options: %+v", omen.Faces[0].Name, face.Name, castOptions(t, e))
			}
			start := len(e.L.Events)
			submitChoices(t, e, alt.Index)
			omenDrain(t, e, 60)

			o := e.G.Obj(id)
			if o.Zone != state.ZLibrary {
				t.Fatalf("%s: resolved Omen zone=%s, want ZLibrary", face.Name, o.Zone)
			}
			if int(o.FaceIdx) != 0 {
				t.Fatalf("%s: resolved Omen faceIdx=%d, want 0", face.Name, o.FaceIdx)
			}
			for _, ev := range e.L.Events[start:] {
				if ev.Kind != events.Note || isOmenPreExistingGapNote(ev.Text) {
					continue
				}
				for _, bad := range []string{"ignores unread parameter", "not implemented", "unimplemented", "not modelled"} {
					if strings.Contains(ev.Text, bad) {
						t.Fatalf("%s: resolution emitted a gap Note: %q", face.Name, ev.Text)
					}
				}
			}
			replayCheck(t, e, cfg)
		})
	}
}
