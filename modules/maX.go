package module

import (
	// "encoding/json"
	// "errors"
	"fmt"
	"github.com/phyer/core"
	"os"
	"strconv"
	//"strings"
	// "sync"
	//"time"
	//
	// simple "github.com/bitly/go-simplejson"
	// "github.com/go-redis/redis"
	// "github.com/phyer/core/utils"
	//logrus "github.com/sirupsen/logrus"
)

type MyMaX struct {
	core.MaX
}

func (mmx *MyMaX) Process(cr *core.Core) {
	mx := mmx.MaX
	_, err := mx.SetToKey(cr)
	if err != nil {
		fmt.Println("max SetToKey err: ", err)
		return
	}
	// TODO
	go func() {
		torqueSorted := os.Getenv("SARDINE_MAKESERIES") == "true"
		if !torqueSorted {
			return
		}
		sm, err := mmx.InsertIntoPlate(cr)
		if err != nil {
			// fmt.Println("InsertIntoPlate err: ", err)
		} else {
			// 只有在ma30计算完成，触发coasterChan相关动作
			if mx.Count == 30 {
				ci := core.CoasterInfo{
					InstID:      mx.InstID,
					Period:      mx.Period,
					InsertedNew: sm != nil,
				}
				cr.CoasterChan <- &ci
			} else {
				// fmt.Println("maX: candle记录尚不足30条，无需触发coaster计算:", mx)
				// TODO
				// bj, _ := json.Marshal(mx)
				// fmt.Println("mx:", string(bj))
				// 这个地方可以加个逻辑，给tunas端发消息，缺啥补啥
			}
		}
	}()

	// 发送给下级消息队列订阅者, 关掉了，暂时用不上，2024-12-16
	// cr.AddToGeneralMaXChnl(mx)
}

func (mmx *MyMaX) InsertIntoPlate(cr *core.Core) (*core.Sample, error) {
	mx := mmx.MaX
	cr.Mu.Lock()
	defer cr.Mu.Unlock()
	pl, ok := cr.PlateMap[mx.InstID]
	// 尝试放弃一级缓存
	// if !ok {
	pl, _ = LoadPlate(cr, mx.InstID)
	cr.PlateMap["period"+mx.Period] = pl
	// }
	_, ok = pl.CoasterMap["period"+mx.Period]
	if !ok {
		pl.MakeCoaster(cr, mx.Period)
	}
	// if pl.CoasterMap["period"+mx.Period] == nil {
	// fmt.Println("candle coaster: ", mx.Period, pl.CoasterMap["period"+mx.Period], pl.CoasterMap)
	// 创建失败的原因是原始数据不够，一般发生在服务中断了，缺少部分数据的情况下, 后续需要数据补全措施
	// err := errors.New("coaster创建失败 maX instId: " + mx.InstID + "; period: " + mx.Period)
	// return nil, err
	// }
	sm, err := pl.CoasterMap["period"+mx.Period].RPushSample(cr, mx, "ma"+strconv.Itoa(mx.Count))
	return sm, err
}