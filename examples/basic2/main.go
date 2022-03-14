package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	logging "github.com/ipfs/go-log"
	"github.com/status-im/go-waku/waku/v2/node"
	"github.com/status-im/go-waku/waku/v2/protocol"
	"github.com/status-im/go-waku/waku/v2/protocol/pb"
	"github.com/status-im/go-waku/waku/v2/utils"
)

var log = logging.Logger("basic2")

func main() {
	lvl, err := logging.LevelFromString("info")
	if err != nil {
		panic(err)
	}
	logging.SetAllLoggers(lvl)

	hostAddr, _ := net.ResolveTCPAddr("tcp", fmt.Sprint("0.0.0.0:0"))

	key, err := randomHex(32)
	if err != nil {
		log.Error("Could not generate random key")
		return
	}
	prvKey, err := crypto.HexToECDSA(key)
	if err != nil {
		log.Error(err)
		return
	}

	ctx := context.Background()

	wakuNode, err := node.New(ctx,
		node.WithPrivateKey(prvKey),
		node.WithHostAddress(hostAddr),
		node.WithWakuRelay(),
	)
	if err != nil {
		log.Error(err)
		return
	}

	if err := wakuNode.Start(); err != nil {
		log.Error(err)
		return
	}

	go writeLoop(ctx, wakuNode)
	go readLoop(ctx, wakuNode)

	// Wait for a SIGINT or SIGTERM signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	fmt.Println("\n\n\nReceived signal, shutting down...")

	// shut the node down
	wakuNode.Stop()

}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func write(ctx context.Context, wakuNode *node.WakuNode, msgContent string) {
	contentTopic := protocol.NewContentTopic("basic2", 1, "test", "proto")

	var version uint32 = 0
	var timestamp int64 = utils.GetUnixEpoch()

	p := new(node.Payload)
	p.Data = []byte(wakuNode.ID() + ": " + msgContent)
	p.Key = &node.KeyInfo{Kind: node.None}

	payload, err := p.Encode(version)
	if err != nil {
		log.Error("Error encoding the payload: ", err)
		return
	}

	msg := &pb.WakuMessage{
		Payload:      payload,
		Version:      version,
		ContentTopic: contentTopic.String(),
		Timestamp:    timestamp,
	}

	_, err = wakuNode.Relay().Publish(ctx, msg)
	if err != nil {
		log.Error("Error sending a message: ", err)
	}
}

func writeLoop(ctx context.Context, wakuNode *node.WakuNode) {
	for {
		time.Sleep(2 * time.Second)
		write(ctx, wakuNode, "Hello world!")
	}
}

func readLoop(ctx context.Context, wakuNode *node.WakuNode) {
	sub, err := wakuNode.Relay().Subscribe(ctx)
	if err != nil {
		log.Error("Could not subscribe: ", err)
		return
	}

	for value := range sub.C {
		payload, err := node.DecodePayload(value.Message(), &node.KeyInfo{Kind: node.None})
		if err != nil {
			fmt.Println(err)
			return
		}

		log.Info("Received msg, ", string(payload.Data))
	}
}
