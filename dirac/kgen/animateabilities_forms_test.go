package kgen

import "testing"

// animateAbilitiesForms: an Animate's Abilities$ on the source itself (the
// manlands) is a latent ability the end-of-turn override record grants
// (lower_zone.go animateSelfAbilities, the self-granted record the kernel
// hosts from its own card); a grant to another object keeps refusing.
var animateAbilitiesForms = []formCase{
	{card: "Spawning Pool", form: "AB$ Animate Defined$ Self Abilities$ ABRegen", lir: "Override obj=Self CS=EOT // override{pt=1/1 addtypes=2 addsub=Skeleton colors=B grant=2}"},
	{card: "Lavaclaw Reaches", form: "AB$ Animate Defined$ Self Abilities$ ABPump ({X}: +X/+0)", lir: "Override obj=Self CS=EOT // override{pt=2/2 addtypes=2 addsub=Elemental colors=BR grant=2}"},
	{card: "Retraction Helix", form: "SP$ Animate ValidTgts$ Creature Abilities$ (another object)", refused: "Animate Abilities$ granted to an object other than the source"},
	{card: "Martyrdom", form: "SP$ Animate ValidTgts$ Creature.YouCtrl Abilities$ (another object)", refused: "Animate Abilities$ granted to an object other than the source"},
}

func TestAnimateAbilitiesDurationForms(t *testing.T) { checkForms(t, animateAbilitiesForms) }
