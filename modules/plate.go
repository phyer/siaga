package module

import (
	"encoding/json"
	// "errors"
	"fmt"
	"github.com/phyer/core"
	// "os"
	// "strconv"
	// "strings"
	// "sync"
	// "time"
	//
	// simple "github.com/bitly/go-simplejson"
	// "github.com/go-redis/redis"
	// "github.com/phyer/core/utils"
	logrus "github.com/sirupsen/logrus"
)

// TODO 从redis里读出来已经存储的plate，如果不存在就创建一个新的
// LoadPlate 加载或创建指定Instrument的Plate
// 1. 尝试从Redis加载已存储的Plate
// 2. 如果Redis中不存在，则初始化一个新的Plate
// 3. 根据配置创建所有需要的Coaster
// 参数:
//   - cr: 核心上下文对象
//   - instId: Instrument ID
//
// 返回值:
//   - *core.Plate: 加载或新建的Plate对象
//   - error: 操作过程中发生的错误
func LoadPlate(cr *core.Core, instId string) (*core.Plate, error) {
	// 初始化一个空的Plate对象
	pl := core.Plate{}

	// 构造Redis中存储Plate数据的key，格式为"instId|plate"
	plateName := instId + "|plate"

	// 尝试从Redis获取Plate数据
	str, err := cr.RedisLocalCli.Get(plateName).Result()

	// 如果Redis中存在数据且没有错误
	if err == nil && str != "" {
		// 将JSON字符串反序列化为Plate对象
		if err := json.Unmarshal([]byte(str), &pl); err != nil {
			// 反序列化失败时初始化一个新的Plate
			logrus.Warnf("failed to unmarshal plate data from redis, init new plate: %v", err)
			pl.Init(instId)
		}
		// 返回从Redis加载的Plate对象
		return &pl, nil
	}

	// Redis不可用或数据不存在时，初始化一个新的Plate
	pl.Init(instId)

	// 从配置中获取candleDimentions配置项
	prs := cr.Cfg.Config.Get("candleDimentions")

	// 检查配置项是否存在
	if prs == nil || prs.Interface() == nil {
		return nil, fmt.Errorf("candleDimentions config not found")
	}

	// 将配置项转换为数组
	periods := prs.MustArray()

	// 遍历所有周期配置
	for _, v := range periods {
		// 将interface{}类型转换为string
		period, ok := v.(string)
		// 检查周期字符串是否有效
		if !ok || period == "" {
			continue // 跳过无效的周期配置
		}
		// 为每个周期创建Coaster
		if err := pl.MakeCoaster(cr, period); err != nil {
			// 如果创建失败，返回错误
			return nil, fmt.Errorf("failed to create coaster for period %s: %v", period, err)
		}
	}

	// 将新创建的Plate保存到Redis（可选操作）
	if err := savePlateToRedis(cr, plateName, &pl); err != nil {
		// 保存失败时记录警告日志
		logrus.Warnf("failed to save plate to redis: %v", err)
	}

	// 返回创建好的Plate对象
	return &pl, nil
}

func savePlateToRedis(cr *core.Core, key string, pl *core.Plate) error {
	data, err := json.Marshal(pl)
	if err != nil {
		return err
	}
	return cr.RedisLocalCli.Set(key, data, 0).Err()
}
