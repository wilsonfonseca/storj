#!/usr/bin/env bash
set -ueo pipefail
set +x

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

make -C "$SCRIPTDIR"/../.. install-sim

# setup tmpdir for testfiles and cleanup
TMP=$(mktemp -d -t tmp.XXXXXXXXXX)
cleanup(){
	rm -rf "$TMP"
}
trap cleanup EXIT

export STORJ_NETWORK_DIR=$TMP

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}
STORJ_SIM_POSTGRES=${STORJ_SIM_POSTGRES:-""}

# setup the network
# if postgres connection string is set as STORJ_SIM_POSTGRES then use that for testing
if [ -z ${STORJ_SIM_POSTGRES} ]; then
	storj-sim -x --satellites 2 --host $STORJ_NETWORK_HOST4 network setup
else
	storj-sim -x --satellites 2 --host $STORJ_NETWORK_HOST4 network --postgres=$STORJ_SIM_POSTGRES setup
fi

OUTPUT_DIR="release-output"
OUTPUT_PREFIX="$OUTPUT_DIR/output"

# make it more secure
rm -rf "$OUTPUT_DIR"
mkdir $OUTPUT_DIR

storj-sim -x --satellites 2 --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-start1.sh | tee "$OUTPUT_PREFIX-1.log"
grep ERROR "$OUTPUT_PREFIX-1.log" > "$OUTPUT_PREFIX-errors-1.log"
grep WARN  "$OUTPUT_PREFIX-1.log" > "$OUTPUT_PREFIX-warns-1.log"

storj-sim -x --satellites 2 --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-start2.sh | tee "$OUTPUT_PREFIX-2.log"
grep ERROR "$OUTPUT_PREFIX-2.log" > "$OUTPUT_PREFIX-errors-2.log"
grep ERROR "$OUTPUT_PREFIX-2.log" >  "$OUTPUT_PREFIX-warns-2.log"

# TODO take current month
# BEGINNING="$(date +"%Y-%m")-01"
satellite --config-dir $STORJ_NETWORK_DIR/satellite/0 reports storagenode-usage 2019-06-01 2019-07-01 | tee "$OUTPUT_PREFIX-payments.csv"

echo ""
echo ""
echo "First run summary:"
if grep 'piecestore:orderssender.*sending' -q "$OUTPUT_PREFIX-1.log"
then 
	echo " * Orders sent with first start"
fi
if grep 'GET_AUDIT' -q "$OUTPUT_PREFIX-1.log"
then 
	echo " * Audits sent with first start"
fi

echo "Second run summary:"
if grep 'piecestore:orderssender.*sending' -q "$OUTPUT_PREFIX-2.log"
then 
	echo " * Orders sent with second start"
fi
if grep 'GET_AUDIT' -q "$OUTPUT_PREFIX-2.log"
then 
	echo " * Audits sent with second start"
fi

storj-sim -x --satellites 2 --host $STORJ_NETWORK_HOST4 network destroy
