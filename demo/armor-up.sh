#!/bin/bash

# vim: filetype=sh:tabstop=2:shiftwidth=2:expandtab

readonly PROGNAME=$(basename "$0")
readonly PROGDIR="$( cd "$(dirname "$0")" ; pwd -P )"
readonly ARGS="$@"
readonly TODAY=$(date +%Y%m%d%H%M%S)

# find project root directory using git
#readonly PROJECT_ROOT=$(readlink -f $(git rev-parse --show-cdup))
readonly PROJECT_ROOT=$(cd $(git rev-parse --show-cdup) && pwd)

# pull in utils
[[ -f "$PROGDIR/utils.sh" ]] && source "$PROGDIR/utils.sh"

DEMO_VAULT_IMAGE_NAME=${DEMO_VAULT_IMAGE_NAME:-'pinterb/vault'}
DEMO_VAULT_IMAGE_TAG=${DEMO_VAULT_IMAGE_TAG:-'0.6.2'}
DEMO_VAULT_CONTAINER_NAME=${DEMO_VAULT_CONTAINER_NAME:-'demo_vault'}
DEMO_VAULT_CONFIG_DIR=${DEMO_VAULT_CONFIG_DIR:-"$PROGDIR/vconfig"}
DEMO_VAULT_TLS_DIR=${DEMO_VAULT_TLS_DIR:-"$PROGDIR/../cmd/armor/test-fixtures/keys"}
DEMO_ARMOR_BIN=${DEMO_ARMOR_BIN:-"$PROGDIR/../bin/amd64/armor"}

armor_up()
{
  bash -c "$DEMO_ARMOR_BIN --vault-ca-cert $DEMO_VAULT_TLS_DIR/ca-cert.pem"
}

vault_stop()
{
  matchingStarted=$(docker ps --filter="name=$DEMO_VAULT_CONTAINER_NAME" -q | xargs)
  [[ -n $matchingStarted ]] && echo "Stopping $matchingStarted" && docker stop $matchingStarted

  matching=$(docker ps -a --filter="name=$DEMO_VAULT_CONTAINER_NAME" -q | xargs)
  [[ -n $matching ]] && echo "Removing $matching" && docker rm $matching
}

# Make sure we have all the right stuff
prerequisites() {
  if ! command_exists docker; then
    error "docker does not appear to be installed. Please install and re-run this script."
    exit 1
  fi
}

main() {
  # Be unforgiving about errors
  set -euo pipefail
  readonly SELF="$(absolute_path "$0")"
  prerequisites

  armor_up
}

[[ "$0" == "$BASH_SOURCE" ]] && main
