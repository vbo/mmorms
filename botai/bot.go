package botai

import (
    "time"
    "encoding/binary"
    "math/rand"
    "math"
    "fmt"
    "log"
)

type Bot struct {
    id uint32
    input chan []byte
    output chan []byte
    deletion chan bool
    direction byte
    gunAngle int32
    gunAngleTarget int32
}

func Start(input, output chan []byte, deletionChan chan bool) {
    bot := Bot {
        input: input,
        output: output,
        deletion: deletionChan,
        direction: 1,
        gunAngle: 0,
        gunAngleTarget: 0,
    }

    for {
        message := <-bot.input
        if (message[0] == 2) {
            bot.id = binary.LittleEndian.Uint32(message[1:])
            break
        }
    }

    nickName := []byte(fmt.Sprintf(" Bot%d", bot.id))
    bot.output <- nickName

    for {
        select {
        case message := <-bot.input:
            switch message[0] {
            case 4: // MSG_OUT_DEATH
                deadId := binary.LittleEndian.Uint32(message[1:]) 
                if (deadId == bot.id) {
                    bot.gunAngleTarget = 0
                    bot.direction = 1
                    bot.gunAngle = 0
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
                msgData [1] = bot.direction
                gunAngleDiff := float64(bot.gunAngleTarget - bot.gunAngle)
                if (gunAngleDiff != 0) {
                    bot.gunAngle += int32(gunAngleDiff / math.Abs(gunAngleDiff))
                }
                binary.LittleEndian.PutUint32(msgData[2:], uint32(bot.gunAngle))
            } else {
                msgData = make([]byte, 10)
                msgData[0] = 1 //MSG_IN_SHOOTING
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
                bot.gunAngleTarget = int32(rand.Intn(180)) - 90
            }
            bot.output <- msgData
            time.Sleep(200 * time.Millisecond)
        }
    }
}
