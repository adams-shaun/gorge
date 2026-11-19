package gonettest

import (
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// PodView is the four-seat Commander position the pod shape tests encode
// (a copy of bots/policynet's podView), from seat 0's view: seats 1 and
// 3 alive, seat 2 lost, seat 3 the monarch, seat 1 with the initiative and
// three poison counters, one of seat 0's creatures attacking seat 3 and
// goaded by seat 1, seat 3 with a partner pair (one in its command zone, one
// on the battlefield), and seat 0 dealt 6 and 9 combat damage by the pair.
func PodView() view.View {
	v := view.View{Turn: 9, Step: "declare-blockers", Phase: "combat", Active: 0, Priority: 0, DayNight: "night"}
	for p := 0; p < 4; p++ {
		id := state.ObjID(100 + p)
		v.Players = append(v.Players, view.PlayerView{
			ID: state.PlayerID(p), Life: 40 - int32(p),
			Commanders:     []view.CardView{{ID: id, Name: "Commander", Owner: state.PlayerID(p), Controller: state.PlayerID(p)}},
			CommanderCasts: []int32{0},
		})
		v.Players[p].Command = []view.CardView{{ID: id, Name: "Commander", Owner: state.PlayerID(p), Controller: state.PlayerID(p), Types: "Legendary Creature"}}
	}
	atk := state.PlayerID(3)
	v.Players[0].Battlefield = []view.CardView{
		{ID: 10, Name: "Bear", Types: "Creature", Power: 2, Toughness: 2, Attacking: true, AttackingPlayer: &atk,
			GoadedBy: []state.PlayerID{1}, Controller: 0, Owner: 0},
		{ID: 11, Name: "Forest", Types: "Basic Land", Controller: 0, Owner: 0},
	}
	v.Players[0].Hand = []view.CardView{{ID: 12, Name: "Bear", Types: "Creature", ManaCost: "1 G", Controller: 0, Owner: 0}}
	v.Players[0].HandSize = 1
	v.Players[1].HasInitiative = true
	v.Players[1].Counters = map[string]int32{"POISON": 3}
	v.Players[1].Battlefield = []view.CardView{{ID: 20, Name: "Wall", Types: "Creature", Power: 0, Toughness: 4, Controller: 1, Owner: 1}}
	v.Players[2].Lost = true
	v.Players[3].IsMonarch = true
	v.Players[3].Commanders = []view.CardView{
		{ID: 103, Name: "Commander A", Owner: 3, Controller: 3},
		{ID: 113, Name: "Commander B", Owner: 3, Controller: 3},
	}
	v.Players[3].CommanderCasts = []int32{0, 1}
	v.Players[3].Command = []view.CardView{{ID: 103, Name: "Commander A", Owner: 3, Controller: 3, Types: "Legendary Creature"}}
	v.Players[3].Battlefield = []view.CardView{{ID: 113, Name: "Commander B", Types: "Legendary Creature", Power: 3, Toughness: 3, Owner: 3, Controller: 3}}
	v.Players[0].CmdDamage = map[state.ObjID]int32{103: 6, 113: 9}
	return v
}
