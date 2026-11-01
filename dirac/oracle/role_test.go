package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated roles pool.
// WOE's Role tokens are created with AttachedTo$ -- an own target, the chain's
// single target, a returned card, or Self -- and the attach-time exclusivity
// sweep puts the old Role into its owner's graveyard. The counters read
// gorge's event log: role TokenCreate, Attach (paired with the carrier that
// minted it), the sweep's MoveZone Text, the CR 303.4g Note, the 704.5m host
// departure, CounterChange, Scry and combat Damage.

var kroleDecks = [][2][]cardset.Entry{{
	{
		{Name: "Monstrous Rage", Count: 4}, {Name: "Embereth Veteran", Count: 3},
		{Name: "Merry Bards", Count: 3}, {Name: "Cut In", Count: 2},
		{Name: "Twisted Fealty", Count: 2}, {Name: "Return Triumphant", Count: 3},
		{Name: "Cursed Courtier", Count: 3}, {Name: "Unassuming Sage", Count: 2},
		{Name: "Spellbook Vendor", Count: 2}, {Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Spiteful Hexmage", Count: 3}, {Name: "Lord Skitter's Blessing", Count: 2},
		{Name: "Charming Scoundrel", Count: 3}, {Name: "Eriette's Whisper", Count: 2},
		{Name: "Shatter the Oath", Count: 2}, {Name: "Witch's Mark", Count: 2},
		{Name: "The Witch's Vanity", Count: 2}, {Name: "Monstrous Rage", Count: 3},
		{Name: "Embereth Veteran", Count: 3}, {Name: "Shock", Count: 2},
		{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8},
	},
}, {
	{
		{Name: "Syr Armont, the Redeemer", Count: 2}, {Name: "Ferocious Werefox", Count: 3},
		{Name: "Living Lectern", Count: 3}, {Name: "Splashy Spellcaster", Count: 3},
		{Name: "Vantress Transmuter", Count: 3}, {Name: "Protective Parents", Count: 3},
		{Name: "Conceited Witch", Count: 2}, {Name: "Opt", Count: 3},
		{Name: "Forest", Count: 6}, {Name: "Island", Count: 6}, {Name: "Plains", Count: 6},
	},
	{
		{Name: "Monstrous Rage", Count: 4}, {Name: "Embereth Veteran", Count: 3},
		{Name: "Merry Bards", Count: 3}, {Name: "Cut In", Count: 2},
		{Name: "Twisted Fealty", Count: 2}, {Name: "Return Triumphant", Count: 3},
		{Name: "Cursed Courtier", Count: 3}, {Name: "Unassuming Sage", Count: 2},
		{Name: "Spellbook Vendor", Count: 2}, {Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8},
	},
}}

// kroleCarriers classifies each Role maker by the destination shape its Token
// node reads (three read shapes plus the "up to one" and
// any-creature variants the eager policy ranks).
var (
	kroleOwn = map[string]bool{
		"Embereth Veteran": true, "Merry Bards": true, "Cut In": true,
		"Twisted Fealty": true, "Spiteful Hexmage": true, "Lord Skitter's Blessing": true,
		"Charming Scoundrel": true, "Witch's Mark": true, "The Witch's Vanity": true,
		"Syr Armont, the Redeemer": true, "Ferocious Werefox": true, "Living Lectern": true,
		"Splashy Spellcaster": true, "Protective Parents": true, "Conceited Witch": true,
		"Spellbook Vendor": true, "Shatter the Oath": true,
	}
	kroleChain = map[string]bool{
		"Monstrous Rage": true, "Return Triumphant": true,
		"Eriette's Whisper": true, "Vantress Transmuter": true,
	}
	kroleSelf        = map[string]bool{"Cursed Courtier": true, "Unassuming Sage": true}
	kroleAnyCreature = map[string]bool{"Monstrous Rage": true, "Embereth Veteran": true, "Twisted Fealty": true}
)

func kroleCarrier(name string) bool {
	return kroleOwn[name] || kroleChain[name] || kroleSelf[name]
}

func kroleIsRole(o *state.Object) bool {
	if o == nil || o.Face() == nil {
		return false
	}
	for _, t := range o.Face().Types {
		if t == "Role" {
			return true
		}
	}
	return false
}

// kroleOn reports whether obj on the battlefield carries a Role.
func kroleHasNamedRole(g *state.Game, bearer state.ObjID, name string) bool {
	for id := state.ObjID(1); int(id) < len(g.Objs); id++ {
		o := g.Obj(id)
		if o != nil && o.AttachedTo == bearer && o.Zone == state.ZBattlefield &&
			o.Face() != nil && o.Face().Name == name {
			return true
		}
	}
	return false
}

func kroleOn(g *state.Game, obj state.ObjID) bool {
	for id := state.ObjID(1); int(id) < len(g.Objs); id++ {
		o := g.Obj(id)
		if o != nil && o.AttachedTo == obj && o.Zone == state.ZBattlefield && kroleIsRole(o) {
			return true
		}
	}
	return false
}

// krolePolicy is Role-eager: it orders the legal options (casting and
// activating Role makers first, then attacking and blocking) and never
// removes one. On a zero-target seed it answers an "up to one target" Role
// ask with none; otherwise it targets a creature already carrying a Role,
// aims Shock at a Role spell's pending target and points Shatter the Oath's
// second slot at its first.
func krolePolicy(seed uint64, zero bool, p *kroleTally) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		g := c.G
		if zero && c.D.Kind == DTarget {
			for _, a := range c.D.Options {
				if a.Kind() == ActDone {
					p.zeroTargets++
					p.zeroAwait = true
					return a
				}
			}
		}
		if a, ok := kroleTarget(c); ok {
			return a
		}
		var roleMakers, attacks, rest []Canon
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActActivate:
				if g != nil {
					if o, ok := g.GorgeOption(a); ok {
						if ob := g.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && kroleCarrier(ob.Face().Name) {
							roleMakers = append(roleMakers, a)
							continue
						}
					}
				}
				rest = append(rest, a)
			case ActAttack, ActBlock, ActPlayLand:
				attacks = append(attacks, a)
			}
		}
		for _, set := range [][]Canon{roleMakers, attacks, rest} {
			if len(set) > 0 {
				d := *c.D
				d.Options = set
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		return rnd.Pick(c)
	})
}

