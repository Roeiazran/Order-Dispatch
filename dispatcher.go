package main

// dispatcher receives all orders and routes each one to its zone's channel.
// When the orders channel is closed it closes every zone channel, which tells
// the zone managers that no more work is coming.
func dispatcher(ordersChan <-chan Order, zoneChans []chan Order, eventsChan chan<- Event) {
	for order := range ordersChan {
		eventsChan <- Event{
			Kind:         EventDispatched,
			RestaurantID: order.RestaurantID,
			OrderID:      order.OrderID,
			Zone:         order.FoodType,
		}

		zoneChans[order.FoodType] <- order
	}

	for _, ch := range zoneChans {
		close(ch)
	}
}
