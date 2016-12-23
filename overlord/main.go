package main

import "fmt"
import "flag"
import "log"
import "math/rand"
import "time"
import "sync"
import "net/http"
import "net/url"
import "strconv"

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
    if err != nil || u.Scheme != "ws" || len(u.Host) < 1 {
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

func main() {
    var addr = flag.String("addr", ":7070", "http service address")
    flag.Parse()
    rand.Seed(time.Now().UTC().UnixNano())
    http.HandleFunc("/list", serveList)
    http.HandleFunc("/update", serveUpdate)
    http.ListenAndServe(*addr, nil)
}
