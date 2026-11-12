package kgen

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

// pseudoSubtype reports whether w is a Forge dynamic type word rather than a
// literal subtype: ChosenType names the creature type a ChooseType picked
// (Mistform Dreamer's Animate, Leyline of Transformation's AddType) and
// ChosenType2 the second choice (Illusionary Terrain). Lowering either as a
// literal subtype would give the permanent a pseudo-subtype, so no lowering
// path may treat them as known.
func pseudoSubtype(w string) bool {
	switch w {
	case "ChosenType", "ChosenType2":
		return true
	}
	return false
}

// unmodeledSupertype reports whether w is a supertype the kernel's type mask
// does not model. CR 205.4a: the supertypes are basic, legendary, ongoing,
// snow and world; typeBits carries Basic and Legendary only. A supertype is
// not a subtype (CR 205.4b), so it is never interned from a type line or an
// AddType: genPrep would otherwise mark it a creature type (dirac/gen.go
// creatSub) and give it to every Changeling (CR 702.73a).
func unmodeledSupertype(w string) bool {
	switch w {
	case "Snow", "World", "Ongoing":
		return true
	}
	return false
}

// knownSubtype reports whether w is a subtype word of the pool: a word on
// some pool card's or token's type line (a supertype typeBits does not model
// is never a subtype, CR 205.4a-b), one a pool effect sets, or a
// non-creature subtype in the corpus's whole type vocabulary (gorge's
// predicateTypeWords, engine/effects/typewords.go:3-9, from the corpus Types
// lines; the pool's own type lines miss words no pool card carries, Room and
// Lesson in a deck pool). The vocabulary arm takes only words
// dirac.NonCreatureSubtype names: genPrep marks every other table subtype
// as a creature type (dirac/gen.go:346-349, creatSub), and changelingSubs
// gives a creature type to every Changeling (CR 702.73a), so a word outside
// the map must stay unknown rather than widen that grant (CR 205.3m).
func (p *pool) knownSubtype(w string) bool {
	if pseudoSubtype(w) {
		return false
	}
	if effects.CreatureTypeWords(w) {
		// a creature type no pool card carries (Spider-Ham's Jackal): an
		// ordinary type word in gorge (typewords.go, the corpus
		// vocabulary); the build's miss pass puts it in the subtype table
		// and changelingSubs gives it to every Changeling (CR 702.73a)
		return true
	}
	if p.typeWords != nil && p.typeWords[w] {
		if _, cardType := typeBits[w]; !cardType && dirac.NonCreatureSubtype(w) {
			// a non-creature subtype no pool card carries: gorge's
			// hasTypeCtx reads it as an ordinary type word and it matches
			// nothing; the kernel's PredSubtype likewise matches nothing
			// (the word is in the matchup table only as a filter reference)
			return true
		}
	}
	if p.subs == nil {
		p.subs = map[string]bool{}
		add := func(c *cards.Card) {
			for _, f := range c.Faces {
				for _, t := range f.Types {
					if _, ok := typeBits[t]; !ok && !unmodeledSupertype(t) {
						p.subs[t] = true
					}
				}
				var walk func(m map[string]string)
				walk = func(m map[string]string) {
					for _, k := range []string{"Types", "AddType", "AddTypes"} {
						for _, w := range strings.FieldsFunc(m[k], func(r rune) bool { return r == ',' || r == '&' }) {
							w = strings.TrimSpace(w)
							if _, ok := typeBits[w]; !ok && w != "" && !pseudoSubtype(w) && !unmodeledSupertype(w) {
								p.subs[w] = true
							}
						}
					}
				}
				for _, a := range f.Abilities {
					for s := a; s != nil; s = s.Sub {
						walk(s.Params)
					}
				}
				for _, s := range f.Statics {
					walk(s.Params)
				}
				for _, body := range f.SVars {
					for _, part := range strings.Split(body, "|") {
						k, v, ok := strings.Cut(strings.TrimSpace(part), "$")
						if ok {
							walk(map[string]string{strings.TrimSpace(k): strings.TrimSpace(v)})
						}
					}
				}
			}
		}
		for _, n := range p.names {
			add(p.byName[n])
		}
		for _, c := range p.reg.Tokens {
			add(c)
		}
	}
	return p.subs[w]
}

