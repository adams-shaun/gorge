// A gorge seat played over HTTP: Java 21, one file, no dependencies.
//
// The engine asks a decision; the agent answers with option indices. gorge
// lists every legal option, validates every answer and refuses (HTTP 409)
// anything it did not offer, so a bot can choose badly but never illegally.
//
// Run it against a gorged started with -vsbot (it creates its own game):
//
//     java Agent.java --server http://127.0.0.1:8080 --policy simple
//
// or join a seat you already hold (gorged -humans 0 -seat-token TOKEN):
//
//     java Agent.java --table t1 --match 1 --seat 0 --token TOKEN
//
// To write your own strategy, implement Policy.choose; see RandomPolicy and
// SimplePolicy below.

import java.io.IOException;
import java.math.BigInteger;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.*;

public class Agent {

    // ----------------------------------------------------------------------
    // The HTTP seat client: four endpoints.
    // ----------------------------------------------------------------------

    record Reply(int status, Object body) {}

    static final class SeatClient {
        final String server, table, token;
        final int match, seat;
        final HttpClient http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(10)).build();

        SeatClient(String server, String table, int match, int seat, String token) {
            this.server = server.replaceAll("/+$", "");
            this.table = table; this.match = match; this.seat = seat; this.token = token;
        }

        Reply call(String method, String path, Object body) throws IOException, InterruptedException {
            HttpRequest.Builder b = HttpRequest.newBuilder(URI.create(server + path))
                    .timeout(Duration.ofSeconds(30));
            if (token != null) b.header("Authorization", "Bearer " + token);
            if (body == null) {
                b.method(method, HttpRequest.BodyPublishers.noBody());
            } else {
                b.header("Content-Type", "application/json")
                 .method(method, HttpRequest.BodyPublishers.ofString(Json.write(body)));
            }
            HttpResponse<String> r = http.send(b.build(), HttpResponse.BodyHandlers.ofString());
            return new Reply(r.statusCode(), r.body().isEmpty() ? null : Json.parse(r.body()));
        }

        String matchPath(String tail) {
            return "/api/tables/" + table + "/matches/" + match + "/" + tail;
        }

        /** The decision asked of this seat, or null (409: nothing pending). */
        Map<String, Object> pending() throws IOException, InterruptedException {
            Reply r = call("GET", matchPath("pending?seat=" + seat), null);
            if (r.status == 200) return Json.obj(r.body);
            if (r.status == 409) return null;
            throw new IOException("pending: HTTP " + r.status + " " + Json.write(r.body));
        }

        /** This seat's redacted view of the game (own hand, public board). */
        Map<String, Object> view() throws IOException, InterruptedException {
            Reply r = call("GET", matchPath("view?seat=" + seat), null);
            if (r.status != 200) throw new IOException("view: HTTP " + r.status + " " + Json.write(r.body));
            return Json.obj(r.body);
        }

        /** POST an intent; null on success (204), else the error body. */
        Map<String, Object> submit(Map<String, Object> d, Answer a) throws IOException, InterruptedException {
            Map<String, Object> intent = new LinkedHashMap<>();
            intent.put("seq", d.get("seq"));
            intent.put("player", d.get("player"));
            if (a.payment != null) intent.put("payment", a.payment);
            else intent.put("choices", a.choices);
            Reply r = call("POST", matchPath("intent"), intent);
            if (r.status == 204) return null;
            return r.body == null ? Map.of("code", String.valueOf(r.status)) : Json.obj(r.body);
        }

