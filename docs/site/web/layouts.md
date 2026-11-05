# Table layouts

Every part of the table is yours to arrange: how opponents are placed, how your own board is cut into rows, how much of your hand shows, and where the stack, log and prompt sit. Pick a preset, then adjust. Layouts live only in your browser and are never sent to the server (`web/src/lib/layoutprofile.ts:3-6`).

Open the **Layout** drawer from the pill on the centre bar (`web/src/components/CentreStrip.svelte:133`) or from Game options, Board layout (`web/src/components/PlaySettingsPanel.svelte:606`). Each control applies at once, so the table behind the drawer is the preview (`web/src/components/LayoutDrawer.svelte:50-52`).

## Presets

A preset sets the opponent arrangement, the opponents' share of the height and your row template. It leaves your card, hand and panel choices alone (`web/src/lib/layoutprofile.ts:145`, `:227`).

<p class="gg-wire-legend"><span class="sw-opp">opponent board</span><span class="sw-c">creatures</span><span class="sw-l">lands</span><span class="sw-o">other permanents</span><span class="sw-p">decision prompt</span></p>

<div class="gg-wire-grid"><figure class="gg-wire"><div class="gg-wire__frame" role="img" aria-label="Duel: one opponent board above, your board below in two rows, hand along the bottom, stack and log on the right."><div class="gg-wire__board"><div class="gg-wire__opp" style="flex:0.4 1 0%;grid-template-columns:repeat(1,1fr);grid-template-rows:repeat(1,1fr)"><i class="gg-wire__seat"></i></div><div class="gg-wire__bar"></div><div class="gg-wire__me" style="flex:0.6 1 0%"><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--c" style="--w:1"></i></div><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--l" style="--w:1"></i><i class="gg-wire__reg gg-wire__reg--o" style="--w:0.7"></i></div></div><div class="gg-wire__hand"><i></i><i></i><i></i><i></i><i></i><i></i></div></div><div class="gg-wire__rail"><i class="gg-wire__box gg-wire__box--stack"><b>stack</b></i><i class="gg-wire__box gg-wire__box--log"><b>log</b></i><i class="gg-wire__box gg-wire__box--prompt"><b>prompt</b></i></div></div><figcaption><strong>Duel.</strong> One opponent. Creatures on the front row; lands and other permanents share the back row. The default.</figcaption></figure><figure class="gg-wire"><div class="gg-wire__frame" role="img" aria-label="Duel with three rows: one opponent board above; your creatures, other permanents and lands each on their own row."><div class="gg-wire__board"><div class="gg-wire__opp" style="flex:0.4 1 0%;grid-template-columns:repeat(1,1fr);grid-template-rows:repeat(1,1fr)"><i class="gg-wire__seat"></i></div><div class="gg-wire__bar"></div><div class="gg-wire__me" style="flex:0.6 1 0%"><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--c" style="--w:1"></i></div><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--o" style="--w:1"></i></div><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--l" style="--w:1"></i></div></div><div class="gg-wire__hand"><i></i><i></i><i></i><i></i><i></i><i></i></div></div><div class="gg-wire__rail"><i class="gg-wire__box gg-wire__box--stack"><b>stack</b></i><i class="gg-wire__box gg-wire__box--log"><b>log</b></i><i class="gg-wire__box gg-wire__box--prompt"><b>prompt</b></i></div></div><figcaption><strong>Duel, 3 rows.</strong> Each of your regions gets its own row. Choose it when you run many noncreature permanents.</figcaption></figure><figure class="gg-wire"><div class="gg-wire__frame" role="img" aria-label="Commander 4: three opponent boards side by side above, your board below in two rows."><div class="gg-wire__board"><div class="gg-wire__opp" style="flex:0.46 1 0%;grid-template-columns:repeat(3,1fr);grid-template-rows:repeat(1,1fr)"><i class="gg-wire__seat"></i><i class="gg-wire__seat"></i><i class="gg-wire__seat"></i></div><div class="gg-wire__bar"></div><div class="gg-wire__me" style="flex:0.54 1 0%"><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--c" style="--w:1"></i></div><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--l" style="--w:1"></i><i class="gg-wire__reg gg-wire__reg--o" style="--w:0.7"></i></div></div><div class="gg-wire__hand"><i></i><i></i><i></i><i></i><i></i><i></i></div></div><div class="gg-wire__rail"><i class="gg-wire__box gg-wire__box--stack"><b>stack</b></i><i class="gg-wire__box gg-wire__box--log"><b>log</b></i><i class="gg-wire__box gg-wire__box--prompt"><b>prompt</b></i></div></div><figcaption><strong>Commander 4.</strong> Three opponents in columns, a slightly larger opponent share. For a four-seat pod.</figcaption></figure><figure class="gg-wire"><div class="gg-wire__frame" role="img" aria-label="6 players, grid: five opponent boards in two rows of three above, your board below."><div class="gg-wire__board"><div class="gg-wire__opp" style="flex:0.52 1 0%;grid-template-columns:repeat(3,1fr);grid-template-rows:repeat(2,1fr)"><i class="gg-wire__seat"></i><i class="gg-wire__seat"></i><i class="gg-wire__seat"></i><i class="gg-wire__seat"></i><i class="gg-wire__seat"></i></div><div class="gg-wire__bar"></div><div class="gg-wire__me" style="flex:0.48 1 0%"><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--c" style="--w:1"></i></div><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--l" style="--w:1"></i><i class="gg-wire__reg gg-wire__reg--o" style="--w:0.7"></i></div></div><div class="gg-wire__hand"><i></i><i></i><i></i><i></i><i></i><i></i></div></div><div class="gg-wire__rail"><i class="gg-wire__box gg-wire__box--stack"><b>stack</b></i><i class="gg-wire__box gg-wire__box--log"><b>log</b></i><i class="gg-wire__box gg-wire__box--prompt"><b>prompt</b></i></div></div><figcaption><strong>6 players, grid.</strong> Opponents wrap into two rows once there are more than three. For five or six seats.</figcaption></figure><figure class="gg-wire"><div class="gg-wire__frame" role="img" aria-label="8 players, focus: one large opponent board with six narrow strips beside it above, your board below."><div class="gg-wire__board"><div class="gg-wire__opp" style="flex:0.5 1 0%;grid-template-columns:2.2fr 1fr;grid-template-rows:repeat(6,1fr)"><i class="gg-wire__seat gg-wire__seat--big" style="grid-row:1/span 6"></i><i class="gg-wire__seat gg-wire__seat--strip" style="grid-column:2;grid-row:1"></i><i class="gg-wire__seat gg-wire__seat--strip" style="grid-column:2;grid-row:2"></i><i class="gg-wire__seat gg-wire__seat--strip" style="grid-column:2;grid-row:3"></i><i class="gg-wire__seat gg-wire__seat--strip" style="grid-column:2;grid-row:4"></i><i class="gg-wire__seat gg-wire__seat--strip" style="grid-column:2;grid-row:5"></i><i class="gg-wire__seat gg-wire__seat--strip" style="grid-column:2;grid-row:6"></i></div><div class="gg-wire__bar"></div><div class="gg-wire__me" style="flex:0.5 1 0%"><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--c" style="--w:1"></i></div><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--l" style="--w:1"></i><i class="gg-wire__reg gg-wire__reg--o" style="--w:0.7"></i></div></div><div class="gg-wire__hand"><i></i><i></i><i></i><i></i><i></i><i></i></div></div><div class="gg-wire__rail"><i class="gg-wire__box gg-wire__box--stack"><b>stack</b></i><i class="gg-wire__box gg-wire__box--log"><b>log</b></i><i class="gg-wire__box gg-wire__box--prompt"><b>prompt</b></i></div></div><figcaption><strong>8 players, focus.</strong> One opponent full size, the rest as slim strips. For big tables.</figcaption></figure></div>

