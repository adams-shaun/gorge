package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/gorged"
)

// deployScript is the demo deploy, relative to this package (cmd/gorged).
var deployScript = filepath.Join("..", "..", "scripts", "deploy-demo.sh")

// emptyDemoEnv is a demo.env holding nothing, for DEMO_ENV: a test deploy must
// never read the checkout's own deploy/demo.env, whose ports and paths are the
// live demo's.
func emptyDemoEnv(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "demo.env")
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// flagValue is fs's parsed value of the named flag, as its String form.
func flagValue(t *testing.T, fs *flag.FlagSet, name string) string {
	t.Helper()
	f := fs.Lookup(name)
	if f == nil {
		t.Fatalf("gorged has no -%s flag", name)
	}
	return f.Value.String()
}

// deployStub is a stand-in gorged for scripts/deploy-demo.sh: each call
// appends one record line (its kind, then every argument, \x1f-separated) to
// the record file. The -prewarm-art-only call then fails or hangs as the case
// asks; a server call just exits, because the test's own listeners answer
// the script's readiness and format probes on the demo ports.
const deployStub = `#!/usr/bin/env bash
kind=serve
for a in "$@"; do [ "$a" = -prewarm-art-only ] && kind=fill; done
line=$kind
for a in "$@"; do line+=$'\x1f'"$a"; done
printf '%s\n' "$line" >>'@REC@'
if [ "$kind" = fill ]; then
	case '@MODE@' in
	fail) echo 'gorged: ART FILL INCOMPLETE (stub)' >&2; exit 1 ;;
	hang) exec sleep 60 ;;
	esac
fi
exit 0
`

