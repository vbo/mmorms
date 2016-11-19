package main

import (
    "log"
    "time"
    "encoding/binary"
    "math/rand"
    "math"
)

const CHANNEL_SIZE = 8096

type Bot struct {
    id uint32
    input chan []byte
    output chan Message
    direction byte
    gunAngle int32
    gunAngleTarget int32
    client *Client
}

func createBot(net *Network) {
    client := &Client{
        id: net.GetNewObjectId(),
        outgoing: make(chan []byte, MESSAGE_QUEUE_SIZE),
        observer: false,
    }
    bot := &Bot {
        input: client.outgoing,
        output: net.incoming,
        client: client,
        direction: 1,
        gunAngle: 0,
        gunAngleTarget: 0,
    }
    net.connect <- client
    log.Println("Bot client connecting...")
    updateBot(bot)
}

func updateBot(bot *Bot) {
    for {
        message := <-bot.input
        if (message[0] == 2) {
            bot.id = binary.LittleEndian.Uint32(message[1:])
            break
        }
    }
    for {
        select {
        case message := <-bot.input:
            switch message[0] {
            case MSG_OUT_DEATH:
                deadId := binary.LittleEndian.Uint32(message[1:]) 
                if (deadId == bot.id) {
                    nickName := []byte("Bot")
                    msgData := make([]byte, len(nickName) + 1)
                    msgData[0] = 32
                    copy(msgData[1:], nickName)
                    bot.gunAngleTarget = 0
                    bot.direction = 1
                    bot.gunAngle = 0
                    msg := Message {
                        from: bot.client.id,
                        data: msgData,
                    }
                    bot.output <- msg
                }
            }
        default:
            var msgData []byte
            rnd := rand.Intn(100)
            if rnd % 20 != 0 {
                msgData = make([]byte, 6)
                msgData[0] = MSG_IN_MOVING
                msgData [1] = bot.direction
                gunAngleDiff := float64(bot.gunAngleTarget - bot.gunAngle)
                if (gunAngleDiff != 0) {
                    bot.gunAngle += int32(gunAngleDiff / math.Abs(gunAngleDiff))
                }
                binary.LittleEndian.PutUint32(msgData[2:], uint32(bot.gunAngle))
            } else {
                msgData = make([]byte, 10)
                msgData[0] = MSG_IN_SHOOTING
                x := 128 * math.Cos(float64(bot.gunAngle) * math.Pi / 180)
                y := 128 * math.Sin(float64(bot.gunAngle) * math.Pi / 180)
                binary.LittleEndian.PutUint32(msgData[1:], uint32(x))
                binary.LittleEndian.PutUint32(msgData[5:], uint32(y))
                msgData[9] = uint8(rand.Intn(5) + 5)
            }
            if rand.Intn(100) % 10 == 0 {
                direction := byte((rnd % 3) - 1)
                if direction != bot.direction && direction != 0 {
                    bot.gunAngle = 180 - bot.gunAngle
                    bot.gunAngleTarget = 180 - bot.gunAngleTarget
                }
                bot.direction = direction
            }
            if rand.Intn(100) % 8 == 0 {
                bot.gunAngleTarget= int32(rand.Intn(180)) - 90
            }
            msg := Message {
                from: bot.client.id,
                data: msgData,
            }
            bot.output <- msg
            time.Sleep(200 * time.Millisecond)
        }
    }
}
