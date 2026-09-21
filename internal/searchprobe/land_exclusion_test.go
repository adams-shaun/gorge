package searchprobe

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// namedDraws counts the copies of name among seat's first `through` draws of
// a sampled world, read off the world's own log.
func namedDraws(w World, seat state.PlayerID, name string, through int) int {
	n, draws := 0, 0
	for _, ev := range w.Engine.L.Events {
		if ev.Kind != events.Draw || ev.Player != seat {
			continue
		}
		if draws >= through {
			break
		}
		draws++
		if o := w.Engine.G.Obj(ev.Obj); o != nil && o.Card != nil && o.Card.Faces[0].Name == name {
			n++
		}
	}
	return n
}

// The declined-land-drop exclusion may only remove worlds the replay rejects
// with probability one; otherwise it would truncate the accepted-world
// distribution. Measured directly: sample the bench fixture WITHOUT the
// exclusion and check that every accepted world already satisfies every
// constraint the exclusion would have taught. Then check the exclusion does
// what it is for: it teaches something and accepts strictly more worlds from
// the same attempt budget.
func TestLandExclusionRemovesOnlyRejectedWorlds(t *testing.T) {
	f := benchRoot(t)
	opts := benchSampleOptions()
	opts.Attempts, opts.Worlds, opts.MinESS = 96, 96, 1

	on, err := Sample(f.setup, f.h, opts)
	if err != nil {
		t.Fatal(err)
	}
	if on.LandExclusions == 0 {
		t.Fatalf("fixture taught no land exclusion: %+v", on)
	}
	opts.NoLandExclusion = true
	off, err := Sample(f.setup, f.h, opts)
	if err != nil {
		t.Fatal(err)
	}
	if off.LandExclusions != 0 || off.Accepted == 0 {
		t.Fatalf("control run: %+v", off)
	}
	if on.Accepted <= off.Accepted {
		t.Fatalf("exclusion accepted %d worlds, control %d", on.Accepted, off.Accepted)
	}

	// The constraint the fixture teaches: the opponent passed main-phase
	// priority with a land drop open at some frame, so among its draws up to
	// that frame there are no more Islands than it was observed playing.
	epochs, err := compileEpochs(f.h)
	if err != nil {
		t.Fatal(err)
	}
	drawStates := frameDrawStates(f.h)
	checked := 0
	for _, bucket := range off.Rejections {
		if bucket.Component != "identities" || bucket.Shape != "hand_to_battlefield" {
			continue
		}
		ds := drawStates[bucket.Frame][1]
		cap := epochDeadlineCountFor(epochs[epochKey{Player: 1, Ordinal: ds.ordinal - 1}], "Island")
		for _, w := range off.Worlds {
			checked++
			if got := namedDraws(w, 1, "Island", ds.draws); got > cap {
				t.Fatalf("an ACCEPTED control world drew %d Islands in its first %d draws (cap %d): the exclusion would have removed a live world", got, ds.draws, cap)
			}
		}
	}
	if checked == 0 {
		t.Fatal("control run never met the declined-land-drop rejection")
	}
}
