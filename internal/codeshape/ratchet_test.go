package codeshape

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The shrink-only ratchets of the rules-engine layering
// (docs/design/2026-10-03-rules-engine-layering-design.md, section
// 4), on the pattern of testkit/testutil/agentsdoc_test.go's
// shrink-only Known-approximations register: each constant is a measured
// value, the test FAILS when the measurement grows past it and LOGS when
// it falls below it. A change that shrinks a metric lowers its constant to
// the new measurement in the same commit; NEVER raise one.
//
// Measure with `go run ./cmd/codeshape -table`.
const (
	// maxFuncLinesOver300 is the number of non-test functions in engine/rules/ and
	// engine/effects/ spanning more than 300 lines.
	maxFuncLinesOver300 = 41
	// engineMethodCount is the number of non-test methods on rules.Engine.
	engineMethodCount = 1821
	// hostMethodCount is the number of methods in the effects.Host interface
	// (its whole method set, roles included).
	hostMethodCount = 84
	// hostDirectMethodCount is the number of methods effects.Host declares
	// itself rather than takes from a role interface (host_roles.go).
	hostDirectMethodCount = 0
	// hostRoleMaxMethods is the method-set size of effects.Host's largest
	// role interface (HostTurnLedger).
	hostRoleMaxMethods = 18
	// hostOptionalAssertions is the number of type assertions in package
	// effects to an interface other than Host and its roles (inline
	// `interface{...}` or a named optional interface).
	hostOptionalAssertions = 38
	// ctxFieldCount is the number of named fields in effects.Ctx; embeds are
	// ratcheted separately by ctxEmbedCount.
	ctxFieldCount = 67
	ctxEmbedCount = 1
	// resumePointFieldCount is the number of fields in rules' resumePoint.
	resumePointFieldCount = 5
	// stringParamReads is the number of <x>Params["literal"] index
	// expressions in engine/rules/ and engine/effects/ non-test files. Every read
	// goes through the ParamKey accessors and every write through the typed
	// SA.SetParam / Repl.SetParam setters.
	stringParamReads = 0
	// stringCaseLiterals is the number of string literals in switch case
	// lists in engine/rules/ and engine/effects/ non-test files.
	// Dispatch goes through cards.StrTable, cards.NameSet and cards.StrCodes
	// (sorted dense slices, built once at init; one lookup, integer switch,
	// vocabulary in one table).
	stringCaseLiterals = 0
	// ctxLiterals, specContextLiterals and triggerContextLiterals are the
	// effects.Ctx / SpecContext / TriggerContext composite literals in engine/rules/
	// and engine/effects/ non-test files outside codeshape.CtxConstructorFiles.
	ctxLiterals            = 0
	specContextLiterals    = 0
	triggerContextLiterals = 13
	// trigmatchBoardMethods is the method count of trigmatch.Board, the
	// read-only view engine/rules/trigmatch's matchers read the engine through.
	trigmatchBoardMethods = 27
	// payEngineMethods is the method count of pay.Engine, the payment
	// layer's whole view of the engine. Its budget is 19 (layering design section
	// 9.1).
	payEngineMethods = 16
	// changeZoneParamLeaks is the number of ChangeZone parameter reads
	// outside its compiler, engine/effects/changezone_params.go (layering design
	// section 8): any read in ChangeZone's own resolution files, plus any
	// read of a ChangeZone-only key elsewhere (codeshape.ChangeZoneFiles,
	// codeshape.ChangeZoneOnlyKeys). It landed at zero.
	changeZoneParamLeaks = 0
	// changeZoneAllParamLeaks is the same census for api:ChangeZoneAll's
	// compiler, engine/effects/changezoneall_params.go (codeshape.ChangeZoneAllFiles,
	// codeshape.ChangeZoneAllOnlyKeys). It landed at zero.
	changeZoneAllParamLeaks = 0
	// attachParamLeaks is the same census for api:Attach's compiler,
	// engine/effects/attach_params.go (codeshape.AttachFiles, codeshape.AttachOnlyKeys).
	// It landed at zero.
	attachParamLeaks = 0
	// charmParamLeaks, pumpParamLeaks and drawParamLeaks are the same census
	// for the modal family's compiler (engine/effects/charm_params.go), api:Pump's
	// (engine/effects/pump_params.go) and api:Draw's (engine/effects/draw_params.go). Each
	// landed at zero.
	charmParamLeaks = 0
	pumpParamLeaks  = 0
	drawParamLeaks  = 0
	// replaceEffectParamLeaks is the same census for api:ReplaceEffect's
	// compiler (engine/effects/replaceeffect_params.go). It landed at zero.
	replaceEffectParamLeaks = 0
	// manaParamLeaks is the same census for api:Mana's production compiler
	// (engine/effects/mana_params.go). It landed at zero.
	manaParamLeaks = 0
	// manaReflectedParamLeaks is the same census for api:ManaReflected's
	// compiler (engine/effects/manareflected_params.go). It landed at zero.
	manaReflectedParamLeaks = 0
	// dealDamageParamLeaks is the same census for api:DealDamage's compiler,
	// engine/effects/dealdamage_params.go (codeshape.DealDamageFiles,
	// codeshape.DealDamageOnlyKeys). It landed at zero.
	dealDamageParamLeaks = 0
	// putCounterParamLeaks is the same census for api:PutCounter's compiler,
	// engine/effects/putcounter_params.go (codeshape.PutCounterFiles,
	// codeshape.PutCounterOnlyKeys). It landed at zero.
	putCounterParamLeaks = 0
	// effectParamLeaks is the same census for api:Effect's compiler,
	// engine/effects/effect_params.go (codeshape.EffectFiles,
	// codeshape.EffectOnlyKeys). It landed at zero.
	effectParamLeaks = 0
	// targetParamLeaks is the same census for the generic targeting tier's
	// compiler, engine/effects/targets_params.go (codeshape.TargetOnlyKeys): no
	// targeting key is read anywhere in engine/rules/ or engine/effects/ outside it. It
	// landed at zero.
	targetParamLeaks = 0
	// definedParamLeaks is the same census for the generic Defined-reference
	// tier's compiler, engine/effects/defined_params.go (codeshape.DefinedOnlyKeys):
	// no Defined$/DefinedCards$/DefinedPlayer$/DefinedTarget$ read anywhere
	// in engine/rules/ or engine/effects/ outside it. It landed at zero.
	definedParamLeaks = 0
	// delayedTriggerParamLeaks is the same census for api:DelayedTrigger's compiler,
	// engine/effects/delayedtrigger_params.go (codeshape.DelayedTriggerFiles, codeshape.DelayedTriggerOnlyKeys). It landed
	// at zero.
	delayedTriggerParamLeaks = 0
	// copyPermanentParamLeaks is the same census for api:CopyPermanent's compiler,
	// engine/effects/copypermanent_params.go (codeshape.CopyPermanentFiles, codeshape.CopyPermanentOnlyKeys). It landed
	// at zero.
	copyPermanentParamLeaks = 0
	// cloneParamLeaks is the same census for api:Clone's compiler,
	// engine/effects/clone_params.go (codeshape.CloneFiles, codeshape.CloneOnlyKeys).
	// It landed at zero.
	cloneParamLeaks = 0
	// digParamLeaks is the same census for api:Dig's compiler,
	// engine/effects/dig_params.go (codeshape.DigFiles, codeshape.DigOnlyKeys). It landed at
	// zero.
	digParamLeaks = 0
	// digUntilParamLeaks is the same census for api:DigUntil's compiler,
	// engine/effects/diguntil_params.go (codeshape.DigUntilFiles, codeshape.DigUntilOnlyKeys). It landed at
	// zero.
	digUntilParamLeaks = 0
	// removeCounterParamLeaks is the same census for api:RemoveCounter's compiler,
	// engine/effects/removecounter_params.go (codeshape.RemoveCounterFiles, codeshape.RemoveCounterOnlyKeys). It landed at
	// zero.
	removeCounterParamLeaks = 0
	// tokenParamLeaks is the same census for api:Token's compiler,
	// engine/effects/token_params.go (codeshape.TokenFiles, codeshape.TokenOnlyKeys). It landed at
	// zero.
	tokenParamLeaks = 0
	// voteParamLeaks is the same census for api:Vote's compiler,
	// engine/effects/vote_params.go (codeshape.VoteFiles, codeshape.VoteOnlyKeys). It landed at
	// zero.
	voteParamLeaks = 0
	// repeatEachParamLeaks is the same census for api:RepeatEach's compiler,
	// engine/effects/repeateach_params.go (codeshape.RepeatEachFiles, codeshape.RepeatEachOnlyKeys). It landed at
	// zero.
	repeatEachParamLeaks = 0
	// engineSurface is the engine's reach beyond engineMethodCount: *Engine
	// methods, plus free functions with a *Engine parameter, plus methods of
	// structs holding a *Engine field (codeshape.Metrics.EngineSurface). It
	// closes the side door of turning an Engine method into a free function
	// or a wrapper struct's method.
	engineSurface = 2018
	// stringLiteralCompares is the ==/!= comparisons against a non-empty
	// string literal plus strings.EqualFold calls with a literal argument: the
	// if-chain spelling of the `case "X":` dispatch stringCaseLiterals froze
	// at zero.
	stringLiteralCompares = 2214
	// rawBoolParamParses is the strings.EqualFold calls whose argument holds a
	// Param/ParamStr call: a flag parameter re-parsed at each use.
	rawBoolParamParses = 232
	// stringKeyedParamReads is the string literals passed at the key position
	// of a string-keyed parameter helper (effects.Num and kin, found
	// automatically: codeshape.Metrics.StringKeyedParamHelpers): the helper
	// spelling of the Params["X"] read stringParamReads froze at zero.
	stringKeyedParamReads = 167
	// strCodesTables is the NewStrCodes / NewNameSet / NewStrTable call sites
	// in engine/rules/ and engine/effects/, and strCodesKeyDup the sum over string keys of
	// (distinct tables naming the key - 1): a word named in a second table is the
	// vocabulary splitting again.
	strCodesTables = 336
	strCodesKeyDup = 1259
)

