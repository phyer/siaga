package main

import (
	// "fmt"
	"github.com/phyer/core"
	md "github.com/phyer/siaga/modules"
	logrus "github.com/sirupsen/logrus"
	"os"
	"sync"
	// "github.com/sirupsen/logrus"
)

func main() {
	cr := core.Core{}
	cr.Init()
	logrus.SetLevel(logrus.InfoLevel)
	cr.TickerInforocessChan = make(chan *core.TickerInfo)
	cr.CandlesProcessChan = make(chan *core.Candle)
	cr.MaXProcessChan = make(chan *core.MaX)
	cr.RsiProcessChan = make(chan *core.Rsi)
	cr.StockRsiProcessChan = make(chan *core.StockRsi)
	cr.MakeMaXsChan = make(chan *core.Candle)
	cr.WriteLogChan = make(chan *core.WriteLog)
	cr.Mu = &sync.Mutex{}
	cr.PlateMap = make(map[string]*core.Plate)
	cli, _ := cr.GetRedisLocalCli()
	cr.RedisRemoteCli = cli

	rdsLs, _ := md.GetRemoteRedisConfigList()
	// 目前只有phyer里部署的tunas会发布tickerInfo信息
	// fmt.Println("len of rdsLs: ", len(rdsLs))

	// 定义订阅配置
	subscriptions := []struct {
		envVar    string
		channel   string
		logPrefix string
	}{
		{"SIAGA_ACCEPTTICKER", core.TICKERINFO_PUBLISH, "TickerInfo"},
		{"SIAGA_ACCEPTCANDLE", core.ALLCANDLES_PUBLISH, "Candles"},
		{"SIAGA_ACCEPTMAX", core.ALLMAXES_PUBLISH, "Max"},
		{"SIAGA_ACCEPTSERIES", core.ALLSERIESINFO_PUBLISH, "Series"},
	}

	// 启动所有订阅
	for _, sub := range subscriptions {
		go func(s struct {
			envVar    string
			channel   string
			logPrefix string
		}) {
			if os.Getenv(s.envVar) != "true" {
				return
			}
			logrus.Infof("start subscribe %s: %s", s.logPrefix, s.channel)
			md.LoopSubscribe(&cr, s.channel, rdsLs[0])
		}(sub)
	}

	go func() {
		md.TickerInfoProcess(&cr)
	}()
	go func() {
		md.CandlesProcess(&cr)
	}()
	go func() {
		md.LoopMakeMaX(&cr)
	}()
	go func() {
		md.MaXsProcess(&cr)
	}()
	go func() {
		md.RsisProcess(&cr)
	}()
	go func() {
		md.StockRsisProcess(&cr)
	}()

	// 这些暂时不运行, 以后要不要运行再说
	// go func() {
	// 	core.CoasterProcess(&cr)
	// }()
	// go func() {
	// 	core.SeriesProcess(&cr)
	// }()
	// go func() {
	// 	core.SegmentItemProcess(&cr)
	// }()
	// go func() {
	// 	core.ShearForceProcess(&cr)
	// }()
	go func() {
		core.WriteLogProcess(&cr)
	}()

	// ip := "0.0.0.0:6061"
	// if err := http.ListenAndServe(ip, nil); err != nil {
	// }
	// allMaxs: {1634413398759-0 map[ma7|candle5m|LUNA-USDT|key:{"ts":1634412300000,"value":36.906796182686605}]}
	// allCandles: {1634413398859-0 map[candle2H|XRP-USDT|key:{"channel":"candle2H","data":"eyJjIjoxLjExNzk1LCJmcm9tIjoicmVzdCIsImgiOjEuMTIyNzksImwiOjEuMTA4ODUsIm8iOjEuMTE3MzUsInRzIjoxNjM0MjkyMDAwMDAwLCJ2b2wiOjUwMDc5OTEuNDM5MDg1LCJ2b2xDY3kiOjU1OTE2MjUuNzI4NDc2fQ==","instId":"XRP-USDT"}]}
	logrus.Info("siaga started")
	select {}
}
