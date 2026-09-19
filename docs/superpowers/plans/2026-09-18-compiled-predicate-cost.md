# Compiled Predicate and Cost Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Eliminate repeated parsing of configured Forge filter and cost text on rules hot paths while preserving exact match, payment, replay, and clone behavior.

**Architecture:** rules.Engine owns a read-only compiledText sidecar built from configured cards and tokens, and Clone shares it. effects compiles conservative filter programs with yes/no/maybe results; only maybe invokes the existing textual matcher. The same sidecar stores frozen Cost values and serves engine-local parser calls. Public ParseCost remains the fallback and diagnostic oracle.

**Tech Stack:** Go standard library plus existing cards, effects, rules, state, and testing packages. No new dependencies.

**Spec:** docs/superpowers/specs/2026-09-18-compiled-predicate-cost-design.md

## Global Constraints

- Keep the pipeline and rules core pure Go; add no cgo or third-party dependency.
- Never add Forge scripts or .cards contents to git.
- Preserve mutation through events.Apply, all event/replay/decision/protocol formats, RNG, face-local trigger/ability ordinals, and clone independence.
- The sidecar is immutable after New and never enters Config, state.Game, an event, replay, decision, snapshot, or protocol.
- Collect source text deterministically: Config.Decks order, token keys sorted, source maps sorted, then unique texts sorted.
- Unconfigured cards, on-demand SVars, dynamic text, and unsupported grammar must take the existing textual path.
- Do not regenerate goldens, run race tests, push, merge, rebase, or create a PR without authorization.
- Before claiming success, compare normalized 500-game output exactly with /tmp/gorge-searchprobe-post-ability-fix-500-20260918.json.

---

### Task 1: Pin focused predicate and parser baselines

**Files:**

- Modify: effects/filter_hotspots_test.go
- Modify: rules/mana_hotspots_test.go
- Create: docs/superpowers/reports/2026-09-18-compiled-predicate-cost-running.md

**Interfaces:**

- Consumes: effects.MatchesSpecCtx, rules.ParseCost, and existing board fixtures.
- Produces: BenchmarkFilterTextual* and BenchmarkParseCost* names used by every subsequent measurement.

- [ ] **Step 1: Write filter benchmarks**

Add match, reject, and unknown/fail-closed cases with package sinks and ReportAllocs:

~~~go
func BenchmarkFilterTextualMatch(b *testing.B) {
	g, ids := board(b)
	sc := SpecContext{You: 0, Source: ids["myBear"]}
	b.ReportAllocs()
	for range b.N {
		benchmarkMatch = MatchesSpecCtx(g, "Creature.YouCtrl+tapped", ids["myBear"], sc)
	}
}
~~~

Add equivalent Land,Artifact rejection and Creature.UnknownPredicate fallback cases. Preserve TestSimpleFilterMatchingDoesNotAllocate.

- [ ] **Step 2: Write parser benchmarks**

Benchmark ParseCost on "2 U U", "GWP 2B Sac<1/Creature>", and "1 B ExileFromGrave<1/CARDNAME> PayLife<2>", storing each Cost in a package sink.

- [ ] **Step 3: Run and record five-run medians**

~~~bash
go test ./effects -run '^$' -bench 'BenchmarkFilterTextual' -benchmem -count=5
go test ./rules -run '^$' -bench 'BenchmarkParseCost' -benchmem -count=5
~~~

Record Go version, CPU, medians, and the existing fixed profile evidence: filters 21.02 CPU-s and parser 18.22 CPU-s.

- [ ] **Step 4: Verify and commit**

~~~bash
go test ./effects ./rules -run 'TestSimpleFilterMatchingDoesNotAllocate' -count=1
git diff --check
git add effects/filter_hotspots_test.go rules/mana_hotspots_test.go docs/superpowers/reports/2026-09-18-compiled-predicate-cost-running.md
git commit -m "test: benchmark predicate and cost parsing"
~~~

### Task 2: Implement immutable tri-state predicate programs

**Files:**

- Create: effects/compiled_predicate.go
- Create: effects/compiled_predicate_test.go
- Modify: effects/filter.go
- Modify: effects/filter_hotspots_test.go

**Interfaces:**

