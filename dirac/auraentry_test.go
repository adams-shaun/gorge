package dirac

import (
	"math/bits"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestCopiedAuraChoosesWhatItEnchants: Extravagant Replication's token
// copy of Imprisoned in the Moon enchants a permanent its controller
// chooses as it enters (CR 303.4f). gorge mints the token, asks "enchant"
// while it is in no zone (its zone 0) and the trigger is still on the
// stack, then puts it onto the battlefield attached (pairs 4016 5228 6813
// 7123 7430 9086 9409). Before, the kernel's copy entered unattached and the
// state-based check put it into the graveyard.
func TestCopiedAuraChoosesWhatItEnchants(t *testing.T) {
	deck := []cardset.Entry{{Name: "Extravagant Replication", Count: 1}, {Name: "Imprisoned in the Moon", Count: 1}, {Name: "Eager Trufflesnout", Count: 2}, {Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	s.active, s.prio = 1, 1
	sc.perm(id["Extravagant Replication"], 0)
	c1 := sc.perm(id["Eager Trufflesnout"], 1)
	c2 := sc.perm(id["Eager Trufflesnout"], 1)
	moon := sc.perm(id["Imprisoned in the Moon"], 0)
	s.perms[sc.slot(moon)].attached = sc.slot(c1)
	s.loseAllOnset(sc.k, sc.slot(moon), sc.slot(c1))
	var d Decision
	for i := 0; i < 80; i++ {
		d = s.Step()
		if d.Kind == DkTarget || d.Kind == DkNone {
			break
		}
		s.Apply(d.Options[d.N-1]) // pass, or Done on a declaration
	}
	if d.Kind != DkTarget || s.active != 0 {
		t.Fatalf("want the Replication's upkeep target ask, got kind %d active %d", d.Kind, s.active)
	}
	sc.s.Apply(MakeAction(AkTarget, 0, TCode(moon), 0))
	for i := 0; i < 4; i++ {
		d = s.Step()
		if d.Kind != DkPriority {
			break
		}
		s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	if d.Kind != DkChoose || d.Sub != ChGenEnchant {
		t.Fatalf("want the enchant ask, got kind %d sub %d", d.Kind, d.Sub)
	}
	var tok Obj = NoObj
	for o := Obj(0); int(o) < int(s.nobj); o++ {
		if s.objs[o].of&ofPending != 0 {
			tok = o
		}
	}
	if tok == NoObj || s.sp == 0 {
		t.Fatalf("want the token pending and the trigger on the stack (sp %d)", s.sp)
	}
	var p Projection
	s.Project(&p)
	if p.Objs[tok].Zone != kop.ZoneLibrary || p.Objs[tok].ZoneIdx != 255 {
		t.Fatalf("pending token projected zone %d idx %d, want gorge's zone 0 (library, no position)", p.Objs[tok].Zone, p.Objs[tok].ZoneIdx)
	}
	found := false
	for _, a := range d.Options[:d.N] {
		found = found || a.Source() == TCode(uint8(c2))
	}
	if !found || d.N < 2 {
		t.Fatalf("enchant options %v lack the second creature", d.Options[:d.N])
	}
	s.Apply(MakeAction(AkChoose, TCode(c2), 0, 0))
	if !sc.onBF(tok) || s.perms[sc.slot(tok)].attached != sc.slot(c2) {
		t.Fatalf("token zone %d, want on the battlefield attached to the chosen creature", s.objs[tok].zone)
	}
	s.Step()
	if !sc.onBF(tok) {
		t.Fatal("the attached token copy left the battlefield")
	}
}

// TestReturnedAuraChoosesWhatItEnchants: kicked Sun-Blessed Healer
// returns Witness Protection from the graveyard to the battlefield; the
// Aura enchants a creature its controller chooses as it enters (CR
// 303.4f), asked while it is still in the graveyard and the trigger on the
// stack (gorge's "enchant"; pairs 1769 2801 5974 8546). Before, it entered
// unattached and went back to the graveyard.
func TestReturnedAuraChoosesWhatItEnchants(t *testing.T) {
	deck := []cardset.Entry{{Name: "Sun-Blessed Healer", Count: 1}, {Name: "Witness Protection", Count: 1}, {Name: "Eager Trufflesnout", Count: 2}, {Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	for i := 0; i < 4; i++ {
		sc.perm(id["Plains"], 0)
	}
	sc.perm(id["Eager Trufflesnout"], 0)
	c2 := sc.perm(id["Eager Trufflesnout"], 1)
	wp := sc.obj(id["Witness Protection"], 0)
	s.place(wp, kop.ZoneGraveyard, false)
	healer := sc.hand(id["Sun-Blessed Healer"], 0)
	d := s.Step()
	kicked := false
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && a.Source() == TCode(uint8(healer)) && a.Mode()&1 != 0 {
			s.Apply(a)
			kicked = true
			break
		}
	}
	if !kicked {
		t.Fatalf("no kicked cast offered: %v", d.Options[:d.N])
	}
	for i := 0; i < 12; i++ {
		d = s.Step()
		if d.Kind == DkChoose && d.Sub == ChGenEnchant {
			break
		}
		if d.Kind == DkTarget {
			s.Apply(MakeAction(AkTarget, 0, TCode(wp), 0))
			continue
		}
		s.Apply(d.Options[0])
	}
	if d.Kind != DkChoose || d.Sub != ChGenEnchant {
		t.Fatalf("want the enchant ask, got kind %d sub %d", d.Kind, d.Sub)
	}
	if s.objs[wp].zone != kop.ZoneGraveyard || s.sp == 0 {
		t.Fatalf("want the Aura in the graveyard and the trigger on the stack at the ask (zone %d sp %d)", s.objs[wp].zone, s.sp)
	}
	s.Apply(MakeAction(AkChoose, TCode(c2), 0, 0))
	s.Step()
	if !sc.onBF(wp) || s.perms[sc.slot(wp)].attached != sc.slot(c2) {
		t.Fatalf("Witness Protection zone %d, want on the battlefield attached to the chosen creature", s.objs[wp].zone)
	}
}

// TestAuraOnIllegalHostGoesToGraveyard: Imprisoned in the Moon makes Witness
// Protection's creature a land, and Witness Protection ("Enchant creature")
// is put into its owner's graveyard by the state-based check (CR 704.5m;
// pair 1133). Before, only an unattached Aura was.
func TestAuraOnIllegalHostGoesToGraveyard(t *testing.T) {
	deck := []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Imprisoned in the Moon", Count: 1}, {Name: "Eager Trufflesnout", Count: 1}, {Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	c := sc.perm(id["Eager Trufflesnout"], 1)
	attach := func(name string) Obj {
		a := sc.perm(id[name], 0)
		s.perms[sc.slot(a)].attached = sc.slot(c)
		s.loseAllOnset(sc.k, sc.slot(a), sc.slot(c))
		return a
	}
	wp := attach("Witness Protection")
	s.Step()
	if !sc.onBF(wp) {
		t.Fatal("precondition: Witness Protection left a creature")
	}
	moon := attach("Imprisoned in the Moon")
	s.Step()
	if sc.onBF(wp) || s.objs[wp].zone != kop.ZoneGraveyard {
		t.Fatalf("Witness Protection on a land: zone %d, want the graveyard", s.objs[wp].zone)
	}
	if !sc.onBF(moon) {
		t.Fatal("Imprisoned in the Moon left its land")
	}
}

// TestCopyTokenAsEntersChoiceFirst: a token copy of Heraldic Banner
// (Extravagant Replication's upkeep copy) is asked its colour before it
// enters: gorge's minted token waits outside every zone (zone 0) for the
// as-enters choice, as a permanent spell waits on the stack (pairs 723 895
// 3961). Before, the kernel put it onto the battlefield and asked after.
func TestCopyTokenAsEntersChoiceFirst(t *testing.T) {
	deck := []cardset.Entry{{Name: "Extravagant Replication", Count: 1}, {Name: "Heraldic Banner", Count: 1}, {Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	sc.perm(id["Extravagant Replication"], 0)
	banner := sc.perm(id["Heraldic Banner"], 0)
	s.perms[sc.slot(banner)].chosen = 0
	n0 := s.nobj
	for i := 0; i < 300; i++ {
		d := s.Step()
		switch {
		case d.Kind == DkNone:
			t.Fatal("the game ended")
		case d.Kind == DkTarget:
			s.Apply(MakeAction(AkTarget, 0, TCode(banner), 0))
			continue
		case d.Kind == DkChoose && d.Sub == ChGenColor && s.nobj > n0:
			tok := Obj(s.nobj - 1)
			if s.objs[tok].zone == kop.ZoneBattlefield || s.objs[tok].of&ofPending == 0 {
				t.Fatalf("at the colour ask the copy token is in zone %d (pending %v); want it waiting outside every zone", s.objs[tok].zone, s.objs[tok].of&ofPending != 0)
			}
			s.Apply(d.Options[2])
			if s.objs[tok].zone != kop.ZoneBattlefield || s.perms[s.objs[tok].slot].chosen != 2 {
				t.Fatalf("after the answer: token zone %d chosen %d, want the battlefield with colour 2", s.objs[tok].zone, s.perms[s.objs[tok].slot].chosen)
			}
			return
		}
		s.Apply(d.Options[0])
	}
	t.Fatal("no copy token colour ask")
}

// TestReturnFromExileAsEntersChoiceFirst: when Banishing Light leaves
// during a resolution, the Heraldic Banner it exiled is asked its colour
// while still in exile, and enters with it (gorge's return replacement;
// pair 544). Before, the kernel returned it and asked after the
// resolution.
func TestReturnFromExileAsEntersChoiceFirst(t *testing.T) {
	deck := []cardset.Entry{{Name: "Banishing Light", Count: 2}, {Name: "Heraldic Banner", Count: 1}, {Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	for i := 0; i < 3; i++ {
		sc.perm(id["Plains"], 0)
	}
	a := sc.perm(id["Banishing Light"], 1)
	banner := sc.obj(id["Heraldic Banner"], 0)
	s.place(banner, kop.ZoneExile, false)
	l := &s.g.links[s.g.nlinks]
	l.src, l.sver, l.obj = a, s.objs[a].ver, banner
	s.g.nlinks++
	b := sc.hand(id["Banishing Light"], 0)
	sc.do(t, AkCast, uint8(b), 0xff)
	for i := 0; i < 40; i++ {
		d := s.Step()
		switch {
		case d.Kind == DkNone:
			t.Fatal("the game ended")
		case d.Kind == DkTarget:
			s.Apply(MakeAction(AkTarget, 0, TCode(a), 0))
			continue
		case d.Kind == DkChoose && d.Sub == ChGenColor:
			if s.objs[banner].zone != kop.ZoneExile {
				t.Fatalf("at the colour ask Heraldic Banner is in zone %d, want exile", s.objs[banner].zone)
			}
			s.Apply(d.Options[3])
			if s.objs[banner].zone != kop.ZoneBattlefield || s.perms[s.objs[banner].slot].chosen != 3 {
				t.Fatalf("after the answer: zone %d, want the battlefield with colour 3", s.objs[banner].zone)
			}
			return
		}
		s.Apply(d.Options[0])
	}
	t.Fatal("no colour ask")
}

// TestReturnedAuraAsksHostInExile: Witness Protection exiled by Banishing
// Light returns when the Banishing Light is destroyed by a resolving
// ability (Cathar Commando's); during the resolution gorge asks the
// Aura's host with the card still in exile (CR 303.4f) and it enters
// attached. Before, the kernel entered it unattached and it went to the
// graveyard (pairs 5605 7877).
func TestReturnedAuraAsksHostInExile(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Banishing Light", Count: 1}, {Name: "Cathar Commando", Count: 1}, {Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 4}})
	sc.perm(id["Plains"], 0)
	sc.perm(id["Plains"], 0)
	cc := sc.perm(id["Cathar Commando"], 0)
	l1 := sc.perm(id["Savannah Lions"], 0)
	sc.perm(id["Savannah Lions"], 0)
	bl := sc.perm(id["Banishing Light"], 1)
	wp := sc.obj(id["Witness Protection"], 0)
	sc.s.place(wp, kop.ZoneExile, false)
	sc.s.g.links[0].src, sc.s.g.links[0].sver, sc.s.g.links[0].obj = bl, sc.s.objs[bl].ver, wp
	sc.s.g.nlinks = 1
	sc.do(t, AkActivate, uint8(cc), 0xff)
	for i := 0; i < 12 && sc.s.objs[bl].zone == kop.ZoneBattlefield; i++ {
		d := sc.s.Step()
		a := d.Options[0]
		for j := 0; j < int(d.N); j++ {
			if o := d.Options[j]; o.Kind() == AkTarget && o.Target() == TCode(uint8(bl)) || o.Kind() == AkPass {
				a = o
				break
			}
		}
		sc.s.Apply(a)
	}
	if sc.s.objs[bl].zone == kop.ZoneBattlefield {
		t.Fatal("Banishing Light not destroyed")
	}
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenEnchant || sc.s.objs[wp].zone != kop.ZoneExile {
		t.Fatalf("decision kind %d sub %d, Witness Protection zone %d: want the host ask in exile", d.Kind, d.Sub, sc.s.objs[wp].zone)
	}
	sc.s.Apply(MakeAction(AkChoose, TCode(l1), 0, 0))
	sc.s.Step()
	if sc.s.objs[wp].zone != kop.ZoneBattlefield || sc.s.perms[sc.slot(wp)].attached != sc.slot(l1) {
		t.Fatalf("Witness Protection zone %d, want it on the battlefield attached to the chosen Lions", sc.s.objs[wp].zone)
	}
}

// TestReturnFromExileAfterEnterChoiceMidResolution: Banner of Kinship's
// type choice is a ReplAfterEnter program (asked once it is on the
// battlefield). When the Banishing Light that exiled it leaves during a
// resolution, gorge asks the type at once, with the resolving trigger
// still on the stack, and the trigger finishes after the answer (pair
// 4150, a sacrifice to Perforating Artist's trigger). Before, the kernel
// queued the ask and posed it after the resolution, on an empty stack.
func TestReturnFromExileAfterEnterChoiceMidResolution(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Banishing Light", Count: 2}, {Name: "Banner of Kinship", Count: 1}, {Name: "Plains", Count: 10}})
	s := sc.s
	for i := 0; i < 3; i++ {
		sc.perm(id["Plains"], 0)
	}
	a := sc.perm(id["Banishing Light"], 1)
	banner := sc.obj(id["Banner of Kinship"], 0)
	s.place(banner, kop.ZoneExile, false)
	l := &s.g.links[s.g.nlinks]
	l.src, l.sver, l.obj = a, s.objs[a].ver, banner
	s.g.nlinks++
	b := sc.hand(id["Banishing Light"], 0)
	sc.do(t, AkCast, uint8(b), 0xff)
	for i := 0; i < 40; i++ {
		d := s.Step()
		switch {
		case d.Kind == DkNone:
			t.Fatal("the game ended")
		case d.Kind == DkTarget:
			s.Apply(MakeAction(AkTarget, 0, TCode(a), 0))
			continue
		case d.Kind == DkChoose && d.Sub == ChGenType:
			if s.objs[banner].zone != kop.ZoneBattlefield {
				t.Fatalf("at the type ask Banner of Kinship is in zone %d, want the battlefield", s.objs[banner].zone)
			}
			if s.sp != 1 {
				t.Fatalf("at the type ask the stack holds %d entries, want 1 (the resolving trigger)", s.sp)
			}
			if d.Player != 0 {
				t.Fatalf("the type ask goes to player %d, want 0", d.Player)
			}
			s.Apply(d.Options[0])
			if s.perms[s.objs[banner].slot].chosen == 0 {
				t.Fatal("after the answer no type is chosen")
			}
			d = s.Step()
			if s.sp != 0 {
				t.Fatalf("after the answer the stack holds %d entries, want 0", s.sp)
			}
			return
		}
		s.Apply(d.Options[0])
	}
	t.Fatal("no type ask")
}

// TestEquipmentFallsOffNoncreature: Imprisoned in the Moon makes an
// equipped creature a land; the Equipment attached to it becomes
// unattached as a state-based action and stays on the battlefield (CR
// 704.5n; gorge attachmentSBAs). Before, the kernel left Goldvein Pick
// attached to the land (pair 9856).
func TestEquipmentFallsOffNoncreature(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Imprisoned in the Moon", Count: 1}, {Name: "Goldvein Pick", Count: 1}, {Name: "Savannah Lions", Count: 1}, {Name: "Island", Count: 4}})
	s := sc.s
	for i := 0; i < 3; i++ {
		sc.perm(id["Island"], 0)
	}
	lions := sc.perm(id["Savannah Lions"], 1)
	pick := sc.perm(id["Goldvein Pick"], 1)
	s.perms[sc.slot(pick)].attached = sc.slot(lions)
	moon := sc.hand(id["Imprisoned in the Moon"], 0)
	sc.do(t, AkCast, uint8(moon), 0xff)
	for i := 0; i < 20 && !(sc.onBF(moon) && s.sp == 0); i++ {
		d := s.Step()
		switch d.Kind {
		case DkNone:
			t.Fatal("the game ended")
		case DkTarget:
			s.Apply(MakeAction(AkTarget, 0, TCode(lions), 0))
			continue
		}
		s.Apply(d.Options[0])
	}
	if !sc.onBF(moon) {
		t.Fatal("Imprisoned in the Moon did not resolve")
	}
	s.Step() // the state-based actions before the next priority
	if !sc.onBF(pick) || !sc.onBF(lions) {
		t.Fatal("the Pick or the enchanted land left the battlefield")
	}
	if at := s.perms[sc.slot(pick)].attached; at != NoSlot {
		t.Fatalf("Goldvein Pick is still attached to slot %d, want unattached", at)
	}
}

// TestLossEndRestoresOlderEOTGrants: Fleeting Flight's until-end-of-turn
// flying and combat-damage prevention, then Witness Protection: the later
// loss removes the older grants only while it applies (CR 613.7, 611.2c);
// once the Aura is destroyed the same turn they are back, as in gorge's
// layer-6 walk. A grant made after the loss survives it. Before, the
// onset cleared the older grants for good (pair 42681 -features).
func TestLossEndRestoresOlderEOTGrants(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 4}})
	s := sc.s
	rat := sc.perm(id["Savannah Lions"], 0)
	sl := sc.slot(rat)
	g := kop.KwFlying | kop.KwPreventCombatDamageTo
	s.grantEOT(sl, g)
	s.grantEOT(sl, kop.KwFlying) // two grants: gorge lists flying twice
	wp := sc.perm(id["Witness Protection"], 1)
	s.perms[sc.slot(wp)].attached = sl
	s.loseAllOnset(sc.k, sc.slot(wp), sl)
	if m := s.kw(sc.k, sl); m&g != 0 {
		t.Fatalf("while the loss applies the Lions keep %#x of the older grants", m&g)
	}
	s.grantEOT(sl, kop.KwVigilance)
	if s.kw(sc.k, sl)&kop.KwVigilance == 0 {
		t.Fatal("a grant newer than the loss is removed")
	}
	s.removePerm(sc.k, sc.slot(wp), kop.ZoneGraveyard, false)
	if m := s.kw(sc.k, sl); m&g != g {
		t.Fatalf("after the loss ends the Lions have %#x of the older grants %#x", m&g, g)
	}
	gr, rep, _ := s.FeatGrantedRep(rat)
	if gr&g != g || rep[bits.TrailingZeros32(uint32(kop.KwFlying))] != 1 {
		t.Fatalf("the feature view lists %#x (flying repeats %d), want %#x with flying listed twice", gr, rep[bits.TrailingZeros32(uint32(kop.KwFlying))], g)
	}
}

// TestReturnedAuraOneHostAttaches: with one creature it can enchant, the
// returning Witness Protection enters attached to it without an ask (CR
// 303.4f; pair 64152). Before, it entered unattached and went to the
// graveyard.
func TestReturnedAuraOneHostAttaches(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Banishing Light", Count: 1}, {Name: "Cathar Commando", Count: 1}, {Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 4}})
	sc.perm(id["Plains"], 0)
	sc.perm(id["Plains"], 0)
	cc := sc.perm(id["Cathar Commando"], 0)
	l1 := sc.perm(id["Savannah Lions"], 0)
	bl := sc.perm(id["Banishing Light"], 1)
	wp := sc.obj(id["Witness Protection"], 0)
	sc.s.place(wp, kop.ZoneExile, false)
	sc.s.g.links[0].src, sc.s.g.links[0].sver, sc.s.g.links[0].obj = bl, sc.s.objs[bl].ver, wp
	sc.s.g.nlinks = 1
	sc.do(t, AkActivate, uint8(cc), 0xff)
	for i := 0; i < 12 && sc.s.objs[bl].zone == kop.ZoneBattlefield; i++ {
		d := sc.s.Step()
		a := d.Options[0]
		for j := 0; j < int(d.N); j++ {
			if o := d.Options[j]; o.Kind() == AkTarget && o.Target() == TCode(uint8(bl)) || o.Kind() == AkPass {
				a = o
				break
			}
		}
		sc.s.Apply(a)
	}
	sc.s.Step()
	if sc.s.objs[wp].zone != kop.ZoneBattlefield || sc.s.perms[sc.slot(wp)].attached != sc.slot(l1) {
		t.Fatalf("Witness Protection zone %d, want it on the battlefield attached to the Lions", sc.s.objs[wp].zone)
	}
}

// TestAuraLeavesInThePassAfterItsHost: gorge gathers a state-based pass before it
// applies any of it, so its attachment check still sees the host, and the Aura goes
// in the next pass after that pass's casualties: here Spectral Sailor, 2/2 with 1
// damage while Empyrean Eagle lived (rules/sba.go checkStateBased, rules/attach.go
// attachmentSBAs).
func TestAuraLeavesInThePassAfterItsHost(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Empyrean Eagle", Count: 1}, {Name: "Spectral Sailor", Count: 1}, {Name: "Blanchwood Armor", Count: 1}, {Name: "Plains", Count: 4}})
	eg := sc.perm(id["Empyrean Eagle"], 0)
	ss := sc.perm(id["Spectral Sailor"], 0)
	ar := sc.perm(id["Blanchwood Armor"], 0)
	sc.s.perms[sc.slot(ar)].attached = sc.slot(eg)
	sc.s.perms[sc.slot(eg)].damage = 3
	sc.s.perms[sc.slot(ss)].damage = 1
	sc.s.sba(sc.k)
	pl := &sc.s.pl[0]
	if pl.graveN != 3 || pl.grave[0] != eg || pl.grave[1] != ss || pl.grave[2] != ar {
		t.Fatalf("graveyard %v (n %d), want Eagle %d, Sailor %d, Armor %d", pl.grave[:pl.graveN], pl.graveN, eg, ss, ar)
	}
}

// TestReturnedAuraNoHostStaysExiled: a returning Aura with nothing it can
// enchant stays in exile (CR 303.4g; gorge "nothing the Aura can legally
// enchant: it stays in its zone"): Witness Protection under a destroyed
// Banishing Light with no creature left (pair 74242). Before, the kernel
// put it onto the battlefield unattached and the state-based check put it
// into the graveyard.
func TestReturnedAuraNoHostStaysExiled(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Banishing Light", Count: 1}, {Name: "Cathar Commando", Count: 1}, {Name: "Plains", Count: 4}})
	sc.perm(id["Plains"], 0)
	sc.perm(id["Plains"], 0)
	cc := sc.perm(id["Cathar Commando"], 0)
	bl := sc.perm(id["Banishing Light"], 1)
	wp := sc.obj(id["Witness Protection"], 0)
	sc.s.place(wp, kop.ZoneExile, false)
	sc.s.g.links[0].src, sc.s.g.links[0].sver, sc.s.g.links[0].obj = bl, sc.s.objs[bl].ver, wp
	sc.s.g.nlinks = 1
	sc.do(t, AkActivate, uint8(cc), 0xff)
	for i := 0; i < 12 && sc.s.objs[bl].zone == kop.ZoneBattlefield; i++ {
		d := sc.s.Step()
		a := d.Options[0]
		for j := 0; j < int(d.N); j++ {
			if o := d.Options[j]; o.Kind() == AkTarget && o.Target() == TCode(uint8(bl)) || o.Kind() == AkPass {
				a = o
				break
			}
		}
		sc.s.Apply(a)
	}
	sc.s.Step()
	if sc.s.objs[bl].zone == kop.ZoneBattlefield || sc.s.objs[wp].zone != kop.ZoneExile {
		t.Fatalf("Banishing Light zone %d, Witness Protection zone %d: want the Aura left in exile", sc.s.objs[bl].zone, sc.s.objs[wp].zone)
	}
}
