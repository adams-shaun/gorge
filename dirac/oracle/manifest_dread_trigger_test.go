package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The manifest-dread trigger-and-owner lockstep: Paranormal Analyst's Mode$
// ManifestDread trigger with its ChooseFromDefined$ TriggeredCards hidden
// graveyard pick ("whenever you manifest dread, put a card you put into your
// graveyard this way into your hand"), and Unidentified Hovership's
// remembered-persist pair — its enters trigger exiles a creature
// (RememberChanged$) and its leave trigger manifests dread for the
// remembered card's owner (DefinedPlayer$ RememberedOwner). The decks are
// over the manifest pool (dirac/cardset/pools/fixtures/manifest), which both
// cards joined.
//
// Matchup A: both seats play Paranormal Analyst and Manifest Dread beside
// moxen; the policy casts the Analyst and dread on sight, answers the hidden
// pick with the milled card, and the tally counts a graveyard-shrink beside a
// hand-grow under an Analyst (the only grave-to-hand mover in the decks).
//
// Matchup B: seat 0 plays the Hovership beside a crew of Grizzly Bears; seat
// 1 plays Reality Shift, the exile spell that turns the crewed vehicle into
// a creature target (ValidTgts$ Creature) and moves it out. The policy crews
// on sight, has seat 0's enters-trigger exile a seat-1 Innocuous Rat (so the
// remembered card's owner is seat 1) and has seat 1 exile the Hovership, so
// the leave trigger manifests dread for seat 1: the only face-down card seat
// 1 can ever get (it has no manifest source of its own).
var kdreadDecks = [][2][]cardset.Entry{{
	{
		{Name: "Paranormal Analyst", Count: 2}, {Name: "Manifest Dread", Count: 6},
		{Name: "Mox Sapphire", Count: 3}, {Name: "Mox Emerald", Count: 3},
		{Name: "Mox Pearl", Count: 3}, {Name: "Mox Jet", Count: 2},
		{Name: "Mind Stone", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Innocuous Rat", Count: 4}, {Name: "Ornithopter", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Zoetic Cavern", Count: 2},
		{Name: "Dryad Arbor", Count: 1},
	},
	{
		{Name: "Paranormal Analyst", Count: 2}, {Name: "Manifest Dread", Count: 6},
		{Name: "Mox Sapphire", Count: 3}, {Name: "Mox Emerald", Count: 3},
		{Name: "Mox Pearl", Count: 3}, {Name: "Mox Jet", Count: 2},
		{Name: "Mind Stone", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Innocuous Rat", Count: 4}, {Name: "Ornithopter", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Zoetic Cavern", Count: 2},
		{Name: "Dryad Arbor", Count: 1},
	},
}, {
	{
		{Name: "Unidentified Hovership", Count: 2}, {Name: "Mox Pearl", Count: 4},
		{Name: "Mox Jet", Count: 4}, {Name: "Mox Emerald", Count: 5},
		{Name: "Mox Sapphire", Count: 5}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Hill Giant", Count: 8}, {Name: "Mind Stone", Count: 4},
	},
	{
		{Name: "Reality Shift", Count: 4}, {Name: "Mox Sapphire", Count: 4},
		{Name: "Mox Emerald", Count: 4}, {Name: "Mox Pearl", Count: 4},
		{Name: "Mox Jet", Count: 4}, {Name: "Mind Stone", Count: 4},
		{Name: "Innocuous Rat", Count: 8}, {Name: "Ornithopter", Count: 8},
	},
}}

type kdreadCounts struct {
	returned       int // Analyst-trigger graveyard-to-hand returns
	dreads         int // face-down rises, either seat (matchup A)
	dread0, dread1 int // face-down rises per seat (matchup B)
}

// kdreadZoneLen is the gorge length of a player's zone.
func kdreadZoneLen(c *PickCtx, z state.Zone, p int) int {
	if c.G == nil {
		return 0
	}
	return len(c.G.E.G.Zone(z, state.PlayerID(p)))
}

// kdreadDownSeat counts the face-down permanents in one seat's battlefield.
func kdreadDownSeat(c *PickCtx, p int) int {
	if c.G == nil {
		return 0
	}
	n := 0
	for _, id := range c.G.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
		if o := c.G.E.G.Obj(id); o != nil && o.FaceDown {
			n++
		}
	}
	return n
}

