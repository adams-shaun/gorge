# gorge web — spectator client

A Svelte 5 + Vite + TypeScript spectator renderer for the gorge match engine.
It holds no rules knowledge: it renders what the server sends over
`web/src/protocol.ts` (generated from the Go `protocol` package; never hand-edit
it — `make lint` / `go run ./cmd/gentypes -check` fails the build if it drifts)
and the SSE stream in `web/src/lib/stream.ts`. `npm run build` writes to
`../cmd/gorged/webdist`, which `cmd/gorged` embeds and serves.

## Node requirement

**`npm test` (Vitest) requires Node `^22.12.0 || ^24.0.0 || >=26.0.0`.**
`npm run dev` / `npm run build` / `npm run check` / `npm run lint` all work on
Node 20, but `vitest@5` does not start on Node 20 — see below for why this
project is pinned to vitest 5 despite that.

If you're on Node 20 and only need to build/lint (e.g. a CI leg that doesn't
run the JS unit tests), that still works; `make lint`'s `lint-web` (svelte-check
+ eslint) and `make web` (build) do not need vitest. `make test-web` and
`npm test` need Node ≥22.

## Why vitest 5, not vitest 4

vitest 4.1.x declares support for both Node 20 and Vite 8 in its own
`engines`/`peerDependencies` metadata, which would otherwise be the natural
pin for a Node-20 gate machine. In practice, `npm@9.2.0` cannot install it:
resolving vitest 4.1.x's peer set (specifically its `vite: "^6.0.0 || ^7.0.0 ||
^8.0.0"` OR-ranged peer, present from 4.1.0 onward) crashes npm 9.2.0's
arborist with `Cannot read properties of null (reading 'edgesOut')` —
reproduced in complete isolation (a scratch `package.json` with only `vite`
and `vitest` as dependencies, no other packages involved) and confirmed fixed
under `npm@10` (`npx npm@10 install` succeeds with the exact same
`package.json`). `vitest@4.0.x` avoids the crash but does not declare `vite`
as a peer at all and only supports vite `^6.0.0 || ^7.0.0` as a hard
dependency (not 8), and it additionally carries a **critical** published
advisory (GHSA-5xrq-8626-4rwp, arbitrary file read/execute when the Vitest UI
server is listening) that 4.1.0+ fixes — so 4.0.x is not an acceptable
fallback either. `vitest@5.0.0` installs cleanly under npm 9.2.0 (only
EBADENGINE warnings, no install failure) and is what's pinned here.

Net effect: on Node 20 + npm 9.2.0, `npm ci`/`npm install` succeed with
EBADENGINE warnings (documented, not silenced) rather than a hard failure;
`npm run dev`/`build`/`check`/`lint` all work; `npm test` needs Node ≥22.
Revisit this pin once the gate machine's npm is upgraded past 9.2.0, or once
Node 20 support lands for a non-4.0.x vitest release.

## Toolchain versions (produced by `npm create vite@latest -- --template svelte-ts` + `npm install`)

- Svelte 5.57.0
- Vite 8.2.2
- Vitest 5.0.0
- Playwright (`@playwright/test`) 1.63.0
