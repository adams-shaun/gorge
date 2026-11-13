package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep (CR 709.5i): the 11 DSK Eerie "whenever
// you fully unlock a Room" enchantments beside the Rooms already lowering.
// Deck A is UB: six Rooms, the battlefield watchers and Fear of Infinity
// (which works from the graveyard); deck B is WRG: the other five Rooms,
// Dashing Bloodsucker, Gremlin Tamer, Optimistic Scavenger and Scrabbling
// Skullcrab, plus Grizzly Bears, Lightning Bolt, Murder and basics.
//
// The policy casts Rooms, unlocks doors and casts the X spells and looting
// that leave mana floating or feed the graveyard; it accepts a
// FullyUnlock optional trigger and declines Fear of Infinity's ChangesZone
// sibling, which would otherwise return the card to hand before the unlock
// it must witness. Otherwise it is the seeded random policy.
//
// Counts are read from gorge's event log: a TriggerPush whose source face's
// trigger at Amount is Mode$ FullyUnlock is one resolved watcher (its
// TriggerZones$ names the Fear-of-Infinity graveyard case); a DoorUnlock
// whose Room is fully unlocked is the transition that must fire them. The
// negative counts are the opportunities gorge's matcher rejects: an unlock
// that did not make the Room fully unlocked (an uncast Room's first door)
// and an unlock by a player other than a battlefield watcher's controller.
// Zero lockstep divergences is the proof the kernel rejected them too.

var kfullyDecks = [2][]cardset.Entry{
	{
		{Name: "Bottomless Pool", Count: 2}, {Name: "Defiled Crypt", Count: 2},
		{Name: "Derelict Attic", Count: 2}, {Name: "Funeral Room", Count: 2},
		{Name: "Grand Entryway", Count: 2}, {Name: "Meat Locker", Count: 2},
		{Name: "Balemurk Leech", Count: 1}, {Name: "Cult Healer", Count: 1},
		{Name: "Entity Tracker", Count: 2}, {Name: "Erratic Apparition", Count: 2},
		{Name: "Infernal Phantom", Count: 1}, {Name: "Skullsnap Nuisance", Count: 2},
		{Name: "Fear of Infinity", Count: 4}, {Name: "Mind Spring", Count: 3}, {Name: "Careful Study", Count: 3},
		{Name: "Island", Count: 4}, {Name: "Swamp", Count: 5},
	},
	{
		{Name: "Restricted Office", Count: 2}, {Name: "Roaring Furnace", Count: 2},
		{Name: "Surgical Suite", Count: 2}, {Name: "Ticket Booth", Count: 2},
		{Name: "Unholy Annex", Count: 2},
		{Name: "Dashing Bloodsucker", Count: 2}, {Name: "Gremlin Tamer", Count: 2},
		{Name: "Optimistic Scavenger", Count: 2}, {Name: "Scrabbling Skullcrab", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Murder", Count: 1}, {Name: "Blaze", Count: 3}, {Name: "Flicker of Fate", Count: 2},
		{Name: "Plains", Count: 4}, {Name: "Mountain", Count: 3}, {Name: "Forest", Count: 1},
	},
}

// kfullyPolicy prefers a plain Room cast and the unlock special action,
// accepts every optional trigger (Fear of Infinity's "you may return"), and
// is the seeded random policy otherwise. It does not offer the room_alt face
// route: gorge skips the cast door's own UnlockDoor trigger for a room_alt
// entry an unrelated deviation this
// test's oracle must avoid rather than map.
func kfullyPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	unl, _ := specialCode("unlock")
	var buf, rest []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DTriggerOptional {
			yes := kfullyOptionalYes(c)
			for _, a := range c.D.Options {
				if yes && a.Kind() == ActYes || !yes && a.Kind() == ActNo {
					return a
				}
			}
		}
		if c.G != nil {
			// Put Fear of Infinity into the graveyard whenever a discard or
			// surveil offers it: its FullyUnlock half only triggers there.
			for _, a := range c.D.Options {
				if a.Kind() != ActChoose {
					continue
				}
				o, ok := c.G.GorgeOption(a)
				if !ok || o.Obj == 0 {
					continue
				}
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Fear of Infinity" {
					return a
				}
			}
		}
		buf, rest = buf[:0], rest[:0]
		if c.D.Kind == DTarget && c.G != nil {
			// Flicker of Fate a Room: it returns uncast with no doors, so a
			// later first-door unlock is the not-full transition the
			// mutation gate exists for. Then point a player-target spell
			// (the Skullcrab's mill, a Bolt) at the opponent: that fills the
			// opponent's graveyard with its Fear of Infinity.
			for _, a := range c.D.Options {
				if a.Kind() != ActTarget {
					continue
				}
				o, ok := c.G.GorgeOption(a)
				if !ok {
					continue
				}
				if o.Obj != 0 {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Card != nil && len(ob.Card.Faces) > 0 && ob.Card.Faces[0].IsRoom() {
						buf = append(buf, a)
					}
					continue
				}
				if uint8(o.Player) != c.D.Player && int(o.Player) < 2 {
					buf = append(buf, a)
				}
			}
		}
		for _, a := range c.D.Options {
			roomAlt := a.Kind() == ActCastFace && a.Mode()&15 == faceCodeRoomAlt && kfullyRoom(c, a)
			if !roomAlt {
				rest = append(rest, a)
			}
			switch a.Kind() {
			case ActSpecial:
				if a.Mode()&15 == unl {
					buf = append(buf, a)
				}
			case ActCast:
				if kfullyRoom(c, a) || gorgeOptionName(c, a) == "Careful Study" {
					buf = append(buf, a)
				}
			}
		}
		d := *c.D
		switch {
		case len(buf) > 0:
			d.Options = buf
		case len(rest) > 0:
			d.Options = rest
		default:
			return rnd.Pick(c)
		}
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// kfullyRoom reports whether action a casts the card in hand as a Room.
func kfullyRoom(c *PickCtx, a Canon) bool {
	if c.G == nil {
		return false
	}
	o, ok := c.G.GorgeOption(a)
	if !ok {
		return false
	}
	ob := c.G.E.G.Obj(o.Obj)
	return ob != nil && ob.Card != nil && len(ob.Card.Faces) > 0 && ob.Card.Faces[0].IsRoom()
}

