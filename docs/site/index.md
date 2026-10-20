# gorge

**gorge** is a deterministic, event-sourced *Magic: The Gathering* rules
engine written in pure Go. It does not hand-code cards: it compiles
[Forge](https://github.com/Card-Forge/forge)'s card scripts into an
intermediate representation (IR), and the engine implements the primitives
that IR references (effects, triggers, keywords, static abilities, value
expressions). Around the engine sit a reference server with a browser client,
baseline and hosted bots, and tools to replay, benchmark and audit games.

Every state change goes through one function, `events.Apply`, into a
SHA-256 hash-chained event log. A game is therefore a pure function of its
configuration and the players' answers, so replay, resume after a restart,
undo, search clones and bug reproduction are all one mechanism.

!!! info "Status"
    gorge plays full two- to eight-player games, including Commander, serves
    bot tables and play-against-a-bot games to a browser, and replays every
    game byte for byte. It is **not** yet at rules parity with mature engines
    and is not ready for production use.

## Who this site is for

| You are… | You want… | Start at |
|---|---|---|
| a **Go developer** who wants to fix a card or a rule | to set up, find the code, and land a change | [Contributing](contributing/index.md) |
| an **engine user** running the demo or bot matches | to run a server, play, watch, replay | [User's guide](guide/index.md) |
| a **bot author** (Python, Java or Go) | to wire a strategy to the engine's decisions | [Bots](bots/index.md) |
| curious how it works | the design and its trade-offs | [Architecture](architecture/index.md) |

## At a glance

**94.2%** of Forge's 34,074 card scripts are playable at the pinned corpus
commit (`make report`, 2026-10-20, `ad3777023`). The dated per-set and
compliance figures are on [Card support](support/sets.md), and engine speed
on [Performance](performance/index.md). Golden bot games at 2, 4, 6 and 8
seats replay byte-identically, and the root `go.mod` has no dependencies and
no cgo.

## Three repositories at a glance

<figure>
<svg viewBox="0 0 780 330" width="100%" role="img" aria-label="The three repositories. gorge, public, holds the engine, server, bots and tools. gorge-compliance, public, checks gorge against XMage and Forge. four-yay, private, holds the maintainers' research and extensions. Both companions import gorge at a pinned commit; gorge imports neither.">
<defs><marker id="hm-arr" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="8" markerHeight="8" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" class="dg-arrow-accent"/></marker></defs>
<rect x="270" y="10" width="240" height="250" rx="12" class="dg-box-accent"/>
<rect x="270" y="10" width="240" height="40" rx="12" class="dg-box-solid"/>
<text x="390" y="36" text-anchor="middle" class="dg-text-on">gorge</text>
<text x="390" y="70" text-anchor="middle" class="dg-text-muted">public · Apache-2.0 · this site</text>
<text x="288" y="98" class="dg-text">rules engine (engine/)</text>
<text x="288" y="120" class="dg-text">Forge-script compiler</text>
<text x="288" y="142" class="dg-text">reference server gorged</text>
<text x="288" y="164" class="dg-text">browser client (web/)</text>
<text x="288" y="186" class="dg-text">baseline and hosted bots</text>
<text x="288" y="208" class="dg-text">replay, bench, audit tools</text>
<text x="390" y="242" text-anchor="middle" class="dg-text-muted">imports nothing but the stdlib</text>
<rect x="10" y="40" width="210" height="190" rx="12" class="dg-box"/>
<text x="115" y="68" text-anchor="middle" class="dg-text-bold">gorge-compliance</text>
<text x="115" y="88" text-anchor="middle" class="dg-text-muted">public · Apache-2.0</text>
<text x="26" y="116" class="dg-text">checks gorge, card by</text>
<text x="26" y="136" class="dg-text">card, against XMage,</text>
<text x="26" y="156" class="dg-text">Forge and hand-written</text>
<text x="26" y="176" class="dg-text">Oracle-text scenarios</text>
<text x="26" y="206" class="dg-text-muted">certification ratchet</text>
<rect x="560" y="40" width="210" height="190" rx="12" class="dg-box-muted"/>
<text x="665" y="68" text-anchor="middle" class="dg-text-bold">four-yay</text>
<text x="665" y="88" text-anchor="middle" class="dg-text-muted">private</text>
<text x="576" y="116" class="dg-text">research and draft</text>
<text x="576" y="136" class="dg-text">events, research bots,</text>
<text x="576" y="156" class="dg-text">the dirty-dirac kernel,</text>
<text x="576" y="176" class="dg-text">demo deployment</text>
<text x="576" y="206" class="dg-text-muted">not needed to use gorge</text>
<line x1="220" y1="135" x2="266" y2="135" class="dg-line-accent" marker-end="url(#hm-arr)"/>
<line x1="560" y1="135" x2="514" y2="135" class="dg-line-accent" marker-end="url(#hm-arr)"/>
<text x="115" y="262" text-anchor="middle" class="dg-text-muted">pins gorge by commit</text>
<text x="115" y="280" text-anchor="middle" class="dg-text-mono">GORGE_PIN + go.mod</text>
<text x="665" y="262" text-anchor="middle" class="dg-text-muted">pins gorge by commit</text>
<text x="665" y="280" text-anchor="middle" class="dg-text-mono">GORGE_PIN + go.mod</text>
<text x="390" y="300" text-anchor="middle" class="dg-text">arrows: importer → imported</text>
<text x="390" y="318" text-anchor="middle" class="dg-text-muted">gorge never imports either companion</text>
</svg>
<figcaption>gorge is self-contained. The two companions build on it at a pinned commit; see <a href="repos/">The three repositories</a>.</figcaption>
</figure>

## Where to go next

<div class="grid cards" markdown>

-   **[Repos](repos/index.md)**

    ---

    What lives in gorge, gorge-compliance and four-yay, which way the
    imports run, licensing, pinning and the branch flow.

-   **[Architecture: the gorge kernel](architecture/index.md)**

    ---

    The event log, the rules engine, the card pipeline, views and the
    server, and the [invariants](architecture/invariants.md) that hold them
    together.

-   **[Architecture: the dirty-dirac kernel](architecture/dirty-dirac/index.md)**

    ---

    A fixed-size, much faster second kernel for chosen card sets, checked
    step by step against gorge.

-   **[Contributing](contributing/index.md)**

    ---

    Set up, run the tests inside the budget, read the ratchets, and land
    your [first card fix](contributing/first-card-fix.md).

-   **[User's guide](guide/index.md)**

    ---

    Run a local server, play against a bot, watch tables, replay a game.

-   **[Bots](bots/index.md)**

    ---

    The decision contract a bot answers, the built-in bots, and how to
    connect your own.

-   **[Performance](performance/index.md)**

    ---

    How engine speed is measured and the current numbers.

-   **[Card support](support/sets.md)**

    ---

    Per-set support for Standard and estimates for everything else.

</div>

## Licensing

gorge is Apache-2.0. Forge's card scripts are GPL-3.0, so gorge ships a
*compiler* for the script format and never the scripts themselves: they are
fetched into a gitignored `.cards/` directory at a pinned commit, and a test
(`TestNoForgeScriptsTracked`) fails the build if one is ever tracked. Details
are on [The three repositories](repos/index.md#licensing-and-the-forge-card-scripts).
