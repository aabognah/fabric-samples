/*
 * QueryAllAssets.js
 * Usage: node queryAllAssets.js org userID
 *
 * This script queries all assets from the ledger (GetAllAssets)
 * and attempts to translate the Owner field (which is stored as a
 * serialized identity string) into a human-friendly label such as
 * "org1/seller" by scanning the local wallet for known identities.
 */

'use strict';

const { Gateway, Wallets } = require('fabric-network');
const path = require('path');
const fs = require('fs');
const { buildCCPOrg1, buildCCPOrg2, buildWallet, prettyJSONString } = require('../../test-application/javascript/AppUtil.js');

const myChannel = 'mychannel';
const myChaincodeName = 'auction';

async function mapOwnerToLabel(ownerEncoded, wallets) {
  // Try to decode the owner string (it is a base64-encoded serialized identity)
  let decoded = null;
  try {
    decoded = Buffer.from(ownerEncoded, 'base64').toString('utf8');
  } catch (err) {
    decoded = ownerEncoded;
  }

  // Try to find a matching identity label in the known wallets
  for (const [orgLabel, walletPath] of Object.entries(wallets)) {
    try {
      if (!fs.existsSync(walletPath)) continue;
      const files = fs.readdirSync(walletPath);
      for (const f of files) {
        // wallet stores identities as files like 'seller.id'
        const label = f.replace(/\.id$/,'');
        if (!label) continue;
        // If the decoded serialized id contains the label, assume it belongs to this label
        if (decoded.includes(label)) {
          return `${orgLabel}/${label}`;
        }
      }
    } catch (err) {
      // ignore and continue
    }
  }

  // Fallbacks: try to extract CN and OU from decoded string
  let cnMatch = decoded && decoded.match(/CN=([^,\n]+)/);
  let ouMatch = decoded && decoded.match(/OU=([^,\n]+)/);
  if (cnMatch) {
    const cn = cnMatch[1];
    const org = ouMatch ? ouMatch[1] : 'unknown-org';
    return `${org}/${cn}`;
  }

  // Return decoded string if nothing matched
  return decoded || ownerEncoded;
}

async function queryAllAssets(ccp, wallet, user, walletDirs) {
  const gateway = new Gateway();
  try {
    await gateway.connect(ccp, { wallet: wallet, identity: user, discovery: { enabled: true, asLocalhost: true } });
    const network = await gateway.getNetwork(myChannel);
    const contract = network.getContract(myChaincodeName);

    console.log('\n--> Evaluate Transaction: GetAllAssets');
    const resultBytes = await contract.evaluateTransaction('GetAllAssets');
    const resultJson = resultBytes.toString();
    let assets = [];
    try {
      assets = JSON.parse(resultJson);
    } catch (err) {
      console.error('Failed to parse GetAllAssets result:', err);
      console.log('Raw result:', resultJson);
      return;
    }

    // Map owners to friendly labels when possible
    const mapped = assets.map(a => {
      const owner = a.Owner || '';
      const ownerLabel = owner ? mapOwnerToLabel(owner, walletDirs) : '';
      return Object.assign({}, a, { OwnerDecoded: ownerLabel });
    });

    console.log('*** Result: All Assets: ' + prettyJSONString(JSON.stringify(mapped)));

  } catch (err) {
    console.error(`******** FAILED to query all assets: ${err}`);
  } finally {
    try { gateway.disconnect(); } catch (e) {}
  }
}

async function main() {
  if (process.argv.length < 4) {
    console.log('Usage: node queryAllAssets.js org userID');
    process.exit(1);
  }

  const org = process.argv[2];
  const user = process.argv[3];

  let ccp, walletPath;
  if (org === 'Org1' || org === 'org1') {
    ccp = buildCCPOrg1();
    walletPath = path.join(__dirname, 'wallet/org1');
  } else if (org === 'Org2' || org === 'org2') {
    ccp = buildCCPOrg2();
    walletPath = path.join(__dirname, 'wallet/org2');
  } else {
    console.log('Org must be Org1 or Org2');
    process.exit(1);
  }

  const wallet = await buildWallet(Wallets, walletPath);

  // Provide wallet directories for both orgs so we can map labels
  const walletDirs = {
    'org1': path.join(__dirname, 'wallet/org1'),
    'org2': path.join(__dirname, 'wallet/org2')
  };

  await queryAllAssets(ccp, wallet, user, walletDirs);
}

main();
