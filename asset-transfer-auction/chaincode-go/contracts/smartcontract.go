package chaincode

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"regexp"
	"strings"

	"github.com/hyperledger/fabric-chaincode-go/v2/pkg/statebased"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
	"github.com/hyperledger/fabric-protos-go-apiv2/msp"
	"google.golang.org/protobuf/proto"
)

// SmartContract provides functions for managing an Asset
type SmartContract struct {
	contractapi.Contract
}

// Asset describes basic details of what makes up a simple asset
type Asset struct {
	AppraisedValue int    `json:"AppraisedValue"`
	Color          string `json:"Color"`
	ID             string `json:"ID"`
	Owner          string `json:"Owner"`
	// OwnerLabel is a short, human-friendly label for the owner (e.g. "org1/seller")
	OwnerLabel     string `json:"OwnerLabel"`
	Size           int    `json:"Size"`
}

// Auction data
type Auction struct {
	Type         string             `json:"objectType"`
	ItemSold     string             `json:"item"`
	Seller       string             `json:"seller"`
	Orgs         []string           `json:"organizations"`
	PrivateBids  map[string]BidHash `json:"privateBids"`
	RevealedBids map[string]FullBid `json:"revealedBids"`
	Winner       string             `json:"winner"`
	Price        int                `json:"price"`
	Status       string             `json:"status"`
}

// FullBid is the structure of a revealed bid
type FullBid struct {
	Type   string `json:"objectType"`
	Price  int    `json:"price"`
	Org    string `json:"org"`
	Bidder string `json:"bidder"`
}

// BidHash is the structure of a private bid
type BidHash struct {
	Org  string `json:"org"`
	Hash string `json:"hash"`
}

const bidKeyType = "bid"

// InitLedger adds a base set of assets to the ledger
func (s *SmartContract) InitLedger(ctx contractapi.TransactionContextInterface) error {
	assets := []Asset{
		{ID: "asset1", Color: "blue", Size: 5, Owner: "Tomoko", OwnerLabel: "Tomoko", AppraisedValue: 300},
		{ID: "asset2", Color: "red", Size: 5, Owner: "Brad", OwnerLabel: "Brad", AppraisedValue: 400},
		{ID: "asset3", Color: "green", Size: 10, Owner: "Jin Soo", OwnerLabel: "Jin Soo", AppraisedValue: 500},
		{ID: "asset4", Color: "yellow", Size: 10, Owner: "Max", OwnerLabel: "Max", AppraisedValue: 600},
		{ID: "asset5", Color: "black", Size: 15, Owner: "Adriana", OwnerLabel: "Adriana", AppraisedValue: 700},
		{ID: "asset6", Color: "white", Size: 15, Owner: "Michel", OwnerLabel: "Michel", AppraisedValue: 800},
	}

	for _, asset := range assets {
		assetJSON, err := json.Marshal(asset)
		if err != nil {
			return err
		}

		err = ctx.GetStub().PutState(asset.ID, assetJSON)
		if err != nil {
			return fmt.Errorf("failed to put to world state. %v", err)
		}
	}

	return nil
}

// CreateAsset issues a new asset to the world state with given details.
func (s *SmartContract) CreateAsset(ctx contractapi.TransactionContextInterface, id string, color string, size int, owner string, appraisedValue int) error {
	exists, err := s.AssetExists(ctx, id)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("the asset %s already exists", id)
	}

	// Derive an owner label where possible (attempt to decode serialized identity)
	ownerLabel := deriveOwnerLabel(owner)

	asset := Asset{
		ID:             id,
		Color:          color,
		Size:           size,
		Owner:          owner,
		OwnerLabel:     ownerLabel,
		AppraisedValue: appraisedValue,
	}
	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(id, assetJSON)
}

// ReadAsset returns the asset stored in the world state with given id.
func (s *SmartContract) ReadAsset(ctx contractapi.TransactionContextInterface, id string) (*Asset, error) {
	assetJSON, err := ctx.GetStub().GetState(id)
	if err != nil {
		return nil, fmt.Errorf("failed to read from world state: %v", err)
	}
	if assetJSON == nil {
		return nil, fmt.Errorf("the asset %s does not exist", id)
	}

	var asset Asset
	err = json.Unmarshal(assetJSON, &asset)
	if err != nil {
		return nil, err
	}

	return &asset, nil
}

