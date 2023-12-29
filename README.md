   ![Banner](.github/bfic.png)

# BFIC Blockchain

A cross-chain protocol to bridge Ethereum compatible Blockchains with other Blockchain networks. BFIC is a developer-friendly protocol that combines the best features of layer 1 and sovereign blockchains.

#### We focus on:

- ###### Ethereum Protocol Compatibility

  Perks of pre-established tools, standards, tech stacks and global acceptance.

- ###### Scalability

  Scalable consensus algorithms, dedication blockchains & custom execution environments.

- ###### Security

  Bleeding-edge security and constant upgrades to the protocol.

- ###### Interoperability

  Arbitrary communication support for contract calls, tokens etc.) & bridging to and from external systems.

- ###### Development Friendly (Solidity smart-contracts)
  Ethereum equivalent with no requirement of permissions, fees or token deposits. Bundled with Web3 support.

## BFIC Blockchain Node Deployment

Refer to the process below for deploying a production grade BFIC Blockchain node and syncing it with the BFIC network. 
- Clone code from the repository & build binary
- Create a secret configuration, utilizing any secret manager (AWS, Hashicorp etc.)
- Generate secrets
- Start the node

### Clone repo & build binary

Clone the repository on your machine and build.

```bash
git clone https://github.com/innovationfactorydev/bfic.git
git checkout origin/v1.3.1
make build
mv ./bfic /usr/local/bin/
```

### Generate secrets

#### Using a secret manager (Hashicorp or AWS etc.)
To create a secure secrets configuration, we'll supply the url of the key vault & token to the ```generate``` command and then pass the resulting config file to the ```init``` to store the secrets in it.

```bash
bfic secrets generate --name secretManager --token my-secure-token --server-url https://SECRETS_MANAGER_URL
```

```bash
bfic secrets init --config ./secretsManagerConfig.json
```
##### The above commands will store the secrets in the specified key vault and print the following (example).

```bash
[SECRETS INIT]
Public key (address) = 0xf0b581F4256B8801D8e397a00248833eBdEe2a38
BLS Public key       = 0x87b756961fa6304becf5156177a782e22f0b077ad2bef02f0b175a76ca4928fd0637704fe724073cd64dbd2c919d0ba8
Node ID              = 16Uiu2HAm6CVzf6VfHqR5WnFwZCdBreiGaZsqU2McXBVTjqfzUTe7
```

#### Using the filesystem
There's also another method to store the secrets on the filesystem if a secret manager is not available, although its not recommended for production.

```bash
bfic secrets init --insecure --data-dir ./chain-data
```
This will create and store the node credentials in the specified directory.
### Start the node
Omit the ```--secrets-config ./secretsManagerConfig.json``` in case of filesystem credentials.
```bash
bfic server --data-dir ./chain-data --secrets-config ./secretsManagerConfig.json --chain ./genesis.json --grpc-address :10000 --libp2p :30301 --jsonrpc :8545 --seal
```