// TestDeployStartsTheServersWhenTheArtFillFails: art is cosmetic, so the
// real script, driven with a stub binary, must start BOTH servers after a
// pre-start art fill that fails and after one that hangs past the hard
// ceiling, say so loudly, pass the fill a budget the real flag set accepts,
// and keep that budget and the ceiling at or under 240s. A deploy that aborted
// on a failed fill once kept new code off the demo, and an unbounded fill held
// the deploy for hours.
//
// Safety: the listeners are this test's own, bound on free ports in 8090-8099
// (a successful bind proves no gorged is there for the script's port sweep to
// stop), and every path the script writes — persistence dirs, logs, art dir —
// is overridden into temp dirs. The test passes DEMO_PORT_HISTORY= (empty
// clears the box's port history) and an empty DEMO_ENV, so the sweep covers
// only this run's ports and never reaches a live demo's server. A probe
// gorged (the test binary re-exec'd from a copy literally named gorged)
// listens on [::1] of the first port through every subtest: the sweep must
// leave a gorged bound to ANOTHER address of a demo port alone.
func TestDeployStartsTheServersWhenTheArtFillFails(t *testing.T) {
	for _, tool := range []string{"bash", "setsid", "nohup", "ss", "curl"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("deploy-demo.sh needs %s: %v", tool, err)
		}
	}
	script, err := filepath.Abs(deployScript)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, mode string
		env        []string
		wantExit   string
	}{
		{name: "fill exits non-zero", mode: "fail", wantExit: "exit 1;"},
		// Any non-zero status: GNU timeout says 124, uutils says 125. What
		// must hold under both is that the deploy moved on inside the
		// ceiling instead of waiting out the stub's 60s hang.
		{name: "fill hangs past the hard ceiling", mode: "hang", env: []string{"ART_FILL_CEILING=0.3s"}, wantExit: "exit "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ports := demoTestPorts(t)
			probe := startGorgedProbe(t, ports[0])
			tmp := t.TempDir()
			rec := filepath.Join(tmp, "calls")
			stub := filepath.Join(tmp, "gorged")
			body := strings.NewReplacer("@REC@", rec, "@MODE@", tc.mode).Replace(deployStub)
			if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
				t.Fatal(err)
			}
			artDir := filepath.Join(tmp, "art")
			outPath := filepath.Join(tmp, "deploy.out")
			outFile, err := os.Create(outPath)
			if err != nil {
				t.Fatal(err)
			}
			defer outFile.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", script)
			cmd.Dir = tmp
			cmd.Stdout, cmd.Stderr = outFile, outFile
			cmd.Env = append(os.Environ(),
				// DEMO_PORT_HISTORY= and an empty DEMO_ENV keep the sweep off
				// any live demo's ports; RETIRED_PORTS= is the one-off caller
				// override, empty here. OMNI_PORT is supplied, and the script
				// must honour it with the second server.
				"DEMO_ENV="+emptyDemoEnv(t), "BIN="+stub, "DECKS="+filepath.Join(tmp, "decks"), "ART_DIR="+artDir, "SWEEP=ports", "RETIRED_PORTS=", "DEMO_PORT_HISTORY=",
				fmt.Sprintf("PUB_PORT=%d", ports[0]), fmt.Sprintf("OMNI_PORT=%d", ports[1]),
				"PUB_DIR="+filepath.Join(tmp, "pub"), "OMNI_DIR="+filepath.Join(tmp, "omni"),
				"PUB_LOG="+filepath.Join(tmp, "pub.log"), "OMNI_LOG="+filepath.Join(tmp, "omni.log"))
			cmd.Env = append(cmd.Env, tc.env...)
			start := time.Now()
			runErr := cmd.Run()
			elapsed := time.Since(start)
			out, _ := os.ReadFile(outPath)
			t.Logf("deploy-demo.sh output:\n%s", out)
			if runErr != nil {
				t.Fatalf("deploy-demo.sh: %v — a failed art fill must not fail the deploy", runErr)
			}
			if elapsed > 20*time.Second {
				t.Errorf("deploy took %v: the fill was not cut off at its ceiling", elapsed)
			}
			for _, want := range []string{"art fill INCOMPLETE (" + tc.wantExit, "starting the servers anyway", "ready — spectator"} {
				if !strings.Contains(string(out), want) {
					t.Errorf("deploy output lacks %q", want)
				}
			}
			if tc.env == nil {
				m := regexp.MustCompile(`\(budget (\S+), ceiling (\S+)\)`).FindStringSubmatch(string(out))
				if m == nil {
					t.Fatal("deploy output does not state the fill's budget and ceiling")
				}
				for _, v := range m[1:] {
					if d, err := time.ParseDuration(v); err != nil || d <= 0 || d > 240*time.Second {
						t.Errorf("default fill bound %q: want a duration in (0, 240s] — under the hook's 600s lock wait", v)
					}
				}
			}
			assertGorgedProbeListening(t, probe)

			// The stub servers are started in the background: wait for both.
			var calls [][]string
			deadline := time.Now().Add(10 * time.Second)
			for {
				calls = readDeployCalls(t, rec)
				if len(calls) >= 3 || time.Now().After(deadline) {
					break
				}
				time.Sleep(pollEvery)
			}
			if len(calls) != 3 || calls[0][0] != "fill" || calls[1][0] != "serve" || calls[2][0] != "serve" {
				t.Fatalf("stub calls = %q, want the fill, then both servers", calls)
			}
			served := strings.Join(append(calls[1][1:], calls[2][1:]...), " ")
			for _, p := range ports {
				if !strings.Contains(served, fmt.Sprintf("127.0.0.1:%d", p)) {
					t.Errorf("no server was started on port %d after the failed fill: %q", p, served)
				}
			}

			// The fill's arguments must be ones the REAL binary accepts:
			// a misspelt flag would exit 2 on every deploy, now silently.
			fs := gorged.ServeFlagSet() // ContinueOnError: an unknown flag must fail this test, not os.Exit the binary
			if err := fs.Parse(calls[0][1:]); err != nil {
				t.Fatalf("real gorged rejects the deploy's fill arguments %q: %v", calls[0][1:], err)
			}
			if flagValue(t, fs, "prewarm-art-only") != "true" || flagValue(t, fs, "art-dir") != artDir {
				t.Errorf("fill flags = only %s art-dir %q, want the one-shot into %q", flagValue(t, fs, "prewarm-art-only"), flagValue(t, fs, "art-dir"), artDir)
			}
			if d, err := time.ParseDuration(flagValue(t, fs, "prewarm-art-budget")); err != nil || d <= 0 || d > 240*time.Second {
				t.Errorf("-prewarm-art-budget = %s, want in (0, 240s]", flagValue(t, fs, "prewarm-art-budget"))
			}
			if n, err := strconv.Atoi(flagValue(t, fs, "prewarm-art-max-consecutive-failures")); err != nil || n <= 0 {
				t.Errorf("-prewarm-art-max-consecutive-failures = %s, want a positive streak limit", flagValue(t, fs, "prewarm-art-max-consecutive-failures"))
			}

			// The demo server's arguments too: the real flag set must accept
			// them, and the demo's defaults must be the ones it serves --
			// bot tables omniscient, a person's vs-bot table public.
			fs = gorged.ServeFlagSet()
			if err := fs.Parse(calls[1][1:]); err != nil {
				t.Fatalf("real gorged rejects the deploy's serve arguments %q: %v", calls[1][1:], err)
			}
			// The bot policy is read from the script rather than pinned here:
			// what this test is for is that the REAL flag set accepts what the
			// script passes, not which policy the demo's owner picked.
			wantPolicy := deployScriptDefault(t, "BOT_POLICY")
			spectator, vsbotSpectator, botPolicy, pace := flagValue(t, fs, "spectator"), flagValue(t, fs, "vsbot-spectator"), flagValue(t, fs, "bot-policy"), flagValue(t, fs, "pace")
			if spectator != "omniscient" || vsbotSpectator != "public" || botPolicy != wantPolicy || pace != "50ms" {
				t.Errorf("demo server: spectator %q vsbot-spectator %q bot-policy %q pace %v, want omniscient/public/%s/50ms",
					spectator, vsbotSpectator, botPolicy, pace, wantPolicy)
			}
			vsbot := flagValue(t, fs, "vsbot")
			if got, want := flagValue(t, fs, "tables")+"/"+flagValue(t, fs, "seats")+"/"+flagValue(t, fs, "commander-seats"),
				deployScriptDefault(t, "TABLES")+"/"+deployScriptDefault(t, "SEATS")+"/"+deployScriptDefault(t, "COMMANDER_SEATS"); got != want || vsbot != "true" {
				t.Errorf("demo server: tables/seats/commander-seats %s vsbot %s, want the script's %s with vsbot on", got, vsbot, want)
			}
		})
	}
}

