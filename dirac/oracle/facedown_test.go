package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// The facedown lockstep: face-down casts of every Morph family, the
// turn-face-up special action and its triggered abilities. The Moxen are the
// zero-cost mana the planner spends on the two Priests; each Priest's enters
// trigger floats three mana, which is the only mana a face-down cast or a
// turn-face-up can spend (both are priced from the floating pool alone).
// Kolaghan Stormsinger's turn-face-up trigger targets a creature; Zoetic
// Cavern is the land with Morph (CR 702.37b: the face-down cast is not a land
// play), and Bolrac-Clan Basher/Basilica Stalker exercise Disguise. Concealed
// Weapon is the cheap Disguise carrier ({2}{R} turn-up, payable from one
// Priest) that proves a Disguise turn-up actually pays and executes.

var kfacedownDeck = []cardset.Entry{
	{Name: "Mox Emerald", Count: 3},
	{Name: "Mox Sapphire", Count: 3},
	{Name: "Mox Jet", Count: 4},
	{Name: "Mox Ruby", Count: 3},
	{Name: "Mox Pearl", Count: 3},
	{Name: "Priest of Urabrask", Count: 4},
	{Name: "Priest of Gix", Count: 4},
	{Name: "Aven Farseer", Count: 3},
	{Name: "Kolaghan Stormsinger", Count: 2},
	{Name: "Bolrac-Clan Basher", Count: 2},
	{Name: "Basilica Stalker", Count: 2},
	{Name: "Zoetic Cavern", Count: 3},
	{Name: "Concealed Weapon", Count: 4},
}

var kfacedownTurnUp = func() uint8 { c, _ := specialCode("turn_face_up"); return c }()

type kfacedownCounts struct {
	morph, megamorph, disguise int
	turnUp                     int
	faceUpTriggers             int
	watcher                    int
	downBy, upBy               map[string]int
}

func (c *kfacedownCounts) down(name string) int { return c.downBy[name] }
func (c *kfacedownCounts) up(name string) int   { return c.upBy[name] }

func (c *kfacedownCounts) addDown(name string) {
	if c.downBy == nil {
		c.downBy = map[string]int{}
	}
	c.downBy[name]++
}

func (c *kfacedownCounts) addUp(name string) {
	if c.upBy == nil {
		c.upBy = map[string]int{}
	}
	c.upBy[name]++
}

// kfacedownSetup counts the policy's own early actions across both seats.
type kfacedownSetup struct {
	moxen, priests, aven, turnUps int
}

func kfacedownMox(n string) bool {
	switch n {
	case "Mox Emerald", "Mox Sapphire", "Mox Jet", "Mox Ruby", "Mox Pearl":
		return true
	}
	return false
}

// kfacedownPolicy drives the setup: six Moxen, Aven Farseer (the
// turn-face-up watcher), then every Priest (each floats three mana), then
// face-down casts and turn-ups. It prefers a carrier it has not exercised
// yet so the counts prove every Morph family and every printed carrier,
// not just one. Zoetic Cavern is never played as a land: its Morph face is
// the deck's Morph carrier.
func kfacedownPolicy(seed uint64, counts *kfacedownCounts, setup *kfacedownSetup) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		var (
			mox, aven, priest, down, up                     Canon
			downName, upName                                string
			downCount, upCount                              int
			haveMox, haveAven, havePriest, haveDown, haveUp bool
		)
		for _, a := range c.D.Options {
			n := optionFaceName(c, a)
			switch {
			case a.Kind() == ActCast && kfacedownMox(n):
				mox, haveMox = a, true
			case a.Kind() == ActCast && n == "Aven Farseer":
				aven, haveAven = a, true
			case a.Kind() == ActCast && (n == "Priest of Urabrask" || n == "Priest of Gix"):
				priest, havePriest = a, true
			case a.Kind() == ActCastFace:
				if !haveDown || counts.down(n) < downCount {
					down, downName, downCount, haveDown = a, n, counts.down(n), true
				}
			case a.Kind() == ActSpecial && a.Mode() == kfacedownTurnUp:
				if !haveUp || counts.up(n) < upCount {
					up, upName, upCount, haveUp = a, n, counts.up(n), true
				}
			}
		}
		switch {
		case haveMox && setup.moxen < 6:
			setup.moxen++
			return mox
		case haveAven && setup.aven < 1:
			setup.aven++
			return aven
		case havePriest:
			setup.priests++
			return priest
		case haveDown && counts.down(downName) == 0:
			counts.addDown(downName)
			return down
		case haveUp && setup.turnUps < 4 && counts.up(upName) == 0:
			counts.addUp(upName)
			setup.turnUps++
			return up
		case haveDown:
			counts.addDown(downName)
			return down
		case haveUp && setup.turnUps < 4:
			counts.addUp(upName)
			setup.turnUps++
			return up
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCastFace, ActCast, ActActivate, ActSpecial:
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func kfacedownEnv(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	return kfacedownDeckEnv(t, kfacedownDeck)
}

func kfacedownDeckEnv(t *testing.T, deck []cardset.Entry) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("facedown")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range deck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}

