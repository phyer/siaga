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
	"github.com/go-redis/redis"
	// "github.com/phyer/core/utils"
	logrus "github.com/sirupsen/logrus"
)

func GetRemoteRedisConfigList() ([]*core.RedisConfig, error) {
	list := []*core.RedisConfig{}
	envListStr := os.Getenv("SIAGA_UPSTREAM_REDIS_LIST")
	envList := strings.Split(envListStr, "|")
	for _, v := range envList {
		if len(v) == 0 {
			continue
		}
		urlstr := "SIAGA_UPSTREAM_REDIS_" + v + "_URL"
		indexstr := "SIAGA_UPSTREAM_REDIS_" + v + "_INDEX"
		password := os.Getenv("SIAGA_UPSTREAM_REDIS_" + v + "_PASSWORD")
		// channelstr := core.REMOTE_REDIS_PRE_NAME + v + "_CHANNEL_PRENAME"
		// channelPreName := os.Getenv(channelstr)
		url := os.Getenv(urlstr)
		index := os.Getenv(indexstr)
		if len(url) == 0 || len(index) == 0 {
			err := errors.New("remote redis config err:" + urlstr + "," + url + "," + indexstr + "," + index)
			return list, err
		}
		idx, err := strconv.Atoi(index)
		if err != nil {
			return list, err
		}
		curConf := core.RedisConfig{
			Url:      url,
			Password: password,
			Index:    idx,
			//	ChannelPreName: channelPreName,
		}
		list = append(list, &curConf)
	}
	return list, nil
}

func LoopSubscribe(cr *core.Core, channelName string, redisConf *core.RedisConfig) {
	redisRemoteCli, _ := cr.GetRedisCliFromConf(*redisConf)
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
		} else if strings.Contains(channelName, "allMaX") {
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
		logrus.Warning("msg.Payload: ", msg.Payload)
		fmt.Println("channelName: ", channelName, " msg.Payload: ", msg.Payload)
		switch ctype {
		// 接收到的candle扔到 candle 二次加工流水线
		case "candle":
			{
				cd := core.Candle{}
				json.Unmarshal([]byte(msg.Payload), &cd)
				founded := cd.Filter(cr)
				if !founded {
					break
				}
				cr.CandlesProcessChan <- &cd
			}

		// 接收到的maX扔到 maX 二次加工流水线
		case "maX":
			{
				mx := core.MaX{}
				if msg.Payload == "" {
					continue
				}
				json.Unmarshal([]byte(msg.Payload), &mx)
				dt := []interface{}{}
				dt = append(dt, mx.Ts)
				dt = append(dt, mx.AvgVal)
				mx.Data = dt
				cr.MaXProcessChan <- &mx
			}

		// 接收到的tinckerInfo扔到 tickerInfo 二次加工流水线
		case "tickerInfo":
			{
				//tickerInfo:  map[askPx:2.2164 askSz:17.109531 bidPx:2.2136 bidSz:73 high24h:2.497 instId:STX-USDT instType:SPOT last:2.2136 lastSz:0 low24h:2.0508 open24h:2.42 sodUtc0:2.4266 sodUtc8:2.4224 ts:1637077323552 vol24h:5355479.488179 :12247398.975501]
				ti := core.TickerInfo{}
				err := json.Unmarshal([]byte(msg.Payload), &ti)
				if err != nil {
					logrus.Warning("tickerInfo payload unmarshal err: ", err, msg.Payload)
				}
				cr.TickerInforocessChan <- &ti
			}
			// case "seriesInfo":
			// 	{
			// 		//tickerInfo:  map[askPx:2.2164 askSz:17.109531 bidPx:2.2136 bidSz:73 high24h:2.497 instId:STX-USDT instType:SPOT last:2.2136 lastSz:0 low24h:2.0508 open24h:2.42 sodUtc0:2.4266 sodUtc8:2.4224 ts:1637077323552 vol24h:5355479.488179 :12247398.975501]
			// 		sei := SeriesInfo{}
			// 		err := json.Unmarshal([]byte(msg.Payload), &sei)
			// 		if err != nil {
			// 			logrus.Warning("seriesInfo payload unmarshal err: ", err, msg.Payload)
			// 		}
			// 		cr.SeriesChan <- &sei
			// continue
			// 	}
		}
	}
}

