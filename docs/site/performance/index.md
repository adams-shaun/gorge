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
    - **dirty-dirac, the fixed-card-set kernel checked in lockstep against
      gorge, is another 15-20× faster than gorge** (0.6 ms a game, 23 µs a
      turn), at the cost of playing only its generated card sets.
    - **Memory:** one game-playing process peaks at about 350 MB for gorge
      (the whole card corpus loaded), 0.9 GB for Forge and 1.2-1.3 GB for
      XMage with a 2 GB heap cap.

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
its MCTS search runs about 1,100 simulations per core-second (see
[Engine speed](engine.md#current-numbers)). These rows have no equivalent in
this harness for the other engines.

## Caveats

!!! warning "Not apples to apples"
    - **Card coverage differs.** XMage and Forge implement every card; gorge
      implements most of Forge's corpus; dirty-dirac plays only its generated
      card sets, two players. The workload is FDN because it is the pool all
      four play.
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
| Date | 2026-10-20, 17:45-18:07 UTC |
| Machine | AMD Ryzen 9 9950X (16 cores, 32 threads), 120 GB, Linux 6.17, `performance` governor |
| Cores | one per run: 24, 26, 28 (three reps at once); `GOMAXPROCS=1` for Go; JVMs pinned the same way (serial GC, `-Xmx2g`) |
| Load | 1-minute load average 4.4-15 across the session (per-row ranges in the per-pair table) |
| gorge | `ad3777023` (develop), Go 1.26.3, `cmd/enginebench` |
| dirty-dirac | private kernel, built from the companion module that pins gorge `aa421da93` |
| XMage | `magefree/mage` `6b602a1c85` (1.4.61), Temurin JDK 17.0.20.1, driver `XEngineBench` (gorge-compliance) |
| Forge | gorge's Forge fork at `da1c712c31` (forge-core/game/ai 2.0.16-SNAPSHOT), JDK 17.0.20.1, driver `forge.oracle.GameReplay` |
| Command | `SECS=30 scripts/xengine-bench.sh gorge dirty-dirac`, then `SECS=60 WARM=40 scripts/xengine-bench.sh xmage forge`, with `CORES="24 26 28"` |
| Measured time | 30 CPU-s per gorge and dirty-dirac run; 40 s warm-up + 60 CPU-s per XMage and Forge run |
| Errors | no stalled, unfinished or failed game in any run |

Engines that are not on this page were not installed on the measuring
machine; [Methodology](methodology.md#adding-an-engine) says what adding one
takes.