// kdreadHas reports whether seat p controls a battlefield object whose face
// name is name.
func kdreadHas(c *PickCtx, p int, name string) bool {
	if c.G == nil {
		return false
	}
	for _, id := range c.G.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
		if o := c.G.E.G.Obj(id); o != nil && !o.FaceDown && o.Face() != nil && o.Face().Name == name {
			return true
		}
	}
	return false
}

// kdreadGraveNames are the printed names of every card in either graveyard.
func kdreadGraveNames(c *PickCtx) map[string]bool {
	m := map[string]bool{}
	if c.G == nil {
		return m
	}
	for p := 0; p < 2; p++ {
		for _, id := range c.G.E.G.Zone(state.ZGraveyard, state.PlayerID(p)) {
			if o := c.G.E.G.Obj(id); o != nil && o.Face() != nil {
				m[o.Face().Name] = true
			}
		}
	}
	return m
}

// kdreadAnalysts reports whether each seat controls a Paranormal Analyst.
func kdreadAnalysts(c *PickCtx) [2]bool {
	return [2]bool{kdreadHas(c, 0, "Paranormal Analyst"), kdreadHas(c, 1, "Paranormal Analyst")}
}

// kdreadPolicyA: moxen, then the Analyst and dread casts, then a turn-up,
// else random; a pick (the hidden TriggeredCards ask, its options naming the
// milled card in a graveyard) wins over everything. Each call adds the
// graveyard-shrink-beside-hand-grow under an Analyst to counts.returned and
// the face-down rise to counts.dreads.
func kdreadPolicyA(seed uint64, counts *kdreadCounts) Policy {
	rnd := RandomPolicy(seed)
	var priorG, priorH, priorDown [2]int
	return PolicyFunc(func(c *PickCtx) Canon {
		aa := kdreadAnalysts(c)
		a0, a1 := aa[0], aa[1]
		for s := 0; s < 2; s++ {
			g, h := kdreadZoneLen(c, state.ZGraveyard, s), kdreadZoneLen(c, state.ZHand, s)
			if g < priorG[s] && h > priorH[s] && (s == 0 && a0 || s == 1 && a1) {
				counts.returned++
			}
			priorG[s], priorH[s] = g, h
		}
		for s := 0; s < 2; s++ {
			if n := kdreadDownSeat(c, s); n > priorDown[s] {
				counts.dreads += n - priorDown[s]
			}
			priorDown[s] = kdreadDownSeat(c, s)
		}
		graves := kdreadGraveNames(c)
		var mox, analyst, dread, up Canon
		haveMox, haveAnalyst, haveDread, haveUp, havePick := false, false, false, false, false
		for _, a := range c.D.Options {
			n := optionFaceName(c, a)
			switch {
			case a.Kind() == ActChoose && graves[n]:
				return a // the milled card: the mandatory hidden pick
			case a.Kind() == ActCast && isMoxName(n):
				mox, haveMox = a, true
			case a.Kind() == ActCast && n == "Paranormal Analyst":
				analyst, haveAnalyst = a, true
			case a.Kind() == ActCast && n == "Manifest Dread":
				dread, haveDread = a, true
			case a.Kind() == ActSpecial:
				up, haveUp = a, true
			}
		}
		switch {
		case haveMox:
			return mox
		case haveAnalyst:
			return analyst
		case haveDread:
			return dread
		case haveUp:
			return up
		}
		_ = havePick
		return rnd.Pick(c)
	})
}

