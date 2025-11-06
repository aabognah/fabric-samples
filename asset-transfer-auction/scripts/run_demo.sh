#!/usr/bin/env bash
# Run a full demo of the asset-transfer-auction sample
# Usage: run_demo.sh [--reuse-network] [--redeploy-cc] [--client js|go] [--cleanup]
# - --reuse-network : use existing running test-network (do not bring it up)
# - --redeploy-cc   : redeploy chaincode (package/install/approve/commit)
# - --client        : choose client 'js' (default) or 'go'
# - --cleanup       : bring the network down at the end and remove wallets

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
TEST_NETWORK_DIR="$REPO_ROOT/../test-network"
APP_JS_DIR="$REPO_ROOT/application-javascript"
APP_GO_DIR="$REPO_ROOT/application-gateway-go"
CC_NAME="auction"
CC_PATH="../asset-transfer-auction/chaincode-go/"
CC_LANG="go"
CC_PEER_POLICY="OR('Org1MSP.peer','Org2MSP.peer')"

# defaults
REUSE_NETWORK=false
REDEPLOY_CC=false
CLIENT="js"
CLEANUP=false
# If true, reuse existing enrolled admins/users (wallets) instead of running enrollment scripts
REUSE_WALLETS=false

function usage() {
  cat <<EOF
Usage: $(basename "$0") [options]
Options:
  --reuse-network       Use an existing running test-network (don't bring it up)
  --redeploy-cc         Redeploy chaincode before running demo
  --client js|go        Choose client implementation to run demo (default: js)
  --cleanup             Bring down network and remove wallets at the end
  -h, --help            Show this help

Example:
  $(basename "$0") --redeploy-cc --client js
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --reuse-network) REUSE_NETWORK=true; shift ;;
    --redeploy-cc) REDEPLOY_CC=true; shift ;;
      --reuse-wallets) REUSE_WALLETS=true; shift ;;
    --client) CLIENT="$2"; shift 2 ;;
    --cleanup) CLEANUP=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown arg: $1"; usage; exit 1 ;;
  esac
done

echo "Demo settings: REUSE_NETWORK=$REUSE_NETWORK, REDEPLOY_CC=$REDEPLOY_CC, REUSE_WALLETS=$REUSE_WALLETS, CLIENT=$CLIENT, CLEANUP=$CLEANUP"

# helper to run a command and echo it
run() { echo "+ $*"; "$@"; }

# 1) Start network if needed
if [ "$REUSE_NETWORK" = false ]; then
  echo "Starting test-network (with CAs)..."
  pushd "$TEST_NETWORK_DIR" >/dev/null
  run ./network.sh up createChannel -ca
  popd >/dev/null
else
  echo "Reusing existing test-network"
fi

# 2) Deploy chaincode if requested
if [ "$REDEPLOY_CC" = true ]; then
  echo "Deploying chaincode ($CC_NAME) to test-network..."
  pushd "$TEST_NETWORK_DIR" >/dev/null
  run ./network.sh deployCC -ccn $CC_NAME -ccp $CC_PATH -ccl $CC_LANG -ccep "$CC_PEER_POLICY"
  popd >/dev/null
else
  echo "Skipping chaincode redeploy"
fi

