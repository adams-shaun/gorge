package kgen

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// OPS.md: the interpreter contract for everything dirac/kgen appended to kop,
// with the pool cards that use each item (from the generated table, so it
// never drifts) ordered by the number of decks that need it.

type opsDoc struct {
	label string // usage key (see opsUsage)
	title string
	text  string
}

// opsEntries is the semantic half of OPS.md, in document order: the core
// families (one opsdoc_<family>.go each), then each opsdoc_<mechanic>.go's
// init() append in file-name order. "Gorge" names the rule or gorge
// behaviour each item mirrors. This list is frozen: a new item goes in its
// own opsdoc_<mechanic>.go as an init() append, never here.
var opsEntries = slices.Concat(
	opsControlFlow,
	opsTrigEvents,
	opsEffectOps,
	opsDamage,
	opsZones,
	opsRefs,
	opsValues,
	opsConditions,
	opsStaticRepl,
	opsPreds,
	opsCosts,
	opsExprs,
	opsExtSubOps,
	opsMeld,
	opsExtMechanics,
	opsKwActions,
	opsFilterFields,
	opsAbilityFields,
	opsStaticFields,
	opsCardFields,
)

// opsUsage records, per doc label, the pool cards using it.
func opsUsage(pt *poolTables, all []*lCard, bs []built) map[string]map[string]bool {
	use := map[string]map[string]bool{}
	add := func(l, card string) {
		if use[l] == nil {
			use[l] = map[string]bool{}
		}
		use[l][card] = true
	}
	kOp := map[kop.Op]string{}
	for _, d := range opsEntries {
		if strings.HasPrefix(d.label, "op:") {
			for o := kop.Op(0); o < kop.OpHighEnd; o++ {
				if o.Known() && "op:"+o.String() == d.label {
					kOp[o] = d.label
				}
			}
		}
	}
	have := map[string]bool{}
	for _, d := range opsEntries {
		have[d.label] = true
	}
	exLabels := docLabels[kop.ExprKind](have, "kop.Ex", "ex")
	condLabels := docLabels[kop.CondKind](have, "kop.Cond", "cond")
	refLabels := docLabels[kop.Ref](have, "kop.Ref", "ref")
	predLabels := docLabels[kop.Pred](have, "kop.Pred", "pred")
	statLabels := docLabels[kop.StaticKind](have, "kop.Stat", "stat")
	replLabels := docLabels[kop.ReplKind](have, "kop.Repl", "repl")
	trigLabels := docLabels[kop.TrigEvent](have, "kop.Trig", "trig")
	f2Names := []struct {
		b kop.FilterFlags2
		l string
	}{
		{kop.F2Kicked, "Kicked"},
		{kop.F2CastByYou, "CastByYou"},
		{kop.F2EnteredThisTurn, "EnteredThisTurn"},
		{kop.F2ChosenType, "ChosenType"},
		{kop.F2ChosenColor, "ChosenColor"},
		{kop.F2Remembered | kop.F2NotRemembered, "Remembered"},
		{kop.F2ChosenCard | kop.F2NotChosenCard, "ChosenCard"},
		{kop.F2AttachedToT0, "AttachedToT0"},
		{kop.F2TargetedBy, "TargetedBy"},
		{kop.F2SameNameAsSource, "SameNameAsSource"},
		{kop.F2WithFlying | kop.F2WithoutFlying, "WithFlying"},
		{kop.F2Emblem, "Emblem"},
		{kop.F2Basic, "Basic"},
		{kop.F2Legendary, "Legendary"},
		{kop.F2SharesColorRem, "SharesColorRem"},
		{kop.F2NotTriggerObj, "NotTriggerObj"},
		{kop.F2WasDamaged, "WasDamaged"},
		{kop.F2Saddled, "Saddled"},
		{kop.F2Sneaked, "Sneaked"},
		{kop.F2Unblocked, "Unblocked"},
		{kop.F2TgtPlayerCtrl, "TgtPlayerCtrl"},
		{kop.F2DefendingCtrl, "DefendingCtrl"},
		{kop.F2CrewedSrc, "CrewedSrc"},
	}
	kwNames := []struct {
		b kop.KwMask
		l string
	}{
		{kop.KwHexproofFromInstants, "HexproofFromInstants"},
		{kop.KwProtBlack, "ProtBlack"},
		{kop.KwProtRed, "ProtRed"},
		{kop.KwPreventCombatDamageTo, "PreventCombatDamageTo"},
		{kop.KwFlashbackOwnCost, "FlashbackOwnCost"},
		{kop.KwPreventCombatDamageFrom, "PreventCombatDamageFrom"},
		{kop.KwUnblockable, "Unblockable"},
		{kop.KwCantBlock, "CantBlock"},
	}
	for ci, c := range all {
		if len(c.Refusals) > 0 {
			continue
		}
		name := c.Name
		d := &bs[ci].def
		seenF := map[kop.FilterID]bool{}
		var filt func(id kop.FilterID)
		var val func(v kop.ValueRef)
		kw := func(m kop.KwMask) {
			for _, k := range kwNames {
				if m&k.b != 0 {
					add("kw:"+k.l, name)
				}
			}
		}
		filt = func(id kop.FilterID) {
			for id != 0 && !seenF[id] {
				seenF[id] = true
				f := pt.filters[id]
				if f.Owner != 0 {
					add("filter:Owner", name)
				}
				if f.Sub != 0 || f.NotSub != 0 {
					add("filter:Sub", name)
				}
				if f.Or != 0 {
					add("filter:Or", name)
				}
				if f.PredV != (kop.ValueRef{}) {
					add("filter:PredV", name)
				}
				if l, ok := predLabels[f.Pred]; ok {
					add(l, name)
				}
				for _, x := range f2Names {
					if f.Flags2&x.b != 0 {
						add("f2:"+x.l, name)
					}
				}
				val(f.PredV)
				id = f.Or
			}
		}
		val = func(v kop.ValueRef) {
			switch v.K {
			case kop.VCountFilter:
				filt(kop.FilterID(v.N))
			case kop.VExpr:
				add("v:Expr", name)
				e := pt.exprs[v.N]
				if l, ok := exLabels[e.Kind]; ok {
					add(l, name)
				}
				if l, ok := refLabels[e.Ref]; ok {
					add(l, name)
				}
				if n := ctrDocKey(e.Ctr); n != "" {
					add(n, name)
				}
				filt(e.F)
				val(e.Sub[0])
				val(e.Sub[1])
			}
		}
		var cond func(id kop.CondID)
		cond = func(id kop.CondID) {
			if id == 0 {
				return
			}
			k := pt.conds[id]
			if l, ok := condLabels[k.Kind]; ok {
				add(l, name)
			}
			if l, ok := refLabels[k.Ref]; ok {
				add(l, name)
			}
			if k.Kind == kop.CondAnd {
				cond(k.Sub[0])
				cond(k.Sub[1])
			}
			filt(k.F)
			val(k.A)
			val(k.B)
		}
		var abil func(ab *kop.Ability)
		abil = func(ab *kop.Ability) {
			if ab.Cost.Flags&kop.CostTapHost != 0 {
				add("cost:TapHost", name)
			}
			if ab.Cost.Flags&kop.CostTapPower != 0 {
				add("cost:TapPower", name)
			}
			if ab.Cost.Flags&kop.CostPayEnergy != 0 {
				add("cost:PayEnergy", name)
			}
			if ab.Cost.Flags&kop.CostReturnSelf != 0 {
				add("cost:ReturnSelf", name)
			}
			if ab.Cost.Flags&kop.CostReturn != 0 {
				add("cost:Return", name)
			}
			if ab.Cost.Flags&kop.CostExileSelf != 0 {
				add("cost:ExileSelf", name)
			}
			usagePutLib(ab.Cost, name, add)
			if ab.Cost.ExileHand {
				add("cost:ExileHand", name)
			}
			if ab.Cost.ExileBf {
				add("cost:ExileBf", name)
			}
			if ab.Cost.Host == kop.CostHostExile {
				add("cost:HostExile", name)
			}
			usageDiscardCosts(ab.Cost, name, add)
			if ab.Cost.Flags&kop.CostOncePerTurn != 0 {
				add("cost:OncePerTurn", name)
			}
			if ab.Cost.Flags&kop.CostSubAmong != 0 {
				add("cost:SubAmong", name)
			}
			if ab.Kind == kop.AbTriggered && ab.Trig.Event == kop.TrigAttacks && ab.Trig.N == kop.TrigNTraining {
				add("trig:Training", name)
			}
			if ab.Kind == kop.AbTriggered && ab.Trig.Event == kop.TrigTaps && ab.Trig.N == kop.TrigNTeamworkTap {
				add("trig:TeamworkTap", name)
			}
			if ab.Kind == kop.AbTriggered && ab.Trig.Flags3&kop.Tf3GameOnce != 0 {
				add("trig:GameOnce", name)
			}
			if ab.Kind == kop.AbTriggered && ab.Trig.Flags3&kop.Tf3FirstAttack != 0 {
				add("trig:FirstAttack", name)
			}
			if ab.Kind == kop.AbTriggered && ab.Trig.Flags4 != 0 {
				add("trigflag4", name)
			}
			if ab.Kind == kop.AbActivated && ab.Trig.Cond != 0 {
				add("ab:ActCond", name)
			}
			if n := ctrDocKey(ab.Cost.Ctr); n != "" {
				add(n, name)
			}
			filt(ab.Cost.Filter)
			for i := 0; i < int(ab.NTargets); i++ {
				t := ab.Targets[i]
				filt(t.Filter)
				if t.Min != 0 || t.Max != 0 {
					add("tgt:MinMax", name)
				}
				if t.TFlags&kop.TgfKickedRange != 0 {
					add("tgt:Kicked", name)
				}
				if t.TFlags&kop.TgfGiftRange != 0 {
					add("tgt:Gift", name)
				}
				if t.TFlags&kop.TgfDistinct != 0 {
					add("tgf:Distinct", name)
				}
				if t.TFlags&kop.TgfSameController != 0 {
					add("tgf:SameController", name)
				}
				if t.TFlags&kop.TgfDiffController != 0 {
					add("tgf:DiffController", name)
				}
				if t.TFlags&kop.TgfDecl != 0 {
					add("tgf:Decl", name)
				}
				if t.TFlags&kop.TgfDeclSub != 0 {
					add("tgf:DeclSub", name)
				}
				if t.TFlags&kop.TgfSingleTgt != 0 {
					add("tgf:SingleTgt", name)
				}
				if t.TFlags&kop.TgfActOnly != 0 {
					add("tgf:ActOnly", name)
				}
				if t.Divide != 0 {
					add("tgt:Divide", name)
				}
				switch t.Kind {
				case kop.TgtStackObjTrig:
					add("tgt:StackObjTrig", name)
				case kop.TgtSpellTgting:
					add("tgt:SpellTgting", name)
				case kop.TgtStackTgting:
					add("tgt:StackTgting", name)
				}
			}
			if ab.NModes > 0 {
				add("op:Charm", name)
			}
			if ab.AFlags&kop.AfLatent != 0 {
				add("af:Latent", name)
			}
			if ab.AFlags&kop.AfManaDynamic != 0 {
				add("af:ManaDynamic", name)
			}
			if ab.AFlags&kop.AfManaChosen != 0 {
				add("af:ManaChosen", name)
			}
			if ab.AFlags&kop.AfManaChoice != 0 {
				add("af:ManaChoice", name)
			}
			if ab.AFlags&kop.AfResolveCount != 0 {
				add("af:ResolveCount", name)
			}
			if ab.AFlags&kop.AfCheckOnTrigger != 0 {
				add("af:CheckOnTrigger", name)
			}
			if ab.AFlags&kop.AfNinjutsu != 0 {
				add("af:Ninjutsu", name)
			}
			if ab.AFlags&kop.AfStation != 0 {
				add("af:Station", name)
			}
			if ab.AFlags&kop.AfGift != 0 {
				add("af:Gift", name)
			}
			if ab.Gate.Ctr != 0 {
				add("ab:Gate", name)
			}
			if l, ok := trigLabels[ab.Trig.Event]; ok && ab.Kind == kop.AbTriggered {
				add(l, name)
			}
			if ab.ManaRestrict != 0 {
				add("ab:ManaRestrict", name)
				filt(ab.ManaRestrict)
			}
			filt(ab.ManaRestrictAb)
			if ab.ActZone != 0 {
				add("ab:ActZone", name)
			}
			if ab.Exhaust {
				add("ab:Exhaust", name)
			}
			if ab.CantCopy {
				add("ab:CantCopy", name)
			}
			if ab.Evidence != 0 {
				add("ab:Evidence", name)
			}
			val(ab.ManaV)
			tr := ab.Trig
			filt(tr.Filter)
			if tr.Filter2 != 0 {
				add("trig:Filter2", name)
				filt(tr.Filter2)
			}
			if tr.Cond != 0 {
				add("trig:Cond", name)
				cond(tr.Cond)
			}
			if tr.Zone != 0 {
				add("trig:Zone", name)
			}
			if tr.N != 0 {
				add("trig:N", name)
			}
			for _, x := range []struct {
				b kop.TrigFlags2
				l string
			}{
				{kop.Tf2FirstTime, "FirstTime"},
				{kop.Tf2YourTurn, "YourTurn"},
				{kop.Tf2OppTurn, "OppTurn"},
				{kop.Tf2NonCombat, "NonCombat"},
				{kop.Tf2ToOpponent, "ToOpponent"},
				{kop.Tf2SelfOrFilter, "SelfOrFilter"},
				{kop.Tf2CastSnapshot, "CastSnapshot"},
				{kop.Tf2CastSpent, "CastSpent"},
				{kop.Tf2NthSpell, "NthSpell"},
				{kop.Tf2BlockedBySelf, "BlockedBySelf"},
				{kop.Tf2TargetsFilter2, "TargetsFilter2"},
				{kop.Tf2Alone, "Alone"},
				{kop.Tf2AtkYou, "AtkYou"},
				{kop.Tf2AtkYourPW, "AtkYourPW"},
				{kop.Tf2AtkOpp, "AtkOpp"},
				{kop.Tf2Batch, "Batch"},
				{kop.Tf2PerPair, "PerPair"},
				{kop.Tf2Cycled, "Cycled"},
				{kop.Tf2CastFromHand, "CastFromHand"},
				{kop.Tf2Convoked, "Convoked"},
				{kop.Tf2Increment, "Increment"},
			} {
				if tr.Flags2&x.b != 0 {
					add("tf2:"+x.l, name)
				}
			}
			st := ab.Static
			if ab.Kind == kop.AbStatic {
				if l, ok := statLabels[st.Kind]; ok {
					add(l, name)
				}
				if st.Cond2 != 0 {
					add("st:Cond2", name)
					cond(st.Cond2)
				}
				if st.StFlags&kop.StDynamic != 0 {
					add("st:Dynamic", name)
				}
				if st.StFlags&kop.StEmbNum != 0 {
					add("st:EmbNum", name)
				}
				if st.StFlags&kop.StTargetsFilter2 != 0 {
					if st.Kind == kop.StatMayPlayFrom {
						add("st:MayZeroAlt", name) // StMayZeroAlt aliases StTargetsFilter2
					} else {
						add("st:TargetsFilter2", name)
					}
				}
				if st.StFlags&kop.StBargained != 0 {
					add("st:Bargained", name)
				}
				if st.StFlags&kop.StReduceColor != 0 {
					if st.Kind == kop.StatMayPlayFrom {
						add("st:MayPayLife", name) // StMayPayLife aliases StReduceColor
					} else {
						add("st:ReduceColor", name)
					}
				}
				if st.StFlags&kop.StYouHexproof != 0 {
					add("st:YouHexproof", name)
				}
				if st.StFlags&kop.StPanhETB != 0 {
					switch {
					case st.Kind == kop.StatIncreaseCost:
						add("st:RaiseLife", name) // StRaiseLife aliases StPanhETB
					case st.Kind == kop.StatMayPlayFrom:
						add("st:MayPayEvidence", name) // StMayPayEvidence aliases StPanhETB
					default:
						add("st:PanhETB", name)
					}
				}
				if st.Kind == kop.StatIncreaseCost && st.StFlags&kop.StRaisePerTarget != 0 {
					add("st:RaisePerTarget", name) // aliases StOncePerTurn
				}
				if st.Kind == kop.StatMayPlayFrom && st.StFlags&kop.StMayPayEnergy != 0 {
					add("st:MayPayEnergy", name) // StMayPayEnergy aliases StRaiseOpp
				} else if st.StFlags&(kop.StRaiseYou|kop.StRaiseOpp) != 0 {
					add("st:RaiseYou", name)
				}
				grantUsage(&st, name, add)
				if st.ProtChosen {
					add("st:ProtChosen", name)
				}
				for _, l := range staticFilterLabels(&st) {
					add(l, name)
				}
				if st.Evasion != 0 {
					add("st:Evasion", name)
				}
				if st.Infect {
					add("st:Infect", name)
				}
				if st.Zone != 0 {
					add("st:Zone", name)
				}
				kw(st.Kw)
				filt(st.Filter)
				filt(st.Filter2)
				filt(st.ProtFrom)
				filt(st.ProtExc)
				filt(st.Landwalk)
				filt(st.GainsOf)
				val(st.VA)
				val(st.VB)
			}
			if ab.Kind == kop.AbReplacement {
				if l, ok := replLabels[ab.Repl.Kind]; ok {
					add(l, name)
				}
				if ab.Repl.Cond != 0 || ab.Repl.Filter != 0 {
					add("repl:Cond", name)
				}
				cond(ab.Repl.Cond)
				filt(ab.Repl.Filter)
			}
			for _, in := range ab.Prog {
				if l, ok := kOp[in.Op]; ok {
					add(l, name)
				}
				if in.Op == kop.OpExt {
					add("ext:"+in.Ext().String(), name) // kop/ext.go sub-ops
				}
				if l, ok := refLabels[in.Obj]; ok {
					add(l, name)
				}
				filt(in.F)
				val(in.A)
				val(in.B)
				switch in.Op {
				case kop.OpSkipUnless:
					cond(kop.CondID(in.C))
				case kop.OpGrantKeyword:
					kw(kop.KwMask(uint16(in.C)) | kop.KwMask(uint16(in.B.N))<<16)
				case kop.OpPutCounter, kop.OpRemoveCounter, kop.OpRemoveCounterRem, kop.OpEnterWithCounters, kop.OpDoubleCounters:
					if n := ctrDocKey(kop.CounterKind(in.C)); in.C != 255 && n != "" {
						add(n, name)
					}
				case kop.OpCounter:
					if in.B.N&kop.CtrRemMV != 0 {
						add("op:CounterRemMV", name)
					}
				case kop.OpOverride:
					kw(pt.overrides[in.C].AddKw)
				case kop.OpCreateEmblem:
					for i := range pt.emblems[in.C].Abilities {
						abil(&pt.emblems[in.C].Abilities[i])
					}
				}
			}
		}
		kw(d.Keywords)
		filt(d.Enchant.Filter)
		filt(d.BestowTgt.Filter)
		filt(d.BeholdF)
		filt(d.WardSac)
		if d.WardLife != 0 {
			add("card:WardLife", name)
		}
		if d.WardEvidence != 0 {
			add("card:WardEvidence", name)
		}
		if d.BeholdN != 0 {
			add("cost:Behold", name)
		}
		if d.WardDiscard != 0 {
			add("card:WardDiscard", name)
		}
		if d.WardSac != 0 {
			add("card:WardSac", name)
		}
		if d.WardWaterbend != 0 {
			add("card:WardWaterbend", name)
		}
		if d.WardPoison != 0 {
			add("card:WardPoison", name)
		}
		if d.WardDiscOrN != 0 {
			add("card:WardDiscOrN", name)
		}
		if d.WardLifePower {
			add("card:WardLifePower", name)
		}
		if d.Daybound || d.Nightbound {
			add("card:Daybound", name)
		}
		if d.Leyline {
			add("card:Leyline", name)
		}
		if d.LeylineGem {
			add("card:LeylineGem", name)
		}
		if d.World {
			add("card:World", name)
		}
		if d.SelfFlash {
			add("card:SelfFlash", name)
		}
		if d.Landwalk != 0 {
			add("card:Landwalk", name)
		}
		filt(d.Landwalk)
		if d.HexColors != 0 {
			add("card:HexColors", name)
		}
		if d.ProtColors != 0 {
			add("card:ProtColors", name)
		}
		if d.Evasion != 0 {
			add("card:Evasion", name)
		}
		if d.MinBlockers != 0 {
			add("card:MinBlockers", name)
		}
		if d.ExtraBlocks != 0 {
			add("card:ExtraBlocks", name)
		}
		if d.HasMadness {
			add("card:Madness", name)
		}
		if d.HasMiracle {
			add("card:Miracle", name)
		}
		keywordFlagUsage(d, name, add) // opsdoc_umbraarmor.go
		if d.AloneAtk {
			add("card:AloneAtk", name)
		}
		if d.AloneBlk {
			add("card:AloneBlk", name)
		}
		if d.Engines {
			add("card:Engines", name)
		}
		opsAddYes(add, d.Ascend, "card:Ascend", name)
		opsAddKwFlags(add, d, name)
		if d.TypedMana {
			add("card:TypedMana", name)
		}
		if d.Infect {
			add("card:Infect", name)
		}
		if d.Toxic != 0 {
			add("card:Toxic", name)
		}
		if d.Cascade != 0 {
			add("card:Cascade", name)
		}
		if d.ProtFrom != 0 {
			add("card:ProtFrom", name)
		}
		filt(d.ProtFrom)
		if d.SpellCost != (kop.Cost{}) {
			add("card:SpellCost", name)
		}
		if d.HasAltAdd {
			add("card:AltAddCost", name)
		}
		if d.RemPersist {
			add("card:RemPersist", name)
		}
		if d.ExiledWith {
			add("card:ExiledWith", name)
		}
		if d.CombatHitRec {
			add("card:CombatHitRec", name)
		}
		if d.SpendRead {
			add("card:SpendRead", name)
		}
		if d.BeamMeUp {
			add("card:BeamMeUp", name)
		}
		if d.FbCost.Flags != 0 {
			add("card:FbCost", name)
		}
		if d.Gift {
			add("card:Gift", name)
		}
		if d.Offspring {
			add("card:Offspring", name)
		}
		if d.Buyback {
			add("card:Buyback", name)
		}
		if d.Entwine {
			add("card:Entwine", name)
		}
		if d.Convoke {
			add("card:Convoke", name)
		}
		if d.Improvise {
			add("card:Improvise", name)
		}
		if d.Delve {
			add("card:Delve", name)
		}
		if d.ReboundAb != 0 {
			add("card:Rebound", name)
		}
		if d.Casualty != 0 {
			add("card:Casualty", name)
		}
		if d.Bargain {
			add("card:Bargain", name)
		}
		if d.Paradigm != 0 {
			add("card:Paradigm", name)
		}
		if d.OptCost.Flags != 0 || d.OptBehold || d.OptEvidence != 0 {
			add("card:OptCost", name)
		}
		for i := range d.Abilities {
			abil(&d.Abilities[i])
		}
	}
	return use
}

