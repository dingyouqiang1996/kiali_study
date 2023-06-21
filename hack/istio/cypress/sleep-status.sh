#!/bin/bash
##############################################################################
# sleep-status.sh
#
# Checks, if the Sleep application is installed. This script is used by the
# Cypress sleep before hook (frontend/cypress/integration/common/hooks.ts).
# 
##############################################################################
set -e

input=$(kubectl get pods -n sleep -o=custom-columns=NAME:.metadata.name,Status:.status.phase --no-headers=true)

count=$(echo "$input" | grep sleep | awk -F ' ' '{print $1}' | wc -l)
status=$(echo "$input" | grep sleep | awk -F ' ' '{print $2}')
  if [ $count -ne 1 ] || [ $status != "Running" ]
  then
    echo "Invalid number of pods in a Running state detected."
    exit 1
  fi
done

echo "Sleep app ready."
exit 0