// deployScriptDefault reads one `NAME=${NAME:-value}` default out of
// scripts/deploy-demo.sh, so a test asserting what the deploy passes tracks the
// script instead of duplicating a value its owner changes by hand.
func deployScriptDefault(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(deployScript)
	if err != nil {
		t.Fatalf("read deploy-demo.sh: %v", err)
	}
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `=\$\{` + regexp.QuoteMeta(name) + `:-([^}]*)\}`)
	m := re.FindSubmatch(b)
	if m == nil {
		t.Fatalf("scripts/deploy-demo.sh has no %s=${%s:-...} default", name, name)
	}
	return string(m[1])
}

// demoTestPorts reserves every IPv4 candidate while it scans the task range,
// then chooses two and answers the deploy script's /api/tables probes on them
// until the test ends. The first port must also accept a temporary [::1] bind,
// because startGorgedProbe will use that address. Thus a review server already
// on [::1]:8090 is treated as a busy candidate rather than becoming a 10-second
// helper failure. It SKIPS — never fails — when no dual-address first port or
// fewer than two IPv4 ports are free: other sessions use this range by design.
func demoTestPorts(t *testing.T) []int {
	t.Helper()
	type candidate struct {
		port int
		ln   net.Listener
		v6   bool
	}
	var candidates []candidate
	for p := 8090; p <= 8099; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err != nil {
			continue
		}
		t.Cleanup(func() { _ = ln.Close() })
		v6, err := net.Listen("tcp6", fmt.Sprintf("[::1]:%d", p))
		if err == nil {
			_ = v6.Close()
		}
		candidates = append(candidates, candidate{port: p, ln: ln, v6: err == nil})
	}

	first := -1
	for i := range candidates {
		if candidates[i].v6 {
			first = i
			break
		}
	}
	if len(candidates) < 2 || first < 0 {
		var ports []int
		for _, c := range candidates {
			ports = append(ports, c.port)
		}
		t.Skipf("need two IPv4 ports and one dual-address port in 8090-8099 (IPv4 candidates %v); another session is using the range", ports)
	}
	second := 0
	if second == first {
		second++
	}
	selected := []candidate{candidates[first], candidates[second]}
	selectedPorts := map[int]bool{selected[0].port: true, selected[1].port: true}
	for _, c := range candidates {
		if !selectedPorts[c.port] {
			_ = c.ln.Close()
		}
	}
	for _, c := range selected {
		srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `[{"id":"t1","format":"commander"}]`)
		})}
		go func() { _ = srv.Serve(c.ln) }()
		t.Cleanup(func() { _ = srv.Close() })
	}
	return []int{selected[0].port, selected[1].port}
}

// startGorgedProbe starts a probe server of the shape a sweep once killed: a
// process whose /proc/<pid>/comm reads gorged (this test binary, copied
// under that name and re-exec'd into its helper) with a LISTENING socket on
// [::1]:port —
// another address of a port the deploy is about to bind on 127.0.0.1. The
// script's port sweep must never touch it; the caller asserts its socket and
// PID still appear in ss after the deploy. That proves the process is alive
// AND still owns the [::1] listener — unlike signal 0, it cannot mistake an
// unreaped zombie for a live probe. The probe is killed and reaped at cleanup.
type gorgedProbe struct {
	cmd  *exec.Cmd
	port int
	done <-chan error
}

