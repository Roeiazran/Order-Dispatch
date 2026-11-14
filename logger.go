package main

import (
	"fmt"
	"io"
	"sync"
)

// logger is the only goroutine that writes output. Every other goroutine sends
// events here, so lines are never interleaved.
func logger(out io.Writer, events <-chan Event, wg *sync.WaitGroup) {
	defer wg.Done()

	for event := range events {
		switch event.Kind {
		case EventCreated:
			fmt.Fprintf(out, "CREATED order=%d restaurant=%d type=%d\n", event.OrderID, event.RestaurantID, event.Zone)
		case EventDispatched:
			fmt.Fprintf(out, "DISPATCHED order=%d zone=%d\n", event.OrderID, event.Zone)
		case EventStarted:
			fmt.Fprintf(out, "STARTED order=%d zone=%d\n", event.OrderID, event.Zone)
		case EventCompleted:
			fmt.Fprintf(out, "COMPLETED order=%d zone=%d\n", event.OrderID, event.Zone)
		case EventDone:
			fmt.Fprintf(out, "DONE total=%d\n", event.OrderID)
		default:
			fmt.Fprintf(out, "UNKNOWN EVENT: %v\n", event)
		}
	}
}
