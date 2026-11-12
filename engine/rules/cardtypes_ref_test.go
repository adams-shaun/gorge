package rules

import (
	"regexp"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRefCardTypesReadsEveryObjectRef: <Ref>$CardTypes is
// the distinct-card-type count over the objects the ref names, for EVERY
// object ref, not only TriggerObjectsCards/TriggerRemembered. Kefka, Court
// Mage (Remembered$CardTypes: "draw a card for each card type among cards
// discarded this way"), Occult Epiphany, Mount Velus Manticore, Baba Lysaga
// (Sacrificed$CardTypes) and Keen-Eyed Curator all read 0 before.
func TestRefCardTypesReadsEveryObjectRef(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	artCreature := inZoneCard(t, e, 0, state.ZGraveyard, "Name:AC\nTypes:Artifact Creature Golem\nPT:1/1\nOracle:x\n")
	land := inZoneCard(t, e, 0, state.ZGraveyard, "Name:L\nTypes:Land\nOracle:x\n")
	artifact := inZoneCard(t, e, 0, state.ZGraveyard, "Name:A\nTypes:Artifact\nOracle:x\n")
	tg := func(ids ...state.ObjID) []state.Target {
		var ts []state.Target
		for _, id := range ids {
			ts = append(ts, state.Target{Obj: id})
		}
		return ts
	}
	eval := func(c *effects.Ctx, body string) int32 {
		t.Helper()
		n, ok := effects.EvalCountOK(e, c, body)
		if !ok {
			t.Fatalf("%s not evaluated", body)
		}
		return n
	}
	// Artifact+Creature+Land = 3 distinct; the artifact adds nothing new.
	c := &effects.Ctx{Controller: 0, Remembered: tg(artCreature, land, artifact)}
	if n := eval(c, "Remembered$CardTypes"); n != 3 {
		t.Fatalf("Remembered$CardTypes = %d, want 3", n)
	}
	c = &effects.Ctx{Controller: 0, Remembered: tg(artCreature, artifact)}
	if n := eval(c, "Remembered$CardTypes"); n != 2 {
		t.Fatalf("Remembered$CardTypes over two artifacts = %d, want 2", n)
	}
	c = &effects.Ctx{Controller: 0, Targets: tg(land, artCreature)}
	if n := eval(c, "Targeted$CardTypes"); n != 3 {
		t.Fatalf("Targeted$CardTypes = %d, want 3", n)
	}
}

// TestSacrificedCardTypesReadsSnapshotTypes: Baba Lysaga's "three or more
// card types among the sacrificed permanents" reads the LKI types captured
// before the sacrifice (layer-derived on the battlefield).
func TestSacrificedCardTypesReadsSnapshotTypes(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	ac := onBoardCard(t, e, 0, card(t, "Name:AC\nTypes:Artifact Creature Golem\nPT:1/1\nOracle:x\n"))
	land := onBoardCard(t, e, 0, card(t, "Name:L\nTypes:Land\nOracle:x\n"))
	ench := onBoardCard(t, e, 0, card(t, "Name:E\nTypes:Enchantment\nOracle:x\n"))
	sac := []state.SacrificedInfo{effects.SacrificedLKI(e, ac), effects.SacrificedLKI(e, land), effects.SacrificedLKI(e, ench)}
	for _, id := range []state.ObjID{ac, land, ench} {
		e.emit(events.Sacrifice(id))
	}
	if o := e.G.Obj(ac); o.Zone == state.ZBattlefield {
		t.Fatalf("precondition: sacrificed permanent left the battlefield")
	}
	ctx := &effects.Ctx{Controller: 0, Sacrificed: sac}
	n, ok := effects.EvalCountOK(e, ctx, "Sacrificed$CardTypes")
	if !ok || n != 4 {
		t.Fatalf("Sacrificed$CardTypes = %d,%v want 4 (Artifact, Creature, Land, Enchantment)", n, ok)
	}
	ctx = &effects.Ctx{Controller: 0, Sacrificed: sac[:1]}
	if n, _ := effects.EvalCountOK(e, ctx, "Sacrificed$CardTypes"); n != 2 {
		t.Fatalf("one artifact creature = %d, want 2", n)
	}
	if n, ok := effects.EvalCountOK(e, &effects.Ctx{Controller: 0}, "Sacrificed$CardTypes"); !ok || n != 0 {
		t.Fatalf("nothing sacrificed = %d,%v want 0,true", n, ok)
	}
}

var refCardTypesBody = regexp.MustCompile(`^(?:Count\$)?([A-Za-z]+)\$CardTypes\b`)

// TestCorpusRefCardTypesHeadsAreEvaluated is the corpus-wide census/ratchet:
// every SVar body of the form <Ref>$CardTypes in the corpus must (a) use a ref
// in knownRefCardTypesRefs -- a new ref must be added here deliberately, with
// a behavioural test above -- and (b) come back as an EVALUATED verdict from
// EvalCountOK rather than falling through to "unmodelled".
func TestCorpusRefCardTypesHeadsAreEvaluated(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	known := map[string]bool{"Remembered": true, "TriggerRemembered": true, "TriggerObjectsCards": true, "Sacrificed": true}
	seen := 0
	for _, cd := range reg.AllCards() {
		for _, f := range cd.Faces {
			for _, body := range f.SVars {
				b := strings.TrimSpace(body)
				m := refCardTypesBody.FindStringSubmatch(b)
				if m == nil || strings.HasPrefix(b, "Count$Valid") {
					continue
				}
				seen++
				if !known[m[1]] {
					t.Errorf("%s: %q uses ref %s not in the census list", f.Name, b, m[1])
					continue
				}
				if _, ok := effects.EvalCountOK(e, &effects.Ctx{Controller: 0}, b); !ok {
					t.Errorf("%s: %q is unmodelled (EvalCountOK verdict false)", f.Name, b)
				}
			}
		}
	}
	if seen < 6 {
		t.Fatalf("census saw %d <Ref>$CardTypes bodies, expected >= 6 (corpus missing?)", seen)
	}
}

// TestKeenEyedCuratorCountsTypesAmongCardsExiledWithIt: the Curator's
// `Count$ValidExile Card.ExiledWithSource$CardTypes` is the zone-head form of
// the same distinct-type read; four types among its exiled cards is its +4/+4
// threshold.
func TestKeenEyedCuratorCountsTypesAmongCardsExiledWithIt(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	cur := onBoardCard(t, e, 0, corpusCard(t, "Keen-Eyed Curator"))
	var ids []state.ObjID
	for _, src := range []string{
		"Name:AC\nTypes:Artifact Creature Golem\nPT:1/1\nOracle:x\n",
		"Name:L\nTypes:Land\nOracle:x\n",
		"Name:E\nTypes:Enchantment\nOracle:x\n",
	} {
		id := inZoneCard(t, e, 1, state.ZGraveyard, src)
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZExile, IDs: []state.ObjID{cur}})
		if o := e.G.Obj(id); o.Zone != state.ZExile {
			t.Fatalf("precondition: %s not exiled (zone %s)", src, o.Zone)
		}
		ids = append(ids, id)
	}
	n, ok := effects.EvalCountOK(e, &effects.Ctx{Controller: 0, Source: cur}, "Count$ValidExile Card.ExiledWithSource$CardTypes")
	if !ok || n != 4 {
		t.Fatalf("types among cards exiled with the Curator = %d,%v want 4", n, ok)
	}
}