// kroleRefObj maps a Canon target ref back to the gorge object.
func kroleRefObj(g *GorgeSide, r Ref) (state.ObjID, bool) {
	if r == RefNone || r >= RefPlayer {
		return 0, false
	}
	g.id.sync(g.E.G)
	if i := int(r); i < len(g.id.obj) {
		return g.id.obj[i], true
	}
	return 0, false
}

// kroleTarget picks among a target ask: the pending Role carrier's own or
// chain target, Shock's kill of that target, or Shatter the Oath's repeated
// slot.
func kroleTarget(c *PickCtx) (Canon, bool) {
	g := c.G
	if g == nil {
		return 0, false
	}
	var targets []Canon
	for _, a := range c.D.Options {
		if a.Kind() == ActTarget {
			targets = append(targets, a)
		}
	}
	if len(targets) == 0 {
		return 0, false
	}
	var pending []state.ObjID
	carrier := ""
	for _, id := range g.E.G.Stack {
		o := g.E.G.Obj(id)
		if o == nil || o.Face() == nil || !kroleCarrier(o.Face().Name) && o.Face().Name != "Shock" {
			continue
		}
		name := o.Face().Name
		if kroleCarrier(name) && carrier == "" {
			carrier = name
		}
		for _, t := range o.Targets {
			if t.Obj != 0 {
				pending = append(pending, t.Obj)
			}
		}
		for _, t := range o.SubTargets {
			if t.Obj != 0 {
				pending = append(pending, t.Obj)
			}
		}
	}
	pick := func(obj state.ObjID) (Canon, bool) {
		for _, a := range targets {
			if got, ok := kroleRefObj(g, a.Tgt()); ok && got == obj {
				return a, true
			}
		}
		return 0, false
	}
	// Shock in response to a Role spell: kill the pending target.
	if len(pending) > 0 {
		for _, a := range c.D.Options {
			if a.Kind() != ActCast {
				continue
			}
			if o, ok := g.GorgeOption(a); ok {
				if ob := g.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Shock" {
					if got, ok := pick(pending[0]); ok {
						return got, true
					}
				}
			}
		}
	}
	if carrier == "" {
		return 0, false
	}
	// Shatter the Oath: both slots on the same creature.
	if carrier == "Shatter the Oath" && len(pending) > 0 {
		if got, ok := pick(pending[0]); ok {
			return got, true
		}
	}
	// A creature already carrying a Role, and for the any-creature Role
	// makers an opponent's creature carrying its own Role first.
	if kroleAnyCreature[carrier] {
		opp := uint8(1) - c.D.Player
		for _, a := range targets {
			obj, ok := kroleRefObj(g, a.Tgt())
			if !ok || !kroleOn(g.E.G, obj) {
				continue
			}
			if o := g.E.G.Obj(obj); o != nil && uint8(o.Controller) == opp {
				return a, true
			}
		}
	}
	for _, a := range targets {
		if obj, ok := kroleRefObj(g, a.Tgt()); ok && kroleOn(g.E.G, obj) {
			return a, true
		}
	}
	return 0, false
}