func startGorgedProbe(t *testing.T, port int) *gorgedProbe {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "gorged") // the copy's basename is what /proc/<pid>/comm shows
	if err := copyFile(bin, exe); err != nil {
		t.Fatalf("copying the test binary to %s: %v", bin, err)
	}
	ready := filepath.Join(dir, "ready")
	cmd := exec.Command(bin, "-test.run", "^TestGorgedProbeHelper$")
	cmd.Env = append(os.Environ(),
		"GORGE_PROBE_PORT="+fmt.Sprint(port),
		"GORGE_PROBE_READY="+ready)
	log, err := os.Create(filepath.Join(dir, "probe.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		select {
		case <-done:
			return
		default:
		}
		_ = cmd.Process.Kill()
		<-done
	})
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		select {
		case err := <-done:
			b, _ := os.ReadFile(log.Name())
			if strings.Contains(string(b), "address already in use") {
				t.Skipf("probe port [::1]:%d was claimed after selection: %v: %s", port, err, b)
			}
			t.Fatalf("probe gorged on [::1]:%d exited before ready: %v: %s", port, err, b)
		default:
		}
		if time.Now().After(deadline) {
			b, _ := os.ReadFile(log.Name())
			t.Fatalf("probe gorged on [::1]:%d never became ready: %s", port, b)
		}
		time.Sleep(pollEvery)
	}
	// Prove the probe really is the shape that used to be swept: a listening
	// socket rendered exactly as [::1]:port in ss -lptn — matched by the old
	// port-only filter, which is how such a probe once got killed.
	deadline = time.Now().Add(10 * time.Second)
	for {
		out, err := exec.Command("ss", "-lptn").Output()
		if err != nil {
			t.Fatalf("ss -lptn: %v", err)
		}
		if strings.Contains(string(out), fmt.Sprintf("[::1]:%d", port)) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the probe gorged's socket never showed in ss -lptn as [::1]:%d; the probe does not have the swept shape", port)
		}
		time.Sleep(pollEvery)
	}
	probe := &gorgedProbe{cmd: cmd, port: port, done: done}
	assertGorgedProbeListening(t, probe)
	return probe
}

// assertGorgedProbeListening proves the exact PID still owns its [::1]
// listener. A killed probe becomes a zombie until Cleanup reaps it, for which
// signal 0 would still succeed; a zombie cannot retain a listening socket.
func assertGorgedProbeListening(t *testing.T, probe *gorgedProbe) {
	t.Helper()
	out, err := exec.Command("ss", "-lptn").Output()
	if err != nil {
		t.Fatalf("ss -lptn: %v", err)
	}
	address := fmt.Sprintf("[::1]:%d", probe.port)
	pid := fmt.Sprintf("pid=%d,", probe.cmd.Process.Pid)
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, address) && strings.Contains(line, pid) {
			return
		}
	}
	t.Errorf("the deploy's port sweep killed probe pid %d or closed its [::1]:%d listener; ss -lptn:\n%s", probe.cmd.Process.Pid, probe.port, out)
}

// copyFile copies src to a new file at dst.
func copyFile(dst, src string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(d, s); err != nil {
		_ = d.Close()
		return err
	}
	return d.Close()
}

// TestGorgedProbeHelper is not a test a human runs: it is the body of the
// probe gorged process startGorgedProbe re-execs this binary into (from a
// copy named gorged, so /proc/<pid>/comm matches the sweep's comm check).
// It binds [::1]:$GORGE_PROBE_PORT, signals readiness by creating
// $GORGE_PROBE_READY, and then blocks until killed — the parent asserts the
// deploy's sweep left it running. In an ordinary run of the suite the env is
// unset and this skips in milliseconds.
func TestGorgedProbeHelper(t *testing.T) {
	port := os.Getenv("GORGE_PROBE_PORT")
	if port == "" {
		t.Skip("probe helper: only run through startGorgedProbe")
	}
	ln, err := net.Listen("tcp6", "[::1]:"+port)
	if err != nil {
		t.Fatalf("probe bind [::1]:%s: %v", port, err)
	}
	defer ln.Close()
	if err := os.WriteFile(os.Getenv("GORGE_PROBE_READY"), []byte("listening"), 0o644); err != nil {
		t.Fatalf("probe ready file: %v", err)
	}
	<-make(chan struct{}) // block until the parent's cleanup kills us
}

func readDeployCalls(t *testing.T, path string) [][]string {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		if line != "" {
			calls = append(calls, strings.Split(line, "\x1f"))
		}
	}
	return calls
}

