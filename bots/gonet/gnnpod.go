package gonet

import "github.com/adams-shaun/gorge/bots/policynet"

// gnLayout is a graph network's fixed nodes: the players, the zones under
// them (or under the root), their node types, and the zone each card is a
// child of. gnLegacy is GraphNet's two-player graph (me, opponent, four
// zones); gnPod is the four-seat Commander pod graph (config player_nodes
// 4, the FeaturesPod encoding): one player node per relative seat, each
// with its own battlefield, graveyard, exile and command zone (the viewer
// also its hand), the stack under the root, a learned embedding per
// relative seat added to each player and its zones, and a player leaf
// Linear(PodSeatVector) carrying the seat's dense facts.
type gnLayout struct {
	players, zones     int
	rootType, cardType int
	playerType         []int // node type per player
	zoneType           []int // node type per zone
	zoneSeat           []int // seat embedding per zone, -1 none
	childZone, childOf []int // the player children: a zone and its player, in segment order
	stackZone          int   // the zone under the root
	seatEmb, seatLeaf  bool
	nodeTypes          int
	edgeTypes          int
	cardZone           func(c *policynet.EntityCard) int
}

func (l *gnLayout) fixed() int { return 1 + l.players + l.zones }

// gnEdgeSeatLeaf is the pod graph's edge type of a player's dense leaf.
const gnEdgeSeatLeaf = gnNumEdgeTypes

var gnLegacy = &gnLayout{
	players: 2, zones: 4, rootType: gnNodeRoot, cardType: gnNodeCard,
	playerType: []int{gnNodeMe, gnNodeOpp},
	zoneType:   []int{gnNodeMyBF, gnNodeOppBF, gnNodeHand, gnNodeStack},
	zoneSeat:   []int{-1, -1, -1, -1},
	childZone:  gnPlayerZones[:], childOf: gnPlayerOfZones[:],
	stackZone: 3, nodeTypes: gnNumNodeTypes, edgeTypes: gnNumEdgeTypes,
	cardZone: func(c *policynet.EntityCard) int {
		if g := int(c.Group); g < 4 {
			return g
		}
		return -1
	},
}

// The pod graph's node types (node_type.weight's rows) and zones: zone
// 4r+podZ* is relative seat r's, then the hand and the stack.
const (
	podNodeRoot = iota
	podNodePlayer
	podNodeBF
	podNodeGrave
	podNodeExile
	podNodeCommand
	podNodeHand
	podNodeStack
	podNodeCard
	podNumNodeTypes

	podZBF, podZGrave, podZExile, podZCommand = 0, 1, 2, 3
	podSeats                                  = 4
	podZoneHand                               = 4 * podSeats
	podZoneStack                              = podZoneHand + 1
)

var gnPod = func() *gnLayout {
	l := &gnLayout{players: podSeats, zones: podZoneStack + 1, rootType: podNodeRoot, cardType: podNodeCard,
		stackZone: podZoneStack, seatEmb: true, seatLeaf: true, nodeTypes: podNumNodeTypes, edgeTypes: gnNumEdgeTypes + 1}
	for r := 0; r < podSeats; r++ {
		l.playerType = append(l.playerType, podNodePlayer)
		for _, t := range []int{podNodeBF, podNodeGrave, podNodeExile, podNodeCommand} {
			l.zoneType = append(l.zoneType, t)
			l.zoneSeat = append(l.zoneSeat, r)
			l.childZone = append(l.childZone, len(l.zoneType)-1)
			l.childOf = append(l.childOf, r)
		}
		if r == 0 {
			l.childZone = append(l.childZone, podZoneHand)
			l.childOf = append(l.childOf, 0)
		}
	}
	l.zoneType = append(l.zoneType, podNodeHand, podNodeStack)
	l.zoneSeat = append(l.zoneSeat, 0, -1)
	l.cardZone = func(c *policynet.EntityCard) int {
		switch g := int(c.Group); {
		case g < podSeats:
			return 4*g + podZBF
		case g == policynet.PodGroupHand:
			return podZoneHand
		case g == policynet.PodGroupStack:
			return podZoneStack
		case g == policynet.PodGroupCommand:
			return 4*max(policynet.PodCardOwner(c), 0) + podZCommand
		}
		return -1
	}
	return l
}()