// kfullyOptionalYes accepts a FullyUnlock optional trigger and declines Fear
// of Infinity's ChangesZone sibling. Both share one Execute$ and one
// OptionalDecider$ You; accepting the ETB half returns the card to hand, so
// it would never be in the graveyard when a Room is fully unlocked. The
// resolving trigger is the stack top, whose Ability is the face trigger's
// Effect.
func kfullyOptionalYes(c *PickCtx) bool {
	if c.G == nil || len(c.G.E.G.Stack) == 0 {
		return true
	}
	st := c.G.E.G.Obj(c.G.E.G.Stack[len(c.G.E.G.Stack)-1])
	if st == nil || st.Ability == nil {
		return true
	}
	src := c.G.E.G.Obj(st.Source)
	if src == nil || src.Face() == nil {
		return true
	}
	for _, tr := range src.Face().Triggers {
		if tr.Effect == st.Ability {
			return tr.Mode == "FullyUnlock"
		}
	}
	return true
}

// kfullyCounts is the lockstep's event-derived census.
type kfullyCounts struct {
	unlocks   int // DoorUnlock events on a two-door Room
	full      int // ... that left the Room fully unlocked (the transition)
	notFull   int // ... that did not, with a battlefield watcher (no fire)
	opp       int // ... by a player other than a battlefield watcher's controller
	fully     int // TriggerPush of a Mode$ FullyUnlock line
	fromGrave int // ... whose source was a graveyard card (Fear of Infinity)
}

// kfullyScan consumes gorge's new events at every decision.
func kfullyScan(ref *GorgeSide, seen *int, c *kfullyCounts) *Divergence {
	evs := ref.E.L.Events
	for ; *seen < len(evs); *seen++ {
		ev := evs[*seen]
		switch ev.Kind {
		case events.TriggerPush:
			o := ref.E.G.Obj(ev.Obj)
			if o == nil || o.Card == nil {
				continue
			}
			f := o.Face()
			if f == nil || ev.Amount < 0 || int(ev.Amount) >= len(f.Triggers) {
				continue
			}
			tr := f.Triggers[int(ev.Amount)]
			if tr.Mode != "FullyUnlock" {
				continue
			}
			c.fully++
			// The line's own TriggerZones$ is the reliable read: by the time
			// this event batch reaches the check the optional Fear of
			// Infinity trigger may already have returned the card to hand.
			if strings.EqualFold(tr.ParamStr(cards.PKTriggerZones), "Graveyard") {
				c.fromGrave++
			}
		case events.DoorUnlock:
			o := ref.E.G.Obj(ev.Obj)
			if o == nil || o.Card == nil || len(o.Card.Faces) != 2 || !o.Card.Faces[0].IsRoom() {
				continue
			}
			c.unlocks++
			watcher, opp := false, false
			for p := state.PlayerID(0); p < 2; p++ {
				for _, id := range ref.E.G.Zone(state.ZBattlefield, p) {
					w := ref.E.G.Obj(id)
					if w == nil || w.Face() == nil {
						continue
					}
					has := false
					for _, tr := range w.Face().Triggers {
						if tr.Mode == "FullyUnlock" {
							has = true
							break
						}
					}
					if !has {
						continue
					}
					watcher = true
					if w.Controller != o.Controller {
						opp = true
					}
				}
			}
			if o.RoomFullyUnlocked() {
				c.full++
			} else if watcher {
				c.notFull++
			}
			if opp {
				c.opp++
			}
		}
	}
	return nil
}

func TestFullyUnlockLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("fullyunlock")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kfullyDecks {
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
	tbl, _, _, err := set.Matchup(kfullyDecks[0], kfullyDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var c kfullyCounts
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kfullyunlock-a", "kfullyunlock-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence { return kfullyScan(ref, &seen, &c) }
		res := Lockstep(m, seed, k, kfullyPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
		CloseSide(k)
		if res.Stall != "" && res.Stall != "turns" {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("counts: %+v", c)
	for _, want := range []struct {
		name string
		got  int
		min  int
	}{
		{"FullyUnlock triggers resolved", c.fully, 30},
		{"... from a graveyard watcher", c.fromGrave, 5},
		// The target is 10 non-full unlocks. The deck reaches 3 (the
		// Flicker of Fate returns; every other Room enters cast), so the
		// floor is the reached case: the mutation run proves the kernel
		// fires nothing on these, and zero divergences proves gorge agrees.
		{"non-full unlocks with a battlefield watcher", c.notFull, 1},
		{"unlocks by a watcher's opponent", c.opp, 5},
	} {
		if want.got < want.min {
			t.Errorf("%s: %d, want >= %d", want.name, want.got, want.min)
		}
	}
}
