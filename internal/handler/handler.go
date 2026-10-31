// Package handler — HTTP-слой на Fiber: разбирает запрос, вызывает сервис,
// превращает ошибки домена в HTTP-коды.
package handler

import (
	"errors"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/Mansurkhon/go-pos/internal/domain"
	"github.com/Mansurkhon/go-pos/internal/service"
)

type Handler struct {
	svc *service.Service
}

func Register(app *fiber.App, svc *service.Service) {
	h := &Handler{svc: svc}

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	api := app.Group("/api/v1")
	api.Get("/products", h.listProducts)
	api.Post("/products", h.createProduct)
	api.Get("/products/:id", h.getProduct)
	api.Put("/products/:id", h.updateProduct)
	api.Delete("/products/:id", h.deleteProduct)
	api.Post("/sales", h.createSale)
	api.Get("/reports/daily", h.dailyReport)
}

// ErrorHandler — единое место, где ошибка превращается в HTTP-ответ.
// Хендлеры просто возвращают err, а не пишут ответы с ошибками сами.
func ErrorHandler(c *fiber.Ctx, err error) error {
	code, msg := fiber.StatusInternalServerError, "internal error"

	var ve *domain.ValidationError
	var fe *fiber.Error
	switch {
	case errors.As(err, &ve):
		code, msg = fiber.StatusBadRequest, ve.Error()
	case errors.Is(err, domain.ErrNotFound):
		code, msg = fiber.StatusNotFound, err.Error()
	case errors.Is(err, domain.ErrInsufficientStock), errors.Is(err, domain.ErrInUse):
		code, msg = fiber.StatusConflict, err.Error()
	case errors.As(err, &fe):
		code, msg = fe.Code, fe.Message
	default:
		// Детали внутренней ошибки пишем в лог, но не отдаём клиенту.
		log.Printf("internal error: %s %s: %v", c.Method(), c.Path(), err)
	}
	return c.Status(code).JSON(fiber.Map{"error": msg})
}

func productID(c *fiber.Ctx) (int64, error) {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return 0, fiber.NewError(fiber.StatusBadRequest, "invalid id")
	}
	return int64(id), nil
}

func (h *Handler) listProducts(c *fiber.Ctx) error {
	products, err := h.svc.ListProducts(c.UserContext())
	if err != nil {
		return err
	}
	return c.JSON(products)
}

func (h *Handler) getProduct(c *fiber.Ctx) error {
	id, err := productID(c)
	if err != nil {
		return err
	}
	p, err := h.svc.GetProduct(c.UserContext(), id)
	if err != nil {
		return err
	}
	return c.JSON(p)
}

func (h *Handler) createProduct(c *fiber.Ctx) error {
	var p domain.Product
	if err := c.BodyParser(&p); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON")
	}
	if err := h.svc.CreateProduct(c.UserContext(), &p); err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(p)
}

func (h *Handler) updateProduct(c *fiber.Ctx) error {
	id, err := productID(c)
	if err != nil {
		return err
	}
	var p domain.Product
	if err := c.BodyParser(&p); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON")
	}
	p.ID = id
	if err := h.svc.UpdateProduct(c.UserContext(), &p); err != nil {
		return err
	}
	return c.JSON(p)
}

func (h *Handler) deleteProduct(c *fiber.Ctx) error {
	id, err := productID(c)
	if err != nil {
		return err
	}
	if err := h.svc.DeleteProduct(c.UserContext(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type saleRequest struct {
	Items []domain.SaleItem `json:"items"`
}

func (h *Handler) createSale(c *fiber.Ctx) error {
	var req saleRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON")
	}
	sale, err := h.svc.CreateSale(c.UserContext(), req.Items)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(sale)
}

func (h *Handler) dailyReport(c *fiber.Ctx) error {
	date := c.Query("date", time.Now().Format("2006-01-02"))
	day, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "date must be YYYY-MM-DD")
	}
	report, err := h.svc.DailyReport(c.UserContext(), day)
	if err != nil {
		return err
	}
	return c.JSON(report)
}
