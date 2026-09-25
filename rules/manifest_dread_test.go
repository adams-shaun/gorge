package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestZimoneManifestDreadLooksAndChooses exercises the real corpus trigger:
// Zimone's landfall ability looks at the top two and lets its controller pick.
func TestZimoneManifestDreadLooksAndChooses(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Zimone, Mystery Unraveler")
	zid := searchMoveByName(t, e, "Zimone, Mystery Unraveler", state.ZHand)
	land := searchMoveByName(t, e, "Forest", state.ZHand)
	if zid == 0 || land == 0 {
		t.Fatal("test precondition: Zimone and a land must be in seat 0's hand")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: zid, From: state.ZHand, To: state.ZBattlefield})
	if e.G.Obj(zid).Zone != state.ZBattlefield {
		t.Fatal("test precondition: Zimone did not enter the battlefield")
	}
	lib := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...)
	if len(lib) < 2 || lib[0] == lib[1] {
		t.Fatalf("test precondition: need two distinct top library objects, got %v", lib)
	}
	first, second := lib[0], lib[1]
	if e.G.Obj(first).Zone != state.ZLibrary || e.G.Obj(second).Zone != state.ZLibrary {
		t.Fatal("test precondition: both offered cards must be in the library")
	}
	// Play the land through the ordinary action so its real landfall trigger
	// is queued by the engine, rather than simulating a zone change.
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("test precondition: expected priority before playing the land, got %+v", d)
	}
	landOption := -1
	for _, option := range d.Options {
		if option.Kind == "play_land" && option.Obj == land {
			landOption = option.Index
		}
	}
	if landOption < 0 {
		t.Fatalf("test precondition: Forest %d is not offered as a land play", land)
	}
	submitChoices(t, e, landOption)
	for attempts := 0; attempts < 20; attempts++ {
		d = e.Pending()
		if d != nil && d.Kind == decision.KChoose && d.ResumeKind == "manifest_dread" {
			break
		}
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("unexpected state while resolving landfall: %+v", d)
		}
		pass := -1
		for _, option := range d.Options {
			if option.Kind == "pass" {
				pass = option.Index
				break
			}
		}
		if pass < 0 {
			t.Fatalf("no pass option while resolving landfall: %+v", d.Options)
		}
		submitChoices(t, e, pass)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "manifest_dread" || d.Player != 0 || len(d.Options) != 2 {
		t.Fatalf("ManifestDread choice = %+v, want controller KChoose over two cards", d)
	}
	selected := d.Options[1].Obj
	if selected != first && selected != second {
		t.Fatalf("chosen option %d was not one of the two preconditioned top cards", selected)
	}
	submitChoices(t, e, d.Options[1].Index)
	if got := e.G.Obj(selected).Zone; got != state.ZBattlefield {
		t.Fatalf("chosen card %d is in %s, want battlefield", selected, got)
	}
	other := first
	if selected == first {
		other = second
	}
	if got := e.G.Obj(other).Zone; got != state.ZGraveyard {
		t.Fatalf("unchosen card %d is in %s, want graveyard", other, got)
	}
	o := e.G.Obj(selected)
	if !o.FaceDown || o.Controller != 0 {
		t.Fatalf("manifested card FaceDown=%v Controller=%d, want true/0", o.FaceDown, o.Controller)
	}
	der := e.Derived(selected)
	if der.Power != 2 || der.Toughness != 2 || len(der.Types) != 1 || der.Types[0] != "Creature" {
		t.Fatalf("manifested card derived=%+v, want face-down 2/2 Creature", der)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented API ManifestDread") {
			t.Fatalf("unimplemented fallback recorded: %q", ev.Text)
		}
	}
	lookSeen := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && ev.Text == "looks at the top two cards of the library" && ev.Player == 0 {
			lookSeen = true
			if !ev.Secret || len(ev.IDs) != 2 || ev.IDs[0] != first || ev.IDs[1] != second {
				t.Fatalf("private look = %+v, want secret note for exactly the two top cards", ev)
			}
		}
	}
	if !lookSeen {
		t.Fatal("no replay-visible private look was recorded")
	}
	replayCheck(t, e, cfg)
}