func LoopMakeMaX(cr *core.Core) {
	for {
		cd := <-cr.MakeMaXsChan
		go func(cad *core.Candle) {
			//当一个candle的多个时间点的数据几乎同时到达时，顺序无法保证，制作maX会因为中间缺失数据而计算错，因此，等待一秒钟等数据都全了再算
			// sz := utils.ShaiziInt(1500) + 500
			time.Sleep(time.Duration(30) * time.Millisecond)
			err, ct := MakeMaX(cr, cad, 7)
			logrus.Warn(GetFuncName(), " ma7 err:", err, " ct:", ct, " cd.InstID:", cd.InstID, " cd.Period:", cd.Period)
			//TODO 这个思路不错，单行不通，远程redis禁不住这么频繁的请求
			// cd.InvokeRestQFromRemote(cr, ct)
		}(cd)
		go func(cad *core.Candle) {
			//当一个candle的多个时间点的数据几乎同时到达时，顺序无法保证，制作maX会因为中间缺失数据而计算错，因此，等待一秒钟等数据都全了再算
			// sz := utils.ShaiziInt(2000) + 500
			time.Sleep(time.Duration(30) * time.Millisecond)
			err, ct := MakeMaX(cr, cad, 30)
			logrus.Warn(GetFuncName(), " ma30 err:", err, " ct:", ct, " cd.InstID:", cd.InstID, " cd.Period:", cd.Period)
			// cd.InvokeRestQFromRemote(cr, ct)
		}(cd)
		go func(cad *core.Candle) {
			time.Sleep(time.Duration(300) * time.Millisecond)
			err, ct := MakeRsi(cr, cad, 14, true)
			logrus.Warn(GetFuncName(), " rsi14 err:", err, " ct:", ct, " cd.InstID:", cd.InstID, " cd.Period:", cd.Period)
		}(cd)
		go func(cad *core.Candle) {
			time.Sleep(time.Duration(300) * time.Millisecond)
			err, ct := MakeRsi(cr, cad, 12, false)
			logrus.Warn(GetFuncName(), " rsi12 err:", err, " ct:", ct, " cd.InstID:", cd.InstID, " cd.Period:", cd.Period)
		}(cd)
		go func(cad *core.Candle) {
			time.Sleep(time.Duration(300) * time.Millisecond)
			err, ct := MakeRsi(cr, cad, 24, false)
			logrus.Warn(GetFuncName(), " rsi24 err:", err, " ct:", ct, " cd.InstID:", cd.InstID, " cd.Period:", cd.Period)
		}(cd)
		// TODO TODO 这地方不能加延时，否则makeMax处理不过来，多的就丢弃了，造成maX的sortedSet比candle的短很多。后面所有依赖的逻辑都受影响.
		// time.Sleep(300 * time.Millisecond)
	}
}

