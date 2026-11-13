package oracle

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// featref.go builds dirac/feat's per-CardID pool from gorge and compares
// the kernel's encoded network inputs with gorge's policynet encoding of the
// same position (FeaturesEntity, the entity network's set).

// FeatPool builds tbl's feat pool from gorge's corpus: each kernel card is
// resolved by name (through names when it maps one; a token by its token
// script's face name). Keywords and off-battlefield P/T are what gorge's
// own ViewCharacteristics projects for an object of the card.
func FeatPool(reg *cards.Registry, tbl *kop.Table, names map[string]string) (*feat.Pool, error) {
	resolved := make([]*cards.Card, len(tbl.Cards))
	tokensByName := map[string][]*cards.Card{}
	var tkeys []string
	for k := range reg.Tokens {
		tkeys = append(tkeys, k)
	}
	sort.Strings(tkeys)
	for _, k := range tkeys {
		c := reg.Tokens[k]
		if n := cardName(c); n != "" {
			tokensByName[n] = append(tokensByName[n], c)
		}
	}
	for i, cd := range tbl.Cards {
		n := cd.Name
		if g, ok := names[n]; ok {
			n = g
		}
		var c *cards.Card
		if tc, ok := reg.Tokens[n]; ok && cd.Types&kop.TToken != 0 {
			c = tc // a generated table names a token by its script ("u_1_1_faerie_flying")
		} else if cd.Types&kop.TToken != 0 {
			c = matchToken(&cd, append(tokensByName[n], tokensByName[n+" Token"]...))
		} else if lc, ok := reg.Lookup(n); ok {
			c = lc
		}
		resolved[i] = c // nil: a kernel-only test card (no deck plays it)
	}
	// One scratch engine holding every card as an object: gorge's own
	// derived keywords and P/T for an object that is not on the battlefield.
	var deck []*cards.Card
	at := make([]int, len(resolved))
	for i, c := range resolved {
		at[i] = -1
		if c != nil {
			at[i] = len(deck)
			deck = append(deck, c)
		}
	}
	e := rules.New(rules.Config{Seed: 1, Names: []string{"p0", "p1"}, Decks: [][]*cards.Card{deck, deck}, Tokens: reg.Tokens})
	specs := make([]feat.CardSpec, len(resolved))
	for i, c := range resolved {
		if c == nil {
			specs[i] = feat.CardSpec{Name: tbl.Cards[i].Name}
			if n, ok := strings.CutPrefix(tbl.Cards[i].Name, "name:"); ok {
				// an override's name card (kgen): the renamed object's view
				cd := &tbl.Cards[i]
				specs[i] = feat.CardSpec{Name: n, Types: kernelTypeLine(tbl, cd), Power: int32(cd.Power), Toughness: int32(cd.Toughness)}
			}
			continue
		}
		f := c.Faces[0]
		sp := feat.CardSpec{Name: f.Name, Types: strings.Join(f.Types, " "), ManaCost: f.ManaCost}
		if sa := f.SpellAbility(); sa != nil {
			sp.SpellAPI = sa.API
			sp.Effect = effectSpec(sa, f.ManaCost)
		}
		_, _, kws, p, t := e.ViewCharacteristics(e.G.Objs[at[i]].ID)
		sp.Keywords = append([]string(nil), kws...)
		sp.Power, sp.Toughness = p, t
		sp.Avail = availWeight(f)
		specs[i] = sp
	}
	p := feat.NewPool(tbl, specs)
	p.SetCreatureTypes(effects.CreatureTypeWordList())
	return p, nil
}

// matchToken picks the token script among same-named candidates whose
// card types and P/T match the kernel's definition (Raise the Alarm's
// white Soldier, not an artifact Soldier).
func matchToken(cd *kop.CardDef, cands []*cards.Card) *cards.Card {
	for _, c := range cands {
		f := c.Faces[0]
		has := func(w string) bool {
			for _, t := range f.Types {
				if t == w {
					return true
				}
			}
			return false
		}
		if has("Creature") != (cd.Types&kop.TCreature != 0) || has("Artifact") != (cd.Types&kop.TArtifact != 0) ||
			has("Land") != (cd.Types&kop.TLand != 0) || has("Enchantment") != (cd.Types&kop.TEnchantment != 0) {
			continue
		}
		if cd.Types&kop.TCreature != 0 && f.PT != fmt.Sprintf("%d/%d", cd.Power, cd.Toughness) {
			continue
		}
		return c
	}
	return nil
}

