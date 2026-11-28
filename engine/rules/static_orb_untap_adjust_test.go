// Static Orb's UntapAdjust player keyword (Forge's
// `UntapAdjust:Permanent:2`) caps a player's untap-step action: at most two
// permanents untap, and the player chooses which of the tapped ones stay
// tapped. The fixture is inline (the corpus script is GPL) and mirrors the
// printed static's Affected$/IsPresent$ shape, so the reader, the gate and
// the ask are all load-bearing.
package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const staticOrbSrc = "Name:Static Orb\nManaCost:3\nTypes:Artifact\n" +
	"S:Mode$ Continuous | IsPresent$ Card.Self+untapped | Affected$ Player | AddKeyword$ UntapAdjust:Permanent:2 | Description$ Untap cap fixture.\n" +
	"Oracle:Fixture: while untapped, each untap step untaps at most two permanents.\n"

const untapProbeSrc = "Name:Untap Probe\nManaCost:0\nTypes:Artifact\nOracle:x\n"

// TestStaticOrbUntapAdjust proves the cap is a real untap-step constraint:
// four tapped permanents, one question, exactly two untapped and the chosen
// two still tapped, with the answer folded through Choose events. A tapped
// Orb's IsPresent$ gate must make the whole static inert.
func TestStaticOrbUntapAdjust(t *testing.T) {
	t.Parallel()
	t.Run("untapped Orb caps", func(t *testing.T) {
		e := layerEngine(t)
		orb := onBoard(t, e, 0, staticOrbSrc)
		if o := e.G.Obj(orb); o == nil || o.Tapped {
			t.Fatal("precondition: Static Orb must be an untapped battlefield permanent")
		}
		var probes []state.ObjID
		for range 4 {
			id := onBoard(t, e, 0, untapProbeSrc)
			e.emit(events.Event{Kind: events.Tap, Obj: id})
			probes = append(probes, id)
		}
		if !slices.Contains(e.playerKeywords(0), "UntapAdjust:Permanent:2") {
			t.Fatal("precondition: seat 0 does not carry the UntapAdjust grant")
		}
		e.G.Step, e.G.Active, e.G.Priority = state.StepUntap, 0, 0
		if e.finishUntapStep(0) {
			t.Fatal("untap scan completed without posing Static Orb's cap election")
		}
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || d.Player != 0 {
			t.Fatalf("pending = %+v, want seat 0's KChoose cap election", d)
		}
		if d.Min != 2 || d.Max != 2 {
			t.Fatalf("cap election Min/Max = %d/%d, want 2/2 (four tapped, keep two)", d.Min, d.Max)
		}
		if len(d.Options) != 4 {
			t.Fatalf("cap election has %d options, want the four tapped permanents", len(d.Options))
		}
		for _, o := range d.Options {
			if o.Kind != "keep_tapped" || !slices.Contains(probes, o.Obj) {
				t.Fatalf("cap option = %+v, want a keep_tapped probe", o)
			}
		}
		keepA, keepB := d.Options[0], d.Options[3]
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{keepA.Index, keepB.Index}}); err != nil {
			t.Fatal(err)
		}
		untapped := 0
		for _, id := range probes {
			o := e.G.Obj(id)
			if o == nil {
				t.Fatalf("probe %d vanished", id)
			}
			if !o.Tapped {
				untapped++
			}
		}
		if untapped != 2 {
			t.Fatalf("%d probes untapped, want exactly 2 (Static Orb's cap)", untapped)
		}
		for _, id := range []state.ObjID{keepA.Obj, keepB.Obj} {
			if o := e.G.Obj(id); o == nil || !o.Tapped {
				t.Fatalf("probe %d untapped despite being chosen to stay tapped", id)
			}
		}
		kept := 0
		for _, ev := range e.L.Events {
			if ev.Kind == events.Choose && ev.Counter == "untap" && ev.Text == "keep" {
				kept++
			}
		}
		if kept != 2 {
			t.Fatalf("logged %d keep-tapped choices, want 2", kept)
		}
	})
	t.Run("tapped Orb is inert", func(t *testing.T) {
		e := layerEngine(t)
		orb := onBoard(t, e, 0, staticOrbSrc)
		e.emit(events.Event{Kind: events.Tap, Obj: orb})
		var probes []state.ObjID
		for range 4 {
			id := onBoard(t, e, 0, untapProbeSrc)
			e.emit(events.Event{Kind: events.Tap, Obj: id})
			probes = append(probes, id)
		}
		e.G.Step, e.G.Active, e.G.Priority = state.StepUntap, 0, 0
		if !e.finishUntapStep(0) {
			t.Fatalf("tapped Static Orb still posed an election: %+v", e.Pending())
		}
		for _, id := range probes {
			if o := e.G.Obj(id); o == nil || o.Tapped {
				t.Fatalf("probe %d stayed tapped under a tapped Static Orb", id)
			}
		}
	})
}
