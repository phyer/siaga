package module

import (
	// "encoding/json"
	// "errors"
	// "fmt"
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
	// logrus "github.com/sirupsen/logrus"
)

type MyRsi struct {
	core.Rsi
}

func (mrsi *MyRsi) Process(cr *core.Core) {
	rsi := mrsi.Rsi
	go func() {
		rsi.PushToWriteLogChan(cr)
	}()
}