The preset ids are `duel`, `duel3`, `cmd4`, `grid6` and `focus8` (`web/src/lib/layoutprofile.ts:133-141`). `duel` is the default (`:197-209`).

## Opponent arrangement

You always sit at the bottom. Opponents fill the top in turn order from you, left to right, so the seat to your left reads first. A spectator has no seat, so the lowest seat takes the bottom (`web/src/lib/tablearrange.ts:403-411`).

<div class="gg-wire-grid"><figure class="gg-wire"><div class="gg-wire__frame" role="img" aria-label="Columns with four opponents: four boards in one row above, your board below."><div class="gg-wire__board"><div class="gg-wire__opp" style="flex:0.46 1 0%;grid-template-columns:repeat(4,1fr);grid-template-rows:repeat(1,1fr)"><i class="gg-wire__seat"></i><i class="gg-wire__seat"></i><i class="gg-wire__seat"></i><i class="gg-wire__seat"></i></div><div class="gg-wire__bar"></div><div class="gg-wire__me" style="flex:0.54 1 0%"><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--c" style="--w:1"></i></div><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--l" style="--w:1"></i><i class="gg-wire__reg gg-wire__reg--o" style="--w:0.7"></i></div></div><div class="gg-wire__hand"><i></i><i></i><i></i><i></i><i></i><i></i></div></div><div class="gg-wire__rail"><i class="gg-wire__box gg-wire__box--stack"><b>stack</b></i><i class="gg-wire__box gg-wire__box--log"><b>log</b></i><i class="gg-wire__box gg-wire__box--prompt"><b>prompt</b></i></div></div><figcaption><strong>Columns.</strong> One row, one column per opponent.</figcaption></figure><figure class="gg-wire"><div class="gg-wire__frame" role="img" aria-label="Grid with four opponents: four boards in two rows of two above, your board below."><div class="gg-wire__board"><div class="gg-wire__opp" style="flex:0.5 1 0%;grid-template-columns:repeat(2,1fr);grid-template-rows:repeat(2,1fr)"><i class="gg-wire__seat"></i><i class="gg-wire__seat"></i><i class="gg-wire__seat"></i><i class="gg-wire__seat"></i></div><div class="gg-wire__bar"></div><div class="gg-wire__me" style="flex:0.5 1 0%"><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--c" style="--w:1"></i></div><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--l" style="--w:1"></i><i class="gg-wire__reg gg-wire__reg--o" style="--w:0.7"></i></div></div><div class="gg-wire__hand"><i></i><i></i><i></i><i></i><i></i><i></i></div></div><div class="gg-wire__rail"><i class="gg-wire__box gg-wire__box--stack"><b>stack</b></i><i class="gg-wire__box gg-wire__box--log"><b>log</b></i><i class="gg-wire__box gg-wire__box--prompt"><b>prompt</b></i></div></div><figcaption><strong>Grid.</strong> Two rows once there are four or more opponents; with three or fewer it looks like columns.</figcaption></figure><figure class="gg-wire"><div class="gg-wire__frame" role="img" aria-label="Focus with four opponents: one large board and three narrow strips above, your board below."><div class="gg-wire__board"><div class="gg-wire__opp" style="flex:0.5 1 0%;grid-template-columns:2.2fr 1fr;grid-template-rows:repeat(3,1fr)"><i class="gg-wire__seat gg-wire__seat--big" style="grid-row:1/span 3"></i><i class="gg-wire__seat gg-wire__seat--strip" style="grid-column:2;grid-row:1"></i><i class="gg-wire__seat gg-wire__seat--strip" style="grid-column:2;grid-row:2"></i><i class="gg-wire__seat gg-wire__seat--strip" style="grid-column:2;grid-row:3"></i></div><div class="gg-wire__bar"></div><div class="gg-wire__me" style="flex:0.5 1 0%"><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--c" style="--w:1"></i></div><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--l" style="--w:1"></i><i class="gg-wire__reg gg-wire__reg--o" style="--w:0.7"></i></div></div><div class="gg-wire__hand"><i></i><i></i><i></i><i></i><i></i><i></i></div></div><div class="gg-wire__rail"><i class="gg-wire__box gg-wire__box--stack"><b>stack</b></i><i class="gg-wire__box gg-wire__box--log"><b>log</b></i><i class="gg-wire__box gg-wire__box--prompt"><b>prompt</b></i></div></div><figcaption><strong>Focus.</strong> One opponent large, the rest as strips. With one opponent it is plain columns.</figcaption></figure></div>

