package models

import (
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