// availWeight is what one untapped source adds to gorge's AvailableMana
// total (rules.AvailableMana / addAvailable): its single usable mana ability
// -- a bare {T}, or a tap-and-sacrifice-itself the paid-cost census counts --
// times every colour alternative it can make, times its amount; 0 when the
// card has none or several such abilities.
// selfSacTap is "{T}, Sacrifice this: ...": Sac<1/CARDNAME> with or without
// its description ("Sac<1/CARDNAME/this token>", a Treasure's).
var selfSacTap = regexp.MustCompile(`^T Sac<1/CARDNAME(/[^>]*)?>$`)

func availWeight(f *cards.Face) int32 {
	var w int32
	n := 0
	for _, sa := range f.Abilities {
		if sa == nil || sa.API != "Mana" || sa.Kind != "AB" {
			continue
		}
		cost := strings.Join(strings.Fields(sa.Params["Cost"]), " ")
		if cost != "T" && !selfSacTap.MatchString(cost) {
			continue
		}
		mp := effects.ManaOf(sa)
		amt := int32(1)
		if mp.AmountTrim != "" {
			amt = 0
			if mp.AmountIsLit && mp.AmountLit > 0 {
				amt = int32(mp.AmountLit)
			}
		}
		counts, _ := cards.ProducedCounts(mp.Produced)
		w = 0
		for _, c := range counts {
			w += c * amt
		}
		if strings.Contains(mp.Produced, "Chosen") {
			// AvailableMana adds the colour already chosen (Heraldic
			// Banner's), one unit, not the five possibilities
			w = amt
		}
		n++
	}
	if n != 1 {
		return 0
	}
	return w
}

// GorgeFeatures is gorge's encoding of e's pending decision for its player:
// the FeaturesEntity state and the given options' encodings (in the given
// order) on the seat's redacted view -- netsimd's corpus recipe.
func GorgeFeatures(e *rules.Engine, d *decision.Decision, opts []decision.Option) (policynet.State, []policynet.Option) {
	v := view.Project(e.G, e, d.Player, d)
	fs := policynet.FeaturesEntity
	st := policynet.EncodeStateWith(fs, v, d.Player, nil)
	enc := policynet.EncodeOptionsWith(fs, &v, d.Player, d.Kind, opts, nil)
	return st, enc
}

// GorgeOption is the gorge decision option canonical action a stands for
// at the current frame: the offered option itself, or for a planner-paid,
// lowered or scripted play the cast/ability option it ends by selecting
// (azmcts' synthetic payCastOption); Done is an option of kind "done".
func (g *GorgeSide) GorgeOption(a Canon) (decision.Option, bool) {
	fr := g.fr
	if fr == nil {
		return decision.Option{}, false
	}
	if a.Kind() == ActDone {
		return decision.Option{Kind: "done"}, true
	}
	for i, x := range fr.cur.Options {
		if x != a {
			continue
		}
		at := fr.curAt[i]
		if fr.dk != DPriority {
			return *optByIndex(fr.d, at), true
		}
		pe := fr.prio[at]
		switch pe.kind {
		case prOption:
			return *optByIndex(fr.d, pe.opt), true
		case prPlan:
			return decision.Option{Kind: "cast", Obj: pe.pay.Cast.Object, Mode: pe.pay.Cast.CastMode()}, true
		case prLower:
			// a lowered potential play knows its own mode: the multikicked
			// cast canonicalises as "kicked", so the canonical name alone
			// would hide the count ask (CR 702.43)
			if pe.play.Kind == "ability" {
				return decision.Option{Kind: "ability", Obj: pe.play.Obj, Ability: pe.play.Ability}, true
			}
			return decision.Option{Kind: pe.play.Kind, Obj: pe.play.Obj, Mode: pe.play.Mode}, true
		default:
			obj := g.id.obj[a.Src()]
			if a.Kind() == ActActivate {
				return decision.Option{Kind: "ability", Obj: obj, Ability: int(a.Mode())}, true
			}
			return decision.Option{Kind: "cast", Obj: obj, Mode: CastModeName(a.Mode() & 15)}, true
		}
	}
	return decision.Option{}, false
}

