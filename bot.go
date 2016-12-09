package main

import (
    "log"

    "mmorms/botai"
)

const CHANNEL_SIZE = 8096

func createBot(net *Network, deletionChan chan bool) {
    client := &Client{
        id: net.GetNewObjectId(),
        outgoing: make(chan []byte, MESSAGE_QUEUE_SIZE),
        observer: true,
        isBot: true,
    }
    net.connect <- client
    log.Println("Bot client connecting...")
    input := client.outgoing
    output := translateOutputChannel(client, net.incoming, net.disconnect)
    botai.Start(input, output, deletionChan)
}

func translateOutputChannel(client *Client, c chan Message, disconnect chan *Client) chan []byte {
    botch := make(chan []byte, MESSAGE_QUEUE_SIZE)
    go func () {
        for {
            m, ok := <-botch
            if !ok {
                disconnect <-client
                return
            }
            c <- Message{from: client.id, data: m}
        }
    }()
    return botch
}
