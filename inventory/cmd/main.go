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
	inventory_v1 "github.com/mbakhodurov/week1/shared/pkg/proto/inventory/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	grpcPort = 50051
	httpPort = 8081
)

type inventoryService struct {
	inventory_v1.UnimplementedInventoryServiceServer
	mu sync.RWMutex

	inventories map[string]*inventory_v1.Part
}

func (i *inventoryService) UpdatePart(_ context.Context, rq *inventory_v1.UpdatePartRequest) (*emptypb.Empty, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	inventory, ok := i.inventories[rq.GetUuid()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "inventory with uuid %s not found", rq.GetUuid())
	}

	updateInfo := rq.GetUpdateInfo()
	if updateInfo == nil {
		return nil, status.Error(codes.InvalidArgument, "update_info is required")
	}
	if updateInfo.Description != nil {
		inventory.Info.Description = updateInfo.Description.Value
	}

	if manufacturer := updateInfo.GetManufacturer(); manufacturer != nil {
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

	if rq.GetUpdateInfo().Name != nil {
		inventory.Info.Name = updateInfo.Name.Value
	}
	inventory.Info.Price = updateInfo.GetPrice()
	return &emptypb.Empty{}, nil
}

func (i *inventoryService) GetAllPart(_ context.Context, rq *inventory_v1.GetAllPartRequest) (*inventory_v1.GetAllPartResponse, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	result := []*inventory_v1.Part{}
	for _, v := range i.inventories {
		result = append(result, v)
	}
	return &inventory_v1.GetAllPartResponse{
		Part:       result,
		TotalCount: int64(len(result)),
	}, nil
}

func (i *inventoryService) DeletePart(_ context.Context, rq *inventory_v1.DeletePartRequest) (*emptypb.Empty, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	inventory, ok := i.inventories[rq.GetUuid()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "Part with uuid %s not found", rq.GetUuid())
	}
	inventory.DeletedAt = timestamppb.New(time.Now())
	return &emptypb.Empty{}, nil
}

func (i *inventoryService) CreateParts(_ context.Context, rq *inventory_v1.CreatePartsRequest) (*inventory_v1.CreatePartsResponse, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if rq == nil {
		log.Println("Received nil request")
		return nil, fmt.Errorf("request cannot be nil")
	}

	for _, v := range i.inventories {
		if v.Info.Name == rq.GetInfo().Name {
			return nil, status.Error(codes.AlreadyExists, "part with this name already exists")
		}
	}

	newUUID := uuid.NewString()

	inventory := &inventory_v1.Part{
		Uuid:      newUUID,
		Info:      rq.GetInfo(),
		CreatedAt: timestamppb.New(time.Now()),
	}

	i.inventories[newUUID] = inventory

	return &inventory_v1.CreatePartsResponse{
		Uuid: newUUID,
	}, nil
}

func (i *inventoryService) ListParts(_ context.Context, rq *inventory_v1.ListPartsRequest) (*inventory_v1.ListPartsResponse, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	filter := rq.GetFilter()
	result := []*inventory_v1.Part{}

	for _, part := range i.inventories {
		if matchPart(part, filter) {
			result = append(result, part)
		}
	}

	return &inventory_v1.ListPartsResponse{
		Part:       result,
		TotalCount: int64(len(result)),
	}, nil
}

func matchPart(part *inventory_v1.Part, filter *inventory_v1.PartsFilter) bool {
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

func (i *inventoryService) GetPart(_ context.Context, rq *inventory_v1.GetPartRequest) (*inventory_v1.GetPartResponse, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	part, ok := i.inventories[rq.GetUuid()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "sighting with UUID %s not found", rq.GetUuid())
	}

	return &inventory_v1.GetPartResponse{
		Part: part,
	}, nil
}

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
		inventories: make(map[string]*inventory_v1.Part),
	}

	inventory_v1.RegisterInventoryServiceServer(s, service)

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
		err = inventory_v1.RegisterInventoryServiceHandlerFromEndpoint(
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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("🛑 Shutting down gRPC server...")
	s.GracefulStop()
	log.Println("✅ Server stopped")
}
