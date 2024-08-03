package main

import (
	"bytes"
	"context"
	"log"
	"math/big"
	"net/url"
	"os"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/joho/godotenv"
)

type Chain struct {
	ChainId uint64
	Name    string
}

var (
	sourceStartBlock *big.Int = nil
	blockRangeSize            = big.NewInt(5)

	initiatingMessageHash = crypto.Keccak256Hash([]byte("SentMessage(bytes)")) // This is anonymous
	executingMessageHash  = crypto.Keccak256Hash([]byte("ExecutingMessage(bytes32,Identifier)"))

	// map of unique identifier to initiating message to hash found from executing message
	initiatingMsgMap = map[*Identifier]*common.Hash{}
	supportedChains  = []Chain{
		{Name: "Optimism", ChainId: 10},
		{Name: "Base", ChainId: 8453},
	}

	chainToRPCUrl = map[string]string{
		"Optimism": "https://opt-mainnet.g.alchemy.com/v2/",
		"Base":     "https://eth-mainnet.g.alchemy.com/v2/",
	}
)

type Identifier struct {
	Origin      common.Address
	BlockNumber uint64
	LogIndex    uint64
	Timestamp   uint64
	ChainId     uint64
}

func newETHClient(ctx context.Context, chain Chain, alchemyApiKey string) (*ethclient.Client, error) {
	endpoint := &url.URL{Path: chainToRPCUrl[chain.Name] + alchemyApiKey}

	rpcClient, err := rpc.DialOptions(ctx, endpoint.String())
	if err != nil {
		return nil, err
	}
	return ethclient.NewClient(rpcClient), nil
}

func shouldProcessBlock(sourceStartBlock *big.Int, sourceLatestBlock *big.Int) bool {
	var diff *big.Int
	diff.Sub(sourceLatestBlock, blockRangeSize)

	comparedToStartBlock := diff.Cmp(sourceStartBlock)

	return comparedToStartBlock == -1 || comparedToStartBlock == 0
}

func decodeExecutingMessage(messageLog *types.Log) (*Identifier, error) {
	var identifier Identifier
	if err := rlp.Decode(bytes.NewReader(messageLog.Data), &identifier); err != nil {
		return nil, err
	}
	return &identifier, nil
}

func main() {
	ctx := context.Background()
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}
	alchemyApiKey := os.Getenv("ALCHEMY_API_KEY")

	go func() {
		sourceChain := supportedChains[0]
		sourceClient, err := newETHClient(ctx, sourceChain, alchemyApiKey)
		if err != nil {
			log.Fatalf("Error creating source client: %v", err)
		}
		sourceStartBlock = big.NewInt(100)
		sourceLatestBlock, err := sourceClient.BlockByNumber(ctx, nil)
		if err != nil {
			log.Fatalf("Error getting source latest block: %v", err)
		}

		if shouldProcessBlock(sourceStartBlock, sourceLatestBlock.Number()) {
			var toBlock *big.Int
			toBlock.Sub(sourceLatestBlock.Number(), blockRangeSize)

			logs, err := sourceClient.FilterLogs(
				ctx,
				ethereum.FilterQuery{
					FromBlock: sourceStartBlock,
					ToBlock:   toBlock,
					Topics:    nil,
				})
			if err != nil {
				log.Fatalf("Error getting source logs: %v", err)
			}

			for _, identifiedLog := range logs {
				for _, topic := range identifiedLog.Topics {
					if topic == initiatingMessageHash {
						identifier := &Identifier{
							Origin:      identifiedLog.Address,
							BlockNumber: identifiedLog.BlockNumber,
							LogIndex:    uint64(identifiedLog.Index),
							Timestamp:   uint64(sourceLatestBlock.ReceivedAt.Unix()),
							ChainId:     sourceChain.ChainId,
						}
						initiatingMsgMap[identifier] = nil
					}
				}
			}
		}
	}()

	go func() {
		destinationChain := supportedChains[0]
		destinationClient, err := newETHClient(ctx, destinationChain, alchemyApiKey)
		if err != nil {
			log.Fatalf("Error creating destination client: %v", err)
		}

		destinationStartBlock := big.NewInt(100)
		destinationLatestBlock, err := destinationClient.BlockByNumber(ctx, nil)
		if err != nil {
			log.Fatalf("Error getting source latest block: %v", err)
		}
		if shouldProcessBlock(destinationStartBlock, destinationLatestBlock.Number()) {
			var toBlock *big.Int
			toBlock.Sub(destinationLatestBlock.Number(), blockRangeSize)

			logs, err := destinationClient.FilterLogs(
				ctx,
				ethereum.FilterQuery{
					FromBlock: destinationStartBlock,
					ToBlock:   toBlock,
					Topics:    nil,
				})
			if err != nil {
				log.Fatalf("Error getting source logs: %v", err)
			}

			for _, identifiedLog := range logs {
				if len(identifiedLog.Topics) < 2 {
					continue
				}
				msgHash := identifiedLog.Topics[1]
				for _, topic := range identifiedLog.Topics {
					if topic == executingMessageHash {
						identifier, err := decodeExecutingMessage(&identifiedLog)
						if err != nil {
							log.Fatalf("Error decoding executing message: %v", err)
						}

						if initiatingMsgMap[identifier] == nil {
							initiatingMsgMap[identifier] = &msgHash
						}
					}
				}
			}
		}

	}()
}
