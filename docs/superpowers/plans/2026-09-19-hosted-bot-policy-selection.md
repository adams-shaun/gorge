# Hosted Bot Policy Selection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let hosted tables and play-vs-bot games select a stable deterministic bot policy by name, defaulting safely to `bot` and making experimental selection deliberate and reproducible.

**Architecture:** `host` owns the supported hosted-policy vocabulary and normalizes/validates a table's persisted `bot_policy` before it can run. It constructs every bot slot and every human timeout caretaker through the same seeded policy factory. Protocol metadata reports the effective name without adding an event or exposing hidden information; `cmd/botbench` shares the production factory while retaining `legacy` as a local diagnostic-only adapter.

**Tech Stack:** Go standard library, existing `host`, `seat`, `botpolicy`, `protocol`, `cmd/gorged`, and generated TypeScript protocol types; no new dependencies.

**Spec:** `docs/superpowers/specs/2026-09-19-hosted-bot-policy-selection-design.md`

## Global Constraints

- Do not commit Forge scripts or `.cards` content.
- Use no cgo or third-party dependencies.
- Keep game-state mutation exclusively in `events.Apply`.
- Keep selection and all outcomes deterministic: no wall clock, ambient random source, or map iteration reaching an event.
- Never expose unrevealed opponent cards, library order, or unredacted decision data to a bot.
- `bot` is the hosted default; `lethal-pressure` is explicit experimental opt-in; `legacy` is diagnostic-only and never host-selectable.
- Reject unknown names; never silently substitute a policy or random behavior.
- Do not regenerate golden heads, push, merge, rebase, or force-push.
- Commit each independently tested task and add a concise verified report under `docs/superpowers/reports/` at the final integration milestone.

## Review Focus

- An old persisted table with no `bot_policy` must load, normalize to `bot`, and replay without altering its event chain (Task 2).
- A `legacy` spelling that is valid in botbench must be rejected by table validation and `POST /api/games` (Tasks 2 and 4).
- A human timeout must use the selected policy rather than a hard-coded `bot` caretaker (Task 3).
- The same policy and seed must yield the same board-adapter and view-adapter intent stream using only public board facts (Task 1).
- Public policy metadata must be present in table/match stream and REST data but must not become a game event or affect the production head (Tasks 3 and 5).

---

## File structure

| File | Responsibility |
| --- | --- |
| `host/bot_policy.go` | Hosted policy names, normalization, deterministic constructor, and intentionally closed hosted vocabulary. |
| `host/bot_policy_test.go` | Resolver, rejection, deterministic constructor, and adapter-parity coverage. |
| `host/table.go` | Persisted `TableConfig.BotPolicy` and pre-registration validation. |
| `host/registry.go`, `host/match.go` | Normalize configuration once and construct slots/caretakers through the resolved factory. |
| `host/persist.go`, `host/restart.go` | Preserve effective policy identity in match sidecars and restored tables. |
| `protocol/protocol.go`, `web/src/protocol.ts` | Additive public metadata fields and generated TypeScript mirror. |
| `host/*_test.go` | Table configuration, persistence, controller construction, replay, and metadata tests. |
| `host/httpapi/rest.go`, `host/httpapi/game_test.go` | `bot_policy` decode and unknown-name HTTP rejection. |
| `cmd/gorged/game.go`, `cmd/gorged/game_test.go` | Persist the requested/default selected policy in on-demand tables. |
| `cmd/botbench/main.go`, `cmd/botbench/*_test.go` | Reuse production constructors while retaining diagnostic-only `legacy`. |
| `seat/integration_test.go` | Both `bot` variants' view/board adapter parity. |
| `docs/superpowers/reports/2026-09-19-hosted-bot-policy-selection.md` | Verification evidence, non-benchmark scope, risks, and AR8 follow-up. |

### Task 1: Closed hosted-policy factory and adapter parity

**Files:**
- Create: `host/bot_policy.go`
- Create: `host/bot_policy_test.go`
- Modify: `seat/integration_test.go`

