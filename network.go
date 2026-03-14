package main

import (
    "io"
    "log"
    "encoding/binary"
    "net/http"
    "net/http/httputil"
    "net/url"
    "strings"
    "time"
    "sync"
    "github.com/gorilla/websocket"
    "flag"
    "fmt"
    "os"
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

var addr = flag.String("addr", "localhost:8080", "http service address")
var overlord = flag.String("overlord", "localhost:7070", "overlord address")

// Effective values after env overlay (set by applyEnvOverrides)
var effectiveOverlordAddr string   // for /update HTTP calls
var effectivePublicWsUrl string   // for registration url param (wss://host/ws)
var varsJsOverlord string         // for client (host or empty if embedded)
var varsJsOverlordPath string     // for client ("/overlord" or "/ws")
var buildVersion string           // set via -ldflags at build time
var effectiveListenAddr string    // for ListenAndServe
var proxyOverlord bool            // true when embedded, proxy /overlord/*

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

func updateOverlord(players int) {
    publicUrl := effectivePublicWsUrl
    if publicUrl == "" {
        publicUrl = "ws://" + *addr + "/ws"
    }
    reqUrl := fmt.Sprintf("http://%s/update?url=%s&players=%d", effectiveOverlordAddr, publicUrl, players)
    resp, err := http.Get(reqUrl)
    if err != nil {
        log.Printf("Error updating overlord: %s", err)
    } else {
        resp.Body.Close()
    }
}

// Represents a single client. Should be allocated in heap
// for each new connection and hold alive by pointer from Network.
type Client struct {
    id uint32
    lifeFrags uint32
    sessionFrags uint32
    ping float64
    outgoing chan []byte
    name []byte
    observer bool
    isBot bool
    lastUpdate time.Time
}

type Message struct {
    from uint32
    data []byte
    ping float64
}

func serveWebsocket(net *Network, w http.ResponseWriter, r *http.Request) {
    // This will be called in a pre-connection goroutine.
    // NOTE(vbo): ignore origin mismatch.
    var upgrader = websocket.Upgrader{
        ReadBufferSize:  1024,
        WriteBufferSize: 1024,
        CheckOrigin: func(r *http.Request) bool { return true },
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
            if len(payload) == 16 && payload[0] == MSG_OUT_PING {
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
            var pingPayload = [16]byte{ MSG_OUT_PING }
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
            if (len(message) > 500) {
                log.Printf("Sending big message of size %d", len(message))
            }
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

func serveVarsJs(w http.ResponseWriter, r *http.Request) {
    if buildVersion == "" {
        buildVersion = "dev"
    }
    fmt.Fprintf(w, "window.overlord='%s';window.overlordPath='%s';window.version='%s';", varsJsOverlord, varsJsOverlordPath, buildVersion)
}

func proxyToOverlord(w http.ResponseWriter, r *http.Request) {
    target, _ := url.Parse("http://localhost:7070")
    path := strings.TrimPrefix(r.URL.Path, "/overlord")
    if path == "" {
        path = "/"
    }
    r.URL.Path = path
    proxy := httputil.NewSingleHostReverseProxy(target)
    proxy.ServeHTTP(w, r)
}

func applyEnvOverrides() {
    if port := os.Getenv("PORT"); port != "" {
        effectiveListenAddr = ":" + port
    } else {
        effectiveListenAddr = *addr
    }
    overlordUrl := os.Getenv("OVERLORD_URL")
    if overlordUrl == "" {
        proxyOverlord = true
        effectiveOverlordAddr = "localhost:7070"
        varsJsOverlord = ""
        varsJsOverlordPath = "/overlord/ws"
        if app := os.Getenv("FLY_APP_NAME"); app != "" {
            effectivePublicWsUrl = "wss://" + app + ".fly.dev/ws"
        } else if pub := os.Getenv("PUBLIC_WS_URL"); pub != "" {
            effectivePublicWsUrl = pub
        } else {
            effectivePublicWsUrl = ""
        }
    } else {
        proxyOverlord = false
        effectiveOverlordAddr = overlordUrl
        varsJsOverlord = overlordUrl
        varsJsOverlordPath = "/ws"
        if app := os.Getenv("FLY_APP_NAME"); app != "" {
            effectivePublicWsUrl = "wss://" + app + ".fly.dev/ws"
        } else if pub := os.Getenv("PUBLIC_WS_URL"); pub != "" {
            effectivePublicWsUrl = pub
        } else {
            effectivePublicWsUrl = "wss://" + overlordUrl + "/ws"
        }
    }
}

func runServer(net *Network) error {
    applyEnvOverrides()
    http.Handle("/", http.FileServer(http.Dir("./public")))
    http.HandleFunc("/vars.js", serveVarsJs)
    if proxyOverlord {
        http.HandleFunc("/overlord/", proxyToOverlord)
        http.HandleFunc("/overlord", proxyToOverlord)
    }
    http.HandleFunc("/ws", func (w http.ResponseWriter, r *http.Request) { serveWebsocket(net, w, r) })
    http.HandleFunc("/stats", func (w http.ResponseWriter, r *http.Request) { serveStats(net, w, r) })

    log.Printf("Starting web server on %s...", effectiveListenAddr)
    return http.ListenAndServe(effectiveListenAddr, nil)
}

func average(vs []float64) float64 {
    var s float64
    for _, v := range vs { s += v }
    return s/float64(len(vs))
}
