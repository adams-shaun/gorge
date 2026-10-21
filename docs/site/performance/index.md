# Performance

How fast gorge plays Magic, measured against the other rules engines on the
same machine, the same decks and one CPU core each. This page is the headline
comparison for anyone choosing an engine to host games or to generate
self-play data; [Methodology](methodology.md) says exactly how it was
measured and how to rerun it, [Engine speed](engine.md) covers gorge's own
benchmark and profile, and [Bots](bots.md) covers what gorge's bots cost and
how well they play.

!!! abstract "The short version (2026-10-20, one core, FDN limited decks)"
    - **gorge plays a random-policy game in about 10 ms, about 40× as many
      games per core-second as XMage** (89 against 2.4 games per
      core-second; 29× cheaper per turn).
    - **With each engine's own AI on both seats, a gorge game costs 7-9 ms,
      against about 1 s for Forge and XMage's default AI**: 100-150× more
      games per core-second. The AIs are not the same kind of player (see
      below), so this is the price of a game with the bot each engine ships,
      not engine speed alone.
    - **dirty-dirac, the two-player kernel checked in lockstep against
      gorge, is another 15-20× faster than gorge** (0.6 ms a game, 23 µs a
      turn), at the cost of playing only its generated card pools. Its rows
      were measured before the 2026-10-21 kernel redesign; see
      [dirty-dirac's rows](#dirty-diracs-rows).
    - **mtg-kernel**, a public Rust kernel with a few hundred cards, is about
      as fast as gorge on its home Burn mirror (174 against 118-143 random
      games per core-second) and 6-13× slower per turn on FDN pair A, the
      FDN pair it plays ([details](#a-second-comparison-decks-mtg-kernel-plays)).
    - **Search:** one MCTS simulation costs 57-108 µs on dirty-dirac with a
      heuristic leaf, 90-177 µs with a small network leaf, and 0.9-1.6 ms on
      gorge, at 25-1,000 simulations per decision; almost all of it is the
      rules engine ([details](#search-bots-cost-per-simulation)).
    - **Memory:** one game-playing process peaks at about 350 MB for gorge
      (the whole card corpus loaded), 0.9 GB for Forge and 1.2-1.3 GB for
      XMage with a 2 GB heap cap; mtg-kernel, with its cards compiled in,
      11-16 MB.

## The numbers

Each bar is the median of six runs (FDN pairs A and B, three reps each); the
thin line through it spans the slowest and fastest run. Engines differ by more
than three orders of magnitude, so the first two charts use a log scale:
each grid line is ten times the one before. Hover a bar for its exact value
and spread.

<figure markdown>
<svg viewBox="0 0 760 344" width="100%" role="img" aria-label="Bar chart, log scale: games per core-second for each engine and policy">
<text class="dg-text-bold" x="0" y="16">Games per core-second (log scale; higher is faster)</text>
<rect class="dg-box-solid" x="460" y="6" width="12" height="12" rx="2"/><text class="dg-text-muted" x="476" y="16">random legal action</text>
<rect class="dg-box-accent" x="610" y="6" width="12" height="12" rx="2"/><text class="dg-text-muted" x="626" y="16">engine's own AI</text>
<line class="dg-line-dashed" x1="210.0" y1="28" x2="210.0" y2="322"/>
<text class="dg-text-muted" x="210.0" y="336" text-anchor="middle">0.1</text>
<line class="dg-line-dashed" x1="306.0" y1="28" x2="306.0" y2="322"/>
<text class="dg-text-muted" x="306.0" y="336" text-anchor="middle">1.00</text>
<line class="dg-line-dashed" x1="402.0" y1="28" x2="402.0" y2="322"/>
<text class="dg-text-muted" x="402.0" y="336" text-anchor="middle">10.0</text>
<line class="dg-line-dashed" x1="498.0" y1="28" x2="498.0" y2="322"/>
<text class="dg-text-muted" x="498.0" y="336" text-anchor="middle">100</text>
<line class="dg-line-dashed" x1="594.0" y1="28" x2="594.0" y2="322"/>
<text class="dg-text-muted" x="594.0" y="336" text-anchor="middle">1,000</text>
<line class="dg-line-dashed" x1="690.0" y1="28" x2="690.0" y2="322"/>
<text class="dg-text-muted" x="690.0" y="336" text-anchor="middle">10,000</text>
<g><title>dirty-dirac · random: 1,722 games/core-s (min 1,580, max 1,848)</title>
<text class="dg-text" x="202" y="48" text-anchor="end">dirty-dirac · random</text>
<rect class="dg-box-solid" x="210" y="34" width="406.7" height="20" rx="3"/>
<line class="dg-line" x1="613.1" y1="44" x2="619.6" y2="44"/>
<text class="dg-text" x="625.6" y="48">1,722</text></g>
<g><title>gorge · random (auto-pay): 89.4 games/core-s (min 85.4, max 93.9)</title>
<text class="dg-text" x="202" y="76" text-anchor="end">gorge · random (auto-pay)</text>
<rect class="dg-box-solid" x="210" y="62" width="283.3" height="20" rx="3"/>
<line class="dg-line" x1="491.4" y1="72" x2="495.4" y2="72"/>
<text class="dg-text" x="501.4" y="76">89.4</text></g>
<g><title>gorge · random (manual): 105 games/core-s (min 102, max 108)</title>
<text class="dg-text" x="202" y="104" text-anchor="end">gorge · random (manual)</text>
<rect class="dg-box-solid" x="210" y="90" width="289.9" height="20" rx="3"/>
<line class="dg-line" x1="498.9" y1="100" x2="501.4" y2="100"/>
<text class="dg-text" x="507.4" y="104">105</text></g>
<g><title>XMage · random: 2.38 games/core-s (min 2.16, max 2.58)</title>
<text class="dg-text" x="202" y="132" text-anchor="end">XMage · random</text>
<rect class="dg-box-solid" x="210" y="118" width="132.1" height="20" rx="3"/>
<line class="dg-line" x1="338.2" y1="128" x2="345.6" y2="128"/>
<text class="dg-text" x="351.6" y="132">2.38</text></g>
<g><title>dirty-dirac · hbot: 1,607 games/core-s (min 1,439, max 1,763)</title>
<text class="dg-text" x="202" y="160" text-anchor="end">dirty-dirac · hbot</text>
<rect class="dg-box-accent" x="210" y="146" width="403.8" height="20" rx="3"/>
<line class="dg-line" x1="609.2" y1="156" x2="617.6" y2="156"/>
<text class="dg-text" x="623.6" y="160">1,607</text></g>
<g><title>gorge · bot (auto-pay): 107 games/core-s (min 102, max 112)</title>
<text class="dg-text" x="202" y="188" text-anchor="end">gorge · bot (auto-pay)</text>
<rect class="dg-box-accent" x="210" y="174" width="290.8" height="20" rx="3"/>
<line class="dg-line" x1="498.7" y1="184" x2="502.6" y2="184"/>
<text class="dg-text" x="508.6" y="188">107</text></g>
<g><title>gorge · bot (manual): 140 games/core-s (min 132, max 145)</title>
<text class="dg-text" x="202" y="216" text-anchor="end">gorge · bot (manual)</text>
<rect class="dg-box-accent" x="210" y="202" width="301.9" height="20" rx="3"/>
<line class="dg-line" x1="509.6" y1="212" x2="513.6" y2="212"/>
<text class="dg-text" x="519.6" y="216">140</text></g>
<g><title>XMage · simple AI: 19.3 games/core-s (min 18.6, max 19.7)</title>
<text class="dg-text" x="202" y="244" text-anchor="end">XMage · simple AI</text>
<rect class="dg-box-accent" x="210" y="230" width="219.4" height="20" rx="3"/>
<line class="dg-line" x1="427.8" y1="240" x2="430.4" y2="240"/>
<text class="dg-text" x="436.4" y="244">19.3</text></g>
<g><title>XMage · mad AI: 0.947 games/core-s (min 0.552, max 1.25)</title>
<text class="dg-text" x="202" y="272" text-anchor="end">XMage · mad AI</text>
<rect class="dg-box-accent" x="210" y="258" width="93.8" height="20" rx="3"/>
<line class="dg-line" x1="281.2" y1="268" x2="315.2" y2="268"/>
<text class="dg-text" x="321.2" y="272">0.947</text></g>
<g><title>Forge · Forge AI: 1.02 games/core-s (min 0.723, max 1.22)</title>
<text class="dg-text" x="202" y="300" text-anchor="end">Forge · Forge AI</text>
<rect class="dg-box-accent" x="210" y="286" width="96.8" height="20" rx="3"/>
<line class="dg-line" x1="292.5" y1="296" x2="314.1" y2="296"/>
<text class="dg-text" x="320.1" y="300">1.02</text></g>
<line class="dg-line" x1="210" y1="28" x2="210" y2="322"/>
</svg>
<figcaption>Games per core-second, log scale. FDN pairs A and B pooled, median of 6 runs, thin line = min-max. 2026-10-20, gorge <code>ad3777023</code>, one pinned core per run (24/26/28), load average 4.4-15; <code>scripts/xengine-bench.sh</code>. Filled bars: random legal action; outlined bars: the engine's own AI.</figcaption>
</figure>

<figure markdown>
<svg viewBox="0 0 760 344" width="100%" role="img" aria-label="Bar chart, log scale: CPU microseconds per turn for each engine and policy">
<text class="dg-text-bold" x="0" y="16">CPU microseconds per turn (log scale; lower is cheaper)</text>
<rect class="dg-box-solid" x="460" y="6" width="12" height="12" rx="2"/><text class="dg-text-muted" x="476" y="16">random legal action</text>
<rect class="dg-box-accent" x="610" y="6" width="12" height="12" rx="2"/><text class="dg-text-muted" x="626" y="16">engine's own AI</text>
<line class="dg-line-dashed" x1="210.0" y1="28" x2="210.0" y2="322"/>
<text class="dg-text-muted" x="210.0" y="336" text-anchor="middle">10.0</text>
<line class="dg-line-dashed" x1="306.0" y1="28" x2="306.0" y2="322"/>
<text class="dg-text-muted" x="306.0" y="336" text-anchor="middle">100</text>
<line class="dg-line-dashed" x1="402.0" y1="28" x2="402.0" y2="322"/>
<text class="dg-text-muted" x="402.0" y="336" text-anchor="middle">1,000</text>
<line class="dg-line-dashed" x1="498.0" y1="28" x2="498.0" y2="322"/>
<text class="dg-text-muted" x="498.0" y="336" text-anchor="middle">10,000</text>
<line class="dg-line-dashed" x1="594.0" y1="28" x2="594.0" y2="322"/>
<text class="dg-text-muted" x="594.0" y="336" text-anchor="middle">100,000</text>
<line class="dg-line-dashed" x1="690.0" y1="28" x2="690.0" y2="322"/>
<text class="dg-text-muted" x="690.0" y="336" text-anchor="middle">1,000,000</text>
<g><title>dirty-dirac · random: 23.3 µs/turn (min 22.8, max 24.0)</title>
<text class="dg-text" x="202" y="48" text-anchor="end">dirty-dirac · random</text>
<rect class="dg-box-solid" x="210" y="34" width="35.3" height="20" rx="3"/>
<line class="dg-line" x1="244.4" y1="44" x2="246.5" y2="44"/>
<text class="dg-text" x="252.5" y="48">23.3</text></g>
<g><title>gorge · random (auto-pay): 459 µs/turn (min 443, max 473)</title>
<text class="dg-text" x="202" y="76" text-anchor="end">gorge · random (auto-pay)</text>
<rect class="dg-box-solid" x="210" y="62" width="159.5" height="20" rx="3"/>
<line class="dg-line" x1="368.0" y1="72" x2="370.8" y2="72"/>
<text class="dg-text" x="376.8" y="76">459</text></g>
<g><title>gorge · random (manual): 233 µs/turn (min 232, max 236)</title>
<text class="dg-text" x="202" y="104" text-anchor="end">gorge · random (manual)</text>
<rect class="dg-box-solid" x="210" y="90" width="131.3" height="20" rx="3"/>
<line class="dg-line" x1="341.0" y1="100" x2="341.8" y2="100"/>
<text class="dg-text" x="347.8" y="104">233</text></g>
<g><title>XMage · random: 13,193 µs/turn (min 12,774, max 13,751)</title>
<text class="dg-text" x="202" y="132" text-anchor="end">XMage · random</text>
<rect class="dg-box-solid" x="210" y="118" width="299.6" height="20" rx="3"/>
<line class="dg-line" x1="508.2" y1="128" x2="511.3" y2="128"/>
<text class="dg-text" x="517.3" y="132">13,193</text></g>
<g><title>dirty-dirac · hbot: 35.0 µs/turn (min 31.8, max 38.6)</title>
<text class="dg-text" x="202" y="160" text-anchor="end">dirty-dirac · hbot</text>
<rect class="dg-box-accent" x="210" y="146" width="52.2" height="20" rx="3"/>
<line class="dg-line" x1="258.2" y1="156" x2="266.3" y2="156"/>
<text class="dg-text" x="272.3" y="160">35.0</text></g>
<g><title>gorge · bot (auto-pay): 525 µs/turn (min 507, max 544)</title>
<text class="dg-text" x="202" y="188" text-anchor="end">gorge · bot (auto-pay)</text>
<rect class="dg-box-accent" x="210" y="174" width="165.2" height="20" rx="3"/>
<line class="dg-line" x1="373.7" y1="184" x2="376.6" y2="184"/>
<text class="dg-text" x="382.6" y="188">525</text></g>
<g><title>gorge · bot (manual): 400 µs/turn (min 385, max 417)</title>
<text class="dg-text" x="202" y="216" text-anchor="end">gorge · bot (manual)</text>
<rect class="dg-box-accent" x="210" y="202" width="153.8" height="20" rx="3"/>
<line class="dg-line" x1="362.2" y1="212" x2="365.5" y2="212"/>
<text class="dg-text" x="371.5" y="216">400</text></g>
<g><title>XMage · simple AI: 761 µs/turn (min 743, max 791)</title>
<text class="dg-text" x="202" y="244" text-anchor="end">XMage · simple AI</text>
<rect class="dg-box-accent" x="210" y="230" width="180.6" height="20" rx="3"/>
<line class="dg-line" x1="389.6" y1="240" x2="392.2" y2="240"/>
<text class="dg-text" x="398.2" y="244">761</text></g>
<g><title>XMage · mad AI: 62,343 µs/turn (min 49,069, max 101,733)</title>
<text class="dg-text" x="202" y="272" text-anchor="end">XMage · mad AI</text>
<rect class="dg-box-accent" x="210" y="258" width="364.3" height="20" rx="3"/>
<line class="dg-line" x1="564.3" y1="268" x2="594.7" y2="268"/>
<text class="dg-text" x="600.7" y="272">62,343</text></g>
<g><title>Forge · Forge AI: 54,513 µs/turn (min 45,904, max 71,387)</title>
<text class="dg-text" x="202" y="300" text-anchor="end">Forge · Forge AI</text>
<rect class="dg-box-accent" x="210" y="286" width="358.7" height="20" rx="3"/>
<line class="dg-line" x1="561.5" y1="296" x2="579.9" y2="296"/>
<text class="dg-text" x="585.9" y="300">54,513</text></g>
<line class="dg-line" x1="210" y1="28" x2="210" y2="322"/>
</svg>
<figcaption>CPU µs per turn, log scale (turns count both players' turns). FDN pairs A and B pooled, median of 6 runs, thin line = min-max. 2026-10-20, gorge <code>ad3777023</code>, one pinned core per run (24/26/28), load average 4.4-15; <code>scripts/xengine-bench.sh</code>. Filled bars: random legal action; outlined bars: the engine's own AI.</figcaption>
</figure>

<figure markdown>
<svg viewBox="0 0 760 344" width="100%" role="img" aria-label="Bar chart: peak resident set size in megabytes for each engine and policy">
<text class="dg-text-bold" x="0" y="16">Peak resident memory, MB (one game-playing process)</text>
<rect class="dg-box-solid" x="460" y="6" width="12" height="12" rx="2"/><text class="dg-text-muted" x="476" y="16">random legal action</text>
<rect class="dg-box-accent" x="610" y="6" width="12" height="12" rx="2"/><text class="dg-text-muted" x="626" y="16">engine's own AI</text>
<line class="dg-line-dashed" x1="210.0" y1="28" x2="210.0" y2="322"/>
<text class="dg-text-muted" x="210.0" y="336" text-anchor="middle">0</text>
<line class="dg-line-dashed" x1="374.8" y1="28" x2="374.8" y2="322"/>
<text class="dg-text-muted" x="374.8" y="336" text-anchor="middle">500</text>
<line class="dg-line-dashed" x1="539.7" y1="28" x2="539.7" y2="322"/>
<text class="dg-text-muted" x="539.7" y="336" text-anchor="middle">1,000</text>
<g><title>dirty-dirac · random: 356 MB (min 353, max 485)</title>
<text class="dg-text" x="202" y="48" text-anchor="end">dirty-dirac · random</text>
<rect class="dg-box-solid" x="210" y="34" width="117.4" height="20" rx="3"/>
<line class="dg-line" x1="326.5" y1="44" x2="370.0" y2="44"/>
<text class="dg-text" x="376.0" y="48">356</text></g>
<g><title>gorge · random (auto-pay): 351 MB (min 347, max 354)</title>
<text class="dg-text" x="202" y="76" text-anchor="end">gorge · random (auto-pay)</text>
<rect class="dg-box-solid" x="210" y="62" width="115.7" height="20" rx="3"/>
<line class="dg-line" x1="324.4" y1="72" x2="326.7" y2="72"/>
<text class="dg-text" x="332.7" y="76">351</text></g>
<g><title>gorge · random (manual): 349 MB (min 348, max 393)</title>
<text class="dg-text" x="202" y="104" text-anchor="end">gorge · random (manual)</text>
<rect class="dg-box-solid" x="210" y="90" width="114.9" height="20" rx="3"/>
<line class="dg-line" x1="324.7" y1="100" x2="339.7" y2="100"/>
<text class="dg-text" x="345.7" y="104">349</text></g>
<g><title>XMage · random: 1,228 MB (min 1,227, max 1,240)</title>
<text class="dg-text" x="202" y="132" text-anchor="end">XMage · random</text>
<rect class="dg-box-solid" x="210" y="118" width="404.8" height="20" rx="3"/>
<line class="dg-line" x1="614.5" y1="128" x2="618.9" y2="128"/>
<text class="dg-text" x="624.9" y="132">1,228</text></g>
<g><title>dirty-dirac · hbot: 485 MB (min 383, max 583)</title>
<text class="dg-text" x="202" y="160" text-anchor="end">dirty-dirac · hbot</text>
<rect class="dg-box-accent" x="210" y="146" width="159.8" height="20" rx="3"/>
<line class="dg-line" x1="336.3" y1="156" x2="402.2" y2="156"/>
<text class="dg-text" x="408.2" y="160">485</text></g>
<g><title>gorge · bot (auto-pay): 348 MB (min 346, max 395)</title>
<text class="dg-text" x="202" y="188" text-anchor="end">gorge · bot (auto-pay)</text>
<rect class="dg-box-accent" x="210" y="174" width="114.6" height="20" rx="3"/>
<line class="dg-line" x1="323.9" y1="184" x2="340.2" y2="184"/>
<text class="dg-text" x="346.2" y="188">348</text></g>
<g><title>gorge · bot (manual): 355 MB (min 346, max 356)</title>
<text class="dg-text" x="202" y="216" text-anchor="end">gorge · bot (manual)</text>
<rect class="dg-box-accent" x="210" y="202" width="117.0" height="20" rx="3"/>
<line class="dg-line" x1="324.1" y1="212" x2="327.2" y2="212"/>
<text class="dg-text" x="333.2" y="216">355</text></g>
<g><title>XMage · simple AI: 1,219 MB (min 1,212, max 1,232)</title>
<text class="dg-text" x="202" y="244" text-anchor="end">XMage · simple AI</text>
<rect class="dg-box-accent" x="210" y="230" width="401.9" height="20" rx="3"/>
<line class="dg-line" x1="609.7" y1="240" x2="616.1" y2="240"/>
<text class="dg-text" x="622.1" y="244">1,219</text></g>
<g><title>XMage · mad AI: 1,288 MB (min 1,243, max 1,324)</title>
<text class="dg-text" x="202" y="272" text-anchor="end">XMage · mad AI</text>
<rect class="dg-box-accent" x="210" y="258" width="424.6" height="20" rx="3"/>
<line class="dg-line" x1="619.8" y1="268" x2="646.4" y2="268"/>
<text class="dg-text" x="652.4" y="272">1,288</text></g>
<g><title>Forge · Forge AI: 882 MB (min 858, max 918)</title>
<text class="dg-text" x="202" y="300" text-anchor="end">Forge · Forge AI</text>
<rect class="dg-box-accent" x="210" y="286" width="290.8" height="20" rx="3"/>
<line class="dg-line" x1="492.9" y1="296" x2="512.7" y2="296"/>
<text class="dg-text" x="518.7" y="300">882</text></g>
<line class="dg-line" x1="210" y1="28" x2="210" y2="322"/>
</svg>
<figcaption>Peak RSS of one game-playing process, card database included; JVMs capped at -Xmx2g. FDN pairs A and B pooled, median of 6 runs, thin line = min-max. 2026-10-20, gorge <code>ad3777023</code>, one pinned core per run (24/26/28), load average 4.4-15; <code>scripts/xengine-bench.sh</code>. Filled bars: random legal action; outlined bars: the engine's own AI.</figcaption>
</figure>

| Engine · policy | games / core-s | ms / game | µs / turn | µs / decision | turns / game | peak RSS (MB) |
|---|--:|--:|--:|--:|--:|--:|
| dirty-dirac · random | 1,722 <small>(1,580–1,848)</small> | 0.584 <small>(0.541–0.633)</small> | 23.3 <small>(22.8–24.0)</small> | 4.62 <small>(4.54–4.75)</small> | 25 | 356 |
| gorge · random (auto-pay) | 89.4 <small>(85.4–93.9)</small> | 11.2 <small>(10.7–11.7)</small> | 459 <small>(443–473)</small> | 19.3 <small>(18.8–19.7)</small> | 24 | 351 |
| gorge · random (manual) | 105 <small>(102–108)</small> | 9.56 <small>(9.23–9.78)</small> | 233 <small>(232–236)</small> | 8.49 <small>(8.46–8.54)</small> | 41 | 349 |
| XMage · random | 2.38 <small>(2.16–2.58)</small> | 420 <small>(387–462)</small> | 13,193 <small>(12,774–13,751)</small> | 499 <small>(487–518)</small> | 32 | 1,228 |
| dirty-dirac · hbot | 1,607 <small>(1,439–1,763)</small> | 0.628 <small>(0.567–0.695)</small> | 35.0 <small>(31.8–38.6)</small> | 6.35 <small>(5.94–6.84)</small> | 18 | 485 |
| gorge · bot (auto-pay) | 107 <small>(102–112)</small> | 9.36 <small>(8.96–9.83)</small> | 525 <small>(507–544)</small> | 21.3 <small>(20.9–21.7)</small> | 18 | 348 |
| gorge · bot (manual) | 140 <small>(132–145)</small> | 7.16 <small>(6.88–7.58)</small> | 400 <small>(385–417)</small> | 14.4 <small>(14.0–14.9)</small> | 18 | 355 |
| XMage · simple AI | 19.3 <small>(18.6–19.7)</small> | 51.9 <small>(50.7–53.9)</small> | 761 <small>(743–791)</small> | 48.3 <small>(47.1–50.2)</small> | 68 | 1,219 |
| XMage · mad AI | 0.947 <small>(0.552–1.25)</small> | 1,121 <small>(801–1,813)</small> | 62,343 <small>(49,069–101,733)</small> | 2,902 <small>(2,292–4,782)</small> | 17 | 1,288 |
| Forge · Forge AI | 1.02 <small>(0.723–1.22)</small> | 983 <small>(823–1,383)</small> | 54,513 <small>(45,904–71,387)</small> | n/a | 18 | 882 |

??? note "Per-pair detail (the summary table `xengine-bench.sh` prints)"
    | engine | policy | pair | reps | games/core-s | ms/game | µs/turn | µs/decision | turns/game | peak RSS MB | load1 | other busy |
    |---|---|---|---|---|---|---|---|---|---|---|---|
    | gorge | random-autopay | A | 3 | 92.3 (91.9-93.9) | 10.8 (10.7-10.9) | 471 (468-473) | 19.7 (19.5-19.7) | 22.9 | 351 | 7-7 | max 0% |
    | gorge | random | A | 3 | 103 (102-103) | 9.74 (9.70-9.78) | 232 (232-233) | 8.50 (8.48-8.54) | 41.9 | 349 | 7-7 | max 0% |
    | gorge | bot-autopay | A | 3 | 103 (102-103) | 9.70 (9.68-9.83) | 540 (538-544) | 21.5 (21.4-21.7) | 18.0 | 346 | 6-7 | max 0% |
    | gorge | bot | A | 3 | 134 (132-136) | 7.45 (7.37-7.58) | 412 (410-417) | 14.7 (14.7-14.9) | 18.1 | 355 | 6-6 | max 0% |
    | gorge | random-autopay | B | 3 | 86.8 (85.4-86.8) | 11.5 (11.5-11.7) | 445 (443-450) | 18.9 (18.8-19.0) | 26.0 | 347 | 6-6 | max 0% |
    | gorge | random | B | 3 | 108 (106-108) | 9.28 (9.23-9.42) | 233 (233-236) | 8.46 (8.46-8.54) | 39.8 | 348 | 6-6 | max 0% |
    | gorge | bot-autopay | B | 3 | 111 (110-112) | 9.02 (8.96-9.05) | 509 (507-512) | 21.0 (20.9-21.1) | 17.7 | 348 | 5-6 | max 0% |
    | gorge | bot | B | 3 | 144 (144-145) | 6.95 (6.88-6.96) | 388 (385-390) | 14.0 (14.0-14.1) | 17.9 | 348 | 5-6 | max 0% |
    | dirty-dirac | random | A | 3 | 1,848 (1,845-1,848) | 0.54 (0.54-0.54) | 22.9 (22.8-22.9) | 4.55 (4.54-4.56) | 23.7 | 361 | 5-6 | max 0% |
    | dirty-dirac | hbot | A | 3 | 1,457 (1,439-1,462) | 0.69 (0.68-0.69) | 38.1 (38.1-38.6) | 6.76 (6.75-6.84) | 18.0 | 490 | 5-5 | max 0% |
    | dirty-dirac | random | B | 3 | 1,587 (1,580-1,598) | 0.63 (0.63-0.63) | 23.8 (23.7-24.0) | 4.71 (4.69-4.75) | 26.4 | 356 | 5-5 | max 0% |
    | dirty-dirac | hbot | B | 3 | 1,763 (1,751-1,763) | 0.57 (0.57-0.57) | 31.8 (31.8-31.9) | 5.95 (5.94-5.95) | 17.9 | 480 | 4-5 | max 0% |
    | xmage | random | A | 3 | 2.50 (2.40-2.58) | 400 (387-416) | 13,063 (12,774-13,145) | 495 (487-496) | 30.6 | 1,228 | 4-5 | max 0% |
    | xmage | simple | A | 3 | 19.6 (19.5-19.7) | 50.9 (50.7-51.3) | 747 (743-753) | 47.4 (47.1-47.8) | 68.2 | 1,218 | 5-5 | max 0% |
    | xmage | mad | A | 3 | 1.22 (1.18-1.25) | 822 (801-850) | 51,169 (49,069-51,492) | 2,349 (2,292-2,391) | 16.3 | 1,286 | 5-12 | max 28% |
    | xmage | random | B | 3 | 2.32 (2.16-2.36) | 430 (425-462) | 13,357 (13,242-13,751) | 502 (502-518) | 32.2 | 1,232 | 12-14 | max 7% |
    | xmage | simple | B | 3 | 19.0 (18.6-19.1) | 52.7 (52.4-53.9) | 774 (770-791) | 49.1 (48.8-50.2) | 68.1 | 1,221 | 13-14 | max 0% |
    | xmage | mad | B | 3 | 0.62 (0.55-0.72) | 1,610 (1,391-1,813) | 91,739 (73,194-101,733) | 4,255 (3,413-4,782) | 17.8 | 1,320 | 10-13 | max 0% |
    | forge | ai | A | 3 | 0.81 (0.72-0.97) | 1,239 (1,035-1,383) | 67,548 (58,991-71,387) | n/a | 18.3 | 868 | 10-15 | max 3% |
    | forge | ai | B | 3 | 1.16 (1.07-1.22) | 862 (823-932) | 46,518 (45,904-50,034) | n/a | 18.6 | 902 | 9-13 | max 0% |

## What the comparison shows

**Engine cost.** The random rows measure each engine with a policy that costs
almost nothing, so they are as close to pure engine cost as each engine
allows. Per turn, dirty-dirac (23 µs) is about 20× cheaper than gorge with
auto-pay (459 µs), which is about 29× cheaper than XMage (13.2 ms). gorge's
manual-tap random row is cheaper per turn (233 µs) but plays twice as many
turns per game, because a random player tapping lands one at a time wastes
mana. Forge has no random player, so it has no row here.

**AI cost.** Within one engine, the AI rows show what the built-in bot adds.
gorge's heuristic bot ends games in 18 turns instead of 24-41, so a game with
two bots is *cheaper* than a random game, even though each turn costs a
little more (about 15% of a bot game's CPU is the bot itself, see
[Engine speed](engine.md#where-the-time-goes)). XMage's base `ComputerPlayer`
is the cheapest XMage policy per turn (0.76 ms), but its games drift to about
68 turns. XMage's default "mad" AI searches the game tree at every priority
and costs 50-100 ms a turn; Forge's AI costs 46-68 ms a turn.

**Search.** Search bots clone and step the engine many times per decision, so
the per-operation costs matter more than per-game ones: gorge clones a
mid-game state in 14 µs (28 µs with GC) and clones-and-steps in 39 µs, and
its MCTS search runs about 1,000-1,100 simulations per core-second at 25-100
simulations a decision (see [Engine speed](engine.md#current-numbers) and
[Search bots](#search-bots-cost-per-simulation) below, which compares it with
dirty-dirac's search).

## A second comparison: decks mtg-kernel plays

[mtg-kernel](https://github.com/jackmaiorino/mtg-kernel) is a public Rust
rules kernel (MIT) built for reinforcement learning on a fixed card pool. It
cannot play FDN pair B (it lacks four of its cards), so it is not in the
tables above. This section adds it on the two workloads it does play,
measured the same way (one pinned core per run, three reps, medians with
min-max): **FDN pair A**, which is exactly mtg-kernel's two FDN fixture decks
and which every engine here plays, and the **Burn mirror**, the mono-red
Pauper deck from mtg-kernel's own pool, which every engine except dirty-dirac
plays. mtg-kernel has no standalone built-in bot, so it has a random row only
([why](methodology.md#the-player-policies)).

!!! info "What differs about mtg-kernel"
    - **Card pool.** 452 card definitions at the measured commit: its Pauper
      decks plus 245 of FDN's 286 names. gorge plays about 94% of Forge's
      ~34,000-card corpus; XMage and Forge play every card.
    - **Its FDN cards are a build option.** They sit behind the cargo feature
      `limited-fdn-fixtures`, and that build plays Burn several times slower
      than the default build, so pair A uses the FDN build and Burn the
      default one.
    - **Its random player is the one from its own benchmark**: every priority
      window and every mana ability is a decision, as in gorge's manual-tap
      random row, which is the closest gorge row to compare with.
    - **Memory is not comparable**: mtg-kernel compiles its few hundred
      cards into the binary (11-16 MB resident), while gorge, XMage and Forge
      load a card database of tens of thousands of cards.

### FDN pair A, every engine

<figure markdown>
<svg viewBox="0 0 760 372" width="100%" role="img" aria-label="Bar chart, log scale: CPU microseconds per turn on FDN pair A for each engine and policy">
<text class="dg-text-bold" x="0" y="16">CPU µs per turn, FDN pair A (log scale; lower is cheaper)</text>
<rect class="dg-box-solid" x="460" y="6" width="12" height="12" rx="2"/><text class="dg-text-muted" x="476" y="16">random legal action</text>
<rect class="dg-box-accent" x="610" y="6" width="12" height="12" rx="2"/><text class="dg-text-muted" x="626" y="16">engine's own AI</text>
<line class="dg-line-dashed" x1="210.0" y1="28" x2="210.0" y2="350"/>
<text class="dg-text-muted" x="210.0" y="364" text-anchor="middle">10.0</text>
<line class="dg-line-dashed" x1="330.0" y1="28" x2="330.0" y2="350"/>
<text class="dg-text-muted" x="330.0" y="364" text-anchor="middle">100</text>
<line class="dg-line-dashed" x1="450.0" y1="28" x2="450.0" y2="350"/>
<text class="dg-text-muted" x="450.0" y="364" text-anchor="middle">1,000</text>
<line class="dg-line-dashed" x1="570.0" y1="28" x2="570.0" y2="350"/>
<text class="dg-text-muted" x="570.0" y="364" text-anchor="middle">10,000</text>
<line class="dg-line-dashed" x1="690.0" y1="28" x2="690.0" y2="350"/>
<text class="dg-text-muted" x="690.0" y="364" text-anchor="middle">100,000</text>
<g><title>dirty-dirac · random: 22.9 µs/turn (min 22.8, max 22.9)</title>
<text class="dg-text" x="202" y="48" text-anchor="end">dirty-dirac · random</text>
<rect class="dg-box-solid" x="210" y="34" width="43.1" height="20" rx="3"/>
<line class="dg-line" x1="253.1" y1="44" x2="253.2" y2="44"/>
<text class="dg-text" x="259.2" y="48">22.9</text></g>
<g><title>gorge · random (auto-pay): 471 µs/turn (min 468, max 473)</title>
<text class="dg-text" x="202" y="76" text-anchor="end">gorge · random (auto-pay)</text>
<rect class="dg-box-solid" x="210" y="62" width="200.8" height="20" rx="3"/>
<line class="dg-line" x1="410.5" y1="72" x2="411.0" y2="72"/>
<text class="dg-text" x="417.0" y="76">471</text></g>
<g><title>gorge · random (manual): 232 µs/turn (min 232, max 233)</title>
<text class="dg-text" x="202" y="104" text-anchor="end">gorge · random (manual)</text>
<rect class="dg-box-solid" x="210" y="90" width="163.9" height="20" rx="3"/>
<line class="dg-line" x1="373.8" y1="100" x2="374.0" y2="100"/>
<text class="dg-text" x="380.0" y="104">232</text></g>
<g><title>mtg-kernel · random: 3,075 µs/turn (min 3,011, max 3,113)</title>
<text class="dg-text" x="202" y="132" text-anchor="end">mtg-kernel · random</text>
<rect class="dg-box-solid" x="210" y="118" width="298.5" height="20" rx="3"/>
<line class="dg-line" x1="507.5" y1="128" x2="509.2" y2="128"/>
<text class="dg-text" x="515.2" y="132">3,075</text></g>
<g><title>XMage · random: 13,063 µs/turn (min 12,774, max 13,145)</title>
<text class="dg-text" x="202" y="160" text-anchor="end">XMage · random</text>
<rect class="dg-box-solid" x="210" y="146" width="373.9" height="20" rx="3"/>
<line class="dg-line" x1="582.8" y1="156" x2="584.2" y2="156"/>
<text class="dg-text" x="590.2" y="160">13,063</text></g>
<g><title>dirty-dirac · hbot: 38.1 µs/turn (min 38.1, max 38.6)</title>
<text class="dg-text" x="202" y="188" text-anchor="end">dirty-dirac · hbot</text>
<rect class="dg-box-accent" x="210" y="174" width="69.8" height="20" rx="3"/>
<line class="dg-line" x1="279.7" y1="184" x2="280.4" y2="184"/>
<text class="dg-text" x="286.4" y="188">38.1</text></g>
<g><title>gorge · bot (auto-pay): 540 µs/turn (min 538, max 544)</title>
<text class="dg-text" x="202" y="216" text-anchor="end">gorge · bot (auto-pay)</text>
<rect class="dg-box-accent" x="210" y="202" width="207.9" height="20" rx="3"/>
<line class="dg-line" x1="417.7" y1="212" x2="418.3" y2="212"/>
<text class="dg-text" x="424.3" y="216">540</text></g>
<g><title>gorge · bot (manual): 412 µs/turn (min 410, max 417)</title>
<text class="dg-text" x="202" y="244" text-anchor="end">gorge · bot (manual)</text>
<rect class="dg-box-accent" x="210" y="230" width="193.8" height="20" rx="3"/>
<line class="dg-line" x1="403.6" y1="240" x2="404.4" y2="240"/>
<text class="dg-text" x="410.4" y="244">412</text></g>
<g><title>XMage · simple AI: 747 µs/turn (min 743, max 753)</title>
<text class="dg-text" x="202" y="272" text-anchor="end">XMage · simple AI</text>
<rect class="dg-box-accent" x="210" y="258" width="224.8" height="20" rx="3"/>
<line class="dg-line" x1="434.5" y1="268" x2="435.2" y2="268"/>
<text class="dg-text" x="441.2" y="272">747</text></g>
<g><title>XMage · mad AI: 51,169 µs/turn (min 49,069, max 51,492)</title>
<text class="dg-text" x="202" y="300" text-anchor="end">XMage · mad AI</text>
<rect class="dg-box-accent" x="210" y="286" width="445.1" height="20" rx="3"/>
<line class="dg-line" x1="652.9" y1="296" x2="655.4" y2="296"/>
<text class="dg-text" x="661.4" y="300">51,169</text></g>
<g><title>Forge · Forge AI: 67,548 µs/turn (min 58,991, max 71,387)</title>
<text class="dg-text" x="202" y="328" text-anchor="end">Forge · Forge AI</text>
<rect class="dg-box-accent" x="210" y="314" width="459.6" height="20" rx="3"/>
<line class="dg-line" x1="662.5" y1="324" x2="672.4" y2="324"/>
<text class="dg-text" x="678.4" y="328">67,548</text></g>
<line class="dg-line" x1="210" y1="28" x2="210" y2="350"/>
</svg>
<figcaption>CPU µs per turn on FDN pair A, log scale. Median of 3 runs, thin line = min-max. 2026-10-20 (17:45-18:07 UTC; mtg-kernel 18:50 UTC), gorge <code>ad3777023</code>, mtg-kernel <code>058d075e3</code>, one pinned core per run (24/26/28); <code>scripts/xengine-bench.sh</code>. Filled bars: random legal action; outlined bars: the engine's own AI.</figcaption>
</figure>

| Engine · policy | games / core-s | ms / game | µs / turn | µs / decision | turns / game | peak RSS (MB) |
|---|--:|--:|--:|--:|--:|--:|
| dirty-dirac · random | 1,848 <small>(1,845–1,848)</small> | 0.541 <small>(0.541–0.542)</small> | 22.9 <small>(22.8–22.9)</small> | 4.55 <small>(4.54–4.56)</small> | 24 | 361 |
| gorge · random (auto-pay) | 92.3 <small>(91.9–93.9)</small> | 10.8 <small>(10.7–10.9)</small> | 471 <small>(468–473)</small> | 19.7 <small>(19.5–19.7)</small> | 23 | 351 |
| gorge · random (manual) | 103 <small>(102–103)</small> | 9.74 <small>(9.70–9.78)</small> | 232 <small>(232–233)</small> | 8.50 <small>(8.48–8.54)</small> | 42 | 349 |
| mtg-kernel · random | 18.6 <small>(18.4–19.4)</small> | 53.8 <small>(51.6–54.5)</small> | 3,075 <small>(3,011–3,113)</small> | 57.2 <small>(56.3–57.7)</small> | 17 | 15.7 |
| XMage · random | 2.50 <small>(2.40–2.58)</small> | 400 <small>(387–416)</small> | 13,063 <small>(12,774–13,145)</small> | 495 <small>(487–496)</small> | 31 | 1,228 |
| dirty-dirac · hbot | 1,457 <small>(1,439–1,462)</small> | 0.686 <small>(0.684–0.695)</small> | 38.1 <small>(38.1–38.6)</small> | 6.76 <small>(6.75–6.84)</small> | 18 | 490 |
| gorge · bot (auto-pay) | 103 <small>(102–103)</small> | 9.70 <small>(9.68–9.83)</small> | 540 <small>(538–544)</small> | 21.5 <small>(21.4–21.7)</small> | 18 | 346 |
| gorge · bot (manual) | 134 <small>(132–136)</small> | 7.45 <small>(7.37–7.58)</small> | 412 <small>(410–417)</small> | 14.7 <small>(14.7–14.9)</small> | 18 | 355 |
| XMage · simple AI | 19.6 <small>(19.5–19.7)</small> | 50.9 <small>(50.7–51.3)</small> | 747 <small>(743–753)</small> | 47.4 <small>(47.1–47.8)</small> | 68 | 1,218 |
| XMage · mad AI | 1.22 <small>(1.18–1.25)</small> | 822 <small>(801–850)</small> | 51,169 <small>(49,069–51,492)</small> | 2,349 <small>(2,292–2,391)</small> | 16 | 1,286 |
| Forge · Forge AI | 0.807 <small>(0.723–0.966)</small> | 1,239 <small>(1,035–1,383)</small> | 67,548 <small>(58,991–71,387)</small> | n/a | 18 | 868 |

### Burn mirror

<figure markdown>
<svg viewBox="0 0 760 288" width="100%" role="img" aria-label="Bar chart, log scale: CPU microseconds per turn on Burn mirror for each engine and policy">
<text class="dg-text-bold" x="0" y="16">CPU µs per turn, Burn mirror (log scale; lower is cheaper)</text>
<rect class="dg-box-solid" x="460" y="6" width="12" height="12" rx="2"/><text class="dg-text-muted" x="476" y="16">random legal action</text>
<rect class="dg-box-accent" x="610" y="6" width="12" height="12" rx="2"/><text class="dg-text-muted" x="626" y="16">engine's own AI</text>
<line class="dg-line-dashed" x1="210.0" y1="28" x2="210.0" y2="266"/>
<text class="dg-text-muted" x="210.0" y="280" text-anchor="middle">100</text>
<line class="dg-line-dashed" x1="330.0" y1="28" x2="330.0" y2="266"/>
<text class="dg-text-muted" x="330.0" y="280" text-anchor="middle">1,000</text>
<line class="dg-line-dashed" x1="450.0" y1="28" x2="450.0" y2="266"/>
<text class="dg-text-muted" x="450.0" y="280" text-anchor="middle">10,000</text>
<line class="dg-line-dashed" x1="570.0" y1="28" x2="570.0" y2="266"/>
<text class="dg-text-muted" x="570.0" y="280" text-anchor="middle">100,000</text>
<line class="dg-line-dashed" x1="690.0" y1="28" x2="690.0" y2="266"/>
<text class="dg-text-muted" x="690.0" y="280" text-anchor="middle">1,000,000</text>
<g><title>gorge · random (auto-pay): 267 µs/turn (min 266, max 267)</title>
<text class="dg-text" x="202" y="48" text-anchor="end">gorge · random (auto-pay)</text>
<rect class="dg-box-solid" x="210" y="34" width="51.2" height="20" rx="3"/>
<line class="dg-line" x1="261.0" y1="44" x2="261.2" y2="44"/>
<text class="dg-text" x="267.2" y="48">267</text></g>
<g><title>gorge · random (manual): 240 µs/turn (min 238, max 240)</title>
<text class="dg-text" x="202" y="76" text-anchor="end">gorge · random (manual)</text>
<rect class="dg-box-solid" x="210" y="62" width="45.5" height="20" rx="3"/>
<line class="dg-line" x1="255.1" y1="72" x2="255.6" y2="72"/>
<text class="dg-text" x="261.6" y="76">240</text></g>
<g><title>mtg-kernel · random: 367 µs/turn (min 365, max 368)</title>
<text class="dg-text" x="202" y="104" text-anchor="end">mtg-kernel · random</text>
<rect class="dg-box-solid" x="210" y="90" width="67.8" height="20" rx="3"/>
<line class="dg-line" x1="277.5" y1="100" x2="277.8" y2="100"/>
<text class="dg-text" x="283.8" y="104">367</text></g>
<g><title>XMage · random: 11,188 µs/turn (min 10,856, max 11,308)</title>
<text class="dg-text" x="202" y="132" text-anchor="end">XMage · random</text>
<rect class="dg-box-solid" x="210" y="118" width="245.9" height="20" rx="3"/>
<line class="dg-line" x1="454.3" y1="128" x2="456.4" y2="128"/>
<text class="dg-text" x="462.4" y="132">11,188</text></g>
<g><title>gorge · bot (auto-pay): 437 µs/turn (min 434, max 439)</title>
<text class="dg-text" x="202" y="160" text-anchor="end">gorge · bot (auto-pay)</text>
<rect class="dg-box-accent" x="210" y="146" width="76.8" height="20" rx="3"/>
<line class="dg-line" x1="286.6" y1="156" x2="287.0" y2="156"/>
<text class="dg-text" x="293.0" y="160">437</text></g>
<g><title>gorge · bot (manual): 308 µs/turn (min 306, max 309)</title>
<text class="dg-text" x="202" y="188" text-anchor="end">gorge · bot (manual)</text>
<rect class="dg-box-accent" x="210" y="174" width="58.6" height="20" rx="3"/>
<line class="dg-line" x1="268.4" y1="184" x2="268.7" y2="184"/>
<text class="dg-text" x="274.7" y="188">308</text></g>
<g><title>XMage · mad AI: 272,821 µs/turn (min 226,484, max 345,899)</title>
<text class="dg-text" x="202" y="216" text-anchor="end">XMage · mad AI</text>
<rect class="dg-box-accent" x="210" y="202" width="412.3" height="20" rx="3"/>
<line class="dg-line" x1="612.6" y1="212" x2="634.7" y2="212"/>
<text class="dg-text" x="640.7" y="216">272,821</text></g>
<g><title>Forge · Forge AI: 35,533 µs/turn (min 33,783, max 36,489)</title>
<text class="dg-text" x="202" y="244" text-anchor="end">Forge · Forge AI</text>
<rect class="dg-box-accent" x="210" y="230" width="306.1" height="20" rx="3"/>
<line class="dg-line" x1="513.4" y1="240" x2="517.5" y2="240"/>
<text class="dg-text" x="523.5" y="244">35,533</text></g>
<line class="dg-line" x1="210" y1="28" x2="210" y2="266"/>
</svg>
<figcaption>CPU µs per turn on the Burn mirror, log scale. XMage's base AI is left out: on Burn it casts nothing and every game hits the 100-turn cap. Median of 3 runs, thin line = min-max. 2026-10-20 (18:50-19:00 UTC, load average 6-36), gorge <code>ad3777023</code>, mtg-kernel <code>058d075e3</code>, one pinned core per run (24/26/28); <code>scripts/xengine-bench.sh</code>. Filled bars: random legal action; outlined bars: the engine's own AI.</figcaption>
</figure>

| Engine · policy | games / core-s | ms / game | µs / turn | µs / decision | turns / game | peak RSS (MB) |
|---|--:|--:|--:|--:|--:|--:|
| gorge · random (auto-pay) | 143 <small>(142–143)</small> | 6.98 <small>(6.97–7.06)</small> | 267 <small>(266–267)</small> | 12.2 <small>(12.1–12.2)</small> | 26 | 354 |
| gorge · random (manual) | 118 <small>(117–118)</small> | 8.48 <small>(8.45–8.53)</small> | 240 <small>(238–240)</small> | 10.9 <small>(10.9–10.9)</small> | 36 | 353 |
| mtg-kernel · random | 174 <small>(173–174)</small> | 5.75 <small>(5.74–5.78)</small> | 367 <small>(365–368)</small> | 8.15 <small>(8.11–8.15)</small> | 16 | 10.9 |
| XMage · random | 3.14 <small>(2.91–3.18)</small> | 318 <small>(315–344)</small> | 11,188 <small>(10,856–11,308)</small> | 527 <small>(508–534)</small> | 29 | 1,262 |
| gorge · bot (auto-pay) | 111 <small>(110–112)</small> | 9.00 <small>(8.96–9.05)</small> | 437 <small>(434–439)</small> | 18.7 <small>(18.6–18.7)</small> | 21 | 347 |
| gorge · bot (manual) | 159 <small>(158–159)</small> | 6.30 <small>(6.27–6.31)</small> | 308 <small>(306–309)</small> | 12.1 <small>(12.1–12.2)</small> | 20 | 352 |
| XMage · mad AI | 0.259 <small>(0.179–0.282)</small> | 3,864 <small>(3,547–5,597)</small> | 272,821 <small>(226,484–345,899)</small> | 13,226 <small>(11,010–17,280)</small> | 16 | 1,336 |
| Forge · Forge AI | 1.58 <small>(1.45–1.64)</small> | 633 <small>(609–688)</small> | 35,533 <small>(33,783–36,489)</small> | n/a | 18 | 892 |

On FDN pair A, gorge's random players spend 233-471 µs a turn against
mtg-kernel's 3,075 µs; on its home Burn mirror mtg-kernel is close to gorge
(367 µs a turn against 240-267 µs), and its games are shorter, so it plays
174 games per core-second against gorge's 118-143. dirty-dirac remains
the fastest engine on pair A by an order of magnitude.

## Search bots: cost per simulation

A search bot clones and steps the game many times per decision, so its cost
is the cost of one simulation times the simulation budget. These rows put an
MCTS search on both seats (dirty-dirac) or on searched priority roots (gorge)
of FDN pair A at 25, 100 and 1,000 simulations per searched decision, on one
pinned core, three reps; [Methodology](methodology.md#search-bots) says
exactly what runs and how each is counted. They measure speed only.

<figure markdown>
<svg viewBox="0 0 760 316" width="100%" role="img" aria-label="Bar chart, log scale: CPU microseconds per MCTS simulation for dirty-dirac and gorge search bots at 25, 100 and 1000 simulations per decision">
<text class="dg-text-bold" x="0" y="16">CPU µs per simulation (log scale; lower is cheaper)</text>
<rect class="dg-box-solid" x="460" y="6" width="12" height="12" rx="2"/><text class="dg-text-muted" x="476" y="16">dirty-dirac</text>
<rect class="dg-box-accent" x="610" y="6" width="12" height="12" rx="2"/><text class="dg-text-muted" x="626" y="16">gorge</text>
<line class="dg-line-dashed" x1="300.0" y1="28" x2="300.0" y2="294"/>
<text class="dg-text-muted" x="300.0" y="308" text-anchor="middle">10.0</text>
<line class="dg-line-dashed" x1="430.0" y1="28" x2="430.0" y2="294"/>
<text class="dg-text-muted" x="430.0" y="308" text-anchor="middle">100</text>
<line class="dg-line-dashed" x1="560.0" y1="28" x2="560.0" y2="294"/>
<text class="dg-text-muted" x="560.0" y="308" text-anchor="middle">1,000</text>
<line class="dg-line-dashed" x1="690.0" y1="28" x2="690.0" y2="294"/>
<text class="dg-text-muted" x="690.0" y="308" text-anchor="middle">10,000</text>
<g><title>dirty-dirac, heuristic leaf, 25 sims: 57.1 µs/sim (min 56.9, max 58.1)</title>
<text class="dg-text" x="292" y="48" text-anchor="end">dirty-dirac, heuristic leaf, 25 sims</text>
<rect class="dg-box-solid" x="300" y="34" width="98.4" height="20" rx="3"/>
<line class="dg-line" x1="398.2" y1="44" x2="399.3" y2="44"/>
<text class="dg-text" x="405.3" y="48">57.1</text></g>
<g><title>dirty-dirac, heuristic leaf, 100 sims: 74.3 µs/sim (min 73.9, max 79.6)</title>
<text class="dg-text" x="292" y="76" text-anchor="end">dirty-dirac, heuristic leaf, 100 sims</text>
<rect class="dg-box-solid" x="300" y="62" width="113.2" height="20" rx="3"/>
<line class="dg-line" x1="412.9" y1="72" x2="417.1" y2="72"/>
<text class="dg-text" x="423.1" y="76">74.3</text></g>
<g><title>dirty-dirac, heuristic leaf, 1000 sims: 108 µs/sim (min 89.9, max 109)</title>
<text class="dg-text" x="292" y="104" text-anchor="end">dirty-dirac, heuristic leaf, 1000 sims</text>
<rect class="dg-box-solid" x="300" y="90" width="134.2" height="20" rx="3"/>
<line class="dg-line" x1="424.0" y1="100" x2="434.7" y2="100"/>
<text class="dg-text" x="440.7" y="104">108</text></g>
<g><title>dirty-dirac, network leaf, 25 sims: 99.3 µs/sim (min 96.9, max 103)</title>
<text class="dg-text" x="292" y="132" text-anchor="end">dirty-dirac, network leaf, 25 sims</text>
<rect class="dg-box-solid" x="300" y="118" width="129.6" height="20" rx="3"/>
<line class="dg-line" x1="428.2" y1="128" x2="431.5" y2="128"/>
<text class="dg-text" x="437.5" y="132">99.3</text></g>
<g><title>dirty-dirac, network leaf, 100 sims: 90.0 µs/sim (min 89.4, max 95.2)</title>
<text class="dg-text" x="292" y="160" text-anchor="end">dirty-dirac, network leaf, 100 sims</text>
<rect class="dg-box-solid" x="300" y="146" width="124.1" height="20" rx="3"/>
<line class="dg-line" x1="423.7" y1="156" x2="427.2" y2="156"/>
<text class="dg-text" x="433.2" y="160">90.0</text></g>
<g><title>dirty-dirac, network leaf, 1000 sims: 177 µs/sim (min 163, max 189)</title>
<text class="dg-text" x="292" y="188" text-anchor="end">dirty-dirac, network leaf, 1000 sims</text>
<rect class="dg-box-solid" x="300" y="174" width="162.2" height="20" rx="3"/>
<line class="dg-line" x1="457.5" y1="184" x2="465.9" y2="184"/>
<text class="dg-text" x="471.9" y="188">177</text></g>
<g><title>gorge azmcts, heuristic leaf, 25 sims: 914 µs/sim (min 908, max 966)</title>
<text class="dg-text" x="292" y="216" text-anchor="end">gorge azmcts, heuristic leaf, 25 sims</text>
<rect class="dg-box-accent" x="300" y="202" width="254.9" height="20" rx="3"/>
<line class="dg-line" x1="554.5" y1="212" x2="558.0" y2="212"/>
<text class="dg-text" x="564.0" y="216">914</text></g>
<g><title>gorge azmcts, heuristic leaf, 100 sims: 942 µs/sim (min 939, max 978)</title>
<text class="dg-text" x="292" y="244" text-anchor="end">gorge azmcts, heuristic leaf, 100 sims</text>
<rect class="dg-box-accent" x="300" y="230" width="256.7" height="20" rx="3"/>
<line class="dg-line" x1="556.5" y1="240" x2="558.7" y2="240"/>
<text class="dg-text" x="564.7" y="244">942</text></g>
<g><title>gorge azmcts, heuristic leaf, 1000 sims: 1,604 µs/sim (min 1,281, max 1,687)</title>
<text class="dg-text" x="292" y="272" text-anchor="end">gorge azmcts, heuristic leaf, 1000 sims</text>
<rect class="dg-box-accent" x="300" y="258" width="286.7" height="20" rx="3"/>
<line class="dg-line" x1="574.0" y1="268" x2="589.5" y2="268"/>
<text class="dg-text" x="595.5" y="272">1,604</text></g>
<line class="dg-line" x1="300" y1="28" x2="300" y2="294"/>
</svg>
<figcaption>CPU µs per MCTS simulation, log scale, FDN pair A, median of 3 runs, thin line = min-max. 2026-10-20 18:35-18:41 UTC, gorge <code>ad3777023</code>, one pinned core per run (24/26/28), load average 17-32 (core contention at most 10%). dirty-dirac counts whole games' CPU per simulation, gorge only the search's CPU.</figcaption>
</figure>

| Search bot | sims per decision | µs per simulation | simulations per core-second | ms per searched decision | ms per game | games or roots per run |
|---|--:|--:|--:|--:|--:|--:|
| dirty-dirac MCTS, heuristic leaf | 25 | 57.1 <small>(56.9–58.1)</small> | 17,499 <small>(17,222–17,560)</small> | 1.43 <small>(1.42–1.45)</small> | 108 <small>(105–113)</small> | 278-286-266 |
| dirty-dirac MCTS, heuristic leaf | 100 | 74.3 <small>(73.9–79.6)</small> | 13,455 <small>(12,560–13,527)</small> | 7.43 <small>(7.39–7.96)</small> | 546 <small>(535–623)</small> | 49-55-57 |
| dirty-dirac MCTS, heuristic leaf | 1000 | 108 <small>(89.9–109)</small> | 9,277 <small>(9,201–11,119)</small> | 108 <small>(89.9–109)</small> | 8,219 <small>(7,263–9,428)</small> | 4-4-4 |
| dirty-dirac MCTS, network leaf | 25 | 99.3 <small>(96.9–103)</small> | 10,075 <small>(9,737–10,323)</small> | 2.48 <small>(2.42–2.57)</small> | 171 <small>(168–174)</small> | 165-157-161 |
| dirty-dirac MCTS, network leaf | 100 | 90.0 <small>(89.4–95.2)</small> | 11,107 <small>(10,508–11,185)</small> | 9.00 <small>(8.94–9.52)</small> | 617 <small>(594–632)</small> | 51-48-49 |
| dirty-dirac MCTS, network leaf | 1000 | 177 <small>(163–189)</small> | 5,654 <small>(5,298–6,142)</small> | 177 <small>(163–189)</small> | 11,792 <small>(7,978–12,205)</small> | 4-3-3 |
| gorge MCTS (azmcts), heuristic leaf | 25 | 914 <small>(908–966)</small> | 1,094 <small>(1,035–1,102)</small> | 22.9 <small>(22.7–24.1)</small> | n/a | 1143-1131-1075 |
| gorge MCTS (azmcts), heuristic leaf | 100 | 942 <small>(939–978)</small> | 1,061 <small>(1,023–1,065)</small> | 94.2 <small>(93.9–97.8)</small> | n/a | 301-312-311 |
| gorge MCTS (azmcts), heuristic leaf | 1000 | 1,604 <small>(1,281–1,687)</small> | 623 <small>(593–780)</small> | 1,604 <small>(1,281–1,687)</small> | n/a | 26-20-20 |

**Where a simulation's time goes**, at 100 simulations:

| Search bot | Engine (re-deal, replay and expansion of the world) | Leaf evaluator | Search bookkeeping (select, backup) |
|---|--:|--:|--:|
| dirty-dirac MCTS, heuristic leaf (79 µs a simulation) | 78.8 µs (99.7%): re-deal 1.7, replaying the line 51.2, expanding the node 25.1 | included in the engine column | 0.24 µs |
| dirty-dirac MCTS, network leaf (99 µs a simulation) | 82.6 µs (84%) | 15.7 µs (16%): building the input 10.7, the forward pass 5.0 | 0.29 µs |
| gorge MCTS, heuristic leaf (about 940 µs a simulation) | most of it: `Engine.Submit` 83% cumulative (game play inside the search), re-dealing the world about 3%, building the in-search bot's board 5.7% | under 1% (`heuristicLeafValue` 0.15%) | about 1% (`azmcts` flat) |

The dirty-dirac split is its runner's per-simulation timers (wall time
inside the search, so its totals differ slightly from the CPU-time table);
the gorge split is a 20-second CPU profile of `enginebench -row az -sims 100`.
In both engines almost all of a simulation is the rules engine replaying and
extending a line of play; the search itself is noise, and a small network
leaf adds about a quarter on dirty-dirac. The per-simulation cost grows with
the budget because deeper trees replay longer lines.

## dirty-dirac's rows

Every dirty-dirac figure on this page (the game rows, the FDN pair A table and
the search rows) was measured on 2026-10-20 with the dirty-dirac source of
that day, which pinned gorge `aa421da93`. That was before the kernel redesign
of 2026-10-21 ([dirty-dirac](../architecture/dirty-dirac/index.md)). The rows
have not been re-run since. The redesign's own A/B, the kernel's transition
benchmark on one pinned core against the pre-redesign kernel, counts about
4 % fewer instructions per transition on FDN and about 2 % fewer on BLB.
Instruction counts are not CPU time, so treat the rows above as dated
measurements of the earlier kernel, not as current figures.

## Caveats

!!! warning "Not apples to apples"
    - **Card coverage differs.** XMage and Forge implement every card; gorge
      implements most of Forge's corpus; dirty-dirac plays only its generated
      card pools, two players; mtg-kernel plays a few hundred cards. The main
      workload is FDN because it is the pool the first four play; mtg-kernel
      plays only pair A of it.
    - **Policies differ.** The random players are close but not identical
      (XMage answers non-priority asks with its base AI); the AIs differ in
      kind and strength. Each row's policy is spelled out in
      [Methodology](methodology.md#the-player-policies).
    - **Decisions are counted differently**, so µs per decision is only
      comparable within an engine; µs per turn is the cross-engine
      normaliser.
    - **JVM numbers are steady state** after 40 s of warm-up games; start-up
      (8-10 s for XMage, about 6 s for Forge to load cards) is not in the
      per-game numbers.
    - **One shared machine.** Three reps of a row ran at once on three
      physical cores; a gorge and a dirty-dirac row run alone on one core
      came out within 4% of the concurrent runs.

## Provenance

| | |
|---|---|
| Date | 2026-10-20: FDN pairs A and B 17:45-18:07 UTC; search rows 18:35-18:41 UTC; mtg-kernel and the Burn mirror 18:50-19:00 UTC |
| Machine | AMD Ryzen 9 9950X (16 cores, 32 threads), 120 GB, Linux 6.17, `performance` governor |
| Cores | one per run: 24, 26, 28 (three reps at once); `GOMAXPROCS=1` for Go; JVMs pinned the same way (serial GC, `-Xmx2g`) |
| Load | 1-minute load average 4.4-15 for the FDN A/B tables (per-row ranges in the per-pair table), 6-36 for the Burn rows, 17-32 for the search rows; no published rep had more than 10% of its core taken by other work except one XMage `mad` pair-A rep (28%), whose result sits inside the other two reps' spread |
| gorge | `ad3777023` (develop), Go 1.26.3, `cmd/enginebench` |
| dirty-dirac | private kernel, the source of 2026-10-20 (before the redesign), which pinned gorge `aa421da93` |
| XMage | `magefree/mage` `6b602a1c85` (1.4.61), Temurin JDK 17.0.20.1, driver `XEngineBench` (gorge-compliance) |
| Forge | gorge's Forge fork at `da1c712c31` (forge-core/game/ai 2.0.16-SNAPSHOT), JDK 17.0.20.1, driver `forge.oracle.GameReplay` |
| mtg-kernel | `jackmaiorino/mtg-kernel` `058d075e3` (2026-10-20), Rust 1.94.1, release build of `examples/bench_kernel` with gorge's `--xengine-json` mode; FDN pair A from the `limited-fdn-fixtures` build, Burn from the default build |
| Command | `SECS=30 scripts/xengine-bench.sh gorge dirty-dirac`, then `SECS=60 WARM=40 scripts/xengine-bench.sh xmage forge`, with `CORES="24 26 28"`; the Burn rows the same with `PAIRS=burn`; mtg-kernel with `MTGKERNEL_BENCH=… MK_PAIRS=A` (FDN build) and `MK_PAIRS=burn` (default build); search rows as in [Methodology](methodology.md#search-bots) |
| Measured time | 30 CPU-s per gorge and dirty-dirac run; 40 s warm-up + 60 CPU-s per XMage and Forge run |
| Errors | no stalled, unfinished or failed game in any published row. XMage's base AI on the Burn mirror ended every game at the 100-turn cap without a winner (it never casts its burn spells), so that row is left out |

Engines that are not on this page were not installed on the measuring
machine; [Methodology](methodology.md#adding-an-engine) says what adding one
takes.
