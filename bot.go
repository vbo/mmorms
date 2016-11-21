package main

import (
    "log"

    "mmorms/botai"
)

const CHANNEL_SIZE = 8096

func createBot(net *Network) {
    client := &Client{
        id: net.GetNewObjectId(),
        outgoing: make(chan []byte, MESSAGE_QUEUE_SIZE),
        observer: true,
    }
    net.connect <- client
    log.Println("Bot client connecting...")
    input := client.outgoing
    output := translateOutputChannel(client.id, net.incoming)
    botai.Start(input, output)
}

func translateOutputChannel(id uint32, c chan Message) chan []byte {
    botch := make(chan []byte, MESSAGE_QUEUE_SIZE)
    go func () {
        for {
            m := <-botch
            c <- Message{from: id, data: m}
        }
    }()
    return botch
}
