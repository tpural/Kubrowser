#!/bin/bash

# Script to kill kubrowser sessions and clean up pods

set -e

NAMESPACE="${NAMESPACE:-default}"
LABEL_SELECTOR="app=kubrowser"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "Kubrowser Session Cleanup Script"
echo "=================================="
echo ""

# List current kubrowser pods
echo "Current kubrowser pods:"
kubectl get pods -n "$NAMESPACE" -l "$LABEL_SELECTOR" --no-headers 2>/dev/null | while read -r line; do
    if [ -n "$line" ]; then
        pod_name=$(echo "$line" | awk '{print $1}')
        status=$(echo "$line" | awk '{print $3}')
        echo "  - $pod_name ($status)"
    fi
done

pod_count=$(kubectl get pods -n "$NAMESPACE" -l "$LABEL_SELECTOR" --no-headers 2>/dev/null | wc -l | tr -d ' ')

if [ "$pod_count" -eq 0 ]; then
    echo -e "${GREEN}No kubrowser pods found.${NC}"
    exit 0
fi

echo ""
echo -e "${YELLOW}Found $pod_count kubrowser pod(s)${NC}"
echo ""

# Check for --all flag or interactive mode
if [ "$1" = "--all" ] || [ "$1" = "-a" ]; then
    echo "Deleting all kubrowser pods..."
    kubectl delete pods -n "$NAMESPACE" -l "$LABEL_SELECTOR" --grace-period=0 --force
    echo -e "${GREEN}All kubrowser pods deleted.${NC}"
elif [ "$1" = "--force" ] || [ "$1" = "-f" ]; then
    echo "Force deleting all kubrowser pods..."
    kubectl delete pods -n "$NAMESPACE" -l "$LABEL_SELECTOR" --grace-period=0 --force
    echo -e "${GREEN}All kubrowser pods force deleted.${NC}"
else
    # Interactive mode
    read -p "Do you want to delete all kubrowser pods? (y/N): " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Deleting kubrowser pods..."
        kubectl delete pods -n "$NAMESPACE" -l "$LABEL_SELECTOR" --grace-period=0 --force
        echo -e "${GREEN}Kubrowser pods deleted.${NC}"
    else
        echo -e "${YELLOW}Operation cancelled.${NC}"
        exit 0
    fi
fi

# Wait a moment and verify
sleep 2
remaining=$(kubectl get pods -n "$NAMESPACE" -l "$LABEL_SELECTOR" --no-headers 2>/dev/null | wc -l | tr -d ' ')

if [ "$remaining" -eq 0 ]; then
    echo -e "${GREEN}✓ All kubrowser pods have been cleaned up.${NC}"
else
    echo -e "${RED}⚠ Warning: $remaining pod(s) still exist.${NC}"
    echo "Remaining pods:"
    kubectl get pods -n "$NAMESPACE" -l "$LABEL_SELECTOR"
fi