// setName := "candle" + period + "|" + instId + "|sortedSet"
// count: 倒推多少个周期开始拿数据
// from: 倒推的起始时间点
// ctype: candle或者maX
func GetRangeCandleSortedSet(cr *core.Core, setName string, count int, from time.Time) (*core.CandleList, error) {
	cdl := core.CandleList{}
	ary1 := strings.Split(setName, "|")
	ary2 := []string{}
	period := ""
	ary2 = strings.Split(ary1[0], "candle")
	period = ary2[1]

	dui, err := cr.PeriodToMinutes(period)
	if err != nil {
		return nil, err
	}
	fromt := from.UnixMilli()
	nw := time.Now().UnixMilli()
	if fromt > nw*2 {
		err := errors.New("时间错了需要debug")
		logrus.Warning(err.Error())
		return nil, err
	}
	froms := strconv.FormatInt(fromt, 10)
	sti := fromt - dui*int64(count)*60*1000
	sts := strconv.FormatInt(sti, 10)
	opt := redis.ZRangeBy{
		Min:   sts,
		Max:   froms,
		Count: int64(count),
	}
	ary := []string{}
	extt, err := GetExpiration(cr, period)
	ot := time.Now().Add(extt * -1)
	oti := ot.UnixMilli()
	cli := cr.RedisLocalCli
	cli.LTrim(setName, 0, oti)
	cunt, _ := cli.ZRemRangeByScore(setName, "0", strconv.FormatInt(oti, 10)).Result()
	if cunt > 0 {
		logrus.Warning("移出过期的引用数量：", setName, count, "ZRemRangeByScore ", setName, 0, strconv.FormatInt(oti, 10))
	}
	logrus.Info("ZRevRangeByScore ", setName, opt)
	ary, err = cli.ZRevRangeByScore(setName, opt).Result()
	fmt.Println("ary: ", ary, " setName: ", setName, " opt: ", opt)
	if err != nil {
		return &cdl, err
	}
	keyAry, err := cli.MGet(ary...).Result()
	if err != nil || len(keyAry) == 0 {
		logrus.Warning("no record with cmd:  ZRevRangeByScore ", setName, froms, sts, " ", err.Error())
		logrus.Warning("zrev lens of ary: lens: ", len(ary), "GetRangeSortedSet ZRevRangeByScore:", "setName:", setName, "opt.Max:", opt.Max, "opt.Min:", opt.Min)
		return &cdl, err
	}
	for _, str := range keyAry {
		if str == nil {
			continue
		}
		cd := core.Candle{}
		err := json.Unmarshal([]byte(str.(string)), &cd)
		if err != nil {
			logrus.Warn(GetFuncName(), err, str.(string))
		}
		tmi := ToInt64(cd.Data[0])
		tm := time.UnixMilli(tmi)
		if tm.Sub(from) > 0 {
			break
		}
		cdl.List = append(cdl.List, &cd)
	}
	cdl.Count = count
	return &cdl, nil
}

