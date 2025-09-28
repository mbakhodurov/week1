package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/mbakhodurov/week1/order/pkg/models"
	inventoryV1 "github.com/mbakhodurov/week1/shared/pkg/proto/inventory/v1"
	paymentV1 "github.com/mbakhodurov/week1/shared/pkg/proto/payment/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	serverAddress2 = "localhost:50052"

	serverAddress = "localhost:50051"
	httpPort      = "8086"
	urlParamCity  = "city"
	// Таймауты для HTTP-сервера
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 10 * time.Second
)

type responseOrders struct {
	Count  int64           `json:"count"`
	Orders []*models.Order `json:"orders"`
}

func GetAllOrders(storage *models.OrderStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orders, _ := storage.GetAllOrder()
		resp := responseOrders{
			Count:  int64(len(orders)),
			Orders: orders,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func GetOrderByUUID(storage *models.OrderStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orderUUID := chi.URLParam(r, "order_uuid")
		// fmt.Println(orderUUID)
		order, err := storage.GetByUUID(orderUUID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(order)
	}
}

func PayOrder(storage *models.OrderStorage, payment paymentV1.PaymentServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		var req struct {
			Paymentmethod string `json:"payment_method"`
		}
		orderUUID := chi.URLParam(r, "order_uuid")
		// fmt.Println(orderUUID)
		order, err := storage.GetByUUID(orderUUID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if order.Status != models.StatusPending {
			http.Error(w, "this order already processed", http.StatusConflict)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		var paymentMethod paymentV1.PaymentMethod
		switch req.Paymentmethod {
		case "CARD":
			paymentMethod = paymentV1.PaymentMethod_CARD
		case "SBP":
			paymentMethod = paymentV1.PaymentMethod_SBP
		case "CREDIT_CARD":
			paymentMethod = paymentV1.PaymentMethod_CREDIT_CARD
		case "INVESTOR_MONEY":
			paymentMethod = paymentV1.PaymentMethod_INVESTOR_MONEY
		default:
			http.Error(w, "Invalid payment method", http.StatusBadRequest)
			return
		}
		payResp, err := payment.PayOrder(r.Context(), &paymentV1.PayOrderRequest{OrderUuid: order.OrderUUID, UserUuid: order.UserUUID, PaymentMethod: paymentMethod})
		if err != nil {
			fmt.Println("fds")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		order.Status = models.StatusPaid
		order.PaymentMethod = (*models.PaymentMethod)(&req.Paymentmethod)
		order.TransactionUUID = &payResp.TransactionUuid
		storage.UpdateOrder(order)

		resp := map[string]interface{}{
			"transaction_uuid": order.TransactionUUID,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func DeleteOrder(storage *models.OrderStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orderUUID := chi.URLParam(r, "order_uuid")
		order, err := storage.GetByUUID(orderUUID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		if order.Status == models.StatusPaid {
			http.Error(w, "cannot cancel paid order", http.StatusConflict)
			return
		}
		order.Status = models.StatusCancelled
		storage.UpdateOrder(order)

		w.WriteHeader(http.StatusNoContent)
	}
}

func CreateOrder(storage *models.OrderStorage, inventory inventoryV1.InventoryServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UserUUID  string   `json:"user_uuid"`
			PartUUIDs []string `json:"part_uuids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.UserUUID == "" || len(req.PartUUIDs) == 0 {
			http.Error(w, "user_uuid and part_uuids are required", http.StatusBadRequest)
			return
		}
		parts, err := inventory.ListParts(r.Context(), &inventoryV1.ListPartsRequest{
			Filter: &inventoryV1.PartsFilter{Uuids: req.PartUUIDs},
		})
		if err != nil || int(parts.GetTotalCount()) != len(req.PartUUIDs) {
			if err != nil {
				fmt.Println(err)
				http.Error(w, "some parts not found", http.StatusBadRequest)
				return
			}
			fmt.Println(0)
			http.Error(w, "some parts not found", http.StatusBadRequest)
			return
		}

		var total_price float64 = 0
		for _, v := range parts.Inventory {
			total_price += v.Info.Price
		}
		orderNew := models.Order{
			OrderUUID:  uuid.NewString(),
			UserUUID:   req.UserUUID,
			PartUUIDs:  req.PartUUIDs,
			TotalPrice: total_price,
			Status:     models.StatusPending,
		}
		storage.CreateOrder(&orderNew)

		resp := map[string]interface{}{
			"order_uuid":  orderNew.OrderUUID,
			"total_price": orderNew.TotalPrice,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func main() {
	conn2, err2 := grpc.NewClient(serverAddress2,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err2 != nil {
		log.Printf("failed to connect: %v\n", err2)
		return
	}
	defer func() {
		if cerr2 := conn2.Close(); cerr2 != nil {
			log.Printf("failed to close connect: %v", cerr2)
		}
	}()

	conn, err := grpc.NewClient(serverAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Printf("failed to connect: %v\n", err)
		return
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			log.Printf("failed to close connect: %v", cerr)
		}
	}()

	client := inventoryV1.NewInventoryServiceClient(conn)
	payment := paymentV1.NewPaymentServiceClient(conn2)

	storage := models.NewOrderStorage()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(10 * time.Second))
	r.Use(render.SetContentType(render.ContentTypeJSON))

	r.Route("/api/v1/orders", func(r chi.Router) {
		r.Post("/", CreateOrder(storage, client))
		r.Get("/", GetAllOrders(storage))
		r.Get("/{order_uuid}", GetOrderByUUID(storage))
		r.Post("/{order_uuid}/pay", PayOrder(storage, payment))
		r.Post("/{order_uuid}/cancel", DeleteOrder(storage))
	})

	server := &http.Server{
		Addr:              net.JoinHostPort("localhost", httpPort),
		Handler:           r,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	go func() {
		log.Printf("🚀 HTTP-сервер запущен на порту %s\n", httpPort)
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("❌ Ошибка запуска сервера: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("🛑 Завершение работы сервера...")
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	err = server.Shutdown(ctx)
	if err != nil {
		log.Printf("❌ Ошибка при остановке сервера: %v\n", err)
	}

	log.Println("✅ Сервер остановлен")
}
