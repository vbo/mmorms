package main

import (
    "log"
    "net/http"
    "time"
    "sync"

    "github.com/gorilla/websocket"
)

const CONNECTION_QUEUE_SIZE = 256
const MESSAGE_QUEUE_SIZE = 256
const INCOMING_QUEUE_SIZE = 1024
const MAX_MESSAGE_SIZE = 1024
const PONG_TIMEOUT = 2 * time.Second
const PING_PERIOD = 1 * time.Second

// Represents a capability to connect and talk to clients.
// We need just one instance of this for the game.
type Network struct {
    clientIDMutex sync.Mutex
    nextClientID uint64
    connect chan *Client
    disconnect chan *Client
    incoming chan Message
}

func (net *Network) Init() {
    net.connect = make(chan *Client, CONNECTION_QUEUE_SIZE)
    net.disconnect = make(chan *Client, CONNECTION_QUEUE_SIZE)
    net.incoming = make(chan Message, INCOMING_QUEUE_SIZE)
}

func (net *Network) GetNewClientID() uint64 {
    net.clientIDMutex.Lock()
    defer net.clientIDMutex.Unlock()
    newID := net.nextClientID
    net.nextClientID++
    return newID
}

// Represents a single client. Should be allocated in heap
// for each new connection and hold alive by pointer from Network.
type Client struct {
    id uint64
    outgoing chan []byte
}

type Message struct {
    from uint64
    data []byte
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

    client := &Client{
        id: net.GetNewClientID(),
        outgoing: make(chan []byte, MESSAGE_QUEUE_SIZE),
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

    conn.SetReadLimit(MESSAGE_QUEUE_SIZE)
    conn.SetWriteDeadline(time.Time{}) // writes won't timeout

    // Accept new messages in a separate goroutine:
    // 1. The ReadMessage call blocks, so I don't see other choice.
    // 2. It is allowed to call Read and Write concurrently so why not?
    go func () {
        // Set the read deadline after which a read becomes timed out,
        // the websocket connection state is corrupt and all future reads will return an error.
        // Deadline will be delayed each time we receive a websocket PONG message.
        delayReadDeadline := func () { conn.SetReadDeadline(time.Now().Add(PONG_TIMEOUT)) }
        conn.SetPongHandler(func(string) error { delayReadDeadline(); return nil })
        delayReadDeadline()

        for {
            log.Println("Blocked on read")
            _, message, err := conn.ReadMessage()
            if err != nil {
                if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
                    log.Printf("error: %v", err)
                }
                break
            }
            log.Printf("Message received: %d %v", client.id, message)
            net.incoming <- Message{ from: client.id, data: message }
        }
    }()

    // Send pings and outgoing messages right in this goroutine:
    //  - We could spin a separate one but we still need to block
    // in this function to keep underlying TCP connection alive.
    pingTicker := time.NewTicker(PING_PERIOD)
    defer pingTicker.Stop()
    for {
        select {
        case <-pingTicker.C:
            err := conn.WriteMessage(websocket.PingMessage, []byte{})
            if err != nil {
                return
            }
        case message, ok := <-client.outgoing:
            if !ok {
                // Client kicked explicitly
                conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }
            err := conn.WriteMessage(websocket.BinaryMessage, message)
            if err != nil {
                return
            }
            log.Printf("Message sent: %d %v", client.id, message)
        }
    }
}

func runServer(net *Network, addr string) error {
    http.Handle("/", http.FileServer(http.Dir("./public")))
    http.HandleFunc("/ws", func (w http.ResponseWriter, r *http.Request) { serveWebsocket(net, w, r) })

    log.Printf("Starting web server on %s...", addr)
    return http.ListenAndServe(addr, nil)
}
