package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// room_alt is priced from the pool plus untapped sources (faceCastable's
// census); the unlock special action is offered only when the floating pool
// pays it (gorge prices it off the real pool; the autopay surface drops a
// potential unlock). Mind Spring's and Blaze's lowered witness taps for the
// largest X, so a smaller X leaves mana floating, as in the faces lockstep.
//
// The Room lockstep: two-door Rooms (CR 709.5) cast as
// either door (gorge "room_alt"), unlocked by the special action, their
// UnlockDoor triggers, a counterspell and a bounce that send a face-1 Room
// back to be recast, and Abrupt Decay's mana-value filter reading a
// battlefield Room's unlocked doors.

var roomsDecks = [2][]cardset.Entry{
	{
		{Name: "Bottomless Pool", Count: 3}, {Name: "Meat Locker", Count: 3},
		{Name: "Restricted Office", Count: 3}, {Name: "Grand Entryway", Count: 3},
		{Name: "Surgical Suite", Count: 3}, {Name: "Counterspell", Count: 3}, {Name: "Boomerang", Count: 3},
		{Name: "Savannah Lions", Count: 3}, {Name: "Mind Spring", Count: 3}, {Name: "Plains", Count: 6}, {Name: "Island", Count: 7},
	},
	{
		{Name: "Defiled Crypt", Count: 3}, {Name: "Derelict Attic", Count: 2},
		{Name: "Funeral Room", Count: 2}, {Name: "Unholy Annex", Count: 3},
		{Name: "Ticket Booth", Count: 3},
		{Name: "Roaring Furnace", Count: 3},
		{Name: "Abrupt Decay", Count: 3}, {Name: "Grizzly Bears", Count: 2}, {Name: "Blaze", Count: 3},
		{Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 4}, {Name: "Island", Count: 2},
	},
}

// roomsPolicy prefers the room_alt cast and the unlock special action, then
// any cast or activation, else random.
func roomsPolicy(seed uint64, n map[string]int) Policy {
	return roomsWithheld(roomsPrefer(seed), n)
}

// roomsWithheld wraps a policy with the locked-door count: a priority
// decision in the end step, or in combat after attackers are declared, of
// the active player's turn, while that player controls a Room whose
// locked door prints a trigger of that moment (Unholy Annex's end step,
// Widow's Walk's and Tunnel of Hate's attack, Locker Room's combat damage):
// gorge's scans skip the locked door, and the lockstep proves the kernel
// queues nothing either. Counted once per Room and step.
func roomsWithheld(p Policy, n map[string]int) Policy {
	seen := map[[3]int]bool{}
	return PolicyFunc(func(c *PickCtx) Canon {
		if g := c.G; g != nil {
			gs := g.E.G
			st := gs.Step.String()
			var want string
			switch st {
			case "end":
				want = "Phase"
			case "declare-attackers", "declare-blockers", "combat-damage":
				want = "combat"
			}
			if want != "" {
				for _, id := range gs.Zone(state.ZBattlefield, gs.Active) {
					o := gs.Obj(id)
					if o == nil || o.Card == nil || len(o.Card.Faces) != 2 || !o.Card.Faces[0].IsRoom() {
						continue
					}
					for fi := 0; fi < 2; fi++ {
						if o.DoorUnlocked(fi) {
							continue
						}
						for _, tr := range o.Card.Faces[fi].Triggers {
							m := tr.Mode
							if want == "Phase" && m == "Phase" || want == "combat" && (m == "Attacks" || m == "DamageDoneOnce" || m == "AttackersDeclared") {
								key := [3]int{int(gs.Turn), int(id), fi}
								if !seen[key] {
									seen[key] = true
									n["withheld"]++
								}
							}
						}
					}
				}
			}
		}
		return p.Pick(c)
	})
}

func roomsPrefer(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	alt, _ := faceCastCode("room_alt")
	unl, _ := specialCode("unlock")
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCastFace && a.Mode() == alt || a.Kind() == ActSpecial && a.Mode()&15 == unl {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 && c.G != nil {
			// aim the counterspell, the bounce and the removal at a Room
			// showing its right door, so face 1 leaves the stack and the
			// battlefield and is recast from the hand
			for _, a := range c.D.Options {
				if a.Kind() != ActTarget {
					continue
				}
				if o, ok := c.G.GorgeOption(a); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Card != nil && len(ob.Card.Faces) == 2 && ob.Card.Faces[0].IsRoom() && ob.FaceIdx == 1 {
						buf = append(buf, a)
					}
				}
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActCastFace || k == ActActivate {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestRoomsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("rooms")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range roomsDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(roomsDecks[0], roomsDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	n := map[string]int{}
	st := &dirac.RoomStats{}
	dirac.DebugRoomStats = st
	defer func() { dirac.DebugRoomStats = nil }()
	for seed := uint64(1); seed <= roomsSeeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"rooms-a", "rooms-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := roomsPolicy(seed, n)
		if seed%4 == 0 {
			pol = roomsWithheld(RandomPolicy(seed), n)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("counts %v kernel %+v", n, *st)
	for _, c := range []struct {
		name string
		n    int
	}{
		{"room_alt casts", st.AltCasts}, {"plain casts of a face-1 Room", st.Face1Casts},
		{"unlocks of the other door", st.UnlockOther}, {"UnlockDoor on entry", st.EntryTrigs},
		{"UnlockDoor on unlock", st.UnlockTrigs}, {"locked-door triggers withheld", n["withheld"]},
		{"face-1 Rooms leaving the battlefield", st.Face1Leaves},
		{"battlefield MV reads, one door", st.MVOneDoor}, {"battlefield MV reads, two doors", st.MVTwoDoors},
	} {
		if c.n < 10 {
			t.Errorf("%s: %d, want >= 10", c.name, c.n)
		}
	}
	t.Logf("unlocks of the FaceIdx door (an uncast Room): %d; uncast entries %d", st.UnlockFace, st.UncastEntries)
}

// roomsSeeds is the lockstep's seed count.
const roomsSeeds = 600

// No seed is pinned: the Room lockstep has no tolerated divergence. The
// non-live Room door's trigger resolves against that door's own SVars in
// gorge and the kernel alike (Unholy Annex's DBBranch, Roaring Furnace's
// X:Count$CardsInYourHand); the face-down power filter deviation (Restricted
// Office's DestroyAll powerGE3) is gone too, since mass effects read current
// P/T.

// roomsRun plays the Room lockstep's seeds up to the first divergence and
// returns the number of divergences (0 or 1).
func roomsRun(t *testing.T, seeds uint64) int {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Skip(err)
	}
	set, err := cardset.RawByName("rooms")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range roomsDecks {
		for _, e := range d {
			c, _ := reg.Lookup(e.Name)
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(roomsDecks[0], roomsDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	n := map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"rooms-a", "rooms-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, roomsPolicy(seed, n), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			div++
			break // one divergence settles a mutation
		}
	}
	return div
}

// TestRoomsMutation: each mutation of a gorge Room rule diverges.
func TestRoomsMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, mu := range []struct {
		name string
		flag *bool
	}{
		{"DebugRoomAllLive", &dirac.DebugRoomAllLive},
		{"DebugRoomFaceReset", &dirac.DebugRoomFaceReset},
		{"DebugRoomMVFace", &dirac.DebugRoomMVFace},
	} {
		*mu.flag = true
		d := roomsRun(t, roomsSeeds)
		*mu.flag = false
		t.Logf("%s: diverged %v", mu.name, d != 0)
		if d == 0 {
			t.Errorf("%s: no divergence", mu.name)
		}
	}
}