**Interfaces:**
- Produces `const BotPolicy = "bot"` and `const LethalPressurePolicy = "lethal-pressure"`.
- Produces `func NormalizeBotPolicy(name string) (string, error)` where `""` returns `BotPolicy` and every unsupported name returns an error whose known hosted names are in fixed lexical order.
- Produces `func NewBotPolicySeat(name string, seed uint64) (seat.Seat, error)`; it normalizes first, then returns `seat.NewBot(seed)` or `seat.NewLethalPressureBot(seed)`.
- Consumed by Tasks 2–5. `legacy` is deliberately absent from this interface.

- [ ] **Step 1: Write resolver and construction tests before implementation**

```go
func TestNormalizeBotPolicy(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", BotPolicy}, {BotPolicy, BotPolicy}, {LethalPressurePolicy, LethalPressurePolicy},
	} {
		got, err := NormalizeBotPolicy(tc.in)
		if err != nil || got != tc.want { t.Fatalf("%q: %q, %v", tc.in, got, err) }
	}
	for _, bad := range []string{"legacy", "random", "Bot"} {
		if _, err := NormalizeBotPolicy(bad); err == nil { t.Fatalf("%q accepted", bad) }
	}
}

func TestNewBotPolicySeatIsDeterministic(t *testing.T) {
	d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "pass"}, {Index: 1, Kind: "cast"}}}
	for _, policy := range []string{BotPolicy, LethalPressurePolicy} {
		a, _ := NewBotPolicySeat(policy, 19); b, _ := NewBotPolicySeat(policy, 19)
		ia, _ := a.Decide(context.Background(), view.View{}, d); ib, _ := b.Decide(context.Background(), view.View{}, d)
		if !slices.Equal(ia.Choices, ib.Choices) { t.Fatalf("%s choices differ", policy) }
	}
}
```

- [ ] **Step 2: Run the new test to verify it fails**

Run: `go test ./host -run 'TestNormalizeBotPolicy|TestNewBotPolicySeatIsDeterministic' -count=1`

Expected: compile failure because `NormalizeBotPolicy` and `NewBotPolicySeat` do not exist.

- [ ] **Step 3: Implement the closed factory**

```go
const (
	BotPolicy            = "bot"
	LethalPressurePolicy = "lethal-pressure"
)

func NormalizeBotPolicy(name string) (string, error) {
	if name == "" { return BotPolicy, nil }
	switch name {
	case BotPolicy, LethalPressurePolicy:
		return name, nil
	default:
		return "", fmt.Errorf("host: unknown bot policy %q (known: bot, lethal-pressure)", name)
	}
}

func NewBotPolicySeat(name string, seed uint64) (seat.Seat, error) {
	name, err := NormalizeBotPolicy(name)
	if err != nil { return nil, err }
	if name == LethalPressurePolicy { return seat.NewLethalPressureBot(seed), nil }
	return seat.NewBot(seed), nil
}
```

- [ ] **Step 4: Extend the existing adapter parity table to both hosted policies**

Replace direct `NewBot` construction in the per-step parity test with a table over `NewBot` and `NewLethalPressureBot`, and use `DecideBoard` for the game-shaped half. Retain the existing whole-game public-board equality tests; do not add any information to `botpolicy.Board`.

- [ ] **Step 5: Run focused policy and seat tests**

Run: `go test ./host ./seat -run 'TestNormalizeBotPolicy|TestNewBotPolicySeatIsDeterministic|TestBotAdaptersAgree' -count=1`

Expected: PASS. Both variants answer the same view-shaped and board-shaped decision from the same public facts and seed.

- [ ] **Step 6: Commit the isolated policy factory**

```bash
git add host/bot_policy.go host/bot_policy_test.go seat/integration_test.go
git commit -m "feat(host): define named bot policy factory"
```

### Task 2: Persist and validate the effective table policy

**Files:**
- Modify: `host/table.go`
- Modify: `host/registry.go`
- Modify: `host/restart.go`
- Modify: `host/persist_test.go`
- Modify: `host/host_test.go`