Choose **columns** for up to three opponents, **grid** for five or six seats, and **focus** when one opponent matters most. Click a side strip to make that opponent the large one (`web/src/components/Board.svelte:213`, `web/src/lib/tablearrange.ts:461-474`).

*Opponent boards* has two settings. **Mirrored** flips each opponent's rows so their creatures sit nearest the centre, facing yours (the default). **Same as mine** keeps their rows in your order (`web/src/components/Quadrant.svelte:42-45`).

### The centre bar

Drag the bar between the boards to change the opponents' share of the height, from 18% to 72% (`web/src/lib/layoutprofile.ts:63-64`). Double-click resets it for the seat count. The grip is also a keyboard separator, so dragging is never required (`web/src/components/CentreStrip.svelte:16-21`, `:113`).

## Your board

Your permanents sort into three regions: **creatures**, **lands** and **other permanents**. Each region has four settings (`web/src/lib/layoutprofile.ts:81-90`, `web/src/components/LayoutDrawer.svelte:133-167`).

<div class="grid cards" markdown>

-   **Row**

    Rows 1 to 3. Row 1 is nearest the centre. Regions on the same row share its width.

-   **Width**

    S, M, L or XL: weights 0.4, 0.7, 1 and 1.6 against the other regions on the row (`web/src/lib/layoutprofile.ts:56-61`).

