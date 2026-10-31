// Package repository — доступ к PostgreSQL через pgx. Здесь только SQL,
// бизнес-правила живут в service.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Mansurkhon/go-pos/internal/domain"
)

// Код ошибки PostgreSQL «нарушение внешнего ключа».
const pgForeignKeyViolation = "23503"

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (r *Postgres) ListProducts(ctx context.Context) ([]domain.Product, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, price, stock, created_at FROM products ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	products := []domain.Product{} // не nil, чтобы в JSON был [], а не null
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Stock, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func (r *Postgres) GetProduct(ctx context.Context, id int64) (*domain.Product, error) {
	var p domain.Product
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, price, stock, created_at FROM products WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.Price, &p.Stock, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("product %d: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get product %d: %w", id, err)
	}
	return &p, nil
}

func (r *Postgres) CreateProduct(ctx context.Context, p *domain.Product) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO products (name, price, stock) VALUES ($1, $2, $3) RETURNING id, created_at`,
		p.Name, p.Price, p.Stock,
	).Scan(&p.ID, &p.CreatedAt)
	if err != nil {
		return fmt.Errorf("create product: %w", err)
	}
	return nil
}

func (r *Postgres) UpdateProduct(ctx context.Context, p *domain.Product) error {
	err := r.pool.QueryRow(ctx,
		`UPDATE products SET name = $1, price = $2, stock = $3 WHERE id = $4 RETURNING created_at`,
		p.Name, p.Price, p.Stock, p.ID,
	).Scan(&p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("product %d: %w", p.ID, domain.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("update product %d: %w", p.ID, err)
	}
	return nil
}

func (r *Postgres) DeleteProduct(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM products WHERE id = $1`, id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
		return fmt.Errorf("product %d: %w", id, domain.ErrInUse)
	}
	if err != nil {
		return fmt.Errorf("delete product %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("product %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

// CreateSale списывает остатки и записывает чек в одной транзакции.
// items должны быть отсортированы по product_id: тогда две параллельные продажи
// блокируют строки в одном порядке и не попадают в deadlock.
func (r *Postgres) CreateSale(ctx context.Context, items []domain.SaleItem) (*domain.Sale, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	sale := &domain.Sale{Items: make([]domain.SaleItem, 0, len(items))}
	for _, it := range items {
		var price int64
		var stock int
		// FOR UPDATE блокирует строку товара до конца транзакции:
		// другая касса не продаст тот же остаток одновременно с нами.
		err := tx.QueryRow(ctx,
			`SELECT price, stock FROM products WHERE id = $1 FOR UPDATE`, it.ProductID,
		).Scan(&price, &stock)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("product %d: %w", it.ProductID, domain.ErrNotFound)
		}
		if err != nil {
			return nil, fmt.Errorf("lock product %d: %w", it.ProductID, err)
		}
		if stock < it.Qty {
			return nil, fmt.Errorf("product %d: have %d, want %d: %w",
				it.ProductID, stock, it.Qty, domain.ErrInsufficientStock)
		}

		if _, err := tx.Exec(ctx,
			`UPDATE products SET stock = stock - $1 WHERE id = $2`, it.Qty, it.ProductID,
		); err != nil {
			return nil, fmt.Errorf("update stock %d: %w", it.ProductID, err)
		}

		it.Price = price
		sale.Total += price * int64(it.Qty)
		sale.Items = append(sale.Items, it)
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO sales (total) VALUES ($1) RETURNING id, created_at`, sale.Total,
	).Scan(&sale.ID, &sale.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert sale: %w", err)
	}

	for _, it := range sale.Items {
		if _, err := tx.Exec(ctx,
			`INSERT INTO sale_items (sale_id, product_id, qty, price) VALUES ($1, $2, $3, $4)`,
			sale.ID, it.ProductID, it.Qty, it.Price,
		); err != nil {
			return nil, fmt.Errorf("insert sale item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return sale, nil
}

// DailyReport считает продажи за сутки [day, day+1) по каждому товару.
func (r *Postgres) DailyReport(ctx context.Context, day time.Time) (*domain.DailyReport, error) {
	from, to := day, day.AddDate(0, 0, 1)
	report := &domain.DailyReport{Date: day.Format("2006-01-02"), Items: []domain.ReportRow{}}

	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(SUM(total), 0)::BIGINT
		   FROM sales
		  WHERE created_at >= $1 AND created_at < $2`, from, to,
	).Scan(&report.SalesCount, &report.Total)
	if err != nil {
		return nil, fmt.Errorf("report totals: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.name, SUM(si.qty)::BIGINT, SUM(si.qty * si.price)::BIGINT AS revenue
		   FROM sale_items si
		   JOIN sales s    ON s.id = si.sale_id
		   JOIN products p ON p.id = si.product_id
		  WHERE s.created_at >= $1 AND s.created_at < $2
		  GROUP BY p.id, p.name
		  ORDER BY revenue DESC`, from, to)
	if err != nil {
		return nil, fmt.Errorf("report items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var row domain.ReportRow
		if err := rows.Scan(&row.ProductID, &row.Name, &row.Qty, &row.Revenue); err != nil {
			return nil, fmt.Errorf("scan report row: %w", err)
		}
		report.Items = append(report.Items, row)
	}
	return report, rows.Err()
}
