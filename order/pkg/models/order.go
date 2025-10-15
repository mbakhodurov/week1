package models

type OrderStatus string

const (
	StatusPending   OrderStatus = "PENDING_PAYMENT"
	StatusPaid      OrderStatus = "PAID"
	StatusCancelled OrderStatus = "CANCELLED"
)

type PaymentMethod string

const (
	PaymentUnknown       PaymentMethod = "UNKNOWN"
	PaymentCard          PaymentMethod = "CARD"
	PaymentSBP           PaymentMethod = "SBP"
	PaymentCreditCard    PaymentMethod = "CREDIT_CARD"
	PaymentInvestorMoney PaymentMethod = "INVESTOR_MONEY"
)

type Order struct {
	OrderUUID       string         `json:"order_uuid"`
	UserUUID        string         `json:"user_uuid"`
	PartUUIDs       []string       `json:"part_uuids"`
	TotalPrice      float64        `json:"total_price"`
	TransactionUUID *string        `json:"transaction_uuid,omitempty"`
	PaymentMethod   *PaymentMethod `json:"payment_method,omitempty"`
	Status          OrderStatus    `json:"status"`
}
