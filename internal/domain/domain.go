// Package domain содержит модели и ошибки предметной области.
// Он не зависит ни от базы, ни от HTTP — его импортируют все остальные слои.
package domain

import (
	"errors"
	"time"
)

// Sentinel-ошибки: сравниваются через errors.Is даже после обёртки через %w.
var (
	ErrNotFound          = errors.New("not found")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrInUse             = errors.New("product is used in sales")
)

// ValidationError — ошибка входных данных, проверяется через errors.As.
type ValidationError struct {
	Field string
	Msg   string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Msg }

// Деньги храним в дирамах (1/100 сомони) как int64 — float для денег не годится.
type Product struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Price     int64     `json:"price"`
	Stock     int       `json:"stock"`
	CreatedAt time.Time `json:"created_at"`
}

type SaleItem struct {
	ProductID int64 `json:"product_id"`
	Qty       int   `json:"qty"`
	Price     int64 `json:"price"` // заполняет сервер из цены товара на момент продажи
}

type Sale struct {
	ID        int64      `json:"id"`
	Total     int64      `json:"total"`
	Items     []SaleItem `json:"items"`
	CreatedAt time.Time  `json:"created_at"`
}

type ReportRow struct {
	ProductID int64  `json:"product_id"`
	Name      string `json:"name"`
	Qty       int64  `json:"qty"`
	Revenue   int64  `json:"revenue"`
}

type DailyReport struct {
	Date       string      `json:"date"`
	SalesCount int64       `json:"sales_count"`
	Total      int64       `json:"total"`
	Items      []ReportRow `json:"items"`
}
