package main

import "log"
import "flag"
import "time"
import "github.com/gorilla/websocket"
import "mmorms/botai"

var addr = flag.String("addr", "ws://localhost:8080/ws", "websocket service address")
var cnt = flag.Int("cnt", 1, "number of bots to create")

func main() {
    flag.Parse()

    for i := 0; i < *cnt; i++ {
        input, output := createConnection(*addr)
        go botai.Start(input, output)
    }

    for {
        time.Sleep(10 * time.Second)
    }
}

func createConnection(addr string) (input, output chan[]byte) {
    input = make(chan []byte, 64)
    output = make(chan []byte, 64)

    conn, _, err := websocket.DefaultDialer.Dial(addr, nil)
    if err != nil {
		log.Fatal("dial:", err)
	}

    go func() {
		defer conn.Close()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
            input <- message
		}
	}()
    
    go func() {
        defer conn.Close()
        for {
            message := <-output
            err := conn.WriteMessage(websocket.BinaryMessage, message)
            if err != nil {
                log.Println("write:", err)
                return
            }
        }
    }()

    return
}
