package utils

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"

	"log/slog"
	"time"

	"github.com/NibiruChain/nibiru/app"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	txTypes "github.com/cosmos/cosmos-sdk/types/tx"
	authTypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/nibiruchain/tooling/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type ChainClient struct {
	chainId  string
	keyring  keyring.Keyring
	encCfg   app.EncodingConfig
	grpcConn *grpc.ClientConn
	txClient txTypes.ServiceClient
}

type SendMsgOptions struct {
	Messages     []sdk.Msg
	SignerRecord KeyringRecord
	GasLimit     uint64
	Fee          int64
}

type KeyringRecord struct {
	*keyring.Record
}

func NewChainClient(chainId string, conn *grpc.ClientConn) ChainClient {
	SetChainPrefixes()

	encCfg := app.MakeEncodingConfig()
	kr := keyring.NewInMemory(encCfg.Marshaler)

	client := ChainClient{
		chainId:  chainId,
		keyring:  kr,
		encCfg:   encCfg,
		grpcConn: conn,
		txClient: txTypes.NewServiceClient(conn),
	}

	return client
}

func SetChainPrefixes() {
	conf := sdk.GetConfig()
	if conf.GetBech32AccountAddrPrefix() != "nibi" {
		app.SetPrefixes(app.AccountAddressPrefix)
	}
}

func (client *ChainClient) GetOrAddAccount(uid string, mnemonic_opt ...string) KeyringRecord {
	accInfo, err := client.keyring.Key(uid)
	if err == nil {
		return KeyringRecord{Record: accInfo}
	}
	if !sdkerrors.ErrKeyNotFound.Is(err) {
		panic(err)
	}

	if len(mnemonic_opt) > 0 {
		accInfo, err := client.keyring.NewAccount(
			uid,
			mnemonic_opt[0],
			"",
			sdk.FullFundraiserPath,
			hd.Secp256k1,
		)
		if err != nil {
			panic(err)
		}
		return KeyringRecord{Record: accInfo}
	} else {
		accInfo, _, err := client.keyring.NewMnemonic(
			/* uid */ uid,
			/* language */ keyring.English,
			/* hdPath */ sdk.FullFundraiserPath,
			/* big39Passphrase */ "",
			/* algo */ hd.Secp256k1,
		)
		if err != nil {
			panic(err)
		}
		return KeyringRecord{Record: accInfo}
	}
}

func (chainClient *ChainClient) SendMsg(options SendMsgOptions) (*sdk.TxResponse, error) {
	txBuilder := chainClient.encCfg.TxConfig.NewTxBuilder()
	err := txBuilder.SetMsgs(options.Messages...)
	if err != nil {
		return nil, err
	}
	txBuilder.SetGasLimit(options.GasLimit)
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("unibi", options.Fee)))
	txBuilder.SetFeePayer(options.SignerRecord.MustGetAddress())

	// Sign transaction
	signerAddress, err := options.SignerRecord.GetAddress()
	if err != nil {
		return nil, err
	}
	accNumber := chainClient.GetAccountNumbers(signerAddress.String())

	txFactory := tx.Factory{}.
		WithChainID(chainClient.chainId).
		WithAccountNumber(accNumber.Number).
		WithSequence(accNumber.Sequence).
		WithKeybase(chainClient.keyring).
		WithTxConfig(chainClient.encCfg.TxConfig)

	err = tx.Sign(txFactory, options.SignerRecord.Name, txBuilder, true)
	if err != nil {
		return nil, err
	}

	// Generated Protobuf-encoded bytes.
	txBytes, err := chainClient.encCfg.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, err
	}

	// Broadcast message
	ctx := context.Background()
	grpcRes, err := chainClient.txClient.BroadcastTx(
		ctx,
		&txTypes.BroadcastTxRequest{
			Mode:    txTypes.BroadcastMode_BROADCAST_MODE_SYNC,
			TxBytes: txBytes,
		},
	)
	if err != nil {
		return nil, err
	}
	if grpcRes.TxResponse.Code != 0 {
		return nil, errors.New(grpcRes.TxResponse.RawLog)
	}

	// Wait while transaction is committed
	txHash := grpcRes.TxResponse.TxHash
	slog.Info("Transaction sent. Waiting for response", "hash", txHash)

	timeout := time.NewTimer(60 * time.Second)
	tick := time.NewTicker(1000 * time.Millisecond)

	for {
		select {
		case <-tick.C:
			resp, _ := chainClient.txClient.GetTx(ctx, &txTypes.GetTxRequest{Hash: txHash})
			if resp != nil && resp.TxResponse != nil {
				timeout.Stop()
				tick.Stop()
				return resp.TxResponse, nil
			}
		case <-timeout.C:
			timeout.Stop()
			tick.Stop()
			return nil, errors.New("create transaction timeout error")
		}
	}
}

