package oracle

import "testing"

// Mortarion, Daemon Primarch's end-step "you may pay {X}" (Cost$ X,
// XMax$ LifeLost, SVar:LifeLost:Count$LifeYouLostThisTurn): the X announcement
// both engines posed at the end step of the turn Mortarion entered offered
// different bounds -- gorge 0..1, the kernel 0..2 -- with p1's life-lost
// tally and mana supply both in play (dr-mbc pair tail-57, kfuzz -synth
// tail-57, pair 187, seed 5222400260449195706).
func TestMBCMortarionDaemonPrimarchReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trig_xmax_life_lost")
}
