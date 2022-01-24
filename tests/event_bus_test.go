package tests

import (
	"context"
	"github.com/hlhgogo/eventbus"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	bus := eventbus.New()
	if bus == nil {
		t.Log("New EventBus not created!")
		t.Fail()
	}
}

func TestQuit(t *testing.T) {
	bus := eventbus.New()
	if bus == nil {
		t.Log("New EventBus not created!")
		t.Fail()
	}

	bus.Quit()
}

func TestSubscribe(t *testing.T) {
	bus := eventbus.New()
	if bus == nil {
		t.Log("New EventBus not created!")
		t.Fail()
	}
	bus.SubscribeAsync(NewEventBusCase(), true)
	time.Sleep(1 * time.Second) // 注册为异步操作
	params := "eventBus"
	bus.Publish("runCase", context.TODO(), params)
	time.Sleep(2 * time.Second) // 消费也为异步操作
}
