#!/bin/bash

# vim: filetype=sh:tabstop=2:shiftwidth=2:expandtab

# http://www.kfirlavi.com/blog/2012/11/14/defensive-bash-programming/

readonly PROGNAME=$(basename $0)
readonly PROGDIR="$( cd "$(dirname "$0")" ; pwd -P )"
readonly ROOTDIR="$( cd "$PROGDIR" ; cd .. ; pwd -P )"
readonly TODAY=$(date +%Y%m%d)
readonly ARGS="$@"
readonly CERTS_DIR="$ROOTDIR/cmd/armor/test-fixtures/keys"

readonly VAULT_IMAGE_NAME="pinterb/vault"
readonly VAULT_IMAGE_TAG="0.6.2"
readonly VAULT_CONTAINER_NAME="vault-test-$TODAY"
readonly VAULT_CONTAINTER_ID=$(docker ps -a -q --filter="name=$CONTAINER_NAME")

readonly ARMOR_BIN="$ROOTDIR/bin/armor"
if [ -n "$VAULT_CONTAINTER_ID" ]; then
  echo "Found Vault container...stopping and removing id $VAULT_CONTAINTER_ID"
  docker rm $(docker stop "$VAULT_CONTAINTER_ID")
else
  echo "No vault container with a name of \"$VAULT_CONTAINER_NAME\" was found"
fi

readonly ARMOR_PID=$(pgrep armor)
if [ -n "$ARMOR_PID" ]; then
  echo ""
  echo ""
  echo "Found running armor...killing pid $ARMOR_PID)"
  kill -15 "$ARMOR_PID"
else
  echo "No armor process was found"
fi
