/*
Copyright 2021 IBM All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/hash"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	mspID string
	cryptoPath string
	certPath string
	keyPath string
	tlsCertPath string
	peerEndpoint string
	gatewayPeer string
)

func setAndGetConnectionDetails(org string, user string) {
	switch org {
	case "org1":
		mspID = "Org1MSP"
		cryptoPath = "../../test-network/organizations/peerOrganizations/org1.example.com"
		peerEndpoint = "localhost:7051"
		gatewayPeer = "peer0.org1.example.com"
	case "org2":
		mspID = "Org2MSP"
		cryptoPath = "../../test-network/organizations/peerOrganizations/org2.example.com"
		peerEndpoint = "localhost:9051"
		gatewayPeer = "peer0.org2.example.com"
	default:
		panic(fmt.Errorf("unknown organization: %s", org))
	}

	if user == "admin" {
		certPath = cryptoPath + "/users/Admin@" + org + ".example.com/msp/signcerts"
		keyPath = cryptoPath + "/users/Admin@" + org + ".example.com/msp/keystore"
	} else {
		certPath = cryptoPath + "/users/" + user + "@" + org + ".example.com/msp/signcerts"
		keyPath = cryptoPath + "/users/" + user + "@" + org + ".example.com/msp/keystore"
	}
	tlsCertPath = cryptoPath + "/peers/peer0." + org + ".example.com/tls/ca.crt"
}

var now = time.Now()
var assetId = fmt.Sprintf("asset%d", now.Unix()*1e3+int64(now.Nanosecond())/1e6)
var auctionId = fmt.Sprintf("auction%d", now.Unix()*1e3+int64(now.Nanosecond())/1e6)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "enrollAdmin":
		if len(os.Args) != 3 {
			fmt.Println("Usage: go run assetTransfer.go enrollAdmin <org>")
			os.Exit(1)
		}
		org := os.Args[2]
		enrollAdmin(org)
	case "registerUser":
		if len(os.Args) != 4 {
			fmt.Println("Usage: go run assetTransfer.go registerUser <org> <user>")
			os.Exit(1)
		}
		org := os.Args[2]
		user := os.Args[3]
		registerUser(org, user)
	case "initLedger":
		if len(os.Args) != 2 {
			fmt.Println("Usage: go run assetTransfer.go initLedger")
			os.Exit(1)
		}
		runApplication(command, "org1", "User1") // Default user for initLedger
	case "createAsset":
		if len(os.Args) != 7 {
			fmt.Println("Usage: go run assetTransfer.go createAsset <id> <color> <size> <owner> <appraisedValue>")
			os.Exit(1)
		}
		runApplication(command, "org1", "User1", os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6])
	case "createAuction":
		if len(os.Args) != 4 {
			fmt.Println("Usage: go run assetTransfer.go createAuction <auctionID> <itemSold>")
			os.Exit(1)
		}
		runApplication(command, "org1", "User1", os.Args[2], os.Args[3])
	case "bid":
		if len(os.Args) != 6 {
			fmt.Println("Usage: go run assetTransfer.go bid <org> <user> <auctionID> <price>")
			os.Exit(1)
		}
		org := os.Args[2]
		user := os.Args[3]
		auctionID := os.Args[4]
		price := os.Args[5]
		runApplication(command, org, user, auctionID, price)
	case "submitBid":
		if len(os.Args) != 6 {
			fmt.Println("Usage: go run assetTransfer.go submitBid <org> <user> <auctionID> <txID>")
			os.Exit(1)
		}
		org := os.Args[2]
		user := os.Args[3]
		auctionID := os.Args[4]
		txID := os.Args[5]
		runApplication(command, org, user, auctionID, txID)
	case "closeAuction":
		if len(os.Args) != 5 {
			fmt.Println("Usage: go run assetTransfer.go closeAuction <org> <user> <auctionID>")
			os.Exit(1)
		}
		org := os.Args[2]
		user := os.Args[3]
		auctionID := os.Args[4]
		runApplication(command, org, user, auctionID)
	case "revealBid":
		if len(os.Args) != 7 {
			fmt.Println("Usage: go run assetTransfer.go revealBid <org> <user> <auctionID> <txID> <price>")
			os.Exit(1)
		}
		org := os.Args[2]
		user := os.Args[3]
		auctionID := os.Args[4]
		txID := os.Args[5]
		price := os.Args[6]
		runApplication(command, org, user, auctionID, txID, price)
	case "endAuction":
		if len(os.Args) != 5 {
			fmt.Println("Usage: go run assetTransfer.go endAuction <org> <user> <auctionID>")
			os.Exit(1)
		}
		org := os.Args[2]
		user := os.Args[3]
		auctionID := os.Args[4]
		runApplication(command, org, user, auctionID)
	case "getAllAssets":
		if len(os.Args) != 3 {
			fmt.Println("Usage: go run assetTransfer.go getAllAssets <org>")
			os.Exit(1)
		}
		org := os.Args[2]
		runApplication(command, org, "User1") // Default user for querying
	case "readAsset":
		if len(os.Args) != 4 {
			fmt.Println("Usage: go run assetTransfer.go readAsset <org> <assetID>")
			os.Exit(1)
		}
		org := os.Args[2]
		assetID := os.Args[3]
		runApplication(command, org, "User1", assetID) // Default user for querying
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  go run assetTransfer.go enrollAdmin <org>")
	fmt.Println("  go run assetTransfer.go registerUser <org> <user>")
	fmt.Println("  go run assetTransfer.go initLedger")
	fmt.Println("  go run assetTransfer.go createAsset <id> <color> <size> <owner> <appraisedValue>")
	fmt.Println("  go run assetTransfer.go createAuction <auctionID> <itemSold>")
	fmt.Println("  go run assetTransfer.go bid <org> <user> <auctionID> <price>")
	fmt.Println("  go run assetTransfer.go submitBid <org> <user> <auctionID> <txID>")
	fmt.Println("  go run assetTransfer.go closeAuction <org> <user> <auctionID>")
	fmt.Println("  go run assetTransfer.go revealBid <org> <user> <auctionID> <txID> <price>")
	fmt.Println("  go run assetTransfer.go endAuction <org> <user> <auctionID>")
	fmt.Println("  go run assetTransfer.go getAllAssets <org>")
	fmt.Println("  go run assetTransfer.go readAsset <org> <assetID>")
}

func runApplication(command string, org string, user string, args ...string) {
	setAndGetConnectionDetails(org, user)

	// The gRPC client connection should be shared by all Gateway connections to this endpoint
	clientConnection := newGrpcConnection()
	defer clientConnection.Close()

	id := newIdentity()
	sign := newSign()

	// Create a Gateway connection for a specific client identity
	gw, err := client.Connect(
		id,
		client.WithSign(sign),
		client.WithHash(hash.SHA256),
		client.WithClientConnection(clientConnection),
		// Default timeouts for different gRPC calls
		client.WithEvaluateTimeout(5*time.Second),
		client.WithEndorseTimeout(15*time.Second),
		client.WithSubmitTimeout(5*time.Second),
		client.WithCommitStatusTimeout(1*time.Minute),
	)
	if err != nil {
		panic(err)
	}
	defer gw.Close()

	// Override default values for chaincode and channel name as they may differ in testing contexts.
	chaincodeName := "auction" // Changed from "basic" to "auction"
	if ccname := os.Getenv("CHAINCODE_NAME"); ccname != "" {
		chaincodeName = ccname
	}

	channelName := "mychannel"
	if cname := os.Getenv("CHANNEL_NAME"); cname != "" {
		channelName = cname
	}

	network := gw.GetNetwork(channelName)
	contract := network.GetContract(chaincodeName)

	switch command {
	case "initLedger":
		initLedger(contract)
	case "createAsset":
		createAsset(contract, args[0], args[1], args[2], args[3], args[4])
	case "createAuction":
		createAuction(contract, args[0], args[1])
	case "bid":
		price, _ := strconv.Atoi(args[2])
		bid(contract, args[0], price, user, org)
	case "submitBid":
		submitBid(contract, args[0], args[1], user, org)
	case "closeAuction":
		closeAuction(contract, args[0], user, org)
	case "revealBid":
		price, _ := strconv.Atoi(args[3])
		revealBid(contract, args[0], args[1], price, user, org)
	case "endAuction":
		endAuction(contract, args[0], user, org)
	case "getAllAssets":
		getAllAssets(contract)
	case "readAsset":
		readAssetByID(contract, args[0])
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

// newGrpcConnection creates a gRPC connection to the Gateway server.
func newGrpcConnection() *grpc.ClientConn {
	certificatePEM, err := os.ReadFile(tlsCertPath)
	if err != nil {
		panic(fmt.Errorf("failed to read TLS certifcate file: %w", err))
	}

	certificate, err := identity.CertificateFromPEM(certificatePEM)
	if err != nil {
		panic(err)
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(certificate)
	transportCredentials := credentials.NewClientTLSFromCert(certPool, gatewayPeer)

	connection, err := grpc.NewClient(peerEndpoint, grpc.WithTransportCredentials(transportCredentials))
	if err != nil {
		panic(fmt.Errorf("failed to create gRPC connection: %w", err))
	}

	return connection
}

// newIdentity creates a client identity for this Gateway connection using an X.509 certificate.
func newIdentity() *identity.X509Identity {
	certificatePEM, err := readFirstFile(certPath)
	if err != nil {
		panic(fmt.Errorf("failed to read certificate file: %w", err))
	}

	certificate, err := identity.CertificateFromPEM(certificatePEM)
	if err != nil {
		panic(err)
	}

	id, err := identity.NewX509Identity(mspID, certificate)
	if err != nil {
		panic(err)
	}

	return id
}

// newSign creates a function that generates a digital signature from a message digest using a private key.
func newSign() identity.Sign {
	privateKeyPEM, err := readFirstFile(keyPath)
	if err != nil {
		panic(fmt.Errorf("failed to read private key file: %w", err))
	}

	privateKey, err := identity.PrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		panic(err)
	}

	sign, err := identity.NewPrivateKeySign(privateKey)
	if err != nil {
		panic(err)
	}

	return sign
}

func readFirstFile(dirPath string) ([]byte, error) {
	dir, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}

	fileNames, err := dir.Readdirnames(1)
	if err != nil {
		return nil, err
	}

	return os.ReadFile(path.Join(dirPath, fileNames[0]))
}

// enrollAdmin is a convenience placeholder to match the CLI used in the README.
// The Go application gateway in these samples does not include a Fabric CA client
// implementation for registering/enrolling identities. Identities are typically
// provisioned by the `test-network` scripts or by the JavaScript helper
// scripts in the samples (see README). This function prints guidance instead
// of attempting enrollment.
func enrollAdmin(org string) {
	fmt.Printf("\nNote: this sample's Go application does not implement CA enrollment.\n")
	fmt.Printf("Please use the test-network scripts or the JavaScript enrollment helper.\n")
	fmt.Printf("For example, from the `fabric-samples` directory run:\n")
	fmt.Printf("  cd test-network && ./network.sh up createChannel -ca\n")
	fmt.Printf("Then run one of the provided enroll scripts, for example:\n")
	fmt.Printf("  node ../asset-transfer-auction/application-javascript/enrollAdmin.js org1\n")
	fmt.Printf("or use the CA utilities under test-application/javascript.\n\n")
}

// registerUser is a placeholder that mirrors the README command but delegates
// identity provisioning to the JavaScript helpers or the network scripts.
func registerUser(org string, user string) {
	fmt.Printf("\nNote: this sample's Go application does not implement CA registration/enrollment for users.\n")
	fmt.Printf("Please use one of the sample JavaScript scripts to register and enroll users.\n")
	fmt.Printf("For example:\n")
	fmt.Printf("  node ../asset-transfer-auction/application-javascript/registerEnrollUser.js %s %s\n", org, user)
	fmt.Printf("or use the helpers in test-application/javascript (buildCAClient/registerAndEnrollUser).\n\n")
}

// This type of transaction would typically only be run once by an application the first time it was started after its
// initial deployment. A new version of the chaincode deployed later would likely not need to run an "init" function.
func initLedger(contract *client.Contract) {
	fmt.Printf("\n--> Submit Transaction: InitLedger, function creates the initial set of assets on the ledger \n")

	_, err := contract.SubmitTransaction("InitLedger")
	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

// Evaluate a transaction to query ledger state.
func getAllAssets(contract *client.Contract) {
	fmt.Println("\n--> Evaluate Transaction: GetAllAssets, function returns all the current assets on the ledger")

	evaluateResult, err := contract.EvaluateTransaction("GetAllAssets")
	if err != nil {
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}
	result := formatJSON(evaluateResult)

	fmt.Printf("*** Result:%s\n", result)
}

// Submit a transaction synchronously, blocking until it has been committed to the ledger.
func createAsset(contract *client.Contract) {
	fmt.Printf("\n--> Submit Transaction: CreateAsset, creates new asset with ID, Color, Size, Owner and AppraisedValue arguments \n")

	_, err := contract.SubmitTransaction("CreateAsset", assetId, "yellow", "5", "User1@org1.example.com", "1300")
	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

// Evaluate a transaction by assetID to query ledger state.
func readAssetByID(contract *client.Contract) {
	fmt.Printf("\n--> Evaluate Transaction: ReadAsset, function returns asset attributes\n")

	evaluateResult, err := contract.EvaluateTransaction("ReadAsset", assetId)
	if err != nil {
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}
	result := formatJSON(evaluateResult)

	fmt.Printf("*** Result:%s\n", result)
}

// Submit a transaction synchronously, blocking until it has been committed to the ledger.
func createAuction(contract *client.Contract, auctionID string, itemSold string) {
	fmt.Printf("\n--> Submit Transaction: CreateAuction, creates new auction with ID and itemSold arguments \n")

	_, err := contract.SubmitTransaction("CreateAuction", auctionID, itemSold)
	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

// Bid is used to add a user's bid to the auction.
func bid(contract *client.Contract, auctionID string, price int) (string, error) {
	fmt.Printf("\n--> Submit Transaction: Bid, creates new bid on an auction \n")

	transientData := make(map[string][]byte)
	bidJSON, err := json.Marshal(map[string]interface{}{
		"price":  price,
		"org":    mspID,
		"bidder": "User1@org1.example.com",
	})
	if err != nil {
		return "", err
	}
	transientData["bid"] = bidJSON

	submitResult, _, err := contract.SubmitAsync("Bid", client.WithTransient(transientData), client.WithArguments(auctionID), client.WithEndorsingOrganizations(mspID))
	if err != nil {
		return "", err
	}

	return string(submitResult), nil
}

// SubmitBid is used by the bidder to add the hash of that bid stored in private data to the auction.
func submitBid(contract *client.Contract, auctionID string, txID string) {
	fmt.Printf("\n--> Submit Transaction: SubmitBid, submits a bid to an auction \n")

	_, err := contract.SubmitTransaction("SubmitBid", auctionID, txID)
	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

// RevealBid is used by a bidder to reveal their bid after the auction is closed.
func revealBid(contract *client.Contract, auctionID string, txID string) {
	fmt.Printf("\n--> Submit Transaction: RevealBid, reveals a bid on an auction \n")

	transientData := make(map[string][]byte)
	bidJSON, err := json.Marshal(map[string]interface{}{
		"price":  1500,
		"org":    mspID,
		"bidder": "User1@org1.example.com",
	})
	if err != nil {
		panic(err)
	}
	transientData["bid"] = bidJSON

	_, err = contract.Submit("RevealBid", client.WithArguments(auctionID, txID), client.WithTransient(transientData))
	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

// CloseAuction can be used by the seller to close the auction.
func closeAuction(contract *client.Contract, auctionID string) {
	fmt.Printf("\n--> Submit Transaction: CloseAuction, closes an auction \n")

	_, err := contract.SubmitTransaction("CloseAuction", auctionID)
	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

// EndAuction both changes the auction status to closed and calculates the winners of the auction.
func endAuction(contract *client.Contract, auctionID string) {
	fmt.Printf("\n--> Submit Transaction: EndAuction, ends an auction and transfers the asset \n")

	_, err := contract.SubmitTransaction("EndAuction", auctionID)
	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

// Format JSON data
func formatJSON(data []byte) string {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, data, "", "  "); err != nil {
		panic(fmt.Errorf("failed to parse JSON: %w", err))
	}
	return prettyJSON.String()
}
