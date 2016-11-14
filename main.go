package main

import (
  "flag"
  "log"
  "encoding/binary"
  "math/rand"
  "math"
  "time"
  "runtime"
)

type Tank struct {
    x, y float64
    moving int8
    hp uint32
}

type Bullet struct {
    id uint32
    x, y float64
    vx, vy float64
}

const EXPLOSION_DMG = 100
const EXPLOSION_DMG_FALLOFF = 2
const BULLET_RAD = 30
const BULLET_SPEED = 80
const GRAV_ACC = 5
const GRAV_SPEED = 20
const TANK_RAD = 15
const TANK_SPEED = 10

const WIDTH = 1260
const HEIGHT = 620

const TARGET_TICK_TIME = 20 * time.Millisecond

func NewTank() *Tank {
    return &Tank{
        x: float64(rand.Intn(WIDTH)),
        y: 20,
        hp: 300,
    }
}

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
    dtTotal := time.Duration(0)
    curTick := uint64(0)
    lastDtTotal := time.Duration(0)
    maxDtTotal := time.Duration(0)
    var memStats, prevMemStats runtime.MemStats
    for {
        if curTick % 1000 == 0 {
            runtime.ReadMemStats(&memStats)
            var numGCDiff = memStats.NumGC - prevMemStats.NumGC
            var numMallocs =  memStats.Mallocs - prevMemStats.Mallocs
            var maxPauseNs uint64 = 0
            for i := memStats.NumGC; i > prevMemStats.NumGC; i-- {
                pause := memStats.PauseNs[(i+255)%256] // circular buffer
                if pause > maxPauseNs { maxPauseNs = pause }
            }
            log.Printf("STAT:tick=%d,dt=%s(%s max),gocnt=%d,alloc=(%d mlcs,%d ngc,%f max,%d kb live)",
                curTick, lastDtTotal, maxDtTotal, runtime.NumGoroutine(),
                numMallocs, numGCDiff, float64(maxPauseNs)/1000000, memStats.Alloc / 1024)
            //log.Println(memStats)
            maxDtTotal = 0.0
            prevMemStats = memStats
        }
        select {
        case client := <-net.connect:
            clients[client.id] = client
            tanks[client.id] = NewTank()
            var greetingMsg [5]byte
            greetingMsg[0] = 2
            binary.LittleEndian.PutUint32(greetingMsg[1:], client.id);
            client.outgoing <- greetingMsg[0:]
            client.outgoing <- mapBitmapBuffer[0:]
            log.Printf("Client %d connected.", client.id)
        case client := <-net.disconnect:
            tank := tanks[client.id]
            broadcastDeath(client.id, tank.x, tank.y, TANK_RAD, clients)
            explodeAt(tank.x, tank.y, TANK_RAD, mapBitmap, tanks)
            delete(clients, client.id)
            delete(tanks, client.id)
            close(client.outgoing)
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
                        aimLen := math.Sqrt(aimX*aimX + aimY*aimY)
                        vx := BULLET_SPEED * aimX / aimLen
                        vy := BULLET_SPEED * aimY / aimLen
                        id := net.GetNewObjectId()
                        bullets = append(bullets, Bullet{
                            id: id,
                            x: tanks[client.id].x,
                            y: tanks[client.id].y,
                            vx: vx,
                            vy: vy,
                        })
                        log.Printf("New wild bullet created %d", id)
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
        dtTotalSeconds := dtTotal.Seconds()
        for dtTotalSeconds > 0.0 {
            dt := math.Min(0.005, dtTotalSeconds)
            dtTotalSeconds = dtTotalSeconds - dt
            for _, tank := range tanks {
                oldX := tank.x
                oldY := tank.y
                wasOnGround := isGroundF(tank.x, tank.y + 1.0, mapBitmap)
                if !wasOnGround {
                    // TODO(vbo): fix slow hill descent by falling
                    // instantly for no more than 2 pixels
                    tank.y += GRAV_SPEED * dt
                    if (isGroundF(oldX, tank.y, mapBitmap)) {
                        tank.y = oldY
                    }
                } else {
                    tank.x += float64(tank.moving) * dt * TANK_SPEED
                    if isGroundF(tank.x, oldY, mapBitmap) {
                        for i := uint32(1); i <= 2; i++ {
                            if !isGroundF(tank.x, oldY - float64(i), mapBitmap) {
                                tank.y = oldY - float64(i)
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
                if isGroundF(bullet.x, bullet.y, mapBitmap) {
                    broadcastDeath(bullet.id, bullet.x, bullet.y, BULLET_RAD, clients)
                    explodeAt(bullet.x, bullet.y, BULLET_RAD, mapBitmap, tanks)
                    log.Printf("Bullet crashed at %v, %v", bullet.x, bullet.y)
                    bullets[bulletIndex] = bullets[len(bullets) - 1]
                    bullets = bullets[:len(bullets) - 1]
                } else {
                    bulletIndex++
                }
            }
            // Explode and respawn dead tanks.
            for clientID, tank := range tanks {
                if tank.hp == 0 {
                    broadcastDeath(clientID, tank.x, tank.y, TANK_RAD, clients)
                    explodeAt(tank.x, tank.y, TANK_RAD, mapBitmap, tanks)
                    tanks[clientID] = NewTank()
                }
            }
        }
        // sending updates for tanks
        {
            // TODO:(vbo): scratch memory arena reuse ideas:
            //  - Under normal load we expect any send operation
            //    to be finished after no more than N server ticks.
            //  - Thus instead of alocating scratch memory arena for each
            //    tick's messages we can preallocate an N-sized ring
            //    buffer of arenas and use the next entry each tick.
            //  - Panic situation can be discovered in R/W goroutines
            //    by comparing deadline tick (curTick+N) passed alongside
            //    the data pointer with the actual curTick at the send time.
            //  - A different solution is to preallocate a pool
            //    of reference counted arenas.
            var stateMessageBuffer [2042]byte
            stateMessageBuffer[0] = 1 // message type state update
            stateMessageBuffer[1] = byte(len(clients))
            message := stateMessageBuffer[2:]
            for clientId, _ := range clients {
                binary.LittleEndian.PutUint32(message[0:], clientId);
                binary.LittleEndian.PutUint32(message[4:], uint32(tanks[clientId].x))
                binary.LittleEndian.PutUint32(message[8:], uint32(tanks[clientId].y))
                binary.LittleEndian.PutUint32(message[12:], uint32(tanks[clientId].hp))
                message = message[16:]
            }
            for clientId, _ := range clients {
                clients[clientId].outgoing <- stateMessageBuffer[0 : len(clients) * 16 + 2]
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

        newTime := time.Now()
        dtTotal = newTime.Sub(startTime)
        lastDtTotal = dtTotal
        if lastDtTotal > maxDtTotal { maxDtTotal = lastDtTotal }
        startTime = newTime
        curTick++

        // TODO: improve sleep precision
        time.Sleep(TARGET_TICK_TIME - dtTotal)
    }
}

func broadcastDeath(id uint32, x float64, y float64, radius uint32,
                    clients map[uint32]*Client) {
    var buffer [17]byte
    buffer[0] = 4 // message type death 
    message := buffer[1:]
    binary.LittleEndian.PutUint32(message[0:], id);
    binary.LittleEndian.PutUint32(message[4:], uint32(x));
    binary.LittleEndian.PutUint32(message[8:], uint32(y));
    binary.LittleEndian.PutUint32(message[12:], radius);
    for clientId, _ := range clients {
        clients[clientId].outgoing <- buffer[0:]
    }
    log.Printf("Object %d died bravely", id)
}

func explodeAt(cxf float64, cyf float64, r uint32, mapBitmap []byte, tanks map[uint32]*Tank) {
    // Destroy terrain
    cx := uint32(cxf)
    cy := uint32(cyf)
    rs := r*r
    sx := maxUint32(cx - r, 0)
    sy := maxUint32(cy - r, 0)
    ly := minUint32(cy + r, HEIGHT)
    lx := minUint32(cx + r, WIDTH)
    for y := sy; y < ly; y++ {
        for x := sx; x < lx; x++ {
            ds := (y-cy)*(y-cy) + (x-cx)*(x-cx);
            if ds < rs {
                mapBitmap[x + y * WIDTH] = 0;
            }
        }
    }
    // Hit tanks
    for tankID, tank := range tanks {
        dx := uint32(tank.x) - cx
        dy := uint32(tank.y) - cy
        ds := dx*dx + dy*dy
        dmg := EXPLOSION_DMG - EXPLOSION_DMG_FALLOFF * math.Sqrt(float64(ds))
        if dmg > 0 {
            realDmg := uint32(math.Min(dmg, float64(tank.hp)))
            tanks[tankID].hp -= realDmg
            log.Printf("%d[%d] hit by explosion: -%f", tankID, tank.hp, dmg)
        }
    }
}

func isGroundF(x float64, y float64, mapBitmap []byte) bool {
    mapX := uint32(x)
    mapY := uint32(y)
    return isGroundUi(mapX, mapY, mapBitmap)
}

func isGroundUi(x uint32, y uint32, mapBitmap []byte) bool {
    index := x + y * WIDTH
    return index < 0 || int(index) > len(mapBitmap) || mapBitmap[index] == 1
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

func maxUint32(a uint32, b uint32) uint32 {
    if a > b {
        return a
    }
    return b
}

func minUint32(a uint32, b uint32) uint32 {
    if a < b {
        return a
    }
    return b
}