        /** This match's row of GET /api/tables/{t}/matches, or null. */
        Map<String, Object> matchState() throws IOException, InterruptedException {
            Reply r = call("GET", "/api/tables/" + table + "/matches", null);
            if (r.status == 404) return null;
            if (r.status != 200) throw new IOException("matches: HTTP " + r.status);
            for (Object row : Json.arr(r.body)) {
                if (Json.num(Json.obj(row).get("match")) == match) return Json.obj(row);
            }
            return null;
        }
    }

    // ----------------------------------------------------------------------
    // Answers and the wire rules every answer must meet.
    // ----------------------------------------------------------------------

    /** Either option indices or a plan-paid cast. */
    record Answer(List<Integer> choices, Map<String, Object> payment) {
        static Answer of(List<Integer> c) { return new Answer(c, null); }
        static Answer pay(Map<String, Object> action) {
            Map<String, Object> p = new LinkedHashMap<>();
            p.put("action_id", action.get("id"));
            p.put("plan", Json.arr(action.get("plans")).get(0)); // echoed verbatim
            return new Answer(null, p);
        }
        public String toString() { return payment != null ? "pay plan" : choices.toString(); }
    }

    static List<Map<String, Object>> options(Map<String, Object> d) {
        List<Map<String, Object>> out = new ArrayList<>();
        for (Object o : Json.arr(d.get("options"))) out.add(Json.obj(o));
        return out;
    }

    static List<Map<String, Object>> paymentActions(Map<String, Object> d) {
        List<Map<String, Object>> out = new ArrayList<>();
        Object pa = d.get("payment_actions");
        if (pa == null) return out;
        for (Object a : Json.arr(pa)) {
            if (!Json.arr(Json.obj(a).get("plans")).isEmpty()) out.add(Json.obj(a));
        }
        return out;
    }

    static int idx(Map<String, Object> o) { return (int) Json.num(o.get("index")); }
    static String kind(Map<String, Object> o) { return (String) o.get("kind"); }
    static long objOf(Map<String, Object> o) { return o.get("obj") == null ? 0 : Json.num(o.get("obj")); }

    /**
     * The keys of which an answer may hold at most one option: the option's
     * Group, and on an attackers decision its creature (one creature attacks
     * one defender; the wire does not mark that with a Group, and an answer
     * that breaks it is accepted by the HTTP layer but refused by the rules,
     * which ends the match, so a client must enforce it itself).
     */
    static List<String> exclusiveKeys(Map<String, Object> d, Map<String, Object> o) {
        List<String> keys = new ArrayList<>();
        if (o.get("group") != null) keys.add("group:" + o.get("group"));
        if ("attackers".equals(d.get("kind")) && o.get("obj") != null) keys.add("attacker:" + o.get("obj"));
        return keys;
    }

    /** Random answer within Min/Max honouring required options and the exclusivity keys. */
    static List<Integer> legalRandom(Map<String, Object> d, Random rng, List<Map<String, Object>> pool) {
        if (pool == null) pool = options(d);
        List<Integer> picked = new ArrayList<>();
        Set<String> taken = new HashSet<>();
        List<Map<String, Object>> rest = new ArrayList<>();
        for (Map<String, Object> o : pool) {
            if ("concede".equals(kind(o))) continue;
            if (Boolean.TRUE.equals(o.get("required"))) {
                picked.add(idx(o));
                taken.addAll(exclusiveKeys(d, o));
            } else rest.add(o);
        }
        Collections.shuffle(rest, rng);
        int min = (int) Json.num(d.get("min")), max = (int) Json.num(d.get("max"));
        int want = min + rng.nextInt(Math.max(1, max - min + 1));
        for (Map<String, Object> o : rest) {
            if (picked.size() >= want) break;
            List<String> keys = exclusiveKeys(d, o);
            if (keys.stream().anyMatch(taken::contains)) continue;
            picked.add(idx(o));
            taken.addAll(keys);
        }
        String k = (String) d.get("kind");
        if (k.equals("trigger_order") || k.equals("arrange")) Collections.shuffle(picked, rng);
        return picked;
    }

    /** The least committal answer: pass at priority, else the first Min. */
    static List<Integer> minimal(Map<String, Object> d) {
        List<Map<String, Object>> opts = options(d);
        if ("priority".equals(d.get("kind"))) {
            for (Map<String, Object> o : opts) if ("pass".equals(kind(o))) return List.of(idx(o));
        }
        List<Integer> out = new ArrayList<>();
        int min = (int) Json.num(d.get("min"));
        for (int i = 0; i < min && i < opts.size(); i++) out.add(idx(opts.get(i)));
        return out;
    }

    /** Mana value from the view's cost string, e.g. "1 W W" -> 3. */
    static int cmc(Map<String, Object> card) {
        if (card == null || card.get("mana_cost") == null) return 0;
        int total = 0;
        for (String s : ((String) card.get("mana_cost")).split(" ")) {
            if (s.matches("\\d+")) total += Integer.parseInt(s);
            else if (!s.isEmpty() && !s.equals("no") && !s.equals("cost") && !s.equals("X")) total++;
        }
        return total;
    }

    // ----------------------------------------------------------------------
    // Strategies. Implement choose(); the loop tries fallbacks on a 409.
    // ----------------------------------------------------------------------

    interface Policy {
        boolean needsView();
        Answer choose(Map<String, Object> d, Map<String, Object> view, Random rng);
    }

    /** Uniform over legal answers; never concedes; passes priority half the time. */
    static final class RandomPolicy implements Policy {
        public boolean needsView() { return false; }
        public Answer choose(Map<String, Object> d, Map<String, Object> view, Random rng) {
            if ("priority".equals(d.get("kind"))) {
                List<Answer> plays = new ArrayList<>();
                for (Map<String, Object> a : paymentActions(d)) plays.add(Answer.pay(a));
                for (Map<String, Object> o : options(d)) {
                    if (!"pass".equals(kind(o)) && !"concede".equals(kind(o))) plays.add(Answer.of(List.of(idx(o))));
                }
                if (plays.isEmpty() || rng.nextBoolean()) return Answer.of(minimal(d));
                return plays.get(rng.nextInt(plays.size()));
            }
            return Answer.of(legalRandom(d, rng, null));
        }
    }

    /** Land first, then the most expensive castable spell; safe attacks and blocks only. */
    static final class SimplePolicy implements Policy {
        public boolean needsView() { return true; }
        public Answer choose(Map<String, Object> d, Map<String, Object> view, Random rng) {
            long me = Json.num(d.get("player"));
            Map<Long, Map<String, Object>> cards = new HashMap<>();
            List<Map<String, Object>> enemy = new ArrayList<>();
            for (Object po : Json.arr(view.get("players"))) {
                Map<String, Object> p = Json.obj(po);
                for (String zone : List.of("hand", "battlefield")) {
                    if (p.get(zone) == null) continue;
                    for (Object co : Json.arr(p.get(zone))) {
                        Map<String, Object> c = Json.obj(co);
                        cards.put(Json.num(c.get("id")), c);
                        if (zone.equals("battlefield") && Json.num(p.get("seat")) != me
                                && ((String) c.get("types")).contains("Creature")
                                && !Boolean.TRUE.equals(c.get("tapped"))) enemy.add(c);
                    }
                }
            }
            List<Map<String, Object>> opts = options(d);
            switch ((String) d.get("kind")) {
                case "mulligan": {
                    for (Map<String, Object> o : opts) if ("keep".equals(kind(o))) return Answer.of(List.of(idx(o)));
                    return Answer.of(legalRandom(d, rng, null)); // bottoming: any cards
                }
                case "starting_player": {
                    for (Map<String, Object> o : opts) if (Json.num(o.get("player")) == me) return Answer.of(List.of(idx(o)));
                    break;
                }
                case "priority": {
                    for (Map<String, Object> o : opts) if ("play_land".equals(kind(o))) return Answer.of(List.of(idx(o)));
                    Answer best = null;
                    int bestCmc = -1;
                    for (Map<String, Object> a : paymentActions(d)) {
                        int c = cmc(cards.get(Json.num(Json.obj(a.get("cast")).get("object"))));
                        if (c > bestCmc) { bestCmc = c; best = Answer.pay(a); }
                    }
                    for (Map<String, Object> o : opts) {
                        if (!"cast".equals(kind(o))) continue;
                        int c = cmc(cards.get(objOf(o)));
                        if (c > bestCmc) { bestCmc = c; best = Answer.of(List.of(idx(o))); }
                    }
                    return best != null ? best : Answer.of(minimal(d));
                }
                case "attackers": {
                    long biggest = 0;
                    for (Map<String, Object> c : enemy) biggest = Math.max(biggest, Json.num(c.get("power")));
                    List<Integer> picked = new ArrayList<>();
                    Set<String> taken = new HashSet<>();
                    for (Map<String, Object> o : opts) {
                        Map<String, Object> c = cards.get(objOf(o));
                        boolean safe = c != null && Json.num(c.get("toughness")) > biggest;
                        List<String> keys = exclusiveKeys(d, o);
                        if ((Boolean.TRUE.equals(o.get("required")) || safe) && keys.stream().noneMatch(taken::contains)) {
                            picked.add(idx(o));
                            taken.addAll(keys);
                        }
                    }
                    return Answer.of(picked);
                }
                case "blockers": {
                    List<Integer> picked = new ArrayList<>();
                    Set<Object> groups = new HashSet<>(), used = new HashSet<>();
                    for (Map<String, Object> o : opts) {
                        Map<String, Object> blocker = cards.get(objOf(o));
                        Map<String, Object> attacker = o.get("attacker") == null ? null : cards.get(Json.num(o.get("attacker")));
                        Object g = o.get("group");
                        if (blocker != null && attacker != null
                                && Json.num(blocker.get("toughness")) > Json.num(attacker.get("power"))
                                && !used.contains(o.get("attacker")) && !(g != null && groups.contains(g))) {
                            picked.add(idx(o));
                            used.add(o.get("attacker"));
                            if (g != null) groups.add(g);
                        }
                    }
                    return Answer.of(picked);
                }
                case "target": {
                    List<Map<String, Object>> theirs = new ArrayList<>();
                    for (Map<String, Object> o : opts) if (Json.num(o.get("player")) != me) theirs.add(o);
                    boolean enough = theirs.size() >= Json.num(d.get("min"));
                    return Answer.of(legalRandom(d, rng, enough ? theirs : opts));
                }
                case "trigger_optional": {
                    for (Map<String, Object> o : opts) if ("yes".equals(kind(o))) return Answer.of(List.of(idx(o)));
                    break;
                }
                default:
                    break;
            }
            return Answer.of(legalRandom(d, rng, null));
        }
    }

    // ----------------------------------------------------------------------
    // The loop: poll, decide, submit, repeat until the match is over.
    // ----------------------------------------------------------------------

    public static void main(String[] argv) throws Exception {
        Map<String, String> a = new HashMap<>(Map.of("--server", "http://127.0.0.1:8080",
                "--policy", "simple", "--seed", "1", "--format", "constructed",
                "--match", "1", "--seat", "0"));
        Set<String> flags = new HashSet<>();
        for (int i = 0; i < argv.length; i++) {
            if (argv[i].equals("-v")) flags.add("-v");
            else a.put(argv[i], argv[++i]);
        }
        boolean verbose = flags.contains("-v");
        SeatClient client;
        if (a.containsKey("--table")) {
            client = new SeatClient(a.get("--server"), a.get("--table"), Integer.parseInt(a.get("--match")),
                    Integer.parseInt(a.get("--seat")), a.get("--token"));
        } else {
            Map<String, Object> body = new LinkedHashMap<>();
            body.put("format", a.get("--format"));
            if (a.containsKey("--bot-policy")) body.put("bot_policy", a.get("--bot-policy"));
            if (a.containsKey("--deck")) body.put("human_deck", a.get("--deck"));
            if (a.containsKey("--bot-deck")) body.put("bot_deck", a.get("--bot-deck"));
            Reply r = new SeatClient(a.get("--server"), "", 0, 0, null).call("POST", "/api/games", body);
            if (r.status != 200) throw new IOException("create game: HTTP " + r.status + " " + Json.write(r.body));
            Map<String, Object> g = Json.obj(r.body);
            System.out.printf("created table %s match %d, seat %d vs %s%n", g.get("table"),
                    Json.num(g.get("match")), Json.num(g.get("seat")), g.get("bot_policy"));
            client = new SeatClient(a.get("--server"), (String) g.get("table"), (int) Json.num(g.get("match")),
                    (int) Json.num(g.get("seat")), (String) g.get("token"));
        }
        Policy policy = a.get("--policy").equals("random") ? new RandomPolicy() : new SimplePolicy();
        Random rng = new Random(Long.parseLong(a.get("--seed")));

        // A fresh table's match goes live a moment after POST /api/games returns.
        long until = System.nanoTime() + 10_000_000_000L;
        while (client.matchState() == null) {
            if (System.nanoTime() > until) throw new IOException("match never appeared");
            Thread.sleep(20);
        }

        long t0 = System.nanoTime();
        int decisions = 0, refused = 0, idle = 0;
        long sleepMs = 2;
        Map<String, Integer> byKind = new TreeMap<>();
        List<Double> rtt = new ArrayList<>();
        Map<String, Object> row;
        long answered = -1; // seq of the last decision we answered
        while (true) {
            Map<String, Object> d = client.pending();
            // Right after a 204 the server can still show the decision just
            // answered for a moment; never answer the same seq twice.
            if (d != null && Json.num(d.get("seq")) <= answered) d = null;
            if (d == null) {
                idle++;
                if (idle % 10 == 0) {
                    row = client.matchState();
                    if (row != null && !"live".equals(row.get("state"))) break;
                }
                Thread.sleep(sleepMs);
                sleepMs = Math.min(sleepMs * 2, 50);
                continue;
            }
            idle = 0; sleepMs = 2;
            Map<String, Object> view = policy.needsView() ? client.view() : null;
            List<Answer> tries = new ArrayList<>();
            tries.add(policy.choose(d, view, rng));
            for (int i = 0; i < 8; i++) tries.add(Answer.of(legalRandom(d, rng, null)));
            tries.add(Answer.of(minimal(d)));
            Answer sent = null;
            boolean accepted = false;
            for (Answer ans : tries) {
                long s = System.nanoTime();
                Map<String, Object> err = client.submit(d, ans);
                rtt.add((System.nanoTime() - s) / 1e6);
                sent = ans;
                if (err == null) { accepted = true; break; }
                refused++;
                if (String.valueOf(err.get("message")).contains("pending decision seq")) break; // moved on
            }
            if (!accepted) continue; // nothing was admitted: read the decision again
            answered = Json.num(d.get("seq"));
            decisions++;
            byKind.merge((String) d.get("kind"), 1, Integer::sum);
            if (verbose) System.out.printf("seq %-5d %-16s -> %s%n", Json.num(d.get("seq")), d.get("kind"), sent);
        }
        double wall = (System.nanoTime() - t0) / 1e9;
        Collections.sort(rtt);
        System.out.printf("decisions answered: %d (%d answers refused and retried)%n", decisions, refused);
        StringJoiner sj = new StringJoiner(", ");
        byKind.forEach((k, v) -> sj.add(k + "=" + v));
        System.out.println("by kind: " + sj);
        if (!rtt.isEmpty()) System.out.printf("intent POST round trip: median %.2f ms, p95 %.2f ms%n",
                rtt.get(rtt.size() / 2), rtt.get((int) (rtt.size() * 0.95)));
        Object w = row.get("winner");
        String who = "draw";
        if (w != null) {
            int ws = (int) Json.num(w);
            Map<String, Object> seatRow = Json.obj(Json.arr(row.get("seats")).get(ws));
            who = String.format("%s (seat %d, %s)", ws == client.seat ? "us" : "opponent", ws, seatRow.get("deck"));
        }
        System.out.printf("match %s after %d turns in %.1f s: winner %s%n", row.get("state"),
                Json.num(row.get("turns")), wall, who);
    }

    // ----------------------------------------------------------------------
    // A minimal JSON reader/writer: objects keep key order so a payment plan
    // is echoed back exactly as the server sent it.
    // ----------------------------------------------------------------------

    static final class Json {
        private final String s;
        private int i;
        private Json(String s) { this.s = s; }

        static Object parse(String text) {
            Json p = new Json(text);
            Object v = p.value();
            p.ws();
            if (p.i != text.length()) throw new IllegalArgumentException("trailing JSON at " + p.i);
            return v;
        }

        @SuppressWarnings("unchecked")
        static Map<String, Object> obj(Object o) { return (Map<String, Object>) o; }
        @SuppressWarnings("unchecked")
        static List<Object> arr(Object o) { return (List<Object>) o; }
        static long num(Object o) { return ((Number) o).longValue(); }

        private void ws() { while (i < s.length() && Character.isWhitespace(s.charAt(i))) i++; }

        private Object value() {
            ws();
            char c = s.charAt(i);
            switch (c) {
                case '{': {
                    i++;
                    Map<String, Object> m = new LinkedHashMap<>();
                    ws();
                    if (s.charAt(i) == '}') { i++; return m; }
                    while (true) {
                        ws();
                        String k = string();
                        ws(); expect(':');
                        m.put(k, value());
                        ws();
                        if (s.charAt(i) == ',') { i++; continue; }
                        expect('}');
                        return m;
                    }
                }
                case '[': {
                    i++;
                    List<Object> l = new ArrayList<>();
                    ws();
                    if (s.charAt(i) == ']') { i++; return l; }
                    while (true) {
                        l.add(value());
                        ws();
                        if (s.charAt(i) == ',') { i++; continue; }
                        expect(']');
                        return l;
                    }
                }
                case '"': return string();
                case 't': i += 4; return Boolean.TRUE;
                case 'f': i += 5; return Boolean.FALSE;
                case 'n': i += 4; return null;
                default: return number();
            }
        }

        private void expect(char c) {
            if (s.charAt(i) != c) throw new IllegalArgumentException("want '" + c + "' at " + i);
            i++;
        }

        private String string() {
            expect('"');
            StringBuilder b = new StringBuilder();
            while (true) {
                char c = s.charAt(i++);
                if (c == '"') return b.toString();
                if (c != '\\') { b.append(c); continue; }
                char e = s.charAt(i++);
                switch (e) {
                    case 'n' -> b.append('\n');
                    case 't' -> b.append('\t');
                    case 'r' -> b.append('\r');
                    case 'b' -> b.append('\b');
                    case 'f' -> b.append('\f');
                    case 'u' -> { b.append((char) Integer.parseInt(s.substring(i, i + 4), 16)); i += 4; }
                    default -> b.append(e);
                }
            }
        }

        private Number number() {
            int start = i;
            while (i < s.length() && "+-0123456789.eE".indexOf(s.charAt(i)) >= 0) i++;
            String t = s.substring(start, i);
            if (t.contains(".") || t.contains("e") || t.contains("E")) return Double.parseDouble(t);
            BigInteger big = new BigInteger(t); // seeds are uint64
            return big.bitLength() < 64 ? (Number) big.longValue() : big;
        }

        static String write(Object v) {
            StringBuilder b = new StringBuilder();
            write(b, v);
            return b.toString();
        }

        private static void write(StringBuilder b, Object v) {
            if (v == null) b.append("null");
            else if (v instanceof String str) {
                b.append('"');
                for (char c : str.toCharArray()) {
                    if (c == '"' || c == '\\') b.append('\\').append(c);
                    else if (c < 0x20) b.append(String.format("\\u%04x", (int) c));
                    else b.append(c);
                }
                b.append('"');
            } else if (v instanceof Map<?, ?> m) {
                b.append('{');
                boolean first = true;
                for (Map.Entry<?, ?> e : m.entrySet()) {
                    if (!first) b.append(',');
                    first = false;
                    write(b, String.valueOf(e.getKey()));
                    b.append(':');
                    write(b, e.getValue());
                }
                b.append('}');
            } else if (v instanceof List<?> l) {
                b.append('[');
                for (int k = 0; k < l.size(); k++) {
                    if (k > 0) b.append(',');
                    write(b, l.get(k));
                }
                b.append(']');
            } else b.append(v); // Number or Boolean
        }
    }
}
