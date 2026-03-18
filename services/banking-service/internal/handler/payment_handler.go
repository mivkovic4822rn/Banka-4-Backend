package handler

import (
	"banking-service/internal/dto"
	"banking-service/internal/service"
	"common/pkg/errors"
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	service        *service.PaymentService
	accountService *service.AccountService
}

func NewPaymentHandler(paymentService *service.PaymentService, accountService *service.AccountService) *PaymentHandler {
	return &PaymentHandler{service: paymentService, accountService: accountService}
}

func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	var req dto.CreatePaymentRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}


	payment, err := h.service.CreatePayment(c.Request.Context(), req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.CreatePaymentResponse{
		PaymentID: payment.PaymentID,
	})
}

func (h *PaymentHandler) VerifyPayment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(errors.BadRequestErr("invalid payment id"))
		return
	}

	var req dto.VerifyPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	payment, err := h.service.VerifyPayment(c.Request.Context(), uint(id), req.Code)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.VerifyPaymentResponse{
		PaymentID: payment.PaymentID,
	})
}

// GetAccountPayments godoc
// @Summary List account payments
// @Description Returns a paginated list of payments for an account, filterable by status, date range, and amount. Only the account owner can access this.
// @Tags payments
// @Produce json
// @Param accountNumber path string true "Account number"
// @Param status query string false "Filter by status (processing, completed, rejected)"
// @Param start_date query string false "Filter from date (YYYY-MM-DD)"
// @Param end_date query string false "Filter to date (YYYY-MM-DD)"
// @Param min_amount query number false "Minimum amount filter"
// @Param max_amount query number false "Maximum amount filter"
// @Param page query int false "Page number" minimum(1)
// @Param page_size query int false "Page size" minimum(1) maximum(100)
// @Success 200 {object} dto.ListPaymentsResponse
// @Failure 400 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Security BearerAuth
// @Router /api/accounts/{accountNumber}/payments [get]
func (h *PaymentHandler) GetAccountPayments(c *gin.Context) {
	valStr := c.Param("clientId")

	idVal, err := strconv.ParseUint(valStr, 10, 64)

	if err != nil {
		c.Error(errors.BadRequestErr("client id must be a number"))
		return
	}

	accountNumber := c.Param("accountNumber")

	if _, err := h.accountService.GetAccountDetails(c.Request.Context(), accountNumber, uint(idVal)); err != nil {
		c.Error(err)
		return
	}

	var filters dto.PaymentFilters
	if err := c.ShouldBindQuery(&filters); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}
	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.PageSize < 1 {
		filters.PageSize = 10
	}

	payments, total, err := h.service.GetAccountPayments(c.Request.Context(), accountNumber, &filters)
	if err != nil {
		c.Error(err)
		return
	}

	data := make([]dto.PaymentSummaryResponse, len(payments))
	for i, p := range payments {
		data[i] = dto.PaymentSummaryResponse{
			ID:               p.PaymentID,
			RecipientName:    p.RecipientName,
			RecipientAccount: p.Transaction.RecipientAccountNumber,
			PayerAccount:     p.Transaction.PayerAccountNumber,
			Amount:           p.Transaction.StartAmount,
			Currency:         string(p.Transaction.StartCurrencyCode),
			Status:           string(p.Transaction.Status),
			Purpose:          p.Purpose,
			PaymentCode:      p.PaymentCode,
			CreatedAt:        p.Transaction.CreatedAt,
		}
	}

	totalPages := int(math.Ceil(float64(total) / float64(filters.PageSize)))
	c.JSON(http.StatusOK, dto.ListPaymentsResponse{
		Data:       data,
		Total:      total,
		Page:       filters.Page,
		PageSize:   filters.PageSize,
		TotalPages: totalPages,
	})
}
