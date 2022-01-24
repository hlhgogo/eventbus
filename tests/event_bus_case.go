package tests

import (
	"context"
	"fmt"
)

// EventBusCase ...
type EventBusCase struct {
	event interface{} `subscribe:"RunCase" topic:"runCase" quit:"Quit"`
}

// NewEventBusCase ...
func NewEventBusCase() *EventBusCase {
	return new(EventBusCase)
}

// RunCase ...
func (event *EventBusCase) RunCase(ctx context.Context, params string) {
	fmt.Printf("RunCase, Params: %s \n", params)
}

// Quit 退出
func (event *EventBusCase) Quit() {

}
