package rules

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

const (
	startFDMorph    = "Name:Hidden Ox\nManaCost:3 U\nTypes:Creature Ox\nPT:4/4\nK:Morph:1 U\nOracle:x\n"
	startFDSorcery  = "Name:Quiet Spark\nManaCost:1 R\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1 | SpellDescription$ x\nOracle:x\n"
	startFDCloakee  = "Name:Veiled Heron\nManaCost:2 W\nTypes:Creature Bird\nPT:3/1\nOracle:x\n"
	startFDCreature = "Name:Masked Toad\nManaCost:2 G\nTypes:Creature Frog\nPT:3/3\nOracle:x\n"
)

// startFDConfig puts the face-down fixtures at seat 0's indices 1, 2, 4, 5
// and the DFC at 7.
func startFDConfig(t *testing.T, start *StartSpec) Config {
	t.Helper()
	return startConfig(t, 13, start, [2]map[int]string{{1: startFDMorph, 2: startFDSorcery, 4: startFDCloakee,
		5: startFDCreature, 7: startDFC}, {}})
}

// TestScriptedStartFaceDown: StartSeat.FaceDown enters each named
// battlefield card face down at genesis (CR 708): a morph creature plain,
// a sorcery manifested (a creature permanent while face down, CR 708.2a),
// a creature cloaked and tapped.
func TestScriptedStartFaceDown(t *testing.T) {
	start := &StartSpec{Seats: []StartSeat{{
		Battlefield: []int{1, 2, 4},
		Tapped:      []int{4},
		FaceDown: []StartFaceDown{
			{Card: 1, Kind: StartFaceDownPlain},
			{Card: 2, Kind: StartFaceDownManifest},
			{Card: 4, Kind: StartFaceDownCloak},
		},
	}}}
	cfg := startFDConfig(t, start)
	if err := start.Validate(cfg.Decks); err != nil {
		t.Fatalf("Validate refused a valid face-down start: %v", err)
	}
	e := New(cfg)
	morph, sorc, cloak := e.G.Obj(2), e.G.Obj(3), e.G.Obj(5)
	if morph.Card.Faces[0].Name != "Hidden Ox" || sorc.Card.Faces[0].Name != "Quiet Spark" ||
		cloak.Card.Faces[0].Name != "Veiled Heron" {
		t.Fatalf("precondition: fixtures at the wrong ids (%s, %s, %s)",
			morph.Card.Faces[0].Name, sorc.Card.Faces[0].Name, cloak.Card.Faces[0].Name)
	}
	sc := e.specCtx(0, 0)
	for _, o := range []*state.Object{morph, sorc, cloak} {
		name := o.Card.Faces[0].Name
		if o.Zone != state.ZBattlefield || !o.FaceDown {
			t.Errorf("%s: zone %s face down %v, want battlefield face down", name, o.Zone, o.FaceDown)
			continue
		}
		if p, tg := e.Power(o.ID), e.Toughness(o.ID); p != 2 || tg != 2 {
			t.Errorf("%s: face-down P/T %d/%d, want 2/2", name, p, tg)
		}
		if !e.matchesSpec("Creature.noName", o.ID, sc) {
			t.Errorf("%s: face down, it is not a nameless creature", name)
		}
		if e.matchesSpec("Card.named"+name, o.ID, sc) {
			t.Errorf("%s: face down, it still matches its printed name", name)
		}
	}
	if morph.Manifested || !sorc.Manifested || cloak.Manifested {
		t.Errorf("Manifested: morph %v sorcery %v cloak %v, want false/true/false", morph.Manifested, sorc.Manifested, cloak.Manifested)
	}
	if morph.Cloaked || sorc.Cloaked || !cloak.Cloaked {
		t.Errorf("Cloaked: morph %v sorcery %v cloak %v, want false/false/true", morph.Cloaked, sorc.Cloaked, cloak.Cloaked)
	}
	if morph.Tapped || sorc.Tapped || !cloak.Tapped {
		t.Errorf("Tapped: morph %v sorcery %v cloak %v, want false/false/true", morph.Tapped, sorc.Tapped, cloak.Tapped)
	}
	// CR 702.37e: a face-down permanent with morph turns up for its morph
	// cost. A placed card was not cast, so the offer is the non-cast keyword
	// path (CR 702.36d), not the cast-provenance one.
	if costs := faceDownKeywordFaceUpCosts(morph); len(costs) != 1 || costs[0].family != "Morph" {
		t.Errorf("face-down morph card turn-up costs = %+v, want one Morph cost", costs)
	}
	// CR 701.40b: a manifested non-creature card has no mana-cost turn-up.
	if mf, ok := manifestFaceUpCost(sorc, e.faceCost); ok {
		t.Errorf("manifested sorcery offers a turn-up %+v, want none", mf)
	}
	// Control: the cloaked creature card does turn up for its mana cost, so
	// the sorcery's refusal is the card type's.
	if _, ok := manifestFaceUpCost(cloak, e.faceCost); !ok {
		t.Error("control: the cloaked creature card offers no mana-cost turn-up")
	}
}

