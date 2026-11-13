package oracle

import (
	"fmt"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// featscen.go: constructed kernel positions for the value-head extension
// block's tests and the value probe. dirac has no position builder, so
// each position is reached by playing a scripted mono-card game on a table
// built from set: decks of basic lands plus one or two role cards per side,
// driven by a rule policy (play a land; cast a role card when the scenario's
// condition holds; else pass) until the scenario's stop predicate holds. The
// viewer ("me") is seat 0 in every scenario; the opponent is seat 1.

// FeatScenario is a constructed kernel position.
type FeatScenario struct {
	Name  string
	State *dirac.State // owned by the caller; State has just posed Dec
	Dec   dirac.Decision
	Seat  uint8 // the seat the probe encodes for
	Pool  *feat.Pool
	Roles map[string]string // role -> card name used
}

// FeatScenarioNames is FeatScenarios' order.
var FeatScenarioNames = []string{"bomb-precast", "bomb-on-stack", "bomb-countered", "bomb-resolved",
	"removal-at-my-creature", "removal-at-my-land", "damage-at-me", "cheapkill-on-oneone", "bigkill-on-bomb",
	"counter-prevents-lethal", "removal-low-only", "removal-bomb-target", "upkeep-instant"}

const (
	featScenSeed     = 9100 // scenario i starts at seed featScenSeed+i
	featScenSeeds    = 16   // seeds tried per candidate set
	featScenCands    = 6    // role candidate sets tried (the nth candidate of every role)
	featScenTurns    = 40
	featScenDecision = 6000
)

// roleCand is a pool card picked for a role.
type roleCand struct {
	name string
	id   kop.CardID // in the set's pool table
	mv   int
	cols kop.ColorMask
	tgt  string // the gorge spell ability's ValidTgts
	sa   bool   // the gorge card has a spell ability
}

// featRoles lists, per role, the pool cards that fit it, by ascending
// CardID.
func featRoles(set cardset.Set, c *Corpus) (map[string][]roleCand, error) {
	tbl, err := set.Pool()
	if err != nil {
		return nil, err
	}
	fp, err := FeatPool(c.Reg, tbl, nil)
	if err != nil {
		return nil, err
	}
	sup := set.Supported()
	var idx []int
	for i := 0; i < set.NumCards(); i++ {
		if sup[i] && !set.Inert(i) {
			idx = append(idx, i)
		}
	}
	slices.SortFunc(idx, func(a, b int) int { return int(set.PoolID(a)) - int(set.PoolID(b)) })
	r := map[string][]roleCand{}
	for _, i := range idx {
		id := set.PoolID(i)
		cd := &tbl.Cards[id]
		cc := roleCand{name: cd.Name, id: id, mv: cd.Cost.MV(), cols: cd.Colors}
		if gc, ok := c.Reg.Lookup(cd.Name); ok && len(gc.Faces) > 0 {
			if sa := gc.Faces[0].SpellAbility(); sa != nil {
				cc.tgt, cc.sa = sa.Params["ValidTgts"], true
			}
		}
		e := fp.Effect(id)
		creature := cd.Types&kop.TCreature != 0
		spell := cd.Types&(kop.TInstant|kop.TSorcery) != 0 && cc.sa
		instant := cd.Types&kop.TInstant != 0
		switch {
		case creature && cc.mv >= 6 && feat.StrengthBucket(fp.PrintedStrength(id)) == 3 && !cantBeCountered(cd):
			// the bomb positions counter it: an uncounterable bomb would resolve
			r["bomb"] = append(r["bomb"], cc)
		case creature && cd.Power == 1 && cd.Toughness == 1 && cc.mv <= 2:
			r["oneone"] = append(r["oneone"], cc)
		}
		if instant && spell && e.Class == feat.EffCounter && cc.mv <= 3 {
			if !strings.Contains(cc.tgt, "nonCreature") {
				r["counter-creature"] = append(r["counter-creature"], cc)
			}
			if strings.Contains(cc.tgt, "nonCreature") || !strings.Contains(cc.tgt, "Creature") {
				r["counter"] = append(r["counter"], cc)
			}
		}
		if !spell {
			// a bait permanent for a landkill that admits no land: a plain
			// artifact or enchantment
			if cc.mv <= 2 && (cd.Types == kop.TArtifact || cd.Types == kop.TEnchantment) {
				k := "bait-enchantment"
				if cd.Types == kop.TArtifact {
					k = "bait-artifact"
				}
				r[k] = append(r[k], cc)
			}
			continue
		}
		if (e.Class == feat.EffKill || e.Class == feat.EffDamage) && e.TgtCreat && cc.mv <= 2 {
			r["cheapkill"] = append(r["cheapkill"], cc)
		}
		if e.Class == feat.EffKill && e.TgtCreat && cc.mv >= 3 && slices.Contains(strings.Split(cc.tgt, ","), "Creature") { // any creature
			r["bigkill"] = append(r["bigkill"], cc)
		}
		if e.Class == feat.EffDamage && e.TgtPlayer {
			if e.Amount >= 3 {
				r["burn"] = append(r["burn"], cc)
			}
			if instant && cc.mv <= 2 {
				r["chip"] = append(r["chip"], cc)
			}
		}
		if e.Class == feat.EffKill && landkillClass(cc.tgt) != "" {
			r["landkill"] = append(r["landkill"], cc)
		}
	}
	return r, nil
}

// landkillClasses reports which of land, artifact and enchantment a kill
// spell with ValidTgts tg admits ("Permanent.nonLand" admits no land).
func landkillClasses(tg string) (land, art, ench bool) {
	for _, t := range strings.Split(tg, ",") {
		switch base, _, _ := strings.Cut(t, "."); base {
		case "Land":
			land = true
		case "Permanent":
			land = land || !strings.Contains(t, "nonLand")
			art = art || !strings.Contains(t, "nonArtifact")
			ench = ench || !strings.Contains(t, "nonEnchantment")
		case "Artifact":
			art = true
		case "Enchantment":
			ench = true
		}
	}
	return land, art, ench
}

// landkillClass is the permanent class a kill spell with ValidTgts tg admits
// beside creatures: "land", "artifact" or "enchantment"; "" for none.
func landkillClass(tg string) string {
	land, art, ench := landkillClasses(tg)
	switch {
	case land:
		return "land"
	case art:
		return "artifact"
	case ench:
		return "enchantment"
	}
	return ""
}

// cantBeCountered reports a card whose spell can't be countered.
func cantBeCountered(cd *kop.CardDef) bool {
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplCantBeCountered {
			return true
		}
	}
	return false
}