// FeatMismatch is the first difference between the two encodings.
type FeatMismatch struct {
	Where string // "state" or "option i"
	Field string
	Gorge string
	Kern  string
}

func (m *FeatMismatch) String() string {
	return fmt.Sprintf("%s %s: gorge %s, kernel %s", m.Where, m.Field, m.Gorge, m.Kern)
}

// CompareState reports the first difference between gorge's state encoding
// and the kernel's (nil when byte-identical).
func CompareState(g *policynet.State, k *feat.State) *FeatMismatch {
	mm := func(field string, a, b any) *FeatMismatch {
		return &FeatMismatch{Where: "state", Field: field, Gorge: fmt.Sprint(a), Kern: fmt.Sprint(b)}
	}
	if len(g.Dense) != feat.DenseWidth {
		return mm("dense width", len(g.Dense), feat.DenseWidth)
	}
	for i, x := range g.Dense {
		if x != k.Dense[i] {
			return mm(fmt.Sprintf("dense[%d]", i), x, k.Dense[i])
		}
	}
	n := min(len(g.Sparse), len(k.Sparse))
	for i := 0; i < n; i++ {
		if g.Sparse[i].Row != k.Sparse[i].Row || g.Sparse[i].Value != k.Sparse[i].Value {
			ga, ka := sparseDiff(g.Sparse, k.Sparse)
			return mm(fmt.Sprintf("sparse[%d] (rows only one side has)", i), ga, ka)
		}
	}
	if len(g.Sparse) != len(k.Sparse) {
		return mm("sparse length", len(g.Sparse), len(k.Sparse))
	}
	_ = n
	if len(g.Cards) != len(k.Cards) {
		return mm("cards length", len(g.Cards), len(k.Cards))
	}
	for i := range g.Cards {
		gc, kc := &g.Cards[i], &k.Cards[i]
		if gc.Group != kc.Group {
			return mm(fmt.Sprintf("cards[%d].group", i), gc.Group, kc.Group)
		}
		for j := 0; j < feat.EntityRawWidth; j++ {
			var x float32
			if j < len(gc.Raw) {
				x = gc.Raw[j]
			}
			if x != kc.Raw[j] {
				return mm(fmt.Sprintf("cards[%d].raw[%d]", i, j), x, kc.Raw[j])
			}
		}
		if len(gc.Rows) != int(kc.NRows) {
			return mm(fmt.Sprintf("cards[%d].rows length", i), len(gc.Rows), kc.NRows)
		}
		for j, f := range gc.Rows {
			if f.Row != kc.Rows[j].Row || f.Value != kc.Rows[j].Value {
				return mm(fmt.Sprintf("cards[%d].rows[%d]", i, j), f, kc.Rows[j])
			}
		}
	}
	return nil
}