- Produces: PredicateResult; PredicatePrograms; CompilePredicatePrograms([]string) *PredicatePrograms; and (*PredicatePrograms).Evaluate(string, *state.Game, *state.Object, SpecContext) PredicateResult.
- Consumes: filterAlternatives, matchesBase, matchPredicate, hasType, and ColorsOf as the textual oracle.

- [ ] **Step 1: Write failing compiler/evaluator tests**

Compile duplicate/reordered values and assert stable program count. On the board fixture assert these results:

~~~go
ps := CompilePredicatePrograms([]string{
	"Creature.YouCtrl+tapped", "Land,Artifact", "Creature.UnknownPredicate",
})
sc := SpecContext{You: 0, Source: ids["myBear"]}
want := map[string]PredicateResult{
	"Creature.YouCtrl+tapped": PredicateYes,
	"Land,Artifact": PredicateNo,
	"Creature.UnknownPredicate": PredicateMaybe,
}
for spec, result := range want {
	if got := ps.Evaluate(spec, g, g.Obj(ids["myBear"]), sc); got != result {
		t.Fatalf("%s = %v, want %v", spec, got, result)
	}
}
~~~

For each definite result, require bool parity with the current textual matcher. Assert absent source text is PredicateMaybe.

- [ ] **Step 2: Confirm the API is absent**

~~~bash
go test ./effects -run 'TestCompiledPredicate' -count=1
~~~

Expected: compile failure for compiler/result symbols.

- [ ] **Step 3: Implement the narrow compiler**

Define:

~~~go
type PredicateResult uint8
const (
	PredicateMaybe PredicateResult = iota
	PredicateNo
	PredicateYes
)
type PredicatePrograms struct { byText map[string]predicateProgram }
~~~

Keep programs, alternatives, and terms unexported. Sort and copy inputs, omitting empty strings. Compile only current exact grammar: normal type bases; Any, Permanent, Spell, SpellAbility; literal colours/types with non and leading !; token, tapped/untapped, attacking, kicked/surged/escaped; ownership/controller; Self/Other; fixed keyword predicates. Represent numeric, name, attachment, blocking, referent, ambiguous, and unknown parts as maybe terms.

A false known term makes its alternative false. An all-true supported alternative is yes. A viable alternative with a maybe term is maybe. All false alternatives is no. Do not allocate split slices while evaluating.

- [ ] **Step 4: Bridge public matching to textual fallback**

Extract the existing MatchesObjectCtx body unchanged into private matchesObjectText. Add PredicatePrograms *PredicatePrograms to SpecContext, default nil. Dispatch:

~~~go
if ps := sc.PredicatePrograms; ps != nil {
	switch ps.Evaluate(spec, g, o, sc) {
	case PredicateYes:
		return true
	case PredicateNo:
		return false
	}
}
return matchesObjectText(g, spec, o, sc)
~~~

Existing SpecContext literals remain textual automatically.

- [ ] **Step 5: Add matrix, allocation, and benchmark coverage**

Cover both truth outcomes for every supported opcode, mixed alternatives, source-relative terms, leading !, and all listed maybe families. Require zero allocations for compiled yes/no evaluation. Add BenchmarkFilterCompiledYes, BenchmarkFilterCompiledNo, and BenchmarkFilterCompiledMaybe; the maybe benchmark must call the public matcher.

- [ ] **Step 6: Verify and commit**

~~~bash
go test ./effects -count=1
go test ./effects -run '^$' -bench 'BenchmarkFilter(Textual|Compiled)' -benchmem -count=5
git diff --check
git add effects/compiled_predicate.go effects/compiled_predicate_test.go effects/filter.go effects/filter_hotspots_test.go
git commit -m "feat: compile conservative filter predicates"
~~~

### Task 3: Build deterministic engine text collection and clone sharing

**Files:**

- Create: rules/compiled_text.go
- Create: rules/compiled_text_test.go
- Modify: rules/engine.go
- Modify: rules/clone.go
- Modify: rules/statics.go

**Interfaces:**

- Produces: private compiledText; newCompiledText(Config) *compiledText; (*Engine).parseCost(string) Cost; and Engine.specCtx carrying PredicatePrograms.
- Consumes: Config.Decks, Config.Tokens, cards.Card, linked cards.SA, and effects.CompilePredicatePrograms.

- [ ] **Step 1: Write failing lifecycle tests**

