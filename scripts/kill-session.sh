#!/bin/bash

# Script to kill a specific kubrowser session by pod name or session ID

set -e

NAMESPACE="${NAMESPACE:-default}"

if [ -z "$1" ]; then
    echo "Usage: $0 <pod-name-or-session-id>"
    echo ""
    echo "Examples:"
    echo "  $0 kubrowser-session-20260111111823-abcdefgh-1768151903"
    echo "  $0 ca9a3260-022f-43ed-b9ce-62e70da52612"
    echo ""
    echo "To list all sessions:"
    echo "  kubectl get pods -l app=kubrowser"
    exit 1
fi

IDENTIFIER="$1"

# Try to find pod by name or label
POD_NAME=""

# Check if it looks like a pod name
if [[ "$IDENTIFIER" == kubrowser-session-* ]]; then
    POD_NAME="$IDENTIFIER"
else
    # Try to find pod by session-id label
    POD_NAME=$(kubectl get pods -n "$NAMESPACE" -l "app=kubrowser,session-id=$IDENTIFIER" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    
    # If not found, try as pod name anyway
    if [ -z "$POD_NAME" ]; then
        POD_NAME="$IDENTIFIER"
    fi
fi

# Verify pod exists
if ! kubectl get pod "$POD_NAME" -n "$NAMESPACE" &>/dev/null; then
    echo "Error: Pod '$POD_NAME' not found in namespace '$NAMESPACE'"
    exit 1
fi

echo "Deleting pod: $POD_NAME"
kubectl delete pod "$POD_NAME" -n "$NAMESPACE" --grace-period=0 --force

echo "Pod deleted successfully."