// CompareOption reports the first difference between one option's two
// encodings (BotPick is policy-side, never compared).
func CompareOption(i int, g *policynet.Option, k *feat.Option) *FeatMismatch {
	mm := func(field string, a, b any) *FeatMismatch {
		return &FeatMismatch{Where: fmt.Sprintf("option %d", i), Field: field, Gorge: fmt.Sprint(a), Kern: fmt.Sprint(b)}
	}
	if len(g.Slots) != int(k.NSlots) {
		return mm("slots", fmt.Sprint(g.Slots), fmt.Sprint(k.Slots[:k.NSlots]))
	}
	// The slots are one-hot rows the network sums, so they are compared as a
	// set: within the keyword block gorge lists granted keywords in effect
	// order, the kernel in bit order (same rows, same net input).
	gs := make([]int, len(g.Slots))
	ks := make([]int, len(g.Slots))
	for j, f := range g.Slots {
		gs[j] = int(f.Row)<<8 | int(f.Value)
		ks[j] = int(k.Slots[j].Row)<<8 | int(k.Slots[j].Value)
	}
	sort.Ints(gs)
	sort.Ints(ks)
	for j := range gs {
		if gs[j] != ks[j] {
			return mm(fmt.Sprintf("slots[%d]", j), fmt.Sprint(g.Slots), fmt.Sprint(k.Slots[:k.NSlots]))
		}
	}
	if len(g.Hashed) != int(k.NHashed) {
		return mm("hashed", fmt.Sprint(g.Hashed), fmt.Sprint(k.Hashed[:k.NHashed]))
	}
	for j, f := range g.Hashed {
		if f.Row != k.Hashed[j].Row || f.Value != k.Hashed[j].Value {
			return mm(fmt.Sprintf("hashed[%d]", j), fmt.Sprint(g.Hashed), fmt.Sprint(k.Hashed[:k.NHashed]))
		}
	}
	for j := 0; j < feat.OptionDenseWidth; j++ {
		var x float32
		if j < len(g.Dense) {
			x = g.Dense[j]
		}
		if x != k.Dense[j] {
			return mm(fmt.Sprintf("dense[%d]", j), x, k.Dense[j])
		}
	}
	if g.EntA != k.EntA || g.EntB != k.EntB {
		return mm("ent", [2]int32{g.EntA, g.EntB}, [2]int32{k.EntA, k.EntB})
	}
	return nil
}

// FeatStats counts a run's feature comparisons.
type FeatStats struct {
	Decisions, Options int
	// IncFallback counts incremental encodes that took the full rebuild.
	IncFallback int
	// First is the first mismatch.
	First *FeatMismatch
	// Rel marks the relational entity slots seen nonzero (index i is raw
	// slot EntityRawWidthV1+i), so a test can assert its positions exercise
	// the whole format.
	Rel [16]bool
}

