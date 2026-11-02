package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// staticClassCase is one class bit beside the predicate its caller uses to
// decide whether it acts on a static. The gate may skip a permanent only
// when the card prints no ability that predicate accepts.
type staticClassCase struct {
	name string
	cls  uint8
	pred func(*kop.Ability) bool
}

func staticClassCases() []staticClassCase {
	cont := func(a *kop.Ability) bool {
		return a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous
	}
	mayPlay := func(a *kop.Ability) bool {
		return a.Kind == kop.AbStatic && a.Static.Kind == kop.StatMayPlayFrom
	}
	return []staticClassCase{
		{"pt", sclsPT, func(a *kop.Ability) bool {
			if !cont(a) {
				return false
			}
			sa := &a.Static
			return sa.StFlags&kop.StDynamic != 0 || sa.A != 0 || sa.B != 0
		}},
		{"kw", sclsKw, func(a *kop.Ability) bool { return cont(a) && a.Static.Kw != 0 }},
		{"sub", sclsSub, func(a *kop.Ability) bool { return cont(a) && a.Static.AddSub != (kop.SubMask{}) }},
		{"cont", sclsCont, cont},
		{"mayplay", sclsMayPlay, func(a *kop.Ability) bool {
			return mayPlay(a) && a.Static.StFlags&kop.StFunctionsInGraveyard == 0
		}},
		{"reduce", sclsReduce, func(a *kop.Ability) bool {
			return a.Kind == kop.AbStatic && a.Static.Kind == kop.StatReduceCost && a.Static.Zone != kop.ZoneCommand
		}},
		{"gymay", sclsGyMay, func(a *kop.Ability) bool {
			return mayPlay(a) && a.Static.StFlags&kop.StFunctionsInGraveyard != 0
		}},
	}
}

// staticSeq is eachStatic's statics accepted by pred; gated selects
// eachStaticC with the case's class bit instead.
func staticSeq(s *State, k *prep, c staticClassCase, gated bool) []statSrc {
	var out []statSrc
	f := func(st statSrc) bool {
		if c.pred(st.a) {
			out = append(out, st)
		}
		return true
	}
	if gated {
		s.eachStaticC(k, c.cls, f)
	} else {
		s.eachStatic(k, f)
	}
	return out
}

func checkStaticClasses(t *testing.T, s *State, k *prep, dec int) {
	t.Helper()
	for _, c := range staticClassCases() {
		want, got := staticSeq(s, k, c, false), staticSeq(s, k, c, true)
		if len(want) != len(got) {
			t.Fatalf("decision %d, class %s: eachStaticC saw %d statics, eachStatic saw %d", dec, c.name, len(got), len(want))
		}
		for i := range want {
			if want[i] != got[i] {
				t.Fatalf("decision %d, class %s: static %d differs (sl %d/%d)", dec, c.name, i, want[i].sl, got[i].sl)
			}
		}
	}
}

// checkMayPlayZones: every card mayPlayStatics offers in a graveyard, exile
// or hand lies in a zone mayPlayZones reports, unless its own graveyard
// permission is the offer.
func checkMayPlayZones(t *testing.T, s *State, k *prep, dec int) {
	t.Helper()
	for p := uint8(0); p < 2; p++ {
		zones := s.mayPlayZones(k, p)
		for o := Obj(0); o < Obj(s.nobj); o++ {
			ob := &s.objs[o]
			if ob.zone != kop.ZoneGraveyard && ob.zone != kop.ZoneExile && ob.zone != kop.ZoneHand {
				continue
			}
			offered := false
			s.mayPlayStatics(k, p, o, func(statSrc) bool {
				offered = true
				return false
			})
			if !offered {
				continue
			}
			if zones&(1<<ob.zone) != 0 {
				continue
			}
			if ob.zone == kop.ZoneGraveyard && ob.owner() == p && k.gen.sCls[ob.card]&sclsGyMay != 0 {
				continue
			}
			t.Fatalf("decision %d: mayPlayStatics offers %s (obj %d, zone %d, owner %d) to p %d, but mayPlayZones is %#x",
				dec, k.t.Cards[ob.card].Name, o, ob.zone, ob.owner(), p, zones)
		}
	}
}

