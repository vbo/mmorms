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
    strategy byte
    gunAngle int32
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
        strategy: byte(rand.Intn(255)),
        gunAngle: 0,
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
            log.Printf("I know my id, it's %d", bot.id)
            break
        }
    }
    for {
        select {
        case message := <-bot.input:
            switch message[0] {
            case 4:
                deadId := binary.LittleEndian.Uint32(message[1:]) 
                if (deadId == bot.id) {
                    log.Println("Goodbye cruel world")
                    nickName := []byte("Bot")
                    msgData := make([]byte, len(nickName) + 1)
                    msgData[0] = 32
                    copy(msgData[1:], nickName)
                    msg := Message {
                        from: bot.client.id,
                        data: msgData,
                    }
                    bot.output <- msg
                }
            }
        default:
            var msgData []byte
            if rand.Intn(100) % 10 != 0 {
                msgData = make([]byte, 6)
                msgData[0] = 0
                msgData [1] = (bot.strategy % 3) - 1
                bot.gunAngle += int32(rand.Intn(11) - 5)
                binary.LittleEndian.PutUint32(msgData[2:], uint32(bot.gunAngle))
            } else {
                msgData = make([]byte, 10)
                msgData[0] = 1
                x := 128 * math.Cos(float64(bot.gunAngle) * math.Pi / 180)
                y := 128 * math.Sin(float64(bot.gunAngle) * math.Pi / 180)
                log.Printf("%d angle, %f,%f", bot.gunAngle, x, y)
                binary.LittleEndian.PutUint32(msgData[1:], uint32(x))
                binary.LittleEndian.PutUint32(msgData[5:], uint32(y))
                msgData[9] = uint8(rand.Intn(5) + 5)
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