// corpusTypeWords reads the corpus's type vocabulary: every word on a
// compiled card's or token's type line (gorge effects typewords.go builds
// its predicateTypeWords from the same source, the corpus Types lines). A
// process reads each corpus once (SharedCorpus) and the walk is memoised
// per directory.
var corpusWords = struct {
	sync.Mutex
	m map[string]map[string]bool
}{m: map[string]map[string]bool{}}

func corpusTypeWords(dir string) (map[string]bool, error) {
	corpusWords.Lock()
	defer corpusWords.Unlock()
	if w, ok := corpusWords.m[dir]; ok {
		return w, nil
	}
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		return nil, err
	}
	w := map[string]bool{}
	add := func(c *cards.Card) {
		if c == nil {
			return
		}
		for _, f := range c.Faces {
			if f == nil {
				continue
			}
			for _, t := range f.Types {
				w[t] = true
			}
		}
	}
	for _, c := range reg.AllCards() {
		add(c)
	}
	for _, c := range reg.Tokens {
		add(c)
	}
	corpusWords.m[dir] = w
	return w, nil
}

// isTypeWord reports whether w is a type word the filter's alternative
// boundary may start (startsFilterAlternative): the corpus's type
// vocabulary on a loaded pool, gorge's own predicateTypeWords' source, so
// a comma after a named<Name> predicate splits exactly where gorge's
// rawNameComma splits it (findings r2: the pool's subtypes are not the
// corpus's vocabulary). A synthetic pool without a corpus keeps the pool's
// knownSubtype fallback.
func (p *pool) isTypeWord(w string) bool {
	if _, ok := typeBits[w]; ok {
		return true
	}
	if p.typeWords == nil {
		return p.knownSubtype(w)
	}
	return p.typeWords[w]
}

// lowered is the whole pool's lowering.
type lowered struct {
	cards  []*lCard // pool cards sorted by name
	tokens []*lCard // tokens the supported cards create, sorted by key
}

func lowerPool(p *pool) *lowered {
	lw := &lowered{}
	tokSet := map[string]bool{}
	for _, n := range p.names {
		lcard := lowerCard(p, n, p.byName[n], false)
		lw.cards = append(lw.cards, lcard)
		for _, t := range lcard.Tokens {
			tokSet[t] = true
		}
	}
	injectEquipGrants(lw.cards)
	var keys []string
	for k := range tokSet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.HasPrefix(k, faceKeyPrefix) {
			lw.tokens = append(lw.tokens, lowerFaceEntry(p, k))
			continue
		}
		if strings.HasPrefix(k, fuseKeyPrefix) {
			lw.tokens = append(lw.tokens, lowerFusedEntry(p, k)) // lower_fuse.go
			continue
		}
		if strings.HasPrefix(k, roomKeyPrefix) {
			lw.tokens = append(lw.tokens, lowerRoomEntry(p, k)) // lower_rooms.go
			continue
		}
		if k == undercityKey {
			lw.tokens = append(lw.tokens, lowerDungeonEntry(p, k)) // lower_initiative.go
			continue
		}
		if strings.HasPrefix(k, nameCardKey) {
			lw.tokens = append(lw.tokens, nameCard(lw.cards, k))
			continue
		}
		if strings.HasPrefix(k, copyCardKey) {
			lw.tokens = append(lw.tokens, lowerCopyCardEntry(p, k)) // lower_copynamed.go
			continue
		}
		c, _ := p.reg.Token(k)
		lw.tokens = append(lw.tokens, lowerCard(p, k, c, true))
	}
	// A card whose token is refused is refused too.
	refusedTok := map[string]bool{}
	for _, t := range lw.tokens {
		if len(t.Refusals) > 0 {
			refusedTok[t.Name] = true
		}
	}
	for _, c := range lw.cards {
		for _, t := range c.Tokens {
			if refusedTok[t] {
				c.Refusals = append(c.Refusals, refusal{Card: c.Name, Node: "token " + t, Reason: "token refused"})
			}
		}
	}
	return lw
}

