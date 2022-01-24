package eventbus

import (
	"reflect"
	"sync"
)

// EventHandler EventHandler
type EventHandler struct {
	observerType  reflect.Type
	observer      interface{}
	callBack      reflect.Value
	quitCallBack  reflect.Value
	flagOnce      bool
	async         bool
	transactional bool
	lock          *sync.Mutex
	topic         string
	ext           string // 扩展信息
}
