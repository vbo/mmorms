package main

import (
    "io"
    "log"
    "encoding/binary"
    "net/http"
    "time"
    "sync"
    "github.com/gorilla/websocket"
)

import _ "net/http/pprof"

const CONNECTION_QUEUE_SIZE = 256
const MESSAGE_QUEUE_SIZE = 256
const INCOMING_QUEUE_SIZE = 1024
const MAX_MESSAGE_SIZE = 1024
const PONG_TIMEOUT = 10 * time.Second
const WRITE_TIMEOUT = 5 * time.Second
const PING_PERIOD = 1 * time.Second
const PINGS_RING = 8

type StatsRequest struct {
    w io.Writer
    done chan bool
}

// Represents a capability to connect and talk to clients.
// We need just one instance of this for the game.
type Network struct {
    objectIDMutex sync.Mutex
    nextObjectID uint32
    connect chan *Client
    disconnect chan *Client
    incoming chan Message
    statsRequests chan StatsRequest
}

func (net *Network) Init() {
    net.connect = make(chan *Client, CONNECTION_QUEUE_SIZE)
    net.disconnect = make(chan *Client, CONNECTION_QUEUE_SIZE)
    net.incoming = make(chan Message, INCOMING_QUEUE_SIZE)
    net.statsRequests = make(chan StatsRequest, 8)
}

func (net *Network) GetNewObjectId() uint32 {
    net.objectIDMutex.Lock()
    defer net.objectIDMutex.Unlock()
    newID := net.nextObjectID
    net.nextObjectID++
    return newID
}

// Represents a single client. Should be allocated in heap
// for each new connection and hold alive by pointer from Network.
type Client struct {
    id uint32
    frags uint32
    ping float64
    outgoing chan []byte
    observer bool
}

type Message struct {
    from uint32
    data []byte
    ping float64
}

func serveWebsocket(net *Network, w http.ResponseWriter, r *http.Request) {
    // This will be called in a pre-connection goroutine.
    var upgrader = websocket.Upgrader{
        ReadBufferSize:  1024,
        WriteBufferSize: 1024,
    }

    // Upgrade HTTP connection to WebSocket
    // Connections support one concurrent reader and one concurrent writer.
    log.Println("New WebSocket connection request.")
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        http.Error(w, "Method not allowed", 405)
		log.Println(err)
		return
	}

    // TODO(vbo): implement clients pool
    //  - Preallocate a pool of clients for the net.
    //  - Get new client for connection from the pool (with lock).
    //  - Communicate client to the game loop with offsetptr (gc-friendly).
    //  - R/W goroutines use client out and net in channels normally.
    //  - On disconnect force-stop using it's out channel and
    //    communicate disconnected client to the game loop.
    //  - If game loop wants to kick the client it needs to communicate
    //    the intention explicitly (e.g. through outgoing channel).
    //  - Return to pool happens in game loop when handling disconnect msg (with lock).
    //    Don't forget to drain the channel and reset all variables in the client!
    client := &Client{
        id: net.GetNewObjectId(),
        outgoing: make(chan []byte, MESSAGE_QUEUE_SIZE),
        observer: true,
    }

    defer func () {
        select {
        case net.disconnect <- client:
            log.Println("Disconnect queued")
        default:
            log.Println("Disconnect queue overflow")
        }
        conn.Close()
    }()

    select {
    case net.connect <- client:
        log.Println("Connection queued.")
    default:
        log.Println("Connection queue overflow")
        return
    }

    // Pings ring buffer
    var pings = make([]float64, PINGS_RING)
    var pingsi = 0
    var avgping float64

    conn.SetReadLimit(MESSAGE_QUEUE_SIZE)

    // Accept new messages in a separate goroutine:
    // 1. The ReadMessage call blocks, so I don't see other choice.
    // 2. It is allowed to call Read and Write concurrently so why not?
    go func () {
        defer conn.Close()
        log.Printf("Starting R(%d)", client.id)
        // TODO(vbo): make sure this goroutine exits correctly on disconnect.
        // Set the read deadline after which a read becomes timed out,
        // the websocket connection state is corrupt and all future reads will return an error.
        // Deadline will be delayed each time we receive a websocket PONG message.
        delayReadDeadline := func () { conn.SetReadDeadline(time.Now().Add(PONG_TIMEOUT)) }
        conn.SetPongHandler(func(payload string) error {
            delayReadDeadline();
            if len(payload) == 16 && payload[0] == 80 {
                v := int64(binary.LittleEndian.Uint64(([]byte)(payload)[1:]))
                pings[pingsi%PINGS_RING] = float64(time.Now().UnixNano() - v)/1000000
                pingsi++
                avgping = average(pings)
            }
            return nil;
        })
        delayReadDeadline()

        for {
            _, message, err := conn.ReadMessage()
            if err != nil {
                log.Printf("Stopping R(%d): %s", client.id, err)
                break
            }
            //log.Printf("Message received: %d %v", client.id, message)
            msg := Message{ from: client.id, data: message, ping: avgping }
            net.incoming <-msg
        }
    }()

    log.Printf("Starting W(%d)", client.id)
    // Send pings and outgoing messages right in this goroutine:
    //  - We could spin a separate one but we still need to block
    // in this function to keep underlying TCP connection alive.
    pingTicker := time.NewTicker(PING_PERIOD)
    defer pingTicker.Stop()
    for {
        select {
        case <-pingTicker.C:
            // Send PING message, curtime payload
            var pingPayload = [16]byte{ 80 }
            binary.LittleEndian.PutUint64(pingPayload[1:], uint64(time.Now().UnixNano()))
            conn.SetWriteDeadline(time.Now().Add(WRITE_TIMEOUT))
            err := conn.WriteMessage(websocket.PingMessage, pingPayload[:])
            if err != nil {
                return
            }
        case message, ok := <-client.outgoing:
            if !ok {
                // Client kicked explicitly
                conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }
            conn.SetWriteDeadline(time.Now().Add(WRITE_TIMEOUT))
            err := conn.WriteMessage(websocket.BinaryMessage, message)
            if err != nil {
                log.Printf("Stopping W(%d): %s", client.id, err)
                return
            }
        }
    }
}

func serveStats(net *Network, w http.ResponseWriter, r *http.Request) {
    done := make(chan bool)
    net.statsRequests <- StatsRequest{ w, done }
    <-done
}


func runServer(net *Network, addr string) error {
    http.Handle("/", http.FileServer(http.Dir("./public")))
    http.HandleFunc("/ws", func (w http.ResponseWriter, r *http.Request) { serveWebsocket(net, w, r) })
    http.HandleFunc("/stats", func (w http.ResponseWriter, r *http.Request) { serveStats(net, w, r) })

    log.Printf("Starting web server on %s...", addr)
    return http.ListenAndServe(addr, nil)
}

func average(vs []float64) float64 {
    var s float64
    for _, v := range vs { s += v }
    return s/float64(len(vs))
}
