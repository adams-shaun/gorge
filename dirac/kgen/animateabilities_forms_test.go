package kgen

import "testing"

// animateAbilitiesForms: an Animate's Abilities$ on the source itself (the
// manlands) is a latent ability the end-of-turn override record grants
// (lower_zone.go animateSelfAbilities, the self-granted record the kernel
// hosts from its own card). A grant to one other permanent lowers the same
// record, which keeps its maker's object (Retraction Helix); Martyrdom still
// refuses, on its damage-redirect Effect rather than on the grant.
var animateAbilitiesForms = []formCase{
	{card: "Spawning Pool", form: "AB$ Animate Defined$ Self Abilities$ ABRegen", lir: "Override obj=Self CS=EOT // override{pt=1/1 addtypes=2 addsub=Skeleton colors=B grant=2}"},
	{card: "Lavaclaw Reaches", form: "AB$ Animate Defined$ Self Abilities$ ABPump ({X}: +X/+0)", lir: "Override obj=Self CS=EOT // override{pt=2/2 addtypes=2 addsub=Elemental colors=BR grant=2}"},
	{card: "Retraction Helix", form: "SP$ Animate ValidTgts$ Creature Abilities$ (another object)", lir: "Override obj=Target"},
	{card: "Martyrdom", form: "SP$ Animate ValidTgts$ Creature.YouCtrl Abilities$ (another object)", refused: "Effect DamageDone redirect"},
}

func TestAnimateAbilitiesDurationForms(t *testing.T) { checkForms(t, animateAbilitiesForms) }
