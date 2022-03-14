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
	"github.com/status-im/go-waku/waku/v2/protocol/filter"
	"github.com/status-im/go-waku/waku/v2/protocol/pb"
	"github.com/status-im/go-waku/waku/v2/utils"
)

var log = logging.Logger("filter2")

var pubSubTopic = protocol.DefaultPubsubTopic()

const contentTopic = "test"

func main() {
	lvl, err := logging.LevelFromString("info")
	if err != nil {
		panic(err)
	}
	logging.SetAllLoggers(lvl)

	hostAddr1, _ := net.ResolveTCPAddr("tcp", fmt.Sprint("0.0.0.0:60000"))
	hostAddr2, _ := net.ResolveTCPAddr("tcp", fmt.Sprint("0.0.0.0:60001"))

	key1, err := randomHex(32)
	if err != nil {
		log.Error("Could not generate random key")
		return
	}
	prvKey1, err := crypto.HexToECDSA(key1)
	if err != nil {
		log.Error("Invalid key")
		return
	}

	key2, err := randomHex(32)
	if err != nil {
		log.Error("Could not generate random key")
		return
	}

	prvKey2, err := crypto.HexToECDSA(key2)
	if err != nil {
		log.Error("Invalid key")
		return
	}

	ctx := context.Background()

	fullNode, err := node.New(ctx,
		node.WithPrivateKey(prvKey1),
		node.WithHostAddress(hostAddr1),
		node.WithWakuRelay(),
		node.WithWakuFilter(true),
	)

	err = fullNode.Start()
	if err != nil {
		panic(err)
	}

	lightNode, err := node.New(ctx,
		node.WithPrivateKey(prvKey2),
		node.WithHostAddress(hostAddr2),
		node.WithWakuFilter(false),
	)
	if err != nil {
		panic(err)
	}

	_, err = lightNode.AddPeer(fullNode.ListenAddresses()[0], filter.FilterID_v20beta1)
	if err != nil {
		log.Info("Error adding filter peer on light node ", err)
	}

	err = lightNode.Start()
	if err != nil {
		panic(err)
	}

	//
	// Setup filter
	//

	// Send FilterRequest from light node to full node
	cf := filter.ContentFilter{
		Topic:         pubSubTopic.String(),
		ContentTopics: []string{contentTopic},
	}

	_, theFilter, err := lightNode.Filter().Subscribe(ctx, cf)
	if err != nil {
		panic(err)
	}

	go func() {
		for env := range theFilter.Chan {
			log.Info("Light node received msg, ", string(env.Message().Payload))
		}
		log.Info("Message channel closed!")
	}()

	go writeLoop(ctx, fullNode)
	go readLoop(ctx, fullNode)

	go func() {
		// Unsubscribe filter after 5 seconds
		time.Sleep(5 * time.Second)
		lightNode.Filter().UnsubscribeFilter(ctx, cf)
	}()
	// Wait for a SIGINT or SIGTERM signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	fmt.Println("\n\n\nReceived signal, shutting down...")

	// shut the nodes down
	fullNode.Stop()
	lightNode.Stop()
}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func write(ctx context.Context, wakuNode *node.WakuNode, msgContent string) {
	var version uint32 = 0
	var timestamp int64 = utils.GetUnixEpoch()

	p := new(node.Payload)
	p.Data = []byte(wakuNode.ID() + ": " + msgContent)
	p.Key = &node.KeyInfo{Kind: node.None}

	payload, _ := p.Encode(version)

	msg := &pb.WakuMessage{
		Payload:      payload,
		Version:      version,
		ContentTopic: contentTopic,
		Timestamp:    timestamp,
	}

	_, err := wakuNode.Relay().Publish(ctx, msg)
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
	pubsubTopic := pubSubTopic.String()
	sub, err := wakuNode.Relay().SubscribeToTopic(ctx, pubsubTopic)
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