Build equivalent Configs with reverse token-map insertion. Assert New produces equal predicate/cost key sets. Assert clone.compiledText == original.compiledText while normal game mutation remains independent. For configured Cost$ 1 U require reflect.DeepEqual(e.parseCost("1 U"), ParseCost("1 U")). Assert absent text and a post-New Game.AddObject card safely fall back.

- [ ] **Step 2: Confirm the symbols are absent**

~~~bash
go test ./rules -run 'TestCompiledText' -count=1
~~~

Expected: compile failure for compiledText and parseCost.

- [ ] **Step 3: Collect text deterministically and freeze costs**

Implement:

~~~go
type compiledText struct {
	predicates *effects.PredicatePrograms
	costs      map[string]Cost
}
~~~

Walk decks in slice order and tokens by sorted key. For every non-nil face, collect ManaCost; direct abilities and recursive linked Sub; trigger effects; replacement With; statics; and parameter values with sorted keys. Add ManaCost, Cost, and UnlessCost values to the cost set; add every nonempty parameter value to the predicate-candidate set. Deduplicate shared SA pointers. Sort unique strings before compilation and retain no collection maps.

Parse each cost once with public ParseCost. Freeze every slice field with s[:len(s):len(s)] before storing it, including hybrid, Phyrexian, twobrid, cost-part, and Unknown slices.

- [ ] **Step 4: Wire New, Clone, contexts, and fallback lookup**

Add compiledText *compiledText to Engine; build it in newWithRNG before events; share it in Clone. Modify Engine.specCtx to attach the predicate set. Implement:

~~~go
func (e *Engine) parseCost(raw string) Cost {
	if e != nil && e.compiledText != nil {
		if c, ok := e.compiledText.costs[raw]; ok {
			return c
		}
	}
	return ParseCost(raw)
}
~~~

Never put the sidecar in Config, Game, events, or mutable continuations.

- [ ] **Step 5: Verify and commit**

~~~bash
go test ./rules -run 'TestCompiledText|Test.*Clone' -count=1
go test ./rules ./effects -count=1
git diff --check
git add rules/compiled_text.go rules/compiled_text_test.go rules/engine.go rules/clone.go rules/statics.go
git commit -m "feat: cache configured predicate and cost text"
~~~

### Task 4: Propagate predicate sets through engine contexts

**Files:**

- Modify: rules/layers.go
- Modify: rules/trigger_referents.go
- Modify: rules/mana.go
- Modify: rules/cast.go
- Modify: rules/legal.go
- Modify: rules/replacement.go
- Modify: rules/statics.go
- Modify: rules/compiled_text_test.go

**Interfaces:**

- Consumes: Engine.specCtx(source, player) from Task 3.
- Produces: engine-originated filter reads use the sidecar; effects and non-engine test contexts retain text fallback.

- [ ] **Step 1: Write failing propagation/parity tests**

Use configured ValidTgts$ Creature.YouCtrl+tapped to create a target offer and assert its context carries the predicate set through a test-only lookup accessor. Add configured trigger/replacement ValidCard$ Land,Artifact and require the reject stays false. Add an inline card after New with equivalent text and require textual parity.

- [ ] **Step 2: Replace direct engine-owned context literals**

For each listed file, begin with sc := e.specCtx(source, controller), then set only preexisting special fields such as AsStack, Resolve, TriggerContext, targets, remembered/chosen values, and mana-value overrides:

~~~go
sc := e.specCtx(ce.Source, ce.Controller)
sc.AsStack = atStack != 0
~~~

Never overwrite sc.PredicatePrograms. Do not alter contexts in effects or fixtures that do not come from Engine.

- [ ] **Step 3: Add corpus differential coverage**

For every collected source text and initial game object, compare each definite program result with matchesObjectText. Permit PredicateMaybe. On mismatch, narrow the compiler to maybe rather than modifying text semantics.

- [ ] **Step 4: Verify and commit**

~~~bash
go test ./rules -run 'Test.*(Target|Trigger|Replacement|Layer|Static|CompiledText)' -count=1
go test ./effects -run 'Test.*Filter' -count=1
git diff --check
git add rules/layers.go rules/trigger_referents.go rules/mana.go rules/cast.go rules/legal.go rules/replacement.go rules/statics.go rules/compiled_text_test.go
git commit -m "perf: use compiled predicates from engine contexts"
~~~

### Task 5: Migrate repeated engine cost parsing safely

**Files:**

