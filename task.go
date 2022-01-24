package eventbus

// 任务
type task struct {
	// 执行函数
	f func(handler *EventHandler, args ...interface{})
	// 处理
	eventHandler *EventHandler
	// 参数
	args []interface{}
}