// UpdateAsset updates an existing asset in the world state with provided parameters.
func (s *SmartContract) UpdateAsset(ctx contractapi.TransactionContextInterface, id string, color string, size int, owner string, appraisedValue int) error {
	exists, err := s.AssetExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("the asset %s does not exist", id)
	}

	// overwriting original asset with new asset
	ownerLabel := deriveOwnerLabel(owner)

	asset := Asset{
		ID:             id,
		Color:          color,
		Size:           size,
		Owner:          owner,
		OwnerLabel:     ownerLabel,
		AppraisedValue: appraisedValue,
	}
	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(id, assetJSON)
}

// DeleteAsset deletes an given asset from the world state.
func (s *SmartContract) DeleteAsset(ctx contractapi.TransactionContextInterface, id string) error {
	exists, err := s.AssetExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("the asset %s does not exist", id)
	}

	return ctx.GetStub().DelState(id)
}

// AssetExists returns true when asset with given ID exists in world state
func (s *SmartContract) AssetExists(ctx contractapi.TransactionContextInterface, id string) (bool, error) {
	assetJSON, err := ctx.GetStub().GetState(id)
	if err != nil {
		return false, fmt.Errorf("failed to read from world state: %v", err)
	}

	return assetJSON != nil, nil
}

// TransferAsset updates the owner field of asset with given id in world state, and returns the old owner.
func (s *SmartContract) TransferAsset(ctx contractapi.TransactionContextInterface, id string, newOwner string) (string, error) {
	asset, err := s.ReadAsset(ctx, id)
	if err != nil {
		return "", err
	}

	oldOwner := asset.Owner
	asset.Owner = newOwner
	// update OwnerLabel as well
	asset.OwnerLabel = deriveOwnerLabel(newOwner)

	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return "", err
	}

	err = ctx.GetStub().PutState(id, assetJSON)
	if err != nil {
		return "", err
	}

	return oldOwner, nil
}

// GetAllAssets returns all assets found in world state
func (s *SmartContract) GetAllAssets(ctx contractapi.TransactionContextInterface) ([]*Asset, error) {
	// range query with empty string for startKey and endKey does an
	// open-ended query of all assets in the chaincode namespace.
	resultsIterator, err := ctx.GetStub().GetStateByRange("", "")
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	var assets []*Asset
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		var asset Asset
		err = json.Unmarshal(queryResponse.Value, &asset)
		if err != nil {
			// If unmarshalling to Asset fails, skip this entry rather than failing the whole query
			continue
		}
		// Only include entries that look like assets (they should have a non-empty ID)
		if asset.ID == "" {
			continue
		}
		assets = append(assets, &asset)
	}

	return assets, nil
}

// CreateAuction creates on auction on the public channel. The identity that
// submits the transacion becomes the seller of the auction
func (s *SmartContract) CreateAuction(ctx contractapi.TransactionContextInterface, auctionID string, itemSold string) error {

	log.Printf("CreateAuction called with auctionID: %s, itemSold: %s", auctionID, itemSold)

	// get ID of submitting client
	clientID, err := s.GetSubmittingClientIdentity(ctx)
	if err != nil {
		return fmt.Errorf("failed to get client identity %v", err)
	}

	// get org of submitting client
	clientOrgID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return fmt.Errorf("failed to get client identity %v", err)
	}

	// Create auction
	bidders := make(map[string]BidHash)
	revealedBids := make(map[string]FullBid)

	auction := Auction{
		Type:         "auction",
		ItemSold:     itemSold,
		Price:        0,
		Seller:       clientID,
		Orgs:         []string{clientOrgID},
		PrivateBids:  bidders,
		RevealedBids: revealedBids,
		Winner:       "",
		Status:       "open",
	}

	auctionJSON, err := json.Marshal(auction)
	if err != nil {
		return err
	}

	// put auction into state
	err = ctx.GetStub().PutState(auctionID, auctionJSON)
	if err != nil {
		return fmt.Errorf("failed to put auction in public data: %v", err)
	}

	return nil
}

