// Package service — бизнес-правила. Он работает с хранилищем только через
// интерфейс Repository, поэтому в тестах базу можно заменить фейком.
package service

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/Mansurkhon/go-pos/internal/domain"
)

type Repository interface {
	ListProducts(ctx context.Context) ([]domain.Product, error)
	GetProduct(ctx context.Context, id int64) (*domain.Product, error)
	CreateProduct(ctx context.Context, p *domain.Product) error
	UpdateProduct(ctx context.Context, p *domain.Product) error
	DeleteProduct(ctx context.Context, id int64) error
	CreateSale(ctx context.Context, items []domain.SaleItem) (*domain.Sale, error)
	DailyReport(ctx context.Context, day time.Time) (*domain.DailyReport, error)
}

type Service struct {
	repo Repository
}

func New(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListProducts(ctx context.Context) ([]domain.Product, error) {
	return s.repo.ListProducts(ctx)
}

func (s *Service) GetProduct(ctx context.Context, id int64) (*domain.Product, error) {
	return s.repo.GetProduct(ctx, id)
}

func (s *Service) CreateProduct(ctx context.Context, p *domain.Product) error {
	if err := validateProduct(p); err != nil {
		return err
	}
	return s.repo.CreateProduct(ctx, p)
}

func (s *Service) UpdateProduct(ctx context.Context, p *domain.Product) error {
	if err := validateProduct(p); err != nil {
		return err
	}
	return s.repo.UpdateProduct(ctx, p)
}

func (s *Service) DeleteProduct(ctx context.Context, id int64) error {
	return s.repo.DeleteProduct(ctx, id)
}

// CreateSale проверяет позиции, объединяет повторы одного товара
// и сортирует их по ID перед транзакцией в репозитории.
func (s *Service) CreateSale(ctx context.Context, items []domain.SaleItem) (*domain.Sale, error) {
	if len(items) == 0 {
		return nil, &domain.ValidationError{Field: "items", Msg: "must not be empty"}
	}

	qtyByProduct := make(map[int64]int, len(items))
	for _, it := range items {
		if it.ProductID <= 0 {
			return nil, &domain.ValidationError{Field: "product_id", Msg: "must be positive"}
		}
		if it.Qty <= 0 {
			return nil, &domain.ValidationError{Field: "qty", Msg: "must be positive"}
		}
		qtyByProduct[it.ProductID] += it.Qty
	}

	ids := slices.Sorted(maps.Keys(qtyByProduct))
	normalized := make([]domain.SaleItem, 0, len(ids))
	for _, id := range ids {
		normalized = append(normalized, domain.SaleItem{ProductID: id, Qty: qtyByProduct[id]})
	}
	return s.repo.CreateSale(ctx, normalized)
}

func (s *Service) DailyReport(ctx context.Context, day time.Time) (*domain.DailyReport, error) {
	return s.repo.DailyReport(ctx, day)
}

func validateProduct(p *domain.Product) error {
	p.Name = strings.TrimSpace(p.Name)
	switch {
	case p.Name == "":
		return &domain.ValidationError{Field: "name", Msg: "must not be empty"}
	case p.Price < 0:
		return &domain.ValidationError{Field: "price", Msg: "must not be negative"}
	case p.Stock < 0:
		return &domain.ValidationError{Field: "stock", Msg: "must not be negative"}
	}
	return nil
}
