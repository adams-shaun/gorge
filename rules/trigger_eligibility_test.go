package rules

import (
	"reflect"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

var benchmarkObjectTriggerEligibility bool

// The mapping is an over-approximation of the CURRENT matcher, not an
// expansion of Forge support. SpellAbilityCast currently means AbilityPush.
func TestTriggerEligibilityEventMatrix(t *testing.T) {
	for _, tc := range []struct {
		mode  string
		kinds []events.Kind // nil means conservatively retain every event
	}{
		{"ChangesZone", []events.Kind{events.MoveZone, events.Draw, events.PutOnStack}},
		{"SpellCast", []events.Kind{events.PutOnStack}},
		{"AbilityCast", []events.Kind{events.AbilityPush}},
		{"SpellAbilityCast", []events.Kind{events.AbilityPush}},
		{"Attacks", []events.Kind{events.DeclareAttackers}},
		{"AttackersDeclaredOneTarget", []events.Kind{events.DeclareAttackers}},
		{"Sacrificed", []events.Kind{events.MoveZone}},
		{"Discarded", []events.Kind{events.MoveZone}},
		{"CommitCrime", []events.Kind{events.TargetsChosen}},
		{"Taps", []events.Kind{events.Tap}},
		{"TapsForMana", []events.Kind{events.Tap}},
		{"DamageDone", []events.Kind{events.Damage}},
		{"DamageDealtOnce", []events.Kind{events.Damage}},
		{"DamageDoneOnce", []events.Kind{events.Damage}},
		{"Drawn", []events.Kind{events.Draw}},
		{"LifeLost", []events.Kind{events.Damage, events.LifeChange}},
		{"LifeLostAll", nil},
		{"BecomesTarget", []events.Kind{events.TargetsChosen}},
		{"Attached", []events.Kind{events.Attach}},
		{"Explores", []events.Kind{events.Explore}},
		{"Investigated", []events.Kind{events.Investigate}},
		{"Exerted", []events.Kind{events.Exert}},
		{"LandPlayed", []events.Kind{events.MoveZone}},
		{"Phase", []events.Kind{events.StepChange}},
		{"Always", nil},
		{"FutureMode", nil},
		{"", nil},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			mask := triggerModeEvents(tc.mode)
			for k := 0; k < 256; k++ {
				kind := events.Kind(k)
				// Kinds beyond this representation must fail OPEN to the old
				// matcher, never silently truncate a new event's eligibility.
				want := tc.kinds == nil || k >= triggerMaskKindBits || slices.Contains(tc.kinds, kind)
				if got := mask.allows(kind); got != want {
					t.Fatalf("%s kind %d: eligible=%v, want %v", tc.mode, k, got, want)
				}
			}
		})
	}
}

func TestTriggerEventInterestMapping(t *testing.T) {
	for kind := events.Kind(0); int(kind) < events.NumKinds; kind++ {
		var want cards.TriggerInterest
		switch kind {
		case events.MoveZone:
			want = cards.TriggerInterestZoneChange
		case events.Draw:
			want = cards.TriggerInterestZoneChange | cards.TriggerInterestDraw
		case events.LifeChange:
			want = cards.TriggerInterestLifeChange
		case events.Damage:
			want = cards.TriggerInterestDamage
		case events.Tap:
			want = cards.TriggerInterestTap
		case events.StepChange:
			want = cards.TriggerInterestStepChange
		case events.PutOnStack:
			want = cards.TriggerInterestZoneChange | cards.TriggerInterestStackPut
		case events.DeclareAttackers, events.DeclareBlockers:
			want = cards.TriggerInterestAttackDeclaration
		case events.TargetsChosen:
			want = cards.TriggerInterestTargetsChosen
		case events.AbilityPush:
			want = cards.TriggerInterestAbilityPush
		case events.Attach:
			want = cards.TriggerInterestAttach
		case events.Explore:
			want = cards.TriggerInterestExplore
		case events.Investigate:
			// A trigger-relevant Kind past the 64-bit mask's reach: the
			// conservative catch-all, and compiledTriggerInterestAllows fails
			// open for it before this mapping is even consulted.
			want = cards.TriggerInterestAny
		}
		if got := eventTriggerInterest(kind); got != want {
			t.Fatalf("kind %s interest = %x, want %x", kind, got, want)
		}
	}
	if got := eventTriggerInterest(events.Kind(events.NumKinds)); got != cards.TriggerInterestAny {
		t.Fatalf("future event interest = %x, want catch-all", got)
	}
}

