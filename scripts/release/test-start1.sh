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

random_bytes_file () {
    size=$1
    output=$2
    dd if=/dev/urandom of="$output" count=1 bs="$size" >/dev/null 2>&1
}

# TODO setup uplink config
uplink --config-dir "$GATEWAY_0_DIR" mb "sj://$BUCKET/"

# Upload some data
head -c 1K </dev/urandom   | uplink --config-dir "$GATEWAY_0_DIR" put "s3://$BUCKET/1K"
head -c 1M </dev/urandom   | uplink --config-dir "$GATEWAY_0_DIR" put "s3://$BUCKET/1M"
head -c 10M </dev/urandom  | uplink --config-dir "$GATEWAY_0_DIR" put "s3://$BUCKET/10M"
# head -c 512M </dev/urandom | uplink --config-dir "$GATEWAY_0_DIR" put "s3://$BUCKET/512M"

# Generate orders (for testing sattelite database migration)
uplink --config-dir "$GATEWAY_0_DIR" cp "s3://$BUCKET/1K"   /dev/null
uplink --config-dir "$GATEWAY_0_DIR" cp "s3://$BUCKET/1M"   /dev/null
uplink --config-dir "$GATEWAY_0_DIR" cp "s3://$BUCKET/10M"  /dev/null
# uplink --config-dir "$GATEWAY_0_DIR" cp "s3://$BUCKET/512M" /dev/null

sleep 30