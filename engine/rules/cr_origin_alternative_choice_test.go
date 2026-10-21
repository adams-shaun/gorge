package rules

// OriginAlternative$ zone election (cr-g51). Tower Winder: "search your
// library and/or graveyard for a card named Command Tower ... If you search
// your library this way, shuffle." "and/or" is the searcher's choice of
// zones: one, the other or both. A search that skips the library offers no
// library card and owes no shuffle (CR 701.24a only shuffles a library that
// was searched); a library-only search offers no graveyard card.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// originAltWinderFixture puts one Command Tower in seat 0's graveyard and
// one in its library, resolves Tower Winder's ETB, and returns the first
// non-priority decision with both tower ids.
func originAltWinderFixture(t *testing.T) (*Engine, *decision.Decision, state.ObjID, state.ObjID, int) {
	t.Helper()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Tower Winder", "Command Tower", "Command Tower")
	var towers []state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 0) {
			if o := e.G.Obj(id); o.Face().Name == "Command Tower" {
				towers = append(towers, id)
			}
		}
	}
	if len(towers) != 2 {
		t.Fatalf("fixture: %d Command Towers, want 2", len(towers))
	}
	gy, lib := towers[0], towers[1]
	e.emit(events.Event{Kind: events.MoveZone, Obj: gy, From: e.G.Obj(gy).Zone, To: state.ZGraveyard})
	if z := e.G.Obj(lib).Zone; z != state.ZLibrary {
		e.emit(events.Event{Kind: events.MoveZone, Obj: lib, From: z, To: state.ZLibrary})
	}
	wind := searchMoveByName(t, e, "Tower Winder", state.ZHand)
	logBefore := len(e.L.Events)
	e.emit(events.Event{Kind: events.MoveZone, Obj: wind, From: state.ZHand, To: state.ZBattlefield})
	e.putTriggersOnStack()
	e.pending = nil
	e.resolveTop()
	return e, e.Pending(), gy, lib, logBefore
}

func originAltZoneIndex(t *testing.T, d *decision.Decision, label string) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind == "zone" && o.Label == label {
			return o.Index
		}
	}
	t.Fatalf("zone election %+v has no %q option", d.Options, label)
	return -1
}

func originAltShuffled(e *Engine, from int) bool {
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.Shuffle && ev.Player == 0 {
			return true
		}
	}
	return false
}

// TestCROriginAlternativeGraveyardOnlySkipsLibrary: the searcher elects the
// graveyard alone; the search offers only the graveyard tower and the
// library is not shuffled.
func TestCROriginAlternativeGraveyardOnlySkipsLibrary(t *testing.T) {
	t.Parallel()
	e, d, gy, lib, logBefore := originAltWinderFixture(t)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "search_zones" {
		t.Fatalf("\"library and/or graveyard\": pending ask %q (%q), want the zone election (ResumeKind search_zones) before the search", originAltPendingKind(d), originAltPendingPrompt(d))
	}
	if d.Player != 0 || d.Min != 1 || d.Max != 2 || len(d.Options) != 2 {
		t.Fatalf("zone election = player %d min %d max %d options %+v, want seat 0, 1..2 of [library graveyard]", d.Player, d.Min, d.Max, d.Options)
	}
	submitChoices(t, e, originAltZoneIndex(t, d, "Search the graveyard"))
	s := e.Pending()
	if s == nil || s.ResumeKind != "search" {
		t.Fatalf("pending = %+v, want the search pick", s)
	}
	if kr4Offers(s, lib) || !kr4Offers(s, gy) {
		t.Fatalf("graveyard-only search options = %+v, want the graveyard tower %d and not the library tower %d", s.Options, gy, lib)
	}
	submitChoices(t, e, s.Options[0].Index)
	kr4Settle(e)
	passUntilStackEmpty(t, e, 30)
	if o := e.G.Obj(gy); o.Zone != state.ZHand {
		t.Fatalf("graveyard tower zone = %s, want hand", o.Zone)
	}
	if originAltShuffled(e, logBefore) {
		t.Fatal("\"If you search your library this way, shuffle\": the library was shuffled although it was not searched")
	}
	for _, ev := range e.L.Events[logBefore:] {
		if ev.Kind == events.SearchedLibrary {
			t.Fatal("a graveyard-only search recorded SearchedLibrary (a library-search trigger would fire)")
		}
	}
}

// TestCROriginAlternativeLibraryOnlyOmitsGraveyard: the library alone is
// searched; the graveyard tower is not offered and the library shuffles.
func TestCROriginAlternativeLibraryOnlyOmitsGraveyard(t *testing.T) {
	t.Parallel()
	e, d, gy, lib, logBefore := originAltWinderFixture(t)
	if d == nil || d.ResumeKind != "search_zones" {
		t.Fatalf("pending ask %q (%q), want the zone election", originAltPendingKind(d), originAltPendingPrompt(d))
	}
	submitChoices(t, e, originAltZoneIndex(t, d, "Search the library"))
	s := e.Pending()
	if s == nil || s.ResumeKind != "search" {
		t.Fatalf("pending = %+v, want the search pick", s)
	}
	if kr4Offers(s, gy) || !kr4Offers(s, lib) {
		t.Fatalf("library-only search options = %+v, want the library tower %d and not the graveyard tower %d", s.Options, lib, gy)
	}
	submitChoices(t, e, s.Options[0].Index)
	kr4Settle(e)
	passUntilStackEmpty(t, e, 30)
	if !originAltShuffled(e, logBefore) {
		t.Fatal("a library search must shuffle")
	}
}

// answerOriginAltAllZones answers a pending OriginAlternative$ zone election
// with every offered zone (the pre-election behaviour) and returns the next
// pending decision; any other pending decision is returned unchanged.
func answerOriginAltAllZones(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	d := e.Pending()
	if d == nil || d.ResumeKind != "search_zones" {
		return d
	}
	all := make([]int, len(d.Options))
	for i, o := range d.Options {
		all[i] = o.Index
	}
	submitChoices(t, e, all...)
	return e.Pending()
}

func originAltPendingKind(d *decision.Decision) string {
	if d == nil {
		return "<none>"
	}
	return d.ResumeKind
}

func originAltPendingPrompt(d *decision.Decision) string {
	if d == nil {
		return ""
	}
	return d.Prompt
}
