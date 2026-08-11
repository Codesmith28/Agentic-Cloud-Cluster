#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# model_promote.sh — Promote a trained model to active with version archival
#
# Takes the latest model from training results, archives the current active
# model with an incremented version number, and installs the new one as
# ppo_latest.pt (the file the gRPC service loads at startup).
#
# Archive layout:
#   agentic_scheduler/models/archive/
#     v001_20260426-163000.pt
#     v002_20260426-180000.pt
#     ...
#     VERSION                  ← current version counter
#     CHANGELOG.md             ← human-readable log of promotions
#
# Usage:
#   ./scripts/model_promote.sh                          # auto-detect latest from results/
#   ./scripts/model_promote.sh path/to/checkpoint.pt    # promote a specific file
#   ./scripts/model_promote.sh --dry-run                # preview without changes
# =============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

MODELS_DIR="${PROJECT_ROOT}/agentic_scheduler/models"
ARCHIVE_DIR="${MODELS_DIR}/archive"
ACTIVE_MODEL="${MODELS_DIR}/ppo_latest.pt"
VERSION_FILE="${ARCHIVE_DIR}/VERSION"
CHANGELOG="${ARCHIVE_DIR}/CHANGELOG.md"
RESULTS_DIR="${PROJECT_ROOT}/agentic_scheduler/results"

DRY_RUN=false
SOURCE_MODEL=""

# ── Parse arguments ─────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
    case "$1" in
        --dry-run|-n)
            DRY_RUN=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS] [model_path]"
            echo ""
            echo "Promotes a trained model to ppo_latest.pt, archiving the previous version."
            echo ""
            echo "Arguments:"
            echo "  model_path      Path to .pt checkpoint (default: auto-detect from results/)"
            echo ""
            echo "Options:"
            echo "  --dry-run, -n   Show what would happen without making changes"
            echo "  -h, --help      Show this help"
            exit 0
            ;;
        -*)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
        *)
            SOURCE_MODEL="$1"
            shift
            ;;
    esac
done

# ── Helpers ──────────────────────────────────────────────────────────────────
info()  { echo -e "\033[1;34m[INFO]\033[0m  $*"; }
ok()    { echo -e "\033[1;32m[OK]\033[0m    $*"; }
warn()  { echo -e "\033[1;33m[WARN]\033[0m  $*"; }
fail()  { echo -e "\033[1;31m[FAIL]\033[0m  $*" >&2; exit 1; }

# ── Find source model ───────────────────────────────────────────────────────
if [[ -z "${SOURCE_MODEL}" ]]; then
    # Auto-detect: prefer ppo_trained_final.pt, fallback to newest .pt in results/
    if [[ -f "${RESULTS_DIR}/ppo_trained_final.pt" ]]; then
        SOURCE_MODEL="${RESULTS_DIR}/ppo_trained_final.pt"
    else
        SOURCE_MODEL="$(find "${RESULTS_DIR}" -maxdepth 1 -name '*.pt' -type f -print0 \
            | xargs -0 ls -t 2>/dev/null | head -1 || true)"
    fi
fi

if [[ -z "${SOURCE_MODEL}" || ! -f "${SOURCE_MODEL}" ]]; then
    fail "No model found. Train first or pass a path: $0 path/to/model.pt"
fi

SOURCE_MODEL="$(cd "$(dirname "${SOURCE_MODEL}")" && pwd)/$(basename "${SOURCE_MODEL}")"
info "Source model: ${SOURCE_MODEL}"

# ── Read current version ────────────────────────────────────────────────────
mkdir -p "${ARCHIVE_DIR}"

if [[ -f "${VERSION_FILE}" ]]; then
    CURRENT_VERSION=$(cat "${VERSION_FILE}" | tr -d '[:space:]')
else
    CURRENT_VERSION=0
fi

NEXT_VERSION=$((CURRENT_VERSION + 1))
NEXT_VERSION_PAD=$(printf "v%03d" "${NEXT_VERSION}")
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
ARCHIVE_NAME="${NEXT_VERSION_PAD}_${TIMESTAMP}.pt"

