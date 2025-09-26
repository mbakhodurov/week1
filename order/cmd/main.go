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
	inventoryV1 "github.com/mbakhodurov/week1/shared/pkg/proto/inventory/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/mbakhodurov/week1/order/pkg/models"
)

const (
	serverAddress = "localhost:50051"
	httpPort      = "8086"
	urlParamCity  = "city"
	// Таймауты для HTTP-сервера
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 10 * time.Second
)

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
			fmt.Println(err)
			http.Error(w, "some parts not found", http.StatusBadRequest)
			return
		}

		var total_price float64 = 0
		for _, v := range parts.Inventory {
			total_price += v.Info.Price
		}
	}
}

func main() {
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

	storage := models.NewOrderStorage()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(10 * time.Second))
	r.Use(render.SetContentType(render.ContentTypeJSON))

	r.Route("/api/v1/orders", func(r chi.Router) {
		r.Post("/", CreateOrder(storage, client))
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