// dumpLIR writes a readable LIR listing.
func dumpLIR(w io.Writer, c *lCard) {
	fmt.Fprintf(w, "CARD %s token=%v cost=%s types=%#x sub=%v colors=%q pt=%d/%d star=%v loyalty=%d kw=%v flash=%v",
		c.Name, c.Token, c.Cost, c.Types, c.Subtypes, c.Colors, c.P, c.T, c.PTStar, c.Loyalty, c.Kw, c.Flash)
	if c.Kicker != nil {
		fmt.Fprintf(w, " kicker=%s", c.Kicker)
	}
	if c.Flashback != nil {
		fmt.Fprintf(w, " flashback=%s", c.Flashback)
	}
	if c.RemPersist {
		fmt.Fprintf(w, " rempersist")
		if c.RemSeed != ^uint32(0) || c.RemKeep != ^uint32(0) {
			fmt.Fprintf(w, " seed=%#x keep=%#x", c.RemSeed, c.RemKeep)
		}
	}
	if c.ExiledWith {
		fmt.Fprintf(w, " exiledwith")
	}
	if c.CombatHitRec {
		fmt.Fprintf(w, " combathitrec")
	}
	if c.SpendRead {
		fmt.Fprintf(w, " spendread")
	}
	if c.FbCost != nil {
		fmt.Fprintf(w, " fbcost=%s", costString(*c.FbCost))
	}
	if c.Madness != nil {
		fmt.Fprintf(w, " madness=%s", c.Madness)
	}
	if c.Miracle != nil {
		fmt.Fprintf(w, " miracle=%s", c.Miracle)
	}
	if c.Warp != nil {
		fmt.Fprintf(w, " warp=%s", c.Warp)
	}
	if c.Impending != nil {
		fmt.Fprintf(w, " impending=%d:%s", c.ImpendingN, c.Impending)
	}
	if c.Plot != nil {
		fmt.Fprintf(w, " plot=%s", c.Plot)
	}
	if c.Suspend != nil {
		fmt.Fprintf(w, " suspend=%d:%s", c.SuspendN, c.Suspend)
	}
	if c.Foretell != nil {
		fmt.Fprintf(w, " foretell=%s", c.Foretell)
	}
	if c.Paradigm {
		fmt.Fprintf(w, " paradigm")
	}
	if c.Sneak != nil {
		fmt.Fprintf(w, " sneak=%s", c.Sneak)
	}
	if c.WebSling != nil {
		fmt.Fprintf(w, " webslinging=%s", c.WebSling)
	}
	if c.MayFlash != nil {
		fmt.Fprintf(w, " mayflash=%s", c.MayFlash)
		if c.MayFlashBehold != nil {
			fmt.Fprintf(w, "+behold1%s", c.MayFlashBehold)
		}
	}
	if c.FbBeholdN != 0 {
		fmt.Fprintf(w, " fbbehold=%d%s", c.FbBeholdN, c.FbBeholdF)
	}
	if c.Evoke != nil {
		fmt.Fprintf(w, " evoke=%s", c.Evoke)
	}
	if c.Dash != nil {
		fmt.Fprintf(w, " dash=%s", c.Dash)
	}
	if c.Convoke {
		fmt.Fprintf(w, " convoke")
	}
	if c.Improvise {
		fmt.Fprintf(w, " improvise")
	}
	if c.Delve {
		fmt.Fprintf(w, " delve")
	}
	if c.Rebound != nil {
		fmt.Fprintf(w, " rebound")
	}
	if c.Harmonize != nil {
		fmt.Fprintf(w, " harmonize=%s", c.Harmonize)
	}
	if c.Retrace {
		fmt.Fprintf(w, " retrace")
		if c.RetraceCost != nil {
			fmt.Fprintf(w, " retracecost=%s", costString(*c.RetraceCost))
		}
	}
	if c.BeamMeUp != nil {
		fmt.Fprintf(w, " beammeup=%s", c.BeamMeUp)
	}
	if c.Mayhem != nil {
		fmt.Fprintf(w, " mayhem=%s", c.Mayhem)
	}
	if c.SpellCost != nil {
		fmt.Fprintf(w, " spellcost=%s", costString(*c.SpellCost))
	}
	if c.AltAddCost != nil {
		fmt.Fprintf(w, " altaddcost=%s", costString(*c.AltAddCost))
	}
	if c.KickerCost != nil {
		fmt.Fprintf(w, " kickercost=%s", costString(*c.KickerCost))
	}
	if c.BeholdExile {
		fmt.Fprintf(w, " beholdexile=%d%s", c.BeholdN, c.BeholdF)
	}
	if c.SpellWaterbend != 0 {
		fmt.Fprintf(w, " spellwaterbend=%d", c.SpellWaterbend)
	}
	if c.OptCost != nil {
		fmt.Fprintf(w, " optcost=blight%d/behold%d%s", c.OptCost.BlightN, c.OptCost.BeholdN, c.OptCost.BeholdF)
		if c.OptCost.Waterbend != 0 {
			fmt.Fprintf(w, " optwaterbend=%d", c.OptCost.Waterbend)
		}
	}
	if c.Leyline {
		fmt.Fprintf(w, " leyline")
	}
	if c.SelfFlash {
		fmt.Fprintf(w, " selfflash")
	}
	if c.Landwalk != nil {
		fmt.Fprintf(w, " landwalk=%s", c.Landwalk)
	}
	if c.Casualty != 0 {
		fmt.Fprintf(w, " casualty=%d", c.Casualty)
	}
	if c.Wither {
		fmt.Fprintf(w, " wither")
	}
	if c.AloneAtk {
		fmt.Fprintf(w, " alone-atk")
	}
	if c.AloneBlk {
		fmt.Fprintf(w, " alone-blk")
	}
	if c.Engines {
		fmt.Fprintf(w, " engines")
	}
	if c.Evasion != 0 {
		fmt.Fprintf(w, " evasion=%d", c.Evasion)
	}
	if c.Daybound {
		fmt.Fprintf(w, " daybound")
	}
	if c.Nightbound {
		fmt.Fprintf(w, " nightbound")
	}
	if c.Lure {
		fmt.Fprintf(w, " lure=true")
	}
	if c.MustBeBlocked {
		fmt.Fprintf(w, " mustbeblocked=true")
	}
	if c.MinBlockers != 0 {
		fmt.Fprintf(w, " minblockers=%d", c.MinBlockers)
	}
	if c.ExtraBlocks != 0 {
		fmt.Fprintf(w, " extrablocks=%d", c.ExtraBlocks)
	}
	if c.ProtFrom != nil {
		fmt.Fprintf(w, " protfrom=%s", c.ProtFrom)
	}
	if c.HexColors != "" {
		fmt.Fprintf(w, " hexproof=%s", c.HexColors)
	}
	if c.ProtColors != "" {
		fmt.Fprintf(w, " protection=%s", c.ProtColors)
	}
	if c.Infect {
		fmt.Fprintf(w, " infect")
	}
	if c.Toxic != 0 {
		fmt.Fprintf(w, " toxic=%d", c.Toxic)
	}
	if c.Cascade != 0 {
		fmt.Fprintf(w, " cascade=%d", c.Cascade)
	}
	fmt.Fprintln(w)
	for _, a := range c.Abilities {
		fmt.Fprintf(w, "  ab[%d] %s %s cost=%s sorcery=%v zone=%d loyalty=%v limit=%d modes=%d\n", a.Index,
			abilityKindNames[a.Kind], a.Name, costString(a.Cost), a.Sorcery, a.Zone, a.Loyalty, a.Limit, a.Modes)
		if a.TurnOnce || a.ActCond.Kind != cNone {
			fmt.Fprintf(w, "    activate turnonce=%v cond=%s\n", a.TurnOnce, a.ActCond)
		}
		if a.Trigger != nil {
			t := a.Trigger
			fmt.Fprintf(w, "    trigger %s F=%s F2=%s whose=%d zone=%d step=%s combat=%d opt=%v cond=%s nth=%d first=%v once=%v yourturn=%v oppturn=%v minatk=%d\n",
				t.Event, t.F, t.F2, t.Whose, t.Zone, t.Step, t.Combat, t.Optional, t.Cond, t.NthDraw, t.First, t.OncePer, t.OnlyYourTurn, t.OnlyOppTurn, t.MinAttackers)
			if t.AtkCmp != 0 || t.NthCmp != 0 {
				fmt.Fprintf(w, "    trigcmp atk=%d nth=%d\n", t.AtkCmp, t.NthCmp)
			}
			if t.SrcInPlay {
				fmt.Fprintf(w, "    src-in-play\n")
			}
			if t.PerRef {
				fmt.Fprintf(w, "    per-referent gate=%d\n", t.DmgGate)
			}
			if t.Cycled {
				fmt.Fprintf(w, "    cycled\n")
			}
			if t.ResolvedLimit {
				fmt.Fprintf(w, "    resolvedlimit=1\n")
			}
			if t.Unblocked {
				fmt.Fprintf(w, "    unblocked\n")
			}
			if t.OrCopy {
				fmt.Fprintf(w, "    or-copy\n")
			}
			if t.DefPlayer != 0 {
				fmt.Fprintf(w, "    defender-player=%d\n", t.DefPlayer)
			}
			if t.ByCreature {
				fmt.Fprintf(w, "    pairs anchor-blocker=%v\n", t.AnchorBlocker)
			}
		}
		if a.Mana != nil {
			fmt.Fprintf(w, "    mana %s x%s restrict=%s\n", a.Mana.Colors, a.Mana.Amount, a.Mana.Restrict)
		}
		for i, t := range a.Targets {
			fmt.Fprintf(w, "    target[%d] mode=%d zone=%d min=%s max=%s F=%s unique=%v same=%v diff=%v divided=%d spell=%v%s\n",
				i, t.Mode, t.Zone, t.Min, t.Max, t.F, t.Unique, t.SameCtrl, t.DiffCtrl, t.Divided, t.Spell, tgtSetLIR(t))
			if t.TrigOnly {
				fmt.Fprintf(w, "      trigonly=true\n")
			}
			if t.TrigChooser {
				fmt.Fprintf(w, "      trigchooser=true\n")
			}
			if t.VTgt != nil {
				fmt.Fprintf(w, "      vtgting=true F=%s\n", t.VTgt)
			}
			if t.VYou {
				fmt.Fprintf(w, "      vtgting=you\n")
			}
			if t.VSpellF != nil {
				fmt.Fprintf(w, "      vspell F=%s\n", t.VSpellF)
			}
		}
		for i, in := range a.Prog {
			fmt.Fprintf(w, "    %2d %s\n", i, in)
		}
	}
	for _, s := range c.Statics {
		fmt.Fprintf(w, "  static %s F=%s F2=%s cond=%s P=%s T=%s kw=%v addsub=%v amount=%s zone=%d", s.Kind, s.F, s.F2, s.Cond, s.Power, s.Tough, s.Kw, s.AddSub, s.Amount, s.Zone)
		if s.AddChosen {
			fmt.Fprintf(w, " addchosen")
		}
		if s.ProtFrom != nil {
			fmt.Fprintf(w, " protfrom=%s", s.ProtFrom)
		}
		if s.Landwalk != nil {
			fmt.Fprintf(w, " landwalk=%s", s.Landwalk)
		}
		if s.OutlivesLoss {
			fmt.Fprintf(w, " outlivesloss")
		}
		if s.PwVsSrc != 0 {
			fmt.Fprintf(w, " pwvs=%d", s.PwVsSrc)
		}
		if s.OvGate != 0 {
			fmt.Fprintf(w, " ovgate=%d", s.OvGate)
		}
		if s.Kind == "CantPutCounter" {
			fmt.Fprintf(w, " ctr=%d", s.Ctr)
		}
		if s.Kind == "FbGrant" && len(s.Flags) != 0 {
			fmt.Fprintf(w, " fbcost=%s", s.Cost.Mana)
		}
		if s.Kind == "WarpGrant" {
			fmt.Fprintf(w, " warpcost=%s", s.Cost.Mana)
		}
		if s.Override != nil {
			fmt.Fprintf(w, " %s", overrideString(s.Override))
		}
		if (s.Kind == "RaiseCost" || s.Kind == "CantBeCast" || s.Kind == "CantBeActivated") && len(s.Flags) > 0 {
			fmt.Fprintf(w, " flags=%v", s.Flags)
		}
		if !s.Cost.Mana.None || s.Cost.Tap {
			fmt.Fprintf(w, " cost=%s", costString(s.Cost))
		}
		fmt.Fprintln(w)
	}
	for _, r := range c.Repls {
		fmt.Fprintf(w, "  repl %s F=%s cond=%s\n", r.Kind, r.F, r.Cond)
		for i, in := range r.Prog {
			fmt.Fprintf(w, "    %2d %s\n", i, in)
		}
	}
	for i, e := range c.Emblems {
		eot := ""
		if e.UntilEOT {
			eot = " until=EOT"
		} else if e.UntilNextTurn {
			eot = " until=next"
		}
		if e.CaptureSet {
			if e.CaptureIsPlayer {
				eot += " remembers-player=" + e.Capture.String()
			} else {
				eot += " remembers=" + e.Capture.String()
			}
			if e.CaptureLapse {
				eot += " lapses"
			}
		}
		if e.Capture2Set {
			eot += " and " + e.Capture2.String()
		}
		for _, f := range [...]struct {
			on   bool
			name string
		}{{e.CaptureRem, "remembers=set"}, {e.Source, "source"}, {e.NotHere, "not-here"}, {e.NumAmount, "number-as-amount"}, {e.Once, "once"}} {
			if f.on {
				eot += " " + f.name
			}
		}
		fmt.Fprintf(w, "  emblem[%d] trigger=%d statics=%d%s\n", i, e.Trigger, len(e.Statics), eot)
	}
	for _, r := range c.Refusals {
		fmt.Fprintf(w, "  REFUSED %s: %s\n", r.Node, r.Reason)
	}
}

