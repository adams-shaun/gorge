package kgen

import "testing"

// keywordformsForms: the keyword classes of greedy classes 63-153,
// checked as staticConditionForms are.
var keywordformsForms = []formCase{
	{card: "Blinding Drone", form: "K:Devoid: the face is colourless ({1U} cost)", lir: `colors=""`},
	{card: "Algae Gharial", form: "K:Shroud (KwShroud)", lir: "kw=[Shroud]"},
	{card: "Cateran Enforcer", form: "K:Fear (KwFear)", lir: "Fear"},
	{card: "Dauthi Marauder", form: "K:Shadow (KwShadow)", lir: "kw=[Shadow]"},
	{card: "Blighted Agent", form: "K:Infect (CardDef.Infect)", lir: " infect"},
	{card: "Bilious Skulldweller", form: "K:Toxic:1 (CardDef.Toxic)", lir: "toxic=1"},
	{card: "Akrasan Squire", form: "K:Exalted: the expansion's Attacks trigger with Alone$ True (Tf2Alone)", lir: "trigger Attacks"},
}

func TestPrintedKeywordForms(t *testing.T) { checkForms(t, keywordformsForms) }
