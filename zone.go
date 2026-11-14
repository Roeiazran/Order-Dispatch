package main

import (
	"math/rand"
	"sync"
	"time"
)

// zoneManager handles all orders routed to a specific zone.
//
// The tokens channel is a counting semaphore: a worker goroutine may only start
// once it takes a token, which caps how many orders a zone processes at once.
//
// The caller must call wg.Add(1) for the manager itself before starting it.
// Because the manager holds its own slot in the WaitGroup until its input
// channel is closed and drained, the counter can never reach zero while an
// order is still waiting for a token, so wg.Wait() cannot return early.
func zoneManager(zoneID int, seedB int64, zoneChan <-chan Order, tokens chan struct{}, eventsChan chan<- Event, wg *sync.WaitGroup) {
	defer wg.Done()

	rng := rand.New(rand.NewSource(seedB + int64(zoneID)*2000003))

	for order := range zoneChan {
		delay := rng.Intn(21) // simulated preparation time in ms, 0–20
		<-tokens
		wg.Add(1)

		go func(o Order, d int) {
			defer wg.Done()
			defer func() { tokens <- struct{}{} }()

			eventsChan <- Event{
				Kind:         EventStarted,
				RestaurantID: o.RestaurantID,
				OrderID:      o.OrderID,
				Zone:         zoneID,
			}

			time.Sleep(time.Duration(d) * time.Millisecond)

			eventsChan <- Event{
				Kind:         EventCompleted,
				RestaurantID: o.RestaurantID,
				OrderID:      o.OrderID,
				Zone:         zoneID,
			}
		}(order, delay)
	}
}
