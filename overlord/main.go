// Package main runs the overlord matchmaking server: keeps a registry of game
// servers (/update), serves the list to clients (/list), and provides an
// optional WebSocket endpoint. Can run standalone or be embedded in mmorms.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Host struct {
    updated time.Time
    players int
}

// TODO(vbo): probably not the best data structure.
var hostmapMutex = sync.Mutex{}
var hostmap = make(map[string]Host, 64)

func serveList(w http.ResponseWriter, r *http.Request) {
    hostmapMutex.Lock()
    defer hostmapMutex.Unlock()
    for url, host := range hostmap {
        fmt.Fprintf(w, "%s\t%d\n", url, host.players)
    }
}

func serveUpdate(w http.ResponseWriter, r *http.Request) {
    // TODO(vbo): check if this is really our server.
    requestStart := time.Now()
    hostmapMutex.Lock()
    defer hostmapMutex.Unlock()

    q := r.URL.Query()
    hostUrl := q.Get("url")
    u, err := url.ParseRequestURI(hostUrl)
    if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") || len(u.Host) < 1 {
        http.Error(w, "Bad request", 400)
        log.Printf("Invalid host url %s %s", hostUrl, err)
        return
    }

    players, err := strconv.Atoi(q.Get("players"))
    if err != nil {
        http.Error(w, "Bad request", 400)
        log.Println(err)
        return
    }

    host := hostmap[hostUrl]
    host.players = players
    host.updated = requestStart
    hostmap[hostUrl] = host

    // Expire dead hosts
    if rand.Intn(100) > 10 {
        for name, host := range hostmap {
            if requestStart.Sub(host.updated) > 5 * time.Second {
                delete(hostmap, name)
            }
        }
    }
}

const MESSAGE_QUEUE_SIZE = 256
const PONG_TIMEOUT = 10 * time.Second
const WRITE_TIMEOUT = 5 * time.Second
const PING_PERIOD = 1 * time.Second

func serveWebsocket(w http.ResponseWriter, r *http.Request) {
    var upgrader = websocket.Upgrader{
        ReadBufferSize:  1024,
        WriteBufferSize: 1024,
        CheckOrigin: func(r *http.Request) bool { return true },
    }

    log.Println("New WebSocket connection request.")
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        http.Error(w, "Method not allowed", 405)
		log.Println(err)
		return
	}
    defer conn.Close()

    conn.SetReadLimit(MESSAGE_QUEUE_SIZE)
    go func () {
        defer conn.Close()
        delayReadDeadline := func () { conn.SetReadDeadline(time.Now().Add(PONG_TIMEOUT)) }
        conn.SetPongHandler(func(payload string) error { delayReadDeadline(); return nil; })
        delayReadDeadline()
        for {
            _, message, err := conn.ReadMessage()
            if err != nil {
                break
            }
            if message[0] == 80 { // MSG_OUT_PING
                conn.WriteMessage(websocket.BinaryMessage, message)
            } else if message[0] == 0 { // LIST
                listMessage := buildListMessage()
                conn.WriteMessage(websocket.TextMessage, listMessage)
            }
        }
    }()

    pingTicker := time.NewTicker(PING_PERIOD)
    defer pingTicker.Stop()
    for {
        <-pingTicker.C
        conn.SetWriteDeadline(time.Now().Add(WRITE_TIMEOUT))
        err := conn.WriteMessage(websocket.PingMessage, []byte{})
        if err != nil {
            return
        }
    }
}

func buildListMessage() []byte {
    buffer := bytes.Buffer{}
    hostmapMutex.Lock()
    defer hostmapMutex.Unlock()
    for url, host := range hostmap {
        buffer.WriteString(fmt.Sprintf("%s\t%d\n", url, host.players))
    }
    return buffer.Bytes()
}

func main() {
    var addr = flag.String("addr", ":7070", "http service address")
    flag.Parse()
    port := os.Getenv("PORT")
    if port != "" {
        *addr = ":" + port
    }
    rand.Seed(time.Now().UTC().UnixNano())
    http.HandleFunc("/list", serveList)
    http.HandleFunc("/update", serveUpdate)
    http.HandleFunc("/ws", serveWebsocket)
    http.ListenAndServe(*addr, nil)
}
