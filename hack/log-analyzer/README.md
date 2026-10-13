# Ramen Log Analyzer

A self-contained, browser-based log analysis tool for ramen controller-manager logs.
No server, no install — open `index.html` in any modern browser.

## Features

- **Multi-cluster support** — load hub and managed cluster logs side by side
- **Auto-detection** — identifies hub vs managed cluster from controller names
  (`drpc`/`drc`/`drp` = hub; `vrg`/`pvrgl`/`rgd`/`rgs` = managed)
- **Three-level drill-down** — Cluster → Object (VRG/DRPC) → Iteration (one reconcile loop per `rid`)
- **Cross-cluster correlation** — a **Linked** bar shows matching VRG iterations on managed clusters
  for the currently selected DRPC iteration (same `namespace/name`, within a 5-minute window)
- **Timestamps normalized to UTC** — hub and managed logs from different timezones compare cleanly
- **Virtual scrolling** — handles 100k+ line log files without freezing
- **Level filtering** — toggle INFO / WARN / ERROR / DEBUG independently
- **Free-text search** — with live match highlighting
- **Expandable field chips** — click any log row to reveal all structured key=value fields

---

## For testers — loading a `gather.rdr/` directory

After an e2e test run, `drenv gather` produces a directory tree like:

```
gather.rdr/
├── hub/pods/ramen-system/ramen-hub-operator-<hash>/logs/manager.log
├── dr1/pods/ramen-system/ramen-dr-cluster-operator-<hash>/logs/manager.log
└── dr2/pods/ramen-system/ramen-dr-cluster-operator-<hash>/logs/manager.log
```

### Option A — Script (easiest)

```bash
# From the ramen repo root, after drenv gather:
hack/log-analyzer/open-gather.sh test/gather.rdr
```

The script:
1. Finds all ramen operator log files under the gather directory
2. Stages them with cluster-prefixed names (`hub__manager.log`, `dr1__manager.log`, …)
3. Opens the analyzer and prints the exact files to load
4. Keeps the staged files alive for 5 minutes

### Option B — Directory picker in the browser

1. Open `hack/log-analyzer/index.html`
2. Click **📁 Load gather dir** in the top bar
3. Navigate to and select your `gather.rdr/` folder
4. The tool scans for ramen log files, auto-labels each cluster from the directory path
   (`hub/` → label `hub`, `dr1/` → label `dr1`, etc.) and loads them all at once

### Option C — Drag and drop

Drag the entire `gather.rdr/` folder from your file manager onto the browser window.
Cluster names are extracted from the path automatically.

---

## For developers — loading individual files

### Collect logs manually

```bash
# Hub cluster
kubectl logs -n ramen-system deployment/ramen-hub-operator \
  --all-containers --since=2h > hub.log

# Managed clusters
kubectl logs -n ramen-system deployment/ramen-dr-cluster-operator \
  --all-containers --since=2h > cluster-1.log
```

### Load into the tool

- Click **📂 Add File** and pick one or more `.log` files, or
- Click **Paste Logs**, choose cluster type and name, paste log text

---

## Navigating logs

```
Left panel                          Right panel
────────────────────────────────    ─────────────────────────────────────
● hub (HUB)                         [hub] › ramen-system/my-app › Iter #2 (a1b2c3d4)
  ▼ ramen-system/my-app             ─────────────────────────────────────
    #1  a1b2c3d4  3 lines            State: Primary  Lines: 5  Duration: 1.2s
    #2  b2c3d4e5  5 lines  1 err    ─────────────────────────────────────
    ▶ #3                            Linked: ● dr1  #4  +0.8s   ● dr2  #2
                                    ─────────────────────────────────────
● dr1 (MANAGED)                     [filter]   INFO  WARN  ERROR  DEBUG
  ▶ ramen-system/my-app
                                    14:27:12.001 UTC  INFO   Entering reconcile loop
● dr2 (MANAGED)                     14:27:12.003 UTC  INFO   VRG status observed
  ▶ ramen-system/my-app             14:27:12.004 UTC  ERROR  Failed to get resource ↓
                                    14:27:12.005 UTC  INFO   Exiting reconcile loop
```

1. **Expand a cluster** by clicking its row
2. **Expand an object** (VRG or DRPC) to see its iterations
3. **Click an iteration** to load its log lines on the right
4. The **Linked** bar shows correlated iterations on other clusters — click to jump there

### Iteration badges

| Badge | Meaning |
|-------|---------|
| `3 err` | 3 ERROR-level lines in this iteration |
| `↔ 2`  | 2 correlated iterations in other clusters |

---

## Relabeling clusters

After loading, the **Loaded files** section at the bottom of the left panel lets you:
- Edit the **cluster name** (e.g. rename `dr1` to `aws-primary`)
- Override the **type** (Hub / Managed / Unknown) if auto-detection was wrong

---

## Log format

Both zap output modes are supported:

**Dev mode** (default):
```
2024-01-31T14:27:12.726-0500  INFO  vrg  Entering reconcile loop  {"vrg":"ramen-system/my-app","rid":"a1b2c3d4"}
```

**Prod mode** (JSON):
```json
{"level":"info","ts":1706720832.726,"msg":"Entering reconcile loop","vrg":"ramen-system/my-app","rid":"a1b2c3d4"}
```

---

## Correlation logic

Hub (DRPC) and managed (VRG) events for the same application are linked using:

1. **Name identity** — `VRG.name == DRPC.name` and `VRG.namespace == DRPC.namespace`
2. **Time window** — the managed iteration must start within ±5 minutes of the hub iteration

---

## Controller reference

| Controller | Cluster | Logger key |
|------------|---------|------------|
| `drpc`     | Hub     | `"drpc"`   |
| `drp`      | Hub     | `"drp"`    |
| `drc`      | Hub     | `"drc"`    |
| `vrg`      | Managed | `"vrg"`    |
| `pvrgl`    | Managed | `"pvrgl"`  |
| `rgd`      | Managed | `"rgd"`    |
| `rgs`      | Managed | `"rgs"`    |