func (chainClient *ChainClient) SendMsgWithCheck(options SendMsgOptions) (*sdk.TxResponse, error) {
	resp, err := chainClient.SendMsg(options)

	if err == nil && resp.Code != 0 {
		strErr := fmt.Sprintf("transaction error. code: %v, log: %s ", resp.Code, resp.RawLog)
		return nil, errors.New(strErr)
	}

	return resp, err
}

type AccountNumbers struct {
	Number   uint64
	Sequence uint64
}

func (chainClient *ChainClient) GetAccountNumbers(address string) AccountNumbers {
	queryClient := authTypes.NewQueryClient(chainClient.grpcConn)
	resp, err := queryClient.Account(context.Background(), &authTypes.QueryAccountRequest{
		Address: address,
	})
	if err != nil {
		slog.Error("Error getting account", "err", err)
		panic(err)
	}
	// register auth interface

	var acc authTypes.AccountI
	err = chainClient.encCfg.InterfaceRegistry.UnpackAny(resp.Account, &acc)
	if err != nil {
		slog.Error("Error unpacking account numbers", "err", err)
		panic(err)
	}

	return AccountNumbers{
		Number:   acc.GetAccountNumber(),
		Sequence: acc.GetSequence(),
	}
}

func (chainClient *ChainClient) QueryAccountBalance(address string) sdk.Coins {
	queryClient := authTypes.NewQueryClient(chainClient.grpcConn)
	resp, err := queryClient.Bank(context.Background(), &banktypes.QueryBalanceRequest{
		Address: address,
	})
	if err != nil {
		slog.Error("Error getting account", "err", err)
		panic(err)
	}
	// register auth interface

	var acc authTypes.AccountI
	err = chainClient.encCfg.InterfaceRegistry.UnpackAny(resp.Account, &acc)
	if err != nil {
		slog.Error("Error unpacking account balance", "err", err)
		panic(err)
	}

	return acc.GetCoins()
}

func (record *KeyringRecord) GetAddressStr() string {
	address := record.MustGetAddress()
	return address.String()
}

func (record *KeyringRecord) MustGetAddress() sdk.AccAddress {
	address, err := record.GetAddress()
	if err != nil {
		panic(err)
	}

	return address
}

func ExtractMsg(msg sdk.Msg, err error) sdk.Msg {
	if err != nil {
		slog.Error(
			"Error extracting message",
			"err", err,
			"msg", msg.String(),
		)
		panic(err)
	}
	return msg
}

func GetGRPCConnection() *grpc.ClientConn {
	var creds credentials.TransportCredentials
	if config.GrpcInsecure {
		creds = insecure.NewCredentials()
	} else {
		creds = credentials.NewTLS(&tls.Config{})
	}
	options := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTransportCredentials(creds),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	conn, err := grpc.DialContext(ctx, config.GrpcUrl, options...)
	if err != nil {
		slog.Error("Cannot connect to gRPC endpoint", "url", config.GrpcUrl)
		panic(err)
	}
	return conn
}
