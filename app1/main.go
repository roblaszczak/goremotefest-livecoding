package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi"
	chiMiddleware "github.com/go-chi/chi/middleware"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type BookRoomRequest struct {
	RoomID      string `json:"room_id"`
	GuestsCount int    `json:"guests_count"`
}

type RoomBookingHandler struct {
	payments PaymentsProvider
}

func (h RoomBookingHandler) Handler(writer http.ResponseWriter, request *http.Request) {
	b, err := ioutil.ReadAll(request.Body)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	req := BookRoomRequest{}
	err = json.Unmarshal(b, &req)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	roomPrice := 42 * req.GuestsCount

	err = h.payments.TakePayment(roomPrice)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		log.WithError(err).Error("Failed to take payment")
		return
	}
}

type PaymentsProvider struct{}

func (p PaymentsProvider) TakePayment(amount int) error {
	// this is not the best payment provider...
	if rand.Int31n(2) == 0 {
		time.Sleep(time.Second * 5)
	}
	if rand.Int31n(3) == 0 {
		return errors.New("error")
	}

	log.Info("payment taken")
	return nil
}

func main() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	log.Info("Starting app")

	h := RoomBookingHandler{
		payments: PaymentsProvider{},
	}

	chiRouter := chi.NewRouter()
	chiRouter.Use(chiMiddleware.Recoverer)
	chiRouter.Post("/book", h.Handler)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)
		<-c
		cancel()
	}()

	runHTTP(ctx, chiRouter)
}

func runHTTP(ctx context.Context, handler http.Handler) {
	log.Info("Running router")
	server := &http.Server{Addr: ":8080", Handler: handler}
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		panic(err)
	}
}
