package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep (CR 709.5f-g, 601.2f): the three
// UnlockDoor effects (Ghostly Keybearer, Keys to the House, Marina Vendrell),
// Inquisitive Glimmer's "unlock costs you pay cost {1} less" and Unwilling
// Vessel's possession counter on its Eerie triggers. The policy casts the
// Rooms, takes the paid unlock special action (so Glimmer's reduction prices
// both the offer and the charge), activates Keys/Marina (the effect half of
// the DoorUnlock/DoorLock pair) and answers gorge's "Lock or unlock a door?"
// and "Choose a Room door" asks, both halves across seeds.
//
// Counts are read from gorge's event log: a DoorUnlock caused by the effect
// (not the special action), a DoorLock, a paid special-action unlock while
// Inquisitive Glimmer is on the battlefield (its reduction applied) and a
// POSSESSION counter added (Unwilling Vessel's Eerie triggers only).

// kdrDecks. Deck A's Mind Spring and Blaze are the float-mana support from
// fullyunlock's pool (support already in) that this test
// kept: an unlock special action pays from the floating pool only
// (legal_walk_battlefield.go offerCastable), so an X spell is what leaves the
// mana that makes the offer reachable and lets a below-printed charge be
// observed. Everything else is the mechanic's own list.
var kdrDecks = [2][]cardset.Entry{
	{
		{Name: "Inquisitive Glimmer", Count: 4}, {Name: "Unwilling Vessel", Count: 1},
		{Name: "Marina Vendrell", Count: 2}, {Name: "Keys to the House", Count: 2},
		{Name: "Ghostly Keybearer", Count: 2},
		{Name: "Defiled Crypt", Count: 2}, {Name: "Derelict Attic", Count: 2},
		{Name: "Bottomless Pool", Count: 2}, {Name: "Unholy Annex", Count: 2},
		{Name: "Mind Spring", Count: 2}, {Name: "Blaze", Count: 2},
		{Name: "Plains", Count: 2}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 8},
		{Name: "Mountain", Count: 1}, {Name: "Forest", Count: 1},
	},
	{
		{Name: "Ghostly Keybearer", Count: 3}, {Name: "Keys to the House", Count: 3},
		{Name: "Unwilling Vessel", Count: 1}, {Name: "Inquisitive Glimmer", Count: 2},
		{Name: "Grand Entryway", Count: 4}, {Name: "Surgical Suite", Count: 3},
		{Name: "Roaring Furnace", Count: 2}, {Name: "Ticket Booth", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Lightning Bolt", Count: 1},
		{Name: "Plains", Count: 8}, {Name: "Island", Count: 3},
		{Name: "Mountain", Count: 3}, {Name: "Swamp", Count: 2},
	},
}

// kdrStats is the lockstep's event-derived census.
type kdrStats struct {
	effectUnlocks  int // DoorUnlock by an effect (not the special action)
	locks          int // DoorLock
	reducedUnlocks int // a special-action unlock whose charge was below the printed cost
	possession     int // POSSESSION counters added (Eerie triggers only)

	wantSpecial    bool  // the last priority pick was the unlock special action
	afterHalf      bool  // the next DChoose is the Room door pick
	specialWho     uint8 // the player who took the special action
	specialSpent   int   // mana that action has paid (negative ManaAdd events)
	specialGlimmer bool  // Inquisitive Glimmer was out at the pick
}

// kdrScan consumes gorge's new events at every decision.
func kdrScan(ref *GorgeSide, seen *int, st *kdrStats) {
	evs := ref.E.L.Events
	for ; *seen < len(evs); *seen++ {
		ev := evs[*seen]
		switch ev.Kind {
		case events.ManaAdd:
			// A special action's payment comes from the pool as negative
			// ManaAdd events; sum them until its DoorUnlock names the door.
			if st.wantSpecial && ev.Amount < 0 && uint8(ev.Player) == st.specialWho {
				st.specialSpent += int(-ev.Amount)
			}
		case events.DoorUnlock:
			if st.wantSpecial {
				st.wantSpecial = false
				if st.specialGlimmer {
					if printed, ok := kdrDoorCost(ref, ev); ok && st.specialSpent < printed {
						st.reducedUnlocks++
					}
				}
			} else {
				st.effectUnlocks++
			}
		case events.DoorLock:
			st.locks++
		case events.CounterChange:
			if ev.Counter == "POSSESSION" && ev.Amount > 0 {
				st.possession += int(ev.Amount)
			}
		}
	}
}

