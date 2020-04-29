// +build example

package main

import (
	"context"
	"github.com/ThreeDotsLabs/watermill/components/cqrs"
)

type cqrsPaymentsHandler struct {
	eventBus         *cqrs.EventBus
	paymentsProvider PaymentsProvider
}

func (b cqrsPaymentsHandler) HandlerName() string {
	return "RoomBookedHandler"
}

func (b cqrsPaymentsHandler) NewEvent() interface{} {
	return &RoomBooked{}
}

func (b cqrsPaymentsHandler) Handle(ctx context.Context, c interface{}) error {
	event := c.(*RoomBooked)

	if err := b.paymentsProvider.TakePayment(event.Price); err != nil {
		return err
	}

	if err := b.eventBus.Publish(ctx, &PaymentTaken{
		RoomID: event.RoomID,
		Price:  event.Price,
	}); err != nil {
		return err
	}

	return nil
}