// TestScriptedStartFaceDownBadEntries: an unknown kind, an index not in
// Battlefield and a BackFace overlap are skipped, placing the card exactly as
// the start without the entry does; Validate names the kind and the overlap.
func TestScriptedStartFaceDownBadEntries(t *testing.T) {
	seat := func(fd []StartFaceDown) *StartSpec {
		return &StartSpec{Seats: []StartSeat{{
			Battlefield: []int{1, 7}, BackFace: []int{7}, FaceDown: fd,
		}}}
	}
	cases := []struct {
		name  string
		entry StartFaceDown
		id    state.ObjID
		bad   bool
	}{
		{"unknown kind", StartFaceDown{Card: 1, Kind: "morphed"}, 2, true},
		{"not on the battlefield", StartFaceDown{Card: 4, Kind: StartFaceDownPlain}, 5, false},
		{"back face", StartFaceDown{Card: 7, Kind: StartFaceDownManifest}, 8, true},
	}
	base := New(startFDConfig(t, seat(nil)))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := seat([]StartFaceDown{tc.entry})
			cfg := startFDConfig(t, spec)
			e := New(cfg)
			got, want := e.G.Obj(tc.id), base.G.Obj(tc.id)
			if got.FaceDown {
				t.Errorf("card %d entered face down from a bad entry", tc.entry.Card)
			}
			if got.Zone != want.Zone || got.FaceDown != want.FaceDown || got.Face().Name != want.Face().Name {
				t.Errorf("card %d: zone %s face %q, want %s %q as without the entry",
					tc.entry.Card, got.Zone, got.Face().Name, want.Zone, want.Face().Name)
			}
			if e.L.Head() != base.L.Head() {
				t.Error("a skipped face-down entry moved the chain head")
			}
			err := spec.Validate(cfg.Decks)
			if tc.bad && !errors.Is(err, ErrStartFaceDown) {
				t.Errorf("Validate = %v, want ErrStartFaceDown", err)
			}
		})
	}
	if base.G.Obj(8).Face().Name != "Back Half" || base.G.Obj(2).Zone != state.ZBattlefield {
		t.Fatal("precondition: the base start does not place its cards")
	}
	// An Attach overlap is refused too.
	att := &StartSpec{Seats: []StartSeat{{Battlefield: []int{1}, Attach: []StartAttach{{Card: 1, To: 4}},
		FaceDown: []StartFaceDown{{Card: 1, Kind: StartFaceDownPlain}}}}}
	if err := att.Validate(startFDConfig(t, nil).Decks); !errors.Is(err, ErrStartFaceDown) {
		t.Errorf("Validate of an attached face-down card = %v, want ErrStartFaceDown", err)
	}
	// A face-down sorcery is a creature permanent (CR 708.2a): no refusal.
	sorc := &StartSpec{Seats: []StartSeat{{Battlefield: []int{2},
		FaceDown: []StartFaceDown{{Card: 2, Kind: StartFaceDownManifest}}}}}
	decks := startFDConfig(t, nil).Decks
	if err := sorc.Validate(decks); err != nil {
		t.Errorf("Validate of a face-down sorcery = %v, want nil", err)
	}
	// Control: face up, the same sorcery is refused.
	sorc.Seats[0].FaceDown = nil
	if err := sorc.Validate(decks); !errors.Is(err, ErrStartNotPermanent) {
		t.Errorf("control: Validate of a face-up sorcery = %v, want ErrStartNotPermanent", err)
	}
}

// TestScriptedStartFaceDownDeterministic: one face-down config builds one
// chain, and a nil FaceDown is the start without the field.
func TestScriptedStartFaceDownDeterministic(t *testing.T) {
	mk := func(fd []StartFaceDown) *StartSpec {
		return &StartSpec{Seats: []StartSeat{{Battlefield: []int{1, 5}, Counters: []StartCounter{{Card: 5, Kind: "P1P1", N: 1}},
			FaceDown: fd}}}
	}
	cfg := startFDConfig(t, mk([]StartFaceDown{{Card: 5, Kind: StartFaceDownCloak}}))
	a, b := New(cfg), New(cfg)
	if a.L.Head() != b.L.Head() || len(a.L.Events) != len(b.L.Events) {
		t.Fatal("two builds of one face-down start differ")
	}
	if !a.G.Obj(6).FaceDown || a.G.Obj(6).Counter("P1P1") != 1 {
		t.Fatal("precondition: the cloaked card is not face down with its counter")
	}
	plain := New(startFDConfig(t, &StartSpec{Seats: []StartSeat{{Battlefield: []int{1, 5},
		Counters: []StartCounter{{Card: 5, Kind: "P1P1", N: 1}}}}}))
	nilFD := New(startFDConfig(t, mk(nil)))
	if plain.L.Head() != nilFD.L.Head() {
		t.Fatal("a nil FaceDown moved the chain head")
	}
	if plain.L.Head() == a.L.Head() {
		t.Fatal("control: the face-down entry did not change the chain")
	}
}