-   **Fill from**

    Left, centre or right. Where the region's cards gather when the row is not full.

-   **Order**

    Entry order, name or power. Entry order never moves cards already on the table; ties always keep entry order.

</div>

## Cards

<div class="grid cards" markdown>

-   **Stack identical**

    Merges identical permanents into one pile marked with a count. Toggle it with `S` (`web/src/lib/keymap.ts:90`).

-   **When a row is full**

    **Overlap** tightens the cards, **scroll** adds a scrollbar, **wrap** starts a new line (`web/src/lib/layoutprofile.ts:26`).

-   **Art tiles**

    Below a card width you choose, cards draw as art tiles. The slider runs from never (0) to 120 px; the default is 58 px (`web/src/lib/layoutprofile.ts:72-73`, `:203`).

-   **Region outlines**

    Draws each region's outline and label. Off by default; useful while you tune rows.

</div>

## Your hand

The hand is a fan along the bottom edge. Cards keep their size. When the hand is too wide, the cards overlap more, and when it fits they sit apart with a small gap (`web/src/lib/handfan.ts:523-541`).

<div class="grid cards" markdown>

-   **Hand visible**

    How much of each hand card shows at rest, 30% to 100%. The default is 72%.

-   **Hand card size**

    A multiplier from 0.6x to 1.6x (`web/src/lib/layoutprofile.ts:74-77`).

-   **Rises on hover**

    On by default: the card under the pointer lifts to full height.

</div>

## Stack, log and prompt

The rail holds the stack, the game log and the decision prompt. It can sit on either side, or be hidden for a wider board. Its width runs from 14% to 40% of the window, 21% by default (`web/src/lib/layoutprofile.ts:27`, `:65-66`, `:178-180`).

<div class="gg-wire-grid"><figure class="gg-wire"><div class="gg-wire__frame" role="img" aria-label="Rail on the right: stack, log and prompt in a column to the right of the board."><div class="gg-wire__board"><div class="gg-wire__opp" style="flex:0.4 1 0%;grid-template-columns:repeat(1,1fr);grid-template-rows:repeat(1,1fr)"><i class="gg-wire__seat"></i></div><div class="gg-wire__bar"></div><div class="gg-wire__me" style="flex:0.6 1 0%"><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--c" style="--w:1"></i></div><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--l" style="--w:1"></i><i class="gg-wire__reg gg-wire__reg--o" style="--w:0.7"></i></div></div><div class="gg-wire__hand"><i></i><i></i><i></i><i></i><i></i><i></i></div></div><div class="gg-wire__rail"><i class="gg-wire__box gg-wire__box--stack"><b>stack</b></i><i class="gg-wire__box gg-wire__box--log"><b>log</b></i><i class="gg-wire__box gg-wire__box--prompt"><b>prompt</b></i></div></div><figcaption><strong>Right.</strong> The default.</figcaption></figure><figure class="gg-wire"><div class="gg-wire__frame gg-wire__frame--left" role="img" aria-label="Rail on the left: stack, log and prompt in a column to the left of the board."><div class="gg-wire__board"><div class="gg-wire__opp" style="flex:0.4 1 0%;grid-template-columns:repeat(1,1fr);grid-template-rows:repeat(1,1fr)"><i class="gg-wire__seat"></i></div><div class="gg-wire__bar"></div><div class="gg-wire__me" style="flex:0.6 1 0%"><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--c" style="--w:1"></i></div><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--l" style="--w:1"></i><i class="gg-wire__reg gg-wire__reg--o" style="--w:0.7"></i></div></div><div class="gg-wire__hand"><i></i><i></i><i></i><i></i><i></i><i></i></div></div><div class="gg-wire__rail"><i class="gg-wire__box gg-wire__box--stack"><b>stack</b></i><i class="gg-wire__box gg-wire__box--log"><b>log</b></i><i class="gg-wire__box gg-wire__box--prompt"><b>prompt</b></i></div></div><figcaption><strong>Left.</strong> Mirrors the rail for left-handed mouse use or a second monitor.</figcaption></figure><figure class="gg-wire"><div class="gg-wire__frame" role="img" aria-label="Rail hidden: the board fills the whole width, no stack or log column."><div class="gg-wire__board"><div class="gg-wire__opp" style="flex:0.4 1 0%;grid-template-columns:repeat(1,1fr);grid-template-rows:repeat(1,1fr)"><i class="gg-wire__seat"></i></div><div class="gg-wire__bar"></div><div class="gg-wire__me" style="flex:0.6 1 0%"><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--c" style="--w:1"></i></div><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--l" style="--w:1"></i><i class="gg-wire__reg gg-wire__reg--o" style="--w:0.7"></i></div></div><div class="gg-wire__hand"><i></i><i></i><i></i><i></i><i></i><i></i></div></div></div><figcaption><strong>Hidden.</strong> The widest board; the stack and log are out of sight.</figcaption></figure></div>