// featBasics are the basic land of each colour bit, W U B R G.
var featBasics = [5]string{"Plains", "Island", "Swamp", "Mountain", "Forest"}

// featGame is one scripted game's table and decks.
type featGame struct {
	tbl   *kop.Table
	decks [2][]kop.CardID
	id    map[string]kop.CardID // role -> CardID in tbl
	name  map[string]string     // role -> card name
	pool  *feat.Pool
}

// featSide is a side's role cards, in order.
type featSide []string

// newFeatGame builds the table for sides' role cards (picked by role from
// the nth candidate).
func newFeatGame(set cardset.Set, c *Corpus, roles map[string][]roleCand, ci int, sides [2]featSide) (*featGame, error) {
	var es [2][]cardset.Entry
	for s := 0; s < 2; s++ {
		var cols kop.ColorMask
		for _, role := range sides[s] {
			cands := roles[role]
			cols |= cands[min(ci, len(cands)-1)].cols
		}
		var basics []string
		for b := 0; b < 5; b++ {
			if cols&(1<<b) != 0 {
				basics = append(basics, featBasics[b])
			}
		}
		if len(basics) == 0 {
			basics = []string{featBasics[0]}
		}
		for i, b := range basics {
			n := 24 / len(basics)
			if i == 0 {
				n = 24 - n*(len(basics)-1)
			}
			es[s] = append(es[s], cardset.Entry{Name: b, Count: n})
		}
		for _, role := range sides[s] {
			cands := roles[role]
			es[s] = append(es[s], cardset.Entry{Name: cands[min(ci, len(cands)-1)].name, Count: 36 / len(sides[s])})
		}
	}
	tbl, a, b, err := set.Matchup(es[0], es[1])
	if err != nil {
		return nil, err
	}
	g := &featGame{tbl: tbl, decks: [2][]kop.CardID{a, b}, id: map[string]kop.CardID{}, name: map[string]string{}}
	for s := 0; s < 2; s++ {
		for j, role := range sides[s] {
			g.id[role] = g.decks[s][24+j*(36/len(sides[s]))]
			cands := roles[role]
			g.name[role] = cands[min(ci, len(cands)-1)].name
		}
	}
	g.pool, err = FeatPool(c.Reg, tbl, nil)
	return g, err
}

