# Compounder Keeper Bot

The Compounder Keeper Bot automates the management of rewards, specifically tailored for use with the [Broker Staking Contract](https://github.com/NibiruChain/cw-nibiru/tree/main/contracts/broker-staking). This bot facilitates seamless reward compounding for token stakers by executing routine operations autonomously.

## Key Features

- **Automated Reward Claims**: Periodically claims all staking rewards using the `claim_rewards` function of the broker staking contract.
- **Reinvestment of Rewards**: After that, stakes the claimed rewards back into the contract. The staking proportions are determined by a predefined distribution within a CSV dataset, utilizing the `stake` message of the contract.

## Security and Permissions

Designed with security in mind, the bot interacts with the broker type contract which allows third-party operators to manage staking and rewards on behalf of a wallet. While it can claim and stake rewards, it does not have permissions to withdraw funds.

## Use Case

This tool is ideal for organizations such as foundations that aim to maximize their staking potential without the need for continual manual intervention. It simplifies the process of claiming and reinvesting rewards, thereby facilitating efficient and automatic compounding.

## Configuration

A `.env` file is defining the configuration of the chain and the operator with these parameters:

```.env
GRPC_ENDPOINT="localhost:9090"
GRPC_INSECURE="true"
CHAIN_ID="nibiru-localnet-0"
COMPOUNDER_MNEMONIC="manual crew resist worry wing beach situate space express auto sight virus census ability stable opinion six draw alpha total joke assume wisdom hedgehog"
COMPOUNDER_CONTRACT_ADDRESS=nibi14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9ssa9gcs
CSV_PATH="data/validator_redelegation_ratio.csv"
COMPOUNDER_FEE_INITIAL=100000
COMPOUNDER_GAS_LIMIT=153426400
```
