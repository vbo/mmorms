package main

import (
  "flag"
  "log"
  "encoding/binary"
  "math/rand"
  "math"
  "time"
  "runtime"
  "fmt"
)

type Tank struct {
    x, y float64
    moving int8
    gunAngle int32
    hp uint32
}

type Bullet struct {
    id uint32
    ownerId uint32
    x, y float64
    vx, vy float64
}

const EXPLOSION_DMG = 100
const EXPLOSION_DMG_FALLOFF = 2

const BULLET_EXPLOSION_RAD = 30
const BULLET_RAD = 5
const BULLET_SPEED = 20

const GRAV_ACC = 30
const GRAV_SPEED = 100

const TANK_RAD = 15
const TANK_SPEED = 20
const TANK_TOWER_HEIGHT = 25
const TANK_GUN_LENGTH = 30
const TANK_WIDTH = 50
const TANK_HEIGHT = 20

const WIDTH = 1260
const HEIGHT = 620

const MSG_OUT_MAP = 0
const MSG_OUT_GREETING = 2
const MSG_OUT_DEATH = 4
const MSG_OUT_STATE = 1
const MSG_OUT_BULLET_STATE = 3

const MSG_IN_MOVING = 0
const MSG_IN_SHOOTING = 1
const MSG_IN_START = 32

const NUM_BOTS = 5

const TARGET_TICK_TIME = 20 * time.Millisecond

func NewTank() *Tank {
    return &Tank{
        x: float64(rand.Intn(WIDTH)),
        y: 20,
        hp: 100,
    }
}

func generateMapBitmap(mapBitmap []byte) {
    for x := 0; x < WIDTH; x++ {
        groundCurve := 200 + (float64(x) / 400.0 * math.Sin(float64(x) / 120.0) + 1.2) * 100;
        for y := 0; y < HEIGHT; y++ {
            if (float64(y) < groundCurve) {
                mapBitmap[x + y * WIDTH] = 0;
            } else {
                mapBitmap[x + y * WIDTH] = 1;
            }
        }
    }
}

func simulateWorld(tanks map[uint32]*Tank, bulletsIn *[]Bullet, mapBitmap []byte, dtTotalSeconds float64, clients map[uint32]*Client) {
    bullets := *bulletsIn
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
                    for i := 1; i <= 2; i++ {
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
            if (isGroundInCircle(
                    coordToPixel(bullet.x),
                    coordToPixel(bullet.y),
                    BULLET_RAD,
                    mapBitmap) ||
                isPointInTank(
                    coordToPixel(bullet.x),
                    coordToPixel(bullet.y),
                    tanks,
                    bullet.ownerId))  {
                broadcastDeath(bullet.id, bullet.x, bullet.y, BULLET_EXPLOSION_RAD, clients)
                explodeAt(
                    coordToPixel(bullet.x),
                    coordToPixel(bullet.y),
                    BULLET_EXPLOSION_RAD,
                    mapBitmap,
                    tanks,
                    clients[bullet.ownerId])
                //log.Printf("Bullet crashed at %v, %v", bullet.x, bullet.y)
                bullets[bulletIndex] = bullets[len(bullets) - 1]
                bullets = bullets[:len(bullets) - 1]
            } else {
                bulletIndex++
            }
        }
        // Explode and respawn dead tanks.
        for tankID, tank := range tanks {
            if tank.hp == 0 {
                broadcastDeath(tankID, tank.x, tank.y, TANK_RAD, clients)
                explodeAt(
                    coordToPixel(tank.x),
                    coordToPixel(tank.y),
                    TANK_RAD,
                    mapBitmap,
                    tanks,
                    clients[tankID])
                delete(tanks, tankID)
            }
        }
    }

    *bulletsIn = bullets
}

