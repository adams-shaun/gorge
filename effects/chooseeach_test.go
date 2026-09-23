package effects

// ChooseEach$ on ChooseCard (param:api:ChooseCard.ChooseEach): instead of one
// pick per chooser, the chooser picks ONE card PER GROUP of the " & "
// separated ChooseEach$ value, over the same Choices$ pool. The real corpus
// carrier is Tragic Arrogance's YouChoose body, driven here by its own
// RepeatEach walk (RepeatPlayers$ Player): each iteration's ChooseCard asks
// the spell's controller once per group among the remembered player's
// permanents, each answered pick re-enters through the ordinary "choice"
// resume arm (the flat (chooser, group) ResumeTarget index), the picks
// accumulate into Chosen/Remembered across the groups (RememberChosen$), and
// a group whose narrowed pool is empty resolves silently (no ask, nothing
// chosen) — Forge's per-type loop with no candidate of that type.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// countingAskHost is askHost that keeps EVERY posed decision in ask order —
// a per-group walk poses several in sequence and the last one alone is not
// enough to pin the walk's shape.
type countingAskHost struct {
	askHost
	asks []*decision.Decision
}

func (h *countingAskHost) Ask(d *decision.Decision) bool {
	cp := *d
	cp.Options = append([]decision.Option(nil), d.Options...)
	h.asks = append(h.asks, &cp)
	return true
}

func tragicArroganceBoard(t *testing.T) (*countingAskHost, *Ctx, *state.Object, *state.Object, *state.Object, *state.Object) {
	t.Helper()
	card, chooseSA := corpusSA(t, "Tragic Arrogance", "YouChoose")
	if chooseSA.API != "ChooseCard" || chooseSA.Params["ChooseEach"] != "Artifact & Creature & Enchantment & Planeswalker" {
		t.Fatalf("Tragic Arrogance's YouChoose SA changed: %+v", chooseSA)
	}
	h := &countingAskHost{}
	h.g = state.NewGame(names(2))
	src := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZLibrary, To: state.ZBattlefield})
	// Seat 1 controls the pool ControlledByPlayer$ Remembered narrows to: one
	// card per group plus a SECOND creature, so the creature group's answer
	// can be a real election (not a forced sole option), and no planeswalker,
	// so that group's ask must resolve silently.
	artifact := h.g.AddObject(mkCard(t, "Name:TA Art\nTypes:Artifact\nOracle:x\n"), 1)
	creFirst := h.g.AddObject(mkCard(t, "Name:TA Cre One\nTypes:Creature\nPT:1/1\nOracle:x\n"), 1)
	creSecond := h.g.AddObject(mkCard(t, "Name:TA Cre Two\nTypes:Creature\nPT:1/1\nOracle:x\n"), 1)
	ench := h.g.AddObject(mkCard(t, "Name:TA Ench\nTypes:Enchantment\nOracle:x\n"), 1)
	for _, o := range []*state.Object{artifact, creFirst, creSecond, ench} {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	}
	c := &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Player: 1, IsPlayer: true}}}
	return h, c, artifact, creFirst, creSecond, ench
}