// FeatCheck is a RunConfig.Check comparing, at every decision, the real
// kernel's encoded inputs (dirac/feat over its State) with gorge's
// policynet encoding of the same position: the state, and every kernel
// option against gorge's encoding of the entry its canonical action has in
// gorge's net list (GorgeNetList: d.Options plus the synthetic payment
// casts), at that entry's index over that list's length -- the position
// and the length are asserted too. An option the list has no entry for
// (Done, a lowered play) is compared as the list's last entry. With two or
// more synthetic casts the last is named the bot's answer on both sides, so
// the bot-first order is exercised.
func FeatCheck(pool *feat.Pool, st *FeatStats) func(ref *GorgeSide, k Side) *Divergence {
	enc := feat.NewEncoder(pool)
	inc := feat.NewEncoder(pool)
	var kopts []feat.Option
	var gopts, list []decision.Option
	var genc []policynet.Option
	return func(ref *GorgeSide, k Side) *Divergence {
		r, ok := k.(*realSide)
		if !ok || ref.fr == nil {
			return nil
		}
		// gorge poses a multi-pick answer as one decision on one state; the
		// kernel's later picks see the earlier ones applied. The inputs are
		// compared at the first pick.
		if len(ref.fr.chosen) > 0 {
			return nil
		}
		gd := ref.fr.d
		kst := enc.EncodeState(&r.s, &r.d)
		// The incremental encoder must reproduce the full rebuild exactly.
		if m := CompareFeat(kst, inc.EncodeStateInc(&r.s, &r.d)); m != nil {
			m.Where = "incremental " + m.Where
			return &Divergence{What: "features", Field: m.Where + " " + m.Field, Detail: m.String()}
		}
		st.IncFallback = inc.IncStats.Fallback
		// The bot-first rule is exercised too: with two or more synthetic
		// casts the last one is named the bot's answer on both sides.
		list = ref.GorgeNetList(0, list)
		bot, botObj := -1, state.ObjID(0)
		if len(list)-len(gd.Options) >= 2 {
			for i, c := range r.opts {
				if ref.GorgeNetPos(c, list) == len(list)-1 {
					bot, botObj = i, list[len(list)-1].Obj
				}
			}
			list = ref.GorgeNetList(botObj, list)
		}
		kopts = enc.EncodeOptionsBot(&r.d, kopts, bot)
		var gst policynet.State
		var gpos []int
		gst, genc, gopts, gpos, ok = ref.GorgeNetFeatures(r.opts, list, genc, gopts, gpos)
		if !ok {
			return &Divergence{What: "features", Field: "option", Detail: "no gorge option for a kernel option"}
		}
		var m *FeatMismatch
		if enc.ListLen() != len(list) {
			m = &FeatMismatch{Where: "options", Field: "gorge list length", Gorge: fmt.Sprint(len(list)), Kern: fmt.Sprint(enc.ListLen())}
		}
		for i, p := range gpos {
			if m == nil && enc.ListPos(i) != p {
				m = &FeatMismatch{Where: fmt.Sprintf("option %d", i), Field: "gorge list position", Gorge: fmt.Sprint(p), Kern: fmt.Sprint(enc.ListPos(i))}
			}
		}
		st.Decisions++
		st.Options += len(gopts)
		if m == nil {
			m = CompareState(&gst, kst)
		}
		if m == nil {
			for ci := range gst.Cards {
				raw := gst.Cards[ci].Raw
				for j := feat.EntityRawWidthV1; j < feat.EntityRawWidthRelEnd && j < len(raw); j++ {
					if raw[j] != 0 {
						st.Rel[j-feat.EntityRawWidthV1] = true
					}
				}
			}
		}
		for i := 0; m == nil && i < len(genc); i++ {
			m = CompareOption(i, &genc[i], &kopts[i])
		}
		if m == nil {
			return nil
		}
		if st.First == nil {
			st.First = m
		}
		if os.Getenv("KFEAT_DEBUG") != "" {
			fmt.Println("MISMATCH", gd.Kind, r.d.Kind, r.d.Sub, m.String())
			if j := strings.Index(m.Where, "option "); j >= 0 {
				var oi int
				fmt.Sscan(m.Where[j+7:], &oi)
				if oi < len(gopts) {
					fmt.Printf("  GOPT %d: %+v\n", oi, gopts[oi])
					fmt.Printf("  KOPT %d: %08x %v\n", oi, uint32(r.d.Options[oi]), r.opts[oi])
				}
			}
			for j, o := range list {
				fmt.Printf("  g%d %s obj=%d pl=%d ab=%d mode=%q amt=%d atk=%d %q\n", j, o.Kind, o.Obj, o.Player, o.Ability, o.Mode, o.Amount, o.Attacker, o.Label)
			}
			for i := 0; i < int(r.d.N); i++ {
				fmt.Printf("  kopt %d: %08x canon %v %s gorge %s at %d, kernel at %d\n", i, uint32(r.d.Options[i]), r.opts[i], enc.DebugObj(uint8(r.d.Options[i].Target())), gopts[i].Kind, ref.GorgeNetPos(r.opts[i], list), enc.ListPos(i))
			}
			for _, pe := range ref.fr.prio {
				fmt.Printf("  prio %v kind %d\n", pe.a, pe.kind)
			}
			for _, pa := range ref.E.EnsurePaymentActions() {
				fmt.Printf("  payment %s obj=%d base=%v plans=%d\n", pa.Label, pa.Cast.Object, pa.BaseOptionIndex != nil, len(pa.Plans))
			}
			if gd.Kind == decision.KPriority {
				for _, pp := range ref.E.PotentialPaymentPlans(gd.Player) {
					fmt.Printf("  potential %s %s reason=%q detail=%q plan=%v script=%d\n", pp.Action.Kind, pp.Action.Label, pp.Reason, pp.Detail, pp.Plan != nil, len(pp.Script))
				}
			}
			fmt.Println("GORGE ", gorgeDebug(ref, gd))
			fmt.Println("KERNEL", enc.DebugView())
		}
		return &Divergence{What: "features", Field: m.Where + " " + m.Field,
			Detail: fmt.Sprintf("%s decision for p%d, turn %d step %v: %s", gd.Kind, gd.Player, ref.E.G.Turn, ref.E.G.Step, m.String())}
	}
}

