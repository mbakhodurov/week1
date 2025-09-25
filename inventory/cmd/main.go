package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	inventoryV1 "github.com/mbakhodurov/week1/shared/pkg/proto/inventory/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type inventoryService struct {
	inventoryV1.UnimplementedInventoryServiceServer

	mu sync.RWMutex

	inventories map[string]*inventoryV1.Inventory
}

func (i *inventoryService) Update(ctx context.Context, req *inventoryV1.UpdatePartRequest) (*emptypb.Empty, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	inventory, ok := i.inventories[req.GetUuid()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "inventory with uuid %s not found", req.GetUuid())
	}

	update := req.GetUpdateInfo()
	if update == nil {
		return nil, status.Error(codes.InvalidArgument, "update_info is required")
	}
	if req.GetUpdateInfo().Description != nil {
		inventory.Info.Description = update.Description.Value
	}
	if manufacturer := update.GetManufacturer(); manufacturer != nil {
		if manufacturer.GetName() != "" {
			inventory.Info.Manufacturer.Name = manufacturer.Name
		}
		if manufacturer.GetCountry() != "" {
			inventory.Info.Manufacturer.Country = manufacturer.Country
		}
		if manufacturer.GetWebsite() != "" {
			inventory.Info.Manufacturer.Website = manufacturer.Website
		}
	}

	if req.GetUpdateInfo().Name != nil {
		inventory.Info.Name = update.Name.Value
	}
	// if req.GetUpdateInfo().Price != nil {
	inventory.Info.Price = update.Price
	// }
	// if req.GetUpdateInfo().StockQuantity.Value > 0 {
	// inventory.Info.StockQuantity = int64(update.StockQuantity.Value)
	// }
	inventory.UpdatedAt = timestamppb.New(time.Now())
	return &emptypb.Empty{}, nil
}

func (i *inventoryService) DeletePart(ctx context.Context, req *inventoryV1.DeletePartRequest) (*emptypb.Empty, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	inventory, ok := i.inventories[req.GetUuid()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "sighting with uuid %s not found", req.GetUuid())
	}
	inventory.DeletedAt = timestamppb.New(time.Now())
	return &emptypb.Empty{}, nil
}

func (i *inventoryService) ListParts(ctx context.Context, req *inventoryV1.ListPartsRequest) (*inventoryV1.ListPartsResponse, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	filter := req.GetFilter()
	// fmt.Println(filter)
	// return nil, nil

	// inventories := make([]*inventoryV1.Inventory, 0, len(i.inventories))
	result := []*inventoryV1.Inventory{}

	for _, part := range i.inventories {
		if matchPart(part, filter) {
			result = append(result, part)
		}
	}
	return &inventoryV1.ListPartsResponse{
		Inventory: result,
	}, nil
}

func matchPart(part *inventoryV1.Inventory, filter *inventoryV1.PartsFilter) bool {
	if filter == nil {
		return true
	}

	// UUIDs
	if len(filter.Uuids) > 0 {
		found := false
		for _, u := range filter.Uuids {
			if part.Uuid == u {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Names
	if len(filter.Names) > 0 {
		found := false
		for _, n := range filter.Names {
			if part.Info.Name == n {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Categories
	if len(filter.Categories) > 0 {
		found := false
		for _, c := range filter.Categories {
			if part.Info.Category == c {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Manufacturer countries
	if len(filter.ManufacturerCountries) > 0 {
		found := false
		for _, country := range filter.ManufacturerCountries {
			if part.Info.Manufacturer.Country == country {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Tags
	if len(filter.Tags) > 0 {
		found := false
		for _, tag := range filter.Tags {
			for _, pt := range part.Info.Tags {
				if pt == tag {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func (i *inventoryService) GetAllPart(ctx context.Context, req *inventoryV1.GetAllPartRequest) (*inventoryV1.GetAllPartResponse, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	inventories := make([]*inventoryV1.Inventory, 0, len(i.inventories))
	for _, i := range i.inventories {
		inventories = append(inventories, i)
	}
	return &inventoryV1.GetAllPartResponse{
		Inventory:  inventories,
		TotalCount: int32(len(inventories)),
	}, nil
}

func (i *inventoryService) CreatePart(ctx context.Context, req *inventoryV1.CreatePartRequest) (*inventoryV1.CreatePartResponse, error) {
	if req == nil {
		log.Println("Received nil request")
		return nil, fmt.Errorf("request cannot be nil")
	}

	for _, inv := range i.inventories {
		if inv.Info.Name == req.Info.Name {
			return nil, status.Error(codes.AlreadyExists, "part with this name already exists")
		}
	}
	newUUID := uuid.NewString()

	inventory := &inventoryV1.Inventory{
		Uuid:      newUUID,
		Info:      req.GetInfo(),
		CreatedAt: timestamppb.New(time.Now()),
	}

	i.inventories[newUUID] = inventory
	log.Printf("Создано inventory с UUID %s", newUUID)
	return &inventoryV1.CreatePartResponse{
		Uuid: newUUID,
	}, nil
}

func (i *inventoryService) GetPart(ctx context.Context, rq *inventoryV1.GetPartRequest) (*inventoryV1.GetPartResponse, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	inventory, ok := i.inventories[rq.GetUuid()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "sighting with UUID %s not found", rq.GetUuid())
	}
	return &inventoryV1.GetPartResponse{
		Inventory: inventory,
	}, nil
}

const (
	grpcPort = 50051
	httpPort = 8081
)

func main() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		log.Printf("failed to listen: %v\n", err)
		return
	}

	defer func() {
		if cerr := lis.Close(); cerr != nil {
			log.Printf("failed to close listener: %v\n", cerr)
		}
	}()
	s := grpc.NewServer()
	service := &inventoryService{
		inventories: make(map[string]*inventoryV1.Inventory),
	}
	inventoryV1.RegisterInventoryServiceServer(s, service)
	reflection.Register(s)

	go func() {
		log.Printf("🚀 gRPC server listening on %d\n", grpcPort)
		err = s.Serve(lis)
		if err != nil {
			log.Printf("failed to serve: %v\n", err)
			return
		}
	}()

	var gwServer *http.Server

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		mux := runtime.NewServeMux()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
		err = inventoryV1.RegisterInventoryServiceHandlerFromEndpoint(
			ctx, mux, fmt.Sprintf("localhost:%d", grpcPort),
			opts,
		)
		if err != nil {
			log.Printf("Failed to register gateway: %v\n", err)
			return
		}
		gwServer = &http.Server{
			Addr:        fmt.Sprintf(":%d", httpPort),
			Handler:     mux,
			ReadTimeout: 10 * time.Second,
		}
		log.Printf("🌐 HTTP server with gRPC-Gateway listening on %d\n", httpPort)
		err = gwServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Printf("Failed to serve HTTP: %v\n", err)
			return
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("🛑 Shutting down gRPC server...")
	s.GracefulStop()
	log.Println("✅ Server stopped")
}
