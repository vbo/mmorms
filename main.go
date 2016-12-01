package main

import (
  "os"
  "flag"
  "log"
  "encoding/binary"
  "math/rand"
  "math"
  "time"
  "runtime"
  "fmt"
  "image"

  "golang.org/x/image/bmp"
)

type Tank struct {
    x, y float64
    vx, vy float64
    moving int8
    jumping bool
    gunAngle int32
    hp uint32
    shield bool
    lastShotTime time.Time
    shieldFlipTime time.Time
}

type Bullet struct {
    id uint32
    ownerId uint32
    x, y float64
    vx, vy float64
}

const EXPLOSION_DMG = 70
const EXPLOSION_DMG_FALLOFF = 2

const BULLET_EXPLOSION_RAD = 30.0
const BULLET_RAD = 5
const BULLET_SPEED = 20

const GRAV_ACC = 30
const GRAV_SPEED = 100

const TANK_RAD = 0
const TANK_SPEED = 20
const TANK_AIR_ACC = 4
const TANK_TOWER_HEIGHT = 25
const TANK_GUN_LENGTH = 30
const TANK_SHOT_DELAY = 500 * time.Millisecond
const TANK_WIDTH = 50
const TANK_HEIGHT = 20
const TANK_SHIELD_DURATION = 2000 * time.Millisecond
const TANK_SHIELD_COOLDOWN = 5000 * time.Millisecond

const DESTROYED_FRACTION_TO_SPACE = 0.15

const NEWMAP_TIMEOUT = 30000 * time.Millisecond
const SPACE_DURATION = 1000 * time.Millisecond
const POPULATION_CHECK_TIMEOUT = 60000 * time.Millisecond

const WIDTH = 1260
const HEIGHT = 620

const (
    MSG_OUT_MAP = 0
    MSG_OUT_STATE = 1
    MSG_OUT_GREETING = 2
    MSG_OUT_BULLET_STATE = 3
    MSG_OUT_DEATH = 4
    MSG_OUT_LEADERBOARD = 5
    MSG_OUT_MAP_CHANGE = 6
    MSG_OUT_PING = 80
)

const (
    MSG_IN_MOVING = 0
    MSG_IN_SHOOTING = 1
    MSG_IN_SHIELD = 2
    MSG_IN_JUMP = 3
    MSG_IN_START = 32
)

const MIN_NUM_PLAYERS = 5

const TARGET_TICK_TIME = 50 * time.Millisecond

var MAP_FILES = []string{
    "./public/islands_1.bmp",
    "./public/islands_2.bmp",
    "./public/platforms_1.bmp",
    "./public/edges_1.bmp",
    "./public/2bases.bmp",
    "./public/slope_obstacles.bmp",
    "./public/slope_1.bmp",
    "./public/slopes_2.bmp",
}


var MAPS_LOADED = make([]image.Image, len(MAP_FILES))

func NewTank() *Tank {
    return &Tank{
        x: float64(1 + rand.Intn(WIDTH - 1)),
        y: 20,
        hp: 100,
    }
}

func generateMapBitmap(mapBitmap []byte) {
    mapi := rand.Intn(len(MAPS_LOADED)) 
    log.Println("Loading map", MAP_FILES[mapi])
    img := MAPS_LOADED[mapi]
    p := 0
    for y := 0; y < HEIGHT; y++ {
        for x := 0; x < WIDTH; x++ {
            r, _, _, _ := img.At(x, y).RGBA()
            if r == 0 {
                mapBitmap[p] = 1
            } else {
                mapBitmap[p] = 0
            }
            p++
        }
    }

    /*
    for x := 0; x < WIDTH; x++ {
        groundCurve := 200 + (float64(x) / 400.0 * math.Sin(float64(x) / 130.0) + 1.2) * 100
        for y := 0; y < HEIGHT; y++ {
            if (float64(y) < groundCurve) {
                mapBitmap[x + y * WIDTH] = 0
            } else {
                mapBitmap[x + y * WIDTH] = 1
            }
        }
    }
    */
}

func simulateWorldInSpace(tanks map[uint32]*Tank,
                          newTanksPos map[uint32]float64,
                          dtTotalSeconds float64,
                          frameStartTime time.Time,
                          spaceStartTime time.Time) int {
    targetTime := spaceStartTime.Add(SPACE_DURATION)
    dt := targetTime.Sub(frameStartTime)
    numFinished :=  0
    for id, tank := range tanks {
        ds := newTanksPos[id] - tank.y
        if dt <= 0 {
            tank.y = newTanksPos[id]
            numFinished++
        } else {
            v := ds / dt.Seconds()
            if math.Abs(ds) > 1 {
                tank.y += dtTotalSeconds * v
            } else {
                numFinished++
            }
        }
    }
    return numFinished
}