// kdreadPolicyB drives the Hovership matchup. Seat 0 casts the ship and
// crews it with a Grizzly Bear; seat 1 holds Reality Shift until a
// Hovership is on seat 0's battlefield and then exiles it. Seat 0's
// enters-trigger ask exiles a seat-1 Innocuous Rat when one is on the
// battlefield (making the remembered card's owner seat 1); seat 1's
// Reality Shift ask takes the Hovership. Face-down rises are tallied per
// seat: seat 1 has no manifest source of its own, so its rise is the
// leave trigger's manifest dread.
func kdreadPolicyB(seed uint64, counts *kdreadCounts) Policy {
	rnd := RandomPolicy(seed)
	var priorDown [2]int
	var mox, ship, bear, giant, shift, chump, act, chooseBear, tgtRat, tgtShip, up Canon
	var haveMox, haveShip, haveBear, haveGiant, haveShift, haveChump, haveAct, haveChooseBear, haveTgtRat, haveTgtShip, haveUp bool
	return PolicyFunc(func(c *PickCtx) Canon {
		seat := 0
		if c.D != nil {
			seat = int(c.D.Player)
		}
		for s := 0; s < 2; s++ {
			if n := kdreadDownSeat(c, s); n > priorDown[s] {
				if s == 0 {
					counts.dread0 += n - priorDown[s]
				} else {
					counts.dread1 += n - priorDown[s]
				}
			}
			priorDown[s] = kdreadDownSeat(c, s)
		}
		haveMox, haveShip, haveBear, haveGiant, haveShift, haveChump, haveAct, haveChooseBear, haveTgtRat, haveTgtShip, haveUp = false, false, false, false, false, false, false, false, false, false, false
		// the crew pick: an ActChoose naming a battlefield Grizzly Bears
		// while seat 0 controls the Hovership
		for _, a := range c.D.Options {
			n := optionFaceName(c, a)
			k := a.Kind()
			switch {
			case k == ActChoose && n == "Grizzly Bears" && kdreadHas(c, 0, "Grizzly Bears"):
				chooseBear, haveChooseBear = a, true
			case k == ActCast && isMoxName(n):
				mox, haveMox = a, true
			case k == ActCast && n == "Unidentified Hovership":
				ship, haveShip = a, true
			case k == ActCast && n == "Reality Shift":
				shift, haveShift = a, true
			case k == ActCast && n == "Grizzly Bears":
				bear, haveBear = a, true
			case k == ActCast && n == "Hill Giant":
				giant, haveGiant = a, true
			case k == ActCast && (n == "Innocuous Rat" || n == "Ornithopter"):
				chump, haveChump = a, true
			case k == ActActivate && n == "Unidentified Hovership":
				act, haveAct = a, true
			case k == ActSpecial:
				up, haveUp = a, true
			}
		}
		shipGate := kdreadHas(c, 0, "Unidentified Hovership")
		if c.D.Kind == DTarget {
			for _, a := range c.D.Options {
				n := optionFaceName(c, a)
				switch {
				case n == "Innocuous Rat" && seat == 0:
					tgtRat, haveTgtRat = a, true
				case n == "Unidentified Hovership" && seat == 1:
					tgtShip, haveTgtShip = a, true
				}
			}
		}
		if haveChooseBear {
			return chooseBear
		}
		if haveAct {
			return act
		}
		if haveTgtRat {
			return tgtRat
		}
		if haveTgtShip {
			return tgtShip
		}
		// an unpreferred optional target ask declines (the first option)
		if c.D.Kind == DTarget {
			return c.D.Options[0]
		}
		// seat 1 holds Reality Shift until a Hovership is out (and never
		// targets one of its own creatures, whose manifest would muddy the
		// seat-1 tally): it passes until the ship is there
		if haveShift && !shipGate {
			for _, a := range c.D.Options {
				if a.Kind() == ActPass {
					return a
				}
			}
		}
		if haveShift && shipGate {
			return shift
		}
		if haveMox {
			return mox
		}
		if haveShip {
			return ship
		}
		if haveBear {
			return bear
		}
		if haveGiant {
			return giant
		}
		if haveChump {
			return chump
		}
		if haveUp {
			return up
		}
		return rnd.Pick(c)
	})
}

func TestManifestDreadTriggerOwnerLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("manifest")
	if err != nil {
		t.Fatal(err)
	}
	var counts kdreadCounts
	games := 0
	for mi, md := range kdreadDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 256; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"mdread-a", "mdread-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			var pol Policy
			if mi == 0 {
				pol = kdreadPolicyA(seed, &counts)
			} else {
				pol = kdreadPolicyB(seed, &counts)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, Analyst returns %d, dreads %d (seat0 %d, seat1 %d)",
		games, counts.returned, counts.dreads, counts.dread0, counts.dread1)
	if counts.returned < 5 {
		t.Errorf("only %d Analyst returns", counts.returned)
	}
	if counts.dreads < 10 {
		t.Errorf("only %d manifest dreads (matchup A)", counts.dreads)
	}
	if counts.dread1 < 2 {
		t.Errorf("only %d face-down entries for seat 1 (matchup B): the leave trigger's manifest dread for the remembered card's owner never ran", counts.dread1)
	}
	if counts.dread0 < 10 {
		t.Errorf("only %d face-down entries for seat 0 (matchup B: Reality Shift's manifests)", counts.dread0)
	}
}