// kdrDoorCost is the printed mana value of the Room face a special-action
// DoorUnlock names: the event's Amount is face+1 for the cast door and zero
// for the alternate one (legal_priority.go's optUnlock encoding).
func kdrDoorCost(g *GorgeSide, ev events.Event) (int, bool) {
	o := g.E.G.Obj(ev.Obj)
	if o == nil || o.Card == nil || len(o.Card.Faces) == 0 {
		return 0, false
	}
	fi := 1 - int(o.FaceIdx)
	if ev.Amount > 0 {
		fi = int(ev.Amount) - 1
	}
	if fi < 0 || fi >= len(o.Card.Faces) || o.Card.Faces[fi] == nil {
		return 0, false
	}
	return int(o.Card.Faces[fi].ManaValue()), true
}

// kdrGlimmerOut reports whether p controls an Inquisitive Glimmer.
func kdrGlimmerOut(g *GorgeSide, p state.PlayerID) bool {
	for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Inquisitive Glimmer" {
			return true
		}
	}
	return false
}

// kdrSpecial reports whether a is the paid unlock special action.
func kdrSpecial(c *PickCtx, a Canon) bool {
	if a.Kind() != ActSpecial {
		return false
	}
	k, _ := specialCode("unlock")
	return uint8(a.Mode()&15) == k
}

// kdrPolicy plays the Rooms: paid unlock special actions first, then the
// Keys/Marina effect activations, then the Room and carrier casts, then any
// spell and a land. It answers both mid-resolution asks by their gorge option
// kinds and always chooses a Room target. The lock-or-unlock half varies by
// seed.
func kdrPolicy(seed uint64, swap int, st *kdrStats) Policy {
	half := (seed + uint64(swap)) % 2
	return PolicyFunc(func(c *PickCtx) Canon {
		// A special action taken earlier and not followed by an unlock (an
		// unpayable or fizzled choice) is forgotten at the next priority.
		if c.D.Kind == DPriority && st.wantSpecial {
			st.wantSpecial = false
		}
		if c.G != nil {
			// A Room door pick follows the half answer at the next DChoose.
			cardKind, _ := chooseCode("card")
			if st.afterHalf && c.D.Kind == DChoose && len(c.D.Options) > 0 && c.D.Options[0].Kind() == ActChoose+CanonKind(cardKind) {
				st.afterHalf = false
				return c.D.Options[int((seed+uint64(c.Seq))/2)%len(c.D.Options)]
			}
			// The lock-or-unlock half is gorge's KChoose whose option kinds
			// are exactly "unlock" and "lock" (option 0 is unlock).
			var unlockOpt, lockOpt Canon
			for _, a := range c.D.Options {
				if o, ok := c.G.GorgeOption(a); ok {
					switch o.Kind {
					case "unlock":
						unlockOpt = a
					case "lock":
						lockOpt = a
					}
				}
			}
			if c.D.Kind == DChoose && (unlockOpt != 0 || lockOpt != 0) {
				st.afterHalf = true
				if half == 1 && lockOpt != 0 {
					return lockOpt
				}
				if unlockOpt != 0 {
					return unlockOpt
				}
				return lockOpt
			}
		}
		if st.afterHalf && c.D.Kind != DChoose {
			st.afterHalf = false
		}
		if c.D.Kind == DTarget {
			// A Room target first; the "up to one" Done declines nothing.
			var first Canon
			for _, a := range c.D.Options {
				if a.Kind() != ActTarget {
					continue
				}
				if first == 0 {
					first = a
				}
				if c.G != nil {
					if o, ok := c.G.GorgeOption(a); ok && o.Obj != 0 {
						if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Card != nil && len(ob.Card.Faces) > 0 && ob.Card.Faces[0].IsRoom() {
							return a
						}
					}
				}
			}
			if first != 0 {
				return first
			}
		}
		// Priority: the special action, then Keys/Marina, then the Rooms and
		// the carriers, then anything else and a land.
		var special, act, xspell, room, carrier, other, land Canon
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActSpecial:
				if kdrSpecial(c, a) && special == 0 {
					special = a
				}
			case ActActivate:
				if act == 0 && c.G != nil {
					if o, ok := c.G.GorgeOption(a); ok && o.Obj != 0 && o.Kind == "ability" {
						if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && int(o.Ability) < len(ob.Face().Abilities) {
							if sa := ob.Face().Abilities[o.Ability]; sa != nil && sa.API == "UnlockDoor" {
								act = a
							}
						}
					}
				}
			case ActCast:
				if c.G != nil {
					if o, ok := c.G.GorgeOption(a); ok && o.Obj != 0 {
						if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Card != nil && len(ob.Card.Faces) > 0 {
							f := ob.Card.Faces[0]
							if f.IsRoom() {
								if room == 0 {
									room = a
								}
								continue
							}
							switch f.Name {
							case "Mind Spring", "Blaze":
								if xspell == 0 {
									xspell = a
								}
								continue
							case "Inquisitive Glimmer", "Unwilling Vessel", "Ghostly Keybearer", "Keys to the House", "Marina Vendrell":
								if carrier == 0 {
									carrier = a
								}
								continue
							}
						}
					}
				}
				if other == 0 {
					other = a
				}
			case ActPlayLand:
				if land == 0 {
					land = a
				}
			}
		}
		switch {
		case special != 0:
			if c.G != nil && c.D.Player < 2 {
				st.specialWho = c.D.Player
				st.specialSpent = 0
				st.specialGlimmer = kdrGlimmerOut(c.G, state.PlayerID(c.D.Player))
			}
			st.wantSpecial = true
			return special
		case act != 0:
			return act
		case xspell != 0:
			return xspell
		case room != 0:
			return room
		case carrier != 0:
			return carrier
		case other != 0:
			return other
		case land != 0:
			return land
		}
		// Never take a Room's other-door cast: gorge skips the cast door's
		// UnlockDoor trigger for a room_alt entry (the gorge
		// finding), so the oracle avoids that route entirely.
		var ok []Canon
		for _, a := range c.D.Options {
			if a.Kind() != ActCastFace {
				ok = append(ok, a)
			}
		}
		if len(ok) == 0 {
			return c.D.Options[0]
		}
		return ok[int((seed+uint64(c.Seq))%uint64(len(ok)))]
	})
}