**Interfaces:**
- Consumes `NormalizeBotPolicy(string) (string, error)` from Task 1.
- Produces a normalized `TableConfig.BotPolicy string` stored as JSON `bot_policy` and used by every later host path.
- `AddTable` and restored `tables.json` configuration reject unknown policy before creating a `table`.

- [ ] **Step 1: Add failing table validation and persistence tests**

```go
func TestTableBotPolicyDefaultsAndRejectsUnknown(t *testing.T) {
	r, _ := New(testOptions(t)); defer r.Close()
	good := TableConfig{ID: "good", Seats: 2, Decks: []string{"a"}, Spectator: view.Public}
	if err := r.AddTable(good); err != nil { t.Fatal(err) }
	if got := r.Tables()[0].BotPolicy; got != BotPolicy { t.Fatalf("default = %q", got) }
	bad := good; bad.ID, bad.BotPolicy = "bad", "legacy"
	if err := r.AddTable(bad); err == nil { t.Fatal("legacy policy accepted") }
}
```

Add a persistence test that writes a table with empty `BotPolicy`, creates a new registry from the directory, and asserts both the restored table metadata and `tables.json` carry `bot`. Add a hand-authored `tables.json` record with `bot_policy:"random"` and assert `host.New` fails before any table starts.

- [ ] **Step 2: Run the focused table tests to verify failure**

Run: `go test ./host -run 'TestTableBotPolicyDefaultsAndRejectsUnknown|Test.*BotPolicy.*Persist|Test.*BotPolicy.*Restore' -count=1`

Expected: compile failure for `TableConfig.BotPolicy` and absent metadata field, then behavioral failures until normalization is stored.

- [ ] **Step 3: Normalize at the configuration boundary**

Add the field:

```go
BotPolicy string `json:"bot_policy,omitempty"`
```

Change validation to return the normalized config, rather than only an error:

```go
func (c TableConfig) validated(load func(string) (Deck, error)) (TableConfig, error) {
	policy, err := NormalizeBotPolicy(c.BotPolicy)
	if err != nil { return TableConfig{}, fmt.Errorf("host: table %s: %w", c.ID, err) }
	c.BotPolicy = policy
	// retain every existing validation branch, returning c on success.
	return c, nil
}
```

Use it in `Registry.AddTable` before taking the registry lock and in
`Registry.load` before installing restored table records. Do not leave a
value-receiver `validate` call that discards the normalized value. Preserve
the existing deck/human/format validation order and errors.

- [ ] **Step 4: Add public table metadata**

Add `BotPolicy string \`json:"bot_policy"\`` to `protocol.TableInfo`; set it from normalized `t.cfg.BotPolicy` in `table.info`. Update protocol fixtures that compare full `TableInfo` values to expect `bot`.

- [ ] **Step 5: Run host persistence and protocol tests**

Run: `go test ./host ./protocol -run 'TestTableBotPolicy|TestConfigurationIsValidated|Test.*Persist|Test.*Restore|TestProtocol' -count=1`

Expected: PASS; old config is normalized, and invalid policy never reaches a table.

- [ ] **Step 6: Commit durable configuration validation**

```bash
git add host/table.go host/registry.go host/restart.go host/host_test.go host/persist_test.go protocol/protocol.go protocol/protocol_test.go
git commit -m "feat(host): persist and validate bot policy"
```

### Task 3: Use the selected factory for all host controllers and match metadata

**Files:**
- Modify: `host/match.go`
- Modify: `host/persist.go`
- Modify: `host/fanout.go`
- Modify: `host/host_test.go`
- Modify: `host/caretaker_test.go`
- Modify: `host/human_match_test.go`
- Modify: `protocol/protocol.go`
- Modify: `protocol/protocol_test.go`
- Run generator: `make gentypes`
- Modify generated: `web/src/protocol.ts`