func TestCompiledTriggerInterestParity(t *testing.T) {
	modes := []string{
		"ChangesZone", "SpellCast", "AbilityCast", "SpellAbilityCast", "Attacks",
		"AttackersDeclaredOneTarget", "AttackersDeclared", "AttackerBlocked", "AttackerBlockedByCreature", "Sacrificed",
		"Discarded", "LandPlayed", "Cycled", "CommitCrime", "BecomesTarget", "Taps",
		"TapsForMana", "DamageDone", "DamageDealtOnce", "DamageDoneOnce", "CounterAdded",
		"Drawn", "LifeLost", "Phase", "Attached", "Explores", "Investigated", "Always", "LifeLostAll", "FutureMode", "",
	}
	card := &cards.Card{}
	for _, mode := range modes {
		card.Faces = append(card.Faces, &cards.Face{Triggers: []cards.Trigger{{Mode: mode}}})
	}
	phaseDiagnostic := &cards.Face{Triggers: []cards.Trigger{{Mode: "SpellCast", Params: map[string]string{"Phase": "Bad"}}}}
	card.Faces = append(card.Faces, phaseDiagnostic)
	r := cards.NewRegistry()
	r.Add(card)
	if err := r.CompileMetadata(); err != nil {
		t.Fatal(err)
	}

	e := &Engine{}
	for i, mode := range modes {
		face := card.Faces[i]
		for kind := events.Kind(0); int(kind) < events.NumKinds; kind++ {
			want := triggerModeEvents(mode).allows(kind)
			if got := e.faceMayTrigger(face, kind); want && !got {
				t.Fatalf("mode %q kind %s: compiled prefilter rejected a textual candidate", mode, kind)
			}
		}
	}
	for kind := events.Kind(0); int(kind) < events.NumKinds; kind++ {
		if !e.faceMayTrigger(phaseDiagnostic, kind) {
			t.Fatalf("phase diagnostic rejected %s", kind)
		}
	}
	if len(e.triggerEventMasks) != 0 {
		t.Fatalf("bound faces populated textual pointer cache with %d entries", len(e.triggerEventMasks))
	}
}

func TestTriggerEligibilityFaceUnionAndConservativeFallback(t *testing.T) {
	for _, tc := range []struct {
		name string
		face *cards.Face
		kind events.Kind
		want bool
	}{
		{"nil", nil, events.Note, false},
		{"empty", &cards.Face{}, events.Note, false},
		{"irrelevant", &cards.Face{Triggers: []cards.Trigger{{Mode: "SpellCast"}}}, events.Note, false},
		{"union", &cards.Face{Triggers: []cards.Trigger{{Mode: "SpellCast"}, {Mode: "Drawn"}}}, events.Draw, true},
		{"phase diagnostic", &cards.Face{Triggers: []cards.Trigger{{Mode: "SpellCast", Params: map[string]string{"Phase": "Bad"}}}}, events.Note, true},
		{"valid phase", &cards.Face{Triggers: []cards.Trigger{{Mode: "SpellCast", Params: map[string]string{"Phase": "Upkeep"}}}}, events.Note, true},
		{"blank phase", &cards.Face{Triggers: []cards.Trigger{{Mode: "SpellCast", Params: map[string]string{"Phase": " \t"}}}}, events.Note, false},
		{"state trigger", &cards.Face{Triggers: []cards.Trigger{{Mode: "Always"}}}, events.Note, true},
		{"life batch", &cards.Face{Triggers: []cards.Trigger{{Mode: "LifeLostAll"}}}, events.Note, true},
		{"unknown", &cards.Face{Triggers: []cards.Trigger{{Mode: "FutureMode"}}}, events.Note, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := &Engine{}
			if got := e.faceMayTrigger(tc.face, tc.kind); got != tc.want {
				t.Fatalf("eligible=%v, want %v", got, tc.want)
			}
			if a := testing.AllocsPerRun(100, func() { e.faceMayTrigger(tc.face, tc.kind) }); a != 0 {
				t.Fatalf("warm face lookup allocated %v objects", a)
			}
		})
	}
}

