package handler

import (
	"banking-service/internal/dto"
	"banking-service/internal/service"
	"common/pkg/errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type AccountHandler struct {
	service *service.AccountService
}

func NewAccountHandler(service *service.AccountService) *AccountHandler {
	return &AccountHandler{service: service}
}

// Create godoc
// @Summary Create a new bank account
// @Description Creates a new bank account for a client. Account kind must be "Current" or "Foreign". Current accounts require a subtype; foreign accounts require a currency code.
// @Tags accounts
// @Accept json
// @Produce json
// @Param account body dto.CreateAccountRequest true "Account creation data"
// @Success 201 {object} dto.AccountResponse
// @Failure 400 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 409 {object} errors.AppError
// @Security BearerAuth
// @Router /api/accounts [post]
func (h *AccountHandler) Create(c *gin.Context) {
	var req dto.CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	account, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToAccountResponse(account))
}

func GetParamUint(c *gin.Context, key string) (uint, bool) {
	valStr := c.Param(key)

	val, err := strconv.ParseUint(valStr, 10, 64)

	if err != nil {
		c.Error(errors.BadRequestErr("client id must be a number"))
		return 0, false
	}

	return uint(val), true
}

// GetClientAccounts godoc
// @Summary List client accounts
// @Description Returns all active accounts belonging to the authenticated client
// @Tags accounts
// @Produce json
// @Param clientId path string true "Client id"
// @Success 200 {array} dto.AccountSummaryResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/accounts [get]

func (h *AccountHandler) GetClientAccounts(c *gin.Context) {

	clientId, ok := GetParamUint(c, "clientId")
	if !ok {
		return
	}

	accounts, err := h.service.GetClientAccounts(c.Request.Context(), clientId)
	if err != nil {
		c.Error(err)
		return
	}

	resp := make([]dto.AccountSummaryResponse, len(accounts))
	for i := range accounts {
		resp[i] = dto.ToAccountSummaryResponse(&accounts[i])
	}
	c.JSON(http.StatusOK, resp)
}

// GetAccountDetails godoc
// @Summary Get account details
// @Description Returns full details for a specific account owned by the authenticated client
// @Tags accounts
// @Produce json
// @Param accountNumber path string true "Account number"
// @Param clientId path string true "Client id"
// @Success 200 {object} dto.AccountResponse
// @Failure 400 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/accounts/{accountNumber} [get]

func (h *AccountHandler) GetAccountDetails(c *gin.Context) {
	clientId, ok := GetParamUint(c, "clientId")
	if !ok {
		return
	}

	accountNumber := c.Param("accountNumber")
	account, err := h.service.GetAccountDetails(c.Request.Context(), accountNumber, clientId)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.ToAccountResponse(account))
}

// UpdateAccountName godoc
// @Summary Update account name
// @Description Updates the display name of an account owned by the authenticated client
// @Tags accounts
// @Accept json
// @Produce json
// @Param accountNumber path string true "Account number"
// @Param request body dto.UpdateAccountNameRequest true "New account name"
// @Success 200
// @Failure 400 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/accounts/{accountNumber}/name [get]
func (h *AccountHandler) UpdateAccountName(c *gin.Context) {
	clientId, ok := GetParamUint(c, "clientId")
	if !ok {
		return
	}

	accountNumber := c.Param("accountNumber")

	var req dto.UpdateAccountNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	if err := h.service.UpdateAccountName(c.Request.Context(), accountNumber, clientId, req.Name); err != nil {
		c.Error(err)
		return
	}

	c.Status(http.StatusOK)
}

// RequestLimitsChange godoc
// @Summary Request account limit change
// @Description Initiates a limit change request for an account. Sends a verification code for confirmation via mobile app, temporarily returned in body as code.
// @Tags accounts
// @Accept json
// @Produce json
// @Param accountNumber path string true "Account number"
// @Param request body dto.RequestLimitsChangeRequest true "New daily and monthly limits"
// @Success 200 {object} dto.RequestLimitsChangeResponse
// @Failure 400 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/accounts/{accountNumber}/limits/request [post]
func (h *AccountHandler) RequestLimitsChange(c *gin.Context) {
	clientId, ok := GetParamUint(c, "clientId")
	if !ok {
		return
	}

	accountNumber := c.Param("accountNumber")

	var req dto.RequestLimitsChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	code, err := h.service.RequestLimitsChange(c.Request.Context(), accountNumber, clientId, req.DailyLimit, req.MonthlyLimit)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.RequestLimitsChangeResponse{Code: code}) //only for testing purposes, in a real app it would be sent to mobile app
}

// ConfirmLimitsChange godoc
// @Summary Confirm account limit change
// @Description Confirms a pending limit change request using the verification code from the mobile app
// @Tags accounts
// @Accept json
// @Produce json
// @Param accountNumber path string true "Account number"
// @Param request body dto.ConfirmLimitsChangeRequest true "Verification code, you can use 1234 for testing but still needs request to be made first"
// @Success 200
// @Failure 400 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/accounts/{accountNumber}/limits [put]
func (h *AccountHandler) ConfirmLimitsChange(c *gin.Context) {
	clientId, ok := GetParamUint(c, "clientId")
	if !ok {
		return
	}

	accountNumber := c.Param("accountNumber")

	var req dto.ConfirmLimitsChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	if err := h.service.ConfirmLimitsChange(c.Request.Context(), accountNumber, clientId, req.Code); err != nil {
		c.Error(err)
		return
	}

	c.Status(http.StatusOK)
}