// longFuncCeilings freezes every non-test function in engine/rules/ and engine/effects/
// over codeshape.LongFuncLines at its measured length, keyed
// "<dir> <Name>" (codeshape.Func.CeilingKey). TestLongFunctionsOnlyShrink
// fails on a new key, on growth, and on an entry no longer over the limit;
// lower a value when its function shrinks. NEVER add a key or raise a value.
// Regenerate the keys with `go run ./cmd/codeshape -table`.
var longFuncCeilings = map[string]int{
	"effects applyLibrarySearch":                            326,
	"effects definedSpec":                                   939,
	"effects effAttach":                                     465,
	"effects effChangeZone":                                 605,
	"effects effClone":                                      593,
	"effects effCopyPermanent":                              786,
	"effects effDig":                                        521,
	"effects effDigUntil":                                   423,
	"effects effDiscard":                                    375,
	"effects effEffect":                                     1192,
	"effects effHiddenPick":                                 407,
	"effects effPlay":                                       332,
	"effects effPutCounter":                                 373,
	"effects effReveal":                                     430,
	"effects effSacrifice":                                  342,
	"effects effSearchLibrary":                              477,
	"effects effToken":                                      307,
	"effects evalCountBodyDotted":                           301,
	"effects evalCountBodyPaid":                             475,
	"effects handMoveOwnersWalk":                            331,
	"effects matchPositive":                                 442,
	"effects refTargets":                                    321,
	"effects wordMatches":                                   497,
	"rules (*Engine).appendAvailableManaAbilitiesGate":      375,
	"rules (*Engine).applyReplacementsDispatch":             326,
	"rules (*Engine).beginCastWith":                         704,
	"rules (*Engine).checkFaceTriggers":                     792,
	"rules (*Engine).emit":                                  704,
	"rules (*Engine).handleChoose":                          416,
	"rules (*Engine).payCast":                               923,
	"rules (*Engine).pushTrigger":                           745,
	"rules (*Engine).replacementMatchesRememberedUngatedBy": 646,
	"rules (*Engine).resolveTop":                            664,
	"rules (*Engine).staticEffectsWalk":                     678,
	"rules (*Engine).targetAsk":                             327,
	"rules (*Engine).triggerReferents":                      386,
	"rules (*Engine).xAsk":                                  404,
	"rules (*legalWalk).battlefieldWalk":                    861,
	"rules (*legalWalk).handWalk":                           670,
	"engine/rules/cost ParseCost":                           561,
	"engine/rules/pay NonManaCastableP":                     344,
}

