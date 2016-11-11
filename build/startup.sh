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

# First shutdown anything currently running...
sh "$PROGDIR/shutdown.sh"

echo ""
echo "Starting a vault container named \"$VAULT_CONTAINER_NAME\""
docker run --cap-add=IPC_LOCK --name "$VAULT_CONTAINER_NAME" -d \
  -v $CERTS_DIR:/vault/tls \
  -e "VAULT_LOCAL_CONFIG=$(cat $PROGDIR/vault-config.json)" \
  -e "VAULT_CACERT=/vault/tls/ca-cert.pem" \
  -p 8200:8200 \
  "$VAULT_IMAGE_NAME:$VAULT_IMAGE_TAG" server >/dev/null

echo "New vault container started. Id: $(docker ps -a -q \
  --filter="name=$CONTAINER_NAME")"
echo ""

if [ -n "$ARGS" ]; then
  $ARMOR_BIN "$ARGS"
else
  $ARMOR_BIN --vault-ca-cert "$CERTS_DIR/ca-cert.pem"
fi
