package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
)

// TestChainLightningMayChooseTargetUsesTheRealCorpus pins CR 707.10c on the
// real Chain Lightning card. Its paid copy must offer the original player
// target first (the keep-current choice) and also offer the legal creature
// target, rather than recording the old copy-keeps-its-targets stand-in.
func TestChainLightningMayChooseTargetUsesTheRealCorpus(t *testing.T) {
	reg := searchTestRegistry(t)
	// The ordinary two-seat corpus harness supplies a real legal alternate
	// target on the battlefield while retaining Chain Lightning in hand.
	eng, cfg := miscHandsEngine(t, reg,
		[]string{"Chain Lightning"}, nil, nil, []string{"Grizzly Bears"})
	addMana(t, eng, 0, "R")
	addMana(t, eng, 1, "RR")
	boltObj := miscHandObj(t, eng, 0, "Chain Lightning")
	submitChoices(t, eng, miscCastOption(t, eng, boltObj))
	d := eng.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Chain Lightning target ask, got %+v", d)
	}
	player := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			player = o.Index
		}
	}
	if player < 0 {
		t.Fatal("seat 1 was not offered as Chain Lightning's target")
	}
	submitChoices(t, eng, player)
	d = passUntilNonPriority(t, eng, 30)
	if d == nil || d.Kind != decision.KModes || d.Player != 1 {
		t.Fatalf("expected Chain Lightning's unless-pay ask, got %+v", d)
	}
	submitChoices(t, eng, d.Options[0].Index)
	d = passUntilNonPriority(t, eng, 30)
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected MayChooseTarget$ ask for the copy, got %+v", d)
	}
	if d.ResumeKind != "copy_targets" || d.Source == 0 {
		t.Fatalf("copy ask transport = kind %q source %d", d.ResumeKind, d.Source)
	}
	if len(d.Options) < 2 || d.Options[0].Kind != "player" || d.Options[0].Player != 1 {
		t.Fatalf("copy target ask does not keep the current target first: %+v", d.Options)
	}
	bear := -1
	for _, o := range d.Options {
		if o.Kind == "permanent" || o.Kind == "card" {
			bear = o.Index
			break
		}
	}
	if bear < 0 {
		t.Fatalf("copy target ask did not offer the real battlefield Grizzly Bears: %+v", d.Options)
	}
	submitChoices(t, eng, bear)
	var copies []events.Event
	for _, ev := range eng.L.Events {
		if ev.Kind == events.StackCopy {
			copies = append(copies, ev)
		}
	}
	if len(copies) != 1 {
		t.Fatalf("StackCopy events = %+v, want one copy", copies)
	}
	foundBearTarget := false
	for _, o := range eng.G.Objs {
		if o.IsCopy && len(o.Targets) == 1 && !o.Targets[0].IsPlayer {
			foundBearTarget = true
		}
	}
	if !foundBearTarget {
		t.Fatal("copy target answer was not recorded on the copy")
	}
	_ = cfg
}