// TestDeployScriptPassesBotCommonSense pins the deploy's default: the script
// carries COMMON_SENSE=${COMMON_SENSE:-false} and passes it as
// -bot-common-sense, and the default the script would serve is false.
func TestDeployScriptPassesBotCommonSense(t *testing.T) {
	if got, want := deployScriptDefault(t, "COMMON_SENSE"), "false"; got != want {
		t.Fatalf("deploy-demo.sh COMMON_SENSE default %q, want %q", got, want)
	}
	script, err := os.ReadFile(deployScript)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), `-bot-common-sense="$COMMON_SENSE"`) {
		t.Fatalf("deploy-demo.sh does not pass -bot-common-sense=\"$COMMON_SENSE\" to the servers:\n%s", script)
	}
}

// TestDeployScriptTurnsOnThePairedBot pins the demo's paired bot: on by
// default with the search that needs no trained network, passed to the
// servers as flags the real binary accepts, and off when PAIRED_BOT=off.
func TestDeployScriptTurnsOnThePairedBot(t *testing.T) {
	for name, want := range map[string]string{"PAIRED_BOT": "az-closed", "PAIRED_BOT_SIMS": "100", "PAIRED_BOT_SEARCHES": "2"} {
		if got := deployScriptDefault(t, name); got != want {
			t.Errorf("deploy-demo.sh %s default %q, want %q", name, got, want)
		}
	}
	served := func(env ...string) *flag.FlagSet {
		t.Helper()
		out, err := dryRun(t, env...)
		if err != nil {
			t.Fatalf("dry run: %v\n%s", err, out)
		}
		m := regexp.MustCompile(`(?m)^\+ env GOMEMLIMIT=\S+ \S+ (.*)$`).FindStringSubmatch(out)
		if m == nil {
			t.Fatalf("the dry run starts no server:\n%s", out)
		}
		fs := gorged.ServeFlagSetFor(extensions())
		// The dry run prints each argument shell-quoted; these need only the comma unescaped.
		if err := fs.Parse(strings.Fields(strings.ReplaceAll(m[1], `\,`, ","))); err != nil {
			t.Fatalf("real gorged rejects the deploy's server arguments %q: %v", m[1], err)
		}
		return fs
	}
	fs := served()
	for name, want := range map[string]string{"paired-bot": "az-closed", "paired-bot-sims": "100", "paired-bot-searches": "2"} {
		if got := flagValue(t, fs, name); got != want {
			t.Errorf("the demo server's -%s = %q, want %q", name, got, want)
		}
	}
	if got := flagValue(t, served("PAIRED_BOT=off"), "paired-bot"); got != "off" {
		t.Errorf("PAIRED_BOT=off serves -paired-bot %q", got)
	}
}

// dryRun runs scripts/deploy-demo.sh --dry-run with env on top of an empty
// DEMO_ENV and returns its combined output and error. A dry run builds, stops
// and starts nothing.
func dryRun(t *testing.T, env ...string) (string, error) {
	t.Helper()
	script, err := filepath.Abs(deployScript)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", script, "--dry-run")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "DEMO_ENV="+emptyDemoEnv(t), "DEMO_PORT_HISTORY=", "PUB_PORT=1", "OMNI_PORT=")
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// TestDeployRefusesACheckoutWithoutTheDraftTree: the demo's gorged links the
// draft events, so a GORGE_ROOT without draft/gorgedevents is refused, naming
// the checkout, before anything is built; this checkout passes.
func TestDeployRefusesACheckoutWithoutTheDraftTree(t *testing.T) {
	bare := t.TempDir()
	out, err := dryRun(t, "GORGE_ROOT="+bare)
	if err == nil {
		t.Errorf("the deploy accepted a checkout with no draft tree:\n%s", out)
	}
	if !strings.Contains(out, bare) || strings.Contains(out, "go build") {
		t.Errorf("the refusal must name %s and come before the build:\n%s", bare, out)
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if out, err := dryRun(t, "GORGE_ROOT="+root); err != nil {
		t.Errorf("the deploy refused this checkout: %v\n%s", err, out)
	}
}

// TestDeployEnvFileYieldsToTheEnvironment: deploy/demo.env sets the deploy's
// configuration, and a variable already in the environment, even an empty
// one, wins over it.
func TestDeployEnvFileYieldsToTheEnvironment(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "demo.env")
	if err := os.WriteFile(envFile, []byte("PACE=7s\nTABLES=5\nOMNI_PORT=4\nART_DIR=/from/the/file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := dryRun(t, "DEMO_ENV="+envFile, "PACE=9s", "OMNI_PORT=")
	if err != nil {
		t.Fatalf("dry run: %v\n%s", err, out)
	}
	for _, want := range []string{"  PACE=9s\n", "  TABLES=5\n", "  OMNI_PORT=''\n", "  ART_DIR=/from/the/file\n", "-art-dir /from/the/file"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry run lacks %q:\n%s", want, out)
		}
	}
}

// TestDemoEnvExampleDocumentsEveryVariable keeps deploy/demo.env.example and
// the script's DEMO_VARS, the variables a demo.env may set, in step.
func TestDemoEnvExampleDocumentsEveryVariable(t *testing.T) {
	script, err := os.ReadFile(deployScript)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)\nDEMO_VARS=\((.*?)\n\)`).FindSubmatch(script)
	if m == nil {
		t.Fatal("scripts/deploy-demo.sh has no DEMO_VARS=( ... ) list")
	}
	vars := strings.Fields(string(m[1]))
	example, err := os.ReadFile(filepath.Join("..", "..", "deploy", "demo.env.example"))
	if err != nil {
		t.Fatal(err)
	}
	ports, err := os.ReadFile(filepath.Join("..", "..", "scripts", "demo-ports.sh"))
	if err != nil {
		t.Fatal(err)
	}
	both := append(append([]byte(nil), script...), ports...)
	documented := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^#?([A-Z][A-Z0-9_]*)=`).FindAllSubmatch(example, -1) {
		documented[string(m[1])] = true
	}
	for _, v := range vars {
		if !documented[v] {
			t.Errorf("deploy/demo.env.example does not document %s", v)
		}
		if !regexp.MustCompile(`(?m)^` + v + `=\$\{` + v + `[:-]`).Match(both) {
			t.Errorf("scripts/deploy-demo.sh never defaults %s with ${%s-...}", v, v)
		}
		delete(documented, v)
	}
	for v := range documented {
		t.Errorf("deploy/demo.env.example documents %s, which DEMO_VARS does not list", v)
	}
}

