package botai

import (
    "time"
    "encoding/binary"
    "math/rand"
    "fmt"
    "log"
)

type Bot struct {
    id uint32
    input chan []byte
    output chan []byte
    deletion chan bool
    direction byte
    changingAngle byte
}

var BOT_NAMES = []string{"Angel", "Xoxo", "Bass", "fish", "Bean", "acad", "Boo", "shelsp", "Bull", "racer :P", "caster", "Chieff", "legor", "Slayer89", "Crazy", "war72", "miate", "Delif", "ore007", "Diddy", "weatty", "Facepalm", "obama", "trump", "iwantyou", "wolfie", "lipslab", "madman", "артишок", "BunnyEater", "motowell", "agent006", "Mysioniz", "Netflow", "piercing", "autokran", "sampand", "rishar", "putin", "bobuk"}

func getRndName() string {
  return BOT_NAMES[rand.Intn(len(BOT_NAMES))]
}

func Start(input, output chan []byte, deletionChan chan bool) {
    bot := Bot {
        input: input,
        output: output,
        deletion: deletionChan,
        direction: 1,
        changingAngle: 0,
    }

    for {
        message := <-bot.input
        if (message[0] == 2) {
            bot.id = binary.LittleEndian.Uint32(message[1:])
            break
        }
    }

    nickName := []byte(fmt.Sprintf(" %s", getRndName()))
    bot.output <- nickName

    for {
        select {
        case message := <-bot.input:
            switch message[0] {
            case 4: // MSG_OUT_DEATH
                deadId := binary.LittleEndian.Uint32(message[1:])
                if (deadId == bot.id) {
                    bot.direction = 1
                    select {
                    case <- bot.deletion:
                        log.Println("Deleting myself as asked")
                        close(bot.output)
                        return
                    default:
                        log.Println("No deletion asked, rejoining")
                        bot.output <- nickName
                    }
                }
            }
        default:
            var msgData []byte
            rnd := rand.Intn(100)
            if rnd % 20 != 0 {
                msgData = make([]byte, 6)
                msgData[0] = 0 // MSG_IN_MOVING
                msgData[1] = bot.direction
                msgData[2] = bot.changingAngle
            } else {
                msgData = make([]byte, 10)
                msgData[0] = 1 //MSG_IN_SHOOTING
                msgData[1] = uint8(rand.Intn(5) + 5)
            }
            if rand.Intn(100) % 10 == 0 {
                direction := byte((rnd % 3) - 1)
                bot.direction = direction
            }
            if rand.Intn(100) % 8 == 0 {
                bot.changingAngle = byte((rnd % 3) -1)
            }
            bot.output <- msgData

            // Always try enabling shield!
            msgData = make([]byte, 1)
            msgData[0] = 2 //MSG_IN_SHIELD
            bot.output <- msgData
            time.Sleep(200 * time.Millisecond)
        }
    }
}
