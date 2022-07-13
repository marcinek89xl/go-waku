package rpc

type SuccessReply = bool

type Empty struct {
}

type MessagesReply = []*RPCWakuMessage

type RelayMessagesReply = []*RPCWakuRelayMessage
