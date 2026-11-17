package kgen

import "testing"

// selfGraveReplForms: the self "if this would be put into a graveyard, exile
// it / put it into its owner's library instead" shapes from
// lower_repl.go: a self exile from the battlefield (Possessed Skaab,
// Gloomshrieker) and a self library move to the top (Gravebane Zombie) or
// bottom (Nissa's Chosen). Saiba Syphoner is the shuffle form.
var selfGraveReplForms = []formCase{
	{card: "Possessed Skaab", form: "self exile from the battlefield", lir: "repl ExileOnLeave"},
	{card: "Gloomshrieker", form: "self exile from the battlefield", lir: "repl ExileOnLeave"},
	{card: "Gravebane Zombie", form: "self library top, no shuffle", lir: "lib bottom=false shuffle=false fromBF=true"},
	{card: "Nissa's Chosen", form: "self library bottom, no shuffle", lir: "lib bottom=true shuffle=false fromBF=true"},
	{card: "Saiba Syphoner", form: "self library shuffle", lir: "lib bottom=false shuffle=true fromBF=true"},
}

func TestSelfGraveReplForms(t *testing.T) { checkForms(t, selfGraveReplForms) }