func GetExpiration(cr *core.Core, per string) (time.Duration, error) {
	if len(per) == 0 {
		erstr := fmt.Sprint("period没有设置")
		logrus.Warn(erstr)
		err := errors.New(erstr)
		return 0, err
	}
	exp, err := cr.PeriodToMinutes(per)
	dur := time.Duration(exp*49) * time.Minute
	return dur, err
}
func MakeRsi(cr *core.Core, cl *core.Candle, count int, makeStock bool) (error, int) {
	data := cl.Data
	js, _ := json.Marshal(data)
	if len(data) == 0 {
		err := errors.New("data is block: " + string(js))
		return err, 0
	}
	tsi := ToInt64(data[0])
	// tss := strconv.FormatInt(tsi, 10)
	// keyName := "candle" + cl.Period + "|" + cl.InstID + "|ts:" + tss
	lastTime := time.UnixMilli(tsi)
	setName := "candle" + cl.Period + "|" + cl.InstID + "|sortedSet"
	// dcount := count * 2
	cdl, err := GetRangeCandleSortedSet(cr, setName, count*2, lastTime)
	if err != nil {
		return err, 0
	}
	// amountLast := float64(0)
	// ct := float64(0)
	if len(cdl.List) < 2*count {
		err = errors.New("sortedSet长度不足:" + ToString(len(cdl.List)) + " ,无法进行rsi计算")
		return err, 0
	}
	cdl.RecursiveBubbleS(len(cdl.List), "asc")
	closeList := []float64{}
	ll := len(cdl.List)
	fmt.Println("candleList len:", ll)
	for k, v := range cdl.List {
		fmt.Println("candle in list", ll, k, v)
		closeList = append(closeList, ToFloat64(v.Data[4]))
	}
	rsiList, err := CalculateRSI(closeList, count)
	if err != nil {
		fmt.Println("Error calculating RSI:", err)
		return err, 0
	}
	rsi := core.Rsi{
		InstID:     cl.InstID,
		Period:     cl.Period,
		Timestamp:  cl.Timestamp,
		Ts:         tsi,
		Count:      count,
		LastUpdate: time.Now(),
		RsiVol:     rsiList[len(rsiList)-1],
		Confirm:    false,
	}
	periodMins, err := cr.PeriodToMinutes(cl.Period)
	duration := rsi.LastUpdate.Sub(cl.Timestamp) // 获取时间差
	//最后更新时间差不多大于一个周期，判定为已完成
	if duration > time.Duration(periodMins-1)*time.Minute {
		rsi.Confirm = true
	}

	fmt.Println("will send rsi")
	go func() {
		fmt.Println("make a rsi")
		cr.RsiProcessChan <- &rsi
	}()
	if !makeStock {
		return nil, 0
	}

	percentK, percentD, err := CalculateStochRSI(rsiList, count, 3, 3)

	if err != nil {
		fmt.Println("Error calculating StochRSI:", err)
		return err, 0
	}
	srsi := core.StockRsi{
		InstID:     cl.InstID,
		Period:     cl.Period,
		Timestamp:  cl.Timestamp,
		Ts:         tsi,
		Count:      count,
		LastUpdate: time.Now(),
		KVol:       percentK[len(percentK)-1],
		DVol:       percentD[len(percentD)-1],
		Confirm:    true,
	}

	fmt.Println("will send stockrsi")
	go func() {
		fmt.Println("make a stockrsi")
		cr.StockRsiProcessChan <- &srsi
	}()

	return nil, 0
}
func MakeMaX(cr *core.Core, cl *core.Candle, count int) (error, int) {
	data := cl.Data
	js, _ := json.Marshal(data)
	// cjs, _ := json.Marshal(cl)
	if len(data) == 0 {
		err := errors.New("data is block: " + string(js))
		return err, 0
	}

	tsi := ToInt64(data[0])
	// tsa := time.UnixMilli(tsi).Format("01-02 15:03:04")
	// fmt.Println("MakeMaX candle: ", cl.InstID, cl.Period, tsa, cl.From)
	tss := strconv.FormatInt(tsi, 10)
	keyName := "candle" + cl.Period + "|" + cl.InstID + "|ts:" + tss
	//过期时间：根号(当前candle的周期/1分钟)*10000

	lastTime := time.UnixMilli(tsi)
	// lasts := lastTime.Format("2006-01-02 15:04")
	// 以当前candle的时间戳为起点倒推count个周期，取得所需candle用于计算maX
	setName := "candle" + cl.Period + "|" + cl.InstID + "|sortedSet"
	// cdl, err := cr.GetLastCandleListOfCoin(cl.InstID, cl.Period, count, lastTime)
	cdl, err := GetRangeCandleSortedSet(cr, setName, count, lastTime)
	if err != nil {
		return err, 0
	}

	// fmt.Println("makeMaX: list: ", "instId: ", cl.InstID, "cl.Period: ", cl.Period, " lastTime:", lastTime, " count: ", count)
	amountLast := float64(0)
	ct := float64(0)
	// fmt.Println("makeMax len of GetLastCandleListOfCoin list: ", len(cdl.List), "makeMax err of GetLastCandleListOfCoin: ", err)
	if len(cdl.List) == 0 {
		return err, 0
	}
	// ljs, _ := json.Marshal(cdl.List)
	// fmt.Println("makeMax: ljs: ", string(ljs))
	for _, v := range cdl.List {
		curLast, err := strconv.ParseFloat(v.Data[4].(string), 64)
		if err != nil {
			continue
		}
		if curLast > 0 {
			ct++
		}
		amountLast += curLast
		//----------------------------------------------
	}
	avgLast := amountLast / ct
	if float64(ct) < float64(count) {
		err := errors.New("no enough source to calculate maX ")
		return err, int(float64(count) - ct)
		// fmt.Println("makeMax err: 没有足够的数据进行计算ma", "candle:", cl, "counts:", count, "ct:", ct, "avgLast: ")
	} else {
		// fmt.Println("makeMax keyName: ma", count, keyName, " avgLast: ", avgLast, "ts: ", tsi, "ct: ", ct, "ots: ", ots, "candle: ", string(cjs))

	}
	tm, _ := core.Int64ToTime(tsi)
	fmt.Println("max tm:", tm)
	mx := core.MaX{
		KeyName:   keyName,
		InstID:    cl.InstID,
		Period:    cl.Period,
		From:      cl.From,
		Count:     count,
		Ts:        tsi,
		AvgVal:    avgLast,
		Timestamp: tm,
	}
	// MaX的Data里包含三个有效信息：时间戳，平均值，计算平均值所采用的数列长度
	dt := []interface{}{}
	dt = append(dt, mx.Ts)
	dt = append(dt, mx.AvgVal)
	dt = append(dt, ct)
	mx.Data = dt

	// key存到redis

	cr.MaXProcessChan <- &mx
	return nil, 0
}

