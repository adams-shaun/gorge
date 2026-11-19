// Copyright 2026 The gorge Authors. Licensed under the Apache License, Version 2.0.
//
// manabrew-bridge: hosts a ManaBrew relay room whose engine is a gorged server.
//
// ManaBrew's client has no "remote engine URL". A multiplayer seat reaches the
// engine through ManaBrew's relay (manabrew-server, WebSocket JSON), and the
// engine runs on the room's host. This process is that host: it signs in to
// the relay as a service session, creates a hosted room, and when the relay
// sends GameStarted it creates the game on gorged (POST /api/games, one human
// seat per relay seat, each with the deck that seat chose in ManaBrew). It then
// opens one manabrewhttp SSE stream per seat and carries the envelopes:
//
//   gorge {kind:state,gameView}       -> relay {kind:state,forPlayer,state:{gameView}}
//   gorge {kind:prompt,promptId,...}  -> relay {kind:prompt,forPlayer,prompt:{promptId,...}}
//   gorge {kind:error,error}          -> relay {kind:error,forPlayer,error}
//   gorge {kind:display,event}        -> relay {kind:display,event}   (seat 0's copy only)
//   relay {kind:response,fromPlayer,promptId,action} -> POST .../send {kind:response,promptId,action}
//   relay {kind:directive,fromPlayer,directive}      -> POST .../send {kind:directive,directive}
//
// The bridge rewrites nothing inside a frame: gorge's ManaBrew wire is meant
// to match ManaBrew's protocol types as they are. Set TRANSCRIPT to log every
// frame both ways as JSONL. See docs/site/guide/manabrew.md.
//
// Node 22 or later (built-in WebSocket and fetch); no dependencies.
import fs from "node:fs";

const env = process.env;
const RELAY = env.RELAY ?? "ws://127.0.0.1:9443";
const RELAY_PASSWORD = env.RELAY_PASSWORD ?? "forge";
const GORGE = (env.GORGE ?? "http://127.0.0.1:8081").replace(/\/$/, "");
const SEATS = Number(env.SEATS ?? "4");
const FORMAT = env.FORMAT ?? "Commander"; // a ManaBrew GameFormat: Commander, Standard, ...
const ROOM_NAME = env.ROOM_NAME ?? "gorge";
const MULLIGANS = Number(env.MULLIGANS ?? "1");
// Fall back to gorge's own pool decks when gorged refuses a seat's deck.
const POOL_FALLBACK = env.POOL_FALLBACK !== "0";
// Optional JSON file {deckName: commanderName}, for seats (such as ManaBrew's
// loadgen) that send no commander_name with their deck.
const COMMANDERS = env.COMMANDERS ? JSON.parse(fs.readFileSync(env.COMMANDERS, "utf8")) : {};
// GATE=1: for pods of seats that start the game themselves as soon as two are
// ready (ManaBrew's loadgen). A second session joins first, never readies, and
// so holds the room until all SEATS seats are ready; then it leaves.
const GATE = env.GATE === "1";
const ONCE = env.ONCE === "1"; // exit after one game

const out = env.TRANSCRIPT ? fs.createWriteStream(env.TRANSCRIPT, { flags: "a" }) : null;
const t0 = Date.now();
function log(dir, msg) {
  out?.write(JSON.stringify({ t: Date.now() - t0, dir, msg }) + "\n");
}
function say(...a) {
  console.log(`[bridge +${((Date.now() - t0) / 1000).toFixed(1)}s]`, ...a);
}

let game = null;
let gate = null;
let roomId = null;

const ws = new WebSocket(RELAY);
function send(msg) {
  log("bridge->relay", msg);
  ws.send(JSON.stringify(msg));
}

ws.addEventListener("open", () => {
  send({ type: "Authenticate", username: `host-${ROOM_NAME}`, password: RELAY_PASSWORD, service: true, client_platform: "unknown" });
  setInterval(() => ws.readyState === 1 && ws.send(JSON.stringify({ type: "Ping" })), 10000);
});
ws.addEventListener("close", (e) => {
  say("relay closed", e.code, e.reason);
  finish(2);
});
ws.addEventListener("error", (e) => say("relay error", e.message ?? e));