// sparseDiff is the multiset difference of two bags: the entries only
// gorge has, and only the kernel has.
func sparseDiff(g []policynet.Feature, k []feat.Feature) (onlyG, onlyK []policynet.Feature) {
	used := make([]bool, len(k))
	for _, f := range g {
		hit := false
		for j, x := range k {
			if !used[j] && x.Row == f.Row && x.Value == f.Value {
				used[j], hit = true, true
				break
			}
		}
		if !hit {
			onlyG = append(onlyG, f)
		}
	}
	for j, x := range k {
		if !used[j] {
			onlyK = append(onlyK, policynet.Feature{Row: x.Row, Value: x.Value})
		}
	}
	return
}

// optionsDebug prints both boards and gorge's payment and potential plans
// at an options divergence (KFEAT_DEBUG).
func optionsDebug(ref *GorgeSide) {
	if ref.fr == nil {
		return
	}
	gd := ref.fr.d
	for i, o := range gd.Options {
		fmt.Printf("  raw %d %s obj=%d ab=%d mode=%q cost=%q %q\n", i, o.Kind, o.Obj, o.Ability, o.Mode, o.Cost, o.Label)
	}
	for _, pe := range ref.fr.prio {
		fmt.Printf("  prio %v kind %d\n", pe.a, pe.kind)
	}
	for _, pa := range ref.E.EnsurePaymentActions() {
		fmt.Printf("  payment %s obj=%d base=%v plans=%d\n", pa.Label, pa.Cast.Object, pa.BaseOptionIndex != nil, len(pa.Plans))
	}
	if gd.Kind == decision.KPriority {
		for _, pp := range ref.E.PotentialPaymentPlans(gd.Player) {
			fmt.Printf("  potential %s %s reason=%q detail=%q plan=%v script=%d\n", pp.Action.Kind, pp.Action.Label, pp.Reason, pp.Detail, pp.Plan != nil, len(pp.Script))
		}
	}
	fmt.Println("GORGE ", gorgeDebug(ref, gd))
}

func gorgeDebug(ref *GorgeSide, gd *decision.Decision) string {
	v := view.Project(ref.E.G, ref.E, gd.Player, gd)
	s := fmt.Sprintf("turn %d step %s active %d prio %d\n", v.Turn, v.Step, v.Active, v.Priority)
	card := func(cv *view.CardView) string {
		return fmt.Sprintf("%s#%d[c%d t%v s%v a%v blk%v %d/%d d%d kw=%v ctr=%v att=%d types=%q]", cv.Name, cv.ID, cv.Controller, cv.Tapped, cv.SummonSick, cv.Attacking, len(cv.BlockedBy) > 0, cv.Power, cv.Toughness, cv.Damage, cv.Keywords, cv.Counters, cv.AttachedTo, cv.Types)
	}
	for _, pv := range v.Players {
		s += fmt.Sprintf(" p%d life %d hand %d lib %d grave %d pool %v avail %v\n  bf:", pv.ID, pv.Life, pv.HandSize, pv.LibrarySize, pv.GraveyardSize, pv.Pool, pv.Available)
		for i := range pv.Battlefield {
			s += " " + card(&pv.Battlefield[i])
		}
		s += "\n  hand:"
		for i := range pv.Hand {
			s += " " + card(&pv.Hand[i])
		}
		s += "\n  grave:"
		for i := range pv.Graveyard {
			s += " " + card(&pv.Graveyard[i])
		}
		s += "\n  exile:"
		for i := range pv.Exile {
			s += " " + card(&pv.Exile[i])
		}
		s += "\n"
	}
	s += " stack:"
	for _, st := range v.Stack {
		s += " " + st.Name
		if st.Card != nil {
			s += "=" + card(st.Card)
		}
	}
	return s
}

// CompareFeat reports the first difference between two kernel-side state
// encodings (the full rebuild "gorge" side, the incremental "kernel" side).
func CompareFeat(a, b *feat.State) *FeatMismatch {
	mm := func(field string, x, y any) *FeatMismatch {
		return &FeatMismatch{Where: "state", Field: field, Gorge: fmt.Sprint(x), Kern: fmt.Sprint(y)}
	}
	if a.Dense != b.Dense {
		return mm("dense", a.Dense, b.Dense)
	}
	if len(a.Sparse) != len(b.Sparse) {
		return mm("sparse length", a.Sparse, b.Sparse)
	}
	for i := range a.Sparse {
		if a.Sparse[i] != b.Sparse[i] {
			return mm(fmt.Sprintf("sparse[%d]", i), a.Sparse[i], b.Sparse[i])
		}
	}
	if len(a.Cards) != len(b.Cards) {
		return mm("cards length", len(a.Cards), len(b.Cards))
	}
	for i := range a.Cards {
		if a.Cards[i] != b.Cards[i] {
			return mm(fmt.Sprintf("cards[%d]", i), a.Cards[i], b.Cards[i])
		}
	}
	return nil
}