// Bid is used to add a user's bid to the auction. The bid is stored in the private
// data collection on the peer of the bidder's organization. The function returns
// the transaction ID so that users can identify and query their bid
func (s *SmartContract) Bid(ctx contractapi.TransactionContextInterface, auctionID string) (string, error) {

	// get bid from transient map
	transientMap, err := ctx.GetStub().GetTransient()
	if err != nil {
		return "", fmt.Errorf("error getting transient: %v", err)
	}

	BidJSON, ok := transientMap["bid"]
	if !ok {
		return "", errors.New("bid key not found in the transient map")
	}

	// get the implicit collection name using the bidder's organization ID
	collection, err := getCollectionName(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get implicit collection name: %v", err)
	}

	// the bidder has to target their peer to store the bid
	err = verifyClientOrgMatchesPeerOrg(ctx)
	if err != nil {
		return "", fmt.Errorf("cannot store bid on this peer, not a member of this org: Error %v", err)
	}

	// the transaction ID is used as a unique index for the bid
	txID := ctx.GetStub().GetTxID()

	// create a composite key using the transaction ID
	bidKey, err := ctx.GetStub().CreateCompositeKey(bidKeyType, []string{auctionID, txID})
	if err != nil {
		return "", fmt.Errorf("failed to create composite key: %v", err)
	}

	// put the bid into the organization's implicit data collection
	err = ctx.GetStub().PutPrivateData(collection, bidKey, BidJSON)
	if err != nil {
		return "", fmt.Errorf("failed to input price into collection: %v", err)
	}

	// return the trannsaction ID so that the uset can identify their bid
	return txID, nil
}

// SubmitBid is used by the bidder to add the hash of that bid stored in private data to the
// auction. Note that this function alters the auction in private state, and needs
// to meet the auction endorsement policy. Transaction ID is used identify the bid
func (s *SmartContract) SubmitBid(ctx contractapi.TransactionContextInterface, auctionID string, txID string) error {

	// get the MSP ID of the bidder's org
	clientOrgID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return fmt.Errorf("failed to get client MSP ID: %v", err)
	}

	// get the auction from public state
	auction, err := s.QueryAuction(ctx, auctionID)
	if err != nil {
		return fmt.Errorf("failed to get auction from public state %v", err)
	}

	// the auction needs to be open for users to add their bid
	Status := auction.Status
	if Status != "open" {
		return errors.New("cannot join closed or ended auction")
	}

	// get the inplicit collection name of bidder's org
	collection, err := getCollectionName(ctx)
	if err != nil {
		return fmt.Errorf("failed to get implicit collection name: %v", err)
	}

	// use the transaction ID passed as a parameter to create composite bid key
	bidKey, err := ctx.GetStub().CreateCompositeKey(bidKeyType, []string{auctionID, txID})
	if err != nil {
		return fmt.Errorf("failed to create composite key: %v", err)
	}

	// get the hash of the bid stored in private data collection
	bidHash, err := ctx.GetStub().GetPrivateDataHash(collection, bidKey)
	if err != nil {
		return fmt.Errorf("failed to read bid bash from collection: %v", err)
	}
	if bidHash == nil {
		return fmt.Errorf("bid hash does not exist: %s", bidKey)
	}

	// store the hash along with the bidder's organization
	NewHash := BidHash{
		Org:  clientOrgID,
		Hash: fmt.Sprintf("%x", bidHash),
	}

	auction.PrivateBids[bidKey] = NewHash

	// Add the bidding organization to the list of participating organizations if it is not already
	Orgs := auction.Orgs
	if !(contains(Orgs, clientOrgID)) {
		newOrgs := append(Orgs, clientOrgID)
		auction.Orgs = newOrgs

		err = addAssetStateBasedEndorsement(ctx, auctionID, clientOrgID)
		if err != nil {
			return fmt.Errorf("failed setting state based endorsement for new organization: %v", err)
		}
	}

	newAuctionJSON, _ := json.Marshal(auction)

	err = ctx.GetStub().PutState(auctionID, newAuctionJSON)
	if err != nil {
		return fmt.Errorf("failed to update auction: %v", err)
	}

	return nil
}

