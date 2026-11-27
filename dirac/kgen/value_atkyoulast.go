package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

func init() {
	exprKinds[vAtkYouLast] = kop.ExAtkYouLastTurn
	opsEntries = append(opsEntries,
		opsDoc{"ex:AtkYouLast", "ExAtkYouLastTurn",
			"The controller's living opponents that attacked the controller during their own most recent completed turn (gorge `PlayerCountPlayers$HasPropertyattackedYouTheirLastTurn`, count_cov3.go cov3PlayerScalarHasPropertyAttackedYouTheirLastTurn over Engine.AttackedDuringLastTurn; the controller never counts, so in two players 0 or 1). Reads the per-seat attack record of dirac/trig_atklastturn.go (bits kept per defending seat, a planeswalker or battle of the controller's included). Avenge's ReduceCost `CheckSVar$ X`. dirac/value_atkyoulast.go."})
}