// kroleTally counts every branch from gorge's log.
type kroleTally struct {
	evAt        int
	carrier     string // the spell or trigger source now resolving
	mintPending bool
	wickedLeft  bool
	zeroAwait   bool
	lastCtl     uint8 // the last Attach's Role controller
	rolesOn     map[state.ObjID]map[state.ObjID]bool

	roleMints, ownAttach, chainAttach, returnedAttach, selfAttach int
	zeroTargets, bearerGone                                       int
	sameSweep, crossSweep, wickedLeave, wickedLoss, hostGone      int
	yhCounter, sorcScry, cursedCombat                             int
	orphan, zeroMinted, maxRoles                                  int
}

func newKroleTally() *kroleTally {
	return &kroleTally{rolesOn: map[state.ObjID]map[state.ObjID]bool{}}
}

func (p *kroleTally) detach(role state.ObjID) {
	for bearer, set := range p.rolesOn {
		delete(set, role)
		if len(set) == 0 {
			delete(p.rolesOn, bearer)
		}
	}
}

func (p *kroleTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.GameStart:
			p.rolesOn = map[state.ObjID]map[state.ObjID]bool{}
			p.carrier, p.lastCtl = "", 0
			p.mintPending, p.wickedLeft, p.zeroAwait = false, false, false
		case events.PutOnStack:
			if o := g.Obj(e.Obj); o != nil && o.Face() != nil {
				p.carrier = o.Face().Name
			}
		case events.TriggerPush:
			if o := g.Obj(e.Obj); o != nil && o.Face() != nil {
				p.carrier = o.Face().Name
			}
		case events.TokenCreate:
			if strings.HasPrefix(e.Text, "role_") {
				if p.zeroAwait {
					p.zeroMinted++
				}
				p.roleMints++
				p.mintPending = true
			}
		case events.Attach:
			o := g.Obj(e.Obj)
			if !kroleIsRole(o) || len(e.IDs) == 0 {
				continue
			}
			p.mintPending = false
			p.lastCtl = uint8(o.Controller)
			bearer := e.IDs[0]
			set := p.rolesOn[bearer]
			for other := range set {
				// CR 704.5z: Roles of different controllers coexist
				if oo := g.Obj(other); other != e.Obj && oo != nil && oo.Controller != o.Controller {
					p.crossSweep++
					break
				}
			}
			if set == nil {
				set = map[state.ObjID]bool{}
				p.rolesOn[bearer] = set
			}
			set[e.Obj] = true
			if len(set) > p.maxRoles {
				p.maxRoles = len(set)
			}
			switch {
			case kroleSelf[p.carrier]:
				p.selfAttach++
			case kroleChain[p.carrier]:
				p.chainAttach++
			case kroleOwn[p.carrier]:
				p.ownAttach++
			}
			if p.carrier == "Return Triumphant" {
				p.returnedAttach++
			}
		case events.MoveZone:
			o := g.Obj(e.Obj)
			if !kroleIsRole(o) || e.From != state.ZBattlefield || e.To != state.ZGraveyard {
				continue
			}
			p.detach(e.Obj)
			switch e.Text {
			case "another Role on it: the old Role goes to the graveyard":
				p.sameSweep++
			case "attached to an object that left the battlefield", "Aura attached to nothing":
				p.hostGone++
			}
			if o.Face() != nil && o.Face().Name == "Wicked" {
				p.wickedLeave++
				p.wickedLeft = true
			}
		case events.Note:
			if strings.Contains(e.Text, "isn't created (CR 303.4g)") {
				p.bearerGone++
			}
		case events.LifeChange:
			if p.wickedLeft && e.Amount == -1 {
				p.wickedLoss++
				p.wickedLeft = false
			}
		case events.CounterChange:
			if e.Counter == "P1P1" && e.Amount > 0 && kroleOn(g, e.Obj) {
				for id := range p.rolesOn[e.Obj] {
					if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Young Hero" {
						p.yhCounter++
						break
					}
				}
			}
		case events.Scry:
			if e.Obj != 0 && kroleOn(g, e.Obj) {
				for id := range p.rolesOn[e.Obj] {
					if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Sorcerer" {
						p.sorcScry++
						break
					}
				}
			}
		case events.DamageProvenance:
			if strings.Contains(e.Text, "damage-combat") {
				cursed := kroleHasNamedRole(g, e.Obj, "Cursed")
				if !cursed && len(e.IDs) > 0 {
					cursed = kroleHasNamedRole(g, e.IDs[0], "Cursed")
				}
				if cursed {
					p.cursedCombat++
				}
			}
		case events.Priority, events.StepChange, events.TurnChange:
			if p.mintPending {
				p.orphan++
				p.mintPending = false
			}
			p.zeroAwait = false
		}
	}
	p.evAt = len(evs)
	return nil
}