// writeOpsDoc renders OPS.md.
func writeOpsDoc(p *pool, pt *poolTables, all []*lCard, bs []built) []byte {
	use := opsUsage(pt, all, bs)
	addCardFlagUsage(use, all, bs) // opsdoc_cardflags.go
	deckUse := func(cs map[string]bool) int {
		n := 0
		for _, d := range p.decks {
			for _, c := range d {
				if cs[c] {
					n++
					break
				}
			}
		}
		return n
	}
	type row struct {
		e     opsDoc
		decks int
		cards []string
	}
	var rows []row
	for _, e := range opsEntries {
		cs := use[e.label]
		var names []string
		for c := range cs {
			names = append(names, c)
		}
		sort.Slice(names, func(i, j int) bool {
			a, b := p.nDecks[names[i]], p.nDecks[names[j]]
			if a != b {
				return a > b
			}
			return names[i] < names[j]
		})
		rows = append(rows, row{e, deckUse(cs), names})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].decks > rows[j].decks })
	var b bytes.Buffer
	b.WriteString("# kop additions: interpreter contract\n\n")
	b.WriteString("The dirac interpreter's contract for the generated card tables.\n\n")
	b.WriteString("Generated by `go run ./dirac/cmd/kgen` (the usage columns come from this table; the semantics from\n")
	b.WriteString("dirac/kgen/opsdoc.go); do not edit. Every op, enum value and field dirac/kgen appended to\n")
	b.WriteString("dirac/kop, with the exact interpreter semantics, the decisions it implies and the cards that\n")
	b.WriteString("use it, ordered by the number of decks that need it (of " + fmt.Sprint(len(p.decks)) + "). Packed operand constants\n")
	b.WriteString("(PayKind, DamageFlags, MoveFlags, SearchFlags, DigFlags, DigUntilFlags, CopyFlags, TokenFlags,\n")
	b.WriteString("Duration) are in dirac/kop/table.go. \"Gorge\" names the gorge parameter or rule mirrored;\n")
	b.WriteString("where this says \"mirror gorge\" the oracle is the arbiter. An item with 0 decks is defined for a\n")
	b.WriteString("refused or token-only use and may be implemented last.\n\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", r.e.title, r.e.text)
		if len(r.cards) == 0 {
			b.WriteString("Used by: no supported pool card.\n\n")
			continue
		}
		var cs []string
		for _, c := range r.cards {
			cs = append(cs, fmt.Sprintf("%s (%d)", c, p.nDecks[c]))
		}
		fmt.Fprintf(&b, "Decks: %d. Cards: %s.\n\n", r.decks, strings.Join(cs, ", "))
	}
	return b.Bytes()
}

