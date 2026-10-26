package effects

// lb-copyperm-sel: CopyPermanent's Choices$ pool (every bare Choices$ filter,
// not only the Zndrsplt / SaddledThisTurn shapes) and the Defined$
// CounterPutCards paid-list selector (Grub, Notorious Auntie's "a copy of the
// blighted creature"), driven through the real corpus bodies.

import (
	"slices"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// copyTokensNamed returns the copy tokens on the battlefield whose face is
// name.
func copyTokensNamed(h *fakeHost, name string) []*state.Object {
	var out []*state.Object
	for i := range h.g.Objs {
		o := &h.g.Objs[i]
		if o.IsToken && o.Zone == state.ZBattlefield && o.Face() != nil && o.Face().Name == name {
			out = append(out, o)
		}
	}
	return out
}

// TestTwilightDivinerCopiesOneOfTheTriggeredCreatures drives Twilight
// Diviner's real TrigCopy (`CopyPermanent Choices$ Card.TriggeredCards`) with
// the entering batch bound on Ctx.Remembered. The pool is exactly the
// triggering creature -- an unrelated creature that sits EARLIER on the
// battlefield is not offered -- and the copy is of it.
func TestTwilightDivinerCopiesOneOfTheTriggeredCreatures(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	twilight, ok := reg.Lookup("Twilight Diviner")
	if !ok || twilight == nil {
		t.Fatal("missing corpus card Twilight Diviner")
	}
	body := cards.ResolveSVar(twilight.Faces[0].SVars, "TrigCopy")
	if body == nil || body.API != "CopyPermanent" || body.Params["Choices"] != "Card.TriggeredCards" {
		t.Fatalf("precondition: Twilight TrigCopy shape %+v", body)
	}
	cp := CopyPermanentOf(body)
	if cp.Blocked || !cp.SupportsChoice || cp.ChoiceFilter != "Card.TriggeredCards" {
		t.Fatalf("Twilight CopyPermanent Choices$ not supported: Blocked=%v SupportsChoice=%v filter=%q note=%q",
			cp.Blocked, cp.SupportsChoice, cp.ChoiceFilter, cp.SkippedNote)
	}

	h := &fakeHost{g: state.NewGame(names(2))}
	src := h.g.AddObject(twilight, 0).ID
	other := h.g.AddObject(mkCard(t, "Name:Fixture Other\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0).ID
	entered := h.g.AddObject(mkCard(t, "Name:Fixture Entered\nTypes:Creature Beast\nPT:4/4\nOracle:x\n"), 0).ID
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{src, other, entered})
	for _, id := range []state.ObjID{src, other, entered} {
		h.g.Obj(id).Zone = state.ZBattlefield
	}
	// Preconditions: both candidates are battlefield creatures, and the
	// unrelated one comes first, so a battlefield-first stand-in would pick it.
	for _, id := range []state.ObjID{other, entered} {
		if o := h.g.Obj(id); o == nil || o.Zone != state.ZBattlefield || !o.Face().IsCreature() {
			t.Fatalf("precondition: candidate %d = %+v, want a battlefield creature", id, o)
		}
	}
	if bf := h.g.Zone(state.ZBattlefield, 0); slices.Index(bf, other) > slices.Index(bf, entered) {
		t.Fatalf("precondition: battlefield order %v must put the unrelated creature first", bf)
	}

	Resolve(h, &Ctx{Source: src, Controller: 0, Remembered: []state.Target{{Obj: entered}},
		SVars: twilight.Faces[0].SVars}, body)

	if h.lastAsk == nil || h.lastAsk.Kind != decision.KChoose {
		t.Fatalf("lastAsk = %+v, want the KChoose over the triggered creatures; notes %v", h.lastAsk, ufNotes(h))
	}
	if len(h.lastAsk.Options) != 1 || h.lastAsk.Options[0].Obj != entered {
		t.Fatalf("Choices$ pool %+v, want exactly the triggering creature %d", h.lastAsk.Options, entered)
	}
	if got := copyTokensNamed(h, "Fixture Entered"); len(got) != 1 || !got[0].IsCopy {
		t.Fatalf("copies of the triggering creature = %d, want 1; notes %v", len(got), ufNotes(h))
	}
	if got := copyTokensNamed(h, "Fixture Other"); len(got) != 0 {
		t.Fatalf("copied the unrelated creature %d times", len(got))
	}
}

// TestCopyPermanentChoicesNarrowsDefined pins the Choices$ + Defined$ arm
// (Mimic Vat's `Defined$ ... | Choices$ Card`): the pool is the Defined$
// objects the Choices$ filter admits, never the battlefield sweep.
func TestCopyPermanentChoicesNarrowsDefined(t *testing.T) {
	vat := mkCard(t, "Name:Fixture Vat\nTypes:Artifact\nSVar:DBCopy:DB$ CopyPermanent | Defined$ Remembered | Choices$ Creature.Beast\nOracle:x\n")
	sa := cards.ResolveSVar(vat.Faces[0].SVars, "DBCopy")
	if sa == nil || sa.API != "CopyPermanent" {
		t.Fatalf("precondition: fixture body %+v", sa)
	}
	cp := CopyPermanentOf(sa)
	if cp.Blocked || !cp.SupportsChoice {
		t.Fatalf("Choices$+Defined$ not supported: %+v", cp)
	}
	h := &fakeHost{g: state.NewGame(names(2))}
	src := h.g.AddObject(vat, 0).ID
	field := h.g.AddObject(mkCard(t, "Name:Fixture Field Beast\nTypes:Creature Beast\nPT:2/2\nOracle:x\n"), 0).ID
	bear := h.g.AddObject(mkCard(t, "Name:Fixture Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0).ID
	beast := h.g.AddObject(mkCard(t, "Name:Fixture Beast\nTypes:Creature Beast\nPT:3/3\nOracle:x\n"), 0).ID
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{src, field, bear, beast})
	for _, id := range []state.ObjID{src, field, bear, beast} {
		h.g.Obj(id).Zone = state.ZBattlefield
	}
	Resolve(h, &Ctx{Source: src, Controller: 0, Remembered: []state.Target{{Obj: bear}, {Obj: beast}}}, sa)
	if h.lastAsk == nil || len(h.lastAsk.Options) != 1 || h.lastAsk.Options[0].Obj != beast {
		t.Fatalf("pool %+v, want exactly the remembered Beast %d (not the remembered Bear, not the battlefield Beast); notes %v",
			h.lastAsk, beast, ufNotes(h))
	}
	if got := copyTokensNamed(h, "Fixture Beast"); len(got) != 1 {
		t.Fatalf("copies of the remembered Beast = %d, want 1; notes %v", len(got), ufNotes(h))
	}
}

// TestGrubCopiesTheBlightedCreature drives Grub, Notorious Auntie's real
// TrigCopy (`Defined$ CounterPutCards`) with the Blight cost's paid list
// bound: the copy is of the blighted creature, not of Grub.
func TestGrubCopiesTheBlightedCreature(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	grub, ok := reg.Lookup("Grub, Storied Matriarch")
	if !ok || grub == nil || len(grub.Faces) < 2 {
		t.Fatal("missing corpus card Grub, Storied Matriarch // Grub, Notorious Auntie")
	}
	back := grub.Faces[1]
	body := cards.ResolveSVar(back.SVars, "TrigCopy")
	if body == nil || body.API != "CopyPermanent" || body.Params["Defined"] != "CounterPutCards" {
		t.Fatalf("precondition: Grub TrigCopy shape %+v", body)
	}
	h := &fakeHost{g: state.NewGame(names(2))}
	src := h.g.AddObject(grub, 0).ID
	blighted := h.g.AddObject(mkCard(t, "Name:Fixture Blighted\nTypes:Creature Elf\nPT:3/3\nOracle:x\n"), 0).ID
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{src, blighted})
	for _, id := range []state.ObjID{src, blighted} {
		h.g.Obj(id).Zone = state.ZBattlefield
	}
	c := &Ctx{Source: src, Controller: 0, SVars: back.SVars}
	c.Paid.CounterPut = []state.ObjID{blighted}
	Resolve(h, c, body)
	got := copyTokensNamed(h, "Fixture Blighted")
	if len(got) != 1 {
		t.Fatalf("copies of the blighted creature = %d, want 1; notes %v", len(got), ufNotes(h))
	}
	if !got[0].Tapped {
		t.Fatal("TokenTapped$ True copy entered untapped")
	}
	for _, n := range ufNotes(h) {
		if n == "CopyPermanent source CounterPutCards is not resolvable; no copy" {
			t.Fatalf("CounterPutCards still unresolvable: %q", n)
		}
	}
	// An unpaid list is the known-empty pool: no copy, and never Grub itself.
	h2 := &fakeHost{g: state.NewGame(names(2))}
	src2 := h2.g.AddObject(grub, 0).ID
	h2.g.SetZone(state.ZBattlefield, 0, []state.ObjID{src2})
	h2.g.Obj(src2).Zone = state.ZBattlefield
	Resolve(h2, &Ctx{Source: src2, Controller: 0, SVars: back.SVars}, body)
	for i := range h2.g.Objs {
		if h2.g.Objs[i].IsToken {
			t.Fatalf("an unpaid CounterPutCards list minted %+v", h2.g.Objs[i].Face())
		}
	}
}

// TestCopyPermanentChoicesCensus is the corpus-wide ratchet over every
// CopyPermanent ability carrying Choices$: the ones still blocked are exactly
// the shapes this primitive does not model (a Pawprint$ mode, a non-Zndrsplt
// Chooser$). The list may only shrink.
func TestCopyPermanentChoicesCensus(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	total := 0
	var blocked []string
	for _, card := range reg.AllCards() {
		for _, face := range card.Faces {
			check := func(label string, sa *cards.SA) {
				if sa == nil || sa.API != "CopyPermanent" {
					return
				}
				if _, ok := sa.Param(cards.PKChoices); !ok {
					return
				}
				total++
				if cp := CopyPermanentOf(sa); cp.Blocked || !cp.SupportsChoice {
					blocked = append(blocked, face.Name+"/"+label)
				}
			}
			for _, a := range face.Abilities {
				check("ability", a)
			}
			keys := make([]string, 0, len(face.SVars))
			for k := range face.SVars {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				check(k, cards.ResolveSVar(face.SVars, k))
			}
		}
	}
	sort.Strings(blocked)
	want := []string{
		"Season of Weaving/DBCopy", // Pawprint$
		"The Bean/DBCopyEach",      // Chooser$ Remembered over ControlledBy Remembered
	}
	if total < 20 {
		t.Fatalf("census saw %d Choices$ CopyPermanent abilities, want >= 20 (corpus missing?)", total)
	}
	if !slices.Equal(blocked, want) {
		t.Fatalf("blocked Choices$ CopyPermanent abilities (of %d) = %q, want %q", total, blocked, want)
	}
}
