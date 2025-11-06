/*
 * QueryAsset.js
 * Usage: node queryAsset.js org userID assetID
 */

'use strict';

const { Gateway, Wallets } = require('fabric-network');
const path = require('path');
const { buildCCPOrg1, buildCCPOrg2, buildWallet, prettyJSONString } = require('../../test-application/javascript/AppUtil.js');

const myChannel = 'mychannel';
const myChaincodeName = 'auction';

async function queryAsset(ccp, wallet, user, assetID) {
  const gateway = new Gateway();
  try {
    await gateway.connect(ccp, { wallet: wallet, identity: user, discovery: { enabled: true, asLocalhost: true } });
    const network = await gateway.getNetwork(myChannel);
    const contract = network.getContract(myChaincodeName);

    console.log(`\n--> Evaluate Transaction: ReadAsset ${assetID}`);
    const result = await contract.evaluateTransaction('ReadAsset', assetID);
    console.log('*** Result: Asset: ' + prettyJSONString(result.toString()));
  } finally {
    gateway.disconnect();
  }
}

async function main() {
  if (process.argv.length < 5) {
    console.log('Usage: node queryAsset.js org userID assetID');
    process.exit(1);
  }
  const org = process.argv[2];
  const user = process.argv[3];
  const assetID = process.argv[4];

  if (org === 'Org1' || org === 'org1') {
    const ccp = buildCCPOrg1();
    const walletPath = path.join(__dirname, 'wallet/org1');
    const wallet = await buildWallet(Wallets, walletPath);
    await queryAsset(ccp, wallet, user, assetID);
  } else if (org === 'Org2' || org === 'org2') {
    const ccp = buildCCPOrg2();
    const walletPath = path.join(__dirname, 'wallet/org2');
    const wallet = await buildWallet(Wallets, walletPath);
    await queryAsset(ccp, wallet, user, assetID);
  } else {
    console.log('Org must be Org1 or Org2');
  }
}

main();