func TestChooseCardChooseEachTragicArroganceAsksPerGroup(t *testing.T) {
	h, c, artifact, creFirst, creSecond, ench := tragicArroganceBoard(t)
	_, chooseSA := corpusSA(t, "Tragic Arrogance", "YouChoose")

	// Fresh entry: one ask for the FIRST group (Artifact) over the remembered
	// player's artifacts only — not the whole Permanent pool.
	Resolve(h, c, chooseSA)
	if len(h.asks) != 1 {
		t.Fatalf("asks so far = %d, want only the Artifact group's", len(h.asks))
	}
	d := h.asks[0]
	if d.Player != 0 || d.Min != 1 || d.Max != 1 || d.ResumeKind != "choice" || d.ResumeTarget != 0 {
		t.Fatalf("Artifact ask = %+v, want a Mandatory single pick for seat 0 at flat index 0", d)
	}
	if len(d.Options) != 1 || d.Options[0].Obj != artifact.ID {
		t.Fatalf("Artifact group options = %+v, want exactly %d", d.Options, artifact.ID)
	}

	// Answered re-entry through the ordinary resume transport: the pick rides
	// Ctx.Choice/ChoiceDone/ChoiceTarget, exactly what rules' "choice" arm
	// sets, and the next group's ask follows at flat index 1.
	c.Choice = []state.Target{{Obj: artifact.ID}}
	c.ChoiceDone = true
	c.ChoiceTarget = d.ResumeTarget
	Resolve(h, c, chooseSA)
	if len(h.asks) != 2 {
		t.Fatalf("asks after the Artifact answer = %d, want the Creature group's too", len(h.asks))
	}
	d = h.asks[1]
	if d.ResumeTarget != 1 {
		t.Fatalf("Creature ask ResumeTarget = %d, want 1", d.ResumeTarget)
	}
	if len(d.Options) != 2 || d.Options[0].Obj != creFirst.ID || d.Options[1].Obj != creSecond.ID {
		t.Fatalf("Creature group options = %+v, want both creatures in registration order", d.Options)
	}

	// Pick the SECOND creature — the election must bind, not collapse to the
	// deterministic first option.
	c.Choice = []state.Target{{Obj: creSecond.ID}}
	c.ChoiceDone = true
	c.ChoiceTarget = d.ResumeTarget
	Resolve(h, c, chooseSA)
	if len(h.asks) != 3 {
		t.Fatalf("asks after the Creature answer = %d, want the Enchantment group's too", len(h.asks))
	}
	d = h.asks[2]
	if len(d.Options) != 1 || d.Options[0].Obj != ench.ID || d.ResumeTarget != 2 {
		t.Fatalf("Enchantment ask = %+v, want the enchantment alone at flat index 2", d)
	}
	c.Choice = []state.Target{{Obj: ench.ID}}
	c.ChoiceDone = true
	c.ChoiceTarget = d.ResumeTarget
	Resolve(h, c, chooseSA)

	// The Planeswalker group has an empty pool: no fourth ask was EVER posed
	// (askHost captured every ask), and the walk completed with the three
	// picks accumulated in Chosen and — RememberChosen$ True — Remembered.
	if len(h.asks) != 3 {
		t.Fatalf("total asks = %d, want 3 (the empty Planeswalker group must resolve silently)", len(h.asks))
	}
	want := []state.Target{{Obj: artifact.ID}, {Obj: creSecond.ID}, {Obj: ench.ID}}
	if len(c.Chosen) != len(want) || c.Chosen[1].Obj != creSecond.ID || c.Chosen[2].Obj != ench.ID {
		t.Fatalf("Chosen = %+v, want the three group picks with the CREATURE ELECTION honored (%d)", c.Chosen, creSecond.ID)
	}
	wantPicks := []state.ObjID{artifact.ID, creSecond.ID, ench.ID}
	if len(c.Remembered) != len(wantPicks)+1 || c.Remembered[0].Player != 1 {
		t.Fatalf("Remembered = %+v, want the iteration's remembered player plus RememberChosen$'s picks", c.Remembered)
	}
	for i, id := range wantPicks {
		if c.Remembered[i+1].Obj != id {
			t.Fatalf("Remembered[%d] = %d, want RememberChosen$ pick %d in group order", i+1, c.Remembered[i+1].Obj, id)
		}
	}
	for _, ev := range h.log {
		// Reveal$ True emits one public ids-Note per answered group (the
		// reveal, not a degradation); anything naming an unimplemented API or
		// an R-9 no-host stand-in must be absent.
		if ev.Kind == events.Note && (strings.Contains(ev.Text, "unimplemented") || strings.Contains(ev.Text, "no engine host")) {
			t.Fatalf("unexpected degradation Note %+v", ev)
		}
	}
	revealNotes := 0
	for _, ev := range h.log {
		if ev.Kind == events.Note && len(ev.IDs) == 1 && ev.IDs[0] != 0 {
			revealNotes++
		}
	}
	if revealNotes != 3 {
		t.Fatalf("reveal Notes = %d, want one per answered group", revealNotes)
	}
}