// sview is the position a policy or predicate reads.
type sview struct {
	s *dirac.State
	d *dirac.Decision
	p dirac.Projection
	g *featGame
}

func (v *sview) top() *dirac.ProjStack {
	if v.p.NStack == 0 {
		return nil
	}
	return &v.p.Stack[v.p.NStack-1]
}

func (v *sview) types(o uint8) kop.TypeMask { return v.g.tbl.Cards[v.p.Objs[o].Card].Types }

// find returns the first object of card id in zone z owned by seat, -1 for none.
func (v *sview) find(seat uint8, z kop.Zone, id kop.CardID) int {
	for i := 0; i < int(v.p.NObj); i++ {
		o := &v.p.Objs[i]
		if o.Zone == z && o.Owner == seat && o.Card == id {
			return i
		}
	}
	return -1
}

// count counts seat's controlled battlefield objects whose types meet t and
// that are untapped when untapped is set.
func (v *sview) count(seat uint8, t kop.TypeMask, untapped bool) int {
	n := 0
	for i := 0; i < int(v.p.NObj); i++ {
		o := &v.p.Objs[i]
		if o.Zone == kop.ZoneBattlefield && o.Ctrl == seat && v.types(uint8(i))&t != 0 && !(untapped && o.Flags&dirac.ProjTapped != 0) {
			n++
		}
	}
	return n
}

// castable reports whether the decision offers a plain cast of a hand card
// of card id.
func (v *sview) castable(id kop.CardID) bool {
	for _, a := range v.d.Options[:v.d.N] {
		if a.Kind() == dirac.AkCast && v.p.Objs[a.Source()].Card == id && v.p.Objs[a.Source()].Zone == kop.ZoneHand {
			return true
		}
	}
	return false
}

// atPrio is a priority decision of seat.
func (v *sview) atPrio(seat uint8) bool { return v.d.Kind == dirac.DkPriority && v.d.Player == seat }

// seatPol is one seat's script. cast says whether to cast the plain-cast
// hand card o (of card id), asked only at an empty stack so a seat casts
// one spell at a time; tgt scores a target byte (the best is chosen).
type seatPol struct {
	cast func(v *sview, id kop.CardID) bool
	tgt  func(v *sview, t uint8) int
}

func (v *sview) choose(pols *[2]seatPol) dirac.Action {
	opts := v.d.Options[:v.d.N]
	pol := &pols[v.d.Player]
	switch v.d.Kind {
	case dirac.DkMulligan:
		for _, a := range opts {
			if a.Kind() == dirac.AkKeep {
				return a
			}
		}
	case dirac.DkPriority:
		for _, a := range opts {
			if a.Kind() == dirac.AkPlayLand {
				return a
			}
		}
		if pol.cast != nil && v.p.NStack == 0 {
			for _, a := range opts {
				if a.Kind() == dirac.AkCast && a.Mode() == 0 {
					if o := &v.p.Objs[a.Source()]; o.Zone == kop.ZoneHand && pol.cast(v, o.Card) {
						return a
					}
				}
			}
		}
		for _, a := range opts {
			if a.Kind() == dirac.AkPass {
				return a
			}
		}
	case dirac.DkTarget:
		if pol.tgt != nil {
			best, bs := opts[0], pol.tgt(v, uint8(opts[0].Target()))
			for _, a := range opts[1:] {
				if sc := pol.tgt(v, uint8(a.Target())); sc > bs {
					best, bs = a, sc
				}
			}
			return best
		}
	case dirac.DkAttackers, dirac.DkBlockers:
		for _, a := range opts {
			if a.Kind() == dirac.AkDone {
				return a
			}
		}
	}
	return opts[0]
}

