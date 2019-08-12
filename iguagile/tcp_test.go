package iguagile

import (
	"encoding/binary"
	"log"
	"net"
	"os"
	"sync"
	"testing"
)

const host = "127.0.0.1:4000"

func Listen(t *testing.T) {
	store := NewRedis(os.Getenv("REDIS_HOST"))
	serverID, err := store.GenerateServerID()
	if err != nil {
		log.Fatal(err)
	}
	r := NewRoom(serverID, store)

	addr, err := net.ResolveTCPAddr("tcp", host)
	if err != nil {
		t.Errorf("%v", err)
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil && err.Error() != "read: connection reset by peer" {
		t.Errorf("%v", err)
	}
	go func() {
		for {
			conn, err := listener.AcceptTCP()
			if err != nil {
				t.Errorf("%v", err)
			}
			ServeTCP(r, conn)
		}
	}()
}

type testClientTCP struct {
	conn           *net.TCPConn
	isHost         bool
	clientID       uint32
	clientIDByte   []byte
	otherClients   map[uint32]bool
	myObjectID     uint32
	myObjectIDByte []byte
	objects        map[uint32]bool
	objectsLock    *sync.Mutex
}

func newTestClientTCP(conn *net.TCPConn) *testClientTCP {
	return &testClientTCP{
		conn:           conn,
		clientID:       0,
		clientIDByte:   make([]byte, 2),
		objects:        make(map[uint32]bool),
		objectsLock:    &sync.Mutex{},
		myObjectID:     0,
		myObjectIDByte: make([]byte, 4),
		otherClients:   make(map[uint32]bool),
	}
}

func (c *testClientTCP) run(t *testing.T, waitGroup *sync.WaitGroup) {
	log.Println("run")
	//First receive register message and get client id.
	sizeBuf := make([]byte, 2)
	if _, err := c.conn.Read(sizeBuf); err != nil {
		t.Error(err)
	}
	log.Println("read first sizeBuf")

	size := int(binary.LittleEndian.Uint16(sizeBuf))
	if size != 3 {
		t.Errorf("invalid length %v", sizeBuf)
	}
	buf := make([]byte, size)
	if _, err := c.conn.Read(buf); err != nil {
		t.Error(err)
	}
	log.Println("read first message")

	if buf[2] != register {
		t.Errorf("invalid message type %v", buf)
	}

	c.clientID = uint32(binary.LittleEndian.Uint16(buf[:2])) << 16

	// Set object id and send instantiate message.
	c.myObjectID = c.clientID | 1
	binary.LittleEndian.PutUint32(c.myObjectIDByte, c.myObjectID)
	message := append(append([]byte{Server, instantiate}, c.myObjectIDByte...), []byte("iguana")...)
	if err := c.send(message); err != nil {
		t.Error(err)
	}
	log.Println("send instantiate message")

	// Prepare a transform message and rpc message in advance.
	transformMessage := append([]byte{OtherClients, transform}, c.myObjectIDByte...)
	rpcMessage := append([]byte{OtherClients, rpc}, []byte("iguagile")...)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		// Wait for the object to be instantiated before starting sending messages.
		wg.Wait()
		log.Println("start send message")
		for i := 0; i < 1000; i++ {
			if err := c.send(transformMessage); err != nil {
				t.Error(err)
			}
		}

		for i := 0; i < 1000; i++ {
			if err := c.send(rpcMessage); err != nil {
				t.Error(err)
			}
		}

		if c.isHost {
			log.Println("start request object control authority")
			c.objectsLock.Lock()
			for objectID := range c.objects {
				objectIDByte := make([]byte, 4)
				binary.LittleEndian.PutUint32(objectIDByte, objectID)
				message := append([]byte{Server, requestObjectControlAuthority}, objectIDByte...)
				if err := c.send(message); err != nil {
					t.Error(err)
				}
			}
			c.objectsLock.Unlock()

			objectIDByte := make([]byte, 4)
			binary.LittleEndian.PutUint32(objectIDByte, c.myObjectID)
			message := append([]byte{Server, requestObjectControlAuthority}, objectIDByte...)
			if err := c.send(message); err != nil {
				t.Error(err)
			}
		}
	}()
	for {
		// Start receiving messages.
		log.Println("before read")
		if _, err := c.conn.Read(sizeBuf); err != nil {
			t.Error(err)
		}
		log.Println("after read")

		size := int(binary.LittleEndian.Uint16(sizeBuf))
		buf := make([]byte, size)
		if _, err := c.conn.Read(buf); err != nil {
			t.Error(err)
		}

		clientID := uint32(binary.LittleEndian.Uint16(buf)) << 16
		messageType := buf[2]
		payload := buf[3:]
		switch messageType {
		case newConnection:
			log.Println("new connection")
			c.otherClients[clientID] = true
		case exitConnection:
			log.Println("exit connection")
			delete(c.otherClients, clientID)
		case instantiate:
			log.Println("instantiate")
			objectID := binary.LittleEndian.Uint32(payload)
			if clientID == c.clientID {
				wg.Done()
			} else {
				c.objectsLock.Lock()
				c.objects[objectID] = true
				c.objectsLock.Unlock()
			}
		case destroy:
			log.Println("destroy")
			objectID := binary.LittleEndian.Uint32(payload)
			if objectID != c.myObjectID {
				c.objectsLock.Lock()
				delete(c.objects, objectID)
				c.objectsLock.Unlock()
			} else {
				waitGroup.Done()
			}
		case migrateHost:
			log.Println("migrate host")
			c.isHost = true
		case requestObjectControlAuthority:
			log.Println("request")
			objectID := binary.LittleEndian.Uint32(payload)
			if objectID != c.myObjectID {
				t.Errorf("invalid object id %v", buf)
				break
			}

			clientIDByte := make([]byte, 4)
			binary.LittleEndian.PutUint32(clientIDByte, clientID)
			message := append(append([]byte{Server, transferObjectControlAuthority}, payload...), clientIDByte...)
			if err := c.send(message); err != nil {
				t.Error(err)
			}
		case transferObjectControlAuthority:
			log.Println("transfer")
			message := append([]byte{Server, destroy}, payload...)
			if err := c.send(message); err != nil {
				t.Error(err)
			}
		case transform:
			log.Println("transform")
			objectID := binary.LittleEndian.Uint32(payload)
			if objectID == c.myObjectID {
				t.Errorf("invalid object id %v", buf)
			}
		case rpc:
			log.Println("rpc")
			if string(payload) != "iguagile" {
				t.Errorf("invalid rpc data %v", buf)
			}
		default:
			t.Errorf("invalid message type %v", buf)
		}
	}
}

func (c *testClientTCP) send(message []byte) error {
	log.Println("send")
	size := len(message)
	sizeBuf := make([]byte, 2)
	binary.LittleEndian.PutUint16(sizeBuf, uint16(size))
	data := append(sizeBuf, message...)
	log.Println("before write")
	if _, err := c.conn.Write(data); err != nil {
		return err
	}
	log.Println("after write")
	return nil
}

func TestConnectionTCP(t *testing.T) {
	log.Println("TestConnectionTCP")
	Listen(t)
	log.Println("Listen")
	wg := &sync.WaitGroup{}
	const clients = 3
	wg.Add(clients)

	addr, err := net.ResolveTCPAddr("tcp", host)
	if err != nil {
		t.Errorf("%v", err)
	}

	for i := 0; i < clients; i++ {
		log.Println("DialTCP")
		conn, err := net.DialTCP("tcp", nil, addr)
		if err != nil {
			t.Error(err)
		}
		client := newTestClientTCP(conn)
		go client.run(t, wg)
	}

	wg.Wait()
}
