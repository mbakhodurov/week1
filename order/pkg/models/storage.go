package models

import (
	"errors"
	"sync"
)

type OrderStorage struct {
	mu sync.RWMutex

	orders map[string]*Order
}

func NewOrderStorage() *OrderStorage {
	return &OrderStorage{
		orders: make(map[string]*Order),
	}
}

func (s *OrderStorage) UpdateOrder(o *Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// if o.PaymentMethod != nil {
	// 	s.orders[o.OrderUUID].PaymentMethod = o.PaymentMethod
	// }
	s.orders[o.OrderUUID] = o
	// s.orders[o.OrderUUID].Status = o.Status
}

func (s *OrderStorage) CreateOrder(o *Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[o.OrderUUID] = o
}

func (s *OrderStorage) GetAllOrder() ([]*Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	orders := make([]*Order, 0, len(s.orders))

	for _, v := range s.orders {
		orders = append(orders, v)
	}
	return orders, nil
}

func (s *OrderStorage) GetByUUID(uuid string) (*Order, error) {
	// fmt.Println(s.orders)
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.orders[uuid]
	if !ok {
		return nil, errors.New("not found")
	}
	return order, nil
}