// GorgeNetList is the option list gorge's network scores at the frame's
// decision (azmcts priorsWith in its payCasts form, the PriorTopK search's
// prior): d.Options, then -- at a priority decision -- one synthetic cast
// (payCastOption) per planner-paid cast with no legacy cast option
// (BaseOptionIndex nil), the bot's (botObj; 0 none) first and the rest in
// EnsurePaymentActions order, one per object.
func (g *GorgeSide) GorgeNetList(botObj state.ObjID, dst []decision.Option) []decision.Option {
	fr := g.fr
	dst = append(dst[:0], fr.d.Options...)
	if fr.dk != DPriority {
		return dst
	}
	base := len(dst)
	seen := map[state.ObjID]bool{}
	for _, a := range g.E.EnsurePaymentActions() {
		if len(a.Plans) == 0 || seen[a.Cast.Object] {
			continue
		}
		seen[a.Cast.Object] = true
		if a.BaseOptionIndex != nil {
			continue
		}
		o := decision.Option{Kind: "cast", Obj: a.Cast.Object, Player: fr.d.Player, Label: a.Label}
		if a.Cast.Object == botObj && len(dst) > base {
			dst = append(dst, decision.Option{})
			copy(dst[base+1:], dst[base:])
			dst[base] = o
			continue
		}
		dst = append(dst, o)
	}
	for i := base; i < len(dst); i++ {
		dst[i].Index = i
	}
	return dst
}

// GorgeNetFeatures is gorge's FeaturesEntity encoding of the frame's
// position for its deciding seat: the state, and for each canonical action
// in cs gorge's encoding of its entry in list (GorgeNetList) at that entry's
// index over len(list) -- an action list has no entry for (Done, a lowered
// play) is encoded as the list's last entry. pos[i] is the index used. ok
// is false when an action names no gorge option.
func (g *GorgeSide) GorgeNetFeatures(cs []Canon, list []decision.Option, genc []policynet.Option, gopts []decision.Option, pos []int) (policynet.State, []policynet.Option, []decision.Option, []int, bool) {
	gd := g.fr.d
	v := view.Project(g.E.G, g.E, gd.Player, gd)
	fs := policynet.FeaturesEntity
	gst := policynet.EncodeStateWith(fs, v, gd.Player, nil)
	var x policynet.OptionEncoder
	x.Reset(&v, gd.Player)
	genc, gopts, pos = genc[:0], gopts[:0], pos[:0]
	for _, c := range cs {
		p := g.GorgeNetPos(c, list)
		var o decision.Option
		if p >= 0 {
			o = list[p]
		} else {
			var ok bool
			if o, ok = g.GorgeOption(c); !ok {
				return gst, genc, gopts, pos, false
			}
			p = len(list) - 1
		}
		gopts = append(gopts, o)
		pos = append(pos, p)
		genc = append(genc, x.Encode(fs, gd.Kind, o, p, len(list)))
	}
	return gst, genc, gopts, pos, true
}

// GorgeNetPos is the position of canonical action a in list (GorgeNetList
// at the current frame): -1 for an action the list has no option for
// (Done; a lowered or scripted play gorge's search never scores).
func (g *GorgeSide) GorgeNetPos(a Canon, list []decision.Option) int {
	fr := g.fr
	if a.Kind() == ActDone {
		return -1
	}
	pos := func(idx int) int {
		for i := range fr.d.Options {
			if fr.d.Options[i].Index == idx {
				return i
			}
		}
		return -1
	}
	for i, x := range fr.cur.Options {
		if x != a {
			continue
		}
		at := fr.curAt[i]
		if fr.dk != DPriority {
			return pos(at)
		}
		pe := fr.prio[at]
		switch pe.kind {
		case prOption:
			return pos(pe.opt)
		case prPlan:
			if pe.pay.BaseOptionIndex != nil {
				return pos(*pe.pay.BaseOptionIndex)
			}
			for j := len(fr.d.Options); j < len(list); j++ {
				if list[j].Obj == pe.pay.Cast.Object {
					return j
				}
			}
		}
		return -1
	}
	return -1
}

