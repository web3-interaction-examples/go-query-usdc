package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// USDC contract address
const USDC_CONTRACT_ADDRESS = "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"

// Transfer event signature
const TRANSFER_EVENT_SIGNATURE = "Transfer(address,address,uint256)"

// USDC interface
type USDC interface {
	Decimals(opts *bind.CallOpts) (uint8, error)
}

func main() {
	// Connect to the Ethereum mainnet
	client, err := ethclient.Dial("https://eth.llamarpc.com")
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum mainnet: %v", err)
	}

	// Get the USDC contract instance
	usdcAddress := common.HexToAddress(USDC_CONTRACT_ADDRESS)
	usdc, err := NewUSDC(usdcAddress, client)
	if err != nil {
		log.Fatalf("Failed to create USDC contract instance: %v", err)
	}

	// Get the USDC decimal places
	decimals, err := usdc.Decimals(&bind.CallOpts{})
	if err != nil {
		log.Fatalf("Failed to get USDC decimal places: %v", err)
	}

	fmt.Printf("USDC decimal places: %d\n", decimals)

	// Get the latest block number
	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		log.Fatalf("Failed to get the latest block number: %v", err)
	}
	latestBlock := header.Number.Uint64()

	// Calculate the start block number (last 100 blocks)
	startBlock := latestBlock - 99
	if startBlock < 0 {
		startBlock = 0
	}

	// Query USDC transfer records
	err = getUSDCTransfers(client, startBlock, latestBlock, decimals)
	if err != nil {
		log.Fatalf("Failed to query USDC transfer records: %v", err)
	}
}

func getUSDCTransfers(client *ethclient.Client, startBlock uint64, endBlock uint64, decimals uint8) error {
	usdcAddress := common.HexToAddress(USDC_CONTRACT_ADDRESS)
	transferSig := []byte(TRANSFER_EVENT_SIGNATURE)
	transferTopic := crypto.Keccak256Hash(transferSig)

	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(startBlock)),
		ToBlock:   big.NewInt(int64(endBlock)),
		Addresses: []common.Address{usdcAddress},
		Topics:    [][]common.Hash{{transferTopic}},
	}

	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		return fmt.Errorf("Failed to filter logs: %v", err)
	}

	fmt.Printf("Found %d USDC transfer records between blocks %d and %d\n", len(logs), startBlock, endBlock)

	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)

	for _, vLog := range logs {
		from := common.HexToAddress(vLog.Topics[1].Hex())
		to := common.HexToAddress(vLog.Topics[2].Hex())
		amount := new(big.Int).SetBytes(vLog.Data)
		amount = new(big.Int).Div(amount, divisor)

		transferType := "Transfer"
		if from == common.HexToAddress("0x0000000000000000000000000000000000000000") {
			transferType = "Mint"
		}

		fmt.Printf("Block #%d: %s from %s to %s, amount: %s USDC\n",
			vLog.BlockNumber, transferType, from.Hex(), to.Hex(), amount.String())
	}

	return nil
}

// NewUSDC creates a new USDC instance
func NewUSDC(address common.Address, backend bind.ContractBackend) (USDC, error) {
	parsed, err := abi.JSON(strings.NewReader(USDCABI))
	if err != nil {
		return nil, err
	}
	contract := bind.NewBoundContract(address, parsed, backend, backend, backend)
	return &usdcCaller{contract: contract}, nil
}

// USDC ABI
const USDCABI = `[{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"type":"function"},{"constant":false,"inputs":[{"name":"newImplementation","type":"address"}],"name":"upgradeTo","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":false,"inputs":[{"name":"newImplementation","type":"address"},{"name":"data","type":"bytes"}],"name":"upgradeToAndCall","outputs":[],"payable":true,"stateMutability":"payable","type":"function"},{"constant":true,"inputs":[],"name":"implementation","outputs":[{"name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"name":"newAdmin","type":"address"}],"name":"changeAdmin","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[],"name":"admin","outputs":[{"name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"},{"inputs":[{"name":"_implementation","type":"address"}],"payable":false,"stateMutability":"nonpayable","type":"constructor"},{"payable":true,"stateMutability":"payable","type":"fallback"},{"anonymous":false,"inputs":[{"indexed":false,"name":"previousAdmin","type":"address"},{"indexed":false,"name":"newAdmin","type":"address"}],"name":"AdminChanged","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"name":"implementation","type":"address"}],"name":"Upgraded","type":"event"}]`

// struct
type usdcCaller struct {
	contract *bind.BoundContract
}

// Decimals
func (u *usdcCaller) Decimals(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := u.contract.Call(opts, &out, "decimals")
	if err != nil {
		return 0, err
	}
	return *abi.ConvertType(out[0], new(uint8)).(*uint8), nil
}