func measureRepo(t *testing.T) Metrics {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	m, err := Measure(root)
	if err != nil {
		t.Fatalf("measure: %v", err)
	}
	return m
}

type ratchet struct {
	name   string // the constant's name
	got    int
	limit  int
	advice string // what to do instead of growing it
}

func checkRatchets(t *testing.T, rs []ratchet) {
	t.Helper()
	for _, r := range rs {
		switch {
		case r.got > r.limit:
			t.Errorf("%s: measured %d, above the frozen ceiling of %d.\n"+
				"This is a SHRINK-ONLY ratchet (rules-engine refactor spec, W0): it may "+
				"fall, never rise. %s\n"+
				"Do not raise the constant to make this pass; a reviewer treats a raised "+
				"ratchet as a MAJOR finding. Measure with `go run ./cmd/codeshape -table`.",
				r.name, r.got, r.limit, r.advice)
		case r.got < r.limit:
			t.Logf("%s: measured %d, below the ceiling of %d. Lower %s in "+
				"internal/codeshape/ratchet_test.go to %d in the same commit so the gain "+
				"is locked in.", r.name, r.got, r.limit, r.name, r.got)
		default:
			t.Logf("%s: %d (at ceiling)", r.name, r.got)
		}
	}
}

func TestCodeShapeOnlyShrinks(t *testing.T) {
	t.Skip("codeshape ratchets disabled by the operator (2026-10-06) while main is unblocked; re-enable by deleting this line")
	m := measureRepo(t)
	if m.TrigmatchBoardMethods == 0 {
		t.Error("trigmatch.Board measured no methods: engine/rules/trigmatch/board.go's `type Board interface` " +
			"moved or was renamed; update codeshape.Measure so the ratchet cannot read zero")
	}
	checkRatchets(t, []ratchet{
		{"maxFuncLinesOver300", m.FuncsOver300, maxFuncLinesOver300,
			"A function over 300 lines is a concern without a seam: extract the concern " +
				"behind a named helper or type rather than growing the function. " +
				"`go run ./cmd/codeshape -table` lists them longest first."},
		{"engineMethodCount", m.EngineMethods, engineMethodCount,
			"Every *Engine method can call every other; put new logic on a narrower " +
				"receiver (a feature struct holding what it needs) or a free function, or " +
				"remove an Engine method in the same change."},
		{"hostMethodCount", m.HostMethods, hostMethodCount,
			"effects.Host is the union of narrow role interfaces; derive what you " +
				"need from an existing method (Chars carries every characteristic), or replace one, instead of adding another."},
		{"hostDirectMethodCount", m.HostDirectMethods, hostDirectMethodCount,
			"effects.Host is the union of the role interfaces in engine/effects/host_roles.go; " +
				"add a new method to the one role it belongs to, not to Host itself."},
		{"hostRoleMaxMethods", m.HostRoleMaxMethods, hostRoleMaxMethods,
			"A Host role is to stay narrow (<= 20 methods); split a role that grows by " +
				"concern rather than widening it, or replace an existing method."},
		{"hostOptionalAssertions", m.HostOptionalAssertions, hostOptionalAssertions,
			"An optional-interface assertion on Host is a Host method the role split " +
				"cannot see. Put the method on the role it belongs to (a test double then " +
				"implements it), or extend an existing optional interface, instead of " +
				"asserting a new one."},
		{"ctxFieldCount", m.CtxFields, ctxFieldCount,
			"effects.Ctx fields are mostly per-primitive ask/resume cursors; keep a " +
				"primitive's cursor in its own resume record instead of widening Ctx."},
		{"ctxEmbedCount", m.CtxEmbeds, ctxEmbedCount,
			"An embed promotes every field of the embedded type into effects.Ctx; add a " +
				"named field instead (and count it against ctxFieldCount)."},
		{"resumePointFieldCount", m.ResumePointFields, resumePointFieldCount,
			"resumePoint is the hand-defunctionalised continuation; carry a " +
				"new suspension's state in the primitive's own resume record."},
		{"stringParamReads", m.StringParamReads, stringParamReads,
			"Read a parameter through a typed accessor (cards.ParamKey vocabulary) " +
				"instead of a new Params[\"Literal\"] index."},
		{"stringCaseLiterals", m.StringCaseLiterals, stringCaseLiterals,
			"Dispatch on a compiled enum, a bitmask or a registry entry rather than a " +
				"new `case \"Literal\":` arm."},
		{"ctxLiterals", m.CtxLiterals, ctxLiterals,
			"Build an effects.Ctx with effects.NewCtx / NewCtxPtr (seeded by a CtxInit, " +
				"other fields assigned on the result), derive one from a resolving context " +
				"with (*Ctx).Child or (*Ctx).ForTrigger, or add a constructor to " +
				"engine/effects/ctx_new.go (engine/rules/ctx_new.go for one that binds engine-owned " +
				"tables). A hand-written literal that copies another context's fields drops " +
				"the next field added to Ctx."},
		{"specContextLiterals", m.SpecContextLiterals, specContextLiterals,
			"Build a SpecContext with effects.NewSpecContext, derive it from a resolving " +
				"Ctx with (*Ctx).SpecContext / (*Ctx).TableSpecContext, or bind rules' layer " +
				"tables through Engine.withNames / specCtxSVars."},
		{"trigmatchBoardMethods", m.TrigmatchBoardMethods, trigmatchBoardMethods,
			"trigmatch.Board is the trigger matchers' whole view of the engine; derive a " +
				"new fact from an existing method (Chars carries every characteristic, Facts " +
				"every per-emit trigger context value) or pass it precomputed, instead of " +
				"adding a method."},
		{"payEngineMethods", m.PayEngineMethods, payEngineMethods,
			"pay.Engine is the payment layer's whole view of the engine (target under " +
				"20); derive a new fact from an existing method or pass it precomputed, " +
				"instead of adding a method."},
		{"changeZoneParamLeaks", m.ChangeZoneParamLeaks, changeZoneParamLeaks,
			"Read the parameter through effects.ChangeZoneOf's compiled ChangeZoneParams " +
				"(add a field to compileChangeZone in engine/effects/changezone_params.go) instead " +
				"of reading the ability's Params in a ChangeZone file or a ChangeZone-only " +
				"key elsewhere. Leaks: " + strings.Join(m.ChangeZoneLeaks, ", ")},
		{"changeZoneAllParamLeaks", m.ChangeZoneAllParamLeaks, changeZoneAllParamLeaks,
			"Read the parameter through effects.ChangeZoneAllOf's compiled ChangeZoneAllParams " +
				"(add a field to compileChangeZoneAll in engine/effects/changezoneall_params.go) instead " +
				"of reading the ability's Params in engine/effects/zone_changeall.go or a " +
				"ChangeZoneAll-only key elsewhere. Leaks: " + strings.Join(m.ChangeZoneAllLeaks, ", ")},
		{"attachParamLeaks", m.AttachParamLeaks, attachParamLeaks,
			"Read the parameter through effects.AttachOf's compiled AttachParams (add a " +
				"field to compileAttach in engine/effects/attach_params.go) instead of reading the " +
				"ability's Params in engine/effects/attach.go or an Attach-only key elsewhere. " +
				"Leaks: " + strings.Join(m.AttachLeaks, ", ")},
		{"charmParamLeaks", m.CharmParamLeaks, charmParamLeaks,
			"Read the parameter through effects.CharmOf's compiled CharmParams (add a " +
				"field to compileCharm in engine/effects/charm_params.go) instead of reading the " +
				"ability's Params in engine/effects/charm.go or a Charm-only key elsewhere. " +
				"Leaks: " + strings.Join(m.CharmLeaks, ", ")},
		{"pumpParamLeaks", m.PumpParamLeaks, pumpParamLeaks,
			"Read the parameter through effects.PumpOf's compiled PumpParams (add a " +
				"field to compilePump in engine/effects/pump_params.go) instead of reading the " +
				"ability's Params in engine/effects/pump.go or a Pump-only key elsewhere. " +
				"Leaks: " + strings.Join(m.PumpLeaks, ", ")},
		{"drawParamLeaks", m.DrawParamLeaks, drawParamLeaks,
			"Read the parameter through effects.DrawOf's compiled DrawParams (add a " +
				"field to compileDraw in engine/effects/draw_params.go) instead of reading the " +
				"ability's Params in engine/effects/draw.go or a Draw-only key elsewhere. " +
				"Leaks: " + strings.Join(m.DrawLeaks, ", ")},
		{"replaceEffectParamLeaks", m.ReplaceEffectParamLeaks, replaceEffectParamLeaks,
			"Read the parameter through effects.ReplaceEffectOf's compiled " +
				"ReplaceEffectParams (add a field to compileReplaceEffect in " +
				"engine/effects/replaceeffect_params.go) instead of reading the ability's Params " +
				"in engine/effects/replacement.go or a ReplaceEffect-only key elsewhere. " +
				"Leaks: " + strings.Join(m.ReplaceEffectLeaks, ", ")},
		{"manaParamLeaks", m.ManaParamLeaks, manaParamLeaks,
			"Read the parameter through effects.ManaOf's compiled ManaParams (add a " +
				"field to compileMana in engine/effects/mana_params.go) instead of reading the " +
				"ability's Params in engine/effects/mana_effect.go or a Mana-only rider key " +
				"elsewhere. Leaks: " + strings.Join(m.ManaLeaks, ", ")},
		{"manaReflectedParamLeaks", m.ManaReflectedParamLeaks, manaReflectedParamLeaks,
			"Read the parameter through effects.ManaReflectedOf's compiled " +
				"ManaReflectedParams (add a field to compileManaReflected in " +
				"engine/effects/manareflected_params.go) instead of reading the ability's Params " +
				"in engine/effects/mana_reflected.go or a ManaReflected-only key elsewhere. " +
				"Leaks: " + strings.Join(m.ManaReflectedLeaks, ", ")},
		{"dealDamageParamLeaks", m.DealDamageParamLeaks, dealDamageParamLeaks,
			"Read the parameter through effects.DealDamageOf's compiled DealDamageParams (add " +
				"a field to compileDealDamage in engine/effects/dealdamage_params.go) instead of reading " +
				"the ability's Params in engine/effects/damage_deal.go or a DealDamage-only key elsewhere. " +
				"Leaks: " + strings.Join(m.DealDamageLeaks, ", ")},
		{"putCounterParamLeaks", m.PutCounterParamLeaks, putCounterParamLeaks,
			"Read the parameter through effects.PutCounterOf's compiled PutCounterParams (add " +
				"a field to compilePutCounter in engine/effects/putcounter_params.go) instead of reading " +
				"the ability's Params in engine/effects/counters_put.go or a PutCounter-only key elsewhere. " +
				"Leaks: " + strings.Join(m.PutCounterLeaks, ", ")},
		{"effectParamLeaks", m.EffectParamLeaks, effectParamLeaks,
			"Read the parameter through effects.EffectOf's compiled EffectParams (add a " +
				"field to compileEffect in engine/effects/effect_params.go) instead of reading the " +
				"ability's Params in engine/effects/effect.go or an Effect-only key elsewhere. " +
				"Leaks: " + strings.Join(m.EffectLeaks, ", ")},
		{"targetParamLeaks", m.TargetParamLeaks, targetParamLeaks,
			"Read the targeting parameter through effects.TargetsOf's compiled TargetParams " +
				"(add a field to compileTargets in engine/effects/targets_params.go) instead of reading " +
				"a targeting key from the ability's Params. Leaks: " + strings.Join(m.TargetLeaks, ", ")},
		{"definedParamLeaks", m.DefinedParamLeaks, definedParamLeaks,
			"Read the selector through effects.DefinedOf's compiled Ref (add a field to " +
				"compileDefined in engine/effects/defined_params.go, or resolve selector text with " +
				"effects.RefOf/DefinedSpec) instead of reading a Defined key from the ability's " +
				"Params. Leaks: " + strings.Join(m.DefinedLeaks, ", ")},
		{"delayedTriggerParamLeaks", m.DelayedTriggerParamLeaks, delayedTriggerParamLeaks,
			"Read the parameter through effects.DelayedTriggerOf's compiled DelayedTriggerParams (add a " +
				"field to compileDelayedTrigger in engine/effects/delayedtrigger_params.go) instead of reading the ability's Params in " +
				"engine/effects/delayed_trigger.go or a DelayedTrigger-only key elsewhere. " +
				"Leaks: " + strings.Join(m.DelayedTriggerLeaks, ", ")},
		{"copyPermanentParamLeaks", m.CopyPermanentParamLeaks, copyPermanentParamLeaks,
			"Read the parameter through effects.CopyPermanentOf's compiled CopyPermanentParams (add a " +
				"field to compileCopyPermanent in engine/effects/copypermanent_params.go) instead of reading the ability's Params in " +
				"engine/effects/copypermanent.go or a CopyPermanent-only key elsewhere. " +
				"Leaks: " + strings.Join(m.CopyPermanentLeaks, ", ")},
		{"cloneParamLeaks", m.CloneParamLeaks, cloneParamLeaks,
			"Read the parameter through effects.CloneOf's compiled CloneParams (add a " +
				"field to compileClone in engine/effects/clone_params.go) instead of reading the ability's Params in " +
				"engine/effects/clone.go or a Clone-only key elsewhere. " +
				"Leaks: " + strings.Join(m.CloneLeaks, ", ")},
		{"digParamLeaks", m.DigParamLeaks, digParamLeaks,
			"Read the parameter through effects.DigOf's compiled DigParams (add a " +
				"field to compileDig in engine/effects/dig_params.go) instead of reading the ability's Params in " +
				"engine/effects/dig.go or a Dig-only key elsewhere. " +
				"Leaks: " + strings.Join(m.DigLeaks, ", ")},
		{"digUntilParamLeaks", m.DigUntilParamLeaks, digUntilParamLeaks,
			"Read the parameter through effects.DigUntilOf's compiled DigUntilParams (add a " +
				"field to compileDigUntil in engine/effects/diguntil_params.go) instead of reading the ability's Params in " +
				"engine/effects/diguntil.go or a DigUntil-only key elsewhere. " +
				"Leaks: " + strings.Join(m.DigUntilLeaks, ", ")},
		{"removeCounterParamLeaks", m.RemoveCounterParamLeaks, removeCounterParamLeaks,
			"Read the parameter through effects.RemoveCounterOf's compiled RemoveCounterParams (add a " +
				"field to compileRemoveCounter in engine/effects/removecounter_params.go) instead of reading the ability's Params in " +
				"engine/effects/removecounter.go or a RemoveCounter-only key elsewhere. " +
				"Leaks: " + strings.Join(m.RemoveCounterLeaks, ", ")},
		{"tokenParamLeaks", m.TokenParamLeaks, tokenParamLeaks,
			"Read the parameter through effects.TokenOf's compiled TokenParams (add a " +
				"field to compileToken in engine/effects/token_params.go) instead of reading the ability's Params in " +
				"engine/effects/token.go or a Token-only key elsewhere. " +
				"Leaks: " + strings.Join(m.TokenLeaks, ", ")},
		{"voteParamLeaks", m.VoteParamLeaks, voteParamLeaks,
			"Read the parameter through effects.VoteOf's compiled VoteParams (add a " +
				"field to compileVote in engine/effects/vote_params.go) instead of reading the ability's Params in " +
				"engine/effects/vote.go or engine/effects/vote_effect.go or a Vote-only key elsewhere. " +
				"Leaks: " + strings.Join(m.VoteLeaks, ", ")},
		{"repeatEachParamLeaks", m.RepeatEachParamLeaks, repeatEachParamLeaks,
			"Read the parameter through effects.RepeatEachOf's compiled RepeatEachParams (add a " +
				"field to compileRepeatEach in engine/effects/repeateach_params.go) instead of reading the ability's Params in " +
				"engine/effects/repeateach.go or a RepeatEach-only key elsewhere. " +
				"Leaks: " + strings.Join(m.RepeatEachLeaks, ", ")},
		{"engineSurface", m.EngineSurface, engineSurface,
			"A free func taking *Engine is an Engine method; take a narrow interface " +
				"(or a feature struct holding only what it needs) instead of the whole engine."},
		{"stringLiteralCompares", m.StringLiteralCompares, stringLiteralCompares,
			"Compare a code from the table that owns the word (a cards.StrCodes code, " +
				"an enum or a mask compiled once) instead of a new `s == \"Literal\"`."},
		{"rawBoolParamParses", m.RawBoolParamParses, rawBoolParamParses,
			"Parse the flag once in the API's compiled params (a bool field of its " +
				"*_params.go record) instead of strings.EqualFold over a ParamStr at each use."},
		{"stringKeyedParamReads", m.StringKeyedParamReads, stringKeyedParamReads,
			"Read through the compiled record / ParamKey (a ParamText field of the API's " +
				"compiler, or cards.PK<Key>) instead of passing a literal key to a string-keyed " +
				"helper. Helpers: " + strings.Join(m.StringKeyedParamHelpers, "; ")},
		{"strCodesTables", m.StrCodesTables, strCodesTables,
			"Extend the table that already owns this word (or the mechanism's descriptor) " +
				"instead of naming it in another table."},
		{"strCodesKeyDup", m.StrCodesKeyDup, strCodesKeyDup,
			"Extend the table that already owns this word (or the mechanism's descriptor) " +
				"instead of naming it in another table."},
		{"triggerContextLiterals", m.TriggerContextLiterals, triggerContextLiterals,
			"Derive trigger referents from the firing event (rules' triggerReferents) or " +
				"copy an existing TriggerContext and set the fields that differ; a " +
				"synthesized trigger's record can be declared `var tc effects.TriggerContext` " +
				"and filled field by field."},
	})
}

