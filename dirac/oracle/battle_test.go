package oracle

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Battle lockstep (CR 310): constructed matchups over the generated
// battle pool. The decks carry every arm:
//
//   - a Siege enters with defense counters and the sole opponent as its
//     protector (H1: no protector ask in two players);
//   - creatures attack the unprotected battle, combat damage removes defense
//     and is not marked, a block stops the battle damage;
//   - Shock/Lightning Strike damage a battle, including the active player's
//     own;
//   - the defeat SBA exiles it and poses the owner's transformed free cast
//     (defeat_cast_yes/no), a permanent back face enters and a sorcery back
//     face (Kylem) reaches the graveyard at face 1 (H4).
//
// The counters come from gorge's event log and option lists. The mutations
// break one rule each and must diverge.
var battleDecks = [2][]cardset.Entry{
	{
		{Name: "Invasion of Ergamon", Count: 1}, {Name: "Invasion of Ixalan", Count: 1},
		{Name: "Invasion of Mercadia", Count: 1}, {Name: "Invasion of Karsus", Count: 1},
		{Name: "Invasion of Kylem", Count: 4}, {Name: "Invasion of Moag", Count: 1},
		{Name: "Invasion of Zendikar", Count: 1}, {Name: "Invasion of Muraganda", Count: 1},
		{Name: "Savannah Lions", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Hill Giant", Count: 2}, {Name: "Gray Ogre", Count: 2},
		{Name: "Giant Spider", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Plains", Count: 5},
	},
	{
		{Name: "Invasion of Dominaria", Count: 2}, {Name: "Invasion of Belenon", Count: 1},
		{Name: "Invasion of Innistrad", Count: 2}, {Name: "Invasion of Xerex", Count: 1},
		{Name: "Invasion of Tolvada", Count: 2}, {Name: "Invasion of Azgol", Count: 2},
		{Name: "Savannah Lions", Count: 2}, {Name: "Coral Merfolk", Count: 2},
		{Name: "Walking Corpse", Count: 2}, {Name: "Gray Ogre", Count: 2},
		{Name: "Hill Giant", Count: 2}, {Name: "Lightning Strike", Count: 2},
		{Name: "Plains", Count: 6}, {Name: "Island", Count: 4},
		{Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 3},
	},
}

// battleTally counts the arms. The event cursor mirrors kprepare's: the log
// only grows, but resets when a side is rebuilt.
type battleTally struct {
	evAt int

	def                   map[state.ObjID]int   // live DEFENSE counters by CounterChange
	face                  map[state.ObjID]int   // active face by FlipFace
	attacked              map[state.ObjID]bool  // attacked this combat (H3)
	left                  map[state.ObjID]bool  // left the battlefield after being attacked (H3)
	entry                 map[state.ObjID]int   // expected DEFENSE of a battle that just entered
	pending               map[state.ObjID]int32 // combat damage to a battle awaiting its CounterChange
	prevDefeat, prevOther bool                  // adjacent SBA-batch leave events (H7)

	pendingDefeat state.ObjID // the defeat ask seen at the last check
	yesObj        state.ObjID // its yes answer, awaiting PutOnStack

	entered, entryBad, protBad int
	protectorAsks              int // H1: must be 0
	protectedFrames            int
	protectedOffered           int // must be 0
	battleAttacked             int
	combatHits, combatBad      int
	spellHits                  int
	blockedBattle              int
	defeated                   int
	castTransformed            int
	backEntered                int
	sorceryGrave               int
	declined                   int
	declineBad                 int
	h3, h6, h7                 int
	maxMay                     int
}

func (p *battleTally) reset() {
	p.evAt = 0
	p.def = map[state.ObjID]int{}
	p.face = map[state.ObjID]int{}
	p.attacked = map[state.ObjID]bool{}
	p.left = map[state.ObjID]bool{}
	p.entry = map[state.ObjID]int{}
	p.pending = map[state.ObjID]int32{}
	p.prevDefeat, p.prevOther = false, false
	p.pendingDefeat, p.yesObj = 0, 0
}

func battleFace(o *state.Object) *cards.Face {
	if o == nil || o.Card == nil || len(o.Card.Faces) == 0 {
		return nil
	}
	return o.Card.Faces[0]
}

// hasLeaveWatcher reports a battlefield permanent with a leave-the-battlefield
// trigger, the only thing H7's SBA batching could perturb.
func hasLeaveWatcher(g *state.Game) bool {
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone != state.ZBattlefield || o.Card == nil {
			continue
		}
		for _, f := range o.Card.Faces {
			if f == nil {
				continue
			}
			for _, tr := range f.Triggers {
				if tr.Params["Mode"] == "ChangesZone" && tr.Params["Origin"] == "Battlefield" {
					return true
				}
			}
		}
	}
	return false
}

func (p *battleTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.reset()
	}
	for ; p.evAt < len(evs); p.evAt++ {
		e := &evs[p.evAt]
		switch e.Kind {
		case events.MoveZone:
			isDefeat := e.From == state.ZBattlefield && e.To == state.ZExile && e.Text == "defeated"
			isOther := false
			if e.From == state.ZBattlefield && e.To != state.ZBattlefield {
				if o := g.Obj(e.Obj); o != nil && o.Face() != nil &&
					(o.Face().IsCreature() || o.Face().IsPlaneswalker()) {
					isOther = true
				}
			}
			if (isDefeat && p.prevOther || isOther && p.prevDefeat) && hasLeaveWatcher(g) {
				p.h7++ // a battle and another permanent left in the same batch
			}
			p.prevDefeat, p.prevOther = isDefeat, isOther
			if e.From == state.ZBattlefield {
				delete(p.def, e.Obj)
				if p.attacked[e.Obj] {
					p.left[e.Obj] = true
				}
			}
			if e.To == state.ZBattlefield {
				p.def[e.Obj] = 0
				delete(p.left, e.Obj)
				if f := battleFace(g.Obj(e.Obj)); f != nil && f.IsBattle() {
					want, _ := strconv.Atoi(strings.TrimSpace(f.Defense))
					p.entry[e.Obj] = want
				}
			}
			if isDefeat {
				p.defeated++
			}
			if e.To == state.ZGraveyard {
				if o := g.Obj(e.Obj); o != nil && o.Card != nil && len(o.Card.Faces) > 1 &&
					o.Card.Faces[1] != nil && o.Card.Faces[1].IsSorcery() && p.face[e.Obj] == 1 {
					p.sorceryGrave++
				}
			}
			if e.To == state.ZBattlefield && p.face[e.Obj] == 1 {
				if f := battleFace(g.Obj(e.Obj)); f != nil && f.IsBattle() {
					p.backEntered++
				}
			}
		case events.CounterChange:
			p.prevDefeat, p.prevOther = false, false
			if e.Counter == "DEFENSE" {
				p.def[e.Obj] += int(e.Amount)
				if e.Amount > 0 {
					if want, ok := p.entry[e.Obj]; ok {
						if p.def[e.Obj] == want {
							p.entered++
							delete(p.entry, e.Obj)
						} else if p.def[e.Obj] > want {
							p.entryBad++
							delete(p.entry, e.Obj)
						}
					}
				} else if e.Amount < 0 {
					if amt, ok := p.pending[e.Obj]; ok {
						if o := g.Obj(e.Obj); o == nil || -e.Amount != amt || o.Damage != 0 {
							p.combatBad++
						}
						delete(p.pending, e.Obj)
					}
				}
			}
		case events.FlipFace:
			p.prevDefeat, p.prevOther = false, false
			p.face[e.Obj] = int(e.Amount)
		case events.Choose:
			p.prevDefeat, p.prevOther = false, false
			if e.Counter == "protector" {
				o := g.Obj(e.Obj)
				if f := battleFace(o); f != nil && f.IsBattle() {
					if o != nil && int(e.Player) != 1-int(o.Controller) {
						p.protBad++
					}
				}
			}
		case events.DeclareAttackers:
			p.prevDefeat, p.prevOther = false, false
			if e.Obj != 0 {
				if o := g.Obj(e.Obj); o != nil && battleFace(o) != nil && battleFace(o).IsBattle() {
					p.battleAttacked++
					p.attacked[e.Obj] = true
				}
			}
		case events.DeclareBlockers:
			p.prevDefeat, p.prevOther = false, false
			for _, pr := range e.Pairs {
				if a := g.Obj(pr[0]); a != nil && a.AttackingBattle != 0 {
					p.blockedBattle++
				}
			}
		case events.DamageProvenance:
			p.prevDefeat, p.prevOther = false, false
			if e.Amount <= 0 || len(e.IDs) == 0 {
				continue
			}
			o := g.Obj(e.IDs[0])
			f := battleFace(o)
			if strings.HasPrefix(e.Text, events.DamageProvenanceCombat) && o != nil &&
				o.Zone == state.ZBattlefield && p.attacked[e.IDs[0]] &&
				(f == nil || !f.IsBattle()) {
				p.h3++ // regular combat damage to an attacked battle cast back (H3)
			}
			if f == nil || !f.IsBattle() {
				continue
			}
			if strings.HasPrefix(e.Text, events.DamageProvenanceCombat) {
				p.combatHits++
				p.pending[e.IDs[0]] = e.Amount
			} else if src := g.Obj(e.Obj); src != nil && src.Face() != nil &&
				(src.Zone == state.ZStack || src.Face().IsInstant() || src.Face().IsSorcery()) {
				p.spellHits++
			}
		case events.PutOnStack:
			p.prevDefeat, p.prevOther = false, false
			if e.Obj == p.yesObj {
				if p.face[e.Obj] == 1 {
					p.castTransformed++
					if o := g.Obj(e.Obj); o != nil && o.Owner != g.Active {
						p.h6++
					}
				}
				p.yesObj = 0
			}
		case events.DecisionMade:
			p.prevDefeat, p.prevOther = false, false
			if p.pendingDefeat == 0 {
				continue
			}
			id := p.pendingDefeat
			p.pendingDefeat = 0
			if !strings.HasPrefix(e.Text, "choose:[") {
				continue
			}
			rest := e.Text[len("choose:["):]
			n, _ := strconv.Atoi(rest[:strings.IndexByte(rest, ']')])
			if n == 0 {
				p.yesObj = id
			} else {
				p.declined++
				if o := g.Obj(id); o == nil || o.Zone != state.ZExile || o.FaceIdx != 0 {
					p.declineBad++
				}
			}
		case events.EndCombatReset:
			p.prevDefeat, p.prevOther = false, false
			p.attacked = map[state.ObjID]bool{}
			p.left = map[state.ObjID]bool{}

		default:
			p.prevDefeat, p.prevOther = false, false
		}
	}
	fr := ref.fr
	if fr == nil || fr.d == nil {
		return nil
	}
	if fr.d.Kind == "choose" {
		for _, o := range fr.d.Options {
			if o.Kind == "protector" {
				p.protectorAsks++
			}
		}
	}
	if fr.d.Kind == "attackers" {
		actor := fr.d.Player
		protected := map[state.ObjID]bool{}
		for i := range g.Objs {
			o := &g.Objs[i]
			f := battleFace(o)
			if o.Zone == state.ZBattlefield && f != nil && f.IsBattle() &&
				o.ProtectorValid && o.Protector == actor && o.Controller != actor {
				protected[o.ID] = true
			}
		}
		if len(protected) > 0 {
			p.protectedFrames++
		}
		for _, o := range fr.d.Options {
			if o.Kind == "attacker" && protected[o.Battle] {
				p.protectedOffered++
			}
		}
	}
	if fr.d.Kind == "choose" {
		for _, o := range fr.d.Options {
			if o.Kind == "defeat_cast_yes" {
				p.pendingDefeat = o.Obj
			}
		}
	}
	return nil
}

