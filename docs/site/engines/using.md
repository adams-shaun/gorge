# Use the engines

You can run either engine from the command line or from Go. Both need the card corpus once: `make fetch-cards compile-cards`. dirac also needs its tables: `make dirac-tables`.

=== "Command line"

    ```mermaid
    flowchart LR
      forgec --> mtgsim
      forgec --> cardfuzz
      forgec --> kgen --> dplay
      kgen --> kfuzz
      kgen --> sbv2dirac
    ```

    ```sh
    go run -trimpath ./cmd/forgec fetch && go run -trimpath ./cmd/forgec compile
    go run -trimpath ./cmd/mtgsim -games 20 -verify          # gorge, 4 seats
    go run -trimpath ./dirac/cmd/kgen -supported fdn          # which FDN cards dirac plays
    go run -trimpath ./dirac/cmd/dplay -pairs 20 -a hbot -b random
    go run -trimpath ./dirac/oracle/cmd/kfuzz -kernel fdn -pairs 2 -seed 1 -surface autopay
    ```

=== "Go"

    Import `engine/rules` to play a game, `bots/` for a policy and `server/host` to serve tables.
    The [Go embedding guide](../api/go.md) walks each entry point with code that compiles.
    dirac has a value API of its own: build a table with `dirac/cardset`, then loop `Step` and `Apply`
    ([dirac](../architecture/dirac/index.md#why-a-second-engine)).

<div class="grid cards" markdown>

-   [**mtgsim and cardfuzz**](tools-gorge.md)

    Play and fuzz games on gorge.

-   [**kgen, dplay and the oracles**](tools-dirac.md)

    Build dirac's tables, play it, and compare it with gorge.

-   [**Every flag**](flags.md)

    Generated from the source, one table per tool.

-   [**Serve dirac**](../guide/dirac.md)

    Run `gorged -engine dirac`.

</div>