**Interfaces:**
- Consumes normalized `TableConfig.BotPolicy` and `NewBotPolicySeat` from Tasks 1–2.
- Produces `BotPolicy string \`json:"bot_policy"\`` on `protocol.MatchInfo` and `protocol.MatchStart`.
- Every host-created non-human controller and every `HumanSeat` caretaker uses `NewBotPolicySeat(m.table.cfg.BotPolicy, m.seed^uint64(i+1))`.

- [ ] **Step 1: Write controller, metadata, determinism, and replay tests**

Add a two-seat host helper that runs once with a supplied policy. For each of `bot` and `lethal-pressure`, run it twice with the same `TableConfig`; assert exact equality of event slices, `Head`, `Result`, `Winner`, and `replay.Replay` output. In a separate human-seat test, install a zero/short timeout, configure `lethal-pressure`, and use a recording `seat.BoardSeat` wrapper or caretaker counter to prove the caretaker is constructed with the experimental factory rather than `seat.NewBot`.

Assert public metadata at every source:

```go
if got := r.Tables()[0].BotPolicy; got != LethalPressurePolicy { t.Fatal(got) }
if got := matches[0].BotPolicy; got != LethalPressurePolicy { t.Fatal(got) }
// Decode MatchStart from a subscribed session and require the same value.
```

Add an explicit default-policy regression that compares a no-field table against an explicit `BotPolicy: BotPolicy`; their logs and heads must be equal. Keep the existing `rules.TestHeads` test as the production golden proof.

- [ ] **Step 2: Run the tests to verify failure**

Run: `go test ./host -run 'Test.*BotPolicy.*(Controller|Caretaker|Replay|Metadata|Default)' -count=1`

Expected: failure because `match.play` and human caretaker construction still call `seat.NewBot` and match metadata has no policy field.

- [ ] **Step 3: Route controller construction through the factory**

Replace `defaultSeats` with `defaultSeats(policy string, names []string, seed uint64) []seat.Seat`; it constructs each slot through `NewBotPolicySeat` in seat-index order and panics only if the already-validated name cannot resolve. Change `Options.Seats` to `func(policy string, names []string, seed uint64) []seat.Seat`, document it as an explicit embedder controller override that receives the normalized name, and update every test helper/call site to accept and forward `policy`. `New` installs `defaultSeats`. This removes the existing production signature that cannot observe `TableConfig.BotPolicy`; a custom embedder that deliberately supplies a different controller remains an explicit override, not an implicit fallback.

In `play`, use the resolved policy for all non-human slots. Replace:

```go
hs.configure(r.opts.ThinkTimeout, seat.NewBot(m.seed^uint64(i+1)))
```

with factory construction using `m.table.cfg.BotPolicy` and the same per-seat seed. Propagate a factory error to the ordinary match crash/halt path; it should be unreachable after Task 2 validation but must not become an implicit fallback.

- [ ] **Step 4: Thread metadata through every match shape**

Set `BotPolicy` in `match.info`, `sidecar.info`, and the live `MatchStart` fanout. Add it to the persisted `sidecar` only if archived match listing cannot otherwise recover the table policy; preserve enough data to make a match list and sidecar restart report the effective policy. Do not add it to `rules.Config`, `events.Event`, the hash chain, or a view.

- [ ] **Step 5: Regenerate and check TypeScript protocol types**

Run: `make gentypes && go run ./cmd/gentypes -check`

Expected: `web/src/protocol.ts` declares the additive `bot_policy` fields for `TableInfo`, `MatchStart`, and `MatchInfo`; generation check passes.

- [ ] **Step 6: Run host, protocol, and adapter verification**

Run: `go test ./host ./protocol ./seat -count=1`

Expected: PASS; each named hosted policy terminates/replays deterministically and default and explicit `bot` have identical output.

- [ ] **Step 7: Commit host controller wiring**

```bash
git add host/match.go host/persist.go host/fanout.go host/host_test.go host/caretaker_test.go host/human_match_test.go protocol/protocol.go protocol/protocol_test.go web/src/protocol.ts
git commit -m "feat(host): construct controllers from selected bot policy"
```

