# Asset Transfer Auction Sample

This sample demonstrates a sealed-bid auction for an asset on the Hyperledger Fabric network, combining asset transfer with a secure auction mechanism. Bids are kept private until the auction period is over, and the auction winner automatically receives the asset upon conclusion.

## How it works

The auction process is designed to ensure privacy and fair play:

1.  **Auction Creation:** A seller creates an auction for an asset. The auction's endorsement policy is initially set to include only the seller's organization.
2.  **Two-Step Bidding:**
    *   **Bid:** Buyers submit their bids, which are stored in their respective organization's implicit private data collection. This keeps the bid amount confidential from other participants.
    *   **SubmitBid:** After creating a private bid, the buyer submits a cryptographic hash of their bid to the public ledger. Crucially, when a new organization submits a bid, its MSPID is added to the auction's endorsement policy. This means that any subsequent updates to the auction (including other bids, closing, or ending the auction) will require endorsement from all organizations that have submitted bids.
3.  **Closing the Auction:** The seller closes the auction, preventing any new bids.
4.  **Revealing Bids:** After the auction is closed, bidders can reveal their full bids. The chaincode verifies that the revealed bid matches the previously submitted hash and the private bid, ensuring integrity.
5.  **Ending the Auction:** The seller ends the auction. The chaincode determines the highest revealed bid, transfers the asset to the winner, and updates the auction status. The `EndAuction` transaction requires endorsement from all participating organizations, which prevents premature ending if there are unrevealed winning bids.

    Note: the chaincode includes a simple on-chain "balance" bookkeeping mechanism for demo purposes. When an auction is ended the chaincode will transfer the asset's `AppraisedValue` from the winning bidder's balance to the seller's balance and then transfer ownership of the asset.

## Prerequisites

Before running this sample, you will need to have the following installed:

*   [Go](https://golang.org/)
*   [Docker](https://www.docker.com/)
*   [Hyperledger Fabric](https://hyperledger-fabric.readthedocs.io/en/latest/install.html)

## Running the sample

### 1. Deploy the chaincode

Navigate to the `test-network` directory and bring up the network with Certificate Authorities (CAs):

```bash
cd fabric-samples/test-network
./network.sh up createChannel -ca
```

Deploy the `asset-transfer-auction` chaincode. We will override the default endorsement policy to allow any channel member to create an auction without requiring an endorsement from another organization initially. The endorsement policy will be dynamically updated as new organizations submit bids.

```bash
./network.sh deployCC -ccn auction -ccp ../asset-transfer-auction/chaincode-go/ -ccl go -ccep "OR('Org1MSP.peer','Org2MSP.peer')"
```

### 2. Application options (Go gateway vs JavaScript app)

This repository provides two client-side applications that can be used to
interact with the `auction` chaincode. Choose one of the following options
for running the demo:

- Go gateway application (`application-gateway-go`) — a single-file Go client
    that demonstrates Gateway usage for synchronous/asynchronous submits. It
    can be useful for examples and production-style clients.
- JavaScript application (`application-javascript`) — a collection of Node.js
    scripts that include CA helpers (enroll/register) and transaction scripts
    (createAuction, bid, submitBid, revealBid, etc.). This is the recommended
    option for running the sample demo because it includes provisioning helpers
    that work with the sample `test-network`.

Pick the section below that matches your preferred client (Go or JavaScript).

#### A. Using the Go gateway application (optional)

1. Install Go dependencies and build/run the app:

```bash
cd asset-transfer-auction/application-gateway-go
go mod tidy
```

2. Identities and wallets

The Go gateway client reads identities directly from the `test-network`
crypto material (for example `test-network/organizations/peerOrganizations/org1.example.com/users/User1@org1.example.com/msp/...`).
You can provision identities either by using the `test-network` crypto
generation scripts or by provisioning identities with the JavaScript
helpers (see the JavaScript section). The Go app contains placeholder
`enrollAdmin`/`registerUser` commands that print guidance and do not perform
CA operations. If you want the Go client to use the identities created by
the JavaScript helpers, copy the certificate and private key files into the
expected `users/<User>@<org>.example.com/msp/...` directory structure or run
the Go client using the admin/user files produced by the `test-network`.

3. Example Go commands

From `asset-transfer-auction/application-gateway-go` you can run:

```bash
# create an asset
go run assetTransfer.go createAsset asset1 blue 5 User1@org1.example.com 1300

# create an auction (seller must be the identity used by the Go client)
go run assetTransfer.go createAuction auction1 asset1

# bid/reveal/close/end (use the same usage printed by the program)
go run assetTransfer.go bid org1 User1@org1.example.com auction1 800
```

Note: the Go client expects identity files in the sample `test-network`
layout. If you prefer to have the Go code perform CA enroll/register, you
can implement Fabric CA client flows in Go or invoke the JavaScript scripts
externally.

#### B. Using the JavaScript application (recommended)

1. Install Node dependencies:

```bash
cd asset-transfer-auction/application-javascript
npm install
```

2. Enroll CA admins and register users

Bring up the network with CAs (if not already running):

```bash
cd ../../test-network
./network.sh up createChannel -ca
```

Then run the provisioning scripts (from the JS app directory):

```bash
cd ../asset-transfer-auction/application-javascript

# Enroll Org1 admin
node enrollAdmin.js org1

# Enroll Org2 admin
node enrollAdmin.js org2

# Register and enroll users (examples)
node registerEnrollUser.js org1 seller
node registerEnrollUser.js org1 bidder1
node registerEnrollUser.js org1 bidder2
node registerEnrollUser.js org2 bidder3
node registerEnrollUser.js org2 bidder4
```

Wallets are created under `application-javascript/wallet/org1` and
`application-javascript/wallet/org2`.

3. Run the auction demo (JS scripts)

Create asset (quickest via Go or create your own JS script). If you used the
Go `createAsset` earlier, skip this. Otherwise create an asset with your
preferred client.

Create an auction (seller on Org1):

```bash
node createAuction.js org1 seller auction1 asset1
```

Two-step bidding (example):

```bash
# Bidder1 (Org1)
node bid.js org1 bidder1 auction1 800
# Save the BidID printed by the script
export BIDDER1_BID_ID=<BidID from output>
node submitBid.js org1 bidder1 auction1 $BIDDER1_BID_ID

# Bidder2 (Org1)
node bid.js org1 bidder2 auction1 500
export BIDDER2_BID_ID=<BidID from output>
node submitBid.js org1 bidder2 auction1 $BIDDER2_BID_ID

# Bidder3 (Org2)
node bid.js org2 bidder3 auction1 700
export BIDDER3_BID_ID=<BidID from output>
node submitBid.js org2 bidder3 auction1 $BIDDER3_BID_ID

# Bidder4 (Org2)
node bid.js org2 bidder4 auction1 900
export BIDDER4_BID_ID=<BidID from output>
node submitBid.js org2 bidder4 auction1 $BIDDER4_BID_ID
```

Close, reveal, and end the auction:

```bash
# Close auction (seller)
node closeAuction.js org1 seller auction1

# Reveal bids (each bidder)
node revealBid.js org1 bidder1 auction1 $BIDDER1_BID_ID
node revealBid.js org2 bidder3 auction1 $BIDDER3_BID_ID
node revealBid.js org2 bidder4 auction1 $BIDDER4_BID_ID

# End auction (seller)
node endAuction.js org1 seller auction1
```

Balance transfer and helper scripts

When `endAuction` is executed the chaincode will attempt to move the asset's `AppraisedValue` from the buyer to the seller before transferring ownership. If the buyer does not have enough balance the `endAuction` call will fail.

For convenience the sample provides a couple of helper JS scripts to inspect balances and assets:

- `getMyBalance.js` — calls chaincode `GetMyBalance` and returns the balance for the submitting client identity (no owner argument required).
- `queryAllAssets.js` — calls `GetAllAssets` and attempts to decode the `Owner` (serialized identity) into a friendly label by scanning the local wallets; it adds an `OwnerDecoded` field to the printed output.

Example: check balances before and after ending the auction (run from `application-javascript`):

```
# check seller balance
node getMyBalance.js org1 seller

# check bidder4 (likely winner) balance
node getMyBalance.js org2 bidder4

# end auction
node endAuction.js org1 seller auction1

# re-check balances
node getMyBalance.js org1 seller
node getMyBalance.js org2 bidder4
```

4. Notes on the JS application

The JS scripts use the helper utilities in `test-application/javascript`
(`AppUtil.js` and `CAUtil.js`) and the standard `test-network`
connection-profile layout. If you change the network layout or TLS/ports, edit
the helpers accordingly.

### 3. Clean up

When done, remove the wallets and bring down the network:

```bash
rm -rf asset-transfer-auction/application-javascript/wallet
cd test-network
./network.sh down
```

Tip: the repository includes a convenience demo script `scripts/run_demo.sh`. It supports a `--reuse-wallets` flag that will skip the JS CA enroll/register steps when wallets already exist under `application-javascript/wallet`:

```
./scripts/run_demo.sh --redeploy-cc --client js --reuse-wallets
```