// TestDemoSweepCoversRetiredPorts: the stop sweep is the union of the box's
// port history and the ports this run binds, so emptying a *_PORT variable to
// retire a listener never takes the server a previous layout left on that
// port off the sweep, a new port joins without dropping an old one, and an
// empty argument contributes no empty port (an empty alternative in the
// sweep's regex would match every listener on the box).
func TestDemoSweepCoversRetiredPorts(t *testing.T) {
	ports := func(history string, args ...string) []string {
		t.Helper()
		quoted := make([]string, len(args))
		for i, a := range args {
			quoted[i] = strconv.Quote(a)
		}
		cmd := exec.Command("bash", "-c", `. "$1"; shift; demo_sweep_ports "$@"`, "bash", filepath.Join("..", "..", "scripts", "demo-ports.sh"))
		cmd.Args = append(cmd.Args, args...)
		cmd.Env = append(os.Environ(), "DEMO_PORT_HISTORY="+history)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("demo_sweep_ports %q: %v", quoted, err)
		}
		return strings.Fields(string(out))
	}
	if got := ports("8080 8081", "", ""); !slices.Equal(got, []string{"8080", "8081"}) {
		t.Errorf("history 8080 8081, both listeners retired: sweep %q, want both history ports", got)
	}
	if got := ports("8080 8081", "8080", "", "", "8082"); !slices.Equal(got, []string{"8080", "8081", "8082"}) {
		t.Errorf("a new port 8082: sweep %q, want it beside the history", got)
	}
	if got := ports("", "8095", ""); !slices.Equal(got, []string{"8095"}) {
		t.Errorf("an empty history: sweep %q, want only the bound port", got)
	}
}

// pollEvery is how often a wait loop re-checks; every wait has a multi-second
// deadline, so the result never depends on this interval.
const pollEvery = time.Second / 100

