package eventbus

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// Bus 总线
type Bus interface {
	PublishWithCtx(ctx context.Context, topic string, args ...interface{})
	Publish(topic string, args ...interface{})
	Subscribe(observer interface{})
	SubscribeAsync(observer interface{}, transactional bool)
	Unsubscribe(observer interface{})
	Quit()
	Wait()
}

// Topic ...
type Topic interface {
	Topic() string
}

// Function ...
type Function interface {
	Function() string
}

// EventBus 事件总线
type EventBus struct {
	handlers    *sync.Map
	wg          *sync.WaitGroup
	lock        *sync.Mutex
	pool        *Pool
	subscribeWg *sync.WaitGroup
}

// SubscribeAsync  订阅-异步
func (bus *EventBus) SubscribeAsync(observer interface{}, transactional bool) {
	bus.subscribeWg.Add(1)
	go func() {
		defer bus.subscribeWg.Done()
		bus.register(observer, false, true, transactional)
	}()

}

// Subscribe 订阅-同步
func (bus *EventBus) Subscribe(observer interface{}) {
	bus.subscribeWg.Add(1)
	go func() {
		defer bus.subscribeWg.Done()
		bus.register(observer, false, false, false)
	}()
}

// Unsubscribe 删除订阅
func (bus *EventBus) Unsubscribe(observer interface{}) {
	topic, t, fn, _, _ := bus.checkObserver(observer)
	for i := range topic[:] {
		function, ok := t.MethodByName(fn)
		if !ok {
			continue
		}
		bus.removeHandler(topic[i], bus.findHandlerIdx(topic[i], function.Func))
	}

}

// PublishWithCtx 推送
func (bus *EventBus) PublishWithCtx(ctx context.Context, topic string, args ...interface{}) {
	eventCtx := DetachCtx(ctx)
	params := make([]interface{}, 0, len(args)+1)
	params = append(append(params, eventCtx), args...)
	bus.Publish(topic, params...)
}

// publish 推送
func (bus *EventBus) Publish(topic string, args ...interface{}) {
	if handlerInterface, ok := bus.handlers.Load(topic); ok {
		handlers := handlerInterface.([]*EventHandler)
		if len(handlers) == 0 {
			return
		}
		for i, handler := range handlers {
			if handler.flagOnce {
				bus.removeHandler(topic, i)
			}
			if !handler.async {
				bus.doPublish(handler, args...)
			} else {
				bus.wg.Add(1)
				if handler.transactional {
					handler.lock.Lock()
				}
				t := &task{
					f:            bus.doPublishAsync,
					eventHandler: handlers[i],
					args:         args,
				}
				if err := bus.pool.Submit(t); err != nil {
					fmt.Errorf("eventbus pool submit topic [%s] callBack [%s] err[%v] ", topic, t.eventHandler.callBack.String(), err)
				}
			}
		}
	}
}

func (bus *EventBus) doPublish(handler *EventHandler, args ...interface{}) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Errorf("eventbus, topic:%s callBack: %s catch err:%s", handler.topic, handler.callBack.String(), err)
		}
	}()
	passedArguments := bus.setUpPublish(handler, args...)
	handler.callBack.Call(passedArguments)
}

func (bus *EventBus) doPublishAsync(handler *EventHandler, args ...interface{}) {
	defer bus.wg.Done()
	defer func() {
		if err := recover(); err != nil {
			fmt.Errorf("eventbus, topic:%s callBack: %s catch err:%s", handler.topic, handler.callBack.String(), err)
		}
	}()
	if handler.transactional {
		defer handler.lock.Unlock()
	}

	bus.doPublish(handler, args...)

}

func (bus *EventBus) setUpPublish(callback *EventHandler, args ...interface{}) []reflect.Value {
	funcType := callback.callBack.Type()
	passedArguments := make([]reflect.Value, 0, len(args)+1)
	passedArguments = append(passedArguments, reflect.ValueOf(callback.observer))
	for i := range args[:] {
		if args[i] == nil {
			passedArguments = append(passedArguments, reflect.New(funcType.In(i)).Elem())
		} else {
			passedArguments = append(passedArguments, reflect.ValueOf(args[i]))
		}
	}

	return passedArguments
}

// Wait 等待Handler注册完成
func (bus *EventBus) Wait() {
	bus.subscribeWg.Wait()
}

