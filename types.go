package main

// Order is a single food order produced by a restaurant.
type Order struct {
	OrderID      int
	RestaurantID int
	FoodType     int // also the ID of the zone that will handle the order
}

// EventKind identifies a step in an order's lifecycle.
type EventKind string

const (
	EventCreated    EventKind = "CREATED"
	EventDispatched EventKind = "DISPATCHED"
	EventStarted    EventKind = "STARTED"
	EventCompleted  EventKind = "COMPLETED"
	EventDone       EventKind = "DONE"
)

// Event is sent to the logger whenever an order changes state.
type Event struct {
	Kind         EventKind
	RestaurantID int
	OrderID      int
	Zone         int
}
