package module

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	simple "github.com/bitly/go-simplejson"
	"github.com/go-redis/redis"
	"github.com/phyer/core"
	"math"
	"math/rand"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	// "github.com/phyer/core/utils"
	logrus "github.com/sirupsen/logrus"
)

func NextPeriod(curPeriod string, idx int) string {
	list := []string{
		"1m",
		"3m",
		"5m",
		"15m",
		"30m",
		"1H",
		"2H",
		"4H",
		"6H",
		"12H",
		"1D",
		"2D",
		"5D",
	}
	nextPer := ""
	for i := 0; i < len(list)-1; i++ {
		if list[i] == curPeriod {
			nextPer = list[i+idx]
		}
	}
	return nextPer
}

func pow(x, n int) int {
	ret := 1 // 结果初始为0次方的值，整数0次方为1。如果是矩阵，则为单元矩阵。
	for n != 0 {
		if n%2 != 0 {
			ret = ret * x
		}
		n /= 2
		x = x * x
	}
	return ret
}

// 获取当前函数名字
func GetFuncName() string {
	pc := make([]uintptr, 1)
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	return f.Name()
}

func Md5V(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

func ToString(val interface{}) string {
	valstr := ""
	if reflect.TypeOf(val).Name() == "string" {
		valstr = val.(string)
	} else if reflect.TypeOf(val).Name() == "float64" {
		valstr = strconv.FormatFloat(val.(float64), 'f', 1, 64)
	} else if reflect.TypeOf(val).Name() == "int64" {
		valstr = strconv.FormatInt(val.(int64), 16)
	}
	return valstr
}

func ToInt64(val interface{}) int64 {
	vali := int64(0)
	if reflect.TypeOf(val).Name() == "string" {
		vali, _ = strconv.ParseInt(val.(string), 10, 64)
	} else if reflect.TypeOf(val).Name() == "float64" {
		vali = int64(val.(float64))
	}
	return vali
}
func ToFloat64(val interface{}) float64 {
	valf := float64(0)
	if reflect.TypeOf(val).Name() == "string" {
		valf, _ = strconv.ParseFloat(val.(string), 64)
	} else if reflect.TypeOf(val).Name() == "float64" {
		valf = val.(float64)
	} else if reflect.TypeOf(val).Name() == "int64" {
		valf = float64(val.(int64))
	}
	return valf
}
