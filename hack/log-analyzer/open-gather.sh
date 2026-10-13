#!/usr/bin/env bash
# SPDX-FileCopyrightText: The RamenDR authors
# SPDX-License-Identifier: Apache-2.0
#
# open-gather.sh — open the Ramen Log Analyzer pre-loaded with logs from a
# drenv gather directory (gather.rdr/ or any equivalent directory tree).
#
# Usage:
#   ./open-gather.sh [GATHER_DIR]
#
# GATHER_DIR defaults to "gather.rdr" relative to the current working directory.
#
# What it does:
#   1. Walks GATHER_DIR looking for ramen operator log files.
#   2. Copies them into a flat staging directory, embedding the cluster name
#      in each filename so the browser tool can auto-label them.
#   3. Opens index.html in the default browser — the user drags the staging
#      directory onto the tool, or the script prints the exact files to load.
#
# The expected gather directory layout is:
#   GATHER_DIR/
#   ├── hub/pods/ramen-system/ramen-hub-operator-<hash>/logs/manager.log
#   ├── dr1/pods/ramen-system/ramen-dr-cluster-operator-<hash>/logs/manager.log
#   └── dr2/pods/ramen-system/ramen-dr-cluster-operator-<hash>/logs/manager.log

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ANALYZER_HTML="${SCRIPT_DIR}/index.html"

GATHER_DIR="${1:-gather.rdr}"

if [[ ! -d "${GATHER_DIR}" ]]; then
    echo "error: gather directory '${GATHER_DIR}' not found." >&2
    echo "       Run 'drenv gather --directory gather.rdr envs/regional-dr.yaml' first," >&2
    echo "       or pass the path to your gather directory as the first argument." >&2
    exit 1
fi

# ── Find ramen operator log files ────────────────────────────────────────────
# kubectl-gather stores logs under:
#   <cluster>/pods/<namespace>/<pod-name>/logs/<container>.log
# We look for the ramen-system namespace and operator pod names.

mapfile -t LOG_FILES < <(
    find "${GATHER_DIR}" -type f -name "*.log" | grep -E \
        'ramen-(hub|dr-cluster)-operator|controller-manager' \
    | sort
)

if [[ ${#LOG_FILES[@]} -eq 0 ]]; then
    # ODF must-gather layout:
    #   namespaces/openshift-dr-system/pods/ramen-dr-cluster-operator-<hash>/manager/manager/logs/current.log
    # Only pick up the manager container log, not kube-rbac-proxy or other sidecars.
    mapfile -t LOG_FILES < <(
        find "${GATHER_DIR}" -type f -name "*.log" \
          \( -path "*/openshift-dr-system/*" -o -path "*/ramen-system/*" \) \
          -not -path "*/kube-rbac-proxy/*" \
          -not -path "*/chown-container*" \
          -name "current.log" -o -name "previous.log" -o -name "manager.log" \
        2>/dev/null | grep -E 'openshift-dr-system|ramen-system' | sort
    )
fi

if [[ ${#LOG_FILES[@]} -eq 0 ]]; then
    echo "warning: no ramen operator log files found under '${GATHER_DIR}'." >&2
    echo "         Searched for: ramen-hub-operator, ramen-dr-cluster-operator, controller-manager" >&2
    echo "         Falling back to all .log files under the directory." >&2
    mapfile -t LOG_FILES < <(find "${GATHER_DIR}" -type f -name "*.log" | sort)
fi

if [[ ${#LOG_FILES[@]} -eq 0 ]]; then
    echo "error: no log files found under '${GATHER_DIR}'." >&2
    exit 1
fi

echo "Found ${#LOG_FILES[@]} log file(s):"

# ── Stage files with cluster-prefixed names ───────────────────────────────────
# Create a temp staging directory. Each file is renamed to <cluster>__<original>.log
# so the browser tool's labelFromFile() function picks up the cluster name.

STAGE_DIR="$(mktemp -d /tmp/ramen-logs-XXXXXX)"
trap 'rm -rf "${STAGE_DIR}"' EXIT

declare -A SEEN_CLUSTERS

for f in "${LOG_FILES[@]}"; do
    # Extract cluster name from path: the first path component under GATHER_DIR
    rel="${f#${GATHER_DIR}/}"          # strip leading GATHER_DIR/
    cluster="${rel%%/*}"               # first path segment = cluster name
    cluster="${cluster:-unknown}"

    # Build a staged filename: <cluster>__manager.log (or __<original>.log)
    orig_base="$(basename "${f}")"
    staged_name="${cluster}__${orig_base}"

    cp "${f}" "${STAGE_DIR}/${staged_name}"
    echo "  [${cluster}] ${f} → ${staged_name}"

    SEEN_CLUSTERS["${cluster}"]=1
done

echo ""
echo "Clusters found: ${!SEEN_CLUSTERS[*]}"
echo ""
echo "Staged logs in: ${STAGE_DIR}"
echo ""

# ── Open the analyzer ─────────────────────────────────────────────────────────

if [[ ! -f "${ANALYZER_HTML}" ]]; then
    echo "error: analyzer not found at ${ANALYZER_HTML}" >&2
    echo "       Run this script from the ramen repository root or pass the correct path." >&2
    exit 1
fi

echo "Opening: ${ANALYZER_HTML}"
echo ""
echo "  ┌─────────────────────────────────────────────────────────────────┐"
echo "  │  In the browser:                                                │"
echo "  │  1. Click '📂 Add File' and select ALL files from:             │"
printf "  │     %-62s│\n" "${STAGE_DIR}/"
echo "  │  2. Cluster labels are auto-filled from file names.            │"
echo "  │  3. Correlated hub ↔ managed events appear automatically.     │"
echo "  └─────────────────────────────────────────────────────────────────┘"
echo ""

# Open the browser (keep staging dir alive while browser loads)
case "$(uname -s)" in
    Darwin)  open "${ANALYZER_HTML}" ;;
    Linux)   xdg-open "${ANALYZER_HTML}" 2>/dev/null || \
             sensible-browser "${ANALYZER_HTML}" 2>/dev/null || \
             echo "Open manually: ${ANALYZER_HTML}" ;;
    *)       echo "Open manually: ${ANALYZER_HTML}" ;;
esac

# Wait briefly so the user can see the instructions before the temp dir is removed
echo "Keeping staged logs available for 5 minutes (Ctrl+C to exit sooner)..."
sleep 300
