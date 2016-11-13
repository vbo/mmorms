package main

import (
  "flag"
  "log"
  "encoding/binary"
  "math"
  "time"
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

type Bullet struct {
    id uint32
    x, y float64
    vx, vy float64
}

const BULLET_SPEED = 30.0
const TANK_SPEED = 10.0
const GRAV_SPEED = 20.0
const GRAV_ACC = 5.0
const WIDTH = 1260
const HEIGHT = 620

func gameLoop(net *Network) {
    clients := make(map[uint32]*Client)
    tanks := make(map[uint32]*Tank)
    bullets := make([]Bullet, 0, 320)
    var mapBitmapBuffer [WIDTH * HEIGHT + 1]byte
    mapBitmapBuffer[0] = 0 // type of message mapBitmap init
    mapBitmap := mapBitmapBuffer[1:]

    for x := 0; x < WIDTH; x++ {
        groundCurve := 200 + (float64(x) / 400.0 * math.Sin(float64(x) / 100.0) + 1.2) * 100;
        for y := 0; y < HEIGHT; y++ {
            if (float64(y) < groundCurve) {
                mapBitmap[x + y * WIDTH] = 0;
            } else {
                mapBitmap[x + y * WIDTH] = 1;
            }
        }
    }
    startTime := time.Now()
    for {
        select {
        case client := <-net.connect:
            clients[client.id] = client
            tanks[client.id] = &Tank{x:900, y:300}
            var greetingMsg [5]byte
            greetingMsg[0] = 2
            binary.LittleEndian.PutUint32(greetingMsg[1:], client.id);
            client.outgoing <- greetingMsg[0:]
            client.outgoing <- mapBitmapBuffer[0:]
            log.Printf("Client %d connected.", client.id)
        case client := <-net.disconnect:
            delete(clients, client.id)
            delete(tanks, client.id)
            log.Printf("Client %d disconnected.", client.id)
        case message := <-net.incoming:
            msgNum := len(net.incoming)
            // log.Printf("Number of messages: %d", msgNum)
            for {
                client, ok := clients[message.from]
                if !ok {
                    continue // already disconnected, ignore
                }
                switch message.data[0] {
                    case 0: // moving
                        tanks[client.id].moving = int8(message.data[1])
                        if tanks[client.id].moving != 0 {
                            log.Printf("Client %d is moving %d", client.id, tanks[client.id].moving)
                        }
                    case 1: // shooting
                        aimX := float64(int32(binary.LittleEndian.Uint32(message.data[1:])))
                        aimY := float64(int32(binary.LittleEndian.Uint32(message.data[5:])))
                        log.Printf("Aim %v %v", aimX, aimY)
                        aimLen := math.Sqrt(aimX*aimX + aimY*aimY)
                        vx := BULLET_SPEED * aimX / aimLen
                        vy := BULLET_SPEED * aimY / aimLen
                        bullets = append(bullets, Bullet{
                            id: net.GetNewObjectId(),
                            x: tanks[client.id].x,
                            y: tanks[client.id].y,
                            vx: vx,
                            vy: vy })
                }
                if msgNum == 0 {
                    break
                }
                message = <-net.incoming
                msgNum--
            }
        default:
        }
        // world simulation 
        newTime := time.Now()
        dtTotal := newTime.Sub(startTime).Seconds()
        for dtTotal > 0.0 {
            dt := math.Min(0.005, dtTotal)
            dtTotal = dtTotal - dt
            for _, tank := range tanks {
                oldX := tank.x
                oldY := tank.y
                mapX := uint32(tank.x)
                mapY := uint32(tank.y)
                wasOnGround := mapBitmap[mapX + (mapY + 1) * WIDTH] == 1
                if (!wasOnGround) {
                    tank.y += GRAV_SPEED * dt
                    if (mapBitmap[mapX + uint32(tank.y) * WIDTH] == 1) {
                        tank.y = oldY
                    }
                } else {
                    tank.x += float64(tank.moving) * dt * TANK_SPEED
                    if mapBitmap[uint32(tank.x) + mapY * WIDTH] == 1 {
                        for i := uint32(1); i <= 2; i++ {
                            if mapBitmap[uint32(tank.x) + (mapY - i) * WIDTH] == 0 {
                                tank.y = float64(mapY - i)
                                break
                            }
                        }
                        if tank.y == oldY {
                            tank.x = oldX
                        }
                    }
                }
            }
            bulletIndex := 0
            for bulletIndex < len(bullets) {
                bullet := &bullets[bulletIndex]
                bullet.x += bullet.vx * dt
                bullet.y += bullet.vy * dt
                bullet.vy += GRAV_ACC * dt
                if (mapBitmap[uint32(bullet.x) + uint32(bullet.y) * WIDTH] == 1) {
                    // explode & remove bullet
                    bullets[bulletIndex] = bullets[len(bullets) - 1]
                    bullets = bullets[:len(bullets) - 1]
                    log.Printf("Bullet crashed at %v, %v", bullet.x, bullet.y)
                } else {
                    bulletIndex++
                }
            }
        }

        // sending updates for tanks
        {
            var stateMessageBuffer [2042]byte
            stateMessageBuffer[0] = 1 // message type state update
            stateMessageBuffer[1] = byte(len(clients))
            message := stateMessageBuffer[2:]
            for clientId, _ := range clients {
                binary.LittleEndian.PutUint32(message[0:], clientId);
                binary.LittleEndian.PutUint32(message[4:], uint32(tanks[clientId].x))
                binary.LittleEndian.PutUint32(message[8:], uint32(tanks[clientId].y))
                message = message[12:]
            }
            for clientId, _ := range clients {
                clients[clientId].outgoing <- stateMessageBuffer[0 : len(clients) * 12 + 2]
            }
        }

        // sedning updates for bullets
        {
            var bulletsMessageBuffer [2560]byte
            bulletsMessageBuffer[0] = 3 // message type bullets update
            bulletsMessageBuffer[1] = byte(len(bullets))
            message := bulletsMessageBuffer[2:]
            for _, bullet := range bullets {
                binary.LittleEndian.PutUint32(message[0:], bullet.id);
                binary.LittleEndian.PutUint32(message[4:], uint32(bullet.x))
                binary.LittleEndian.PutUint32(message[8:], uint32(bullet.y))
                message = message[12:]
            }
            for clientId, _ := range clients {
                clients[clientId].outgoing <- bulletsMessageBuffer[0 : len(bullets) * 12 + 2]
            }
        }

        startTime = newTime
        time.Sleep(20 * time.Millisecond)
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

