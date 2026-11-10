package dirac

// The generated table's per-card flags and sparse small values
// genPrep used to carry eleven []bool arrays, five []int8
// arrays and two []int16 arrays, one entry per card, most of them the -1
// sentinel. One uint32 per card now holds the flags and the presence bits
// of the small values; the values themselves live in smallRecs, in card
// order, one record per card that carries any. Readers are pure loads (a
// bit test, then at most a binary search over a few records), so a walk
// still allocates nothing.

const (
	gfEnterRepl   uint32 = 1 << iota // ReplEntersProgram on the card
	gfOthersRep                      // ReplOthersEnterProgram on the card
	gfLifeRepl                       // ReplCantLoseGame / ReplOppsCantWinGame (reserved; no reader today)
	gfCDAPT                          // StatCDAPT
	gfOvrStat                        // a StatOverride static
	gfLoseStat                       // an override static that removes abilities (Witness Protection)
	gfGlobOvr                        // an override static not limited to the source's host (Maha, Ygra)
	gfSubGrant                       // a Continuous subtype grant
	gfCtrlStat                       // a StatControl static (static_control.go)
	gfContGrant                      // a Continuous grant of a mana ability not limited to its host
	gfDivCard                        // a target slot whose damage divides
	gfEachCol                        // smallRecs.eachCol is set
	gfRestr                          // smallRecs.restr is set
	gfAnyR                           // smallRecs.anyR is set
	gfFilt                           // smallRecs.filt is set
	gfGapOther                       // smallRecs.gapOther is set
	gfManaVar                        // the card has a dynamic-amount planner view (varPC)
	gfManaGate                       // gated mana ability: varPC holds the gate-off and gate-on views (mana_gate.go)
	gfGlobGrant                      // a Continuous static Grant not limited to the source's host (globalGrantsGen)
	gfManaRefl                       // reflected mana: varPC holds one view per reflected colour set (mana_reflected.go)
	gfRestrSrc                       // a restricted mana ability the payment planner counts per payment (gorge; mana_restrictedsource.go)
	gfRestrScript                    // a restricted source that is a scripted-prefix gap (smallRecs.restr set and not a gfRestrSrc one)
	gfShockEntry                     // the enters program opens with the shock land's pay-life election (repl_shockland.go)
	gfFiltFee                        // a filter whose fee a reducer may take to nothing: varPC holds the fee-paid and fee-free views (mana_filterfree.go)
	gfSpendRd                        // a Tf2CastSpent or Tf3CastConverge reader: gorge's planner prices no cast while it is out (trig_castmanaspent.go)
	gfCtrAdd                         // an ExCtrAdded Self reader (ctr_addedthisturn.go)
	gfDeferred                       // a TriggersWhenSpent$ mana source gorge's planner defers (Pyromancer's Goggles; kop.Ability.ManaDeferred; mana_deferred.go)
	gfTapsDefer                      // a TrigTaps watcher classed kop.TapPlanDefer: its tapped mana source is a payment tier gorge defers (mana_tapsdefer.go)
	gfTapsPain                       // a TrigTaps watcher classed kop.TapPlanPain: its tapped mana source is a last-resort unit (mana_tapsdefer.go)
	gfEtbOrder                       // the card's own enter body is exactly one prepared line the CR 616.1 entry-order ask orders (repl_etborder.go)
	gfManaNone                       // a plain {T} mana ability of no colour (commander identity, CR 903.4f): a census gap of gorge's that adds nothing (mana_grantnone.go)
)

// cardSmall is a card's sparse generated-table values, all -1 when absent.
type cardSmall struct {
	card     CardID
	eachCol  int8 // its "one mana of each colour among" ability, -1 none
	restr    int8 // its restricted tap mana ability, -1 none
	anyR     int8 // its restricted any-colour tap mana ability, -1 none
	filt     int8 // its filter mana ability, -1 none
	gapOther int8 // another mana ability gorge's census misses, -1 none
	cdaAb    int16
	manaVar  int16 // the varPC index of the card's dynamic-amount view, -1 none
}

// has reports whether the card carries flag f.
func (g *genPrep) has(c CardID, f uint32) bool { return g.flags[c]&f != 0 }

// setFlag adds flag f to the card.
func (g *genPrep) setFlag(c CardID, f uint32) { g.flags[c] |= f }

// small returns the card's sparse record, nil when it carries none.
func (g *genPrep) small(c CardID) *cardSmall {
	lo, hi := 0, len(g.smallRecs)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		r := &g.smallRecs[mid]
		switch {
		case r.card < c:
			lo = mid + 1
		case r.card > c:
			hi = mid
		default:
			return r
		}
	}
	return nil
}

// smallFor returns the card's sparse record, appending one when the card has
// none. prepareGen sets the values in card order, so the slice stays sorted
// and a later read finds it with small.
func (g *genPrep) smallFor(c CardID) *cardSmall {
	if r := g.small(c); r != nil {
		return r
	}
	if n := len(g.smallRecs); n > 0 && g.smallRecs[n-1].card > c {
		panic("kernel: genPrep small values set out of card order")
	}
	g.smallRecs = append(g.smallRecs, cardSmall{
		card: c, eachCol: -1, restr: -1, anyR: -1, filt: -1, gapOther: -1, cdaAb: -1, manaVar: -1,
	})
	return &g.smallRecs[len(g.smallRecs)-1]
}

func (g *genPrep) eachCol(c CardID) int8 {
	if g.flags[c]&gfEachCol == 0 {
		return -1
	}
	if r := g.small(c); r != nil {
		return r.eachCol
	}
	return -1
}

func (g *genPrep) restr(c CardID) int8 {
	if g.flags[c]&gfRestr == 0 {
		return -1
	}
	if r := g.small(c); r != nil {
		return r.restr
	}
	return -1
}

func (g *genPrep) anyR(c CardID) int8 {
	if g.flags[c]&gfAnyR == 0 {
		return -1
	}
	if r := g.small(c); r != nil {
		return r.anyR
	}
	return -1
}

func (g *genPrep) filt(c CardID) int8 {
	if g.flags[c]&gfFilt == 0 {
		return -1
	}
	if r := g.small(c); r != nil {
		return r.filt
	}
	return -1
}

func (g *genPrep) gapOther(c CardID) int8 {
	if g.flags[c]&gfGapOther == 0 {
		return -1
	}
	if r := g.small(c); r != nil {
		return r.gapOther
	}
	return -1
}

func (g *genPrep) manaVar(c CardID) int16 {
	if g.flags[c]&gfManaVar == 0 {
		return -1
	}
	if r := g.small(c); r != nil {
		return r.manaVar
	}
	return -1
}

func (g *genPrep) cdaAb(c CardID) int16 {
	if g.flags[c]&gfCDAPT == 0 {
		return -1
	}
	if r := g.small(c); r != nil {
		return r.cdaAb
	}
	return -1
}