// fdnClassGame plays one FDN pairing, checking both static-gate properties
// at every decision.
func fdnClassGame(t *testing.T, decks [][]cardset.Entry, g, maxDec int) (checked int) {
	t.Helper()
	a := decks[(g*7919)%len(decks)]
	b := decks[(g*104729+31)%len(decks)]
	set := cardsettest.Set(t, "fdn")
	tbl, da, db, err := set.Matchup(a, b)
	if err != nil {
		t.Fatalf("pairing %d: %v", g, err)
	}
	defer ReleaseTable(tbl)
	s := NewConfig(tbl, da, db, uint64(g)*0x9e37+1, Config{Mulligans: uint8(g % 3), NoElide: g%2 == 1})
	pol := NewRandomPolicy(uint64(g)*7919 + 3)
	k := s.k()
	var d Decision
	for checked < maxDec {
		s.StepInto(&d)
		checkStaticClasses(t, s, k, checked)
		checkMayPlayZones(t, s, k, checked)
		checked++
		if d.Kind == DkNone {
			return checked
		}
		s.Apply(pol.Pick(&d))
	}
	return checked
}

// TestEachStaticCMatchesEachStatic plays FDN soak games, then checks a
// scenario holding only dynamic P/T statics: the soak decks need not draw
// one, and dropping StDynamic from sclsPT must turn the test red.
func TestEachStaticCMatchesEachStatic(t *testing.T) {
	decks := fdnDecks(t)
	const games = 6
	total := 0
	for g := 0; g < games; g++ {
		total += fdnClassGame(t, decks, g, 4000)
	}
	sc, id := newObjScen(t, []cardset.Entry{
		{Name: "Fiend Artisan", Count: 1},
		{Name: "Banner of Kinship", Count: 1},
		{Name: "Plains", Count: 20},
	})
	sc.perm(id["Fiend Artisan"], 0)
	sc.perm(id["Banner of Kinship"], 0)
	checkStaticClasses(t, sc.s, sc.k, -1)
	t.Logf("%d FDN games, %d states checked", games, total)
}

// TestMayPlayZonesCoversPermissions plays the same soak games, then a
// scenario with a hand permission, so the controller filter and the zone
// bitmask are exercised even when the random games never cast one.
func TestMayPlayZonesCoversPermissions(t *testing.T) {
	decks := fdnDecks(t)
	const games = 6
	total := 0
	for g := 0; g < games; g++ {
		total += fdnClassGame(t, decks, g, 4000)
	}
	sc, id := newObjScen(t, []cardset.Entry{
		{Name: "Omniscience", Count: 1},
		{Name: "Quilled Greatwurm", Count: 1},
		{Name: "Burglar Rat", Count: 1},
		{Name: "Plains", Count: 20},
	})
	sc.perm(id["Omniscience"], 0)
	sc.hand(id["Burglar Rat"], 0)
	wurm := sc.obj(id["Quilled Greatwurm"], 0)
	sc.s.place(wurm, kop.ZoneGraveyard, false)
	checkMayPlayZones(t, sc.s, sc.k, -1)
	t.Logf("%d FDN games, %d states checked", games, total)
}

// grantMiss is the first occupied slot for which gate reports false while
// grantedAbilitiesN still makes callbacks, with the callback count. A
// gate that is a superset of "grantedAbilitiesN reports something" never
// produces a miss.
func grantMiss(s *State, k *prep, gate func(*prep, Slot) bool) (Slot, int) {
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if gate(k, sl) {
			continue
		}
		n := 0
		s.grantedAbilitiesN(k, sl, func(CardID, uint8, int) { n++ })
		if n != 0 {
			return sl, n
		}
	}
	return NoSlot, 0
}