func TestTriggerEligibilityCacheIsCloneIndependent(t *testing.T) {
	e := layerEngine(t)
	f := &cards.Face{Triggers: []cards.Trigger{{Mode: "SpellCast"}}}
	e.faceMayTrigger(f, events.Note)
	c := e.Clone()
	other := &cards.Face{Triggers: []cards.Trigger{{Mode: "Drawn"}}}
	if !c.faceMayTrigger(other, events.Draw) {
		t.Fatal("clone rejected newly encountered face")
	}
	if _, shared := e.triggerEventMasks[other]; shared {
		t.Fatal("clone writes to parent's eligibility cache")
	}
	// Concurrent misses on independently writable caches must not race.
	for _, branch := range []*Engine{e, c} {
		t.Run("branch", func(t *testing.T) {
			t.Parallel()
			for range 100 {
				face := &cards.Face{Triggers: []cards.Trigger{{Mode: "Attacks"}}}
				if !branch.faceMayTrigger(face, events.DeclareAttackers) {
					t.Fatal("branch rejected attack trigger")
				}
			}
		})
	}
}

// TestObjectTriggerEligibilityTracksBothFaces catches a dense object cache
// reusing the active face's mask after a transform or evicting one Room half
// when both halves are checked on every event.
func TestObjectTriggerEligibilityTracksBothFaces(t *testing.T) {
	e := layerEngine(t)
	spell := &cards.Face{Triggers: []cards.Trigger{{Mode: "SpellCast"}}}
	draw := &cards.Face{Triggers: []cards.Trigger{{Mode: "Drawn"}}}
	card := &cards.Card{Faces: []*cards.Face{spell, draw}}
	r := cards.NewRegistry()
	r.Add(card)
	if err := r.CompileMetadata(); err != nil {
		t.Fatal(err)
	}
	o := e.G.AddObject(card, 0)

	if !e.objectFaceMayTrigger(o.ID, 0, spell, events.PutOnStack) {
		t.Fatal("front face rejected its spell-cast event")
	}
	if e.objectFaceMayTrigger(o.ID, 0, spell, events.Draw) {
		t.Fatal("front face admitted an unrelated draw")
	}
	if !e.objectFaceMayTrigger(o.ID, 1, draw, events.Draw) {
		t.Fatal("back face rejected its draw event")
	}
	if e.objectFaceMayTrigger(o.ID, 1, draw, events.PutOnStack) {
		t.Fatal("back face admitted an unrelated cast")
	}
	if !e.objectFaceMayTrigger(o.ID, 0, spell, events.PutOnStack) {
		t.Fatal("checking the back face evicted the front-face eligibility")
	}

	c := e.Clone()
	replacement := &cards.Face{Triggers: []cards.Trigger{{Mode: "Attacks"}}}
	if !c.objectFaceMayTrigger(o.ID, 0, replacement, events.DeclareAttackers) {
		t.Fatal("clone did not independently accept replacement face metadata")
	}
	if !e.objectFaceMayTrigger(o.ID, 0, spell, events.PutOnStack) {
		t.Fatal("clone cache mutation changed parent eligibility")
	}
}

