package module

import (
	"encoding/json"
	"errors"
	"fmt"
	simple "github.com/bitly/go-simplejson"
	"github.com/go-redis/redis"
	"github.com/phyer/core"
	"math"
	"math/rand"
	"os"
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