### Task 4: Expose deliberate policy selection in play-vs-bot and keep legacy diagnostic-only

**Files:**
- Modify: `host/httpapi/rest.go`
- Modify: `host/httpapi/game_test.go`
- Modify: `cmd/gorged/game.go`
- Modify: `cmd/gorged/game_test.go`
- Modify: `cmd/botbench/main.go`
- Modify: `cmd/botbench/main_test.go`

**Interfaces:**
- Consumes `host.NormalizeBotPolicy` and `host.NewBotPolicySeat` from Task 1.
- Adds `BotPolicy string` to `httpapi.CreateGameRequest` and `httpapi.CreateGameOptions`.
- `CreateGameResponse` returns effective `BotPolicy string \`json:"bot_policy"\``.
- Botbench uses the shared factory for `bot` and `lethal-pressure`; its `legacySeat` remains available only through an explicitly named diagnostic branch.

- [ ] **Step 1: Write failing HTTP parsing and gorged persistence tests**

```go
func TestCreateGameBotPolicyDecodeAndRejectsDiagnostic(t *testing.T) {
	r, _ := host.New(host.Options{LoadDeck: loader(t), Sleep: func(time.Duration, <-chan struct{}) {}})
	defer r.Close()
	var got []CreateGameOptions
	srv := httptest.NewServer(NewHandler(r, Options{CreateGame: func(o CreateGameOptions) (CreateGameResponse, error) {
		got = append(got, o)
		return CreateGameResponse{Table: "g1", BotPolicy: o.BotPolicy}, nil
	}}))
	defer srv.Close()
	for _, body := range []string{`{}`, `{"bot_policy":"lethal-pressure"}`} {
		status, _, _ := postGames(t, srv.URL, body)
		if status != http.StatusOK { t.Fatalf("%s returned %d", body, status) }
	}
	if got[0].BotPolicy != host.BotPolicy || got[1].BotPolicy != host.LethalPressurePolicy { t.Fatalf("options = %+v", got) }
	for _, body := range []string{`{"bot_policy":"legacy"}`, `{"bot_policy":"random"}`} {
		status, e, _ := postGames(t, srv.URL, body)
		if status != http.StatusBadRequest || e.Code != "bad_request" { t.Fatalf("%s: %d %+v", body, status, e) }
	}
	if len(got) != 2 { t.Fatalf("invalid policy reached builder: %+v", got) }
}

func TestCreateGamePersistsRequestedBotPolicy(t *testing.T) {
	r, gate := freshGameLock(t)
	create := (config{mulligans: 0}).createGame(r, gate, []string{"a", "b"}, []string{"c", "d"}, view.Omniscient)
	resp, err := create(httpapi.CreateGameOptions{Format: host.FormatConstructed, BotPolicy: host.LethalPressurePolicy})
	if err != nil { t.Fatal(err) }
	if resp.BotPolicy != host.LethalPressurePolicy { t.Fatal(resp.BotPolicy) }
	if got := r.Tables()[0].BotPolicy; got != host.LethalPressurePolicy { t.Fatal(got) }
	if ms, _ := r.Matches(host.TableID(resp.Table)); len(ms) != 1 || ms[0].BotPolicy != host.LethalPressurePolicy { t.Fatalf("matches = %+v", ms) }
}
```

Add a botbench test that `resolvePolicy("bot")` and
`resolvePolicy("lethal-pressure")` produce the shared host constructors,
while `resolvePolicy("legacy")` still returns the benchmark-only adapter and
no hosted resolver accepts it.

- [ ] **Step 2: Run focused failures**

Run: `go test ./host/httpapi ./cmd/gorged ./cmd/botbench -run 'TestCreateGame.*BotPolicy|Test.*Legacy.*Policy' -count=1`

Expected: compile failures for the new request/response fields and behavior failures for accepted legacy.

- [ ] **Step 3: Parse, normalize, and report the API selection**