ws.addEventListener("message", async (ev) => {
  const msg = JSON.parse(ev.data);
  if (msg.type === "Pong") return;
  log("relay->bridge", msg);
  switch (msg.type) {
    case "AuthResult":
      if (!msg.success) {
        say("relay refused the sign-in:", msg.error);
        return finish(2);
      }
      send({
        type: "CreateRoom",
        room_name: ROOM_NAME,
        max_players: GATE ? SEATS + 1 : SEATS,
        format: FORMAT,
        protocol_version: 5,
        hosted: true,
        engine: "Manabrew",
        reconnect_timeout_s: 90,
      });
      break;
    case "RoomCreated":
      roomId = msg.room_id;
      say(`room "${msg.room_name}" created (${msg.room_id}); gorged at ${GORGE}`);
      if (GATE) openGate();
      break;
    case "RoomUpdate":
      onRoomUpdate(msg.room);
      break;
    case "GameStarted":
      say("game started:", msg.player_order.join(", "));
      startGorgeGame(msg).catch((e) => {
        say("could not start the game on gorged:", e.message);
        send({ type: "BroadcastState", state: { kind: "fatal", message: String(e.message) } });
        send({ type: "EndGame", game_id: msg.game_id });
      });
      break;
    case "StateUpdate":
      await fromSeat(msg.from_player, msg.state);
      break;
    case "Error":
      say("relay error", msg.code, msg.message);
      break;
  }
});

function onRoomUpdate(room) {
  const seats = room.players.filter((p) => p.username !== `gate-${ROOM_NAME}`);
  if (gate && room.status === "Lobby" && seats.length >= SEATS && seats.every((p) => p.ready && p.selected_deck_name)) {
    say("gate: all seats ready, leaving");
    gate.send(JSON.stringify({ type: "LeaveRoom" }));
    gate.close();
    gate = null;
  }
  const who = seats.map((p) => `${p.username}${p.ready ? "*" : ""}${p.selected_deck_name ? ` [${p.selected_deck_name}]` : ""}`);
  say(`room ${room.status}: ${who.join(", ") || "empty"}`);
}

function openGate() {
  const g = new WebSocket(RELAY);
  g.addEventListener("open", () =>
    g.send(JSON.stringify({ type: "Authenticate", username: `gate-${ROOM_NAME}`, password: RELAY_PASSWORD, client_platform: "unknown" })),
  );
  g.addEventListener("message", (ev) => {
    const m = JSON.parse(ev.data);
    if (m.type === "AuthResult" && m.success) {
      g.send(JSON.stringify({ type: "JoinRoom", room_id: roomId, observe: false, as_bot: false }));
      gate = g;
    }
  });
}

// A ManaBrew deck as gorged's ManaBrew adapter reads it: the same cards, a
// commander list, and a format.
function toGorgeDeck(info) {
  const deck = info.deck ?? {};
  const cards = (deck.cards ?? []).map((c, i) => ({ ...c, identity: { id: c.identity?.id || `mb-${i}`, ...c.identity } }));
  const commanderName = info.commander_name ?? COMMANDERS[info.deck_name] ?? COMMANDERS[deck.name];
  const cmd = commanderName ? cards.find((c) => c.identity.name === commanderName) : null;
  const format = FORMAT === "Commander" ? "commander" : "constructed";
  return { ...deck, name: deck.name || info.deck_name, format, cards, commanders: cmd ? [cmd] : [] };
}

