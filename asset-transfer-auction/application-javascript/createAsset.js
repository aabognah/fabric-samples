/*
 * SPDX-License-Identifier: Apache-2.0
 */

'use strict';

const { Gateway, Wallets } = require('fabric-network');
const path = require('path');
const { buildCCPOrg1, buildCCPOrg2, buildWallet, prettyJSONString } = require('../../test-application/javascript/AppUtil.js');

const myChannel = 'mychannel';
const myChaincodeName = 'auction';

async function createAsset(ccp,wallet,user,assetID,color,size,owner,appraisedValue) {
	try {
		const gateway = new Gateway();

		await gateway.connect(ccp,
			{ wallet: wallet, identity: user, discovery: { enabled: true, asLocalhost: true } });

		const network = await gateway.getNetwork(myChannel);
		const contract = network.getContract(myChaincodeName);

		console.log('\n--> Submit Transaction: CreateAsset');
		// Use submitTransaction for fabric-network Contract API
		await contract.submitTransaction('CreateAsset', assetID, color, size.toString(), owner, appraisedValue.toString());
		console.log('*** Result: committed');

		let result = await contract.evaluateTransaction('ReadAsset', assetID);
		console.log('*** Result: Asset: ' + prettyJSONString(result.toString()));

		gateway.disconnect();
	} catch (error) {
		console.error(`******** FAILED to submit CreateAsset: ${error}`);
		process.exit(1);
	}
}

async function main() {
	try {
		if (process.argv.length < 8) {
			console.log('Usage: node createAsset.js org userID assetID color size owner appraisedValue');
			process.exit(1);
		}

		const org = process.argv[2];
		const user = process.argv[3];
		const assetID = process.argv[4];
		const color = process.argv[5];
		const size = process.argv[6];
		const owner = process.argv[7];
		const appraisedValue = process.argv[8];

		if (org === 'Org1' || org === 'org1') {
			const ccp = buildCCPOrg1();
			const walletPath = path.join(__dirname, 'wallet/org1');
			const wallet = await buildWallet(Wallets, walletPath);
			await createAsset(ccp,wallet,user,assetID,color,size,owner,appraisedValue);
		} else if (org === 'Org2' || org === 'org2') {
			const ccp = buildCCPOrg2();
			const walletPath = path.join(__dirname, 'wallet/org2');
			const wallet = await buildWallet(Wallets, walletPath);
			await createAsset(ccp,wallet,user,assetID,color,size,owner,appraisedValue);
		} else {
			console.log('Org must be Org1 or Org2');
		}
	} catch (error) {
		console.error(`******** FAILED to run the createAsset app: ${error}`);
		process.exit(1);
	}
}

main();
