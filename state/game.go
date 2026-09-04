package state

import "github.com/adams-shaun/gorge/cards"

type Player struct {
	ID          PlayerID
	Name        string
	Life        int32
	Lost        bool
	LandsPlayed int32
	Pool        Mana
}

// Game is the complete authoritative state. Everything a client sees is a
// projection of this. Only the events package may mutate it.
type Game struct {
	Players []Player
	// Objs is a dense arena: Objs[i] has ID i+1, so ObjID 0 is "no object".
	Objs     []Object
	Stack    []ObjID
	Turn     int32
	Active   PlayerID
	Priority PlayerID
	Step     Step
	Passes   int32
	Over     bool
	Winner   PlayerID
	// Draw marks a game that ended with no surviving seats (CR 104.4a).
	// Winner's zero value is PlayerID(0), a real seat, so Over alone cannot
	// distinguish "seat 0 won" from "nobody did" -- Draw is what does.
	Draw   bool
	NextID ObjID
	// Clock is a monotonic timestamp source for continuous-effect ordering.
	Clock uint32

	// zones is indexed by zoneIndex(z, p); the stack lives in Stack instead.
	zones [][]ObjID
}

const startingLife = 20

func NewGame(names []string) *Game {
	g := &Game{NextID: 1, zones: make([][]ObjID, numZones*len(names))}
	for i, n := range names {
		g.Players = append(g.Players, Player{ID: PlayerID(i), Name: n, Life: startingLife})
	}
	return g
}

func (g *Game) zoneIndex(z Zone, p PlayerID) int { return int(p)*numZones + int(z) }

func (g *Game) Zone(z Zone, p PlayerID) []ObjID {
	if z == ZStack {
		return g.Stack
	}
	return g.zones[g.zoneIndex(z, p)]
}

func (g *Game) SetZone(z Zone, p PlayerID, ids []ObjID) {
	if z == ZStack {
		g.Stack = ids
		return
	}
	g.zones[g.zoneIndex(z, p)] = ids
}

// Obj returns the object with this ID, or nil for ObjID 0 and out-of-range IDs.
func (g *Game) Obj(id ObjID) *Object {
	if id == 0 || int(id) > len(g.Objs) {
		return nil
	}
	return &g.Objs[id-1]
}

func (g *Game) AddObject(card *cards.Card, owner PlayerID) *Object {
	o := Object{ID: g.NextID, Card: card, Owner: owner, Controller: owner, Zone: ZLibrary}
	g.NextID++
	g.Objs = append(g.Objs, o)
	return &g.Objs[len(g.Objs)-1]
}

// Clone deep-copies the game. Everything is slices of value types, so this is
// a handful of copy() calls rather than a graph walk.
func (g *Game) Clone() *Game {
	c := *g
	c.Players = append([]Player(nil), g.Players...)
	c.Objs = append([]Object(nil), g.Objs...)
	for i := range c.Objs {
		if n := g.Objs[i].Counters; n != nil {
			c.Objs[i].Counters = append([]Counter(nil), n...)
		}
		if n := g.Objs[i].Targets; n != nil {
			c.Objs[i].Targets = append([]Target(nil), n...)
		}
		if n := g.Objs[i].Remembered; n != nil {
			c.Objs[i].Remembered = append([]Target(nil), n...)
		}
		if n := g.Objs[i].BlockedBy; n != nil {
			c.Objs[i].BlockedBy = append([]ObjID(nil), n...)
		}
	}
	c.Stack = append([]ObjID(nil), g.Stack...)
	c.zones = make([][]ObjID, len(g.zones))
	for i, z := range g.zones {
		if z != nil {
			c.zones[i] = append([]ObjID(nil), z...)
		}
	}
	return &c
}

// AliveFrom lists surviving seats in APNAP order starting at start.
func (g *Game) AliveFrom(start PlayerID) []PlayerID {
	n := PlayerID(len(g.Players))
	out := make([]PlayerID, 0, n)
	for i := PlayerID(0); i < n; i++ {
		p := (start + i) % n
		if !g.Players[p].Lost {
			out = append(out, p)
		}
	}
	return out
}

func (g *Game) AliveCount() int { return len(g.AliveFrom(0)) }

// NextAlive returns the next surviving seat after p, or p itself if none is.
func (g *Game) NextAlive(p PlayerID) PlayerID {
	n := PlayerID(len(g.Players))
	for i := PlayerID(1); i <= n; i++ {
		q := (p + i) % n
		if !g.Players[q].Lost {
			return q
		}
	}
	return p
}