// ctrDocKey is the ops-doc key of counter kind c: "ctr:New" for the
// appended featurized kinds, "ctr:Unfeat" for the kinds above CtrLevel.
func ctrDocKey(c kop.CounterKind) string {
	switch {
	case c == kop.CtrShield:
		return "ctr:Shield"
	case c >= kop.CtrCharge:
		return "ctr:Unfeat"
	case c >= kop.NumCounterKinds:
		return "ctr:New"
	}
	return ""
}

// usagePutLib records the PutCardToLibFromGrave self cost (a Cost byte, not a
// CostFlags bit).
func usagePutLib(c kop.Cost, name string, add func(label, card string)) {
	if c.PutLibSelf {
		add("cost:PutLibSelf", name)
	}
	if c.ExileGSelf {
		add("cost:ExileGSelf", name)
	}
	if c.Discard2N > 0 {
		add("cost:DiscardTwo", name)
	}
}

// docLabels maps each value of the kop enum T to its OPS.md label tag:X, where
// kgen names the value prefix+X (names.go, names_*.go) and opsEntries holds the
// label. A new kind is recorded from its opsdoc entry and its kgen name alone,
// with no edit here. Labels in opsUnwired are skipped.
func docLabels[T ~uint8 | ~uint16](have map[string]bool, prefix, tag string) map[T]string {
	var zero T
	out := map[T]string{}
	for _, e := range enumNames[reflect.TypeOf(zero)] {
		x, ok := strings.CutPrefix(e.name, prefix)
		if l := tag + ":" + x; ok && have[l] && !opsUnwired[l] {
			out[T(e.v)] = l
		}
	}
	return out
}

