package kgen

func init() {
	opsEntries = append(opsEntries, opsDoc{"card:Enlist", "CardDef.Enlist",
		"A printed K:Enlist (CR 702.154; kop.KwEnlist; gorge engine/rules/enlist.go): once the attack declaration is answered and its attack tax paid, each declared attacker printing Enlist (not under a lose-all-abilities effect) that has an eligible creature asks its controller, in declaration order (ctlEnlist, ChEnlist: the decline first, then each untapped, nonattacking creature they control without summoning sickness in battlefield order); a yes taps that creature (its Taps triggers fire with the tap) and gives the attacker +X/+0 until end of turn, X the enlisted creature's power then. The elections come before the declaration's own taps and Attacks triggers. A GRANTED Enlist stays refused (the kernel reads only the printed field), and so do Mode$ Enlisted and enlistedThisCombat. Coalition Warbrute, Keldon Flamesage. dirac/kw_enlist.go."})
}