Add `BotPolicy string \`json:"bot_policy,omitempty"\`` to the request. In
`games`, call `host.NormalizeBotPolicy(req.BotPolicy)` before invoking the
builder; return its error with existing `bad_request` semantics. Pass the
normalized result to `CreateGameOptions`. Add the effective name to the
success response. The builder in `cmd/gorged/game.go` sets
`TableConfig.BotPolicy: req.BotPolicy`; it must not reimplement validation or
fall back on error.

- [ ] **Step 4: Share production factory with botbench without exposing legacy**

Replace botbench's `bot` and `lethal-pressure` map constructors with wrappers
around `host.NewBotPolicySeat`. Keep `legacySeat` and its map entry local to
`cmd/botbench`; preserve its existing random stream and error/report behavior.
Make `resolvePolicy` list its names in sorted order as it does today. Do not
import botbench from host or seat.

- [ ] **Step 5: Run endpoint and botbench unit verification**

Run: `go test ./host/httpapi ./cmd/gorged ./cmd/botbench -count=1 -skip 'TestFullPairsIteratesSorted|TestConstructedDefaultIsByteIdentical'`

Expected: PASS; omission selects `bot`, experimental selection is visible,
and `legacy` is 400 from hosted API but remains a botbench diagnostic.

- [ ] **Step 6: Commit the user-facing policy contract**

```bash
git add host/httpapi/rest.go host/httpapi/game_test.go cmd/gorged/game.go cmd/gorged/game_test.go cmd/botbench/main.go cmd/botbench/main_test.go
git commit -m "feat(gorged): select named hosted bot policy"
```

### Task 5: Integration evidence, golden preservation, and handoff report

**Files:**
- Create: `docs/superpowers/reports/2026-09-19-hosted-bot-policy-selection.md`
- Modify only files required by failures discovered in the commands below.

**Interfaces:**
- Consumes all previous tasks.
- Produces a concise evidence report with exact commands/results, selected hosted behavior, policy-strength non-claim, retained/rejected alternatives, risks, and AR8 as next experiment.

- [ ] **Step 1: Run focused complete tests and deterministic replay proof**

Run:

```bash
go test ./botpolicy ./seat ./host ./host/httpapi ./cmd/gorged -count=1
go test ./cmd/botbench -count=1 -skip 'TestFullPairsIteratesSorted|TestConstructedDefaultIsByteIdentical'
```

Expected: PASS. Record any skipped known long leaves exactly as skipped, not as a full benchmark result.

- [ ] **Step 2: Verify production heads and simulator without changing goldens**

Run:

```bash
go test ./rules -run TestHeads -count=1
make sim
```

Expected: PASS and the existing heads remain unchanged. If a head differs,
stop and diagnose rather than editing `rules/heads_test.go`.

- [ ] **Step 3: Run static/wire/diff checks**

Run:

```bash
go vet ./...
go run ./cmd/gentypes -check
git diff --check
git status --short
```

Expected: clean verification output; status contains only intentional code,
generated type, and report changes before the final commit.

- [ ] **Step 4: Compare any broad-suite failures with known baseline**

If a broad command fails outside the focused packages, capture the failing
package/test and compare it against the documented stale deck-pool, `host`,
`internal/archtest`, and `internal/searchprobe` baseline before classifying
it as a regression. Do not mask or edit unrelated tests.

- [ ] **Step 5: Write the evidence report**

Include the committed implementation hashes, exact commands and pass/fail
output summaries, supported hosted names and default, rejected names,
metadata locations, deterministic replay evidence, unchanged golden heads,
explicit statement that no new strength claim or botbench matrix was run, the
retained host factory and rejected implicit/legacy alternatives, any residual
embedder seam risk, and AR8 combined-attacker pressure as the next experiment.

- [ ] **Step 6: Commit verified evidence**

```bash
git add docs/superpowers/reports/2026-09-19-hosted-bot-policy-selection.md
git commit -m "docs: report hosted bot policy verification"
```
