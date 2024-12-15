package module

import (
	"encoding/json"
	// "errors"
	// "fmt"
	"github.com/phyer/core"
	// "os"
	// "strconv"
	// "strings"
	// // "sync"
	// "time"
	// //
	// // simple "github.com/bitly/go-simplejson"
	// "github.com/go-redis/redis"
	// // "github.com/phyer/core/utils"
	// logrus "github.com/sirupsen/logrus"
)

// TODO 从redis里读出来已经存储的plate，如果不存在就创建一个新的
func LoadPlate(cr *core.Core, instId string) (*core.Plate, error) {
	pl := core.Plate{}
	plateName := instId + "|plate"
	_, err := cr.RedisLocalCli.Exists().Result()
	if err == nil {
		str, _ := cr.RedisLocalCli.Get(plateName).Result()
		json.Unmarshal([]byte(str), &pl)
	} else {
		pl.Init(instId)
		prs := cr.Cfg.Config.Get("candleDimentions").MustArray()
		for _, v := range prs {
			pl.MakeCoaster(cr, v.(string))
		}
	}
	return &pl, nil
}