func gameLoop(net *Network) {
    clients := make(map[uint32]*Client)
    tanks := make(map[uint32]*Tank)
    bullets := make([]Bullet, 0, 320)
    mapBitmapBuffer := make([]byte, WIDTH * HEIGHT + 1)
    mapBitmapBuffer[0] = MSG_OUT_MAP
    mapBitmap := mapBitmapBuffer[1:]

    // TODO(vbo): load from predesigned file?
    generateMapBitmap(mapBitmap)

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

        // Make a GC-ed copy of the mapBitmap to respond to connects
        // TODO: can we allocate the space just once and reuse it?
        mapBitmapBufferCopy := make([]byte, len(mapBitmapBuffer))
        copy(mapBitmapBufferCopy, mapBitmapBuffer)

        // Handle incoming messages
        select {
        case client := <-net.connect:
            clients[client.id] = client
            var greetingMsg [5]byte
            greetingMsg[0] = MSG_OUT_GREETING
            binary.LittleEndian.PutUint32(greetingMsg[1:], client.id);
            //log.Printf("Client %d connected.", client.id)
            client.outgoing <- greetingMsg[0:]
            client.outgoing <- mapBitmapBufferCopy[0:]
            //log.Println("Map & greetings sent")
            if !client.observer {
                tanks[client.id] = NewTank()
            }
        case client := <-net.disconnect:
            tank, ok := tanks[client.id]
            if ok {
                broadcastDeath(client.id, tank.x, tank.y, TANK_RAD, clients)
                explodeAt(
                    coordToPixel(tank.x),
                    coordToPixel(tank.y),
                    TANK_RAD,
                    mapBitmap,
                    tanks,
                    client)
                delete(tanks, client.id)
            }
            delete(clients, client.id)
            close(client.outgoing)
            log.Printf("Client %d disconnected.", client.id)
        case statsRequest := <- net.statsRequests:
            for _, client := range clients {
                fmt.Fprintf(statsRequest.w,
                            "%d\t%f\t%d\n",
                            client.id,
                            client.ping,
                            client.frags)
            }
            statsRequest.done <- true
        case message := <-net.incoming:
            msgNum := len(net.incoming)
            /*
            if msgNum > 1 {
              log.Printf("Number of messages: %d", msgNum)
            } */
            for {
                client, ok := clients[message.from]
                if !ok {
                    /** 
                      Client is either disconnected or is a bot
                      whose connection was not established yet.
                      We should break and wait for connection to be
                      established first. If there are more messages
                      waiting - they will be processed next time.
                    */
                    break
                }
                client.ping = message.ping
                switch message.data[0] {
                    case MSG_IN_MOVING: // moving
                        _, ok := tanks[client.id]
                        if ok {
                            tanks[client.id].moving = int8(message.data[1])
                            tanks[client.id].gunAngle =
                                int32(binary.LittleEndian.Uint32(message.data[2:]))
                        }

                    case MSG_IN_SHOOTING: // shooting
                        _, ok := tanks[client.id]
                        if ok {
                            aimX := float64(int32(binary.LittleEndian.Uint32(message.data[1:])))
                            aimY := float64(int32(binary.LittleEndian.Uint32(message.data[5:])))
                            power := float64(message.data[9])
                            if power <= 0 {
                                power = 1
                            } else if power > 10 {
                                power = 10
                            }
                            aimLen := math.Sqrt(aimX*aimX + aimY*aimY)
                            aimX /= aimLen; aimY /= aimLen
                            vx := BULLET_SPEED * aimX * power
                            vy := BULLET_SPEED * aimY * power
                            id := net.GetNewObjectId()
                            x := tanks[client.id].x + TANK_GUN_LENGTH * aimX
                            y := tanks[client.id].y - TANK_TOWER_HEIGHT * (1 - aimY)
                            bullets = append(bullets, Bullet{
                                id: id,
                                ownerId: client.id,
                                x: x,
                                y: y,
                                vx: vx,
                                vy: vy,
                            })
                            //log.Printf("New wild bullet created %d", id)
                        }

                    case MSG_IN_START: // start
                        var clientID = message.from
                        _, ok := tanks[clientID]
                        if !ok {
                            var login = string(message.data[1:])
                            log.Printf("%s joined", login)
                            tanks[clientID] = NewTank()
                        }

                    // TODO(vbo): what if no cases match message type?
                }
                if msgNum == 0 {
                    break
                }
                message = <-net.incoming
                msgNum--
                //log.Printf ("%d messages left", msgNum)
            }
        default:
        }

        // world simulation 
        dtTotalSeconds := dtTotal.Seconds()
        simulateWorld(tanks, &bullets, mapBitmap, dtTotalSeconds, /* TODO(vbo): remove */ clients)

        // Broadcast world snapshot
        broadcastTanks(tanks, clients)
        broadcastBullets(bullets, clients)

        // Bookkeeping
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

func broadcastTanks(tanks map[uint32]*Tank, clients map[uint32]*Client) {
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
    stateMessageBuffer[0] = MSG_OUT_STATE
    stateMessageBuffer[1] = byte(len(tanks))
    message := stateMessageBuffer[2:]
    for tankId, tank := range tanks {
        binary.LittleEndian.PutUint32(message[0:], tankId);
        binary.LittleEndian.PutUint32(message[4:], uint32(tank.x))
        binary.LittleEndian.PutUint32(message[8:], uint32(tank.y))
        binary.LittleEndian.PutUint32(message[12:], uint32(tank.hp))
        binary.LittleEndian.PutUint32(message[16:], uint32(tank.gunAngle))
        message = message[20:]
    }

    for clientId, _ := range clients {
        clients[clientId].outgoing <- stateMessageBuffer[0 : len(tanks) * 20 + 2]
    }
}

func broadcastBullets(bullets []Bullet, clients map[uint32]*Client) {
    var bulletsMessageBuffer = make([]byte, len(bullets) * 12 + 5)
    bulletsMessageBuffer[0] = MSG_OUT_BULLET_STATE
    binary.LittleEndian.PutUint32(bulletsMessageBuffer[1:], uint32(len(bullets)));
    message := bulletsMessageBuffer[5:]
    for _, bullet := range bullets {
        binary.LittleEndian.PutUint32(message[0:], bullet.id);
        binary.LittleEndian.PutUint32(message[4:], uint32(bullet.x))
        binary.LittleEndian.PutUint32(message[8:], uint32(bullet.y))
        message = message[12:]
    }

    for clientId, _ := range clients {
        clients[clientId].outgoing <- bulletsMessageBuffer[0 : len(bulletsMessageBuffer)]
    }
}

func broadcastDeath(id uint32, x float64, y float64, radius uint32,
                    clients map[uint32]*Client) {
    var buffer [17]byte
    buffer[0] = MSG_OUT_DEATH
    message := buffer[1:]
    binary.LittleEndian.PutUint32(message[0:], id);
    binary.LittleEndian.PutUint32(message[4:], uint32(x));
    binary.LittleEndian.PutUint32(message[8:], uint32(y));
    binary.LittleEndian.PutUint32(message[12:], radius);
    for clientId, _ := range clients {
        clients[clientId].outgoing <- buffer[0:]
    }
    //log.Printf("Object %d died bravely", id)
}

func explodeAt(cx int32,
               cy int32,
               r int32,
               mapBitmap []byte,
               tanks map[uint32]*Tank,
               owner *Client) {
    // Destroy terrain
    // Use signed int math to make it possible to write equivalent js.
    // TODO: dragons here
    sx := maxInt32(cx - r, 0)
    sy := maxInt32(cy - r, 0)
    ly := minInt32(cy + r, HEIGHT)
    lx := minInt32(cx + r, WIDTH)
    rs := r*r
    for y := sy; y < ly; y++ {
        for x := sx; x < lx; x++ {
            ds := (y-cy)*(y-cy) + (x-cx)*(x-cx);
            if ds < rs {
                mapBitmap[uint32(x) + uint32(y) * WIDTH] = 0;
            }
        }
    }
    // Hit tanks
    for tankID, tank := range tanks {
        dx := coordToPixel(tank.x) - cx
        dy := coordToPixel(tank.y) - cy
        ds := dx*dx + dy*dy
        dmg := EXPLOSION_DMG - EXPLOSION_DMG_FALLOFF * math.Sqrt(float64(ds))
        if dmg > 0.1 {
            realDmg := uint32(math.Min(dmg, float64(tank.hp)))
            tanks[tankID].hp -= realDmg
            if (tanks[tankID].hp <= 0 && tankID != owner.id) {
                owner.frags++
            }
            //log.Printf("%d[%d] hit by explosion: -%f", tankID, tank.hp, dmg)
        }
    }
}

func isPointInTank(cx int32, cy int32, tanks map[uint32]*Tank, ownerId uint32) bool {
    for clientId, tank := range tanks {
        x := int32(tank.x)
        y := int32(tank.y)
        if (clientId != ownerId &&
            cx > x - TANK_WIDTH / 2 && cx  < x + TANK_WIDTH /2 &&
            cy > y - TANK_HEIGHT  && cy <= y ) {
            return true
        }
    }
    return false
}

func isGroundInCircle(cx int32, cy int32, r int32, mapBitmap []byte) bool {
    sx := cx - r
    sy := cy - r
    ly := cy + r
    lx := cx + r
    rs := r*r
    for y := sy; y < ly; y++ {
        for x := sx; x < lx; x++ {
            if isGroundI(x, y, mapBitmap) {
                ds := (y-cy)*(y-cy) + (x-cx)*(x-cx)
                if (ds < rs) {
                    return true
                }
            }
        }
    }
    return false
}

func isGroundF(x float64, y float64, mapBitmap []byte) bool {
    mapX := coordToPixel(x)
    mapY := coordToPixel(y)
    return isGroundI(mapX, mapY, mapBitmap)
}

func isGroundI(x int32, y int32, mapBitmap []byte) bool {
    if x <= 0 || y >= HEIGHT || x >= WIDTH {
        return true
    }
    index := x + y * WIDTH
    return index < 0 || int(index) >= len(mapBitmap) || mapBitmap[index] == 1
}

func main() {
    flag.Parse()
    var addr = flag.String("addr", ":8080", "http service address")
    rand.Seed(time.Now().UTC().UnixNano())

    var net Network
    net.Init()

    go gameLoop(&net)
    for i := 0; i < NUM_BOTS; i++ {
        go createBot(&net)
    }

    err := runServer(&net, *addr)
    if err != nil {
        log.Fatal("Server: ", err)
    }
}

func maxInt32(a int32, b int32) int32 {
    if a > b {
        return a
    }
    return b
}

func minInt32(a int32, b int32) int32 {
    if a < b {
        return a
    }
    return b
}

func coordToPixel(x float64) int32 {
    return int32(x)
}