// evalTemp is a fresh temp directory with its symlinks resolved, as the
// deploy script's pwd -P sees it.
func evalTemp(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// gorgeRoot is this checkout's root with its symlinks resolved.
func gorgeRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(abs)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// writeFile writes body to path, creating its directory.
func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// serverMainFixture is a scratch Go module declaring goVersion, with a main
// package at cmd/srv.
func serverMainFixture(t *testing.T, goVersion string) (modDir, mainDir string) {
	t.Helper()
	modDir = evalTemp(t)
	mainDir = filepath.Join(modDir, "cmd", "srv")
	writeFile(t, filepath.Join(modDir, "go.mod"), "module example.test/srv\n\ngo "+goVersion+"\n")
	writeFile(t, filepath.Join(mainDir, "main.go"), "package main\n\nfunc main() {}\n")
	return modDir, mainDir
}

// gorgeGoVersion is the go directive of this checkout's go.mod.
func gorgeGoVersion(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^go (\S+)$`).FindSubmatch(b)
	if m == nil {
		t.Fatal("go.mod has no go directive")
	}
	return string(m[1])
}

// TestDeployServerMainBuildsTheNamedPackage: SERVER_MAIN builds that main
// package, from its own module, against this checkout through a workspace
// file next to the binary; empty keeps cmd/gorged.
func TestDeployServerMainBuildsTheNamedPackage(t *testing.T) {
	root := gorgeRoot(t)
	gov := gorgeGoVersion(t)
	if gov == "1.21" {
		t.Fatalf("the fixture's go 1.21 must differ from gorge's go %s", gov)
	}
	run := func(env ...string) string {
		t.Helper()
		out, err := dryRun(t, env...)
		if err != nil {
			t.Fatalf("dry run: %v\n%s", err, out)
		}
		return out
	}
	modDir, mainDir := serverMainFixture(t, "1.21")
	bin := filepath.Join(evalTemp(t), "gorged")
	out := run("SERVER_MAIN="+mainDir, "BIN="+bin)
	for _, want := range []string{
		"\n+ workspace " + bin + ".work go " + gov + " use " + modDir + " " + root + "\n",
		"\n+ env CGO_ENABLED=0 GOWORK=" + bin + ".work go build -C " + modDir + " -trimpath -o " + bin + " ./cmd/srv\n",
		"  SERVER_MAIN=" + mainDir + "\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dry run lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "./cmd/gorged") {
		t.Errorf("SERVER_MAIN still builds ./cmd/gorged:\n%s", out)
	}

	modHi, mainHi := serverMainFixture(t, "99.0.0")
	if out := run("SERVER_MAIN="+mainHi, "BIN="+bin); !strings.Contains(out, "\n+ workspace "+bin+".work go 99.0.0 use "+modHi+" "+root+"\n") {
		t.Errorf("the module's higher go directive does not win:\n%s", out)
	}

	writeFile(t, filepath.Join(modDir, "main.go"), "package main\n\nfunc main() {}\n")
	if out := run("SERVER_MAIN="+modDir, "BIN="+bin); !strings.Contains(out, " go build -C "+modDir+" -trimpath -o "+bin+" .\n") {
		t.Errorf("SERVER_MAIN at the module directory does not build package .:\n%s", out)
	}

	out = run("BIN=" + bin)
	if !strings.Contains(out, "\n+ env CGO_ENABLED=0 go build -C "+root+" -trimpath -o "+bin+" ./cmd/gorged\n") || strings.Contains(out, "workspace") {
		t.Errorf("empty SERVER_MAIN does not build ./cmd/gorged as before:\n%s", out)
	}

	out = run("SERVER_MAIN=" + mainDir)
	if !strings.Contains(out, " -trimpath -o "+root+"/bin/server-main/gorged ./cmd/srv\n") {
		t.Errorf("SERVER_MAIN without BIN does not build into bin/server-main/gorged:\n%s", out)
	}
}

// TestDeployServerMainRefusals: a SERVER_MAIN the build cannot use is refused
// with one line, before anything is built.
func TestDeployServerMainRefusals(t *testing.T) {
	root := gorgeRoot(t)
	noMain := evalTemp(t)
	writeFile(t, filepath.Join(noMain, "go.mod"), "module example.test/x\n\ngo 1.21\n")
	noMod := evalTemp(t)
	writeFile(t, filepath.Join(noMod, "main.go"), "package main\n\nfunc main() {}\n")
	cases := []struct{ name, dir, want string }{
		{"relative", "relative/dir", "deploy-demo: SERVER_MAIN must be an absolute path: relative/dir"},
		{"missing", filepath.Join(evalTemp(t), "missing"), "is not a directory"},
		{"no main.go", noMain, "has no main.go"},
		{"no go.mod", noMod, "is not inside a Go module"},
		{"inside gorge", filepath.Join(root, "cmd", "gorged"), "is inside GORGE_ROOT"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.dir == noMod {
				for d := filepath.Dir(noMod); ; d = filepath.Dir(d) {
					if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
						t.Skipf("%s has a go.mod above the temp dir", d)
					}
					if d == filepath.Dir(d) {
						break
					}
				}
			}
			out, err := dryRun(t, "SERVER_MAIN="+c.dir)
			if err == nil {
				t.Errorf("SERVER_MAIN=%s was accepted:\n%s", c.dir, out)
			}
			if !strings.Contains(out, "deploy-demo: SERVER_MAIN ") || !strings.Contains(out, c.want) {
				t.Errorf("SERVER_MAIN=%s: output lacks %q:\n%s", c.dir, c.want, out)
			}
			if strings.Contains(out, "go build") {
				t.Errorf("SERVER_MAIN=%s: the refusal came after a build:\n%s", c.dir, out)
			}
		})
	}
}

// TestDeployExtraArgsReachEveryServer: SERVER_EXTRA_ARGS, from DEMO_ENV only,
// reaches every server after -events; each SERVER_EXTRA_DECKS directory gets
// one art fill; SERVER_NOTE ends the ready summary.
func TestDeployExtraArgsReachEveryServer(t *testing.T) {
	root := gorgeRoot(t)
	decksA, decksB := evalTemp(t), evalTemp(t)
	envFile := filepath.Join(t.TempDir(), "demo.env")
	writeFile(t, envFile, `SERVER_EXTRA_ARGS=(-extra-flag "a b" -other)
SERVER_EXTRA_DECKS=(`+decksA+` `+decksB+` `+decksA+` `+filepath.Join(root, "testkit", "testutil", "decks")+` )
SERVER_NOTE="note x"
`)
	out, err := dryRun(t, "DEMO_ENV="+envFile, "OMNI_PORT=3", "SERVER_EXTRA_ARGS=junk")
	if err != nil {
		t.Fatalf("dry run: %v\n%s", err, out)
	}
	servers := regexp.MustCompile(`(?m)^\+ env GOMEMLIMIT=.*$`).FindAllString(out, -1)
	if len(servers) != 2 {
		t.Fatalf("want two server lines, got %d:\n%s", len(servers), out)
	}
	for _, s := range servers {
		if !strings.Contains(s, ` -events -extra-flag a\ b -other`) {
			t.Errorf("server line lacks the extra args after -events: %s", s)
		}
	}
	for _, d := range []string{decksA, decksB} {
		if n := strings.Count(out, "-prewarm-art-only -prewarm-art-budget 200s -prewarm-art-max-consecutive-failures 10 -decks "+d+" "); n != 1 {
			t.Errorf("%d art fills of %s, want 1:\n%s", n, d, out)
		}
	}
	if n := strings.Count(out, "-prewarm-art-only"); n != 3 {
		t.Errorf("%d art fills, want 3 (DECKS once, then each extra once):\n%s", n, out)
	}
	if !strings.Contains(out, "  SERVER_EXTRA_ARGS: -extra-flag a\\ b -other\n") {
		t.Errorf("the configuration echo lacks the extra args:\n%s", out)
	}
	if strings.Contains(out, "junk") {
		t.Errorf("an inherited SERVER_EXTRA_ARGS leaked into the deploy:\n%s", out)
	}
	if !regexp.MustCompile(`(?m)^deploy-demo: would report: ready — spectator \(.*, note x\)$`).MatchString(out) {
		t.Errorf("the ready summary does not end with the note:\n%s", out)
	}

	out, err = dryRun(t)
	if err != nil {
		t.Fatalf("dry run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "  SERVER_EXTRA_ARGS:\n") || strings.Contains(out, "note x") {
		t.Errorf("an empty env file still carries extras:\n%s", out)
	}
}

// TestDeployPairedBotDecks: PAIRED_BOT_DECKS passes -paired-bot-decks after
// -paired-bot-searches, as a flag the real binary accepts; empty passes none.
func TestDeployPairedBotDecks(t *testing.T) {
	served := func(env ...string) (string, *flag.FlagSet) {
		t.Helper()
		out, err := dryRun(t, env...)
		if err != nil {
			t.Fatalf("dry run: %v\n%s", err, out)
		}
		m := regexp.MustCompile(`(?m)^\+ env GOMEMLIMIT=\S+ \S+ (.*)$`).FindStringSubmatch(out)
		if m == nil {
			t.Fatalf("the dry run starts no server:\n%s", out)
		}
		fs := gorged.ServeFlagSetFor(extensions())
		if err := fs.Parse(strings.Fields(strings.ReplaceAll(m[1], `\,`, ","))); err != nil {
			t.Fatalf("real gorged rejects the deploy's server arguments %q: %v", m[1], err)
		}
		return m[1], fs
	}
	if line, _ := served(); strings.Contains(line, "-paired-bot-decks") {
		t.Errorf("the default deploy passes -paired-bot-decks: %s", line)
	}
	if line, _ := served("PAIRED_BOT_DECKS="); strings.Contains(line, "-paired-bot-decks") {
		t.Errorf("an empty PAIRED_BOT_DECKS passes -paired-bot-decks: %s", line)
	}
	dir := evalTemp(t)
	line, fs := served("PAIRED_BOT_DECKS=" + dir)
	if !strings.Contains(line, " -paired-bot-searches 2 -paired-bot-decks "+dir+" ") {
		t.Errorf("-paired-bot-decks %s does not follow -paired-bot-searches: %s", dir, line)
	}
	if got := flagValue(t, fs, "paired-bot-decks"); got != dir {
		t.Errorf("-paired-bot-decks parses to %q, want %q", got, dir)
	}
}