// Bound catalog faces own immutable trigger-interest metadata. Looking one up
// must not allocate or populate a mutable per-engine object cache merely to
// repeat that same immutable lookup.
func TestCompiledObjectTriggerEligibilitySkipsRuntimeCache(t *testing.T) {
	e := &Engine{G: state.NewGame([]string{"a"})}
	face := &cards.Face{Triggers: []cards.Trigger{{Mode: "SpellCast"}}}
	card := &cards.Card{Faces: []*cards.Face{face}}
	r := cards.NewRegistry()
	r.Add(card)
	if err := r.CompileMetadata(); err != nil {
		t.Fatal(err)
	}
	o := e.G.AddObject(card, 0)
	if !e.objectFaceMayTrigger(o.ID, o.FaceIdx, face, events.PutOnStack) {
		t.Fatal("compiled face rejected its spell-cast event")
	}
	if len(e.triggerObjectMasks) != 0 {
		t.Fatalf("compiled face populated %d runtime object-mask entries", len(e.triggerObjectMasks))
	}
}

func BenchmarkCompiledObjectTriggerEligibility(b *testing.B) {
	e := &Engine{G: state.NewGame([]string{"a"})}
	face := &cards.Face{Triggers: []cards.Trigger{{Mode: "SpellCast"}}}
	card := &cards.Card{Faces: []*cards.Face{face}}
	r := cards.NewRegistry()
	r.Add(card)
	if err := r.CompileMetadata(); err != nil {
		b.Fatal(err)
	}
	o := e.G.AddObject(card, 0)
	b.ReportAllocs()
	for range b.N {
		benchmarkObjectTriggerEligibility = e.objectFaceMayTrigger(o.ID, o.FaceIdx, face, events.PutOnStack)
	}
}

func TestGrantedKeywordTriggerEventFilter(t *testing.T) {
	for _, tc := range []struct {
		kind events.Kind
		want bool
	}{
		{events.TargetsChosen, true},
		{events.DeclareAttackers, true},
		{events.Priority, false},
		{events.MoveZone, false},
		{events.StepChange, false},
	} {
		if got := grantedKeywordTriggerEvent(tc.kind); got != tc.want {
			t.Fatalf("kind %s: eligible=%v, want %v", tc.kind, got, tc.want)
		}
	}
}

// An impossible event must be rejected before parsing dynamic gates, even on
// a cold look-back observer. Moving the kind guard after phaseGate allocates.
func TestTriggerEligibilityRejectsBeforeDynamicGates(t *testing.T) {
	e := layerEngine(t)
	id := onBoard(t, e, 0, "Name:Gate probe\nTypes:Enchantment\nOracle:x\n")
	tr := cards.Trigger{Mode: "ChangesZone", Params: map[string]string{"Phase": "Upkeep"}}
	observer := &Engine{G: e.G, L: e.L}
	allocs := testing.AllocsPerRun(100, func() {
		observer.phaseSpecs = nil // cold syntax; observer construction is setup
		if observer.triggerMatches(tr, id, events.Event{Kind: events.Note}, nil) {
			t.Fatal("a Note matched ChangesZone")
		}
	})
	if allocs != 0 {
		t.Fatalf("irrelevant event allocated %.0f objects evaluating gates; want zero", allocs)
	}
}

