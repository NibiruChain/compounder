package compounder

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nibiruchain/compounder/config"
	"github.com/nibiruchain/compounder/utils"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (compounder *Compounder) LogError(msg string, args ...any) {
	args = append(args, "compounder")
	slog.Error(msg, args...)
}

func (compounder *Compounder) LogInfo(msg string, args ...any) {
	args = append(args, "compounder")
	slog.Info(msg, args...)
}

type Compounder struct {
	CompounderInterval time.Duration
	ChainClient        utils.ChainClient
	Account            utils.KeyringRecord
}

type StakeMsg struct {
	Share     uint64 `json:"share"`
	Validator string `json:"validator"`
}

func NewCompounder() Compounder {
	chainClient := utils.NewChainClient(config.ChainId, utils.GetGRPCConnection())

	compounder := Compounder{
		ChainClient: chainClient,
		Account:     chainClient.GetOrAddAccount("compounder", config.CompounderMnemonic),
	}

	compounder.LogInfo("Compounder initialized with address ", compounder.Account.GetAddressStr())

	return compounder
}

func (compounder *Compounder) ClaimRewards() {
	msgData := map[string]interface{}{
		"claim_rewards": map[string]interface{}{},
	}

	msgBytes, err := json.Marshal(msgData)
	if err != nil {
		compounder.LogError("Error marshaling msg data", "err", err)
		panic(err)
	}

	msg := wasmtypes.MsgExecuteContract{
		Sender:   compounder.Account.GetAddressStr(),
		Contract: config.CompounderContractAddress,
		Msg:      msgBytes,
		Funds:    sdk.NewCoins(),
	}

	options := utils.SendMsgOptions{
		Messages:     []sdk.Msg{&msg},
		SignerRecord: compounder.Account,
		GasLimit:     config.CompounderGasLimit,
		Fee:          config.CompounderFeeInitial,
	}

	_, txHash, err := compounder.executeMessage(options, 1)
	if err != nil {
		compounder.LogError("Error executing compound message", "err", err)
	}
	compounder.LogInfo("Redeem successful - txHash ", txHash)
}

func (compounder *Compounder) Compound() {
	// read current nibi balance
	balance := compounder.ChainClient.QueryAccountBalance(compounder.Account.GetAddressStr())

	// we will stake the whole balance minus 1 nibi for fees
	stakeAmount := balance.AmountOf("unibi").Sub(sdk.NewInt(1))
	if stakeAmount.IsNegative() {
		compounder.LogError("Insufficient balance to stake", "balance", balance.AmountOf("unibi"))
		return
	}

	file, err := os.Open(config.CsvPath)
	if err != nil {
		compounder.LogError("Error opening CSV file", "err", err)
		return
	}
	defer file.Close()

	// Create a new reader.
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		compounder.LogError("Error reading CSV file", "err", err)
		return
	}

	// Parse records and populate msgData.
	var stakeMsgs []StakeMsg
	for _, record := range records[1:] { // Skipping header row.
		validatorAddress := record[0]
		share, err := strconv.ParseUint(record[1], 10, 64)
		if err != nil {
			compounder.LogError("Error parsing share from CSV", "err", err)
			continue
		}

		stakeMsg := StakeMsg{
			Share:     share,
			Validator: validatorAddress,
		}
		stakeMsgs = append(stakeMsgs, stakeMsg)
	}

	msgData := map[string]interface{}{
		"stake": map[string]interface{}{
			"stake_msgs": stakeMsgs,
		},
	}

	msgBytes, err := json.Marshal(msgData)
	if err != nil {
		compounder.LogError("Error marshaling msg data", "err", err)
		panic(err)
	}

	msg := wasmtypes.MsgExecuteContract{
		Sender:   compounder.Account.GetAddressStr(),
		Contract: config.CompounderContractAddress,
		Msg:      msgBytes,
		Funds:    sdk.NewCoins(),
	}

	// we send one message per pair since we don't want a single failure
	// to prevent the other pairs from being repegged.
	options := utils.SendMsgOptions{
		Messages:     []sdk.Msg{&msg},
		SignerRecord: compounder.Account,
		GasLimit:     config.CompounderGasLimit,
		Fee:          config.CompounderFeeInitial,
	}

	_, txHash, err := compounder.executeMessage(options, 1)
	if err != nil {
		compounder.LogError("Error executing compound message", "err", err)
	}
	compounder.LogInfo("Compound successful - txHash ", txHash)
}

// executeMessage sends liquidation message to a chain, retries if out of gas
func (compounder *Compounder) executeMessage(
	options utils.SendMsgOptions, attempt int,
) (int64, string, error) {
	txResp, err := compounder.ChainClient.SendMsg(options)

	if err != nil {
		text := err.Error()
		if strings.Contains(text, "insufficient fee") {
			if attempt >= config.CompounderGasMaxAttempts {
				slog.Error(
					"Compounder failed",
					"fee", options.Fee,
					"gas_limit", options.GasLimit,
					"attempt", attempt,
					"reason", "insufficient fee",
				)
				return 0, "", errors.New(text)
			}
			prevFee := options.Fee
			options.Fee = int64(math.Ceil(float64(options.Fee) * config.CompounderGasMultiplier))
			newAttempt := attempt + 1
			slog.Warn(
				"Retrying liquidation due to insufficient fee",
				"fee", prevFee,
				"new_fee", options.Fee,
				"gas_limit", options.GasLimit,
				"attempt", newAttempt,
			)
			return compounder.executeMessage(options, newAttempt)
		} else {
			return 0, "", err
		}
	} else if txResp.Code != 0 {
		text := txResp.RawLog
		if strings.Contains(text, "out of gas") {
			if attempt >= config.CompounderGasMaxAttempts {
				slog.Error(
					"Compounder failed",
					"fee", options.Fee,
					"gas_limit", options.GasLimit,
					"attempt", attempt,
					"reason", "out of gas",
				)
				return txResp.Height, txResp.TxHash, errors.New(text)
			}
			prevGasLimit := options.GasLimit
			options.GasLimit = uint64(math.Ceil(float64(options.GasLimit) * config.CompounderGasMultiplier))
			newAttempt := attempt + 1
			slog.Warn(
				"Retrying liquidation due to out of gas",
				"fee", options.Fee,
				"gas_limit", prevGasLimit,
				"new_gas_limit", options.GasLimit,
				"attempt", newAttempt,
			)
			return compounder.executeMessage(options, newAttempt)
		} else {
			slog.Error(
				"Compounder failed",
				"raw_log", text,
				"reason", "unknown",
			)
			return 0, "", errors.New(text)
		}
	} else {
		return txResp.Height, txResp.TxHash, nil
	}
}