// kfacedownTurnFaceUpTrigger reports whether (card, ab) is a lowered
// "when turned face up" trigger (kop.TrigTurnFaceUp), the event the count
// asserts fired; native triggers (abProwess, abWard) index past the card's
// lowered abilities and are never counted.
func kfacedownTurnFaceUpTrigger(tbl *kop.Table, card dirac.CardID, ab uint8) bool {
	if int(card) >= len(tbl.Cards) {
		return false
	}
	abs := tbl.Cards[card].Abilities
	return int(ab) < len(abs) && abs[ab].Trig.Event == kop.TrigTurnFaceUp
}

func TestFacedownLockstep(t *testing.T) {
	t.Parallel()

	reg, set, decks, tbl := kfacedownEnv(t)
	var counts kfacedownCounts
	seeds := uint64(48)
	if testing.Short() {
		seeds = 8
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kf-a", "kf-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var setup kfacedownSetup
		pol := kfacedownPolicy(seed, &counts, &setup)
		seen := map[[2]uint32]bool{}
		watcherSeen := map[[2]uint32]bool{}
		note := func(src dirac.Obj, card dirac.CardID, ab uint8) {
			key := [2]uint32{uint32(src), uint32(ab)}
			seen[key] = true
			if int(card) < len(tbl.Cards) && tbl.Cards[card].Name == "Aven Farseer" {
				watcherSeen[key] = true
			}
		}
		check := func(_ *GorgeSide, side Side) *Divergence {
			ks, ok := side.(*realSide)
			if !ok {
				return nil
			}
			s := &ks.s
			for i := 0; i < s.NumPending(); i++ {
				src, card, ab, _, _ := s.PendingTrigger(i)
				if kfacedownTurnFaceUpTrigger(tbl, card, ab) {
					note(src, card, ab)
				}
			}
			for i := 0; i < s.StackLen(); i++ {
				kind, src, card, _ := s.StackEntry(i)
				if kind != 2 {
					continue
				}
				if ab := s.StackAbility(i); kfacedownTurnFaceUpTrigger(tbl, card, ab) {
					note(src, card, ab)
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
		CloseSide(k)
		counts.faceUpTriggers += len(seen)
		counts.watcher += len(watcherSeen)
		for _, a := range res.Actions {
			switch {
			case a.Kind() == ActCastFace && a.Mode() == faceCodeMorphed:
				counts.morph++
			case a.Kind() == ActCastFace && a.Mode() == faceCodeMegamorphed:
				counts.megamorph++
			case a.Kind() == ActCastFace && a.Mode() == faceCodeDisguised:
				counts.disguise++
			case a.Kind() == ActSpecial && a.Mode() == kfacedownTurnUp:
				counts.turnUp++
			}
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("counts %+v down=%v up=%v watcher=%d", counts, counts.downBy, counts.upBy, counts.watcher)
	if counts.morph == 0 || counts.megamorph == 0 || counts.disguise == 0 {
		t.Errorf("face-down casts: %+v", counts)
	}
	if counts.turnUp == 0 {
		t.Errorf("no turn-face-up special action: %+v", counts)
	}
	if counts.faceUpTriggers == 0 {
		t.Errorf("no turn-face-up trigger: %+v", counts)
	}
	for _, n := range []string{"Kolaghan Stormsinger", "Bolrac-Clan Basher", "Basilica Stalker", "Zoetic Cavern"} {
		if counts.down(n) == 0 {
			t.Errorf("%s was never cast face down (down=%v)", n, counts.downBy)
		}
	}
	if counts.up("Kolaghan Stormsinger")+counts.up("Zoetic Cavern") == 0 {
		t.Errorf("no Morph carrier was turned face up (up=%v)", counts.upBy)
	}
	if counts.up("Basilica Stalker")+counts.up("Bolrac-Clan Basher")+counts.up("Concealed Weapon") == 0 {
		t.Errorf("no Disguise carrier was turned face up (up=%v)", counts.upBy)
	}
	if counts.watcher == 0 {
		t.Errorf("Aven Farseer's turn-face-up trigger never fired (face-up triggers=%d)", counts.faceUpTriggers)
	}
}

// TestFacedownMutation: with the face-down offers switched off
// (dirac.DebugIgnoreFaceDown) the same games diverge from gorge: the
// lockstep sees the face-down cast and turn-face-up paths.
func TestFacedownMutation(t *testing.T) {
	reg, set, decks, tbl := kfacedownEnv(t)
	dirac.DebugIgnoreFaceDown = true
	defer func() { dirac.DebugIgnoreFaceDown = false }()
	seeds := uint64(16)
	if testing.Short() {
		seeds = 4
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kf-a", "kf-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var counts kfacedownCounts
		var setup kfacedownSetup
		res := Lockstep(m, seed, k, kfacedownPolicy(seed, &counts, &setup), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("seed %d: mutation diverges at decision %d (%s %s)", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("ignoring face-down casts/turn-ups diverged in no game: the lockstep does not see the face-down path")
}

// TestFacedownProjection: the kernel's seat view hides a face-down
// permanent's identity from its opponent exactly as gorge's view does
// (CR 708.2): the controller still reads the printed card (FaceDown set),
// the opponent sees a blank face-down CardView. A face-down spell on the
// stack is redacted the same way (CR 708.4): the caster sees the printed
// band, the opponent sees no name and no card.
func TestFacedownProjection(t *testing.T) {
	t.Parallel()

	reg, set, decks, tbl := kfacedownEnv(t)
	seeds := uint64(8)
	if testing.Short() {
		seeds = 2
	}
	total, stackTotal := 0, 0
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kf-a", "kf-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		cs := NewSBTCards(reg, tbl, nil)
		ag := NewSBT(cs)
		ag.NewGame([2]uint64{SeatSeed(seed, 0), SeatSeed(seed, 1)})
		checked, stackChecked := 0, 0
		var bad string
		check := func(_ *GorgeSide, side Side) *Divergence {
			ks, ok := side.(*realSide)
			if !ok {
				return nil
			}
			ag.rs.kp = ks.kp
			for i := 0; i < int(ks.kp.NObj); i++ {
				po := &ks.kp.Objs[i]
				if po.Zone != kop.ZoneBattlefield || po.Flags&dirac.ProjFaceDown == 0 {
					continue
				}
				checked++
				id := sbtObjID(uint8(i))
				for seat := uint8(0); seat < 2; seat++ {
					v := ag.buildView(&ks.s, seat, nil)
					var cv *view.CardView
					bf := v.Players[po.Ctrl&1].Battlefield
					for j := range bf {
						if bf[j].ID == id {
							cv = &bf[j]
							break
						}
					}
					if cv == nil {
						bad = fmt.Sprintf("seed %d object %d: no card view for seat %d", seed, i, seat)
						return nil
					}
					if !cv.FaceDown {
						bad = fmt.Sprintf("seed %d object %d seat %d: FaceDown false", seed, i, seat)
						return nil
					}
					if seat == po.Ctrl {
						if cv.Name == "" {
							bad = fmt.Sprintf("seed %d object %d: the controller's view hides the face", seed, i)
							return nil
						}
						continue
					}
					if cv.Name != "" || cv.CardName != "" || cv.Types != "" || cv.ManaCost != "" ||
						cv.SpellAPI != "" || cv.Power != 0 || cv.Toughness != 0 {
						bad = fmt.Sprintf("seed %d object %d: the opponent's view leaks %+v", seed, i, *cv)
						return nil
					}
				}
			}
			for i := 0; i < int(ks.kp.NStack); i++ {
				e := &ks.kp.Stack[i]
				if e.Kind != 0 || int(e.Source) >= int(ks.kp.NObj) {
					continue
				}
				po := &ks.kp.Objs[e.Source]
				if po.Flags&dirac.ProjFaceDown == 0 {
					continue
				}
				stackChecked++
				id := sbtObjID(uint8(e.Source))
				for seat := uint8(0); seat < 2; seat++ {
					v := ag.buildView(&ks.s, seat, nil)
					if i >= len(v.Stack) {
						bad = fmt.Sprintf("seed %d stack %d: no stack view for seat %d", seed, i, seat)
						return nil
					}
					sv := &v.Stack[i]
					if sv.Kind != "spell" || sv.ID != id {
						bad = fmt.Sprintf("seed %d stack %d seat %d: kind %q id %d", seed, i, seat, sv.Kind, sv.ID)
						return nil
					}
					if seat != po.Ctrl {
						if sv.Name != "" || sv.Card != nil {
							bad = fmt.Sprintf("seed %d stack %d: the opponent's view leaks %+v", seed, i, *sv)
							return nil
						}
						continue
					}
					if sv.Name == "" || sv.Card == nil || sv.Card.Name == "" || !sv.Card.FaceDown {
						bad = fmt.Sprintf("seed %d stack %d: the caster's view hides the face: %+v", seed, i, *sv)
						return nil
					}
				}
			}
			return nil
		}
		var counts kfacedownCounts
		var setup kfacedownSetup
		res := Lockstep(m, seed, k, kfacedownPolicy(seed, &counts, &setup), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
		CloseSide(k)
		if bad != "" {
			t.Fatal(bad)
		}
		if res.Div != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
		}
		total += checked
		stackTotal += stackChecked
	}
	if total == 0 {
		t.Fatal("no face-down permanent reached a projection")
	}
	if stackTotal == 0 {
		t.Fatal("no face-down spell reached a stack projection")
	}
}

// kfacedownWardDeck adds Shock to the Disguise carriers: the ward lockstep
// casts it at a face-down Disguise (ward {2}), declines the ward, then turns
// a Disguise face up (the ward is gone).
var kfacedownWardDeck = []cardset.Entry{
	{Name: "Mox Emerald", Count: 3},
	{Name: "Mox Sapphire", Count: 3},
	{Name: "Mox Jet", Count: 3},
	{Name: "Mox Ruby", Count: 3},
	{Name: "Mox Pearl", Count: 3},
	{Name: "Priest of Urabrask", Count: 4},
	{Name: "Priest of Gix", Count: 4},
	{Name: "Basilica Stalker", Count: 3},
	{Name: "Bolrac-Clan Basher", Count: 3},
	{Name: "Shock", Count: 4},
	{Name: "Kolaghan Stormsinger", Count: 2},
	{Name: "Zoetic Cavern", Count: 1},
	{Name: "Concealed Weapon", Count: 4},
}

func kfacedownDisguiseName(n string) bool {
	return n == "Basilica Stalker" || n == "Bolrac-Clan Basher" || n == "Concealed Weapon"
}

// kfacedownCardName is the printed card name behind a canonical option:
// Face() hides a face-down permanent's name, but the oracle may read the
// physical card.
func kfacedownCardName(c *PickCtx, a Canon) string {
	if c.G == nil {
		return ""
	}
	o, ok := c.G.GorgeOption(a)
	if !ok {
		return ""
	}
	obj := o.Obj
	if a.Src() != RefNone && int(a.Src()) < len(c.G.id.obj) {
		if so := c.G.id.obj[a.Src()]; so != 0 {
			obj = so
		}
	}
	ob := c.G.E.G.Obj(obj)
	if ob == nil || ob.Card == nil || len(ob.Card.Faces) == 0 {
		return ""
	}
	return ob.Card.Faces[0].Name
}

// kfacedownOppDisguise reports whether the opponent controls a face-down
// Disguise permanent, the target the ward lockstep's Shock needs.
func kfacedownOppDisguise(c *PickCtx, p uint8) bool {
	if c.G == nil {
		return false
	}
	for _, o := range c.G.E.G.Zone(state.ZBattlefield, state.PlayerID(1-p)) {
		ob := c.G.E.G.Obj(o)
		if ob == nil || !ob.FaceDown || ob.Card == nil || len(ob.Card.Faces) == 0 {
			continue
		}
		if kfacedownDisguiseName(ob.Card.Faces[0].Name) {
			return true
		}
	}
	return false
}

// kfacedownDisguiseTarget picks the opponent's face-down Disguise for a
// Shock target decision.
func kfacedownDisguiseTarget(c *PickCtx) (Canon, bool) {
	for _, a := range c.D.Options {
		if a.Kind() != ActTarget {
			continue
		}
		o, ok := c.G.GorgeOption(a)
		if !ok || o.Obj == 0 {
			continue
		}
		ob := c.G.E.G.Obj(o.Obj)
		if ob == nil || !ob.FaceDown || ob.Controller == state.PlayerID(c.D.Player) || ob.Card == nil || len(ob.Card.Faces) == 0 {
			continue
		}
		if kfacedownDisguiseName(ob.Card.Faces[0].Name) {
			return a, true
		}
	}
	return 0, false
}

type kfacedownWardSetup struct {
	moxen, ruby, priests, down, up [2]int
	upDisguise                     int
}

// kfacedownWardPolicy floats mana with the Moxen and Priests, casts a
// Disguise face down, Shocks it, declines the ward, then turns it face up.
// It declines every two-option ActMode ask (ward is canonical mode 0).
func kfacedownWardPolicy(seed uint64, setup *kfacedownWardSetup) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DChoose && len(c.D.Options) == 2 &&
			c.D.Options[0] == Pack(ActMode, RefNone, RefNone, 0) &&
			c.D.Options[1] == Pack(ActMode, RefNone, RefNone, 1) {
			return c.D.Options[0] // decline the ward
		}
		if c.D.Kind == DTarget {
			if a, ok := kfacedownDisguiseTarget(c); ok {
				return a
			}
		}
		p := c.D.Player
		var (
			mox, ruby, priest, down, up                     Canon
			shock                                           Canon
			haveMox, haveRuby, havePriest, haveDown, haveUp bool
			haveShock                                       bool
		)
		for _, a := range c.D.Options {
			n := kfacedownCardName(c, a)
			switch {
			case a.Kind() == ActCast && n == "Shock":
				shock, haveShock = a, true
			case a.Kind() == ActCast && n == "Mox Ruby":
				ruby, haveRuby = a, true
			case a.Kind() == ActCast && kfacedownMox(n):
				mox, haveMox = a, true
			case a.Kind() == ActCast && (n == "Priest of Urabrask" || n == "Priest of Gix"):
				priest, havePriest = a, true
			case a.Kind() == ActCastFace && kfacedownDisguiseName(n):
				down, haveDown = a, true
			case a.Kind() == ActSpecial && a.Mode() == kfacedownTurnUp && kfacedownDisguiseName(n):
				up, haveUp = a, true
			}
		}
		if haveShock && kfacedownOppDisguise(c, p) {
			return shock
		}
		switch {
		case haveRuby && setup.ruby[p] < 2:
			setup.ruby[p]++
			return ruby
		case haveMox && setup.moxen[p] < 8:
			setup.moxen[p]++
			return mox
		case havePriest:
			setup.priests[p]++
			return priest
		case haveUp && setup.up[p] < 1:
			setup.up[p]++
			setup.upDisguise++
			return up
		case haveDown && setup.moxen[p] >= 4 && setup.down[p] < 1:
			setup.down[p]++
			return down
		}
		return rnd.Pick(c)
	})
}

// TestFacedownWardLockstep covers the Disguise ward end to end: the pool
// carries Shock, the policy Shocks a face-down Disguise and the kernel
// triggers abWard with amount 2; a Disguise turn-up follows and the ward is
// gone. The layer-6 suppression (Humility) is the kernel unit test
// TestFaceDownDisguiseWardSuppressed.
func TestFacedownWardLockstep(t *testing.T) {
	t.Parallel()

	reg, set, decks, tbl := kfacedownDeckEnv(t, kfacedownWardDeck)
	seeds := uint64(48)
	if testing.Short() {
		seeds = 8
	}
	var disguiseDown, turnUp, ward, wardAmt2, upDisguise int
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kfw-a", "kfw-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		check := func(_ *GorgeSide, side Side) *Divergence {
			ks, ok := side.(*realSide)
			if !ok {
				return nil
			}
			s := &ks.s
			for i := 0; i < s.NumPending(); i++ {
				_, _, ab, _, _ := s.PendingTrigger(i)
				if ab != dirac.WardAbility {
					continue
				}
				ward++
				if s.PendingTriggerAmount(i) == 2 {
					wardAmt2++
				}
			}
			for i := 0; i < s.StackLen(); i++ {
				kind, _, _, _ := s.StackEntry(i)
				if kind != 2 || s.StackAbility(i) != dirac.WardAbility {
					continue
				}
				ward++
				if s.StackTriggerAmount(i) == 2 {
					wardAmt2++
				}
			}
			return nil
		}
		var setup kfacedownWardSetup
		res := Lockstep(m, seed, k, kfacedownWardPolicy(seed, &setup), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
		upDisguise += setup.upDisguise
		CloseSide(k)
		for _, a := range res.Actions {
			switch {
			case a.Kind() == ActCastFace && a.Mode() == faceCodeDisguised:
				disguiseDown++
			case a.Kind() == ActSpecial && a.Mode() == kfacedownTurnUp:
				turnUp++
			}
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("ward counts disguise-down=%d turn-up=%d ward=%d amount2=%d upDisguise=%d", disguiseDown, turnUp, ward, wardAmt2, upDisguise)
	if disguiseDown == 0 {
		t.Error("no Disguise permanent was cast face down")
	}
	if ward == 0 || wardAmt2 == 0 {
		t.Errorf("Disguise ward did not trigger with amount 2 (ward=%d amount2=%d)", ward, wardAmt2)
	}
	if turnUp == 0 || upDisguise == 0 {
		t.Errorf("no Disguise carrier was turned face up (turn-up=%d upDisguise=%d)", turnUp, upDisguise)
	}
}
