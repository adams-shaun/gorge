# Prints "<uncomputed> <json list of conflicting PR numbers>" for open same-repo non-draft PRs read from pulls.json.
# GH_TOKEN and REPO come from the environment. Used by .github/workflows/pr-autoresolve.yml (no gh on the box runners).
import json, os, urllib.request
tok, repo = os.environ["GH_TOKEN"], os.environ["REPO"]
def get(u):
    r = urllib.request.Request(u, headers={"Authorization": "Bearer " + tok, "Accept": "application/vnd.github+json"})
    return json.load(urllib.request.urlopen(r))
unknown, conflicting = 0, []
for p in json.load(open("pulls.json")):
    if p["draft"] or p["head"]["ref"].startswith("regen/"):
        continue
    if p["head"]["repo"] is None or p["head"]["repo"]["full_name"] != repo:
        continue
    m = get("https://api.github.com/repos/%s/pulls/%d" % (repo, p["number"])).get("mergeable")
    if m is None:
        unknown += 1
    elif m is False:
        conflicting.append(p["number"])
print(unknown, json.dumps(conflicting, separators=(",", ":")))
