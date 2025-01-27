package module

import (
	"context"
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

func getChannelType(channelName string) string {
	switch {
	case strings.Contains(channelName, "allCandle"):
		return "candle"
	case strings.Contains(channelName, "allMaX"):
		return "maX"
	case strings.Contains(channelName, "ticker"):
		return "tickerInfo"
	case strings.Contains(channelName, "ccyPositions"):
		return "ccyPositions"
	case strings.Contains(channelName, "allseriesinfo"):
		return "seriesinfo"
	case strings.Contains(channelName, "private|order"):
		return "private|order"
	default:
		return ""
	}
}

// LoopSubscribe 循环订阅指定的 Redis 频道
// 参数：
//   - cr: 核心对象
//   - channelName: 要订阅的频道名称
//   - redisConf: Redis 配置信息
func LoopSubscribe(cr *core.Core, channelName string, redisConf *core.RedisConfig) {
	// 创建上下文用于控制循环
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 重试机制配置
	retryCount := 0               // 当前重试次数
	maxRetries := 3               // 最大重试次数
	retryDelay := time.Second * 5 // 重试间隔时间

	// 主循环
	for {
		select {
		case <-ctx.Done(): // 如果上下文被取消
			return
		default:
			// 尝试订阅并处理消息
			err := subscribeAndProcess(ctx, cr, channelName, redisConf)
			if err != nil {
				// 如果出错，增加重试计数
				retryCount++
				if retryCount > maxRetries {
					// 超过最大重试次数，记录错误并返回
					logrus.Errorf("Max retries reached for channel %s, giving up", channelName)
					return
				}
				// 记录警告并等待重试
				logrus.Warnf("Subscription failed, retrying in %v (attempt %d/%d)", retryDelay, retryCount, maxRetries)
				time.Sleep(retryDelay)
				continue
			}
			// 成功则返回
			return
		}
	}
}

// subscribeAndProcess 订阅指定的 Redis 频道并处理接收到的消息
// 参数：
//   - ctx: 上下文对象，用于控制订阅的生命周期
//   - cr: 核心对象，包含系统配置和功能
//   - channelName: 要订阅的频道名称
//   - redisConf: Redis 连接配置信息
//
// 返回值：
//   - error: 如果订阅或处理过程中出现错误则返回错误信息
func subscribeAndProcess(ctx context.Context, cr *core.Core, channelName string, redisConf *core.RedisConfig) error {
	// 根据配置获取 Redis 客户端
	redisRemoteCli, err := cr.GetRedisCliFromConf(*redisConf)
	if err != nil {
		return fmt.Errorf("failed to get redis client: %w", err)
	}

	// 根据环境变量判断是否需要添加后缀
	suffix := ""
	if strings.Contains(os.Getenv("GO_ENV"), "demoEnv") {
		suffix = "-demoEnv"
	}

	// 构建完整的频道名称
	fullChannelName := channelName + suffix
	logrus.Infof("Starting subscription to channel: %s", fullChannelName)

	// 订阅指定频道
	pubsub := redisRemoteCli.Subscribe(fullChannelName)
	defer pubsub.Close() // 确保在函数结束时关闭订阅

	// 设置5秒超时等待订阅确认
	if _, err := pubsub.ReceiveTimeout(time.Second * 5); err != nil {
		return fmt.Errorf("failed to subscribe to channel %s: %w", fullChannelName, err)
	}

	// 获取消息通道
	ch := pubsub.Channel()

	// 主消息处理循环
	for {
		select {
		case <-ctx.Done(): // 如果上下文被取消则退出
			return nil
		case msg := <-ch: // 接收到新消息
			// 过滤无效消息
			if msg == nil || msg.Payload == "" {
				continue
			}

			// 处理消息内容
			if err := processMessage(cr, channelName, msg.Payload); err != nil {
				logrus.Warnf("Failed to process message: %v", err)
			}
		}
	}
}

func processMessage(cr *core.Core, channelName string, payload string) error {
	ctype := getChannelType(channelName)
	if ctype == "" {
		return fmt.Errorf("unrecognized channel name: %s", channelName)
	}

	logrus.Debugf("Received message from %s", channelName)

	switch ctype {
	case "candle":
		var cd core.Candle
		if err := json.Unmarshal([]byte(payload), &cd); err != nil {
			return fmt.Errorf("failed to unmarshal candle: %w", err)
		}
		if cd.Filter(cr) {
			cr.CandlesProcessChan <- &cd
		}

	case "maX":
		var mx core.MaX
		if err := json.Unmarshal([]byte(payload), &mx); err != nil {
			return fmt.Errorf("failed to unmarshal maX: %w", err)
		}
		mx.Data = []interface{}{mx.Ts, mx.AvgVal}
		cr.MaXProcessChan <- &mx

	case "tickerInfo":
		var ti core.TickerInfo
		if err := json.Unmarshal([]byte(payload), &ti); err != nil {
			return fmt.Errorf("failed to unmarshal tickerInfo: %w", err)
		}
		cr.TickerInforocessChan <- &ti
	}

	return nil
}

func LoopMakeMaX(cr *core.Core) {
	for {
		cd := <-cr.MakeMaXsChan
		go func(cad *core.Candle) {
			//当一个candle的多个时间点的数据几乎同时到达时，顺序无法保证，制作maX会因为中间缺失数据而计算错，因此，等待一秒钟等数据都全了再算
			// sz := utils.ShaiziInt(1500) + 500
			time.Sleep(time.Duration(1500) * time.Millisecond)
			err, ct := MakeMaX(cr, cad, 7)
			if err != nil {
				logrus.Warn(GetFuncName(), " ma7 err:", err, " ct:", ct, " cd.InstID:", cd.InstID, " cd.Period:", cd.Period, " cd.Timestamp: ", cd.Timestamp, " cd.Data[0]: ", cd.Data[0])
			}
			//TODO 这个思路不错，单行不通，远程redis禁不住这么频繁的请求
			// cd.InvokeRestQFromRemote(cr, ct)
		}(cd)
		go func(cad *core.Candle) {
			//当一个candle的多个时间点的数据几乎同时到达时，顺序无法保证，制作maX会因为中间缺失数据而计算错，因此，等待一秒钟等数据都全了再算
			// sz := utils.ShaiziInt(2000) + 500
			time.Sleep(time.Duration(1600) * time.Millisecond)
			err, ct := MakeMaX(cr, cad, 30)
			if err != nil {
				logrus.Warn(GetFuncName(), " ma30 err:", err, " ct:", ct, " cd.InstID:", cd.InstID, " cd.Period:", cd.Period, " cd.Timestamp: ", cd.Timestamp)
			}
			// cd.InvokeRestQFromRemote(cr, ct)
		}(cd)
		go func(cad *core.Candle) {
			time.Sleep(time.Duration(1700) * time.Millisecond)
			err, ct := MakeRsi(cr, cad, 14, true)
			logrus.Warn(GetFuncName(), " rsi14 err:", err, " ct:", ct, " cd.InstID:", cd.InstID, " cd.Period:", cd.Period)
		}(cd)
		go func(cad *core.Candle) {
			time.Sleep(time.Duration(1700) * time.Millisecond)
			err, ct := MakeRsi(cr, cad, 12, false)
			logrus.Warn(GetFuncName(), " rsi12 err:", err, " ct:", ct, " cd.InstID:", cd.InstID, " cd.Period:", cd.Period)
		}(cd)
		go func(cad *core.Candle) {
			time.Sleep(time.Duration(1700) * time.Millisecond)
			err, ct := MakeRsi(cr, cad, 24, false)
			logrus.Warn(GetFuncName(), " rsi24 err:", err, " ct:", ct, " cd.InstID:", cd.InstID, " cd.Period:", cd.Period)
		}(cd)
		// TODO TODO 这地方不能加延时，否则makeMax处理不过来，多的就丢弃了，造成maX的sortedSet比candle的短很多。后面所有依赖的逻辑都受影响.
		// time.Sleep(300 * time.Millisecond)
	}
}

func InvokeCandle(cr *core.Core, candleName string, period string, from int64, to int64) error {
	// 计算from到to之间的时间差(毫秒)
	timeDiff := to - from

	// 根据period计算每个周期的毫秒数
	periodMinutes, err := cr.PeriodToMinutes(period)
	if err != nil {
		return fmt.Errorf("failed to convert period to minutes: %v", err)
	}
	periodMs := periodMinutes * 60 * 1000 // 转换成毫秒

	// 计算需要多少个周期的数据
	candleCount := int(timeDiff / periodMs)
	if candleCount <= 0 {
		return fmt.Errorf("invalid time range: from %d to %d", from, to)
	}

	// 限制最大请求数量,避免请求过大
	if candleCount > 100 {
		candleCount = 100
	}

	restQ := core.RestQueue{
		InstId: candleName,
		Bar:    period,
		WithWs: false,
		Limit:  strconv.Itoa(candleCount), // 动态计算limit
		After:  from,
	}

	js, err := json.Marshal(restQ)
	if err != nil {
		return fmt.Errorf("failed to marshal RestQueue: %v", err)
	}

	cli := cr.RedisLocalCli
	_, err = cli.LPush("restQueue", js).Result()
	if err != nil {
		return fmt.Errorf("failed to push to redis: %v", err)
	}

	return err
}

// setName := "candle" + period + "|" + instId + "|sortedSet"
// count: 倒推多少个周期开始拿数据
// from: 倒推的起始时间点
// ctype: candle或者maX
//
// GetRangeCandleSortedSet 从 Redis 的有序集合中获取指定时间范围内的蜡烛图数据
// 参数：
//   - cr: 核心对象，包含 Redis 连接等配置
//   - setName: Redis 有序集合的名称，格式为 "candle{period}|{instId}|sortedSet"
//   - count: 需要获取的蜡烛图数量
//   - from: 时间范围的结束时间点
//
// 返回值：
//   - *core.CandleList: 获取到的蜡烛图列表
//   - error: 如果获取过程中出现错误则返回错误信息
func GetRangeCandleSortedSet(cr *core.Core, setName string, count int, from time.Time) (*core.CandleList, error) {
	cdl := core.CandleList{}

	// 解析 setName 获取周期和交易对信息
	// setName 格式示例："candle1m|BTC-USDT|sortedSet"
	ary1 := strings.Split(setName, "|")
	ary2 := []string{}
	period := ""
	ary2 = strings.Split(ary1[0], "candle")
	period = ary2[1] // 获取周期，如 "1m"

	// 将周期转换为分钟数
	dui, err := cr.PeriodToMinutes(period)
	if err != nil {
		return nil, err
	}

	// 将时间转换为毫秒时间戳
	fromt := from.UnixMilli()
	nw := time.Now().UnixMilli()

	// 检查时间是否合理，防止时间戳错误
	if fromt > nw*2 {
		err := errors.New("时间错了需要debug")
		logrus.Warning(err.Error())
		return nil, err
	}
	// 计算时间范围
	froms := strconv.FormatInt(fromt, 10)   // 结束时间
	sti := fromt - dui*int64(count)*60*1000 // 开始时间 = 结束时间 - (周期 * 数量)
	sts := strconv.FormatInt(sti, 10)       // 开始时间字符串

	// 构建 Redis ZRangeBy 查询参数
	opt := redis.ZRangeBy{
		Min:   sts,          // 最小时间戳
		Max:   froms,        // 最大时间戳
		Count: int64(count), // 最大数量
	}
	ary := []string{}

	// 清理过期数据
	extt, err := cr.GetExpiration(period) // 获取过期时间
	ot := time.Now().Add(extt * -1)       // 计算过期时间点
	oti := ot.UnixMilli()                 // 转换为毫秒时间戳
	cli := cr.RedisLocalCli

	// 清理过期数据
	cli.LTrim(setName, 0, oti)                                                         // 修剪列表
	cunt, _ := cli.ZRemRangeByScore(setName, "0", strconv.FormatInt(oti, 10)).Result() // 移除过期数据
	if cunt > 0 {
		logrus.Warning("移出过期的引用数量：setName: ", setName, " cunt: ", cunt, " , ZRemRangeByScore ", setName, 0, strconv.FormatInt(oti, 10))
	}
	// 从 Redis 有序集合中获取数据
	logrus.Info("ZRevRangeByScore in GetRangeCandleSortedSet: setName:", setName, " opt:", opt)
	ary, err = cli.ZRevRangeByScore(setName, opt).Result() // 按分数范围获取数据
	if err != nil {
		return &cdl, err
	}
	keyAry, err := cli.MGet(ary...).Result()
	if err != nil || len(keyAry) == 0 {
		logrus.Warning("no record with cmd:  ZRevRangeByScore ", "setName: ", setName, " from: ", froms, " sts: ", sts, " err:", err.Error())
		logrus.Warning("zrev lens of ary: lens: ", len(ary), "GetRangeSortedSet ZRevRangeByScore:", "setName:", setName, " opt.Max:", opt.Max, " opt.Min:", opt.Min)
		// go func() {
		// 	parts := strings.Split(setName, "|")
		// 	instId := parts[1]
		// 	period, _ := extractString(setName)
		// 	InvokeCandle(cr, instId, period, fromt, sti)
		// }()
		return &cdl, err
	}
	// 解析并处理获取到的蜡烛图数据
	for _, str := range keyAry {
		if str == nil {
			continue
		}
		cd := core.Candle{}
		// 反序列化 JSON 数据
		err := json.Unmarshal([]byte(str.(string)), &cd)
		if err != nil {
			logrus.Warn(GetFuncName(), err, str.(string))
		}

		// 检查时间戳是否在指定范围内
		tmi := ToInt64(cd.Data[0])
		tm := time.UnixMilli(tmi)
		if tm.Sub(from) > 0 {
			break
		}

		// 将有效的蜡烛图数据添加到列表中
		cdl.List = append(cdl.List, &cd)
	}

	// 设置返回的蜡烛图数量
	cdl.Count = count
	return &cdl, nil
}

func extractString(input string) (string, error) {
	// 定位关键词 maX 或 candle
	var prefix string
	if strings.HasPrefix(input, "maX") {
		prefix = "maX"
	} else if strings.HasPrefix(input, "candle") {
		prefix = "candle"
	} else {
		return "", fmt.Errorf("input does not start with 'maX' or 'candle'")
	}

	// 去掉前缀部分
	remaining := strings.TrimPrefix(input, prefix)

	// 找到第一个竖线的位置
	pipeIndex := strings.Index(remaining, "|")
	if pipeIndex == -1 {
		return "", fmt.Errorf("no '|' found in the input")
	}

	// 返回竖线之前的部分
	return remaining[:pipeIndex], nil
}

//	func GetExpiration(cr *core.Core, per string) (time.Duration, error) {
//		if len(per) == 0 {
//			erstr := fmt.Sprint("period没有设置")
//			logrus.Warn(erstr)
//			err := errors.New(erstr)
//			return 0, err
//		}
//		exp, err := cr.PeriodToMinutes(per)
//		dur := time.Duration(exp*319) * time.Minute
//		return dur, err
//	}
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
		err = errors.New("sortedSet长度不足, 实际长度：" + ToString(len(cdl.List)) + "需要长度：" + ToString(count) + "的2倍, 无法进行rsi计算," + " setName:" + setName + ", fromTime: " + ToString(tsi))
		return err, 0
	}
	cdl.RecursiveBubbleS(len(cdl.List), "asc")
	closeList := []float64{}
	// ll := len(cdl.List)
	// fmt.Println("candleList len:", ll)
	for _, v := range cdl.List {
		// fmt.Println("candle in list", ll, k, v)
		closeList = append(closeList, ToFloat64(v.Data[4]))
	}
	rsiList, err := CalculateRSI(closeList, count)
	if err != nil {
		logrus.Error("Error calculating RSI:", err)
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

	// fmt.Println("will send rsi")
	go func() {
		// fmt.Println("make a rsi")
		cr.RsiProcessChan <- &rsi
	}()
	if !makeStock {
		return nil, 0
	}

	percentK, percentD, err := CalculateStochRSI(rsiList, count, 3, 3)

	if err != nil {
		logrus.Error("Error calculating StochRSI:", err)
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

	// fmt.Println("will send stockrsi")
	go func() {
		// fmt.Println("make a stockrsi")
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
		// to, _ := cr.PeriodToMinutes(cl.Period)
		// to = tsi + to*ToInt64(count)
		// InvokeCandle(cr, cl.InstID, cl.Period, tsi, to)
		return err, 0
	}

	fmt.Println("makeMaX: list: ", "instId: ", cl.InstID, "cl.Period: ", cl.Period, " lastTime:", lastTime, " count: ", count)
	amountLast := float64(0)
	ct := float64(0)
	// fmt.Println("makeMax len of GetLastCandleListOfCoin list: ", len(cdl.List), "makeMax err of GetLastCandleListOfCoin: ", err)
	if len(cdl.List) == 0 {
		return err, 0
	}
	// ljs, _ := json.Marshal(cdl.List)
	// fmt.Println("makeMax: ljs: ", string(ljs))
	if len(cdl.List) < count {
		err := errors.New("由于sortedSet容量有限，没有足够的元素数量来计算 maX, setName: " + setName + " ct: " + ToString(ct) + "count: " + ToString(count))
		return err, int(float64(count) - ct)
	}
	for _, v := range cdl.List {
		curLast, err := strconv.ParseFloat(v.Data[4].(string), 64)
		if err != nil {
			logrus.Warn("strconv.ParseFloat err:", err)
			continue
		}
		if curLast > 0 {
			ct++
		} else {
			logrus.Warn("strconv.ParseFloat curLast:", curLast)
		}
		amountLast += curLast
		//----------------------------------------------
	}
	avgLast := amountLast / ct
	if float64(ct) < float64(count) {
		err := errors.New("no enough source to calculate maX, setName: " + setName + " ct: " + ToString(ct) + "count: " + ToString(count))
		return err, int(float64(count) - ct)
		// fmt.Println("makeMax err: 没有足够的数据进行计算ma", "candle:", cl, "counts:", count, "ct:", ct, "avgLast: ")
	} else {
		// fmt.Println("makeMax keyName: ma", count, keyName, " avgLast: ", avgLast, "ts: ", tsi, "ct: ", ct, "ots: ", ots, "candle: ", string(cjs))

	}
	tm, _ := core.Int64ToTime(tsi)
	logrus.Debug("max tm:", tm)
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
		logrus.Debug("candle in process: ", cd)
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

		// fmt.Println("makeSoftCandles for: ", cd1)
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
		logrus.Debug("rsi recieved:", rsi)
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
		logrus.Debug("stockrsi recieved:", srsi)
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
