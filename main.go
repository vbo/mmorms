package main

import (
	"flag"
	"log"
)

// game design
//  - Server authoritative.
//  - Moving, shooting (up-down to control aim?)
//  - Input from player:
//     - Move(-1..1), Angle(0..255), Shoot(0..16)
//  - Output from server:
//     - BMP on game load (todo: dynamic)
//     - T0, 16xPlayer: ID, X,Y in pixels, Shoot+Angle+T0

// chat example
func gameLoop(net *Network) {
    clients := make(map[uint64]*Client)
    for {
        select {
        case client := <-net.connect:
            clients[client.id] = client
            log.Printf("Client %d connected.", client.id)
        case client := <-net.disconnect:
            delete(clients, client.id)
            log.Printf("Client %d disconnected.", client.id)
        case message := <-net.incoming:
            sender, ok := clients[message.from]
            if !ok {
                continue // already disconnected, ignore
            }
            for _, receiver := range clients {
                if receiver.id != sender.id {
                    receiver.outgoing <- message.data
                }
            }
            log.Printf("Broadcasted to %d clients", len(clients)-1)
        }
    }
}

func main() {
    flag.Parse()
    var addr = flag.String("addr", ":8080", "http service address")

    var net Network
    net.Init()

    go gameLoop(&net)

    err := runServer(&net, *addr)
    if err != nil {
        log.Fatal("Server: ", err)
    }
}