// RevealBid is used by a bidder to reveal their bid after the auction is closed
func (s *SmartContract) RevealBid(ctx contractapi.TransactionContextInterface, auctionID string, txID string) error {

	// get bid from transient map
	transientMap, err := ctx.GetStub().GetTransient()
	if err != nil {
		return fmt.Errorf("error getting transient: %v", err)
	}

	transientBidJSON, ok := transientMap["bid"]
	if !ok {
		return errors.New("bid key not found in the transient map")
	}

	// get implicit collection name of organization ID
	collection, err := getCollectionName(ctx)
	if err != nil {
		return fmt.Errorf("failed to get implicit collection name: %v", err)
	}

	// use transaction ID to create composit bid key
	bidKey, err := ctx.GetStub().CreateCompositeKey(bidKeyType, []string{auctionID, txID})
	if err != nil {
		return fmt.Errorf("failed to create composite key: %v", err)
	}

	// get bid hash of bid if private bid on the public ledger
	bidHash, err := ctx.GetStub().GetPrivateDataHash(collection, bidKey)
	if err != nil {
		return fmt.Errorf("failed to read bid bash from collection: %v", err)
	}
	if bidHash == nil {
		return fmt.Errorf("bid hash does not exist: %s", bidKey)
	}

	// get auction from public state
	auction, err := s.QueryAuction(ctx, auctionID)
	if err != nil {
		return fmt.Errorf("failed to get auction from public state %v", err)
	}

	// Complete a series of three checks before we add the bid to the auction

	// check 1: check that the auction is closed. We cannot reveal a
	// bid to an open auction
	Status := auction.Status
	if Status != "closed" {
		return errors.New("cannot reveal bid for open or ended auction")
	}

	// check 2: check that hash of revealed bid matches hash of private bid
	// on the public ledger. This checks that the bidder is telling the truth
	// about the value of their bid

	hash := sha256.New()
	hash.Write(transientBidJSON)
	calculatedBidJSONHash := hash.Sum(nil)

	// verify that the hash of the passed immutable properties matches the on-chain hash
	if !bytes.Equal(calculatedBidJSONHash, bidHash) {
		return fmt.Errorf("hash %x for bid JSON %s does not match hash in auction: %x",
			calculatedBidJSONHash,
			transientBidJSON,
			bidHash,
		)
	}

	// check 3; check hash of relealed bid matches hash of private bid that was
	// added earlier. This ensures that the bid has not changed since it
	// was added to the auction

	privateBidHashString := auction.PrivateBids[bidKey].Hash

	onChainBidHashString := fmt.Sprintf("%x", bidHash)
	if privateBidHashString != onChainBidHashString {
		return fmt.Errorf("hash %s for bid JSON %s does not match hash in auction: %s, bidder must have changed bid",
			privateBidHashString,
			transientBidJSON,
			onChainBidHashString,
		)
	}

	// we can add the bid to the auction if all checks have passed
	type transientBidInput struct {
		Price  int    `json:"price"`
		Org    string `json:"org"`
		Bidder string `json:"bidder"`
	}

	// unmarshal bid input
	var bidInput transientBidInput
	err = json.Unmarshal(transientBidJSON, &bidInput)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	// Get ID of submitting client identity
	clientID, err := s.GetSubmittingClientIdentity(ctx)
	if err != nil {
		return fmt.Errorf("failed to get client identity %v", err)
	}

	// marshal transient parameters and ID and MSPID into bid object
	NewBid := FullBid{
		Type:   bidKeyType,
		Price:  bidInput.Price,
		Org:    bidInput.Org,
		Bidder: bidInput.Bidder,
	}

	// check 4: make sure that the transaction is being submitted is the bidder
	if bidInput.Bidder != clientID {
		return fmt.Errorf("permission denied, client id %v is not the owner of the bid", clientID)
	}

	auction.RevealedBids[bidKey] = NewBid

	newAuctionJSON, _ := json.Marshal(auction)

	// put auction with bid added back into state
	err = ctx.GetStub().PutState(auctionID, newAuctionJSON)
	if err != nil {
		return fmt.Errorf("failed to update auction: %v", err)
	}

	return nil
}

// CloseAuction can be used by the seller to close the auction. This prevents
// bids from being added to the auction, and allows users to reveal their bid
func (s *SmartContract) CloseAuction(ctx contractapi.TransactionContextInterface, auctionID string) error {

	// get auction from public state
	auction, err := s.QueryAuction(ctx, auctionID)
	if err != nil {
		return fmt.Errorf("failed to get auction from public state %v", err)
	}

	// the auction can only be closed by the seller

	// get ID of submitting client
	clientID, err := s.GetSubmittingClientIdentity(ctx)
	if err != nil {
		return fmt.Errorf("failed to get client identity %v", err)
	}

	Seller := auction.Seller
	if Seller != clientID {
		return fmt.Errorf("auction can only be closed by seller: %v", err)
	}

	Status := auction.Status
	if Status != "open" {
		return errors.New("cannot close auction that is not open")
	}

	auction.Status = string("closed")

	closedAuctionJSON, _ := json.Marshal(auction)

	err = ctx.GetStub().PutState(auctionID, closedAuctionJSON)
	if err != nil {
		return fmt.Errorf("failed to close auction: %v", err)
	}

	return nil
}

