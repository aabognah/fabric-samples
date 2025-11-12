/*
 * SPDX-License-Identifier: Apache-2.0
 */

'use strict';

const { Gateway, Wallets } = require('fabric-network');
const path = require('path');
const { buildCCPOrg1, buildCCPOrg2, buildWallet, prettyJSONString } = require('../../test-application/javascript/AppUtil.js');

const myChannel = 'mychannel';
const myChaincodeName = 'auction';

async function createAsset(ccp,wallet,user,org,assetID,color,size,owner,appraisedValue) {
	try {
		const gateway = new Gateway();

		await gateway.connect(ccp,
			{ wallet: wallet, identity: user, discovery: { enabled: true, asLocalhost: true } });

		const network = await gateway.getNetwork(myChannel);
		const contract = network.getContract(myChaincodeName);

		console.log('\n--> Submit Transaction: CreateAsset (with private asset_properties transient)');
		// Build transient data containing confidential asset properties
		const transient = {
			asset_properties: Buffer.from(JSON.stringify({ AppraisedValue: parseInt(appraisedValue), ReservePrice: parseInt(appraisedValue) }))
		};
		const tx = contract.createTransaction('CreateAsset');
		// Limit endorsing organizations to the submitting org when writing private data
		// to that org's implicit collection. This avoids endorsement mismatches where
		// peers from other orgs simulate private-data writes they cannot access.
		const orgMSP = (org.toLowerCase().startsWith('org1')) ? 'Org1MSP' : 'Org2MSP';
		tx.setEndorsingOrganizations(orgMSP);
		tx.setTransient(transient);
		try {
			await tx.submit(assetID, color, size.toString(), owner, appraisedValue.toString());
			console.log('*** Result: committed (with transient)');
		} catch (err) {
			console.error('CreateAsset with transient failed:', err.message ? err.message : err);
			// If the failure looks like an endorsement / policy failure, retry without transient
			if (err.message && (err.message.includes('ENDORSEMENT_POLICY_FAILURE') || err.message.includes('ENDORSEMENT') || err.message.includes('Peer endorsements do not match'))) {
				console.log('Retrying CreateAsset without transient data (falling back to public appraisedValue)');
				// fallback: do a plain submit without transient private data, still target the same org
				const tx2 = contract.createTransaction('CreateAsset');
				tx2.setEndorsingOrganizations(orgMSP);
				try {
					await tx2.submit(assetID, color, size.toString(), owner, appraisedValue.toString());
					console.log('*** Result: committed (without transient)');
				} catch (err2) {
					console.error('Retry without transient also failed:', err2.message ? err2.message : err2);
					throw err2;
				}
			} else {
				throw err;
			}
		}

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
			await createAsset(ccp,wallet,user,org,assetID,color,size,owner,appraisedValue);
		} else if (org === 'Org2' || org === 'org2') {
			const ccp = buildCCPOrg2();
			const walletPath = path.join(__dirname, 'wallet/org2');
			const wallet = await buildWallet(Wallets, walletPath);
			await createAsset(ccp,wallet,user,org,assetID,color,size,owner,appraisedValue);
		} else {
			console.log('Org must be Org1 or Org2');
		}
	} catch (error) {
		console.error(`******** FAILED to run the createAsset app: ${error}`);
		process.exit(1);
	}
}

main();