info "Current version: v$(printf '%03d' "${CURRENT_VERSION}")"
info "New version:     ${NEXT_VERSION_PAD}"

# ── Check if source is different from active ────────────────────────────────
if [[ -f "${ACTIVE_MODEL}" ]]; then
    # Compare file checksums
    SRC_HASH=$(shasum -a 256 "${SOURCE_MODEL}" | cut -d' ' -f1)
    ACT_HASH=$(shasum -a 256 "${ACTIVE_MODEL}" | cut -d' ' -f1)

    if [[ "${SRC_HASH}" == "${ACT_HASH}" ]]; then
        ok "Source model is identical to current active model. Nothing to do."
        exit 0
    fi

    ACTIVE_SIZE=$(wc -c < "${ACTIVE_MODEL}" | tr -d ' ')
    info "Archiving current active model ($(numfmt --to=iec "${ACTIVE_SIZE}" 2>/dev/null || echo "${ACTIVE_SIZE} bytes")) → archive/${ARCHIVE_NAME}"
else
    warn "No active model found at ${ACTIVE_MODEL} — fresh install"
    ACT_HASH=""
fi

# ── Dry-run mode ────────────────────────────────────────────────────────────
if [[ "${DRY_RUN}" == "true" ]]; then
    echo ""
    echo "Dry-run summary:"
    if [[ -f "${ACTIVE_MODEL}" ]]; then
        echo "  Archive: ${ACTIVE_MODEL} → ${ARCHIVE_DIR}/${ARCHIVE_NAME}"
    fi
    echo "  Install: ${SOURCE_MODEL} → ${ACTIVE_MODEL}"
    echo "  Version: ${NEXT_VERSION_PAD}"
    echo ""
    echo "No changes made."
    exit 0
fi

# ── Archive current model ───────────────────────────────────────────────────
if [[ -f "${ACTIVE_MODEL}" ]]; then
    cp "${ACTIVE_MODEL}" "${ARCHIVE_DIR}/${ARCHIVE_NAME}"
    ok "Archived: archive/${ARCHIVE_NAME}"
fi

# ── Install new model ───────────────────────────────────────────────────────
cp "${SOURCE_MODEL}" "${ACTIVE_MODEL}"
ok "Installed: ${SOURCE_MODEL} → ppo_latest.pt"

# ── Update version counter ──────────────────────────────────────────────────
echo "${NEXT_VERSION}" > "${VERSION_FILE}"

# ── Update changelog ────────────────────────────────────────────────────────
SRC_HASH=$(shasum -a 256 "${SOURCE_MODEL}" | cut -d' ' -f1)
SRC_SIZE=$(wc -c < "${SOURCE_MODEL}" | tr -d ' ')
SRC_BASENAME=$(basename "${SOURCE_MODEL}")

ENTRY="## ${NEXT_VERSION_PAD} — $(date '+%Y-%m-%d %H:%M:%S')
- **Source**: \`${SRC_BASENAME}\`
- **SHA-256**: \`${SRC_HASH:0:16}...\`
- **Size**: ${SRC_SIZE} bytes
"

if [[ -f "${CHANGELOG}" ]]; then
    # Prepend new entry after the header
    EXISTING=$(cat "${CHANGELOG}")
    HEADER="# Model Version History"
    BODY="${EXISTING#"${HEADER}"}"
    printf '%s\n\n%s\n%s' "${HEADER}" "${ENTRY}" "${BODY}" > "${CHANGELOG}"
else
    printf '# Model Version History\n\n%s\n' "${ENTRY}" > "${CHANGELOG}"
fi

ok "Changelog updated"

# ── Summary ──────────────────────────────────────────────────────────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Model promoted to ${NEXT_VERSION_PAD}"
echo "  Active:  ${ACTIVE_MODEL}"
echo "  Archive: ${ARCHIVE_DIR}/${ARCHIVE_NAME}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Archive contents:"
ls -lh "${ARCHIVE_DIR}"/*.pt 2>/dev/null | awk '{print "  " $NF " (" $5 ")"}'