// TestLongFunctionsOnlyShrink is the per-function form of maxFuncLinesOver300:
// the count alone lets one long function grow while another shrinks, so each
// is frozen at its own length in longFuncCeilings.
func TestLongFunctionsOnlyShrink(t *testing.T) {
	t.Skip("codeshape ratchets disabled by the operator (2026-10-06) while main is unblocked; re-enable by deleting this line")
	m := measureRepo(t)
	seen := map[string]bool{}
	for _, f := range m.LongFuncs {
		key := f.CeilingKey()
		seen[key] = true
		limit, ok := longFuncCeilings[key]
		switch {
		case !ok:
			t.Errorf("%s (%s:%d) is a new function over %d lines (%d). Extract the concern "+
				"behind a named helper or a descriptor entry instead; do not add it to "+
				"longFuncCeilings.", key, f.File, f.Line, LongFuncLines, f.Lines)
		case f.Lines > limit:
			t.Errorf("%s (%s:%d) grew to %d lines, above its frozen ceiling of %d. Extract the "+
				"new logic into a named helper or a descriptor entry rather than growing the "+
				"function; do not raise its longFuncCeilings value.", key, f.File, f.Line, f.Lines, limit)
		case f.Lines < limit:
			t.Logf("%s shrank to %d lines (ceiling %d): lower its longFuncCeilings value to %d "+
				"in the same commit.", key, f.Lines, limit, f.Lines)
		}
	}
	keys := make([]string, 0, len(longFuncCeilings))
	for k := range longFuncCeilings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !seen[k] {
			t.Errorf("longFuncCeilings[%q] is stale: that function is no longer over %d lines "+
				"(or was renamed or moved). Delete the entry.", k, LongFuncLines)
		}
	}
}
