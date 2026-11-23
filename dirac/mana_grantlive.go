package dirac

// A sacrifice-another source's step (Ashnod's Altar, mana_grantsacx.go) or a
// granted sacrifice step of the holder itself (Rain of Filth's "Sacrifice this
// land: Add {B}" on The World Tree, mana_grantcost.go sacBranch) can
// sacrifice the permanent that grants a candidate its mana abilities (Dune
// Chanter's "Lands you control have '{T}: Add one mana of any color'"). gorge's
// prefix search keeps the census gaps it chose at the root (the granted lands
// among them) but asks each source's abilities afresh at every node
// (rules scriptSearch.visit -> the "choose a mana ability" ask), so a land
// whose grant died with its holder offers only its printed colours below that
// step: a Plains W, a Crumbling Necropolis U/B/R, never the granted any colour.
// The grant search read the root's grant lists below it, branching on mana
// no ask offers, which spent its node budget (potentialScriptNodes) before
// the line gorge finds (the Altar, then the two lands' printed mana, Spiked
// Ripsaw's equip {3}) was reached.

// grantSave holds the candidates' root grant lists a node replaced
// (refreshGrants), to put back when the node's subtree is done: the lists are
// the search's shared state, read by every shallower node.
type grantSave struct {
	n  int
	at [MaxPerm]uint8
	ng [MaxPerm]uint8
	gi [MaxPerm][maxGrants]int16
}

// refreshGrants re-reads the granted abilities of each grant holder among the
// candidates on node state u, and replaces a candidate's list where it
// changed (a holder of the grant left the battlefield). A candidate that is
// no longer the permanent the root listed keeps its list: visit skips it
// (mana_grantcost.go).
func (g *grantSrch) refreshGrants(u *State) (sv grantSave) {
	for i := 0; i < g.n; i++ {
		sl := g.cand[i]
		if g.ng[i] == 0 || !u.occ.has(sl) || u.perms[sl].obj != g.cobj[i] {
			continue
		}
		var fresh [maxGrants]int16
		n := u.grantIdxs(g.k, sl, &fresh)
		if n == int(g.ng[i]) && fresh == g.gi[i] {
			continue
		}
		j := sv.n
		sv.at[j], sv.ng[j], sv.gi[j] = uint8(i), g.ng[i], g.gi[i]
		sv.n++
		g.ng[i], g.gi[i] = uint8(n), fresh
	}
	return sv
}

// restoreGrants puts back the lists refreshGrants replaced.
func (g *grantSrch) restoreGrants(sv *grantSave) {
	for j := 0; j < sv.n; j++ {
		i := sv.at[j]
		g.ng[i], g.gi[i] = sv.ng[j], sv.gi[j]
	}
}
