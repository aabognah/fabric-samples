
# Asset Transfer Auction

This sample demonstrates a sealed-bid auction for an asset on the Hyperledger Fabric network. The auction winner automatically receives the asset upon the auction's conclusion.

## Prerequisites

Before running this sample, you will need to have the following installed:

*   [Go](https://golang.org/)
*   [Docker](https://www.docker.com/)
*   [Hyperledger Fabric](https://hyperledger-fabric.readthedocs.io/en/latest/install.html)

## Running the sample

1.  Start the Fabric test network:

    ```bash
    cd fabric-samples/test-network
    ./network.sh up createChannel
    ./network.sh deployCC -ccn basic -ccp ../asset-transfer-auction/chaincode-go -ccl go
    ```

2.  Run the application:

    ```bash
    cd ../asset-transfer-auction/application-gateway-go
    go run assetTransfer.go
    ```

## How it works

The sample consists of a smart contract and a client application.

### Smart Contract

The smart contract is written in Go and is located in the `chaincode-go` directory. It combines the logic from the `asset-transfer-basic` and `auction-simple` samples.

The smart contract defines an `Asset` and an `Auction` struct. It provides the following functions:

*   `CreateAsset`: Creates a new asset.
*   `CreateAuction`: Creates a new auction for an asset.
*   `Bid`: Submits a bid for an auction.
*   `SubmitBid`: Submits the hash of the bid to the public chain.
*   `RevealBid`: Reveals a bid after the auction is closed.
*   `CloseAuction`: Closes an auction to new bids.
*   `EndAuction`: Ends an auction, determines the winner, and transfers the asset to the winner.

### Client Application

The client application is written in Go and is located in the `application-gateway-go` directory. It uses the Fabric Gateway to interact with the smart contract.

The application demonstrates the following workflow:

1.  Creates an asset.
2.  Creates an auction for the asset.
3.  Submits a bid for the auction.
4.  Closes the auction.
5.  Reveals the bid.
6.  Ends the auction.
7.  Verifies that the asset has been transferred to the auction winner.
