package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-googlecloud/pkg/googlecloud"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

type BookRoomRequest struct {
	RoomID      string `json:"room_id"`
	GuestsCount int    `json:"guests_count"`
}

type RoomBookingHandler struct {
	publisher message.Publisher
}

type RoomBooked struct {
	RoomID      string `json:"room_id"`
	GuestsCount int    `json:"guests_count"`
	Price       int    `json:"price"`
}

func (h RoomBookingHandler) Handler(writer http.ResponseWriter, request *http.Request) {
	b, err := io.ReadAll(request.Body)
	if err != nil {
		panic(err)
	}

	req := BookRoomRequest{}
	err = json.Unmarshal(b, &req)
	if err != nil {
		panic(err)
	}

	roomPrice := 42 * req.GuestsCount

	event := RoomBooked{
		RoomID:      req.RoomID,
		GuestsCount: req.GuestsCount,
		Price:       roomPrice,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		panic(err)
	}

	err = h.publisher.Publish("bookings", message.NewMessage(watermill.NewUUID(), payload))
	if err != nil {
		panic(err)
	}
}

type PaymentsHandler struct {
	provider PaymentsProvider
}

type PaymentTaken struct {
	RoomID string `json:"room_id"`
	Price  int    `json:"price"`
}

func (p PaymentsHandler) Handler(msg *message.Message) (messages []*message.Message, err error) {
	roomBooked := RoomBooked{}

	err = json.Unmarshal(msg.Payload, &roomBooked)
	if err != nil {
		return nil, err
	}

	err = p.provider.TakePayment(roomBooked.Price)
	if err != nil {
		return nil, err
	}

	event := PaymentTaken{
		RoomID: roomBooked.RoomID,
		Price:  roomBooked.Price,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		panic(err)
	}

	return message.Messages{message.NewMessage(watermill.NewUUID(), payload)}, nil
}

type PaymentsProvider struct{}

func (p PaymentsProvider) TakePayment(amount int) error {
	// this is not the best payment provider...
	if rand.Int31n(2) == 0 {
		time.Sleep(time.Second * 5)
	}
	if rand.Int31n(3) == 0 {
		return errors.New("random error")
	}

	log.Println("payment taken")
	return nil
}

func main() {
	log.Println("Starting app")

	watermillLogger := watermill.NewStdLogger(true, false)

	subscriber, err := googlecloud.NewSubscriber(googlecloud.SubscriberConfig{}, watermillLogger)
	if err != nil {
		panic(err)
	}
	publisher, err := googlecloud.NewPublisher(googlecloud.PublisherConfig{}, watermillLogger)
	if err != nil {
		panic(err)
	}

	h := RoomBookingHandler{publisher}

	watermillRouter, err := message.NewRouter(
		message.RouterConfig{},
		watermillLogger,
	)
	if err != nil {
		panic(err)
	}

	watermillRouter.AddHandler(
		"payments",
		"bookings",
		subscriber,
		"payments",
		publisher,
		PaymentsHandler{}.Handler,
	)

	chiRouter := chi.NewRouter()
	chiRouter.Use(chiMiddleware.Recoverer)
	chiRouter.Post("/book", h.Handler)

	ctx, cancel := context.WithCancel(context.Background())

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)
		<-c
		cancel()
	}()

	go func() {
		defer wg.Done()
		runHTTP(ctx, chiRouter)
	}()

	go func() {
		defer wg.Done()

		err := watermillRouter.Run(ctx)
		if err != nil {
			panic(err)
		}
	}()

	// waiting for routers for proper graceful shutdown
	wg.Wait()
	log.Println("Server stopped")

}

func runHTTP(ctx context.Context, handler http.Handler) {
	log.Println("Running router")
	server := &http.Server{Addr: ":8080", Handler: handler}
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		panic(err)
	}
}