func kroleSetup(t *testing.T) (*cards.Registry, cardset.Set, [][2][]*cards.Card, []*kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("roles")
	if err != nil {
		t.Fatal(err)
	}
	decks := make([][2][]*cards.Card, len(kroleDecks))
	tbls := make([]*kop.Table, len(kroleDecks))
	for mi, md := range kroleDecks {
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

func kroleRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, cfg Config, p *kroleTally) Result {
	t.Helper()
	if p == nil {
		p = newKroleTally()
	}
	m, err := NewMatchup(reg, [2]string{"roles-a", "roles-b"}, decks, 1)
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
	res := LockstepLed(m, seed, lead, k, krolePolicy(seed, seed%4 == 0, p), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 60, Check: p.check})
	CloseSide(k)
	return res
}

const kroleSeeds = 96

func TestRoleLockstep(t *testing.T) {
	t.Parallel()

	reg, set, decks, tbls := kroleSetup(t)
	p := newKroleTally()
	games, div := 0, 0
	for mi := range kroleDecks {
		for seed := uint64(1); seed <= kroleSeeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			res := kroleRun(t, reg, set, decks[mi], tbls[mi], seed, cfg, p)
			games++
			if res.Stall != "" && res.Stall != "turns" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				div++
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
					mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, %d diverged: %d Role mints, own %d, chain %d, returned %d, self %d; zero-target %d, bearer gone %d; sweep same %d cross %d; Wicked leaves %d losses %d; host gone %d; Young Hero counters %d, Sorcerer scries %d, Cursed combat damage %d; orphan mints %d; max Roles on one bearer %d",
		games, div, p.roleMints, p.ownAttach, p.chainAttach, p.returnedAttach, p.selfAttach,
		p.zeroTargets, p.bearerGone, p.sameSweep, p.crossSweep, p.wickedLeave, p.wickedLoss,
		p.hostGone, p.yhCounter, p.sorcScry, p.cursedCombat, p.orphan, p.maxRoles)
	for _, c := range []struct {
		name string
		n    int
	}{
		{"own-slot attach", p.ownAttach}, {"chain-slot attach", p.chainAttach},
		{"returned-card attach", p.returnedAttach}, {"Self attach", p.selfAttach},
		{"zero targets elected", p.zeroTargets}, {"bearer gone", p.bearerGone},
		{"same-controller sweep", p.sameSweep}, {"cross-controller coexisting Role", p.crossSweep},
		{"Wicked leaves", p.wickedLeave}, {"Wicked life loss", p.wickedLoss},
		{"host died", p.hostGone}, {"Young Hero counter", p.yhCounter},
		{"Sorcerer scry", p.sorcScry}, {"Cursed combat damage", p.cursedCombat},
	} {
		if c.n < 10 {
			t.Errorf("%s: %d, want at least 10", c.name, c.n)
		}
	}
	if p.orphan != 0 {
		t.Errorf("%d Role TokenCreate events were not followed by an Attach in the same resolution", p.orphan)
	}
	if p.zeroMinted != 0 {
		t.Errorf("%d zero-target Role asks still created a Role token", p.zeroMinted)
	}
	if div != 0 {
		t.Errorf("%d diverged games", div)
	}
}