- Modify: rules/mana_activation.go
- Modify: rules/mana_available.go
- Modify: rules/activate.go
- Modify: rules/legal.go
- Modify: rules/speed.go
- Modify: rules/statics.go
- Modify: rules/cast.go
- Modify: rules/ward.go
- Modify: rules/mayplay.go
- Modify: rules/cumulative.go
- Modify: rules/rooms.go
- Modify: rules/resolution.go
- Modify: rules/compiled_text_test.go
- Modify: rules/mana_hotspots_test.go

**Interfaces:**

- Consumes: (*Engine).parseCost(string) Cost from Task 3.
- Produces: receiver-owned cost reads use immutable configured values and every other call remains fallback-safe.

- [ ] **Step 1: Write failing frozen-cache and payment-parity tests**

Configure a cost containing hybrid, sacrifice, discard, counter, exile, and Unknown data. Deep-compare cache hit to ParseCost. Append a CostPart to every nonempty returned slice; a later hit must still equal the parser result. Add castable/uncastable and mana-ability fixtures comparing event sequence, pool, and life with an absent-string fallback fixture.

- [ ] **Step 2: Replace receiver-owned parser calls**

Replace only calls with an Engine receiver:

~~~go
cost := e.parseCost(ma.Params["Cost"])
printed := e.parseCost(o.Face().ManaCost)
alt := e.parseCost(raw)
~~~

Preserve public parser helpers, pre-engine work, parser regexes, Unknown reporting, Cost.Plus, payment ordering, and dynamic-string fallback behavior.

- [ ] **Step 3: Add cached-hit benchmarks**

Add BenchmarkEngineParseCostCached* with actual New Configs carrying the three Task 1 values. Require zero allocations for simple and complex cache hits; report sidecar construction separately.

- [ ] **Step 4: Verify and commit**

~~~bash
go test ./rules -run 'Test.*(Cost|Mana|Cast|Activate|Ward|Unless|Cumulative|Alternative|CompiledText)' -count=1
go test ./rules -run '^$' -bench '(BenchmarkParseCost|BenchmarkEngineParseCostCached)' -benchmem -count=5
git diff --check
git add rules/mana_activation.go rules/mana_available.go rules/activate.go rules/legal.go rules/speed.go rules/statics.go rules/cast.go rules/ward.go rules/mayplay.go rules/cumulative.go rules/rooms.go rules/resolution.go rules/compiled_text_test.go rules/mana_hotspots_test.go
git commit -m "perf: reuse configured parsed costs"
~~~

### Task 6: Verify integration and make the retention decision

**Files:**

- Modify: docs/superpowers/reports/2026-09-18-compiled-predicate-cost-running.md

**Interfaces:**

- Consumes: Tasks 1-5 and the preserved post-AbilityPush control artifact.
- Produces: focused/end-to-end measurements and an evidence-backed retain or revert decision.

- [ ] **Step 1: Run quality gates**

~~~bash
go test ./cards ./effects ./rules ./state ./internal/searchprobe ./cmd/searchprobe -count=1
go vet ./...
git diff --check
~~~

Reproduce any known full-suite failure at 1b703c0 before recording it. Do not hide a failure by changing host behavior or a golden.

- [ ] **Step 2: Produce a fresh fixed-workload artifact**

Run the exact 500-game command/options that produced /tmp/gorge-compiled-face-500-20260918.json. Retain CPU/heap profiles under new date-stamped /tmp/gorge-compiled-predicate-cost-* names. Do not run race tests or mutate corpus caches.

- [ ] **Step 3: Compare normalized semantics exactly**

On copies remove only top-level LoadSeconds, TotalSeconds, AllocatedBytes, HeapAllocBytes, HeapSysBytes and result SampleNS/SearchNS. Compare remaining JSON byte-for-byte. Require 500 games, 498 eligible roots, two no-root games, 100 covered roots, and zero errors.

- [ ] **Step 4: Record and commit retention decision**

Record focused medians, construction/retained-memory cost, fixed-workload wall/CPU/allocation deltas, and exact normalized comparison. Retain only if direct paths improve without new steady-state allocation and normalized workload is exact; otherwise revert migration commits while preserving baseline/report evidence.

~~~bash
git diff --check
git add docs/superpowers/reports/2026-09-18-compiled-predicate-cost-running.md
git commit -m "docs: report predicate and cost measurements"
~~~

