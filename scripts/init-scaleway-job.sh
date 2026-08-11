#!/usr/bin/env bash
set -euo pipefail

# -----------------------------------------------------------------------------
# One-time setup script for Scaleway Serverless Jobs
# Creates: Container Registry namespace, Job Definition with env vars, Cron trigger
#
# Prerequisites:
#   - scw CLI installed and configured (scw init)
#   - Docker installed and running
#   - SCW_DEFAULT_PROJECT_ID and SCW_DEFAULT_ORGANIZATION_ID set
#
# Usage:
#   ./scripts/init-scaleway-job.sh
# -----------------------------------------------------------------------------

REGION="fr-par"
REGISTRY="rg.${REGION}.scw.cloud"

# --- Configurable names ---
REGISTRY_NAMESPACE="${SCW_REGISTRY_NAMESPACE:-boavizta-registry}"
IMAGE_NAME="helloasso-renew-contribution"
JOB_NAME="helloasso-renew-contribution"
CRON_SCHEDULE="0 6 * * *"   # Daily at 6:00 AM
CRON_TIMEZONE="Europe/Paris"

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log()  { echo -e "${GREEN}[OK]${NC}  $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[ERR]${NC} $*"; }

# --- Pre-flight checks ---
echo ""
echo "=========================================="
echo "  Scaleway Serverless Jobs — Init Script"
echo "=========================================="
echo ""

if ! command -v scw &>/dev/null; then
  err "scw CLI not found. Install it: brew install scw  or  curl -s https://raw.githubusercontent.com/scaleway/scaleway-cli/master/scripts/get.sh | sh"
  exit 1
fi

if ! command -v docker &>/dev/null; then
  err "Docker not found. Please install Docker first."
  exit 1
fi

if [ -z "${SCW_DEFAULT_PROJECT_ID:-}" ]; then
  err "SCW_DEFAULT_PROJECT_ID is not set. Run 'scw init' or export the variable."
  exit 1
fi

log "scw CLI: $(scw version --output json | jq -r '.version')"
log "Project ID: $SCW_DEFAULT_PROJECT_ID"
log "Region: $REGION"

# -----------------------------------------------------------------------------
# Step 1: Create Container Registry namespace (idempotent)
# -----------------------------------------------------------------------------
echo ""
echo "--- Step 1: Container Registry namespace ---"

EXISTING_NS=$(scw registry namespace list name="$REGISTRY_NAMESPACE" region="$REGION" --output json | jq -r '.[0].id // empty')

if [ -n "$EXISTING_NS" ]; then
  log "Registry namespace '$REGISTRY_NAMESPACE' already exists (id: $EXISTING_NS)"
else
  log "Creating registry namespace '$REGISTRY_NAMESPACE'..."
  scw registry namespace create name="$REGISTRY_NAMESPACE" region="$REGION"
  log "Registry namespace created."
fi

# -----------------------------------------------------------------------------
# Step 2: Build and push Docker image
# -----------------------------------------------------------------------------
echo ""
echo "--- Step 2: Build and push Docker image ---"

scw registry login region="$REGION"
log "Logged in to Container Registry."

FULL_IMAGE="$REGISTRY/$REGISTRY_NAMESPACE/$IMAGE_NAME:latest"

docker build --platform linux/amd64 -t "$FULL_IMAGE" "$(git rev-parse --show-toplevel)"
docker push "$FULL_IMAGE"
log "Image pushed: $FULL_IMAGE"

# -----------------------------------------------------------------------------
# Step 3: Create or update Job Definition
# -----------------------------------------------------------------------------
echo ""
echo "--- Step 3: Job Definition ---"

JOB_ID=$(scw jobs definition list --output json | jq -r --arg name "$JOB_NAME" '.[] | select(.name == $name) | .id // empty')

if [ -n "$JOB_ID" ]; then
  warn "Job definition '$JOB_NAME' already exists (id: $JOB_ID). Updating image..."
  scw jobs definition update "$JOB_ID" image-uri="$FULL_IMAGE"
  log "Job definition updated."
else
  log "Creating job definition '$JOB_NAME'..."
  scw jobs definition create \
    name="$JOB_NAME" \
    cpu-limit=560 \
    memory-limit=1024 \
    local-storage-capacity=1024 \
    image-uri="$FULL_IMAGE" \
    region="$REGION" \
    environment-variables.HELLOASSO_API_ID="fake-helloasso-api-id" \
    environment-variables.HELLOASSO_API_SECRET="fake-helloasso-api-secret" \
    environment-variables.HELLOASSO_FROM_DATE="2024-01-01" \
    environment-variables.HELLOASSO_ORG_SLUG="fake-org-slug" \
    environment-variables.BASEROW_API_TOKEN="fake-baserow-api-token" \
    environment-variables.BASEROW_MEMBER_TABLE_ID="12345" \
    environment-variables.BREVO_API_KEY="fake-brevo-api-key"

  JOB_ID=$(scw jobs definition list --output json | jq -r --arg name "$JOB_NAME" '.[] | select(.name == $name) | .id')
  log "Job definition created (id: $JOB_ID)."
fi

# -----------------------------------------------------------------------------
# Step 4: Create Cron Trigger (idempotent)
# -----------------------------------------------------------------------------
echo ""
echo "--- Step 4: Cron Trigger ---"

EXISTING_TRIGGER=$(scw jobs trigger list job-definition-id="$JOB_ID" --output json | jq -r '.[0].id // empty')

if [ -n "$EXISTING_TRIGGER" ]; then
  warn "Trigger already exists (id: $EXISTING_TRIGGER). Skipping."
else
  log "Creating cron trigger (schedule: $CRON_SCHEDULE $CRON_TIMEZONE)..."
  scw jobs trigger create job-definition-id="$JOB_ID" \
    name="daily-renewal-check" \
    cron-config.schedule="$CRON_SCHEDULE" \
    cron-config.timezone="$CRON_TIMEZONE"
  log "Cron trigger created."
fi

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------
echo ""
echo "=========================================="
echo "  Setup Complete"
echo "=========================================="
echo ""
echo "  Registry:      $REGISTRY/$REGISTRY_NAMESPACE"
echo "  Image:         $FULL_IMAGE"
echo "  Job name:      $JOB_NAME"
echo "  Job ID:        $JOB_ID"
echo "  Schedule:      $CRON_SCHEDULE ($CRON_TIMEZONE)"
echo ""
echo "  ⚠️  Environment variables are set with FAKE values."
echo "     Update them in the Scaleway console:"
echo "     → Jobs → $JOB_NAME → Environment Variables"
echo ""
echo "  Or via CLI:"
echo "     scw jobs definition update $JOB_ID \\"
echo "       environment-variables.HELLOASSO_API_ID=\"real-value\" \\"
echo "       environment-variables.HELLOASSO_API_SECRET=\"real-value\" \\"
echo "       ..."
echo ""
echo "  Test run:"
echo "     scw jobs definition start $JOB_ID"
echo ""
