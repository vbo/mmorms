package main

import (
    "log"
    "time"
    "encoding/binary"
    "math/rand"
)

const CHANNEL_SIZE = 8096

type Bot struct {
    input chan []byte
    output chan Message
    strategy byte
    client *Client
}

func createBot(net *Network) {
    client := &Client{
        id: net.GetNewObjectId(),
        outgoing: make(chan []byte, MESSAGE_QUEUE_SIZE),
    }
    bot := &Bot {
        input: client.outgoing,
        output: net.incoming,
        client: client,
        strategy: byte(rand.Intn(255)),
    }
    net.connect <- client
    log.Println("Bot client connecting...")
    updateBot(bot)
}

func updateBot(bot *Bot) {
    for {
        select {
        case <-bot.input:
        default:
            var msgData []byte
            if rand.Intn(100) % 10 != 0 {
                msgData = make([]byte, 2)
                msgData[0] = 0
                msgData [1] = (bot.strategy % 3) - 1 
            } else {
                msgData = make([]byte, 9)
                msgData[0] = 1
                binary.LittleEndian.PutUint32(msgData[1:], uint32(rand.Intn(255) - 255))
                binary.LittleEndian.PutUint32(msgData[5:], uint32(rand.Intn(255) - 255))
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