// kroleCopyTable deep-copies the generated table's cards so a mutation can
// edit its instructions without touching the shared pool.
func kroleCopyTable(tbl *kop.Table) *kop.Table {
	cp := *tbl
	cp.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cp.Cards {
		abs := make([]kop.Ability, len(cp.Cards[i].Abilities))
		copy(abs, cp.Cards[i].Abilities)
		for j := range abs {
			if abs[j].Prog != nil {
				prog := append([]kop.Instr(nil), abs[j].Prog...)
				abs[j].Prog = prog
			}
		}
		cp.Cards[i].Abilities = abs
	}
	return &cp
}

// kroleMutateRole clears the Role subtype bit on every role_* CardDef.
func kroleMutateRole(tbl *kop.Table) {
	for i := range tbl.Subtypes {
		if tbl.Subtypes[i] != "Role" {
			continue
		}
		for c := range tbl.Cards {
			if !strings.HasPrefix(tbl.Cards[c].Name, "role_") {
				continue
			}
			tbl.Cards[c].Subtypes[i>>6] &^= 1 << (i & 63)
		}
	}
}

// TestRoleMutation: each table edit must diverge on some seed.
func TestRoleMutation(t *testing.T) {
	t.Parallel()

	reg, set, decks, tbls := kroleSetup(t)
	muts := []struct {
		name string
		edit func(*kop.Table)
	}{
		{"clear TokAttach", func(tbl *kop.Table) {
			for c := range tbl.Cards {
				for a := range tbl.Cards[c].Abilities {
					for i := range tbl.Cards[c].Abilities[a].Prog {
						in := &tbl.Cards[c].Abilities[a].Prog[i]
						if in.Op == kop.OpCreateTokenEx {
							in.B.N &^= kop.TokAttach
						}
					}
				}
			}
		}},
		{"retarget the destination", func(tbl *kop.Table) {
			for _, name := range []string{"Cut In", "Eriette's Whisper"} {
				c, ok := kroleCard(tbl, name)
				if !ok {
					t.Fatalf("%s not in the table", name)
				}
				for a := range tbl.Cards[c].Abilities {
					for i := range tbl.Cards[c].Abilities[a].Prog {
						in := &tbl.Cards[c].Abilities[a].Prog[i]
						if in.Op == kop.OpCreateTokenEx && in.B.N&kop.TokAttach != 0 {
							in.B.N = in.B.N&0xff | kop.TokAttachN(kop.RefTarget0)
						}
					}
				}
			}
			c, ok := kroleCard(tbl, "Monstrous Rage")
			if !ok {
				t.Fatal("Monstrous Rage not in the table")
			}
			for a := range tbl.Cards[c].Abilities {
				for i := range tbl.Cards[c].Abilities[a].Prog {
					in := &tbl.Cards[c].Abilities[a].Prog[i]
					if in.Op == kop.OpCreateTokenEx && in.B.N&kop.TokAttach != 0 {
						in.B.N = in.B.N&0xff | kop.TokAttachN(kop.RefSelf)
					}
				}
			}
		}},
		{"clear the Role subtype", kroleMutateRole},
	}
	for _, mu := range muts {
		diverged := false
		for mi := range kroleDecks {
			for seed := uint64(1); seed <= kroleSeeds && !diverged; seed++ {
				tbl := kroleCopyTable(tbls[mi])
				mu.edit(tbl)
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				res := kroleRun(t, reg, set, decks[mi], tbl, seed, cfg, nil)
				if res.Div != nil {
					diverged = true
					t.Logf("mutation %q: matchup %d seed %d diverged at decision %d: %s %s", mu.name, mi, seed, res.Div.At, res.Div.What, res.Div.Field)
				}
			}
		}
		if !diverged {
			t.Errorf("mutation %q never diverged", mu.name)
		}
	}
}

// kroleCard finds a card index by its table Name.
func kroleCard(tbl *kop.Table, name string) (int, bool) {
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == name {
			return i, true
		}
	}
	return 0, false
}