// kdrSetup builds the corpus, the set and the two decks.
func kdrSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("dskrooms")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kdrDecks {
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
	return reg, set, decks
}

// kdrRun plays seeds 1..seeds with both seatings, gorge leading.
func kdrRun(t *testing.T, seeds uint64, mutate bool) (games, divs, stalls int, st kdrStats) {
	t.Helper()
	reg, set, decks := kdrSetup(t)
	old := dirac.DebugUnlockDoorOff
	dirac.DebugUnlockDoorOff = mutate
	defer func() { dirac.DebugUnlockDoorOff = old }()
	for swap := 0; swap < 2; swap++ {
		md := kdrDecks
		dd := decks
		if swap == 1 {
			md = [2][]cardset.Entry{kdrDecks[1], kdrDecks[0]}
			dd = [2][]*cards.Card{decks[1], decks[0]}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kdskrooms-a", "kdskrooms-b"}, dd, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			check := func(ref *GorgeSide, _ Side) *Divergence {
				kdrScan(ref, &seen, &st)
				return nil
			}
			res := LockstepLed(m, seed, g, k, kdrPolicy(seed, swap, &st), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 20, Check: check})
			CloseSide(k)
			games++
			if res.Stall != "" && res.Stall != "turns" {
				stalls++
				t.Logf("swap %d seed %d: stalled: %s", swap, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				divs++
				t.Logf("stackdbg len=%d ids=%v", len(g.E.G.Stack), g.E.G.Stack)
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("swap %d seed %d last actions %v", swap, seed, actionStrings(res.Actions[lo:d.At]))
				t.Logf("swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
					swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				if mutate {
					return games, divs, stalls, st
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestDskroomsLockstep(t *testing.T) {
	games, divs, stalls, st := kdrRun(t, 192, false) // 192 seeds x 2 seatings = 384 games
	t.Logf("games %d counts: %+v", games, st)
	if divs != 0 {
		t.Errorf("%d divergences", divs)
	}
	if stalls != 0 {
		t.Errorf("%d stalls", stalls)
	}
	for _, want := range []struct {
		name string
		got  int
		min  int
	}{
		{"DoorUnlock by an effect", st.effectUnlocks, 3},
		{"DoorLock", st.locks, 3},
		{"paid unlock at a reduced cost with Glimmer out", st.reducedUnlocks, 3},
		{"possession counters from Eerie", st.possession, 3},
	} {
		if want.got < want.min {
			t.Errorf("%s: %d, want at least %d", want.name, want.got, want.min)
		}
	}
}

func TestDskroomsMutation(t *testing.T) {
	_, divs, _, st := kdrRun(t, 64, true)
	t.Logf("mutation counts: %+v", st)
	if divs == 0 {
		t.Fatalf("DebugUnlockDoorOff did not diverge in 64 seeds")
	}
}