// kernelTypeLine is a kernel card definition's type line in gorge's
// spelling: its card types then its subtypes.
func kernelTypeLine(tbl *kop.Table, cd *kop.CardDef) string {
	var w []string
	for _, t := range []struct {
		bit  kop.TypeMask
		name string
	}{{kop.TLegendary, "Legendary"}, {kop.TBasic, "Basic"}, {kop.TArtifact, "Artifact"}, {kop.TEnchantment, "Enchantment"}, {kop.TLand, "Land"}, {kop.TCreature, "Creature"}, {kop.TPlaneswalker, "Planeswalker"}, {kop.TInstant, "Instant"}, {kop.TSorcery, "Sorcery"}} {
		if cd.Types&t.bit != 0 {
			w = append(w, t.name)
		}
	}
	for i, s := range tbl.Subtypes {
		if cd.Subtypes.Has(i) {
			w = append(w, s)
		}
	}
	return strings.Join(w, " ")
}

// effectSpec classifies a card's first spell ability (a SubAbility$ chain is
// not read, so a kill-plus-draw card is its first half only).
func effectSpec(sa *cards.SA, manaCost string) feat.EffectSpec {
	e := feat.EffectSpec{Class: feat.EffOther, Amount: -2, CostX: strings.Contains(manaCost, "X")}
	if sa == nil {
		return e
	}
	p := sa.Params
	tg := p["ValidTgts"]
	e.TgtPlayer = strings.Contains(tg, "Player") || strings.Contains(tg, "Opponent") || strings.Contains(tg, "Any")
	e.TgtCreat = strings.Contains(tg, "Creature") || strings.Contains(tg, "Any")
	amount := func(s string) int16 {
		if s == "X" {
			return -1
		}
		if n, err := strconv.Atoi(s); err == nil && n >= -32768 && n <= 32767 {
			return int16(n)
		}
		return -2
	}
	switch sa.API {
	case "Destroy", "Sacrifice":
		e.Class = feat.EffKill
	case "ChangeZone":
		switch p["Destination"] {
		case "Exile":
			if tg != "" {
				e.Class, e.Exile = feat.EffKill, true
			}
		case "Hand", "Library":
			e.Class = feat.EffBounce
		}
	case "DealDamage":
		e.Class, e.Amount = feat.EffDamage, amount(p["NumDmg"])
	case "Counter":
		e.Class = feat.EffCounter
	case "Protection":
		e.Class = feat.EffPump
	case "Tap":
		e.Class = feat.EffDebuff
	case "Draw":
		e.Amount = amount(p["NumCards"])
	case "Pump":
		// signed value of a Pump number: an integer, else 0 (X and
		// expressions) with a leading "-" counting as -1 (a debuff).
		sv := func(s string) int {
			if n, err := strconv.Atoi(s); err == nil {
				return n
			}
			if strings.HasPrefix(s, "-") {
				return -1
			}
			return 0
		}
		a, d := sv(p["NumAtt"]), sv(p["NumDef"])
		e.NumAtt, e.NumDef = int8(max(-9, min(9, a))), int8(max(-9, min(9, d)))
		switch {
		case a < 0 || d < 0:
			e.Class = feat.EffDebuff
			// Amount = |NumDef|: an integer NumDef gives its magnitude,
			// "-X" gives -1, any other expression (or absent) -2.
			e.Amount = amount(strings.TrimPrefix(p["NumDef"], "-"))
			if n, err := strconv.Atoi(p["NumDef"]); err == nil {
				e.Amount = int16(min(max(n, -n), 1<<15-1))
			}
		case a != 0 || d != 0:
			e.Class = feat.EffPump
		}
	}
	return e
}