// battlePolicy prefers the battle arms: cast Sieges, attack battles, block
// battle attackers, burn the active player's own battle, and answer the
// defeat ask yes on most seeds (no on every third).
func battlePolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	yes, _ := chooseCode("defeat_cast_yes")
	no, _ := chooseCode("defeat_cast_no")
	defeats := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, a := range c.D.Options {
			if a.Kind() == ActChoose+CanonKind(yes) {
				defeats++
				if defeats%3 == 0 {
					for _, b := range c.D.Options {
						if b.Kind() == ActChoose+CanonKind(no) {
							return b
						}
					}
				}
				return a
			}
		}
		// attackers: a battle target first, else attack the player
		var anyAtk Canon
		haveAtk := false
		for _, a := range c.D.Options {
			if a.Kind() != ActAttack {
				continue
			}
			if int(a.Tgt()) < int(RefPlayer) {
				return a
			}
			if !haveAtk {
				anyAtk, haveAtk = a, true
			}
		}
		if haveAtk {
			return anyAtk
		}
		// blockers: block an attacker that is attacking a battle
		if c.G != nil {
			for _, a := range c.D.Options {
				if a.Kind() != ActBlock {
					continue
				}
				if o := c.G.E.G.Obj(c.G.id.obj[a.Tgt()]); o != nil && o.AttackingBattle != 0 {
					return a
				}
			}
		}
		// targets: the active player's own battle first, then any battle
		if c.G != nil {
			for pass := 0; pass < 2; pass++ {
				for _, a := range c.D.Options {
					if a.Kind() != ActTarget || int(a.Tgt()) >= int(RefPlayer) {
						continue
					}
					o := c.G.E.G.Obj(c.G.id.obj[a.Tgt()])
					f := battleFace(o)
					if f == nil || !f.IsBattle() {
						continue
					}
					if pass == 0 && o.Controller != state.PlayerID(c.D.Player) {
						continue
					}
					return a
				}
			}
		}
		// priority: cast a Siege, else the burn
		var castBattle, castBurn Canon
		haveB, haveBurn := false, false
		for _, a := range c.D.Options {
			if a.Kind() != ActCast && a.Kind() != ActCastFace {
				continue
			}
			if c.G == nil || int(a.Src()) >= len(c.G.id.obj) {
				continue
			}
			o := c.G.E.G.Obj(c.G.id.obj[a.Src()])
			if o == nil || o.Face() == nil {
				continue
			}
			if o.Face().IsBattle() && !haveB {
				castBattle, haveB = a, true
			}
			if (o.Face().Name == "Shock" || o.Face().Name == "Lightning Strike") && !haveBurn {
				castBurn, haveBurn = a, true
			}
		}
		if haveB {
			return castBattle
		}
		if haveBurn {
			return castBurn
		}
		var buf []Canon
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActCastFace || k == ActActivate || k == ActPlayLand {
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

func battleRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, cfg Config, check func(*GorgeSide, Side) *Divergence) Result {
	t.Helper()
	m, err := NewMatchup(reg, [2]string{"battle-a", "battle-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	lead, err := NewGorgeSide(m, seed, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	pol := battlePolicy(seed)
	if seed%4 == 0 {
		pol = RandomPolicy(seed)
	}
	res := LockstepLed(m, seed, lead, k, pol, RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 80, Check: check})
	if res.Div != nil && os.Getenv("KBATTLE_DEBUG") != "" {
		if gs := lead; gs != nil {
			evs := gs.E.L.Events
			lo := len(evs) - 40
			if lo < 0 {
				lo = 0
			}
			for i := lo; i < len(evs); i++ {
				t.Logf("EV %d %+v", i, evs[i])
			}
			rf := reflect.ValueOf(gs.E).Elem().FieldByName("defeatedCasts")
			t.Logf("GORGE defeatedCasts len %d", rf.Len())
		}
		if gs := lead; gs != nil {
			var gp Projection
			gs.Project(&gp)
			t.Logf("PROJ inst1 ref zone=%d card=%d | gorge oid=%d", gp.Cards[1].Zone, gp.Cards[1].Card, gs.id.obj[1])
		}
		if ks, ok := k.(*realSide); ok {
			var op Projection
			ks.Project(&op)
			t.Logf("PROJ inst1 ker zone=%d card=%d | NObj=%d", op.Cards[1].Zone, op.Cards[1].Card, op.NInst)
			ks.s.Project(&ks.kp)
			for i := 0; i < int(ks.kp.NObj); i++ {
				po := &ks.kp.Objs[i]
				cd := &ks.tbl.Cards[po.Card]
				if cd.Types&kop.TBattle != 0 || po.Zone == 4 {
					t.Logf("KER battle obj %d %s zone=%d face=%d def=%d dmg=%d", i, cd.Name, po.Zone, cd.Face, po.CountersX[kop.CtrDefense-kop.CtrCharge], po.Damage)
				}
			}
		}
	}
	CloseSide(k)
	return res
}

func battleSetup(t *testing.T) (*cards.Registry, cardset.Set, [][2][]*cards.Card, []*kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("battle")
	if err != nil {
		t.Fatal(err)
	}
	matchups := [][2][]cardset.Entry{{battleDecks[0], battleDecks[1]}, {battleDecks[1], battleDecks[0]}}
	decks := make([][2][]*cards.Card, len(matchups))
	tbls := make([]*kop.Table, len(matchups))
	for mi, md := range matchups {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[mi][s] = append(decks[mi][s], c)
				}
			}
			if len(decks[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[mi][s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		tbls[mi] = tbl
	}
	return reg, set, decks, tbls
}

const battleSeeds = 100

func TestBattleLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	reg, set, decks, tbls := battleSetup(t)
	var p battleTally
	p.reset()
	prevHook := OccupancyHook
	OccupancyHook = func(s *dirac.State) {
		if n := s.FeatMayCount(); n > p.maxMay {
			p.maxMay = n
		}
	}
	defer func() { OccupancyHook = prevHook }()
	games := 0
	onlySeed := os.Getenv("KBATTLE_SEED")
	onlyMatch := os.Getenv("KBATTLE_MATCHUP")
	for mi := range decks {
		if onlyMatch != "" && onlyMatch != strconv.Itoa(mi) {
			continue
		}
		for seed := uint64(1); seed <= battleSeeds; seed++ {
			if onlySeed != "" && onlySeed != strconv.FormatUint(seed, 10) {
				continue
			}
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			p.reset()
			res := battleRun(t, reg, set, decks[mi], tbls[mi], seed, cfg, p.check)
			games++
			if res.Stall != "" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games: entered %d (defense/entry errors %d, protector errors %d, protector asks %d); attacked %d, protected frames %d (offered %d); combat hits %d (bad %d), spell hits %d, blocked battle attackers %d; defeated %d, casts transformed %d, back permanents %d, sorcery graves %d, declined %d (bad %d); H3 %d H6 %d H7 %d; max nmay %d",
		games, p.entered, p.entryBad, p.protBad, p.protectorAsks, p.battleAttacked, p.protectedFrames, p.protectedOffered,
		p.combatHits, p.combatBad, p.spellHits, p.blockedBattle, p.defeated, p.castTransformed, p.backEntered,
		p.sorceryGrave, p.declined, p.declineBad, p.h3, p.h6, p.h7, p.maxMay)
	for _, c := range []struct {
		name string
		n    int
	}{
		{"entered battles", p.entered}, {"battles attacked", p.battleAttacked},
		{"protected battle frames", p.protectedFrames}, {"combat damage to a battle", p.combatHits},
		{"blocked battle attackers", p.blockedBattle}, {"spell damage to a battle", p.spellHits},
		{"defeated battles", p.defeated}, {"transformed casts", p.castTransformed},
		{"back permanents entered", p.backEntered}, {"sorcery back faces in the graveyard", p.sorceryGrave},
		{"declined casts", p.declined},
	} {
		if c.n < 10 {
			t.Errorf("%d %s, want at least 10", c.n, c.name)
		}
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"protector asks", p.protectorAsks}, {"entry defense errors", p.entryBad},
		{"protector errors", p.protBad}, {"protected battle offered", p.protectedOffered},
		{"combat damage errors", p.combatBad}, {"declined card not in exile face 0", p.declineBad},
		{"H3 same-combat leaves", p.h3}, {"H7 mixed SBA batches", p.h7},
	} {
		if c.n != 0 {
			t.Errorf("%d %s, want 0", c.n, c.name)
		}
	}
}

// TestBattleMutation: each mutation must diverge on some seed.
func TestBattleMutation(t *testing.T) {
	reg, set, decks, tbls := battleSetup(t)
	muts := []struct {
		name string
		on   *bool
	}{
		{"protector is the controller", &dirac.DebugBattleOwnProtector},
		{"battle damage is marked", &dirac.DebugBattleMarkDamage},
		{"the defeat record is not appended", &dirac.DebugBattleNoDefeatAsk},
	}
	for _, mu := range muts {
		*mu.on = true
		diverged := false
		for mi := range decks {
			for seed := uint64(1); seed <= battleSeeds && !diverged; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				res := battleRun(t, reg, set, decks[mi], tbls[mi], seed, cfg, nil)
				if diverged = res.Div != nil; diverged {
					t.Logf("mutation %q: matchup %d seed %d diverged at decision %d: %s %s", mu.name, mi, seed, res.Div.At, res.Div.What, res.Div.Field)
				}
			}
		}
		*mu.on = false
		if !diverged {
			t.Errorf("mutation %q never diverged", mu.name)
		}
	}
}
