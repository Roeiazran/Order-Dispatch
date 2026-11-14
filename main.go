package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Config holds all the parsed command-line arguments.
type Config struct {
	N           int
	Restaurants int
	Zones       int
	SeedA       int64
	SeedB       int64
	Tokens      []int
}

// parseFlags reads the command-line arguments and returns a Config.
func parseFlags() (Config, error) {
	n := flag.Int("n", 0, "total number of orders")
	restaurants := flag.Int("restaurants", 0, "number of restaurants (producers)")
	zones := flag.Int("zones", 0, "number of delivery zones")
	seedA := flag.Int64("seedA", 0, "seed for order generation (which zone each order goes to)")
	seedB := flag.Int64("seedB", 0, "seed for per-zone processing delays")
	tokensStr := flag.String("tokens", "", "max concurrent orders per zone, comma separated (one value per zone)")
	flag.Parse()

	var tokens []int
	for _, s := range strings.Split(*tokensStr, ",") {
		val, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return Config{}, fmt.Errorf("invalid token value %q", s)
		}
		tokens = append(tokens, val)
	}

	cfg := Config{
		N:           *n,
		Restaurants: *restaurants,
		Zones:       *zones,
		SeedA:       *seedA,
		SeedB:       *seedB,
		Tokens:      tokens,
	}
	return cfg, cfg.Validate()
}

// Validate rejects configurations that would crash or deadlock the pipeline.
func (c Config) Validate() error {
	switch {
	case c.N < 0:
		return errors.New("-n must be >= 0")
	case c.Restaurants < 1:
		return errors.New("-restaurants must be >= 1")
	case c.Zones < 1:
		return errors.New("-zones must be >= 1")
	case len(c.Tokens) != c.Zones:
		return fmt.Errorf("-tokens has %d values but -zones is %d", len(c.Tokens), c.Zones)
	}
	for z, t := range c.Tokens {
		if t < 1 {
			return fmt.Errorf("token count for zone %d must be >= 1 (got %d)", z, t)
		}
	}
	return nil
}

// startZones creates each zone's input channel and token semaphore, then
// starts one manager goroutine per zone.
func startZones(cfg Config, eventsChan chan<- Event) ([]chan Order, *sync.WaitGroup) {
	zoneChans := make([]chan Order, cfg.Zones)
	zonesWg := &sync.WaitGroup{}

	for z := 0; z < cfg.Zones; z++ {
		zoneChans[z] = make(chan Order)

		limit := cfg.Tokens[z]
		tokens := make(chan struct{}, limit)
		for i := 0; i < limit; i++ {
			tokens <- struct{}{}
		}

		zonesWg.Add(1) // released by the manager when its channel is drained
		go zoneManager(z, cfg.SeedB, zoneChans[z], tokens, eventsChan, zonesWg)
	}

	return zoneChans, zonesWg
}

// startRestaurants starts all producer goroutines and returns their WaitGroup.
func startRestaurants(cfg Config, ordersChan chan<- Order, eventsChan chan<- Event) *sync.WaitGroup {
	producersWg := &sync.WaitGroup{}

	for i := 0; i < cfg.Restaurants; i++ {
		producersWg.Add(1)
		go func(rid int) {
			defer producersWg.Done()
			restaurant(rid, cfg.N, cfg.Restaurants, cfg.Zones, cfg.SeedA, ordersChan, eventsChan)
		}(i)
	}

	return producersWg
}

// run wires up the whole pipeline and blocks until every order is completed:
//
//	restaurants -> dispatcher -> zone managers -> workers
//	     \______________\______________\____________\____> logger
func run(cfg Config, out io.Writer) {
	eventsChan := make(chan Event)
	var loggerWg sync.WaitGroup
	loggerWg.Add(1)
	go logger(out, eventsChan, &loggerWg)

	zoneChans, zonesWg := startZones(cfg, eventsChan)

	ordersChan := make(chan Order)
	producersWg := startRestaurants(cfg, ordersChan, eventsChan)

	// Close the orders channel once every restaurant has finished producing.
	go func() {
		producersWg.Wait()
		close(ordersChan)
	}()

	// Route orders until the channel closes; this then closes all zone channels.
	dispatcher(ordersChan, zoneChans, eventsChan)

	// Wait for the zone managers and all their in-flight workers.
	zonesWg.Wait()

	eventsChan <- Event{Kind: EventDone, OrderID: cfg.N}
	close(eventsChan)
	loggerWg.Wait()
}

func main() {
	cfg, err := parseFlags()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		flag.Usage()
		os.Exit(2)
	}
	run(cfg, os.Stdout)
}
