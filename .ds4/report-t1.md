# Report — api:Attach Optional$ / Yuffie object-choice attach

## Changes

- `effects/attach.go`: Generalized destination validation to accept the actual object being attached, rather than always using the resolving source. For a `Choices$` object-side pool, each candidate is now checked against its possible destinations; the offered candidates are restricted to attachable objects and the destination list saved over suspension is the intersection valid for every offered object. This fixes Yuffie's real `Choices$ Equipment.YouCtrl | Defined$ Self` ETB: formerly the resolver treated Yuffie as the attached object, filtered Yuffie itself as an illegal self-destination, and emitted `cannot attach: no legal target` without posing the optional election. The existing Optional$ ask/decline flow had already landed in commit `744f665c`; this change corrects the Choices$ candidate/destination interaction surfaced by this card.
- `rules/yuffie_attach_optional_test.go`: Added a real-corpus Yuffie ETB integration test. It confirms the Equipment is offered in a Min-0/Max-1 choice, the preceding gain-control rider resolves, and declining produces no Attach event and leaves the Equipment unattached. The test also replay-checks the resulting event stream.

The implementation is role-based rather than Yuffie-specific: other object-side Choices$ Attach effects receive destination validation against their offered attaching objects too.

## Workspace and controller directive

- `.cards` existed in this worktree before testing.
- The worktree started clean. Main advanced after the initial inspection: current `HEAD` is `db5952897d663ab39f3d0ee0b560cbc6700634e1`, while current `main` is `9b8072c6966fe7839a7ae7719a92763c97865c6c`.
- I did not run `git rebase main`: the repo worktree instructions explicitly prohibit running `git rebase`. This work is committed on its task branch for the controller's integration/rebase handling.

## Fails without the fix

Saved `effects/attach.go` to `.ds4/scratch/attach.go.fixed`, then temporarily changed object-side candidate validation back to validate against the resolving source. Ran the new test, restored the source file, and confirmed byte identity with `cmp` (`restore_cmp=0`). The negative run failed as intended:

```text
--- FAIL: TestYuffieMayDeclineHerETBAttach (0.63s)
    yuffie_attach_optional_test.go:91: Yuffie's ETB never posed its Optional$ attach choice; events=[...]
FAIL
```

The recorded events included `cannot attach: no legal target` after the gain-control rider, confirming the setup reached the actual Yuffie trigger and failed specifically at attachment destination validation.

## Gates run

```text
go test -run '^(TestYuffieMayDeclineHerETBAttach|TestEveryRepoDeckParamsAreRead|TestAjanisChosenMayAttachAskPosesAndYesAttachesTheAura|TestAjanisChosenMayAttachDeclineLeavesTheAuraAndRunsTheChain|TestCoriSteelCutterOptionalAttachAttachesTheEquipment)$' ./rules/
ok   github.com/adams-shaun/gorge/rules  1.101s
```

This includes the parameter census gate (`TestEveryRepoDeckParamsAreRead`) and the Yuffie, Ajani's Chosen and Cori-Steel Cutter attach coverage.

```text
go test ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest  3.553s

go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench  1.339s

gofmt -l effects/attach.go rules/yuffie_attach_optional_test.go
[no output]
go run ./cmd/gentypes -check
[no output]
git diff --check
[no output]
```

No chain-head or botbench golden movement was observed; `TestHeads` was not run because it is a daemon-only gate outside the task's named test.

## Issues

No separate unfixed issue was found. The earlier attached optional-choice implementation was already present in this branch; the Yuffie object-side Choices$ destination mismatch was fixed here.