// fdnGrantGame plays one FDN pairing, checking at every decision that
// grantMay false means grantedAbilitiesN makes no callback for any occupied
// slot (a superset property: it holds in every state, not only priority).
func fdnGrantGame(t *testing.T, decks [][]cardset.Entry, g, maxDec int) (decisions, priority int) {
	t.Helper()
	a := decks[(g*7919)%len(decks)]
	b := decks[(g*104729+31)%len(decks)]
	set := cardsettest.Set(t, "fdn")
	tbl, da, db, err := set.Matchup(a, b)
	if err != nil {
		t.Fatalf("pairing %d: %v", g, err)
	}
	defer ReleaseTable(tbl)
	s := NewConfig(tbl, da, db, uint64(g)*0x9e37+1, Config{Mulligans: uint8(g % 3), NoElide: g%2 == 1})
	pol := NewRandomPolicy(uint64(g)*7919 + 3)
	var d Decision
	for decisions < maxDec {
		s.StepInto(&d)
		if d.Kind == DkNone {
			return decisions, priority
		}
		k := s.k()
		if sl, n := grantMiss(s, k, s.grantMay); sl != NoSlot {
			t.Fatalf("pairing %d decision %d: grantMay false for slot %d, grantedAbilitiesN made %d callbacks", g, decisions, sl, n)
		}
		if d.Kind == DkPriority {
			priority++
		}
		decisions++
		s.Apply(pol.Pick(&d))
	}
	return decisions, priority
}

// TestGrantMaySuperset plays FDN soak games, then checks the one board the
// soak decks may never draw: an Imprisoned in the Moon land, where a grant
// is live and grantMay must report it. Forcing the gate false must find the
// missed grant (kfuzz pair 5618), so the assertion cannot go vacuous.
func TestGrantMaySuperset(t *testing.T) {
	decks := fdnDecks(t)
	const games = 6
	total, priority := 0, 0
	for g := 0; g < games; g++ {
		d, p := fdnGrantGame(t, decks, g, 4000)
		total += d
		priority += p
	}
	sc, id := newObjScen(t, []cardset.Entry{
		{Name: "Imprisoned in the Moon", Count: 1},
		{Name: "Island", Count: 4},
	})
	s := sc.s
	gap := sc.perm(id["Island"], 0)
	moon := sc.perm(id["Imprisoned in the Moon"], 0)
	s.perms[sc.slot(moon)].attached = sc.slot(gap)
	s.loseAllOnset(sc.k, sc.slot(moon), sc.slot(gap))
	k := s.k()
	if !s.grantMay(k, sc.slot(gap)) {
		t.Fatal("grantMay false for an Imprisoned in the Moon land with a live grant")
	}
	if sl, n := grantMiss(s, k, s.grantMay); sl != NoSlot {
		t.Fatalf("grantMay false for slot %d, grantedAbilitiesN made %d callbacks", sl, n)
	}
	if sl, _ := grantMiss(s, k, func(*prep, Slot) bool { return false }); sl == NoSlot {
		t.Fatal("a forced-false gate missed no grant on the Imprisoned in the Moon board")
	}
	t.Logf("%d FDN games, %d priority decisions checked", games, priority)
}

// sbaWidenedFastPath is the fast-path condition in sba (turn.go) with the
// widened term, so the test checks exactly the creatures whose P/T sba
// skips reading.
func sbaWidenedFastPath(s *State, k *prep, sl Slot, ptQuiet bool) bool {
	p := &s.perms[sl]
	return p.damage == 0 && p.ctrLow(int(kop.CtrM1M1)) == 0 && p.eotT >= 0 &&
		(!s.statM.any() || ptQuiet && !s.fxM.has(sl) && p.ovr.walk&walkOn == 0) &&
		p.ovr.flags&(ovSetPT|ovSetPTEOT|ovCopy) == 0 && k.cards[p.card].tough > 0
}

// checkSbaPTQuiet checks every creature taking the widened fast path: sba
// skips ptGen there, so ptGen must still report toughness > 0 and >
// damage (the SBA 704.5f/g conditions cannot hold).
func checkSbaPTQuiet(t *testing.T, s *State, k *prep, dec int) (fast, widened int) {
	t.Helper()
	ptQuiet := s.statM.any() && s.sbaPTQuiet(k)
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if s.typesAt(k, sl)&kop.TCreature == 0 || !sbaWidenedFastPath(s, k, sl, ptQuiet) {
			continue
		}
		fast++
		if ptQuiet {
			widened++
		}
		p := &s.perms[sl]
		_, tough := s.ptGen(k, sl)
		if tough <= 0 || int(p.damage) >= tough {
			t.Fatalf("decision %d: slot %d took sba's fast path but ptGen toughness %d (damage %d, ptQuiet %v)", dec, sl, tough, p.damage, ptQuiet)
		}
	}
	return fast, widened
}