// These synthetic fixtures pin the scanner's existing semantics before
// pruning. Diagnostics ignore event kind AND source zone; duplicate specs
// name the first source in seat/zone/face/trigger order, not insertion order.
func TestTriggerEligibilityKeepsHiddenDiagnosticOrder(t *testing.T) {
	e := layerEngine(t)
	later := onBoard(t, e, 0, "Name:Later\nTypes:Enchantment\n"+
		"T:Mode$ ChangesZone | Phase$ BadA | Execute$ Gain\n"+
		"T:Mode$ SpellCast | Phase$ BadC | Execute$ Gain\n"+
		"SVar:Gain:DB$ GainLife | LifeAmount$ 1 | Defined$ You\nOracle:x\n")
	first := onBoard(t, e, 0, "Name:Hidden first\nTypes:Enchantment\n"+
		"T:Mode$ SpellCast | Phase$ BadA | Execute$ Gain\n"+
		"T:Mode$ FutureMode | Phase$ BadB | Execute$ Gain\n"+
		"SVar:Gain:DB$ GainLife | LifeAmount$ 1 | Defined$ You\nOracle:x\n")
	events.Apply(e.G, events.Event{Kind: events.MoveZone, Obj: first, From: state.ZBattlefield, To: state.ZLibrary})
	start := len(e.L.Events)
	for range 2 {
		e.checkFaceTriggers(e, events.Event{Kind: events.Priority}, nil, 0, 0, false, false, false)
	}
	var got []string
	var sources []state.ObjID
	for _, ev := range e.L.Events[start:] {
		got = append(got, ev.Text)
		sources = append(sources, ev.Obj)
	}
	want := []string{
		"Phase$ BadA names no engine step; the trigger never fires",
		"Phase$ BadB names no engine step; the trigger never fires",
		"Phase$ BadC names no engine step; the trigger never fires",
	}
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(sources, []state.ObjID{first, first, later}) {
		t.Fatalf("diagnostics = %v from %v; want %v in zone order", got, sources, want)
	}
}

func TestTriggerEligibilityKeepsAlwaysOnBookkeepingEvents(t *testing.T) {
	e := layerEngine(t)
	id := onBoard(t, e, 0, "Name:State watcher\nTypes:Enchantment\n"+
		"T:Mode$ Always | LifeTotal$ You | LifeAmount$ GE20 | Execute$ Gain\n"+
		"SVar:Gain:DB$ GainLife | LifeAmount$ 1 | Defined$ You\nOracle:x\n")
	for _, kind := range []events.Kind{events.Priority, events.Note, events.DecisionMade} {
		e.checkFaceTriggers(e, events.Event{Kind: kind}, nil, 0, 0, false, false, false)
	}
	if len(e.pendingTriggers) != 1 || e.pendingTriggers[0].Source != id {
		t.Fatalf("Always queue = %+v, want exactly one outstanding instance", e.pendingTriggers)
	}
}

// The active face has no printed triggers. Its fast rejection must still
// inspect an unlocked Room's other face, including when face 1 was cast.
func TestTriggerEligibilityKeepsRoomAlternateFace(t *testing.T) {
	for _, face := range []int32{0, 1} {
		e := layerEngine(t)
		quiet := "Name:Quiet\nTypes:Enchantment Room\nOracle:x\n"
		watcher := "Name:Watcher\nTypes:Enchantment Room\n" +
			"T:Mode$ SpellCast | Execute$ Gain\nSVar:Gain:DB$ GainLife | LifeAmount$ 1 | Defined$ You\nOracle:x\n"
		src := quiet + "ALTERNATE\n" + watcher
		if face == 1 {
			src = watcher + "ALTERNATE\n" + quiet
		}
		id := onBoard(t, e, 0, src)
		events.Apply(e.G, events.Event{Kind: events.FlipFace, Obj: id, Amount: face})
		spell := onBoard(t, e, 0, "Name:Spell\nTypes:Sorcery\nOracle:x\n")
		ev := events.Event{Kind: events.PutOnStack, Obj: spell, Player: 0}
		e.checkFaceTriggers(e, ev, nil, 0, 0, false, false, false)
		if len(e.pendingTriggers) != 0 {
			t.Fatal("locked alternate face fired")
		}
		events.Apply(e.G, events.Event{Kind: events.DoorUnlock, Obj: id})
		e.checkFaceTriggers(e, ev, nil, 0, 0, false, false, false)
		if len(e.pendingTriggers) != 1 || e.pendingTriggers[0].Source != id || !e.pendingTriggers[0].Delayed {
			t.Fatalf("cast face %d: queue = %+v, want alternate-face trigger", face, e.pendingTriggers)
		}
	}
}
