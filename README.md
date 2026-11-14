# Concurrent Order Dispatch Simulator

A small Go program that simulates a food-delivery backend as a concurrent pipeline. Restaurants produce orders. A dispatcher routes each order to a delivery zone, and each zone processes orders in parallel up to a per-zone limit. Everything is written with goroutines and channels from the standard library, with no third-party dependencies.

It was built as a parallel-programming assignment. The goal was correct, deadlock-free coordination between many goroutines, with reproducible behaviour.

## Architecture

```
  restaurant 0 ─┐
  restaurant 1 ─┼──► ordersChan ──► dispatcher ──┬──► zone 0 manager ──► workers (≤ tokens[0] at once)
  restaurant R ─┘                                ├──► zone 1 manager ──► workers (≤ tokens[1] at once)
                                                 └──► zone Z manager ──► workers (≤ tokens[Z] at once)

        every stage ──► eventsChan ──► logger (the only goroutine that prints)
```

| Component | File | Role |
|---|---|---|
| Restaurants | [`restaurant.go`](restaurant.go) | `R` producer goroutines. Each one owns a contiguous slice of the `N` order IDs and picks a zone for each order with its own seeded random generator. |
| Dispatcher | [`dispatcher.go`](dispatcher.go) | Fan-in from all restaurants, fan-out to the zone channels. Closes every zone channel when input runs out. |
| Zone manager | [`zone.go`](zone.go) | One per zone. Takes a token from the zone's semaphore, then starts a worker goroutine that "prepares" the order (sleeps 0–20 ms) and returns the token. |
| Logger | [`logger.go`](logger.go) | Single consumer of all events, so output lines never interleave. |
| Wiring | [`main.go`](main.go) | Parses flags, validates them, starts the pipeline and shuts it down in order. |

## Concurrency techniques used

- **Producer / consumer pipeline** built from unbuffered channels.
- **Fan-in / fan-out:** many producers feed one dispatcher, which feeds many zones.
- **Counting semaphore** made from a buffered channel (`chan struct{}`) to cap how many orders each zone runs at once.
- **Ordered, graceful shutdown** through channel closing and `sync.WaitGroup`:
  restaurants finish → `ordersChan` closed → dispatcher closes zone channels → managers drain and wait for their workers → `DONE` is logged → `eventsChan` closed → the logger exits.
- **Deterministic randomness:** each goroutine has its own `rand.Rand` seeded from `seedA` / `seedB`, so the same seed always routes orders the same way, and no random generator is shared between goroutines.
- **A single writer for output:** only the logger goroutine prints, so there is no lock around stdout.

### A subtle bug worth mentioning

An earlier version called `wg.Add(1)` for each worker only *after* the zone manager got a token. If the dispatcher had already closed the channels while a manager was still waiting for that token, the WaitGroup counter could be 0. `main` would then stop waiting too early, close `eventsChan`, and the late worker would panic with `send on closed channel`. This happened in roughly 1 in 4 runs. The fix is to count each zone manager in the WaitGroup as well. The counter can't reach zero while any manager is still running, and calling `Add` while the counter is above zero is always safe.

## Usage

Requires Go 1.22+.

```bash
go run . -n 6 -restaurants 2 -zones 3 -seedA 4 -seedB 2 -tokens 2,1,1
```

| Flag | Meaning |
|---|---|
| `-n` | total number of orders |
| `-restaurants` | number of producer goroutines |
| `-zones` | number of delivery zones |
| `-tokens` | comma-separated max concurrent orders per zone (one value per zone) |
| `-seedA` | seed for order generation (which zone each order goes to) |
| `-seedB` | seed for per-zone processing delays |

Invalid input, such as a `-tokens` count that doesn't match `-zones` or a zone with 0 tokens (which would deadlock), is rejected with an error message.

### Example output

```
CREATED order=0 restaurant=0 type=1
CREATED order=3 restaurant=1 type=1
DISPATCHED order=0 zone=1
STARTED order=0 zone=1
COMPLETED order=0 zone=1
...
DISPATCHED order=5 zone=0
STARTED order=4 zone=1
STARTED order=5 zone=0
COMPLETED order=5 zone=0
COMPLETED order=4 zone=1
...
DONE total=6
```

Each order moves through `CREATED → DISPATCHED → STARTED → COMPLETED`. Different orders interleave freely, which shows the stages running in parallel.

## Testing

```bash
go test -race ./...
```

The tests run the whole pipeline on several configurations and parse its output to check that:

- every order goes through all four stages exactly once and in the right order,
- every order is handled by the zone it was created for,
- no zone ever has more orders running than its token limit,
- `DONE` is the last line,
- the same seed always gives the same routing,
- invalid configurations are rejected.

Because the tests run under the race detector, they also check for data races.