// advance applies forced (else the policy's answer) to the pending decision
// d of st, then plays on until stop holds at a decision. false: the game
// ended, ran past the turn limit or past the decision budget.
func (g *featGame) advance(st *dirac.State, d *dirac.Decision, pols *[2]seatPol, stop func(*sview) bool, forced *dirac.Action) bool {
	v := &sview{s: st, d: d, g: g}
	st.Project(&v.p)
	for n := 0; n < featScenDecision; n++ {
		a := dirac.Action(0)
		if forced != nil {
			a, forced = *forced, nil
		} else {
			a = v.choose(pols)
		}
		st.Apply(a)
		st.StepInto(d)
		if over, _ := st.Terminal(); over || d.Kind == dirac.DkNone || st.Turn() > featScenTurns {
			return false
		}
		st.Project(&v.p)
		if stop(v) {
			return true
		}
	}
	return false
}

// start is a fresh game at its first decision.
func (g *featGame) start(seed uint64) (*dirac.State, *dirac.Decision) {
	cfg := dirac.LockstepConfig(dirac.SurfaceFull)
	cfg.NoElide = true // upkeep and the other empty-step priorities are posed
	st := new(dirac.State)
	st.Init(g.tbl, g.decks[0], g.decks[1], seed, cfg)
	d := new(dirac.Decision)
	st.StepInto(d)
	return st, d
}

// fork copies a position.
func fork(st *dirac.State, d *dirac.Decision) (*dirac.State, *dirac.Decision) {
	c, cd := new(dirac.State), new(dirac.Decision)
	c.CopyFrom(st)
	*cd = *d
	return c, cd
}

func (g *featGame) scenario(name string, st *dirac.State, d *dirac.Decision, roles ...string) FeatScenario {
	r := map[string]string{}
	for _, k := range roles {
		r[k] = g.name[k]
	}
	return FeatScenario{Name: name, State: st, Dec: *d, Seat: 0, Pool: g.pool, Roles: r}
}

// Policy pieces.

func castAlways(*sview, kop.CardID) bool { return true }

func castOnly(id kop.CardID) func(*sview, kop.CardID) bool {
	return func(_ *sview, c kop.CardID) bool { return c == id }
}

// score is a target scorer: 100 for a target byte t of seat 0 meeting want.
func targetObj(want func(v *sview, o uint8) bool) func(*sview, uint8) int {
	return func(v *sview, t uint8) int {
		if t < uint8(dirac.MaxObj) && want(v, t) {
			return 100
		}
		return 0
	}
}

func mineOnBf(types kop.TypeMask) func(v *sview, o uint8) bool {
	return func(v *sview, o uint8) bool {
		ob := &v.p.Objs[o]
		return ob.Zone == kop.ZoneBattlefield && ob.Ctrl == 0 && v.types(o)&types != 0
	}
}

func featSearch(set cardset.Set, c *Corpus, roles map[string][]roleCand, first int, names []string, sides func(ci int) [2]featSide, build func(g *featGame, seed uint64) ([]FeatScenario, bool)) ([]FeatScenario, error) {
	var last error
	for ci := 0; ci < featScenCands; ci++ {
		g, err := newFeatGame(set, c, roles, ci, sides(ci))
		if err != nil {
			last = err
			continue
		}
		for k := 0; k < featScenSeeds; k++ {
			if out, ok := build(g, uint64(featScenSeed+first+k)); ok {
				return out, nil
			}
		}
		last = fmt.Errorf("no seed reached it")
	}
	return nil, fmt.Errorf("featscen %v: %v", names, last)
}

