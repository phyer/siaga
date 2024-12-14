package main

import (
	"fmt"
	"os"
	"time"

	"github.com/phyer/core"
	md "github.com/phyer/siaga/module"
	"github.com/sirupsen/logrus"
)

func main() {
	cr := core.Core{}
	cr.Init()

	cli, err := cr.GetRedisCli()
	cr.RedisRemoteCli = cli

	allCandleAdd := core.ALLCANDLES_PUBLISH
	allMaXAdd := core.ALLMAX_PUBLISH
	// 目前只有phyer里部署的tunas会发布tickerInfo信息
	go func(vv *core.RedisConfig) {
		allowed := os.Getenv("SIAGA_ACCEPTTICKER") == "true"
		if !allowed {
			return
		}
		md.LoopSubscribe(&cr, core.TICKERINFO_PUBLISH, vv)
	}(v)
	time.Sleep(5 * time.Second)
	go func(vv *core.RedisConfig) {
		allowed := os.Getenv("SIAGA_ACCEPTCANDLE") == "true"
		if !allowed {
			return
		}
		// core.LoopSubscribe(&cr, allCandleAdd, vv)
	}(v)
	go func(vv *core.RedisConfig) {
		allowed := os.Getenv("SIAGA_ACCEPTMAX") == "true"
		if !allowed {
			return
		}
		core.LoopSubscribe(&cr, allMaXAdd, vv)
	}(v)
	go func(vv *core.RedisConfig) {
		allowed := os.Getenv("SIAGA_ACCEPTSERIES") == "true"
		if !allowed {
			return
		}
		core.LoopSubscribe(&cr, core.ALLSERIESINFO_PUBLISH, vv)
	}(v)

	go func() {
		core.LoopMakeMaX(&cr)
	}()
	go func() {
		core.LoopCheckRemoteRedis(&cr)
	}()
	go func() {
		core.CandlesProcess(&cr)
	}()
	go func() {
		core.MaXsProcess(&cr)
	}()
	go func() {
		core.TickerInfoProcess(&cr)
	}()
	go func() {
		core.CoasterProcess(&cr)
	}()
	go func() {
		core.SeriesProcess(&cr)
	}()
	go func() {
		core.SegmentItemProcess(&cr)
	}()
	go func() {
		core.ShearForceProcess(&cr)
	}()
	go func() {
		core.WriteLogProcess(&cr)
	}()

	// ip := "0.0.0.0:6061"
	// if err := http.ListenAndServe(ip, nil); err != nil {
	// }
	// allMaxs: {1634413398759-0 map[ma7|candle5m|LUNA-USDT|key:{"ts":1634412300000,"value":36.906796182686605}]}
	// allCandles: {1634413398859-0 map[candle2H|XRP-USDT|key:{"channel":"candle2H","data":"eyJjIjoxLjExNzk1LCJmcm9tIjoicmVzdCIsImgiOjEuMTIyNzksImwiOjEuMTA4ODUsIm8iOjEuMTE3MzUsInRzIjoxNjM0MjkyMDAwMDAwLCJ2b2wiOjUwMDc5OTEuNDM5MDg1LCJ2b2xDY3kiOjU1OTE2MjUuNzI4NDc2fQ==","instId":"XRP-USDT"}]}

	time.Sleep(1000000 * time.Hour)
}