When the table offers a [paired bot](paired-play.md), its panel gets a rail of its own. **Paired bot** has three settings: **opposite side** (the default) puts that rail on the side of the board away from the stack and log; **in the rail** keeps the panel inside the main rail; **hidden** tucks it into a drawer at the opposite edge that slides in when you point at it or tab into it (`web/src/lib/layoutprofile.ts:28-29`, `web/src/lib/partnerrail.ts`). Its width is separate from the main rail's: 14% to 30% of the window, 20% by default, so both rails at their widest still leave the board 30% (`web/src/lib/layoutprofile.ts:67-69`, `:182-184`). Below 64rem of width the paired bot never takes a column: set to opposite side, it joins the main rail. Without a paired bot the table looks exactly as above.

The **log** has three settings: per table (remember your last choice for each table), show, or hide (`web/src/lib/layoutprofile.ts:30`, `:52`). `L` toggles it (`web/src/lib/keymap.ts:89`).

Choose where the decision prompt appears (`web/src/lib/layoutprofile.ts:31`, `:53`). While you pay a cost, a prompt set to near table moves to the rail (`web/src/lib/prompts/dock.ts:54-57`):

<div class="gg-wire-grid"><figure class="gg-wire"><div class="gg-wire__frame" role="img" aria-label="Prompt at the top of the rail, above the stack and log."><div class="gg-wire__board"><div class="gg-wire__opp" style="flex:0.4 1 0%;grid-template-columns:repeat(1,1fr);grid-template-rows:repeat(1,1fr)"><i class="gg-wire__seat"></i></div><div class="gg-wire__bar"></div><div class="gg-wire__me" style="flex:0.6 1 0%"><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--c" style="--w:1"></i></div><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--l" style="--w:1"></i><i class="gg-wire__reg gg-wire__reg--o" style="--w:0.7"></i></div></div><div class="gg-wire__hand"><i></i><i></i><i></i><i></i><i></i><i></i></div></div><div class="gg-wire__rail"><i class="gg-wire__box gg-wire__box--prompt"><b>prompt</b></i><i class="gg-wire__box gg-wire__box--stack"><b>stack</b></i><i class="gg-wire__box gg-wire__box--log"><b>log</b></i></div></div><figcaption><strong>Rail top.</strong> Prompt above the stack.</figcaption></figure><figure class="gg-wire"><div class="gg-wire__frame" role="img" aria-label="Prompt at the bottom of the rail, below the stack and log."><div class="gg-wire__board"><div class="gg-wire__opp" style="flex:0.4 1 0%;grid-template-columns:repeat(1,1fr);grid-template-rows:repeat(1,1fr)"><i class="gg-wire__seat"></i></div><div class="gg-wire__bar"></div><div class="gg-wire__me" style="flex:0.6 1 0%"><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--c" style="--w:1"></i></div><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--l" style="--w:1"></i><i class="gg-wire__reg gg-wire__reg--o" style="--w:0.7"></i></div></div><div class="gg-wire__hand"><i></i><i></i><i></i><i></i><i></i><i></i></div></div><div class="gg-wire__rail"><i class="gg-wire__box gg-wire__box--stack"><b>stack</b></i><i class="gg-wire__box gg-wire__box--log"><b>log</b></i><i class="gg-wire__box gg-wire__box--prompt"><b>prompt</b></i></div></div><figcaption><strong>Rail bottom.</strong> The default.</figcaption></figure><figure class="gg-wire"><div class="gg-wire__frame" role="img" aria-label="Prompt on the table, in a strip between the opponents and your board."><div class="gg-wire__board"><div class="gg-wire__opp" style="flex:0.4 1 0%;grid-template-columns:repeat(1,1fr);grid-template-rows:repeat(1,1fr)"><i class="gg-wire__seat"></i></div><div class="gg-wire__bar"></div><i class="gg-wire__box gg-wire__box--prompt gg-wire__box--inline"><b>prompt</b></i><div class="gg-wire__me" style="flex:0.6 1 0%"><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--c" style="--w:1"></i></div><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--l" style="--w:1"></i><i class="gg-wire__reg gg-wire__reg--o" style="--w:0.7"></i></div></div><div class="gg-wire__hand"><i></i><i></i><i></i><i></i><i></i><i></i></div></div><div class="gg-wire__rail"><i class="gg-wire__box gg-wire__box--stack"><b>stack</b></i><i class="gg-wire__box gg-wire__box--log"><b>log</b></i></div></div><figcaption><strong>Near table.</strong> The prompt sits by the centre bar.</figcaption></figure><figure class="gg-wire"><div class="gg-wire__frame" role="img" aria-label="Prompt floating over the board, draggable."><div class="gg-wire__board"><div class="gg-wire__opp" style="flex:0.4 1 0%;grid-template-columns:repeat(1,1fr);grid-template-rows:repeat(1,1fr)"><i class="gg-wire__seat"></i></div><div class="gg-wire__bar"></div><div class="gg-wire__me" style="flex:0.6 1 0%"><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--c" style="--w:1"></i></div><div class="gg-wire__row"><i class="gg-wire__reg gg-wire__reg--l" style="--w:1"></i><i class="gg-wire__reg gg-wire__reg--o" style="--w:0.7"></i></div></div><div class="gg-wire__hand"><i></i><i></i><i></i><i></i><i></i><i></i></div><i class="gg-wire__box gg-wire__box--prompt gg-wire__box--float"><b>prompt</b></i></div><div class="gg-wire__rail"><i class="gg-wire__box gg-wire__box--stack"><b>stack</b></i><i class="gg-wire__box gg-wire__box--log"><b>log</b></i></div></div><figcaption><strong>Floating.</strong> Drag it anywhere; the position is saved.</figcaption></figure></div>

## Saved layouts

Save the current layout under a name of up to 24 characters. Saved layouts appear in the drawer's menu and in the cycle list after the presets. `Ctrl+Shift+.` and `Ctrl+Shift+,` step through the list. Keys for layouts 1 to 9 exist but start unbound (`web/src/lib/keymap.ts:87-88`, `:23`, `web/src/lib/layoutlibrary.ts:193-201`).

Export writes your saved layouts to `gorge-layouts.json`. Import adds them back, skipping any it cannot read (`web/src/components/LayoutDrawer.svelte:73-90`). The library lives at the browser key `gorge.layouts.v1`, and an older single-layout key is migrated once on first load (`web/src/lib/layoutlibrary.ts:36`, `:99-123`).

## Motion and effects

Two more drawer settings belong to the browser, not to a layout, so switching layouts never changes them (`web/src/components/LayoutDrawer.svelte:193-202`):

- **Animations:** off, fast or normal (`web/src/lib/motion/settings.ts:9-10`). A system request for reduced motion turns them off.
- **Visual effects:** on or off. Turn them off if your GPU hangs on blur and shadows; `?fx=off` in the address does the same (`web/src/lib/visualfx.ts:20-21`).

Next: [Gameplay options](gameplay.md).