# 3) Choose client flow
if [ "$CLIENT" = "js" ]; then
  echo "Running demo with JavaScript client"
  pushd "$APP_JS_DIR" >/dev/null

  echo "Installing Node dependencies (npm ci)..."
  if [ -f package-lock.json ]; then
    run npm ci
  else
    run npm install
  fi

  echo "Enrolling CA admins and registering users..."
  # If the user asked to reuse wallets, only skip enrollment/register when wallet directories exist.
  if [ "$REUSE_WALLETS" = true ]; then
    if [ -d "wallet/org1" ] && [ -d "wallet/org2" ]; then
      echo "Reusing existing wallets under $APP_JS_DIR/wallet (skipping enroll/register)"
    else
      echo "--reuse-wallets specified but wallets not found; running enrollment and registration"
      run node enrollAdmin.js org1
      run node enrollAdmin.js org2

      # register example users
      run node registerEnrollUser.js org1 seller
      run node registerEnrollUser.js org1 bidder1
      run node registerEnrollUser.js org1 bidder2
      run node registerEnrollUser.js org2 bidder3
      run node registerEnrollUser.js org2 bidder4
    fi
  else
    run node enrollAdmin.js org1
    run node enrollAdmin.js org2

    # register example users
    run node registerEnrollUser.js org1 seller
    run node registerEnrollUser.js org1 bidder1
    run node registerEnrollUser.js org1 bidder2
    run node registerEnrollUser.js org2 bidder3
    run node registerEnrollUser.js org2 bidder4
  fi

  echo "Creating asset via JS createAsset.js..."
  # create asset: asset1 blue 5 seller 1300
  run node createAsset.js org1 seller asset1 blue 5 seller 1300

  echo "Creating auction (seller)..."
  run node createAuction.js org1 seller auction1 asset1

  echo "Bidding and submitting bids"
  # bidder1
  OUT=$(node bid.js org1 bidder1 auction1 800)
  echo "$OUT"
  BID1=$(echo "$OUT" | grep -oE '[a-f0-9]{64}') || true
  echo "BidID1=$BID1"
  run node submitBid.js org1 bidder1 auction1 $BID1

  # bidder2
  OUT=$(node bid.js org1 bidder2 auction1 500)
  echo "$OUT"
  BID2=$(echo "$OUT" | grep -oE '[a-f0-9]{64}') || true
  echo "BidID2=$BID2"
  run node submitBid.js org1 bidder2 auction1 $BID2

  # bidder3
  OUT=$(node bid.js org2 bidder3 auction1 700)
  echo "$OUT"
  BID3=$(echo "$OUT" | grep -oE '[a-f0-9]{64}') || true
  echo "BidID3=$BID3"
  run node submitBid.js org2 bidder3 auction1 $BID3

  # bidder4
  OUT=$(node bid.js org2 bidder4 auction1 900)
  echo "$OUT"
  BID4=$(echo "$OUT" | grep -oE '[a-f0-9]{64}') || true
  echo "BidID4=$BID4"
  run node submitBid.js org2 bidder4 auction1 $BID4

  echo "Close auction (seller)"
  run node closeAuction.js org1 seller auction1

  echo "Reveal bids"
  run node revealBid.js org1 bidder1 auction1 $BID1
  run node revealBid.js org1 bidder2 auction1 $BID2
  run node revealBid.js org2 bidder3 auction1 $BID3
  run node revealBid.js org2 bidder4 auction1 $BID4

  echo "End auction (seller)"
  run node endAuction.js org1 seller auction1

  popd >/dev/null

elif [ "$CLIENT" = "go" ]; then
  echo "Running demo with Go gateway client"

  # For Go client, the sample expects identity files under the test-network crypto path.
  # We'll assume test-network has created admin/user identities (e.g. via cryptogen or CA).
  pushd "$APP_GO_DIR" >/dev/null

  echo "(Go) create asset"
  run go run assetTransfer.go createAsset asset1 blue 5 User1@org1.example.com 1300

  echo "(Go) create auction"
  run go run assetTransfer.go createAuction auction1 asset1

  echo "Note: The Go client cannot register/enroll users in this sample."
  echo "If you want to run bid/reveal flows with Go you must ensure the user identities exist in the test-network crypto material."

  popd >/dev/null
else
  echo "Unknown client: $CLIENT"; exit 1
fi

# 4) Cleanup if requested
if [ "$CLEANUP" = true ]; then
  echo "Cleaning up wallets and bringing down test-network"
  # remove wallets created by JS app
  rm -rf "$APP_JS_DIR/wallet" || true
  pushd "$TEST_NETWORK_DIR" >/dev/null
  run ./network.sh down
  popd >/dev/null
fi

echo "Demo finished"
