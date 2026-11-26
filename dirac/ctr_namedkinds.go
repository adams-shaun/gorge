package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// namedCounterKinds are the named counters the generic per-kind walks serve
// (ProjObj.CountersX, the per-kind put/remove paths and the oracle's per-kind
// comparison). They carry no rules meaning (CR 122.1): a named counter is only
// counted, put and removed, so the kernel needs no per-kind code. Listing a
// kind here is the kernel's statement that it implements it.
var namedCounterKinds = [...]kop.CounterKind{
	kop.CtrKi,
	kop.CtrBounty,
	kop.CtrBrick,
	kop.CtrPressure,
	kop.CtrBlood,
	kop.CtrBook,
	kop.CtrCorpse,
	kop.CtrDevotion,
	kop.CtrFate,
	kop.CtrGold,
	kop.CtrHatchling,
	kop.CtrIce,
	kop.CtrInfection,
	kop.CtrSlime,
	kop.CtrStudy,
	kop.CtrAcorn,
	kop.CtrAcquiredTaste,
	kop.CtrArrowhead,
	kop.CtrBlaze,
	kop.CtrBlessing,
	kop.CtrBribery,
	kop.CtrCarrion,
	kop.CtrCollection,
	kop.CtrCredit,
	kop.CtrCroak,
	kop.CtrCrystal,
	kop.CtrDeath,
	kop.CtrEmber,
	kop.CtrEverything,
	kop.CtrFeather,
	kop.CtrFilibuster,
	kop.CtrFlood,
	kop.CtrForeshadow,
	kop.CtrFungus,
	kop.CtrFury,
	kop.CtrGem,
	kop.CtrHarmony,
	kop.CtrHealing,
	kop.CtrHit,
	kop.CtrHoofprint,
	kop.CtrHope,
	kop.CtrHour,
	kop.CtrInfluence,
	kop.CtrIngredient,
	kop.CtrInvitation,
	kop.CtrIsolation,
	kop.CtrJavelin,
	kop.CtrKnowledge,
	kop.CtrManaBond,
	kop.CtrMuster,
	kop.CtrNight,
	kop.CtrPetal,
	kop.CtrPhyresis,
	kop.CtrPin,
	kop.CtrPlague,
	kop.CtrPlot,
	kop.CtrPolyp,
	kop.CtrRally,
	kop.CtrRibbon,
	kop.CtrShell,
	kop.CtrShred,
	kop.CtrSin,
	kop.CtrSleep,
	kop.CtrSlumber,
	kop.CtrSoot,
	kop.CtrStrife,
	kop.CtrTower,
	kop.CtrVoid,
	kop.CtrElixir,  // Essence Bottle (cost_prog.go: SubCounter<All/ELIXIR>)
	kop.CtrEyeball, // Jar of Eyeballs
	kop.CtrPain,    // Torture Chamber
}

func init() {
	for _, k := range namedCounterKinds {
		if !kop.IsCounterKind(int(k)) || k >= kop.NumCounterKinds3 {
			panic("dirac: named counter kind outside the frozen cover")
		}
	}
}
