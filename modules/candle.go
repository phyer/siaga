package module

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/phyer/core"

	// "sync"
	"time"
	//
	// simple "github.com/bitly/go-simplejson"
	// "github.com/go-redis/redis"
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
	// 把candle存下来
	go func() {
		saveCandle := os.Getenv("SIAGA_SAVECANDLE")
		if saveCandle == "true" {
			_, err := cd.SetToKey(cr)
			if err != nil {
				logrus.Warning("SetToKey err: ", err)
			}
		}
		// 对于软candle，推到elasticSearch
		logrus.Info("cd.from in MyCandle Process:", cd.From)
		if strings.HasPrefix(cd.From, "soft") {
			cd.PushToWriteLogChan(cr)
		}
	}()
	// TODO update plate and coaster
	go func() {
		makeSeries := os.Getenv("SIAGA_MAKESERIES")
		if makeSeries == "true" {
			_, err := cd.InsertIntoPlate(cr)
			if err != nil {
				logrus.Warning("SetToKey err: ", err)
			}
		}
	}()

	// 由于max可以从远端直接拿，不需要sardine自己做，所以可以sardine做也可以不做
	go func(cad *MyCandle) {
		makeMaX := os.Getenv("SIAGA_MAKEMAX")
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
		if os.Getenv("SIAGA_MAKESOFTCANDLE") == "true" {
			makeSoft = true
		}
		// 根据低维度candle，模拟出"软"的高纬度candle
		if cad.Period == "1m" && makeSoft {
			MakeSoftCandles(cr, cad)
		}
	}(cd)
	// 再次发布到redis channel， 給可能的收听者，我觉得没有这个必要了吧
	// go func(cad *MyCandle) {
	// 	time.Sleep(100 * time.Millisecond)
	// 	cad.AddToGeneralCandleChnl(cr)
	// }(cd)
}

func (cd *MyCandle) InsertIntoPlate(cr *core.Core) (*core.Sample, error) {
	if cr == nil {
		return nil, errors.New("core is nil")
	}

	if cr.Mu == nil {
		return nil, errors.New("core mutex is nil")
	}

	cr.Mu.Lock()
	defer cr.Mu.Unlock()

	if cr.PlateMap == nil {
		return nil, errors.New("PlateMap is nil")
	}

	pl, err := LoadPlate(cr, cd.InstID)
	if err != nil {
		return nil, fmt.Errorf("failed to load plate: %w", err)
	}
	cr.PlateMap[cd.InstID] = pl

	if pl == nil {
		return nil, errors.New("loaded plate is nil")
	}

	po, coasterFounded := pl.CoasterMap["period"+cd.Period]
	if !coasterFounded {
		_, err := pl.MakeCoaster(cr, cd.Period)
		if err != nil {
			return nil, fmt.Errorf("failed to make coaster: %w", err)
		}
		po, coasterFounded = pl.CoasterMap["period"+cd.Period]
	}

	if !coasterFounded || len(po.InstID) == 0 {
		return nil, fmt.Errorf("coaster creation failed for instID: %s, period: %s", cd.InstID, cd.Period)
	}

	sm, err := po.RPushSample(cr, &cd.Candle, "candle")
	if err != nil {
		return nil, fmt.Errorf("failed to push sample: %w", err)
	}

	return sm, nil
}

func (cad *MyCandle) AddToGeneralCandleChnl(cr *core.Core) {
	suffix := ""
	env := os.Getenv("GO_ENV")
	if strings.Contains(env, "demoEnv") {
		suffix = "-demoEnv"
	}
	redisCli := cr.RedisLocalCli
	ab, _ := json.Marshal(cad.Candle)
	_, err := redisCli.Publish(core.ALLCANDLES_PUBLISH+suffix, string(ab)).Result()
	// logrus.Debug("publish, res,err:", res, err, "candle:", string(ab))
	if err != nil {
		logrus.Debug("err of ma7|ma30 add to redis2:", err, cad.Candle.From)
	}
}

// TODO  用key名 BTC-USDT|3M|CandleInfo 维护唯一的一份。
// 当这个key不存在的时候，创建这个key。周期的起始时间，目前MakeSoftCandle函数已经实现了这个逻辑。
// 当存在时，检查现有key里的时间信息，和刚创建的周期其实时间一致不一致，
// 如果一致，更新这个CandleInfo的last值，如果需要的话，更新high和low值。
// 当周期起始时间和已经保存的周期起始时间不一致的时候，说明到了跨周期的时间点了。这时候更新现有的key里保存的open，high，low，close信息为当前tickerInfo的last值。
func (mcd MyCandle) GetSetCandleInfo(cr *core.Core, newPeriod string, ts int64) []interface{} {
	tss := strconv.FormatInt(ts, 10)
	cd := mcd.Candle
	keyName := cd.InstID + "|" + newPeriod + "|candleData"
	str, _ := cr.RedisLocalCli.Get(keyName).Result()
	odata := []interface{}{}
	founded := false

	if len(str) > 0 {
		err := json.Unmarshal([]byte(str), &odata)
		if err != nil {
			logrus.Panic(GetFuncName(), " str2:", str, " err:", err)
		} else {
			founded = true
		}
	}
	if !founded {
		cd.Data[0] = tss        // 时间，数据都是新的
		cd.Data[1] = cd.Data[4] //开盘价
		cd.Data[2] = cd.Data[4] //最高价
		cd.Data[3] = cd.Data[4] //最低价
		bj, _ := json.Marshal(cd.Data)
		cr.RedisLocalCli.Set(keyName, string(bj), 0)
		return cd.Data
	} else {
		// 发现原有的candleData，且还没过期

		if (odata[0]).(string) == tss {
			oHigh := ToFloat64(odata[2])
			oLow := ToFloat64(odata[3])
			cd.Data[0] = tss
			cd.Data[1] = odata[1]
			cd.Data[5] = odata[5]
			cd.Data[6] = odata[6]
			if oHigh <= ToFloat64(cd.Data[4]) {
				cd.Data[2] = cd.Data[4]
			} else {
				cd.Data[2] = odata[2]
			}
			if oLow >= ToFloat64(cd.Data[4]) {
				cd.Data[3] = cd.Data[4]
			} else {
				cd.Data[3] = odata[3]
			}
			bj, _ := json.Marshal(cd.Data)
			cr.RedisLocalCli.Set(keyName, string(bj), 0)
			return cd.Data
		} else {
			// 发现原有的candleData，但是已经过期了
			cd.Data[0] = tss        // 新开始一个cd周期，时间，数据都是新的
			cd.Data[1] = cd.Data[4] //开盘价
			cd.Data[2] = cd.Data[4] //最高价
			cd.Data[3] = cd.Data[4] //最低价
			cd.Data[5] = odata[5]
			cd.Data[6] = odata[6]
			bj, _ := json.Marshal(cd.Data)
			cr.RedisLocalCli.Set(keyName, string(bj), 0)
			return cd.Data
		}
	}
}
