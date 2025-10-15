package models

import (
	"errors"
	"sync"
)

type Storage struct {
	mu     sync.RWMutex
	orders map[string]*Order
}

func NewStorage() *Storage {
	return &Storage{
		orders: make(map[string]*Order),
	}
}

func (s *Storage) CreateOrder(o *Order) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.orders[o.OrderUUID] = o
}

func (s *Storage) GetAllOrder() ([]*Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	orders := make([]*Order, 0, len(s.orders))

	for _, v := range s.orders {
		orders = append(orders, v)
	}
	return orders, nil
}

func (s *Storage) GetOrderByUUID(uuid string) (*Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	order, ok := s.orders[uuid]
	if !ok {
		return nil, errors.New("not found")
	}
	return order, nil
}
