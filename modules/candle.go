package module

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/phyer/core"
	"os"
	"strconv"
	"strings"
	// "sync"
	"time"
	//
	// simple "github.com/bitly/go-simplejson"
	"github.com/go-redis/redis"
	// "github.com/phyer/core/utils"
	logrus "github.com/sirupsen/logrus"
)

type MyCandle struct {
	core.Candle
}

func (cd *MyCandle) Process(cr *core.Core) {
	tmi := ToInt64(cd.Data[0])
	time.UnixMilli(tmi).Format("01-02 15:04")
	// 如果sardine是运行在远端，不能让candle存盘, 因为这是tunnas该干的事情，不能跟它抢
	founded := cd.Filter(cr)
	if !founded {
		return
	}
	go func() {
		saveCandle := os.Getenv("SARDINE_SAVECANDLE")
		if saveCandle == "true" {
			_, err := cd.SetToKey(cr)
			if err != nil {
				logrus.Warning("SetToKey err: ", err)
			}
		}
	}()
	// TODO update plate and coaster
	go func() {
		makeSeries := os.Getenv("SARDINE_MAKESERIES")
		if makeSeries == "true" {
			_, err := cd.InsertIntoPlate(cr)
			if err != nil {
				logrus.Warning("SetToKey err: ", err)
			}
		}
	}()

	// 由于max可以从远端直接拿，不需要sardine自己做，所以可以sardine做也可以不做
	go func(cad *MyCandle) {
		makeMaX := os.Getenv("SARDINE_MAKEMAX")
		if makeMaX == "true" {
			// 等一会儿防止candle还没有加进CoinMap
			time.Sleep(200 * time.Millisecond)
			cr.MakeMaXsChan <- &cad.Candle
		}
	}(cd)

	go func(cad *MyCandle) {
		// 触发制作插值candle
		makeSoft := false
		// makeVolSoft := true
		// makeVolSoft := false
		if os.Getenv("SARDINE_MAKESOFTCANDLE") == "true" {
			makeSoft = true
		}
		// 根据低维度candle，模拟出"软"的高纬度candle
		if cad.Period == "1m" && makeSoft {
			fmt.Println("makeSoft:", cad.Period, cad.InstId)
			MakeSoftCandles(cr, &cad.Candle)
		}
	}(cd)
	go func(cad *MyCandle) {
		time.Sleep(100 * time.Millisecond)
		cr.AddToGeneralCandleChnl(cad)
	}(cd)
}

func (cd *MyCandle) InsertIntoPlate(cr *core.Core) (*core.Sample, error) {
	cr.Mu.Lock()
	defer cr.Mu.Unlock()
	// pl, plateFounded := cr.PlateMap[cd.InstID]
	// if !plateFounded || pl == nil {
	pl, _ := LoadPlate(cr, cd.InstID)
	cr.PlateMap[cd.InstID] = pl
	// }
	po, coasterFounded := pl.CoasterMap["period"+cd.Period]
	err := errors.New("")
	if !coasterFounded {
		pl.MakeCoaster(cr, cd.Period)
	}

	if len(po.InstID) == 0 {
		// logrus.Debug("candle coaster: ", cd.Period, pl.CoasterMap["period"+cd.Period], pl.CoasterMap)
		//创建失败的原因是原始数据不够，一般发生在服务中断了，缺少部分数据的情况下, 后续需要数据补全措施
		erstr := fmt.Sprintln("coaster创建失败 candle instID: "+cd.InstID+"; period: "+cd.Period, "coasterFounded: ", coasterFounded, " ", err)
		err := errors.New(erstr)
		return nil, err
	}
	sm, err := po.RPushSample(cr, *cd, "candle")
	return sm, err
}
