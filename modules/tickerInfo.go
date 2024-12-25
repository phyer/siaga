package module

import (
	// "errors"
	// "fmt"
	"github.com/phyer/core"
	logrus "github.com/sirupsen/logrus"
	// "math"
	"os"
	"strconv"
)

type MyTickerInfo struct {
	core.TickerInfo
}

func (mti *MyTickerInfo) Process(cr *core.Core) {
	ti := mti.TickerInfo
	go func() {
		// 无需发布出去, 下面内容注释 2024-12-15
		// tickerPush := os.Getenv("SIAGA_TICKERPUSH") == "true"
		// if !tickerPush {
		// 	return
		// }
		// err := AddToGeneralTickerChnl(ti)
		// if err != nil {
		// 	logrus.Warn("tickerPush err:", err, ti)
		// }
	}()
	go func() {
		torqueSorted := os.Getenv("SIAGA_TORQUESORTED") == "true"
		if !torqueSorted {
			return
		}
		// 这个功能貌似很有意思，以后可以考虑打开
		// ti.MakePriceSorted(cr, "2m")
		// ti.MakePriceSorted(cr, "4m")
		// ti.MakePriceSorted(cr, "8m")
		//用这个方法保证事件在每个周期只发生一次
	}()
	go func() {
		tickerToCandle := os.Getenv("SIAGA_TICKERTOCANDLE") == "true"
		if tickerToCandle {
			logrus.Debug("tickerToCandle: ", ti)
			cd := mti.ConvertToCandle(cr, "1m")
			cr.CandlesProcessChan <- cd
		}
	}()

	go func() {
		ti.SetToKey(cr)
		// 流出时间让下面的函数进行比对
		// time.Sleep(50 * time.Millisecond)
		// snapshot 先关掉 2024-12-15
		// ti.MakeSnapShot(cr)
	}()
}

// TODO 当前这个版本的实现里，开盘价，最高价，最低价都不重要，主要是收盘价，用来算ma7，ma30，这个就够了，以后需要的话，再细化
func (mti *MyTickerInfo) ConvertToCandle(cr *core.Core, period string) *core.Candle {
	ti := mti.TickerInfo
	hn := os.Getenv("HOSTNAME")
	lst := strconv.FormatFloat(ti.Last, 'f', -1, 64)
	tmi := ti.Ts
	tmi = tmi - tmi%60000
	cd := core.Candle{
		InstID: ti.InstID,
		Period: period,
		Data: []interface{}{
			strconv.FormatInt(tmi, 10), //开始时间，Unix时间戳的毫秒数格式，如 1597026383085
			"-1",                       //o String  开盘价格
			"-1",                       //h String  最高价格
			"-1",                       //l String  最低价格
			lst,                        //c String  收盘价格
			"-1",                       //c String  成交量
			"-1",                       //c String  成交美元数
		},
		From: "tickerInfo|" + hn,
	}

	return &cd
}
