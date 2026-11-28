# gorged on a local Kubernetes cluster

One pod of the [daily image](../../.github/workflows/daily-image.yml) on a local
cluster (written for k3d/k3s, context `k3d-bm`), in its own namespace `gorge`. Nothing here is
cluster-scoped except the namespace itself.

| Object | What |
|---|---|
| Deployment `gorged` | 1 replica, strategy `Recreate`; requests and limits both `cpu: "2"`, `memory: 2Gi` (`GOMEMLIMIT=1536MiB`) |
| Service `gorged` | ClusterIP, port 8080 |
| PVC `data` (2Gi) | gorged's persistence, `/data` |
| PVC `cards` (4Gi) | the Forge card scripts, `/cards`, fetched on first start (GPL-3.0, never in the image; a few minutes) |
| PVC `carddb` (1Gi) | a copy of carddb, mounted read-only at `/carddb` |
| PVC `art` | card art cache, `/art` |
| HTTPRoute `gorged` | on gateway `eg` (namespace `envoy-gateway-system`, listener `http`), hosts `master-edh.net` and `www.master-edh.net`, open to everyone; a `BackendTrafficPolicy` lifts the request timeout to 1h for the server-sent event streams |
| `emptyDir` | `/feedback` |

The Deployment passes the full gorged command as `args` (it replaces the image `CMD`): four
Commander tables of four seats, plus play-vs-bot, draft events and the ManaBrew routes.

Probes: all three (startup, readiness, liveness) read `GET /healthz` on port 8080, which
answers as soon as the listener serves. The startup probe allows 15 minutes for the first fetch.

Limits: the args also turn on

- `-keep-matches=200`: each bot-only table keeps 200 finished matches on disk and deletes older ones; matches with a human seat and crashed matches are never deleted.
- `-rate-limit=10`: one client may make 10 creating requests a minute (new games, events, deck checks, feedback).
- `-rate-burst=10`: the most creating requests one client may make at once.
- `-trusted-proxy-header=CF-Connecting-IP`: the rate limiter keys on the client address in this header, not on the proxy's.
- `-site-name=master-edh.net`: the name in the served page's title.
- `-metrics`: serve Prometheus counts at `/metrics` on the main port, readable by anything proxied to it.

`-trusted-proxy-header` is safe here because the only public route to the pod is the gateway behind
the Cloudflare tunnel (`httproute.yaml`). The gateway's LoadBalancer from the host network,
`port-forward` and the ClusterIP also reach the pod, and a client on those paths can set the header
itself. On a cluster the public reaches any other way, drop the flag: a client could otherwise set
the header and dodge the rate limit.

## Use

```sh
kubectl --context k3d-bm apply -k deploy/k8s            # image tag from kustomization.yaml (latest)
# Deploy the daily image's dated tag (daily-image.yml) instead of :latest:
kubectl kustomize deploy/k8s | sed 's|\(image: ghcr.io/adams-shaun/gorge\):latest|\1:2026-11-27|' | kubectl --context k3d-bm apply -f -
kubectl --context k3d-bm -n gorge port-forward svc/gorged 8080:8080
```

The image name is set once, under `images:` in `kustomization.yaml`; change it there if
the repository variable `IMAGE_NAME` does. `imagePullPolicy: Always` makes a restart re-pull the
tag, so a dated tag is what keeps a restart from changing the build. A private GHCR package needs
an `imagePullSecret`.

## Reaching it from outside

The route needs Envoy Gateway's `eg` gateway (LoadBalancer `172.23.255.200:80` on k3d-bm). The one
external step, which is not in these manifests: the Cloudflare tunnel's ingress must send
`master-edh.net -> http://172.23.255.200:80` (and `www.master-edh.net` likewise). To rename the
site, change the two `hostnames` in `httproute.yaml`.

## The carddb copy

carddb is the data set gorge-compliance publishes (`carddb.sqlite` and `public/`). The
cluster cannot see the host path, so the data is a copy pushed into the `carddb` volume:

```sh
kubectl --context k3d-bm apply -f deploy/k8s/carddb-seed.yaml        # a pod holding the volume
kubectl --context k3d-bm -n gorge wait --for=condition=Ready pod/carddb-seed
kubectl --context k3d-bm -n gorge cp ~/.local/share/carddb/. carddb-seed:/carddb
kubectl --context k3d-bm -n gorge delete pod carddb-seed
```

Re-run it to refresh the copy. carddb is never baked into the image. At this commit nothing
in gorged (`server`, `cmd`) reads carddb: the mount and the `CARDDB_DIR=/carddb` variable
are there so a reader finds it without a manifest change.
