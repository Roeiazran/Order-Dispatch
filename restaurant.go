package main

import (
	"math/rand"
)

// restaurant produces its share of the N orders (a contiguous ID range) and
// sends them to the dispatcher. Each restaurant has its own seeded generator,
// so the zone chosen for each order is reproducible.
func restaurant(rid, N, R, Z int, seedA int64, ordersChan chan<- Order, eventsChan chan<- Event) {
	startID := (rid * N) / R
	endID := ((rid + 1) * N) / R

	rng := rand.New(rand.NewSource(seedA + int64(rid)*1000003))

	for i := startID; i < endID; i++ {
		zone := rng.Intn(Z)

		eventsChan <- Event{
			Kind:         EventCreated,
			RestaurantID: rid,
			OrderID:      i,
			Zone:         zone,
		}

		ordersChan <- Order{
			OrderID:      i,
			RestaurantID: rid,
			FoodType:     zone,
		}
	}
}