async function postGame(body) {
  const res = await fetch(`${GORGE}/api/games`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
  const text = await res.text();
  log("gorge-http", { req: "POST /api/games", status: res.status, body: text.slice(0, 4000) });
  return { status: res.status, text };
}

async function startGorgeGame(gs) {
  const order = gs.player_order;
  const format = FORMAT === "Commander" ? "commander" : "constructed";
  const base = { format, seats: order.length, humans: order.length, mulligans: MULLIGANS };
  const decks = order.map((u, i) => ({ seat: i, deck: toGorgeDeck(gs.player_decks.find((d) => d.username === u) ?? {}) }));
  let r = await postGame({ ...base, decks });
  if (r.status !== 200) {
    say(`gorged refused the seats' decks (${r.status}): ${r.text.slice(0, 600)}`);
    if (!POOL_FALLBACK) throw new Error(`POST /api/games ${r.status}`);
    r = await postGame(base);
    if (r.status !== 200) throw new Error(`POST /api/games ${r.status}: ${r.text}`);
    say("playing gorge's own pool decks instead");
  }
  const resp = JSON.parse(r.text);
  game = { gameId: gs.game_id, order, table: resp.table, match: resp.match, tokens: resp.seats.map((s) => s.token), over: new Set(), aborts: [] };
  say(`gorged table ${resp.table} match ${resp.match}`);
  order.forEach((u, i) => streamSeat(game, i, u));
}

function url(g, kind) {
  return `${GORGE}/api/manabrew/v0/tables/${g.table}/matches/${g.match}/${kind}`;
}

async function streamSeat(g, i, username) {
  const ac = new AbortController();
  g.aborts.push(ac);
  try {
    const res = await fetch(url(g, "stream"), {
      headers: { authorization: `Bearer ${g.tokens[i]}`, accept: "text/event-stream" },
      signal: ac.signal,
    });
    if (res.status !== 200) say(`seat ${i} (${username}): stream ${res.status}`);
    const dec = new TextDecoder();
    let buf = "";
    for await (const chunk of res.body) {
      buf += dec.decode(chunk, { stream: true });
      let idx;
      while ((idx = buf.indexOf("\n\n")) >= 0) {
        const frame = buf.slice(0, idx);
        buf = buf.slice(idx + 2);
        const data = frame
          .split("\n")
          .filter((l) => l.startsWith("data:"))
          .map((l) => l.slice(5).trimStart())
          .join("\n");
        if (!data) continue;
        const m = JSON.parse(data);
        log(`gorge->bridge[player-${i}]`, m);
        fromGorge(g, i, username, m);
      }
    }
  } catch (e) {
    if (!ac.signal.aborted) say(`seat ${i} stream error:`, e.message);
  }
}

function fromGorge(g, i, username, m) {
  const slot = `player-${i}`;
  switch (m.kind) {
    case "state":
      send({ type: "BroadcastState", state: { kind: "state", forPlayer: slot, state: { gameView: m.gameView } }, target_player: username });
      if (m.gameView?.gameOver && !g.over.has(i)) {
        g.over.add(i);
        if (g.over.size === g.order.length) endGame(g, m.gameView);
      }
      break;
    case "prompt": {
      const { kind, ...prompt } = m;
      send({ type: "BroadcastState", state: { kind: "prompt", forPlayer: slot, prompt }, target_player: username });
      break;
    }
    case "error":
      say(`seat ${i} gorge error`, JSON.stringify(m.error));
      send({ type: "BroadcastState", state: { kind: "error", forPlayer: slot, error: m.error }, target_player: username });
      break;
    case "display":
      if (i === 0) send({ type: "BroadcastState", state: { kind: "display", event: m.event } });
      break;
    default:
      say(`seat ${i}: unknown gorge frame kind ${m.kind}`);
  }
}

async function fromSeat(username, envl) {
  const g = game;
  if (!g) return;
  const i = g.order.indexOf(username);
  if (i < 0) return;
  let body;
  if (envl.kind === "response") body = { kind: "response", promptId: envl.promptId, action: envl.action };
  else if (envl.kind === "directive") body = { kind: "directive", directive: envl.directive };
  else return;
  const res = await fetch(url(g, "send"), {
    method: "POST",
    headers: { authorization: `Bearer ${g.tokens[i]}`, "content-type": "application/json" },
    body: JSON.stringify(body),
  });
  const text = await res.text();
  log(`gorge-http[player-${i}]`, { req: body, status: res.status, body: text.slice(0, 2000) });
  if (res.status >= 300) say(`seat ${i}: gorged refused a ${body.kind} (${res.status}): ${text.slice(0, 300)}`);
}

function endGame(g, view) {
  send({ type: "ReportGameOutcome", game_id: g.gameId, outcome: { game_over: true, winner_slot: view.winnerId ?? undefined, turns: view.turn } });
  send({ type: "EndGame", game_id: g.gameId });
  say(`game over on turn ${view.turn}; winner ${view.winnerId ?? "none"}`);
  setTimeout(() => {
    g.aborts.forEach((a) => a.abort());
    if (game === g) game = null;
    if (ONCE) finish(0);
  }, 3000);
}

function finish(code) {
  if (game) game.aborts.forEach((a) => a.abort());
  if (out) out.end(() => process.exit(code));
  else process.exit(code);
}
process.on("SIGINT", () => finish(130));
process.on("SIGTERM", () => finish(143));
