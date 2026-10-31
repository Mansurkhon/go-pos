package service

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/Mansurkhon/go-pos/internal/domain"
)

// fakeRepo подменяет PostgreSQL: сервис видит только интерфейс Repository.
// Встроенный интерфейс даёт все методы; нужные тесту переопределяем ниже.
type fakeRepo struct {
	Repository
	saleItems []domain.SaleItem
	created   *domain.Product
}

func (f *fakeRepo) CreateSale(_ context.Context, items []domain.SaleItem) (*domain.Sale, error) {
	f.saleItems = items
	return &domain.Sale{ID: 1, Items: items, CreatedAt: time.Now()}, nil
}

func (f *fakeRepo) CreateProduct(_ context.Context, p *domain.Product) error {
	f.created = p
	return nil
}

func TestCreateSaleValidation(t *testing.T) {
	tests := []struct {
		name      string
		items     []domain.SaleItem
		wantField string
	}{
		{"empty", nil, "items"},
		{"zero qty", []domain.SaleItem{{ProductID: 1, Qty: 0}}, "qty"},
		{"negative qty", []domain.SaleItem{{ProductID: 1, Qty: -2}}, "qty"},
		{"bad product id", []domain.SaleItem{{ProductID: 0, Qty: 1}}, "product_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := New(&fakeRepo{})
			_, err := svc.CreateSale(context.Background(), tt.items)

			var ve *domain.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("want ValidationError, got %v", err)
			}
			if ve.Field != tt.wantField {
				t.Errorf("field = %q, want %q", ve.Field, tt.wantField)
			}
		})
	}
}

func TestCreateSaleMergesAndSortsItems(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo)

	_, err := svc.CreateSale(context.Background(), []domain.SaleItem{
		{ProductID: 5, Qty: 1},
		{ProductID: 2, Qty: 3},
		{ProductID: 5, Qty: 2},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []domain.SaleItem{{ProductID: 2, Qty: 3}, {ProductID: 5, Qty: 3}}
	if !slices.Equal(repo.saleItems, want) {
		t.Errorf("items = %v, want %v", repo.saleItems, want)
	}
}

func TestCreateProduct(t *testing.T) {
	tests := []struct {
		name      string
		product   domain.Product
		wantField string // пусто — ошибки быть не должно
	}{
		{"ok", domain.Product{Name: "  Хлеб  ", Price: 550, Stock: 10}, ""},
		{"blank name", domain.Product{Name: "   ", Price: 100}, "name"},
		{"negative price", domain.Product{Name: "Чай", Price: -1}, "price"},
		{"negative stock", domain.Product{Name: "Чай", Stock: -5}, "stock"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{}
			p := tt.product
			err := New(repo).CreateProduct(context.Background(), &p)

			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if repo.created.Name != "Хлеб" {
					t.Errorf("name not trimmed: %q", repo.created.Name)
				}
				return
			}
			var ve *domain.ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.wantField {
				t.Fatalf("want ValidationError on %q, got %v", tt.wantField, err)
			}
			if repo.created != nil {
				t.Error("invalid product must not reach the repository")
			}
		})
	}
}
