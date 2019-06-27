#!/usr/bin/env bash
set -ueo pipefail

TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)

cleanup(){
    rm -rf "$TMPDIR"
    echo "cleaned up test successfully"
}

trap cleanup EXIT

BUCKET=release-bucket
SRC_DIR=$TMPDIR/source
DST_DIR=$TMPDIR/dst

mkdir -p "$SRC_DIR" "$DST_DIR"

# Generate more orders (for testing storage node database migration)
uplink --config-dir "$GATEWAY_0_DIR" cp "s3://$BUCKET/1K"   /dev/null
uplink --config-dir "$GATEWAY_0_DIR" cp "s3://$BUCKET/1M"   /dev/null
uplink --config-dir "$GATEWAY_0_DIR" cp "s3://$BUCKET/10M"  /dev/null
# uplink --config-dir "$GATEWAY_0_DIR" cp "s3://$BUCKET/512M" /dev/null

sleep 30