func CandlesProcess(cr *core.Core) {
	for {
		cd := <-cr.CandlesProcessChan
		cd.LastUpdate = time.Now()
		// logrus.Debug("cd: ", cd)
		fmt.Println("candle in process: ", cd)
		go func(cad *core.Candle) {
			mcd := MyCandle{
				Candle: *cad,
			}
			mcd.Process(cr)
		}(cd)
	}
}

// 使用当前某个原始维度的candle对象，生成其他目标维度的candle对象，比如用3分钟的candle可以生成15分钟及以上的candle
// {
// "startTime": "2021-12-04 20:00",
// "seg": "m",
// "count": 1
// },
// 从startTime开始，经历整数个(count * seg)之后，还能不大于分钟粒度的当前时间的话，那个时间点就是最近的当前段起始时间点
func MakeSoftCandles(cr *core.Core, mcd *MyCandle) {
	segments := cr.Cfg.Config.Get("softCandleSegmentList").MustArray()
	for k, v := range segments {
		cs := core.CandleSegment{}
		sv, _ := json.Marshal(v)
		json.Unmarshal(sv, &cs)
		// if k > 2 {
		// continue
		// }
		if !cs.Enabled {
			continue
		}
		// TODO: 通过序列化和反序列化，对原始的candle进行克隆，因为是对引用进行操作，所以每个seg里对candle进行操作都会改变原始对象，这和预期不符
		bt, _ := json.Marshal(mcd.Candle)
		cd0 := core.Candle{}
		json.Unmarshal(bt, &cd0)

		tmi := ToInt64(cd0.Data[0])
		ts, _ := core.Int64ToTime(tmi)
		tm := time.UnixMilli(tmi)
		if tm.Unix() > 10*time.Now().Unix() {
			continue
		}
		// 下面这几种目标维度的，不生成softCandle
		if cs.Seg == "1m" {
			continue
		}

		otm, err := cr.PeriodToLastTime(cs.Seg, tm)
		logrus.Warn("MakeSoftCandles cs.Seg: ", cs.Seg, ", otm:", otm)

		if err != nil {
			logrus.Warning("MakeSoftCandles err: ", err)
		}
		otmi := otm.UnixMilli()
		cd1 := core.Candle{
			InstID:    cd0.InstID,                      // string        `json:"instId", string`
			Period:    cs.Seg,                          //   `json:"period", string`
			Data:      cd0.Data,                        //  `json:"data"`
			From:      "soft|" + os.Getenv("HOSTNAME"), //  string        `json:"from"`
			Timestamp: ts,
		}
		// cd0是从tickerInfo创建的1m Candle克隆来的, Data里只有Data[4]被赋值，是last，其他都是"-1"
		// TODO 填充其余几个未赋值的字段，除了成交量和成交美元数以外，并存入redis待用
		// strconv.FormatInt(otmi, 10)
		mcd := MyCandle{
			Candle: cd0,
		}
		cd1.Data = mcd.GetSetCandleInfo(cr, cs.Seg, otmi)
		tmi = ToInt64(cd1.Data[0])
		tm = time.UnixMilli(tmi)
		cd1.Timestamp = tm
		cd1.Open = ToFloat64(cd1.Data[1])
		cd1.High = ToFloat64(cd1.Data[2])
		cd1.Low = ToFloat64(cd1.Data[3])
		cd1.Close = ToFloat64(cd1.Data[4])
		cd1.VolCcy = ToFloat64(cd1.Data[6])

		// 生成软交易量和交易数对,用于代替last生成max
		go func(k int) {
			time.Sleep(time.Duration(10*k) * time.Millisecond)
			cr.CandlesProcessChan <- &cd1
		}(k)
	}
}

func MaXsProcess(cr *core.Core) {
	for {
		mx := <-cr.MaXProcessChan
		mx.LastUpdate = time.Now()
		logrus.Debug("mx: ", mx)
		go func(maX *core.MaX) {
			mmx := MyMaX{
				MaX: *mx,
			}
			mmx.Process(cr)
		}(mx)
	}
}
func RsisProcess(cr *core.Core) {
	for {
		rsi := <-cr.RsiProcessChan
		// logrus.Debug("mx: ", mx)
		fmt.Println("rsi recieved:", rsi)
		go func(rsi *core.Rsi) {
			mrs := MyRsi{
				Rsi: *rsi,
			}
			mrs.Process(cr)
		}(rsi)
	}
}

