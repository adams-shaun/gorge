# Payment in other engines

For context only. This page does not describe how other engines work inside.

=== "Forge"

    gorge's comparison harness plays Forge and gorge through one scenario file. Its notes say Forge asks neither the pip-by-pip payment choice nor gorge's mana window, so the driver does not count those picks as leftover. A step can carry the exact pool mana gorge spent, and an empty entry means auto-pay ([`cmd/oraclediff/forge.go:60`](https://github.com/adams-shaun/gorge-compliance/blob/main/cmd/oraclediff/forge.go#L60), [`:76`](https://github.com/adams-shaun/gorge-compliance/blob/main/cmd/oraclediff/forge.go#L76)).

    That is what the harness shows. It is not a description of Forge's own payment interface. This is a summary from a public repository, and no Forge script text is quoted.

=== "XMage"

    The XMage side of the harness replays gorge's logged spends. It turns the test player's automatic payment off for the step, then spends one unit of the logged colour per unit of cost, choosing the colour the unpaid cost still needs. It restores the pool's mode afterwards ([`oracle/xmageoracle/src/org/mage/test/oracle/ScenarioReplay.java:317`](https://github.com/adams-shaun/gorge-compliance/blob/main/oracle/xmageoracle/src/org/mage/test/oracle/ScenarioReplay.java#L317)).

    So XMage has both an automatic and a manual payment mode on its mana pool. The page says nothing more about XMage's interface.

=== "MTG Arena"

    General knowledge of the public client, not a claim about its code. Arena pays for most spells automatically and shows the lands it will tap, with a way to pick the mana by hand when a choice matters. It asks about hybrid or Phyrexian symbols, X and optional costs as announcements before payment, and lets a player cancel a cast.

## What this shows

gorge sits between the two shapes. It offers the one-click engine payment, and the announce-then-pay window with Auto-fill, undo and cancel. The design that led to that window is in [`docs/design/2026-09-27-announce-then-pay.md`](https://github.com/adams-shaun/gorge/blob/main/docs/design/2026-09-27-announce-then-pay.md).

Back to [paying for spells](payment.md).
