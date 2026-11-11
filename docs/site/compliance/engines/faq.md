# FAQ

## Getting started

??? question "Does my engine have to be written in Go, or link against gorge?"
    No. The runner is any program that reads and writes JSON lines. The Forge and XMage drivers are Java,
    the planned phase.rs and ManaBrew followers are Rust, and the quickstart's toy is Python. Only the
    harness (gorge-compliance) is Go.

??? question "Do I need XMage or Forge installed?"
    No. With only your engine in `XENGINE_FOLLOWERS`, no JVM starts. You compare against gorge, whose
    snapshots are already in each play directory's `gorge.jsonl`. Add Forge or XMage later, if you want
    their votes too.

??? question "What's the smallest thing that counts as a follower?"
    A runner that answers `--preflight` and writes a `harness` row for every request. It is never
    wrong, it just covers nothing. The next step, placing the board and writing the `"setup"` snapshot,
    is already a real comparison.

??? question "How big does the runner get?"
    The complete Forge driver's replay code is about 4,800 lines of Java ([the breakdown](index.md#what-it-costs-you)).
    A first useful version is much smaller: setup plus snapshots plus casting from the pool.

??? question "Which cards should I start with?"
    The FDN commons pool (`oracle/xengine/pools/fdn-commons.txt`, 196 cards). Evergreen mechanics, few
    multi-face cards, and gorge supports all of it. Then all of FDN, then other sets.

## The protocol

??? question "Why do I get the board as a list of placements instead of a deck and a seed?"
    So no two engines have to agree on shuffling, mulligans or who goes first. Setup puts every card
    exactly where gorge had it, including the whole library in order, and the comparison starts from
    there. That also lets a window start in the middle of a game.

??? question "Do I need to reproduce gorge's shuffles?"
    No. A whole-game tape stops at the first random choice. A window that shuffles has library order
    excluded from the comparison, and a difference after a random draw is classed `rng`. Seed your own
    randomness from the request so your reruns are stable.

??? question "What about mulligans and the starting player?"
    They never happen. Setup places the opening hand directly, and `p0` is always the active player on
    turn 1, skipping its draw.

??? question "My engine gives priority somewhere gorge doesn't, or skips one. What happens?"
    The `pass` steps stop lining up, so your driver should stop with a harness row the moment the wrong
    seat holds priority. If it's your engine's deliberate behaviour, we add a registry quirk so the
    classifier stops calling it `diverge`, as we did for Forge skipping an empty combat damage step. If
    you think gorge is the one that's wrong, tell us: priority is in the CR, and gorge follows it.

??? question "My engine asks a question gorge never recorded an answer for."
    Report it as a `strict_miss` and stop. The item is classed `decision_shape`. Don't let a default
    answer it: a silent default turns into a false `diverge`. If one kind of prompt causes many misses,
    tell us which. gorge can start recording that choice.

??? question "My card names don't match gorge's."
    gorge uses Forge's spellings. Map your names on the way in and back on the way out. The usual
    differences are accents, split and Room cards (gorge uses the first half), multi-face cards (the
    current face's name on the battlefield) and token names (anonymised by the comparator anyway).

??? question "How do I know which activated ability a step means?"
    Use `neutral.ability[step]`: the ability's class, its position among that class on the card, its
    cost and its text. If your engine reads Forge scripts, `abilities[step]` is the exact script line.
    See [match an ability](howto.md#match-an-ability-to-gorges-index).

??? question "Why does the request rewrite some `pass` steps into `pass_to`?"
    Forge asks for attackers and blockers before the step's first priority. The rewrites let a driver
    follow either model, and step indices never move. See [Forge-shaped rewrites](protocol.md#forge-shaped-rewrites).

??? question "Can I see what gorge expected?"
    Yes. Every play directory has `gorge.jsonl` with gorge's snapshots for each item, and every finding's
    repro directory has `gorge.json` with the snapshots, decisions and a transcript of the game.

??? question "What does the request's `scenario_sha` guarantee?"
    It's a hash of the item before any rewrite. Every follower gets the same sha for the same item, so you
    can join your results with Forge's and XMage's.

## Results

??? question "Most of my rows are `harness`. Is that bad?"
    No. It means your driver says when it can't do something, which is what keeps the `diverge` list
    honest. Group the harness messages and build the most common one next.

??? question "My engine and gorge disagree. Who's right?"
    Neither, until a person reads the printed card and the Comprehensive Rules. Agreement between engine
    families is evidence, not a ruling, and so is disagreement. If gorge is wrong, the fix lands in gorge;
    if your engine is wrong, it's yours to fix.

??? question "Why is a missing card called `xmage_lacks` for my engine?"
    A leftover name from when XMage was the only follower. It means your harness message used your
    registry's `LacksCardMsg`. Refusing the card in the preflight is better: it saves the replay.

??? question "Does my engine's result count toward gorge's per-set compliance numbers?"
    Not today. The per-set claims rest on hand scenarios, XMage verdicts and mtgish. Full-game comparisons
    are leads, and nothing a sweep finds is committed or ruled automatically.

## Licences and data

??? question "Which licences are involved?"
    gorge and gorge-compliance are Apache-2.0. The card scripts gorge compiles are Forge's, GPL-3.0, so
    request files (which quote script lines) and Forge's output are never committed: the harness refuses
    to write them inside the repository except under the gitignored `.oracle-build/`. XMage is MIT, and
    phase.rs is MIT or Apache-2.0. ManaBrew is AGPL; its follower would run as a separate process that
    exchanges files with the harness, as the GPL Forge driver does.

??? question "Do I need to put my engine's code in gorge-compliance?"
    No. The registry entry names a runner script, and that script can call a binary built anywhere. The
    Forge driver lives in our Forge fork, and the phase.rs follower is planned so that it could move into
    phase's own workspace.

??? question "Can I run the comparison in my own CI, without gorge?"
    Not yet as a package. The plan is a conformance pack: a few hundred FDN-commons requests with gorge's
    snapshots for them and a small standalone compare tool, published per gorge pin. Until then, the
    quickstart's commands are the way.

## Working together

??? question "We'd like to own our engine's follower. Can we?"
    Yes, please. That's the goal. The follower talks to the harness only through this protocol, so it can
    live in your repository, and the registry entry just points at it. Open an issue on
    [gorge-compliance](https://github.com/adams-shaun/gorge-compliance/issues) and we'll help move it over.

??? question "We found a bug in gorge with this. Where do we report it?"
    On [gorge-compliance's issues](https://github.com/adams-shaun/gorge-compliance/issues), with the item
    id, the finding's `verdict.json` and the rule you're relying on. The repro directory makes it easy for
    us to replay.

??? question "You found a bug in our engine. How will we hear about it?"
    Before anything is reported, a person checks the printed card or the CR, reads the cause in your
    engine's current source, and rules out our driver with a replay. Only then does it go to your project's
    issue tracker. The confirmed list is kept in
    [upstream-bugs.md](https://github.com/adams-shaun/gorge-compliance/blob/main/docs/upstream-bugs.md).

??? question "The protocol is missing something my engine needs."
    Tell us. It has grown before, for example the `neutral` identities were added for engines that don't
    read Forge scripts. A field your engine needs is probably one the next engine needs too.