// FeatScenarios builds every scenario of FeatScenarioNames on set, in that
// order. Each State is the caller's. A role set lacks is an error naming it.
func FeatScenarios(set cardset.Set, c *Corpus) ([]FeatScenario, error) {
	roles, err := featRoles(set, c)
	if err != nil {
		return nil, err
	}
	need := []string{"bomb", "counter-creature", "counter", "cheapkill", "bigkill", "oneone", "burn", "chip", "landkill"}
	for _, r := range need {
		if len(roles[r]) == 0 {
			return nil, fmt.Errorf("featscen: set %s has no card for role %q", set.Name(), r)
		}
	}
	// the landkill's target class, and a bait permanent of mine when it admits no land
	lk := landkillClass(roles["landkill"][0].tgt)
	baitRole := ""
	if lk != "land" {
		_, art, ench := landkillClasses(roles["landkill"][0].tgt)
		for _, k := range []struct {
			cls string
			ok  bool
		}{{"artifact", art}, {"enchantment", ench}} {
			if k.ok && len(roles["bait-"+k.cls]) > 0 {
				baitRole = "bait-" + k.cls
				break
			}
		}
		if baitRole == "" {
			return nil, fmt.Errorf("featscen: set %s has no card for role %q", set.Name(), "bait-"+lk)
		}
	}
	roles["landkill"] = slices.DeleteFunc(roles["landkill"], func(c roleCand) bool { return landkillClass(c.tgt) != lk })
	by := map[string]FeatScenario{}
	add := func(ss []FeatScenario) {
		for _, s := range ss {
			by[s.Name] = s
		}
	}
	idx := func(n string) int { return slices.Index(FeatScenarioNames, n) }
	type group struct {
		first string
		names []string
		sides func(ci int) [2]featSide
		build func(g *featGame, seed uint64) ([]FeatScenario, bool)
	}
	groups := []group{
		{"bomb-precast", []string{"bomb-precast", "bomb-on-stack", "bomb-countered", "bomb-resolved"},
			func(int) [2]featSide { return [2]featSide{{"counter-creature"}, {"bomb"}} }, buildBomb},
		{"removal-at-my-creature", []string{"removal-at-my-creature"},
			func(int) [2]featSide { return [2]featSide{{"oneone"}, {"bigkill"}} },
			func(g *featGame, seed uint64) ([]FeatScenario, bool) {
				pols := &[2]seatPol{{cast: castAlways}, {cast: castAlways, tgt: targetObj(mineOnBf(kop.TCreature))}}
				k := g.id["bigkill"]
				return g.one("removal-at-my-creature", seed, pols, func(v *sview) bool {
					t := v.top()
					return v.atPrio(0) && t != nil && t.Card == k && t.Ctrl == 1 && t.Targets[0] < uint8(dirac.MaxObj) && mineOnBf(kop.TCreature)(v, t.Targets[0])
				}, "oneone", "bigkill")
			}},
		{"removal-at-my-land", []string{"removal-at-my-land"},
			func(int) [2]featSide {
				if baitRole == "" {
					return [2]featSide{{"counter"}, {"landkill"}}
				}
				return [2]featSide{{baitRole}, {"landkill"}}
			},
			func(g *featGame, seed uint64) ([]FeatScenario, bool) {
				want := kop.TLand
				cls := "land"
				if baitRole != "" {
					cls = strings.TrimPrefix(baitRole, "bait-")
					want = kop.TArtifact
					if cls == "enchantment" {
						want = kop.TEnchantment
					}
				}
				pols := &[2]seatPol{{cast: castAlways}, {cast: castAlways, tgt: targetObj(mineOnBf(want))}}
				k := g.id["landkill"]
				out, ok := g.one("removal-at-my-land", seed, pols, func(v *sview) bool {
					t := v.top()
					return v.atPrio(0) && t != nil && t.Card == k && t.Ctrl == 1 && t.Targets[0] < uint8(dirac.MaxObj) && mineOnBf(want)(v, t.Targets[0])
				}, "landkill")
				if ok {
					out[0].Roles["landkill-target"] = cls
					if baitRole != "" {
						out[0].Roles["landkill-bait"] = g.name[baitRole]
					}
				}
				return out, ok
			}},
		{"damage-at-me", []string{"damage-at-me"},
			func(int) [2]featSide { return [2]featSide{{"counter"}, {"chip"}} },
			func(g *featGame, seed uint64) ([]FeatScenario, bool) {
				pols := &[2]seatPol{{}, {cast: castAlways, tgt: func(_ *sview, t uint8) int {
					if dirac.TCode(t) == dirac.PlayerTarget0 {
						return 100
					}
					return 0
				}}}
				k := g.id["chip"]
				return g.one("damage-at-me", seed, pols, func(v *sview) bool {
					t := v.top()
					return v.atPrio(0) && t != nil && t.Card == k && t.Ctrl == 1 && dirac.TCode(t.Targets[0]) == dirac.PlayerTarget0
				}, "chip")
			}},
		{"cheapkill-on-oneone", []string{"cheapkill-on-oneone"},
			func(int) [2]featSide { return [2]featSide{{"oneone", "counter"}, {"cheapkill"}} },
			func(g *featGame, seed uint64) ([]FeatScenario, bool) {
				one, kill, ctr := g.id["oneone"], g.id["cheapkill"], g.id["counter"]
				// the kill is cast on its own turn, so my counter's lands are untapped
				pols := &[2]seatPol{{cast: castOnly(one)}, {
					cast: func(v *sview, _ kop.CardID) bool { return v.p.Active == 1 && v.count(0, kop.TCreature, false) > 0 },
					tgt: targetObj(func(v *sview, o uint8) bool {
						return mineOnBf(kop.TCreature)(v, o) && v.p.Objs[o].Card == one
					})}}
				return g.one("cheapkill-on-oneone", seed, pols, func(v *sview) bool {
					t := v.top()
					return v.atPrio(0) && t != nil && t.Card == kill && t.Ctrl == 1 && t.Targets[0] < uint8(dirac.MaxObj) &&
						mineOnBf(kop.TCreature)(v, t.Targets[0]) && v.p.Objs[t.Targets[0]].Card == one && v.find(0, kop.ZoneHand, ctr) >= 0 && v.castable(ctr)
				}, "cheapkill", "oneone", "counter")
			}},
		{"bigkill-on-bomb", []string{"bigkill-on-bomb"},
			func(int) [2]featSide { return [2]featSide{{"bomb", "counter"}, {"bigkill"}} },
			func(g *featGame, seed uint64) ([]FeatScenario, bool) {
				bomb, kill, ctr := g.id["bomb"], g.id["bigkill"], g.id["counter"]
				// one bomb of mine; the kill is cast on its own turn, so my
				// counter's lands are untapped
				pols := &[2]seatPol{{cast: func(v *sview, id kop.CardID) bool { return id == bomb && v.count(0, kop.TCreature, false) == 0 }}, {
					cast: func(v *sview, _ kop.CardID) bool { return v.p.Active == 1 && v.count(0, kop.TCreature, false) > 0 },
					tgt: targetObj(func(v *sview, o uint8) bool {
						return mineOnBf(kop.TCreature)(v, o) && v.p.Objs[o].Card == bomb
					})}}
				return g.one("bigkill-on-bomb", seed, pols, func(v *sview) bool {
					t := v.top()
					return v.atPrio(0) && t != nil && t.Card == kill && t.Ctrl == 1 && t.Targets[0] < uint8(dirac.MaxObj) &&
						mineOnBf(kop.TCreature)(v, t.Targets[0]) && v.p.Objs[t.Targets[0]].Card == bomb && v.castable(ctr)
				}, "bigkill", "bomb", "counter")
			}},
		{"counter-prevents-lethal", []string{"counter-prevents-lethal"},
			func(int) [2]featSide { return [2]featSide{{"counter"}, {"burn", "chip"}} },
			func(g *featGame, seed uint64) ([]FeatScenario, bool) {
				burn, chip, ctr := g.id["burn"], g.id["chip"], g.id["counter"]
				amt := int16(g.pool.Effect(burn).Amount)
				pols := &[2]seatPol{{}, {
					cast: func(v *sview, id kop.CardID) bool {
						if v.p.Life[0] <= amt {
							return id == burn
						}
						return id == chip
					},
					tgt: func(_ *sview, t uint8) int {
						if dirac.TCode(t) == dirac.PlayerTarget0 {
							return 100
						}
						return 0
					}}}
				return g.one("counter-prevents-lethal", seed, pols, func(v *sview) bool {
					t := v.top()
					return v.atPrio(0) && t != nil && t.Card == burn && t.Ctrl == 1 && dirac.TCode(t.Targets[0]) == dirac.PlayerTarget0 &&
						v.p.Life[0] <= amt && v.castable(ctr)
				}, "burn", "chip", "counter")
			}},
		{"removal-low-only", []string{"removal-low-only"},
			func(int) [2]featSide { return [2]featSide{{"bigkill"}, {"oneone"}} },
			func(g *featGame, seed uint64) ([]FeatScenario, bool) {
				return g.lowOnly("removal-low-only", seed, "oneone")
			}},
		{"removal-bomb-target", []string{"removal-bomb-target"},
			func(int) [2]featSide { return [2]featSide{{"bigkill"}, {"bomb"}} },
			func(g *featGame, seed uint64) ([]FeatScenario, bool) {
				return g.lowOnly("removal-bomb-target", seed, "bomb")
			}},
		{"upkeep-instant", []string{"upkeep-instant"},
			func(int) [2]featSide { return [2]featSide{{"chip"}, {"oneone"}} },
			func(g *featGame, seed uint64) ([]FeatScenario, bool) {
				chip := g.id["chip"]
				pols := &[2]seatPol{{}, {cast: castAlways}}
				out, ok := g.one("upkeep-instant", seed, pols, func(v *sview) bool {
					return v.atPrio(0) && v.p.Step == kop.StepUpkeep && v.p.Active == 0 && v.p.NStack == 0 && v.castable(chip)
				}, "chip")
				if ok {
					out[0].Roles["instant"] = out[0].Roles["chip"]
				}
				return out, ok
			}},
	}
	for _, gr := range groups {
		ss, err := featSearch(set, c, roles, idx(gr.first), gr.names, gr.sides, gr.build)
		if err != nil {
			return nil, err
		}
		add(ss)
	}
	out := make([]FeatScenario, 0, len(FeatScenarioNames))
	for _, n := range FeatScenarioNames {
		s, ok := by[n]
		if !ok {
			return nil, fmt.Errorf("featscen: %s was not built", n)
		}
		out = append(out, s)
	}
	return out, nil
}