func StockRsisProcess(cr *core.Core) {
	for {
		srsi := <-cr.StockRsiProcessChan
		// logrus.Debug("mx: ", mx)
		fmt.Println("stockrsi recieved:", srsi)
		go func(srsi *core.StockRsi) {
			mrs := MyStockRsi{
				StockRsi: *srsi,
			}
			mrs.Process(cr)
		}(srsi)
	}
}
func TickerInfoProcess(cr *core.Core) {
	for {
		ti := <-cr.TickerInforocessChan
		logrus.Debug("ti: ", ti)
		go func(ti *core.TickerInfo) {
			mti := MyTickerInfo{
				TickerInfo: *ti,
			}
			mti.Process(cr)
		}(ti)
	}
}

// 计算 RSI 的函数

// CalculateRSI calculates the RSI value for a given period and price data.
// prices: input price data, must be equal to the period length.

// CalculateRSI calculates the Relative Strength Index (RSI) for a given period.
func CalculateRSI(prices []float64, period int) ([]float64, error) {
	if len(prices) < period {
		return nil, errors.New("not enough data to calculate RSI")
	}

	rsi := make([]float64, len(prices)-period+1)
	var avgGain, avgLoss float64

	// Initial average gain and loss
	for i := 1; i <= period; i++ {
		change := prices[i] - prices[i-1]
		if change > 0 {
			avgGain += change
		} else {
			avgLoss -= change
		}
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	if avgLoss == 0 {
		rsi[0] = 100
	} else {
		rs := avgGain / avgLoss
		rsi[0] = 100 - (100 / (1 + rs))
	}

	// Calculate RSI for the rest of the data
	for i := period; i < len(prices); i++ {
		change := prices[i] - prices[i-1]
		if change > 0 {
			avgGain = (avgGain*(float64(period)-1) + change) / float64(period)
			avgLoss = (avgLoss * (float64(period) - 1)) / float64(period)
		} else {
			avgGain = (avgGain * (float64(period) - 1)) / float64(period)
			avgLoss = (avgLoss*(float64(period)-1) - change) / float64(period)
		}

		if avgLoss == 0 {
			rsi[i-period+1] = 100
		} else {
			rs := avgGain / avgLoss
			rsi[i-period+1] = 100 - (100 / (1 + rs))
		}
	}

	return rsi, nil
}

// CalculateStochRSI calculates the Stochastic RSI.
func CalculateStochRSI(rsi []float64, period int, kSmoothing int, dSmoothing int) ([]float64, []float64, error) {
	if len(rsi) < period {
		return nil, nil, errors.New("not enough data to calculate StochRSI")
	}

	stochRsi := make([]float64, len(rsi)-period+1)
	for i := period; i <= len(rsi); i++ {
		lowest := rsi[i-period]
		highest := rsi[i-period]
		for j := i - period + 1; j < i; j++ {
			if rsi[j] < lowest {
				lowest = rsi[j]
			}
			if rsi[j] > highest {
				highest = rsi[j]
			}
		}

		if highest == lowest {
			stochRsi[i-period] = 0
		} else {
			stochRsi[i-period] = (rsi[i-1] - lowest) / (highest - lowest)
		}
	}

	// Smooth %K
	percentK := smooth(stochRsi, kSmoothing)

	// Smooth %D (signal line)
	percentD := smooth(percentK, dSmoothing)

	return percentK, percentD, nil
}

// Smooth applies a simple moving average to smooth the data.
func smooth(data []float64, period int) []float64 {
	if period <= 1 || len(data) < period {
		return data
	}

	smoothed := make([]float64, len(data)-period+1)
	for i := period - 1; i < len(data); i++ {
		sum := 0.0
		for j := i - period + 1; j <= i; j++ {
			sum += data[j]
		}
		smoothed[i-period+1] = sum / float64(period)
	}

	return smoothed
}