// TestChooseCardChooseEachNoHostTakesFirstPerGroup pins the no-host stand-in:
// with a host that cannot ask, every Mandatory group takes its first
// eligible candidate in group order (the same R-9 deterministic path the
// single-pool shape runs, applied per group).
func TestChooseCardChooseEachNoHostTakesFirstPerGroup(t *testing.T) {
	card, chooseSA := corpusSA(t, "Tragic Arrogance", "YouChoose")
	h := newHost(t, 2)
	src := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZLibrary, To: state.ZBattlefield})
	artifact := h.g.AddObject(mkCard(t, "Name:TA Art\nTypes:Artifact\nOracle:x\n"), 1)
	creFirst := h.g.AddObject(mkCard(t, "Name:TA Cre One\nTypes:Creature\nPT:1/1\nOracle:x\n"), 1)
	ench := h.g.AddObject(mkCard(t, "Name:TA Ench\nTypes:Enchantment\nOracle:x\n"), 1)
	for _, o := range []*state.Object{artifact, creFirst, ench} {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	}
	c := &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Player: 1, IsPlayer: true}}}
	Resolve(h, c, chooseSA)
	want := []state.Target{{Obj: artifact.ID}, {Obj: creFirst.ID}, {Obj: ench.ID}}
	if len(c.Chosen) != len(want) || c.Chosen[0].Obj != artifact.ID || c.Chosen[1].Obj != creFirst.ID || c.Chosen[2].Obj != ench.ID {
		t.Fatalf("no-host Chosen = %+v, want each group's first eligible (%+v)", c.Chosen, want)
	}
}

// TestChooseCardChooseEachPartyExpands sticks the "Party" spelling to its
// Forge reading (Stick Together's reminder: "choose up to one each of Cleric,
// Rogue, Warrior, and Wizard"): four party-member groups, the non-party
// creature offered to none of them, and the no-candidate groups silent.
func TestChooseCardChooseEachPartyExpands(t *testing.T) {
	chooseSA := sa(t, "SP$ ChooseCard | Defined$ You | Choices$ Creature | ChooseEach$ Party | Mandatory$ True | Reveal$ True")
	h := &countingAskHost{}
	h.g = state.NewGame(names(2))
	cleric := h.g.AddObject(mkCard(t, "Name:Party Cleric\nTypes:Creature Cleric\nPT:1/1\nOracle:x\n"), 0)
	warrior := h.g.AddObject(mkCard(t, "Name:Party Warrior\nTypes:Creature Warrior\nPT:1/1\nOracle:x\n"), 0)
	goblin := h.g.AddObject(mkCard(t, "Name:Not Party\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n"), 0)
	for _, o := range []*state.Object{cleric, warrior, goblin} {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	}
	// Fresh entry asks the Cleric group only: each answered pick re-enters
	// through the resume transport before the next group's ask is posed, so
	// the walk is driven one re-entry per ask exactly as rules does it.
	c := &Ctx{Controller: 0}
	Resolve(h, c, chooseSA)
	last := -1
	for n := 0; ; n++ {
		if len(h.asks) != n+1 {
			t.Fatalf("ask %d missing after %d re-entries (walk = %+v)", n, n, h.asks)
		}
		d := h.asks[n]
		// The flat pair index advances past the empty groups silently: Cleric
		// is flat 0, the empty Rogue group is flat 1 and never asks, Warrior
		// is flat 2.
		if d.ResumeTarget <= last {
			t.Fatalf("party ask %d ResumeTarget = %d, want it advancing past %d", n, d.ResumeTarget, last)
		}
		last = d.ResumeTarget
		c.Choice = []state.Target{{Obj: d.Options[0].Obj}}
		c.ChoiceDone = true
		c.ChoiceTarget = d.ResumeTarget
		Resolve(h, c, chooseSA)
		if len(h.asks) == n+1 {
			break // the walk completed: the remaining groups had no candidate
		}
		if n > 4 {
			t.Fatal("the party walk posed more asks than it has groups")
		}
	}
	if len(h.asks) != 2 || h.asks[1].ResumeTarget != 2 {
		t.Fatalf("walk = %d asks, last ResumeTarget %d, want Cleric at 0 and Warrior at 2 (Rogue skipped silently)", len(h.asks), h.asks[len(h.asks)-1].ResumeTarget)
	}
	if h.asks[0].Options[0].Obj != cleric.ID || h.asks[1].Options[0].Obj != warrior.ID {
		t.Fatalf("party group options = [%+v %+v], want the cleric then the warrior", h.asks[0].Options, h.asks[1].Options)
	}
	if len(c.Chosen) != 2 || c.Chosen[0].Obj != cleric.ID || c.Chosen[1].Obj != warrior.ID {
		t.Fatalf("Chosen = %+v, want the cleric and the warrior", c.Chosen)
	}
}
