package main

import (
	"flag"
	"log"
  "time"
  "encoding/binary"
)

// game design
//  - Server authoritative.
//  - Moving, shooting (up-down to control aim?)
//  - Input from player:
//     - Move(-1..1), Angle(0..255), Shoot(0..16)
//  - Output from server:
//     - BMP on game load (todo: dynamic)
//     - T0, 16xPlayer: ID, X,Y in pixels, Shoot+Angle+T0

type Tank struct {
  x, y float64
  moving int8
}

const TANK_SPEED = 10.0

func gameLoop(net *Network) {
    clients := make(map[uint64]*Client)
    tanks := make(map[uint64]*Tank)
    startTime := time.Now()
    for {
        select {
        case client := <-net.connect:
            clients[client.id] = client
            tanks[client.id] = &Tank{x:200, y:200}
            log.Printf("Client %d connected.", client.id)
        case client := <-net.disconnect:
            delete(clients, client.id)
            delete(tanks, client.id)
            log.Printf("Client %d disconnected.", client.id)
        case message := <-net.incoming:
            client, ok := clients[message.from]
            if !ok {
                continue // already disconnected, ignore
            }
            tanks[client.id].moving = int8(message.data[0])
            log.Printf("Client %d is moving %d", client.id, tanks[client.id].moving)
        }
        // world simulation 
        newTime := time.Now()
        dt := newTime.Sub(startTime).Seconds()
        for clientId, tank := range tanks {
            tank.x += float64(tank.moving) * dt * TANK_SPEED
            log.Printf("Client %d is moving %f", clientId, tank.x)
            bs := make([]byte, 8)
            binary.LittleEndian.PutUint32(bs, uint32(tank.x))
            binary.LittleEndian.PutUint32(bs[4:], uint32(tank.y))
            clients[clientId].outgoing <- bs
        }
        startTime = newTime
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

