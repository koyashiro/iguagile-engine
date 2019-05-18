package iguagile

import (
	"github.com/google/uuid"
	"log"
	"os"
	"time"

	"github.com/iguagile/iguagile-engine/data"
)

type Room struct {
	id []byte
	clients map[Client]bool
	buffer  map[*[]byte]bool
	log     *log.Logger
}

func NewRoom() *Room {
	uid, err := uuid.NewUUID()
	if err != nil {
		log.Println(err)
	}
	return &Room{
		id:      uid[:],
		clients: make(map[Client]bool),
		buffer:  make(map[*[]byte]bool),
		log:     log.New(os.Stderr, "iguagile-engine ", log.Lshortfile),
	}
}

const (
	allClients = iota
	otherClients
	allClientsBuffered
	otherClientsBuffered
)

const (
	newConnection = iota
	exitConnection
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

func (r *Room) Register(client Client) {
	go client.Run()
	message := append(client.GetID(), newConnection)
	client.SendToOtherClients(message)
	r.clients[client] = true
	for message := range r.buffer {
		client.Send(*message)
	}
	client.AddBuffer(&message)
}

func (r *Room) Receive(sender Client, receivedData []byte) {
	rowData, err := data.NewBinaryData(receivedData, data.Inbound)
	if err != nil {
		log.Println(err)
	}
	message := append(append(sender.GetID(), rowData.MessageType), rowData.Payload...)
	switch rowData.Target {
	case otherClients:
		sender.SendToOtherClients(message)
	case allClients:
		sender.SendToAllClients(message)
	case otherClientsBuffered:
		sender.SendToOtherClients(message)
		sender.AddBuffer(&message)
	case allClientsBuffered:
		sender.SendToAllClients(message)
		sender.AddBuffer(&message)
	default:
		r.log.Println(receivedData)
	}
}