// EndAuction both changes the auction status to closed and calculates the winners
// of the auction
func (s *SmartContract) EndAuction(ctx contractapi.TransactionContextInterface, auctionID string) error {

	// get auction from public state
	auction, err := s.QueryAuction(ctx, auctionID)
	if err != nil {
		return fmt.Errorf("failed to get auction from public state %v", err)
	}

	// Check that the auction is being ended by the seller

	// get ID of submitting client
	clientID, err := s.GetSubmittingClientIdentity(ctx)
	if err != nil {
		return fmt.Errorf("failed to get client identity %v", err)
	}

	Seller := auction.Seller
	if Seller != clientID {
		return fmt.Errorf("auction can only be ended by seller: %v", err)
	}

	Status := auction.Status
	if Status != "closed" {
		return errors.New("can only end a closed auction")
	}

	// get the list of revealed bids
	revealedBidMap := auction.RevealedBids
	if len(auction.RevealedBids) == 0 {
		return fmt.Errorf("no bids have been revealed, cannot end auction: %v", err)
	}

	// determine the highest bid
	for _, bid := range revealedBidMap {
		if bid.Price > auction.Price {
			auction.Winner = bid.Bidder
			auction.Price = bid.Price
		}
	}

	// check if there is a winning bid that has yet to be revealed
	err = checkForHigherBid(ctx, auction.Price, auction.RevealedBids, auction.PrivateBids)
	if err != nil {
		return fmt.Errorf("cannot end auction: %v", err)
	}

	auction.Status = "ended"

	endedAuctionJSON, _ := json.Marshal(auction)

	err = ctx.GetStub().PutState(auctionID, endedAuctionJSON)
	if err != nil {
		return fmt.Errorf("failed to end auction: %v", err)
	}

	// Transfer asset to the winner
	if auction.Winner != "" {
		// Before transferring ownership, move the asset's AppraisedValue from buyer to seller
		asset, err := s.ReadAsset(ctx, auction.ItemSold)
		if err != nil {
			return fmt.Errorf("failed to read asset for payment: %v", err)
		}
		amount := asset.AppraisedValue

		// Debit buyer
		_, err = s.AdjustBalance(ctx, auction.Winner, -amount)
		if err != nil {
			return fmt.Errorf("failed to debit buyer: %v", err)
		}

		// Credit seller
		_, err = s.AdjustBalance(ctx, auction.Seller, amount)
		if err != nil {
			return fmt.Errorf("failed to credit seller: %v", err)
		}

		// Now transfer the asset
		_, err = s.TransferAsset(ctx, auction.ItemSold, auction.Winner)
		if err != nil {
			return fmt.Errorf("failed to transfer asset to winner: %v", err)
		}
	}

	return nil
}

// QueryAuction returns the auction stored in the world state with given id.
func (s *SmartContract) QueryAuction(ctx contractapi.TransactionContextInterface, auctionID string) (*Auction, error) {
	auctionJSON, err := ctx.GetStub().GetState(auctionID)
	if err != nil {
		return nil, fmt.Errorf("failed to read from world state: %v", err)
	}
	if auctionJSON == nil {
		return nil, fmt.Errorf("the auction %s does not exist", auctionID)
	}

	var auction Auction
	err = json.Unmarshal(auctionJSON, &auction)
	if err != nil {
		return nil, err
	}

	return &auction, nil
}

