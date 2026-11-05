package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep (CR 709.5, 113.7a): Painter's Studio cast as Defaced Gallery,
// then its front door unlocked by Keys to the House. The unlock trigger's
// DB$ Effect names the STPlay static on Painter's Studio's own script table,
// so the cards it exiles must be playable from exile on the next turn. The
// policy casts the room_alt face, casts and activates Keys, plays lands, and
// only then plays the exiled cards from the next turn on (it never plays
// them on the unlock turn, where the permission also exists).
//
// The count is the proof the scenario was reached: at least one game exiles
// two cards and offers BOTH of them from exile on a later turn. A kernel that
// reads the shown face's table offers neither, so the lockstep diverges.
type rdStats struct {
	seen       int
	unlockTurn int
	unlocks    int
	exiled     map[state.ObjID]bool
	playable   map[state.ObjID]bool
}

func rdScan(ref *GorgeSide, st *rdStats) {
	evs := ref.E.L.Events
	for ; st.seen < len(evs); st.seen++ {
		ev := evs[st.seen]
		switch ev.Kind {
		case events.DoorUnlock:
			o := ref.E.G.Obj(ev.Obj)
			// The room_alt cast shows Defaced Gallery (FaceIdx 1); the front
			// door is the alternate one, so its unlock is DoorUnlocked(0).
			if o != nil && o.FaceIdx == 1 && o.DoorUnlocked(0) && o.Card != nil &&
				len(o.Card.Faces) > 0 && o.Card.Faces[0].Name == "Painter's Studio" {
				st.unlocks++
				st.unlockTurn = int(ref.E.G.Turn)
			}
		case events.MoveZone:
			if st.unlockTurn != 0 && ev.From == state.ZLibrary && ev.To == state.ZExile {
				if st.exiled == nil {
					st.exiled = map[state.ObjID]bool{}
				}
				st.exiled[ev.Obj] = true
			}
		}
	}
}

// rdName is the printed front name of the card action a casts or chooses.
func rdName(c *PickCtx, a Canon) string {
	if c.G == nil {
		return ""
	}
	o, ok := c.G.GorgeOption(a)
	if !ok {
		return ""
	}
	if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Card != nil && len(ob.Card.Faces) > 0 {
		return ob.Card.Faces[0].Name
	}
	return ""
}

// rdPainterLocked reports whether seat 0 controls Painter's Studio as its
// back half with the front door still locked.
func rdPainterLocked(c *PickCtx) bool {
	if c.G == nil {
		return false
	}
	for _, id := range c.G.E.G.Zone(state.ZBattlefield, 0) {
		o := c.G.E.G.Obj(id)
		if o != nil && o.FaceIdx == 1 && !o.DoorUnlocked(0) && o.Card != nil &&
			len(o.Card.Faces) > 0 && o.Card.Faces[0].Name == "Painter's Studio" {
			return true
		}
	}
	return false
}

