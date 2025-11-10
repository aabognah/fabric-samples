/*
 * getMyBalance.js
 * Usage: node getMyBalance.js org userID
 * Calls chaincode transaction GetMyBalance which returns the balance for the
 * submitting client identity (no owner parameter required).
 */

'use strict';

const { Gateway, Wallets } = require('fabric-network');
const path = require('path');
const { buildCCPOrg1, buildCCPOrg2, buildWallet } = require('../../test-application/javascript/AppUtil.js');

const myChannel = 'mychannel';
const myChaincodeName = 'auction';

async function getMyBalance(ccp, wallet, user) {
  const gateway = new Gateway();
  try {
    await gateway.connect(ccp, { wallet: wallet, identity: user, discovery: { enabled: true, asLocalhost: true } });
    const network = await gateway.getNetwork(myChannel);
    const contract = network.getContract(myChaincodeName);

    console.log('\n--> Evaluate Transaction: GetMyBalance');
    const result = await contract.evaluateTransaction('GetMyBalance');
    console.log('*** Result: My balance: ' + result.toString());
  } catch (err) {
    console.error(`******** FAILED to get my balance: ${err}`);
  } finally {
    try { gateway.disconnect(); } catch (e) {}
  }
}

async function main() {
  if (process.argv.length < 4) {
    console.log('Usage: node getMyBalance.js org userID');
    process.exit(1);
  }

  const org = process.argv[2];
  const user = process.argv[3];

  if (org === 'Org1' || org === 'org1') {
    const ccp = buildCCPOrg1();
    const walletPath = path.join(__dirname, 'wallet/org1');
    const wallet = await buildWallet(Wallets, walletPath);
    await getMyBalance(ccp, wallet, user);
  } else if (org === 'Org2' || org === 'org2') {
    const ccp = buildCCPOrg2();
    const walletPath = path.join(__dirname, 'wallet/org2');
    const wallet = await buildWallet(Wallets, walletPath);
    await getMyBalance(ccp, wallet, user);
  } else {
    console.log('Org must be Org1 or Org2');
  }
}

main();
