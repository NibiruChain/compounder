package config

import (
	"github.com/joho/godotenv"
	"log/slog"
	"os"
	"strconv"
)

var (
	GrpcUrl                   string
	GrpcInsecure              bool
	ChainId                   string
	CsvPath                   string
	CompounderMnemonic        string
	CompounderContractAddress string
	CompounderGasMultiplier   float64
	CompounderGasMaxAttempts  int
	CompounderGasLimit        uint64
	CompounderFeeInitial      int64
)

func InitConfig() {
	err := godotenv.Load(".env")
	if err != nil {
		slog.Warn(
			"Error loading env file",
			"filename", ".env",
			"err", err,
		)
	} else {
		slog.Info("Loaded env file")
	}
	GrpcUrl = os.Getenv("GRPC_ENDPOINT")
	GrpcInsecure = os.Getenv("GRPC_INSECURE") == "true"
	ChainId = os.Getenv("CHAIN_ID")

	CompounderMnemonic = os.Getenv("COMPOUNDER_MNEMONIC")
	CsvPath = os.Getenv("CSV_PATH")
	CompounderContractAddress = os.Getenv("COMPOUNDER_CONTRACT_ADDRESS")

	CompounderGasMaxAttempts, _ = strconv.Atoi(os.Getenv("COMPOUNDER_GAS_MAX_ATTEMPTS"))
	CompounderGasLimit, _ = strconv.ParseUint(os.Getenv("COMPOUNDER_GAS_LIMIT"), 10, 64)
	CompounderGasMultiplier, _ = strconv.ParseFloat(os.Getenv("COMPOUNDER_GAS_MULTIPLIER"), 64)
	CompounderFeeInitial, _ = strconv.ParseInt(os.Getenv("COMPOUNDER_FEE_INITIAL"), 10, 64)
}
