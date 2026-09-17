# Search Probe Land-Mixture Weight-Cap Design

## Status

Rejected after the bounded development comparison. The committed 50/50
mixture remains the implementation. This document preserves the exact tested
alternative and why it was not retained.

## Outcome

Retain the existing land-isolation proposal while preventing its accepted
inside worlds from overwhelming effective sample size. This is a bounded
prototype for sampling coverage, not production search, policy training, or a
strength claim.

The fixed 50/50 mixture lost 43 roots that the prior proposal covered. Fifteen
lost roots had fewer than four accepted attempts; 28 still had at least four
acceptances and failed only ESS. Across those 28 ESS-only losses, accepted
outside-isolation worlds averaged 2.07 attempts but held 85.4% of normalized
weight and 98.8% of squared-weight mass. Every one of all 50 ESS-only fallback
roots assigned more than 90% of squared-weight mass to outside worlds.

## Observable inputs and unchanged boundaries

The proposal uses only the existing exact physical-permutation counts `|C|`
and `|L|`, where `C` is the history-compatible base set and `L` is its
land-isolated subset. Those counts already derive only from owned `History`,
public deck definitions, and the hypothetical hand/library at the shuffle.

No source engine, original seed, hidden source zone/hash, opponent-private
intent index, or source callback is added. Opponent actions still come only
from `botpolicy.Decide`, and the complete observed prefix remains the
acceptance oracle.

## Count-adaptive exact mixture

For nonempty `L`, select the isolation component with probability

```text
p = 3|L| / (|C| + 3|L|).
```

Select the base component with probability `1-p`. Sampling within the selected
component remains uniform. For a physical permutation `x`, the resulting
proposal density is

```text
q(x) = (1-p)/|C| + I[x in L] p/|L|
     = 1/(|C|+3|L|)                        when x is outside L
     = 4/(|C|+3|L|)                        when x is inside L.
```

The target remains `1/n!`, so the exact importance factors are

```text
outside L: (|C|+3|L|)/n!
inside L:  (|C|+3|L|)/(4 n!).
```

Outside accepted worlds can therefore weigh at most four times inside worlds at
each active mixture site, instead of the unbounded `1+|C|/|L|` ratio produced
by a fixed 50/50 component choice. This does not guarantee root ESS when other
guided epochs also vary, but it bounds the measured land-mixture source of
extreme concentration while retaining more isolation pressure than the 2:1
development prototype.

Component selection is exact integer sampling: draw uniformly below
`|C|+3|L|` and select isolation for the first `3|L|` ranks. No floating-point
selection probability controls proposal behavior. Log-density arithmetic uses
the exact counts before conversion.

## Duplicate copies and fallback

`C`, `L`, component selection, membership, and density remain defined over
distinct physical permutations. Same-name copies stay interchangeable only in
the public constraint; their object IDs remain distinct in exact counts and
unranking.

If `L` is empty or ineligible, sample uniformly from `C` exactly as before and
consume no component-selection draw. If `C` is empty, reject only that attempt.
All current contradiction, replay, all-root accounting, and baseline fallback
behavior remains unchanged.

## Diagnostics and spell-rejection boundary

Keep the new identity-free per-root diagnostics: accepted log-weight range and
median, normalized maximum/top-four mass, 50%/90% mass counts, and aggregate
isolation selected/inside/outside first- and second-moment shares. No card name
or hypothetical object ID is serialized.

Do not add spell isolation in this prototype. Representative leading
`hand_to_stack` roots showed 206/248 hypothetical-extra-cast mismatches, 22/248
different-name cast competitions, 20/248 missing observed casts, and zero
observer-reference novelty. A spell-isolation constraint would therefore be a
new policy-model assumption, not a direct repair of a reference or public-fact
defect.

## Verification and evaluation

1. Exhaustively test exact component-selection frequency and both density
   factors on tiny physical decks, including duplicate names.
2. Preserve support, empty/ineligible fallback RNG behavior, replay, and all
   ordinary package tests.
3. Use seeds 10500--10624 as the development comparison. The fixed 50/50
   baseline covered 37/125. A first 2:1-cap prototype eliminated all 10
   ESS-only failures but reduced acceptance 876 -> 733 and covered 36/125;
   retain that as a rejected development point. Compare the single 4:1-cap
   candidate against both results with the existing
   64-attempt/four-world protocol, one worker, sampler seed 54321,
   `GOMAXPROCS=5`, and `GOMEMLIMIT=5GiB`.
4. If and only if the 4:1 candidate beats both development results, compare it
   once with fixed 50/50 on untouched holdout seeds 10625--10749. Make no
   further probability change from that holdout.
5. Treat coverage and ESS as the primary outcomes; report rejection, time, and
   allocations. Do not infer action quality or tune from the 2026-09-17
   500-root checkpoint.

Do not run race tests or regenerate goldens. Do not commit, push, merge,
rebase, edit a PR, deploy, or promote the prototype without fresh
authorization.

## Development result

Artifacts:

```text
/tmp/gorge-searchprobe-weightcap-baseline-125-20260917.json
/tmp/gorge-searchprobe-weightcap-prototype-125-20260917.json
/tmp/gorge-searchprobe-weightcap4-development-125-20260917.json
```

On seeds 10500--10624, fixed 50/50 covered 37/125 with 876 accepted
proposals and 10 ESS-only fallbacks. The 2:1 cap covered 36/125 with 733
accepted proposals and zero ESS-only fallbacks. The 4:1 cap covered 37/125
with 708 accepted proposals and two ESS-only fallbacks; it gained seven roots
and lost seven against fixed 50/50. The 4:1 candidate therefore did not beat
both development results and did not advance to holdout. Its probability and
density changes were reverted.
