package module

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/phyer/core"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	simple "github.com/bitly/go-simplejson"
	"github.com/go-redis/redis"
	// "github.com/phyer/core/utils"
	logrus "github.com/sirupsen/logrus"
)

func LoopSubscribe(cr *core.Core, channelName string, redisConf *core.RedisConfig) {
	redisRemoteCli := cr.RedisRemoteCli
	suffix := ""
	env := os.Getenv("GO_ENV")
	if strings.Contains(env, "demoEnv") {
		suffix = "-demoEnv"
	}
	fmt.Println("loopSubscribe: ", channelName+suffix)
	pubsub := redisRemoteCli.Subscribe(channelName + suffix)
	_, err := pubsub.Receive()
	if err != nil {
		// cr.ErrorToRobot(utils.GetFuncName(), err)
		fmt.Println(GetFuncName(), " ", err)
		panic(err)
	}

	// 用管道来接收消息
	ch := pubsub.Channel()
	// 处理消息
	for msg := range ch {
		if msg.Payload == "" {
			continue
		}
		ctype := ""
		if strings.Contains(channelName, "allCandle") {
			ctype = "candle"
		} else if strings.Contains(channelName, "allMaXs") {
			ctype = "maX"
		} else if strings.Contains(channelName, "ticker") {
			ctype = "tickerInfo"
		} else if strings.Contains(channelName, "ccyPositions") {
			ctype = "ccyPositions"
		} else if strings.Contains(channelName, "allseriesinfo") {
			ctype = "seriesinfo"
		} else if strings.Contains(channelName, "private|order") {
			ctype = "private|order"
		} else {
			logrus.Warning("channelname not match", channelName)
		}
		// logrus.Warning("msg.Payload: ", msg.Payload)
		// fmt.Println("channelName: ", channelName, " msg.Payload: ", msg.Payload)
		switch ctype {
		case "candle":
			{
				cd := core.Candle{}
				json.Unmarshal([]byte(msg.Payload), &cd)
				founded := cd.Filter(cr)
				if !founded {
					break
				}
				cr.CandlesProcessChan <- &cd
				break
			}
		case "maX":
			{
				mx := MaX{}
				if msg.Payload == "" {
					continue
				}
				json.Unmarshal([]byte(msg.Payload), &mx)
				dt := []interface{}{}
				dt = append(dt, mx.Ts)
				dt = append(dt, mx.AvgVal)
				mx.Data = dt
				cr.MaXProcessChan <- &mx
				break
			}
		case "tickerInfo":
			{
				//tickerInfo:  map[askPx:2.2164 askSz:17.109531 bidPx:2.2136 bidSz:73 high24h:2.497 instId:STX-USDT instType:SPOT last:2.2136 lastSz:0 low24h:2.0508 open24h:2.42 sodUtc0:2.4266 sodUtc8:2.4224 ts:1637077323552 vol24h:5355479.488179 :12247398.975501]
				ti := TickerInfo{}
				err := json.Unmarshal([]byte(msg.Payload), &ti)
				if err != nil {
					logrus.Warning("tickerInfo payload unmarshal err: ", err, msg.Payload)
				}
				cr.TickerInforocessChan <- &ti
				break
			}
		case "seriesInfo":
			{
				//tickerInfo:  map[askPx:2.2164 askSz:17.109531 bidPx:2.2136 bidSz:73 high24h:2.497 instId:STX-USDT instType:SPOT last:2.2136 lastSz:0 low24h:2.0508 open24h:2.42 sodUtc0:2.4266 sodUtc8:2.4224 ts:1637077323552 vol24h:5355479.488179 :12247398.975501]
				sei := SeriesInfo{}
				err := json.Unmarshal([]byte(msg.Payload), &sei)
				if err != nil {
					logrus.Warning("seriesInfo payload unmarshal err: ", err, msg.Payload)
				}
				cr.SeriesChan <- &sei
				break
			}
		}
	}
}