// QueryBid allows the submitter of the bid to read their bid from their org's private data
func (s *SmartContract) QueryBid(ctx contractapi.TransactionContextInterface, auctionID string, txID string) (*FullBid, error) {

	// Ensure the client is querying from a peer belonging to their org
	err := verifyClientOrgMatchesPeerOrg(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get implicit collection name: %v", err)
	}

	clientID, err := s.GetSubmittingClientIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get client identity %v", err)
	}

	collection, err := getCollectionName(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get implicit collection name: %v", err)
	}

	bidKey, err := ctx.GetStub().CreateCompositeKey(bidKeyType, []string{auctionID, txID})
	if err != nil {
		return nil, fmt.Errorf("failed to create composite key: %v", err)
	}

	bidJSON, err := ctx.GetStub().GetPrivateData(collection, bidKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get bid %v: %v", bidKey, err)
	}
	if bidJSON == nil {
		return nil, fmt.Errorf("bid %v does not exist", bidKey)
	}

	var bid FullBid
	err = json.Unmarshal(bidJSON, &bid)
	if err != nil {
		return nil, err
	}

	// check that the client querying the bid is the bid owner
	if bid.Bidder != clientID {
		return nil, fmt.Errorf("permission denied, client id %v is not the owner of the bid", clientID)
	}

	return &bid, nil
}

func (s *SmartContract) GetSubmittingClientIdentity(ctx contractapi.TransactionContextInterface) (string, error) {

	b64ID, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return "", fmt.Errorf("failed to read clientID: %v", err)
	}
	return b64ID, nil
}

func getCollectionName(ctx contractapi.TransactionContextInterface) (string, error) {

	// Get the MSP ID of submitting client identity
	clientMSPID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return "", fmt.Errorf("failed to get verified MSPID: %v", err)
	}

	// Create the collection name
	collection := "_implicit_org_" + clientMSPID

	return collection, nil
}

func verifyClientOrgMatchesPeerOrg(ctx contractapi.TransactionContextInterface) error {
	clientMSPID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return fmt.Errorf("failed getting the client's MSPID: %v", err)
	}
	peerCreator, err := ctx.GetStub().GetCreator()
	if err != nil {
		return fmt.Errorf("failed getting the peer's creator: %v", err)
	}

	// Extract MSPID from the creator's serialized identity
	signedData := &msp.SerializedIdentity{}
	err = proto.Unmarshal(peerCreator, signedData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal creator: %v", err)
	}
	peerMSPID := signedData.GetMspid()

	log.Printf("Client MSPID: %s, Peer MSPID: %s", clientMSPID, peerMSPID)

	if clientMSPID != peerMSPID {
		return fmt.Errorf("client from org %v is not authorized to read or write private data from an org %v peer", clientMSPID, peerMSPID)
	}

	return nil
}

func setAssetStateBasedEndorsement(ctx contractapi.TransactionContextInterface, assetID string, orgToEndorse string) error {

	endorsementPolicy, err := statebased.NewStateEP(nil)
	if err != nil {
		return err
	}
	err = endorsementPolicy.AddOrgs(statebased.RoleTypePeer, orgToEndorse)
	if err != nil {
		return fmt.Errorf("failed to add org to endorsement policy: %v", err)
	}
	policy, err := endorsementPolicy.Policy()
	if err != nil {
		return fmt.Errorf("failed to create endorsement policy bytes from org:. %v", err)
	}
	err = ctx.GetStub().SetStateValidationParameter(assetID, policy)
	if err != nil {
		return fmt.Errorf("failed to set validation parameter on auction: %v", err)
	}

	return nil
}