func costString(c lCost) string {
	var p []string
	if !c.Mana.None {
		p = append(p, c.Mana.String())
	}
	if c.Tap {
		p = append(p, "T")
	}
	if c.SacSelf {
		p = append(p, "SacSelf")
	}
	if c.Sac != nil {
		p = append(p, fmt.Sprintf("Sac%d%s", c.SacN, c.Sac))
	}
	if c.SacX {
		p = append(p, "SacX")
	}
	if c.XMin > 0 {
		p = append(p, fmt.Sprintf("XMin%d", c.XMin))
	}
	if c.DiscardN > 0 {
		p = append(p, fmt.Sprintf("Discard%d", c.DiscardN))
	}
	if c.DiscardSelf {
		p = append(p, "DiscardSelf")
	}
	if c.DiscardF != nil {
		p = append(p, fmt.Sprintf("Discard%d%s", c.DiscardN, c.DiscardF))
	}
	if c.Loyalty != 0 {
		p = append(p, fmt.Sprintf("Loyalty%+d", c.Loyalty))
	}
	if c.SubCtr > 0 {
		p = append(p, fmt.Sprintf("Remove%d%s", c.SubCtr, c.SubCtrKnd))
	}
	if c.PayLife > 0 {
		p = append(p, fmt.Sprintf("Life%d", c.PayLife))
	}
	if c.LifeX {
		p = append(p, "LifeX")
	}
	if c.TapN > 0 {
		p = append(p, fmt.Sprintf("Tap%d%s", c.TapN, c.TapF))
	}
	if c.TapHost {
		p = append(p, "TapHost")
	}
	if c.SacHost {
		p = append(p, "SacHost")
	}
	if c.HostExile {
		p = append(p, "HostExile")
	}
	if c.SacExile {
		p = append(p, "SacExile")
	}
	if c.TapPowN > 0 {
		p = append(p, fmt.Sprintf("TapPow%d%s", c.TapPowN, c.TapPowF))
	}
	if c.Forage != nil {
		p = append(p, "Forage")
	}
	if c.Energy > 0 {
		p = append(p, fmt.Sprintf("Energy%d", c.Energy))
	}
	if c.ReturnSelf {
		p = append(p, "ReturnSelf")
	}
	if c.ExileSelf {
		p = append(p, "ExileSelf")
	}
	if c.Return != nil {
		p = append(p, fmt.Sprintf("Return%d%s", c.ReturnN, c.Return))
	}
	if c.ExileG != nil {
		if c.ExileGSelf {
			p = append(p, "ExileGraveSelf")
		}
		p = append(p, fmt.Sprintf("ExileGrave%d%s", c.ExileGN, c.ExileG))
	}
	if c.Untap {
		p = append(p, "Q")
	}
	if c.DrawN > 0 {
		p = append(p, fmt.Sprintf("Draw%d", c.DrawN))
	}
	if len(p) == 0 {
		return "-"
	}
	return strings.Join(p, " ")
}

// nameCardKey prefixes the token key of an override's name card.
const nameCardKey = "name:"

// nameCard is the card definition an override's SetName carries: the new
// name and the override's type line, colours and P/T, no abilities (the
// first override in the pool naming it).
func nameCard(cards []*lCard, key string) *lCard {
	name := strings.TrimPrefix(key, nameCardKey)
	c := &lCard{Name: key, Token: false}
	for _, lc := range cards {
		for _, o := range lc.overrides {
			if o.Name == name {
				c.Types, c.Subtypes, c.Colors = o.SetTypes, o.SetSubtypes, o.SetColors
				c.P, c.T = o.SetP, o.SetT
				return c
			}
		}
	}
	return c
}
