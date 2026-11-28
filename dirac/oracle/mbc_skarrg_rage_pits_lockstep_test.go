package oracle

import "testing"

const skarrgRagePitsRepro = "skarrg_rage_pits_pump_offer"

// CR 602.2b, 605.3a: Skarrg, the Rage Pits' "{R}{G}, {T}: pump" is payable here
// only through Chromatic Star ("{1}, {T}, Sacrifice: Add one mana of any
// color"), the board's one green source, with a Mountain funding the Star's
// {1}. Gorge's planner prices no filter from its census, so its prefix search
// runs for the activation. The search tries the Star's red answer first, a dead
// line, and spends its whole potentialScriptNodes budget on that line's draw
// triggers (two Tolarian Krakens, each asking whether to pay {1} and for a
// target) before it reaches green: the play is "unsupported census_incomplete"
// and withheld, while the kernel's mirror walks a far smaller trigger tree,
// reaches green at node 55 and offers it (tail-62-c000 pair 9, decision 653,
// "options options"). That was the divergence the card was refused for. At the
// recorded decision both engines now offer Skarrg's activation, and the
// recorded game replays clean on a table of its own cards, so the card is
// unrefused with this game as its proof (dirac/support/unrefused.tsv).
func TestMBCSkarrgTheRagePitsReprosLockstep(t *testing.T) {
	t.Parallel()
	oneOffReproClean(t, skarrgRagePitsRepro)
}