func addAssetStateBasedEndorsement(ctx contractapi.TransactionContextInterface, assetID string, orgToEndorse string) error {

	endorsementPolicy, err := ctx.GetStub().GetStateValidationParameter(assetID)
	if err != nil {
		return err
	}

	newEndorsementPolicy, err := statebased.NewStateEP(endorsementPolicy)
	if err != nil {
		return err
	}

	err = newEndorsementPolicy.AddOrgs(statebased.RoleTypePeer, orgToEndorse)
	if err != nil {
		return fmt.Errorf("failed to add org to endorsement policy: %v", err)
	}
	policy, err := newEndorsementPolicy.Policy()
	if err != nil {
		return fmt.Errorf("failed to create endorsement policy bytes from org: %v", err)
	}
	err = ctx.GetStub().SetStateValidationParameter(assetID, policy)
	if err != nil {
		return fmt.Errorf("failed to set validation parameter on auction: %v", err)
	}

	return nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// deriveOwnerLabel tries to transform an owner identifier (often a base64
// serialized identity) into a short human-friendly label such as
// "org1/seller" or "department11/seller". If no reasonable label can be
// extracted, it returns the decoded or original owner string.
func deriveOwnerLabel(owner string) string {
	// try base64 decode
	decoded := owner
	if b, err := base64.StdEncoding.DecodeString(owner); err == nil {
		decoded = string(b)
	}

	// Try to extract CN and OU from the decoded identity
	reCN := regexp.MustCompile(`CN=([^,\n]+)`)
	reOU := regexp.MustCompile(`OU=([^,\n]+)`)
	reO := regexp.MustCompile(`O=([^,\n]+)`)
	cn := ""
	ou := ""
	if m := reCN.FindStringSubmatch(decoded); len(m) > 1 {
		cn = m[1]
	}
	if m := reOU.FindStringSubmatch(decoded); len(m) > 1 {
		ou = m[1]
	}

	if cn != "" {
		// Prefer using organization (O) when available, falling back to OU
		if oMatch := reO.FindStringSubmatch(decoded); len(oMatch) > 1 {
			// map 'org1.example.com' -> 'org1'
			orgFull := oMatch[1]
			orgParts := strings.Split(orgFull, ".")
			org := strings.ToLower(orgParts[0])
			return fmt.Sprintf("%s/%s", org, cn)
		}
		// Normalize OU -> lowercase, remove spaces
		if ou != "" {
			normOU := strings.ToLower(strings.ReplaceAll(ou, " ", ""))
			return fmt.Sprintf("%s/%s", normOU, cn)
		}
		return cn
	}

	// No CN found; return decoded string as-is
	return decoded
}

const defaultBalance = 1000

// balance key helper
func balanceKey(ctx contractapi.TransactionContextInterface, owner string) (string, error) {
	return ctx.GetStub().CreateCompositeKey("balance", []string{owner})
}

// GetBalance returns the balance for owner, creating a default if missing.
func (s *SmartContract) GetBalance(ctx contractapi.TransactionContextInterface, owner string) (int, error) {
	key, err := balanceKey(ctx, owner)
	if err != nil {
		return 0, err
	}
	b, err := ctx.GetStub().GetState(key)
	if err != nil {
		return 0, err
	}
	if b == nil {
		// initialize default balance
		if err := ctx.GetStub().PutState(key, []byte(strconv.Itoa(defaultBalance))); err != nil {
			return 0, err
		}
		return defaultBalance, nil
	}
	bal, err := strconv.Atoi(string(b))
	if err != nil {
		return 0, err
	}
	return bal, nil
}

// GetMyBalance returns the balance for the submitting client identity.
func (s *SmartContract) GetMyBalance(ctx contractapi.TransactionContextInterface) (int, error) {
	clientID, err := s.GetSubmittingClientIdentity(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get client identity: %v", err)
	}
	return s.GetBalance(ctx, clientID)
}

// AdjustBalance adjusts the owner's balance by delta (can be negative). Returns new balance.
func (s *SmartContract) AdjustBalance(ctx contractapi.TransactionContextInterface, owner string, delta int) (int, error) {
	key, err := balanceKey(ctx, owner)
	if err != nil {
		return 0, err
	}
	bal, err := s.GetBalance(ctx, owner)
	if err != nil {
		return 0, err
	}
	newBal := bal + delta
	if newBal < 0 {
		return 0, fmt.Errorf("insufficient funds: have %d, need %d", bal, -delta)
	}
	if err := ctx.GetStub().PutState(key, []byte(strconv.Itoa(newBal))); err != nil {
		return 0, err
	}
	return newBal, nil
}

func checkForHigherBid(ctx contractapi.TransactionContextInterface, highestPrice int, revealedBids map[string]FullBid, privateBids map[string]BidHash) error {
	// check if there is a winning bid that has yet to be revealed
	for id, hash := range privateBids {
		// check if the bid has been revealed
		if _, ok := revealedBids[id]; !ok {
			// get the hash of the bid from the private data collection
			collection, err := getCollectionName(ctx)
			if err != nil {
				return fmt.Errorf("failed to get implicit collection name: %v", err)
			}
			bidHash, err := ctx.GetStub().GetPrivateDataHash(collection, id)
			if err != nil {
				return fmt.Errorf("failed to read bid bash from collection: %v", err)
			}
			if bidHash == nil {
				return fmt.Errorf("bid hash does not exist: %s", id)
			}
			// compare the hash of the bid with the hash on the public ledger
			if hash.Hash != fmt.Sprintf("%x", bidHash) {
				return fmt.Errorf("hash of bid %s does not match hash on public ledger", id)
			}
		}
	}
	return nil
}