// one plays a fresh game from seed to stop and returns its single scenario.
func (g *featGame) one(name string, seed uint64, pols *[2]seatPol, stop func(*sview) bool, roles ...string) ([]FeatScenario, bool) {
	st, d := g.start(seed)
	// the first decision is a mulligan keep: advance applies before it steps
	if !g.advance(st, d, pols, stop, nil) {
		return nil, false
	}
	return []FeatScenario{g.scenario(name, st, d, roles...)}, true
}

// lowOnly is the removal-low-only / removal-bomb-target position: my main 1
// with a castable bigkill, the opponent's only creature being role oppRole.
func (g *featGame) lowOnly(name string, seed uint64, oppRole string) ([]FeatScenario, bool) {
	kill, opp := g.id["bigkill"], g.id[oppRole]
	pols := &[2]seatPol{{}, {cast: func(v *sview, id kop.CardID) bool { return id == opp && v.count(1, kop.TCreature, false) == 0 }}}
	return g.one(name, seed, pols, func(v *sview) bool {
		if !(v.atPrio(0) && v.p.Step == kop.StepMain1 && v.p.Active == 0 && v.p.NStack == 0 && v.castable(kill)) {
			return false
		}
		if v.count(1, kop.TCreature, false) != 1 {
			return false
		}
		for i := 0; i < int(v.p.NObj); i++ {
			if o := &v.p.Objs[i]; o.Zone == kop.ZoneBattlefield && o.Ctrl == 1 && v.types(uint8(i))&kop.TCreature != 0 {
				return o.Card == opp
			}
		}
		return false
	}, "bigkill", oppRole)
}

