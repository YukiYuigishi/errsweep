// cleanarch は errsweep の検出能力と限界を示す Clean Architecture API サーバー。
//
// 使い方:
//
//	go run .
//	curl -X POST localhost:8080/orders/order-1/place -d '{"card_number":"4242424242424242","cvc":"123"}'
//	curl -X POST localhost:8080/orders/order-1/cancel
//	curl localhost:8080/orders/order-1
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"example.com/cleanarch/application"
	"example.com/cleanarch/domain/order"
	"example.com/cleanarch/domain/payment"
	"example.com/cleanarch/infra/gateway"
	"example.com/cleanarch/infra/persistence"
)

func main() {
	// --- DI 組み立て ---
	repo := persistence.NewMemoryOrderRepository()
	gw := gateway.NewMockGateway()

	placeOrder := application.NewPlaceOrderUseCase(repo, gw)
	cancelOrder := application.NewCancelOrderUseCase(repo, gw)

	// デモ用の初期データ
	repo.Seed(
		order.Order{
			ID:     "order-1",
			UserID: "user-1",
			Status: order.StatusPending,
			Items: []order.OrderItem{
				{ProductID: "prod-1", Quantity: 2, UnitPrice: 1500},
				{ProductID: "prod-2", Quantity: 1, UnitPrice: 3000},
			},
			CreatedAt: time.Now(),
		},
		order.Order{
			ID:     "order-2",
			UserID: "user-1",
			Status: order.StatusPaid,
			Items: []order.OrderItem{
				{ProductID: "prod-3", Quantity: 1, UnitPrice: 500},
			},
			ChargeID:  "mock_ch_0",
			CreatedAt: time.Now().Add(-24 * time.Hour),
		},
	)

	mux := http.NewServeMux()

	// GET /orders/{id}
	mux.HandleFunc("GET /orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		o, err := repo.FindByID(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, o)
	})

	// POST /orders/{id}/place
	mux.HandleFunc("POST /orders/{id}/place", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var body struct {
			CardNumber string `json:"card_number"`
			CVC        string `json:"cvc"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}

		card := payment.Card{
			Number:   body.CardNumber,
			ExpMonth: 12,
			ExpYear:  2030,
			CVC:      body.CVC,
		}
		if err := placeOrder.Execute(r.Context(), id, card); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "placed"})
	})

	// POST /orders/{id}/cancel
	mux.HandleFunc("POST /orders/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := cancelOrder.Execute(r.Context(), id); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
	})

	addr := ":8080"
	log.Printf("cleanarch example server listening on %s", addr)
	log.Printf("  GET  /orders/{id}")
	log.Printf("  POST /orders/{id}/place   body: {\"card_number\":\"...\",\"cvc\":\"...\"}")
	log.Printf("  POST /orders/{id}/cancel")
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, order.ErrOrderNotFound):
		status = http.StatusNotFound
	case errors.Is(err, order.ErrOrderAlreadyPaid),
		errors.Is(err, order.ErrOrderCancelled):
		status = http.StatusConflict
	case errors.Is(err, payment.ErrPaymentDeclined),
		errors.Is(err, payment.ErrInvalidCard):
		status = http.StatusPaymentRequired
	}
	writeJSON(w, status, map[string]string{"error": fmt.Sprintf("%v", err)})
}
