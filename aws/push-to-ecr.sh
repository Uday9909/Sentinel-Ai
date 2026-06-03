#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# push-to-ecr.sh — Build, tag, and push Sentinel images to Amazon ECR.
#
# Prerequisites:
#   - AWS CLI v2 configured with appropriate IAM permissions
#   - Docker daemon running
#
# Usage:
#   chmod +x aws/push-to-ecr.sh
#   ./aws/push-to-ecr.sh
# ---------------------------------------------------------------------------
set -euo pipefail

# --- Configuration ---
REGION="${AWS_REGION:-us-east-1}"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
ECR_BASE="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"

SERVICES=(
  "sentinel/ingestion-service:ingestion-service"
  "sentinel/processing-service:processing-service"
  "sentinel/dashboard:dashboard"
)

echo "============================================"
echo "  Sentinel AI — ECR Push Script"
echo "  Account:  ${ACCOUNT_ID}"
echo "  Region:   ${REGION}"
echo "  Registry: ${ECR_BASE}"
echo "============================================"

# --- Step 1: Authenticate Docker with ECR ---
echo ""
echo ">>> Authenticating Docker with ECR..."
aws ecr get-login-password --region "${REGION}" \
  | docker login --username AWS --password-stdin "${ECR_BASE}"

# --- Step 2: Create ECR repositories if they don't exist ---
echo ""
echo ">>> Ensuring ECR repositories exist..."
for entry in "${SERVICES[@]}"; do
  REPO="${entry%%:*}"
  if aws ecr describe-repositories --repository-names "${REPO}" --region "${REGION}" > /dev/null 2>&1; then
    echo "  ✓ ${REPO} already exists"
  else
    echo "  → Creating ${REPO}..."
    aws ecr create-repository \
      --repository-name "${REPO}" \
      --region "${REGION}" \
      --image-scanning-configuration scanOnPush=true \
      --encryption-configuration encryptionType=AES256 \
      > /dev/null
    echo "  ✓ ${REPO} created"
  fi
done

# --- Step 3: Build, tag, and push images ---
echo ""
echo ">>> Building and pushing images..."
for entry in "${SERVICES[@]}"; do
  REPO="${entry%%:*}"
  DIR="${entry##*:}"

  echo ""
  echo "--- ${REPO} ---"
  echo "  Building from ./${DIR}..."
  docker build -t "${REPO}:latest" "./${DIR}"

  ECR_URI="${ECR_BASE}/${REPO}:latest"
  echo "  Tagging as ${ECR_URI}..."
  docker tag "${REPO}:latest" "${ECR_URI}"

  echo "  Pushing to ECR..."
  docker push "${ECR_URI}"
  echo "  ✓ ${REPO} pushed successfully"
done

echo ""
echo "============================================"
echo "  All images pushed to ECR successfully!"
echo ""
echo "  Next steps:"
echo "  1. Update image URIs in k8s/ manifests to:"
for entry in "${SERVICES[@]}"; do
  REPO="${entry%%:*}"
  echo "     ${ECR_BASE}/${REPO}:latest"
done
echo ""
echo "  2. Deploy to EKS:"
echo "     kubectl apply -k k8s/"
echo "============================================"