// Quit 退出
func (bus *EventBus) Quit() {
	bus.handlers.Range(func(topic, value interface{}) bool {
		handlers := value.([]*EventHandler)
		for i := range handlers[:] {
			handlers[i].quitCallBack.Call([]reflect.Value{reflect.ValueOf(handlers[i].observer)})
			// _ = bus.Unsubscribe(handlers[i].observer)
		}

		return true
	})
}

// register 注册
func (bus *EventBus) register(observer interface{}, flagOnce, async, transactional bool) {
	topic, t, fn, quit, ext := bus.checkObserver(observer)
	for i := range topic[:] {
		function, ok := t.MethodByName(fn)
		if !ok {
			continue
		}
		quit, ok := t.MethodByName(quit)
		if ok {
			_ = bus.doSubscribe(topic[i], &EventHandler{
				t, observer, function.Func, quit.Func, flagOnce, async, transactional, new(sync.Mutex), topic[i], ext,
			})
		}
	}
}

// doSubscribe 处理订阅逻辑
func (bus *EventBus) doSubscribe(topic string, handler *EventHandler) error {
	bus.lock.Lock()
	defer bus.lock.Unlock()
	handlerInterface, _ := bus.handlers.LoadOrStore(topic, make([]*EventHandler, 0))
	handlers := handlerInterface.([]*EventHandler)
	for i := range handlers[:] {
		if handlers[i].observerType.Elem() == handler.observerType.Elem() {
			return nil
		}
	}
	bus.handlers.Store(topic, append(handlers, handler))
	return nil
}

// checkObserver 验证observer格式正确
func (bus *EventBus) checkObserver(observer interface{}) ([]string, reflect.Type, string, string, string) {
	var t reflect.Type
	var fn string
	var quit string
	var ok bool
	var ext string

	t = reflect.TypeOf(observer)
	if t.Elem().Kind() != reflect.Struct {
		panic(fmt.Sprintf("%s is not of type reflect.Struct", t.Kind()))
	}
	fnField, _ := t.Elem().FieldByName("event")
	if fnField.Tag == "" {
		panic(fmt.Sprintf("%v has no field or no fn field", fnField))
	}

	fn, _ = fnField.Tag.Lookup("subscribe")
	obsFunc, ok := observer.(Function)
	if ok {
		fn = obsFunc.Function()
	}
	if fn == "" {
		panic("subscribe tag doesn't exist or empty")
	}
	quit, ok = fnField.Tag.Lookup("quit")
	if !ok || quit == "" {
		panic("quit tag doesn't exist or empty")
	}

	topics, _ := fnField.Tag.Lookup("topic")
	obsTopic, ok := observer.(Topic)
	if ok {
		topics = obsTopic.Topic()
	}
	if topics == "" {
		panic("topic tag doesn't exist or empty")
	}

	topic := strings.Split(topics, ",")
	ext, _ = fnField.Tag.Lookup("ext")
	return topic, t, fn, quit, ext
}

func (bus *EventBus) removeHandler(topic string, idx int) {
	handlerInterface, ok := bus.handlers.Load(topic)
	if !ok {
		return
	}
	handlers := handlerInterface.([]*EventHandler)
	l := len(handlers)

	if !(0 <= idx && idx < l) {
		return
	}
	handlers = append(handlers[:idx], handlers[idx+1:]...)
	if len(handlers) > 0 {
		bus.handlers.Store(topic, handlers)
	} else {
		bus.handlers.Delete(topic)
	}
}

func (bus *EventBus) findHandlerIdx(topic string, callback reflect.Value) int {
	if handlerInterface, ok := bus.handlers.Load(topic); ok {
		handlers := handlerInterface.([]*EventHandler)
		for i := range handlers[:] {
			if handlers[i].callBack.Type() == callback.Type() &&
				handlers[i].callBack.Pointer() == callback.Pointer() {
				return i
			}
		}
	}
	return -1
}

// New ...
func New() Bus {
	pool, err := NewPool(DefaultPoolSize)
	if err != nil {
		panic(err)
	}
	return &EventBus{
		handlers:    &sync.Map{},
		wg:          &sync.WaitGroup{},
		lock:        &sync.Mutex{},
		pool:        pool,
		subscribeWg: &sync.WaitGroup{},
	}
}