// opsUnwired are OPS.md labels whose usage opsUsage has never recorded, so their
// rows say "no supported pool card" whatever the pool holds. They were missing
// from the hand-kept maps and switches docLabels replaced, and are kept out so the
// rendered OPS.md stayed byte-identical through that change. The list only
// shrinks: deleting a line starts recording that label (and changes OPS.md only,
// never a table).
var opsUnwired = map[string]bool{
	"ex:CastBattlefield":    true,
	"ex:CommittedCrime":     true,
	"ex:DilemmasFaced":      true,
	"ex:DiscardedThisTurn":  true,
	"ex:FirstCast":          true,
	"ex:OppsAttacked":       true,
	"ex:Pow":                true,
	"ex:RemEachOwnNonLand":  true,
	"ex:SacrificedThisTurn": true,
	"ex:ScriedThisTurn":     true,
	"ex:YourSpeed":          true,
	"repl:DrawMore":         true,
	"repl:RewriteTokens":    true,
	"stat:CantActivate":     true,
	"stat:DisableETB":       true,
	"stat:MaxBlockers":      true,
	"stat:ReduceAbility":    true,
	"stat:TapExtra":         true,
	"trig:CollectEvidence":  true,
	"trig:DiscardedAll":     true,
	"trig:Discover":         true,
	"trig:Exploited":        true,
	"trig:Explores":         true,
	"trig:Forage":           true,
	"trig:FullyUnlock":      true,
	"trig:LandPlayed":       true,
	"trig:ManaExpend":       true,
	"trig:Scry":             true,
	"trig:SearchedLibrary":  true,
	"trig:Surveil":          true,
}