func simulateWorld(
    tanks map[uint32]*Tank,
    bulletsIn *[]Bullet,
    mapBitmap []byte,
    dtTotalSeconds float64,
    startTime time.Time,
    clients map[uint32]*Client,
    numGroundDestroyed *int) {

    bullets := *bulletsIn
    for dtTotalSeconds > 0.0 {
        dt := math.Min(0.005, dtTotalSeconds)
        dtTotalSeconds = dtTotalSeconds - dt
        for _, tank := range tanks {
            oldX := tank.x
            oldY := tank.y
            // TODO(vbo): use separate point for ground collision,
            // otherwise the logical center of the tank is too low
            // and it feels weird.
            wasOnGround := isGroundF(tank.x, tank.y + 1.0, mapBitmap)
            if !wasOnGround || tank.vy < -2 {
                tank.vy += GRAV_ACC * dt
                if tank.jumping {
                    tank.vx += float64(tank.moving) * TANK_AIR_ACC * dt
                }
                tank.x += tank.vx * dt
                if (isGroundF(tank.x, tank.y, mapBitmap)) {
                    tank.x = oldX
                    tank.vx = 0
                }
                tank.y += tank.vy * dt
                if (isGroundF(tank.x, tank.y, mapBitmap)) {
                    tank.y = oldY
                    tank.vy = 0
                    tank.jumping = false
                }
            } else {
                tank.vy = 0
                tank.vx = float64(tank.moving) * TANK_SPEED
                tank.x += tank.vx * dt
                // hill sliding mechanics:
                if isGroundF(tank.x, oldY, mapBitmap) {
                    for i := 1; i <= 2; i++ {
                        if !isGroundF(tank.x, oldY - float64(i), mapBitmap) {
                            tank.y = oldY - float64(i)
                            break
                        }
                    }
                    if tank.y == oldY {
                        tank.x = oldX
                    } else {
                        tank.vx = 0
                    }
                } else {
                    for i := 1; i <= 2; i++ {
                        if isGroundF(tank.x, oldY + float64(i), mapBitmap) {
                            break
                        } else {
                            tank.y = oldY + float64(i)
                        }
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
                required := uint32(1)
                frags := uint32(0)
                if clients[bullet.ownerId] != nil {
                    frags = clients[bullet.ownerId].lifeFrags
                }
                stars := uint32(0)
                for frags > required {
                    frags -= required
                    stars++
                    required *= 2
                }
                radius := int32(BULLET_EXPLOSION_RAD * math.Sqrt(float64(stars)/2.0 + 1.0))
                broadcastDeath(bullet.id, bullet.x, bullet.y, radius, clients)
                explodeAt(
                    coordToPixel(bullet.x),
                    coordToPixel(bullet.y),
                    radius,
                    mapBitmap,
                    tanks,
                    clients,
                    clients[bullet.ownerId],
                    numGroundDestroyed)
                //log.Printf("Bullet crashed at %v, %v", bullet.x, bullet.y)
                bullets[bulletIndex] = bullets[len(bullets) - 1]
                bullets = bullets[:len(bullets) - 1]
            } else {
                bulletIndex++
            }
        }
        for tankID, tank := range tanks {
            // Explode dead tanks.
            if tank.hp == 0 {
                broadcastDeath(tankID, tank.x, tank.y, 0, clients)
                delete(tanks, tankID)
            }
            // Drop shields
            if tank.shield && startTime.Sub(tank.shieldFlipTime) > TANK_SHIELD_DURATION {
                tank.shield = false
                tank.shieldFlipTime = startTime
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

    lastPopulationCheck := time.Time{}
    botDeletionChan := make(chan bool, 2)

    // map transfer variables
    numGroundDestroyed := 0
    newMapChannel := make(chan []byte)
    startOfSpaceMode := make(chan bool, 2)
    var newMapBitmap []byte
    var newMapBitmapArchive []byte
    var spaceStartTime time.Time
    spaceMode := false
    var newTanksPos map[uint32]float64

    // TODO(vbo): load from predesigned file?
    // TODO(vbo): we need precomputed surface normals for:
    //  - grenades bouncing
    //  - tank slope orientation
    generateMapBitmap(mapBitmap)

    startTime := time.Now()
    dtTotal := time.Duration(0)
    curTick := uint64(0)
    lastDtTotal := time.Duration(0)
    maxDtTotal := time.Duration(0)
    maxTimeBeforeSleep := time.Duration(0)
    var memStats, prevMemStats runtime.MemStats

    for {
        // adding bots
        if len(tanks) < MIN_NUM_PLAYERS {
            if lastPopulationCheck == (time.Time{}) {
                lastPopulationCheck = time.Now()
            } else {
                if time.Now().Sub(lastPopulationCheck) > 1000 * time.Millisecond {
                    go createBot(net, botDeletionChan)
                    lastPopulationCheck = time.Time{}
                }
            }
        } else {
            lastPopulationCheck = time.Time{}
            if len(tanks) > MIN_NUM_PLAYERS && len(botDeletionChan) == 0 {
                log.Println("Deletion asked")
                botDeletionChan <-true
            }
        }

        // writing stats
        if curTick % 1000 == 0 {
            runtime.ReadMemStats(&memStats)
            var numGCDiff = memStats.NumGC - prevMemStats.NumGC
            var numMallocs =  memStats.Mallocs - prevMemStats.Mallocs
            var maxPauseNs uint64 = 0
            for i := memStats.NumGC; i > prevMemStats.NumGC; i-- {
                pause := memStats.PauseNs[(i+255)%256] // circular buffer
                if pause > maxPauseNs { maxPauseNs = pause }
            }
            log.Printf("STAT:tick=%d,dt=%s(%s max),tbs=%s,gocnt=%d,alloc=(%d mlcs,%d ngc,%f max,%d kb live)",
                curTick, lastDtTotal, maxDtTotal, maxTimeBeforeSleep, runtime.NumGoroutine(),
                numMallocs, numGCDiff, float64(maxPauseNs)/1000000, memStats.Alloc / 1024)
            //log.Println(memStats)
            maxDtTotal = 0.0
            prevMemStats = memStats
        }

        // Make a GC-ed copy of the mapBitmap to respond to connects
        // TODO: can we allocate the space just once and reuse it?
        mapBitmapBufferCopy := make([]byte, len(mapBitmapBuffer))
        mapBitmapBufferCopy[0] = mapBitmapBuffer[0]
        mapArchiveSize := archiveMap(mapBitmapBuffer[1:], mapBitmapBufferCopy[1:])
        mapBitmapBufferCopy = mapBitmapBufferCopy[0:mapArchiveSize + 1]

        // Handle incoming messages
        select {
        case client := <-net.connect:
            clients[client.id] = client
            var greetingMsg [5]byte
            greetingMsg[0] = MSG_OUT_GREETING
            binary.LittleEndian.PutUint32(greetingMsg[1:], client.id)
            //log.Printf("Client %d connected.", client.id)
            client.outgoing <- greetingMsg[0:]
            client.outgoing <- mapBitmapBufferCopy
            if newMapBitmap != nil {
                client.outgoing <- newMapBitmapArchive
            }
            //log.Println("Map & greetings sent")
            if !client.observer {
                tanks[client.id] = NewTank()
            }

        case client := <-net.disconnect:
            tank, ok := tanks[client.id]
            if ok {
                broadcastDeath(client.id, tank.x, tank.y, 0, clients)
                delete(tanks, client.id)
            }
            delete(clients, client.id)
            close(client.outgoing)
            log.Printf("Client %d disconnected.", client.id)

        case statsRequest := <-net.statsRequests:
            for _, client := range clients {
                fmt.Fprintf(statsRequest.w,
                            "%d\t%f\t\n",
                            client.id,
                            client.ping)
            }
            statsRequest.done <- true

        case newMapBitmap = <-newMapChannel:
            newMapBitmapArchive = make([]byte, len(newMapBitmap))
            newMapBitmapArchive[0] = newMapBitmap[0]
            mapArchiveSize := archiveMap(newMapBitmap[1:], newMapBitmapArchive[1:])
            newMapBitmapArchive = newMapBitmapArchive[0:mapArchiveSize + 1]
            for _,client := range clients {
                client.outgoing <- newMapBitmapArchive
            }
            go func() {
                time.Sleep(NEWMAP_TIMEOUT)
                startOfSpaceMode <-true
            }()

        case _ = <-startOfSpaceMode:
            log.Println("Entering the space mode...")
            mapChangeMsg := make([]byte, 2)
            mapChangeMsg[0] = MSG_OUT_MAP_CHANGE
            mapChangeMsg[1] = 1
            for _,client := range clients {
                client.outgoing <- mapChangeMsg
            }
            spaceMode = true
            spaceStartTime = startTime
            bullets = bullets[0:0]

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
                    case MSG_IN_MOVING:
                        _, ok := tanks[client.id]
                        if ok {
                            tanks[client.id].moving = int8(message.data[1])
                            tanks[client.id].gunAngle =
                                int32(binary.LittleEndian.Uint32(message.data[2:]))
                        }

                    case MSG_IN_SHOOTING:
                        tank, ok := tanks[client.id]
                        if ok && !spaceMode && startTime.Sub(tank.lastShotTime) > TANK_SHOT_DELAY {
                            tank.lastShotTime = startTime
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
                            vx := BULLET_SPEED * aimX * power + 0.7 * tank.vx
                            vy := BULLET_SPEED * aimY * power + 0.7 * tank.vy
                            id := net.GetNewObjectId()
                            x := tank.x + TANK_GUN_LENGTH * aimX
                            y := tank.y - TANK_TOWER_HEIGHT * (1 - aimY)
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

                    case MSG_IN_SHIELD:
                        tank, ok := tanks[client.id]
                        if ok && !spaceMode && !tank.shield && startTime.Sub(tank.shieldFlipTime) > TANK_SHIELD_COOLDOWN {
                            tank.shield = true
                            tank.shieldFlipTime = startTime
                        }

                    case MSG_IN_JUMP:
                        tank, ok := tanks[client.id]
                        if ok && isGroundF(tank.x, tank.y + 1, mapBitmap) {
                            tank.vy = -40
                            tank.jumping = true
                        }

                    case MSG_IN_START:
                        _, ok := tanks[client.id]
                        if !ok {
                            client.name = message.data[1:]
                            if (len(client.name) == 0) {
                                client.name = []byte(fmt.Sprintf("Tank%d", client.id))
                            }
                            log.Printf("%s joined", string(client.name))
                            client.lifeFrags = 0
                            tanks[client.id] = NewTank()
                            if spaceMode {
                                newTanksPos = getNewTanksPos(tanks, newMapBitmap[1:])
                            }
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
        if !spaceMode {
            simulateWorld(tanks,
                &bullets,
                mapBitmap,
                dtTotalSeconds,
                startTime,
                /* TODO(vbo): remove */ clients,
                &numGroundDestroyed)
        } else {
            if (newTanksPos == nil) {
                log.Println("New tank pos calc start")
                newTanksPos = getNewTanksPos(tanks, newMapBitmap[1:])
                log.Println("New tank pos calc done")
            }
            finished := simulateWorldInSpace(
                tanks,
                newTanksPos,
                dtTotalSeconds,
                startTime,
                spaceStartTime)
            if finished == len(tanks) {
                newTanksPos = nil
                spaceMode = false
                copy(mapBitmapBuffer[1:], newMapBitmap[1:])
                newMapBitmap = nil
                newMapBitmapArchive = nil
                numGroundDestroyed = 0
                log.Println("Trasfer finished")

                mapChangeMsg := make([]byte, 2)
                mapChangeMsg[0] = MSG_OUT_MAP_CHANGE
                mapChangeMsg[1] = 0
                for _,client := range(clients) {
                    client.outgoing <- mapChangeMsg
                }
            }
        }

        // Broadcast world snapshot
        broadcastTanks(tanks, clients, startTime)
        broadcastBullets(bullets, clients)
        broadcastLeaderboard(clients)

        if newMapBitmap == nil && numGroundDestroyed > WIDTH * HEIGHT * DESTROYED_FRACTION_TO_SPACE {
            log.Println("Time to restart...")
            numGroundDestroyed = 0
            go generateMapBitmapAsync(newMapChannel)
        }

        sleepStart := time.Now()
        timeBeforeSleep :=  sleepStart.Sub(startTime)
        timeToSleep := TARGET_TICK_TIME - timeBeforeSleep
        time.Sleep(timeToSleep)

        newTime := time.Now()
        dtTotal = newTime.Sub(startTime)
        startTime = newTime
        curTick++

        // Bookkeeping
        lastDtTotal = dtTotal
        if lastDtTotal > maxDtTotal { maxDtTotal = lastDtTotal }

        if timeBeforeSleep > maxTimeBeforeSleep { maxTimeBeforeSleep = timeBeforeSleep }
    }
}

func getNewTanksPos(tanks map[uint32]*Tank, newMap []byte) map[uint32]float64 {
    result := make(map[uint32]float64)
    for id, tank := range(tanks) {
        y := tank.y
        if (isGroundF(tank.x, y, newMap)) {
            // Gettin up from the ground
            for {
                if isGroundF(tank.x, y, newMap) {
                    y -= 1
                } else {
                    result[id] = y
                    break
                }
            }
        } else {
            // Falling down
            for {
                if !isGroundF(tank.x, y + 1, newMap) {
                    y += 1
                } else {
                    result[id] = y
                    break
                }
            }
        }
    }
    return result
}

func archiveMap(mapBitmap []byte, result []byte) int {
    counter := uint32(1)
    value := mapBitmap[0]
    k := 0
    for i := 1; i < len(mapBitmap); i++ {
        if mapBitmap[i] != value {
            result[k] = value
            binary.LittleEndian.PutUint32(result[k+1:], counter)
            k += 5
            counter = 1
            value = mapBitmap[i]
        } else {
            counter++
        }
    }
    result[k] = value
    binary.LittleEndian.PutUint32(result[k+1:], counter)
    k += 5
    return k
}

func generateMapBitmapAsync(result chan []byte) {
    mapBitmapBuffer := make([]byte, WIDTH * HEIGHT + 1)
    mapBitmapBuffer[0] = MSG_OUT_MAP
    generateMapBitmap(mapBitmapBuffer[1:])
    result <- mapBitmapBuffer
}

func broadcastLeaderboard(clients map[uint32]*Client) {
    namesLen := 0
    namedClientsCount := 0
    for _, client := range clients {
        nameLen := len(client.name)
        if nameLen > 0 {
            namesLen += nameLen
            namedClientsCount++
        }
    }
    messageLen := 5 + (13 * namedClientsCount) + namesLen
    messageBuffer := make([]byte, messageLen)
    messageBuffer[0] = MSG_OUT_LEADERBOARD
    binary.LittleEndian.PutUint32(messageBuffer[1:], uint32(namedClientsCount))
    message := messageBuffer[5:]
    for _, client := range clients {
        nameLen := byte(len(client.name))
        if nameLen == 0 { continue }
        binary.LittleEndian.PutUint32(message[0:], client.id)
        binary.LittleEndian.PutUint32(message[4:], uint32(client.sessionFrags))
        binary.LittleEndian.PutUint32(message[8:], uint32(client.lifeFrags))
        message[12] = nameLen
        copy(message[13:], client.name)
        message = message[13 + nameLen:]
    }
    for _, client := range clients {
        client.outgoing <- messageBuffer[0 : messageLen]
    }
}

func broadcastTanks(tanks map[uint32]*Tank, clients map[uint32]*Client, startTime time.Time) {
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
    stateMessageBuffer := make([]byte, 2 + len(tanks) * 24)
    stateMessageBuffer[0] = MSG_OUT_STATE
    stateMessageBuffer[1] = byte(len(tanks))
    message := stateMessageBuffer[2:]
    for tankId, tank := range tanks {
        binary.LittleEndian.PutUint32(message[0:], tankId)
        binary.LittleEndian.PutUint32(message[4:], uint32(tank.x))
        binary.LittleEndian.PutUint32(message[8:], uint32(tank.y))
        binary.LittleEndian.PutUint32(message[12:], uint32(tank.hp))
        binary.LittleEndian.PutUint32(message[16:], uint32(tank.gunAngle))
        var shieldInfo uint32
        sinceFlip := startTime.Sub(tank.shieldFlipTime)
        if tank.shield {
            percent := math.Min(sinceFlip.Seconds()/TANK_SHIELD_DURATION.Seconds(), 1)
            shieldInfo = 0xFF000000 + uint32(percent*255)
        } else {
            percent := math.Min(sinceFlip.Seconds()/TANK_SHIELD_COOLDOWN.Seconds(), 1)
            shieldInfo = 0x00000000 + uint32(percent*255)
        }
        binary.LittleEndian.PutUint32(message[20:], shieldInfo)
        message = message[24:]
    }
    for _, client := range clients {
        client.outgoing <- stateMessageBuffer[0 : len(stateMessageBuffer)]
    }
}

func broadcastBullets(bullets []Bullet, clients map[uint32]*Client) {
    var bulletsMessageBuffer = make([]byte, len(bullets) * 12 + 5)
    bulletsMessageBuffer[0] = MSG_OUT_BULLET_STATE
    binary.LittleEndian.PutUint32(bulletsMessageBuffer[1:], uint32(len(bullets)))
    message := bulletsMessageBuffer[5:]
    for _, bullet := range bullets {
        binary.LittleEndian.PutUint32(message[0:], bullet.id)
        binary.LittleEndian.PutUint32(message[4:], uint32(bullet.x))
        binary.LittleEndian.PutUint32(message[8:], uint32(bullet.y))
        message = message[12:]
    }
    for clientId, _ := range clients {
        clients[clientId].outgoing <- bulletsMessageBuffer[0 : len(bulletsMessageBuffer)]
    }
}

func broadcastDeath(id uint32, x float64, y float64, radius int32,
                    clients map[uint32]*Client) {
    var buffer [17]byte
    buffer[0] = MSG_OUT_DEATH
    message := buffer[1:]
    binary.LittleEndian.PutUint32(message[0:], id)
    binary.LittleEndian.PutUint32(message[4:], uint32(x))
    binary.LittleEndian.PutUint32(message[8:], uint32(y))
    binary.LittleEndian.PutUint32(message[12:], uint32(radius))
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
               clients map[uint32]*Client,
               owner *Client,
               numGroundDestroyed *int) {
    // Destroy terrain
    // Use signed int math to make it possible to write equivalent js.
    sx := maxInt32(cx - r, 0)
    sy := maxInt32(cy - r, 0)
    ly := minInt32(cy + r, HEIGHT)
    lx := minInt32(cx + r, WIDTH)
    rs := r*r
    for y := sy; y < ly; y++ {
        for x := sx; x < lx; x++ {
            ds := (y-cy)*(y-cy) + (x-cx)*(x-cx)
            if ds < rs {
                index := uint32(x) + uint32(y) * WIDTH;
                if isGroundI(x, y, mapBitmap) {
                    *numGroundDestroyed++
                }
                mapBitmap[index] = 0
            }
        }
    }
    // Hit tanks
    for tankID, tank := range tanks {
        dx := coordToPixel(tank.x) - cx
        dy := coordToPixel(tank.y) - cy
        ds := dx*dx + dy*dy
        dmg := EXPLOSION_DMG + float64(r) - EXPLOSION_DMG_FALLOFF * math.Sqrt(float64(ds))
        if dmg > 0.1 {
            dxa := math.Abs(float64(dx))
            dya := math.Abs(float64(dy))
            dxs := 1.0
            if dx != 0 { dxs = dxa / float64(dx) }
            dys := 1.0
            if dy != 0 { dys = dya / float64(dy) }
            if dxa < 1 { dxa = 1 }
            if dya < 1 { dya = 1 }
            dvx := dxs * float64(r) / math.Pow(dxa, 0.25)
            dvy := dys * float64(r) / math.Pow(dya, 0.25)
            tank.vx += dvx
            tank.vy -= dvy
            if tank.shield { continue }
            realDmg := uint32(math.Min(dmg, float64(tank.hp)))
            tank.hp -= realDmg
            if owner != nil && tanks[tankID].hp <= 0 && tankID != owner.id {
                //owner.name = append(owner.name, []byte(string('★')))
                owner.lifeFrags += 1
                if clients[tankID] != nil {
                    owner.lifeFrags += clients[tankID].lifeFrags
                }
                owner.sessionFrags++
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

func loadMap(path string) image.Image {
    file, err := os.Open(path)
    if err != nil { panic(err) }
    
    img, err := bmp.Decode(file)
    if err != nil { panic(err) }

    bounds := img.Bounds()
    if bounds.Max.X != WIDTH || bounds.Max.Y != HEIGHT {
        panic(fmt.Sprintf("invalid map file: %s"))
    }
    
    return img
}

func main() {
    var addr = flag.String("addr", ":8080", "http service address")
    flag.Parse()
    rand.Seed(time.Now().UTC().UnixNano())

    var net Network
    net.Init()

    for i, mapfile := range MAP_FILES {
        MAPS_LOADED[i] = loadMap(mapfile)
    }
    go gameLoop(&net)

    serverErr := runServer(&net, *addr)
    if serverErr != nil {
        log.Fatal("Server: ", serverErr)
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
