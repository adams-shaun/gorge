package codeshape

import "strings"

// The shape census: the tree-wide count metrics cmd/countgrowth holds to
// the base-relative count growth rule (countgrowth.go) against the merge
// base. There are no stored ceilings: each metric's value at the base is
// recomputed there. The names are the old ratchet-test constants' names, so
// tooling and reviews read the same identifiers; Measure with
// `go run ./cmd/codeshape -table`.
//
// Each row's Advice is what to do instead of growing the metric; a leak
// census's advice carries the leak list from the head measurement.

// CountMetric is one tree-wide count metric: its name, how to read it from a
// Measure of the tree, and its advice (rendered from the head measurement,
// so a leak census can list its leaks).
type CountMetric struct {
	Name   string
	Get    func(Metrics) int
	Advice func(Metrics) string
}

// staticAdvice returns an advice func for advice that does not depend on
// the measurement.
func staticAdvice(s string) func(Metrics) string {
	return func(Metrics) string { return s }
}

// leakAdvice returns an advice func for a leak census: what to read
// instead, then the leaks the head measurement found.
func leakAdvice(what string, leaks func(Metrics) []string) func(Metrics) string {
	return func(m Metrics) string { return what + " Leaks: " + strings.Join(leaks(m), ", ") }
}

// CountMetrics returns the census, in the old ratchet table's order.
func CountMetrics() []CountMetric {
	return []CountMetric{
		{"maxFuncLinesOver300", func(m Metrics) int { return m.FuncsOver300 },
			staticAdvice("A function over 300 lines is a concern without a seam: extract the concern " +
				"behind a named helper or type rather than growing the function. " +
				"`go run ./cmd/codeshape -table` lists them longest first.")},
		{"engineMethodCount", func(m Metrics) int { return m.EngineMethods },
			staticAdvice("Every *Engine method can call every other; put new logic on a narrower " +
				"receiver (a feature struct holding what it needs) or a free function, or " +
				"remove an Engine method in the same change.")},
		{"hostMethodCount", func(m Metrics) int { return m.HostMethods },
			staticAdvice("effects.Host is the union of narrow role interfaces; derive what you " +
				"need from an existing method (Chars carries every characteristic), or replace one, instead of adding another.")},
		{"hostDirectMethodCount", func(m Metrics) int { return m.HostDirectMethods },
			staticAdvice("effects.Host is the union of the role interfaces in engine/effects/host_roles.go; " +
				"add a new method to the one role it belongs to, not to Host itself.")},
		{"hostRoleMaxMethods", func(m Metrics) int { return m.HostRoleMaxMethods },
			staticAdvice("A Host role is to stay narrow (<= 20 methods); split a role that grows by " +
				"concern rather than widening it, or replace an existing method.")},
		{"hostOptionalAssertions", func(m Metrics) int { return m.HostOptionalAssertions },
			staticAdvice("An optional-interface assertion on Host is a Host method the role split " +
				"cannot see. Put the method on the role it belongs to (a test double then " +
				"implements it), or extend an existing optional interface, instead of " +
				"asserting a new one.")},
		{"ctxFieldCount", func(m Metrics) int { return m.CtxFields },
			staticAdvice("effects.Ctx fields are mostly per-primitive ask/resume cursors; keep a " +
				"primitive's cursor in its own resume record instead of widening Ctx.")},
		{"ctxEmbedCount", func(m Metrics) int { return m.CtxEmbeds },
			staticAdvice("An embed promotes every field of the embedded type into effects.Ctx; add a " +
				"named field instead (and count it against ctxFieldCount).")},
		{"resumePointFieldCount", func(m Metrics) int { return m.ResumePointFields },
			staticAdvice("resumePoint is the hand-defunctionalised continuation; carry a " +
				"new suspension's state in the primitive's own resume record.")},
		{"stringParamReads", func(m Metrics) int { return m.StringParamReads },
			staticAdvice("Read a parameter through a typed accessor (cards.ParamKey vocabulary) " +
				"instead of a new Params[\"Literal\"] index.")},
		{"stringCaseLiterals", func(m Metrics) int { return m.StringCaseLiterals },
			staticAdvice("Dispatch on a compiled enum, a bitmask or a registry entry rather than a " +
				"new `case \"Literal\":` arm.")},
		{"ctxLiterals", func(m Metrics) int { return m.CtxLiterals },
			staticAdvice("Build an effects.Ctx with effects.NewCtx / NewCtxPtr (seeded by a CtxInit, " +
				"other fields assigned on the result), derive one from a resolving context " +
				"with (*Ctx).Child or (*Ctx).ForTrigger, or add a constructor to " +
				"engine/effects/ctx_new.go (engine/rules/ctx_new.go for one that binds engine-owned " +
				"tables). A hand-written literal that copies another context's fields drops " +
				"the next field added to Ctx.")},
		{"specContextLiterals", func(m Metrics) int { return m.SpecContextLiterals },
			staticAdvice("Build a SpecContext with effects.NewSpecContext, derive it from a resolving " +
				"Ctx with (*Ctx).SpecContext / (*Ctx).TableSpecContext, or bind rules' layer " +
				"tables through Engine.withNames / specCtxSVars.")},
		{"trigmatchBoardMethods", func(m Metrics) int { return m.TrigmatchBoardMethods },
			staticAdvice("trigmatch.Board is the trigger matchers' whole view of the engine; derive a " +
				"new fact from an existing method (Chars carries every characteristic, Facts " +
				"every per-emit trigger context value) or pass it precomputed, instead of " +
				"adding a method.")},
		{"payEngineMethods", func(m Metrics) int { return m.PayEngineMethods },
			staticAdvice("pay.Engine is the payment layer's whole view of the engine (target under " +
				"20); derive a new fact from an existing method or pass it precomputed, " +
				"instead of adding a method.")},
		{"changeZoneParamLeaks", func(m Metrics) int { return m.ChangeZoneParamLeaks },
			leakAdvice("Read the parameter through effects.ChangeZoneOf's compiled ChangeZoneParams "+
				"(add a field to compileChangeZone in engine/effects/changezone_params.go) instead "+
				"of reading the ability's Params in a ChangeZone file or a ChangeZone-only "+
				"key elsewhere.", func(m Metrics) []string { return m.ChangeZoneLeaks })},
		{"changeZoneAllParamLeaks", func(m Metrics) int { return m.ChangeZoneAllParamLeaks },
			leakAdvice("Read the parameter through effects.ChangeZoneAllOf's compiled ChangeZoneAllParams "+
				"(add a field to compileChangeZoneAll in engine/effects/changezoneall_params.go) instead "+
				"of reading the ability's Params in engine/effects/zone_changeall.go or a "+
				"ChangeZoneAll-only key elsewhere.", func(m Metrics) []string { return m.ChangeZoneAllLeaks })},
		{"attachParamLeaks", func(m Metrics) int { return m.AttachParamLeaks },
			leakAdvice("Read the parameter through effects.AttachOf's compiled AttachParams (add a "+
				"field to compileAttach in engine/effects/attach_params.go) instead of reading the "+
				"ability's Params in engine/effects/attach.go or an Attach-only key elsewhere.",
				func(m Metrics) []string { return m.AttachLeaks })},
		{"charmParamLeaks", func(m Metrics) int { return m.CharmParamLeaks },
			leakAdvice("Read the parameter through effects.CharmOf's compiled CharmParams (add a "+
				"field to compileCharm in engine/effects/charm_params.go) instead of reading the "+
				"ability's Params in engine/effects/charm.go or a Charm-only key elsewhere.",
				func(m Metrics) []string { return m.CharmLeaks })},
		{"pumpParamLeaks", func(m Metrics) int { return m.PumpParamLeaks },
			leakAdvice("Read the parameter through effects.PumpOf's compiled PumpParams (add a "+
				"field to compilePump in engine/effects/pump_params.go) instead of reading the "+
				"ability's Params in engine/effects/pump.go or a Pump-only key elsewhere.",
				func(m Metrics) []string { return m.PumpLeaks })},
		{"drawParamLeaks", func(m Metrics) int { return m.DrawParamLeaks },
			leakAdvice("Read the parameter through effects.DrawOf's compiled DrawParams (add a "+
				"field to compileDraw in engine/effects/draw_params.go) instead of reading the "+
				"ability's Params in engine/effects/draw.go or a Draw-only key elsewhere.",
				func(m Metrics) []string { return m.DrawLeaks })},
		{"replaceEffectParamLeaks", func(m Metrics) int { return m.ReplaceEffectParamLeaks },
			leakAdvice("Read the parameter through effects.ReplaceEffectOf's compiled "+
				"ReplaceEffectParams (add a field to compileReplaceEffect in "+
				"engine/effects/replaceeffect_params.go) instead of reading the ability's Params "+
				"in engine/effects/replacement.go or a ReplaceEffect-only key elsewhere.",
				func(m Metrics) []string { return m.ReplaceEffectLeaks })},
		{"manaParamLeaks", func(m Metrics) int { return m.ManaParamLeaks },
			leakAdvice("Read the parameter through effects.ManaOf's compiled ManaParams (add a "+
				"field to compileMana in engine/effects/mana_params.go) instead of reading the "+
				"ability's Params in engine/effects/mana_effect.go or a Mana-only rider key "+
				"elsewhere.", func(m Metrics) []string { return m.ManaLeaks })},
		{"manaReflectedParamLeaks", func(m Metrics) int { return m.ManaReflectedParamLeaks },
			leakAdvice("Read the parameter through effects.ManaReflectedOf's compiled "+
				"ManaReflectedParams (add a field to compileManaReflected in "+
				"engine/effects/manareflected_params.go) instead of reading the ability's Params "+
				"in engine/effects/mana_reflected.go or a ManaReflected-only key elsewhere.",
				func(m Metrics) []string { return m.ManaReflectedLeaks })},
		{"dealDamageParamLeaks", func(m Metrics) int { return m.DealDamageParamLeaks },
			leakAdvice("Read the parameter through effects.DealDamageOf's compiled DealDamageParams (add "+
				"a field to compileDealDamage in engine/effects/dealdamage_params.go) instead of reading "+
				"the ability's Params in engine/effects/damage_deal.go or a DealDamage-only key elsewhere.",
				func(m Metrics) []string { return m.DealDamageLeaks })},
		{"putCounterParamLeaks", func(m Metrics) int { return m.PutCounterParamLeaks },
			leakAdvice("Read the parameter through effects.PutCounterOf's compiled PutCounterParams (add "+
				"a field to compilePutCounter in engine/effects/putcounter_params.go) instead of reading "+
				"the ability's Params in engine/effects/counters_put.go or a PutCounter-only key elsewhere.",
				func(m Metrics) []string { return m.PutCounterLeaks })},
		{"effectParamLeaks", func(m Metrics) int { return m.EffectParamLeaks },
			leakAdvice("Read the parameter through effects.EffectOf's compiled EffectParams (add a "+
				"field to compileEffect in engine/effects/effect_params.go) instead of reading the "+
				"ability's Params in engine/effects/effect.go or an Effect-only key elsewhere.",
				func(m Metrics) []string { return m.EffectLeaks })},
		{"targetParamLeaks", func(m Metrics) int { return m.TargetParamLeaks },
			leakAdvice("Read the targeting parameter through effects.TargetsOf's compiled TargetParams "+
				"(add a field to compileTargets in engine/effects/targets_params.go) instead of reading "+
				"a targeting key from the ability's Params.", func(m Metrics) []string { return m.TargetLeaks })},
		{"definedParamLeaks", func(m Metrics) int { return m.DefinedParamLeaks },
			leakAdvice("Read the selector through effects.DefinedOf's compiled Ref (add a field to "+
				"compileDefined in engine/effects/defined_params.go, or resolve selector text with "+
				"effects.RefOf/DefinedSpec) instead of reading a Defined key from the ability's "+
				"Params.", func(m Metrics) []string { return m.DefinedLeaks })},
		{"delayedTriggerParamLeaks", func(m Metrics) int { return m.DelayedTriggerParamLeaks },
			leakAdvice("Read the parameter through effects.DelayedTriggerOf's compiled DelayedTriggerParams (add a "+
				"field to compileDelayedTrigger in engine/effects/delayedtrigger_params.go) instead of reading the ability's Params in "+
				"engine/effects/delayed_trigger.go or a DelayedTrigger-only key elsewhere.",
				func(m Metrics) []string { return m.DelayedTriggerLeaks })},
		{"copyPermanentParamLeaks", func(m Metrics) int { return m.CopyPermanentParamLeaks },
			leakAdvice("Read the parameter through effects.CopyPermanentOf's compiled CopyPermanentParams (add a "+
				"field to compileCopyPermanent in engine/effects/copypermanent_params.go) instead of reading the ability's Params in "+
				"engine/effects/copypermanent.go or a CopyPermanent-only key elsewhere.",
				func(m Metrics) []string { return m.CopyPermanentLeaks })},
		{"cloneParamLeaks", func(m Metrics) int { return m.CloneParamLeaks },
			leakAdvice("Read the parameter through effects.CloneOf's compiled CloneParams (add a "+
				"field to compileClone in engine/effects/clone_params.go) instead of reading the ability's Params in "+
				"engine/effects/clone.go or a Clone-only key elsewhere.",
				func(m Metrics) []string { return m.CloneLeaks })},
		{"digParamLeaks", func(m Metrics) int { return m.DigParamLeaks },
			leakAdvice("Read the parameter through effects.DigOf's compiled DigParams (add a "+
				"field to compileDig in engine/effects/dig_params.go) instead of reading the ability's Params in "+
				"engine/effects/dig.go or a Dig-only key elsewhere.",
				func(m Metrics) []string { return m.DigLeaks })},
		{"digUntilParamLeaks", func(m Metrics) int { return m.DigUntilParamLeaks },
			leakAdvice("Read the parameter through effects.DigUntilOf's compiled DigUntilParams (add a "+
				"field to compileDigUntil in engine/effects/diguntil_params.go) instead of reading the ability's Params in "+
				"engine/effects/diguntil.go or a DigUntil-only key elsewhere.",
				func(m Metrics) []string { return m.DigUntilLeaks })},
		{"removeCounterParamLeaks", func(m Metrics) int { return m.RemoveCounterParamLeaks },
			leakAdvice("Read the parameter through effects.RemoveCounterOf's compiled RemoveCounterParams (add a "+
				"field to compileRemoveCounter in engine/effects/removecounter_params.go) instead of reading the ability's Params in "+
				"engine/effects/removecounter.go or a RemoveCounter-only key elsewhere.",
				func(m Metrics) []string { return m.RemoveCounterLeaks })},
		{"tokenParamLeaks", func(m Metrics) int { return m.TokenParamLeaks },
			leakAdvice("Read the parameter through effects.TokenOf's compiled TokenParams (add a "+
				"field to compileToken in engine/effects/token_params.go) instead of reading the ability's Params in "+
				"engine/effects/token.go or a Token-only key elsewhere.",
				func(m Metrics) []string { return m.TokenLeaks })},
		{"voteParamLeaks", func(m Metrics) int { return m.VoteParamLeaks },
			leakAdvice("Read the parameter through effects.VoteOf's compiled VoteParams (add a "+
				"field to compileVote in engine/effects/vote_params.go) instead of reading the ability's Params in "+
				"engine/effects/vote.go or engine/effects/vote_effect.go or a Vote-only key elsewhere.",
				func(m Metrics) []string { return m.VoteLeaks })},
		{"repeatEachParamLeaks", func(m Metrics) int { return m.RepeatEachParamLeaks },
			leakAdvice("Read the parameter through effects.RepeatEachOf's compiled RepeatEachParams (add a "+
				"field to compileRepeatEach in engine/effects/repeateach_params.go) instead of reading the ability's Params in "+
				"engine/effects/repeateach.go or a RepeatEach-only key elsewhere.",
				func(m Metrics) []string { return m.RepeatEachLeaks })},
		{"engineSurface", func(m Metrics) int { return m.EngineSurface },
			staticAdvice("A free func taking *Engine is an Engine method; take a narrow interface " +
				"(or a feature struct holding only what it needs) instead of the whole engine.")},
		{"stringLiteralCompares", func(m Metrics) int { return m.StringLiteralCompares },
			staticAdvice("Compare a code from the table that owns the word (a cards.StrCodes code, " +
				"an enum or a mask compiled once) instead of a new `s == \"Literal\"`.")},
		{"rawBoolParamParses", func(m Metrics) int { return m.RawBoolParamParses },
			staticAdvice("Parse the flag once in the API's compiled params (a bool field of its " +
				"*_params.go record) instead of strings.EqualFold over a ParamStr at each use.")},
		{"stringKeyedParamReads", func(m Metrics) int { return m.StringKeyedParamReads },
			func(m Metrics) string {
				return "Read through the compiled record / ParamKey (a ParamText field of the API's " +
					"compiler, or cards.PK<Key>) instead of passing a literal key to a string-keyed " +
					"helper. Helpers: " + strings.Join(m.StringKeyedParamHelpers, "; ")
			}},
		{"strCodesTables", func(m Metrics) int { return m.StrCodesTables },
			staticAdvice("Extend the table that already owns this word (or the mechanism's descriptor) " +
				"instead of naming it in another table.")},
		{"strCodesKeyDup", func(m Metrics) int { return m.StrCodesKeyDup },
			staticAdvice("Extend the table that already owns this word (or the mechanism's descriptor) " +
				"instead of naming it in another table.")},
		{"triggerContextLiterals", func(m Metrics) int { return m.TriggerContextLiterals },
			staticAdvice("Derive trigger referents from the firing event (rules' triggerReferents) or " +
				"copy an existing TriggerContext and set the fields that differ; a " +
				"synthesized trigger's record can be declared `var tc effects.TriggerContext` " +
				"and filled field by field.")},
	}
}
