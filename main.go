package main

import (
	"fmt"
	"os"

	"github.com/phyer/core"
	md "github.com/phyer/siaga/modules"
	// "github.com/sirupsen/logrus"
)

func main() {
	cr := core.Core{}
	cr.Init()

	cli, _ := cr.GetRedisLocalCli()
	cr.RedisRemoteCli = cli

	rdsLs, _ := md.GetRemoteRedisConfigList()
	// 目前只有phyer里部署的tunas会发布tickerInfo信息
	fmt.Println("len of rdsLs: ", len(rdsLs))

	// 订阅 redis TickerInfo
	go func(vv *core.RedisConfig) {
		allowed := os.Getenv("SIAGA_ACCEPTTICKER") == "true"
		if !allowed {
			return
		}
		fmt.Println("start subscribe core.TICKERINFO_PUBLISH")
		md.LoopSubscribe(&cr, core.TICKERINFO_PUBLISH, vv)
	}(rdsLs[0])

	// 订阅 redis Candles
	go func(vv *core.RedisConfig) {
		allowed := os.Getenv("SIAGA_ACCEPTCANDLE") == "true"
		if !allowed {
			return
		}
		fmt.Println("start subscribe core.TICKERINFO_PUBLISH")
		md.LoopSubscribe(&cr, core.ALLCANDLES_PUBLISH, vv)
	}(rdsLs[0])

	// 订阅 redis Max
	go func(vv *core.RedisConfig) {
		allowed := os.Getenv("SIAGA_ACCEPTMAX") == "true"
		if !allowed {
			return
		}
		md.LoopSubscribe(&cr, core.ALLMAXES_PUBLISH, vv)
	}(rdsLs[0])

	// 下面这个暂时不运行, 在环境变量里把它关掉
	go func(vv *core.RedisConfig) {
		allowed := os.Getenv("SIAGA_ACCEPTSERIES") == "true"
		if !allowed {
			return
		}
		md.LoopSubscribe(&cr, core.ALLSERIESINFO_PUBLISH, vv)
	}(rdsLs[0])

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
	fmt.Println("siaga started")
	select {}
}