// buildBomb plays the bomb sequence: precast, on the stack, then the two
// branches from the stack position (my counter resolves; I pass).
func buildBomb(g *featGame, seed uint64) ([]FeatScenario, bool) {
	bomb, ctr := g.id["bomb"], g.id["counter-creature"]
	pols := &[2]seatPol{{}, {cast: castOnly(bomb)}}
	pols[0].tgt = func(v *sview, t uint8) int {
		if t < uint8(dirac.MaxObj) && v.p.Objs[t].Zone == kop.ZoneStack && v.p.Objs[t].Card == bomb {
			return 100
		}
		return 0
	}
	st, d := g.start(seed)
	mv := g.tbl.Cards[ctr].Cost.MV()
	if !g.advance(st, d, pols, func(v *sview) bool {
		return v.atPrio(1) && v.p.Step == kop.StepMain1 && v.p.Active == 1 && v.p.NStack == 0 && v.castable(bomb) &&
			v.find(0, kop.ZoneHand, ctr) >= 0 && v.count(0, kop.TLand, true) >= mv
	}, nil) {
		return nil, false
	}
	pre := g.scenario("bomb-precast", st, d, "bomb", "counter-creature")
	st, d = fork(st, d)
	if !g.advance(st, d, pols, func(v *sview) bool {
		t := v.top()
		return v.atPrio(0) && v.p.NStack == 1 && t.Card == bomb && t.Ctrl == 1 && v.castable(ctr)
	}, nil) {
		return nil, false
	}
	onStack := g.scenario("bomb-on-stack", st, d, "bomb", "counter-creature")
	var p0 dirac.Projection
	st.Project(&p0)
	src := p0.Stack[0].Source // the bomb spell's object, followed into both branches
	turn := st.Turn()         // both branches stop on this turn

	cs, cd := fork(st, d)
	var cast *dirac.Action
	var v sview
	v.p = dirac.Projection{}
	cs.Project(&v.p)
	for _, a := range cd.Options[:cd.N] {
		if a.Kind() == dirac.AkCast && a.Mode() == 0 && v.p.Objs[a.Source()].Card == ctr && v.p.Objs[a.Source()].Zone == kop.ZoneHand {
			a := a
			cast = &a
			break
		}
	}
	if cast == nil {
		return nil, false
	}
	if !g.advance(cs, cd, pols, func(v *sview) bool {
		o := &v.p.Objs[src]
		return v.atPrio(0) && v.s.Turn() == turn && v.p.NStack == 0 && o.Card == bomb && o.Zone == kop.ZoneGraveyard
	}, cast) {
		return nil, false
	}
	countered := g.scenario("bomb-countered", cs, cd, "bomb", "counter-creature")

	rs, rd := fork(st, d)
	pass := dirac.MakeAction(dirac.AkPass, 0, 0, 0)
	for _, a := range d.Options[:d.N] {
		if a.Kind() == dirac.AkPass {
			pass = a
		}
	}
	if !g.advance(rs, rd, pols, func(v *sview) bool {
		o := &v.p.Objs[src]
		return v.atPrio(0) && v.s.Turn() == turn && v.p.NStack == 0 && o.Card == bomb && o.Zone == kop.ZoneBattlefield
	}, &pass) {
		return nil, false
	}
	resolved := g.scenario("bomb-resolved", rs, rd, "bomb", "counter-creature")
	for _, s := range []*FeatScenario{&pre, &onStack, &countered, &resolved} {
		s.Roles["bomb"], s.Roles["counter"] = g.name["bomb"], g.name["counter-creature"]
		delete(s.Roles, "counter-creature")
	}
	return []FeatScenario{pre, onStack, countered, resolved}, true
}
