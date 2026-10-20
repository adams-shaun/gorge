#!/usr/bin/env python3
"""Parse every Mermaid diagram of a built docs site with the Mermaid the site loads.

mkdocs build --strict does not parse Mermaid: Material renders each diagram in
the browser, inside a closed shadow root, so a syntax error ships as a "Syntax
error in text" box that no build step sees. This script extracts every
<pre class="mermaid"> block from the built site, serves the site on a local
port, and has headless Chrome run mermaid.parse() on each block with the same
major version Material loads (mermaid@11 from unpkg). It exits 1 and names the
page and block of every diagram that fails.

    python3 scripts/docs_mermaid_check.py site/          # after mkdocs build
    CHROME=chromium python3 scripts/docs_mermaid_check.py site/

Standard library only; needs Chrome/Chromium and network access to unpkg.
"""

import glob
import html
import http.server
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import threading

MERMAID = "https://unpkg.com/mermaid@11/dist/mermaid.min.js"

HARNESS = """<!doctype html><meta charset=utf-8><pre id=out>running</pre>
<script src="%s"></script>
<script>
(async () => {
  const blocks = await (await fetch('blocks.json')).json();
  const bad = [];
  mermaid.initialize({startOnLoad: false});
  for (const b of blocks) {
    try { await mermaid.parse(b.src); }
    catch (e) { bad.push(b.page + ' diagram ' + (b.i + 1) + ': ' + String(e.message || e).split('\\n').slice(0, 3).join(' | ')); }
  }
  document.getElementById('out').textContent = 'DONE ' + blocks.length + '\\n' + bad.join('\\n');
})().catch(e => { document.getElementById('out').textContent = 'HARNESS ' + e; });
</script>
""" % MERMAID


def find_chrome():
    for c in (os.environ.get("CHROME"), "google-chrome", "google-chrome-stable", "chromium", "chromium-browser"):
        if c and shutil.which(c):
            return shutil.which(c)
    sys.exit("docs_mermaid_check: no Chrome/Chromium found (set CHROME)")


def main(argv):
    if len(argv) != 2:
        sys.exit(__doc__)
    site = os.path.abspath(argv[1])
    blocks = []
    for f in sorted(glob.glob(os.path.join(site, "**", "*.html"), recursive=True)):
        text = open(f, encoding="utf-8").read()
        for i, m in enumerate(re.finditer(r'<pre class="mermaid"><code>(.*?)</code></pre>', text, re.S)):
            blocks.append({"page": os.path.relpath(f, site), "i": i, "src": html.unescape(m.group(1))})
    if not blocks:
        print("docs_mermaid_check: no Mermaid diagrams found")
        return 0

    work = tempfile.mkdtemp(prefix="mermaid-check-")
    try:
        with open(os.path.join(work, "blocks.json"), "w") as f:
            json.dump(blocks, f)
        with open(os.path.join(work, "index.html"), "w") as f:
            f.write(HARNESS)
        class Quiet(http.server.SimpleHTTPRequestHandler):
            def __init__(self, *a, **k):
                super().__init__(*a, directory=work, **k)

            def log_message(self, *a):
                pass

        srv = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Quiet)
        threading.Thread(target=srv.serve_forever, daemon=True).start()
        url = "http://127.0.0.1:%d/" % srv.server_address[1]
        dom = subprocess.run(
            [find_chrome(), "--headless=new", "--no-sandbox", "--disable-gpu",
             "--user-data-dir=" + os.path.join(work, "profile"),
             "--virtual-time-budget=30000", "--dump-dom", url],
            capture_output=True, text=True, timeout=120).stdout
        srv.shutdown()
    finally:
        shutil.rmtree(work, ignore_errors=True)

    m = re.search(r'<pre id="out">(.*?)</pre>', dom, re.S)
    out = html.unescape(m.group(1)) if m else ""
    if not out.startswith("DONE "):
        sys.exit("docs_mermaid_check: the harness did not finish: %r" % (out or dom[:300]))
    lines = out.split("\n")
    errors = [l for l in lines[1:] if l]
    print("docs_mermaid_check: %s diagrams parsed, %d failed" % (lines[0].split()[1], len(errors)))
    for e in errors:
        print("  " + e)
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