func rdPolicy(st *rdStats) Policy {
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil || c.D.Player != 0 {
			for _, a := range c.D.Options {
				if a.Kind() == ActPlayLand || a.Kind() == ActPass {
					return a
				}
			}
			return c.D.Options[0]
		}
		if c.D.Kind == DChoose {
			for _, a := range c.D.Options {
				if o, ok := c.G.GorgeOption(a); ok && o.Kind == "unlock" {
					return a
				}
			}
			return c.D.Options[0]
		}
		if c.D.Kind == DTarget {
			for _, a := range c.D.Options {
				if a.Kind() != ActTarget {
					continue
				}
				if o, ok := c.G.GorgeOption(a); ok && o.Obj != 0 {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.FaceIdx == 1 && !ob.DoorUnlocked(0) &&
						ob.Card != nil && len(ob.Card.Faces) > 0 && ob.Card.Faces[0].Name == "Painter's Studio" {
						return a
					}
				}
			}
			for _, a := range c.D.Options {
				if a.Kind() == ActDone {
					return a
				}
			}
			return c.D.Options[0]
		}
		// Cast Painter's Studio as its other (Defaced Gallery) half.
		for _, a := range c.D.Options {
			if a.Kind() == ActCastFace && a.Mode()&15 == faceCodeRoomAlt && rdName(c, a) == "Painter's Studio" {
				return a
			}
		}
		// With Painter's Studio live as its back half, cast and activate Keys
		// to the House to unlock the front door.
		if rdPainterLocked(c) {
			for _, a := range c.D.Options {
				if a.Kind() == ActCast && rdName(c, a) == "Keys to the House" {
					return a
				}
			}
			for _, a := range c.D.Options {
				if a.Kind() != ActActivate {
					continue
				}
				o, ok := c.G.GorgeOption(a)
				if !ok || o.Obj == 0 {
					continue
				}
				ob := c.G.E.G.Obj(o.Obj)
				if ob == nil || ob.Face() == nil || int(o.Ability) >= len(ob.Face().Abilities) {
					continue
				}
				if sa := ob.Face().Abilities[o.Ability]; sa != nil && sa.API == "UnlockDoor" {
					return a
				}
			}
		}
		// Next turn: play the cards Painter's Studio exiled. Record every one
		// the priority list offers, then take the first.
		if st.unlockTurn != 0 && int(c.G.E.G.Turn) > st.unlockTurn {
			var first Canon
			for _, a := range c.D.Options {
				k := a.Kind()
				if k != ActCast && k != ActCastFace && k != ActPlayLand {
					continue
				}
				o, ok := c.G.GorgeOption(a)
				if !ok || o.Obj == 0 || !st.exiled[o.Obj] {
					continue
				}
				if ob := c.G.E.G.Obj(o.Obj); ob == nil || ob.Zone != state.ZExile {
					continue
				}
				if st.playable == nil {
					st.playable = map[state.ObjID]bool{}
				}
				st.playable[o.Obj] = true
				if first == 0 {
					first = a
				}
			}
			if first != 0 {
				return first
			}
		}
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
				return a
			}
		}
		for _, a := range c.D.Options {
			if a.Kind() == ActPass {
				return a
			}
		}
		return c.D.Options[0]
	})
}

func TestRoomDoorTriggerSVarsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-dsk")
	entries := []cardset.Entry{
		{Name: "Painter's Studio", Count: 8},
		{Name: "Keys to the House", Count: 8},
		{Name: "Clockwork Percussionist", Count: 8},
		{Name: "Mountain", Count: 16},
	}
	var deck []*cards.Card
	for _, e := range entries {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			deck = append(deck, c)
		}
	}
	if len(deck) != 40 {
		t.Fatalf("deck has %d cards", len(deck))
	}
	opponent := make([]*cards.Card, 40)
	forest, ok := reg.Lookup("Forest")
	if !ok {
		t.Fatal("Forest not in the corpus")
	}
	for i := range opponent {
		opponent[i] = forest
	}
	decks := [2][]*cards.Card{deck, opponent}
	tbl, _, _, err := set.Matchup(entries, []cardset.Entry{{Name: "Forest", Count: 40}})
	if err != nil {
		t.Fatal(err)
	}

	var games, unlocks, exiled, bothPlayable int
	for seed := uint64(1); seed <= 80; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"rdts-a", "rdts-b"}, decks, 2)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		st := &rdStats{}
		check := func(ref *GorgeSide, _ Side) *Divergence {
			rdScan(ref, st)
			return nil
		}
		res := Lockstep(m, seed, k, rdPolicy(st), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 12, Check: check})
		CloseSide(k)
		games++
		unlocks += st.unlocks
		exiled += len(st.exiled)
		if len(st.playable) >= 2 {
			bothPlayable++
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("games %d, painter unlocks %d, exiled %d, games with both exiled cards playable next turn %d", games, unlocks, exiled, bothPlayable)
	if unlocks == 0 {
		t.Fatalf("the matchup never unlocked Painter's Studio's front door")
	}
	if exiled < 2 {
		t.Fatalf("Painter's Studio exiled %d cards, want at least 2", exiled)
	}
	if bothPlayable == 0 {
		t.Fatalf("no game offered both exiled cards from exile on a later turn")
	}
}
