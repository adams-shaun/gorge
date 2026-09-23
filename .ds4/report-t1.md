# Report — `K:Companion` registration

## Changes

- `rules/trigger_match.go`: registered `kw:Companion` in `effects.RegisterNonAPI`, documenting the Partner deck-construction precedent and CR 702.139. The comment explicitly scopes out the pregame pick and activation.
- `rules/companion_702139_registration_test.go`: added corpus-wide registration/census coverage and an exact parsed Jegantha keyword pin. The first test verifies registration, finds carriers by keyword head, pins the population to 8–12, confirms none still report `kw:Companion` unsupported, and requires at least one fully-supported carrier (logging other gaps). The second asserts Jegantha and the exact keyword are present before checking `Primitives()` and unsupported status.

Corpus measurement: `.cards` was present as a symlink to `/home/sadams/projects/gorge/.cards`. `grep -rl '^K:Companion' .cards/cardsfolder --include='*.txt' | wc -l` returned `10`; counting matching lines returned `10` as well.

## Fails without the fix

Saved the modified registration file, removed only its `kw:Companion` entry, ran the targeted tests, and restored the file byte-identically (`cmp` succeeded). Real output:

```text
without-fix exit=1 restore-identical=0
--- FAIL: TestCompanionPrimitiveIsRegistered (0.00s)
    companion_702139_registration_test.go:17: effects.Supported() is missing "kw:Companion"
--- FAIL: TestCompanionCarrierIsUnderstood (0.59s)
    companion_702139_registration_test.go:68: Jegantha still names kw:Companion as unsupported: [kw:Companion]
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.619s
FAIL
```

## Gates

- `go test -run 'TestCompanionPrimitiveIsRegistered|TestCompanionCarrierIsUnderstood' ./rules/`
  ```text
  exit=0
  ok   github.com/adams-shaun/gorge/rules  0.677s
  ```
- `go test ./internal/archtest/`
  ```text
  archtest exit=0
  ok   github.com/adams-shaun/gorge/internal/archtest  3.884s
  ```
- `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/`
  ```text
  botbench exit=0
  ok   github.com/adams-shaun/gorge/cmd/botbench  1.263s
  ```
- `gofmt -l rules/trigger_match.go rules/companion_702139_registration_test.go`: no output.
- `go run ./cmd/gentypes -check`
  ```text
  gentypes exit=0
  ```
- `git diff --check`: no output.

No deck acceptance, head, or botbench pin was changed; the byte-identical botbench golden passed.

## Issues

Issue `agent-20260918T234402Z-c77011ce` is closed for keyword registration. The companion mechanic remains unimplemented: there is no pregame chosen-companion selection or `{3}` activation to put the card into its owner's hand from outside the game. Follow-up work should implement deck validation for the ten restriction forms observed in the corpus (`Card.cmcGE3,Land`; `Card.cmcM20`; `Card.cmcM21,Land`; `Creature.Cat,Creature.Elemental,Creature.Nightmare,Creature.Dinosaur,Creature.Beast,Card.nonCreature`; `Permanent.cmcLE2,Instant,Sorcery`; `Permanent.hasAbility Activated,Instant,Sorcery`; `Special:DeckSizePlus20`; `Special:SharesCardType`; `Special:UniqueManaSymbols`; `Special:UniqueNames`) and separately address play-side selection/activation. The corpus has 10 files with `K:Companion` lines. No other defects were found in scope.

## Commit

`f7e41c45 feat(rules): register Companion keyword`
