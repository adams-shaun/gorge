# gorge's external contracts

A **contract** is anything a program outside this module reads or writes: a
file layout, a wire, a command grammar. DraftZero's trainer (Python, driving
`cmd/dzgorge` as a subprocess) is the first such consumer, and its shards,
networks and configs outlive any one gorge commit. So each contract has a name
and a major version, golden files whose SHA-256 the registry pins, and readers
that refuse a version they cannot read.

The registry is the package [`contracts`](../contracts/contracts.go):
`contracts/registry.json` lists every contract (name, major, description,
owner package, goldens with their hashes), and Go constants mirror each name
and major (`contracts.DZGPack`, `contracts.DZGPackVersion`, ...).
`dzgorge version` prints every contract's major. Command-line conventions
(flags, environment variables, config files) are in [cli.md](cli.md).

## Change rules

| Change | What to do |
|---|---|
| **Additive**: a new field a reader may ignore, a new entry appended to an append-only table (`contracts.PackKinds`) | Keep the major. Regenerate the golden (the owner test's `-update` / `-update-contracts` flag), put its new hash in `contracts/registry.json`, and add a CHANGELOG line naming the contract. |
| **Anything else**: a field renamed, removed, retyped or given a new meaning; a table reordered; a default changed | A new major: bump the constant in `contracts/contracts.go` and the entry in `registry.json`, write the new format tag, and keep the reader (or a `-format` switch on the writer) accepting the previous major for one release. Add a CHANGELOG line. |
| **Reading** | A reader that meets a major it does not know refuses it by name. It never guesses. |

The tests that hold this:

- `contracts.TestGoldensMatchRegistry` hashes every listed golden. When one
  moves, the failure says which contract and tells the author the rule above.
- Each owner package has a test that reproduces its golden byte for byte from
  a synthetic input, so a code change that changes the output fails in the
  owner package first.
- `contracts.TestConstantsMatchRegistry` holds the Go constants, and the
  version constants owner packages already declare (`protocol.Version`,
  `wire.Version`, `policynet.VisitSchemaVersion`), to the registry.

Goldens use stand-in values wherever a live value would move with an
unrelated change. For example, the encoder hash in the pack and visit goldens
is `0123456789abcdef`, and the gorge build is `v0.0.0-golden`.

## The contracts

### dzg-games, v1

- **Producer:** `dzgorge play -out games.jsonl` (`cmd/dzgorge/play.go`, `gameRec`),
  plus `games.jsonl.summary.json`.
- **Consumers:** DraftZero's evaluation and paired scoring.
- **Layout:** one JSON object per game, in game order. Every record and the
  summary carry `format: "dzg-games-1"` and `gorge: {version, revision}` (the
  module version and VCS revision from the binary's build info; `(devel)` and
  an empty revision for a `go run` or test build). The record fields are
  `game, pair, leg, seed, decks, side, pols, winner, result, stall, turns,
  intents, starter, ms, in_hand, mull, mm, visits, error, lines`.
- **Golden:** `cmd/dzgorge/testdata/contracts/games.jsonl`
  (`TestContractGamesRecord`).
- **Known gaps:** the summary's keys are not pinned by a golden; only its
  `format` and `gorge` fields are part of the contract today.

### dzg-pack, v2

- **Producer:** `dzgorge pack -in corpus.jsonl.gz[,...] -out DIR`
  (`cmd/dzgorge/pack.go`).
- **Consumers:** DraftZero's shard reader (`gorge/dzg`), and shards published
  on disk and on Hugging Face.
- **Layout:** `DIR/00000/, 00001/, ...`, each holding version 1.0 `.npy`
  arrays (the state, option, candidate and per-record columns that
  `shardWriter.flush` lists) and a `meta.json`:
  - `format`: `"dzg-pack-2"`
  - `n`, `games`, `source`: as before
  - `kinds`: the whole canonical kind table, `contracts.PackKinds`. The
    `kind.npy` column is an index into it. The table is append-only: an index
    never changes meaning, so a reader may hard-code it. It is pinned by
    `contracts/testdata/pack_kinds.txt` (`TestPackKindsAppendOnly`).
  - `encoder_hash`, `record_type`, `schema_version`: the input corpus's
    identity, copied from its records.
  - `gorge`: `{version, revision}` of the build that packed it.
- **Input checks:** every record of every input must have
  `record_type: "azvisits-v1"` and `schema_version: 1`, and all must share one
  `encoder_hash`. A mismatch is an error naming both values and the
  `file:line` of each. A record that does not decode is an error at its
  `file:line`. A truncated gzip tail (a run killed mid-write) is an error
  unless `-allow-truncated` is given.
- **v1, deprecated:** `pack -format 1` writes the dzg-pack-1 layout byte for
  byte. Its `kinds` are indexed in first-seen order per pack run, and its
  meta has no identity fields. It will be removed one release after v2.
  Golden: `cmd/dzgorge/testdata/contracts/pack1/`, frozen, written by the
  pre-contract pack.
- **Goldens:** `cmd/dzgorge/testdata/contracts/pack2/SHA256SUMS` and
  `pack1/SHA256SUMS` (manifests over the golden shard files),
  `contracts/testdata/pack_kinds.txt` (`TestContractPack2`,
  `TestContractPack1`).

### azvisits, v1

- **Producer:** the az seat's recorder (`internal/azmcts/record.go`), written
  by `dzgorge play -corpus` and `botbench -az-corpus`.
- **Consumers:** `dzgorge pack`, `policytrain -visits-corpus`
  (`policynet.LoadVisits`), DraftZero.
- **Layout:** gzip members of JSON lines, one `policynet.VisitRecord` per
  searched decision: `record_type: "azvisits-v1"`, `schema_version: 1`, and
  `encoder_hash` set to `%016x` of `policynet.EncoderHashFor(feature set)`.
  The schema comment on `VisitRecord` (`internal/policynet/visits.go`) is the
  field reference.
- **Golden:** `internal/policynet/testdata/contracts/azvisits.jsonl`
  (`TestContractVisitRecord`).
- **Known gaps:** the entity feature set has no version number of its own.
  `encoder_hash` is its identity, and `dzgorge version` prints the current
  build's.

### dzgw, v1

- **Producer:** DraftZero's `python -m dzg.export`.
- **Consumer:** `internal/gonet` (`gonet.Load`, the `gonet=` policy key).
- **Layout:** the magic `DZGW0001`, a little-endian uint64 header length, a
  JSON header, zero padding to a 64-byte boundary, then little-endian float32
  tensors at the header's offsets. The header holds `format: "dzgw-1"`,
  `arch: "mlp"` (the only architecture with a Go forward pass), `config`,
  `consts` (the widths, which must equal this build's), `layernorm_eps` and
  `tensors`.
- **`encoder_hash`** (optional, additive): the `%016x`
  `policynet.EncoderHashFor` of the feature set the network was trained on
  (`0x` prefix accepted). When present it must equal this build's entity
  encoder hash, or `Load` refuses the file, as `.gpol` checkpoints already
  refuse encoder drift. When absent the file loads, `Net.EncoderHashChecked()`
  reports false, and dzgorge prints one stderr warning per path. The exporter
  should write it. That change belongs on the DraftZero side.
- **Golden:** `internal/gonet/testdata/contracts/dzgw_header.json`
  (`TestContractDZGWHeader`; `TestLoadChecksEncoderHash`).

### dzg-remote, v1 (GZB1 / GZR1)

- **Producer and consumer:** `dzgorge`'s `remote=` client
  (`cmd/dzgorge/remote.go`) and `python -m dzg.serve`.
- **Layout:** a request is a little-endian uint32 byte count, then
  `GZB1` (0x31425A47), B (states), the sparse, card, card-row, option,
  option-slot and option-hashed counts, then the `want` bytes and the packed
  columns (the same arrays as a dzg-pack shard), each padded to 8 bytes. A
  response is a byte count, then `GZR1` (0x31525A47), B, the option count and
  a reserved word, then B float32 values (padded to 8 bytes) and the option
  scores of the states that asked for them.
- **Goldens:** `cmd/dzgorge/testdata/contracts/remote_request.bin` and
  `remote_response.bin` (`TestContractRemoteFrames`).
- **Known gaps:** there is no handshake, so neither side checks the other's
  encoder hash or version. Adding one changes the wire, so it would be
  dzg-remote v2. Until then, run the server on a network whose encoder hash
  matches `dzgorge version`'s `entity_encoder_hash`.

### gorge-agent, v1

- **Owner:** `draftlab/extern/wire` (`Protocol = "gorge-agent/1"`, `Version = 1`).
- **Consumers:** out-of-process drafters, builders and players
  (`draftlab/extern`).
- **Layout:** JSON lines; the hello exchange names the protocol and a mismatch
  ends the session.
- **Goldens:** every file in `draftlab/extern/wire/testdata/`
  (`TestMessageGoldens`).

### gorge-protocol, v1

- **Owner:** `protocol` (`protocol.Version`), the host-client frame envelope.
- **Consumers:** the web client (TypeScript via `make gentypes`) and any
  external client.
- **Goldens:** every file in `protocol/testdata/` (`TestGoldens`).

### dzgorge-spec, v1

- **Owner:** `cmd/dzgorge/policy.go` (`parsePolicy`).
- **Consumers:** every DraftZero config and script
  (`src/draftzero/engine/specs.py`).
- **Grammar:** `kind[:key=value|:flag]...`, where kind is `random`, `bot`,
  `sbtactical`, `az` or `prior`. `remote=` takes the rest of the spec. An
  unknown key is an error. A flag key (`explore`, `nonoise`, `autopay`,
  `mull`, `keep7`, `oppnodes`, `oppfull`, `closedlist`, `morekinds`) given a
  value is an error: before this contract `autopay=0` silently turned autopay
  on. `skippass` takes an optional value.
- **Object form:** in a config file (or as a `-a`/`-b` value starting with
  `{`), a spec may be a JSON object with the same keys:
  `{"kind": "az", "sims": 200, "autopay": true, "topk": 6, "gonet": "model.dzgw"}`.
  Flag keys are bools (`false` means absent), `kinds` is an array of names,
  `skippass` is `true` or a number, and the rest are numbers or strings. An
  unknown key or a wrong type is an error. `mull` with `keep7`, or
  `morekinds` with `kinds`, is an error, where the string form lets the last
  one win. A path may contain `:`, which the string form cannot express. Both
  forms set each key through the same code (`policySpec.setKey`). The
  recorded spec (`pols` in dzg-games) of an object is its canonical string:
  the kind, then the keys in `policyKeys` order.
- **Golden:** `cmd/dzgorge/testdata/contracts/spec_grammar.jsonl`, each
  accepted spec's parsed struct and the refused spellings
  (`TestContractSpecGrammar`, `TestFlagKeysRefuseValue`). Every accepted spec
  round-trips through the object form to the identical struct
  (`TestPolicyObjectRoundTrip`).

### dzgorge-version, v1

- **Owner:** `dzgorge version` (`cmd/dzgorge/version.go`).
- **Consumers:** DraftZero's build record (`bin/dzgorge.build.json`).
- **Layout:** one JSON object with these fields:
  - `format: "dzgorge-version-1"`
  - `module {path, version}` and `vcs {revision, modified}`, from the build info
  - `go`
  - `contracts`, mapping each name to its major
  - `entity_encoder_hash`
  - `cards_compiler_fingerprint` and `ir_cache_version`
  - `cards_lock {repo, commit, digest}`, read from `-cards` (default
    `.cards`), or `null` when there is no `cards.lock`
- **Golden:** the key set, `cmd/dzgorge/testdata/contracts/version_keys.txt`
  (`TestContractVersionKeys`). Values vary by build.

### dzgorge-config, v1

- **Owner:** `cmd/dzgorge` (`rootCommand`), built on package `cli`
  ([cli.md](cli.md)).
- **Consumers:** DraftZero run configs, and anyone who saves a
  `--print-config`.
- **Layout:** one JSON object. `"$schema": "dzgorge-config-1"` is optional on
  input, always written by `--print-config`, and any other value is refused.
  Each subcommand has a section (`"play"`, `"pack"`, `"coverage"`,
  `"version"`, `"gonet-check"`) whose keys are that subcommand's flag names,
  with JSON values typed per flag: string, number, bool, or a duration string
  for `remote-wait`. `play`'s `a` and `b` take a dzgorge-spec string or its
  object form. dzgorge's root command has no flags of its own, so there are
  no top-level keys besides `$schema`. Unknown keys, wrong types and
  duplicate keys are errors naming the file and the dotted key, in every
  section, not only the invoked one.
- **Precedence:** explicit flag > `GORGE_DZGORGE_<CMD>_<FLAG>` (upper case,
  `-` as `_`, for example `GORGE_DZGORGE_PLAY_MAX_TURNS`) > config file
  (`--config`, or `GORGE_DZGORGE_CONFIG`) > default. Old single-dash
  invocations (`-a az:sims=200:autopay -pairs 100`) parse exactly as before.
- **Versioning:** a new flag is additive (regenerate `config_print.json` and
  rehash). Renaming or removing a flag, changing its type, or changing a
  default (the printed configuration changes meaning) is dzgorge-config v2,
  and the `$schema` tag moves to `dzgorge-config-2`.
- **Goldens:** `cmd/dzgorge/testdata/contracts/config_example.json`, a
  hand-written config using both policy forms, and `config_print.json`,
  `dzgorge play --config config_example.json --print-config`. That second
  file pins every `play` flag with its type and default
  (`TestContractConfig`, which also checks the round trip).
- **Known gaps:** a reader for a previous major does not exist yet, because
  there is none. When v2 comes, the root needs a list of accepted tags.

## Not yet registered

- The belief corpus `dzgorge play` reads for `:closedlist`: the deck
  directory and the `pool.tsv` split.
- `sealed-pools` JSON lines. Its command lives only on DraftZero's fork
  today.
