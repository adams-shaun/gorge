package kgen

func init() {
	opsEntries = append(opsEntries, opsDoc{"ext:Venture", "OpExt ExtVenture (Obj the player)",
		"api Venture (CR 701.49; gorge effVenture: every corpus line is Defined$ You or none). With no dungeon the player picks one (a plain venture offers every dungeon script but Undercity, in token-key order: the pool's dungeon cards sorted by key, the \"dungeon\" KChoose ChDungeon, mode the candidate's index; a single candidate is entered without asking) and its object enters the command zone with the marker on the top room; else the marker follows the room's one arrow or the player picks between two (ChRoom); from the bottommost room the dungeon is completed and the player chooses a new one (CR 701.49c). The room entered queues its latent ability as a trigger the dungeon's owner controls (CR 309.4c), read off the card of the dungeon object. The carrier names the three dungeon scripts and their rooms' tokens. Dungeon$ and a Defined$ other than You refuse. Dungeon Descent, Acererak the Archlich. dirac/act_venture.go."})
}