// fdnSbaGame plays one FDN pairing, checking the widened sba fast path at
// every decision (sba runs before every priority decision and between the
// combat and resolution steps, so this samples the states it sees).
func fdnSbaGame(t *testing.T, decks [][]cardset.Entry, g, maxDec int) (decisions, fast, widened int) {
	t.Helper()
	a := decks[(g*7919)%len(decks)]
	b := decks[(g*104729+31)%len(decks)]
	set := cardsettest.Set(t, "fdn")
	tbl, da, db, err := set.Matchup(a, b)
	if err != nil {
		t.Fatalf("pairing %d: %v", g, err)
	}
	defer ReleaseTable(tbl)
	s := NewConfig(tbl, da, db, uint64(g)*0x9e37+1, Config{Mulligans: uint8(g % 3), NoElide: g%2 == 1})
	pol := NewRandomPolicy(uint64(g)*7919 + 3)
	var d Decision
	for decisions < maxDec {
		s.StepInto(&d)
		if d.Kind == DkNone {
			return decisions, fast, widened
		}
		f, w := checkSbaPTQuiet(t, s, s.k(), decisions)
		fast += f
		widened += w
		decisions++
		s.Apply(pol.Pick(&d))
	}
	return decisions, fast, widened
}

// TestSbaPTQuietMatchesPT plays FDN soak games, then checks boards holding
// each kind of static sclsPTRel must name: a P/T lord, a CDA creature and an
// override Aura all make sbaPTQuiet false; a board with none makes it true.
func TestSbaPTQuietMatchesPT(t *testing.T) {
	decks := fdnDecks(t)
	const games = 6
	total, fast, widened := 0, 0, 0
	for g := 0; g < games; g++ {
		d, f, w := fdnSbaGame(t, decks, g, 4000)
		total += d
		fast += f
		widened += w
	}
	if widened == 0 {
		t.Fatal("no creature took the widened sba fast path in the soak games")
	}
	quiet, qid := newObjScen(t, []cardset.Entry{
		{Name: "Savannah Lions", Count: 1},
		{Name: "Island", Count: 4},
	})
	quiet.perm(qid["Savannah Lions"], 0)
	quiet.perm(qid["Island"], 0)
	if !quiet.s.sbaPTQuiet(quiet.k) {
		t.Fatal("sbaPTQuiet false with no P/T-related static on the battlefield")
	}
	lord, lid := newObjScen(t, []cardset.Entry{
		{Name: "Empyrean Eagle", Count: 1},
		{Name: "Savannah Lions", Count: 1},
		{Name: "Island", Count: 4},
	})
	lord.perm(lid["Empyrean Eagle"], 0)
	lord.perm(lid["Savannah Lions"], 0)
	if lord.s.sbaPTQuiet(lord.k) {
		t.Fatal("sbaPTQuiet true with a P/T lord (sclsPTRel) on the battlefield")
	}
	cda, cid := newObjScen(t, []cardset.Entry{
		{Name: "Consuming Aberration", Count: 1},
		{Name: "Island", Count: 4},
	})
	cda.perm(cid["Consuming Aberration"], 0)
	if cda.s.sbaPTQuiet(cda.k) {
		t.Fatal("sbaPTQuiet true with a CDA creature (sclsPTRel) on the battlefield")
	}
	ovr, oid := newObjScen(t, []cardset.Entry{
		{Name: "Imprisoned in the Moon", Count: 1},
		{Name: "Island", Count: 4},
	})
	host := ovr.perm(oid["Island"], 0)
	moon := ovr.perm(oid["Imprisoned in the Moon"], 0)
	ovr.s.perms[ovr.slot(moon)].attached = ovr.slot(host)
	ovr.s.loseAllOnset(ovr.k, ovr.slot(moon), ovr.slot(host))
	if ovr.s.sbaPTQuiet(ovr.k) {
		t.Fatal("sbaPTQuiet true with an override Aura (sclsPTRel) on the battlefield")
	}
	t.Logf("%d FDN games, %d decisions, %d fast-path checks (%d widened)", games, total, fast, widened)
}
