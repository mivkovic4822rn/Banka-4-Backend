package service

import (
	"common/pkg/errors"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
	"user-service/internal/dto"
	"user-service/internal/model"
	"user-service/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

type EmployeeService struct {
	repo           repository.EmployeeRepository
	tokenRepo      repository.ActivationTokenRepository
	resetTokenRepo repository.ResetTokenRepository
	emailService   *EmailService
}

func NewEmployeeService(
	repo repository.EmployeeRepository,
	tokenRepo repository.ActivationTokenRepository,
	resetTokenRepo repository.ResetTokenRepository,
	emailService *EmailService,
) *EmployeeService {
	return &EmployeeService{
		repo:           repo,
		tokenRepo:      tokenRepo,
		resetTokenRepo: resetTokenRepo,
		emailService:   emailService,
	}
}

func (s *EmployeeService) Register(ctx context.Context, req *dto.CreateEmployeeRequest) (*model.Employee, error) {

	existing, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if existing != nil {
		return nil, errors.ConflictErr("email already in use")
	}

	existingByUsername, err := s.repo.FindByUserName(ctx, req.Username)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if existingByUsername != nil {
		return nil, errors.ConflictErr("username already in use")
	}

	employee := &model.Employee{
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		Gender:      req.Gender,
		DateOfBirth: req.DateOfBirth,
		Email:       req.Email,
		PhoneNumber: req.PhoneNumber,
		Address:     req.Address,
		Username:    req.Username,
		Department:  req.Department,
		PositionID:  req.PositionID,
		Active:      req.Active,
	}

	if err := s.repo.Create(ctx, employee); err != nil {
		return nil, errors.InternalErr(err)
	}

	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, errors.InternalErr(err)
	}
	tokenStr := hex.EncodeToString(tokenBytes)

	activationToken := &model.ActivationToken{
		EmployeeID: employee.EmployeeID,
		Token:      tokenStr,
		ExpiresAt:  time.Now().Add(24 * time.Hour),
	}

	if err := s.tokenRepo.Create(ctx, activationToken); err != nil {
		return nil, errors.InternalErr(err)
	}

	link := fmt.Sprintf("http://localhost:8080/activate?token=%s", tokenStr)
	s.emailService.Send(
		employee.Email,
		"Welcome!",
		fmt.Sprintf("Kliknite ovde da postavite lozinku: %s", link),
	)

	return employee, nil
}

func (s *EmployeeService) ActivateAccount(ctx context.Context, tokenStr, password string) error {
	activationToken, err := s.tokenRepo.FindByToken(ctx, tokenStr)
	if err != nil || activationToken == nil {
		return errors.BadRequestErr("invalid or expired token")
	}

	if activationToken.ExpiresAt.Before(time.Now()) {
		return errors.BadRequestErr("token expired")
	}

	employee, err := s.repo.FindByID(ctx, activationToken.EmployeeID)
	if err != nil {
		return errors.InternalErr(err)
	}
	if employee == nil {
		return errors.ConflictErr("employee not found")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return errors.InternalErr(err)
	}

	employee.Password = string(hashedPassword)
	if err := s.repo.Update(ctx, employee); err != nil {
		return errors.InternalErr(err)
	}

	_ = s.tokenRepo.Delete(ctx, activationToken)

	s.emailService.Send(employee.Email, "Account activated", "Vaš nalog je uspešno aktiviran.")

	return nil
}

func (s *EmployeeService) GetAllEmployees(ctx context.Context, query *dto.ListEmployeesQuery) (*dto.ListEmployeesResponse, error) {
	employees, total, err := s.repo.GetAll(ctx, query.Email, query.FirstName, query.LastName, query.Position, query.Page, query.PageSize)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return dto.ToEmployeeResponseList(employees, total, query.Page, query.PageSize), nil
}

func (s *EmployeeService) RequestPasswordReset(ctx context.Context, email string) error {
	employee, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return errors.InternalErr(err)
	}
	if employee == nil {
		return nil
	}

	if err := s.resetTokenRepo.DeleteByEmployeeID(ctx, employee.EmployeeID); err != nil {
		return errors.InternalErr(err)
	}

	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return errors.InternalErr(err)
	}
	code := hex.EncodeToString(tokenBytes)

	resetToken := &model.ResetToken{
		EmployeeID: employee.EmployeeID,
		Code:       code,
		ExpiresAt:  time.Now().Add(15 * time.Minute),
	}

	if err := s.resetTokenRepo.Create(ctx, resetToken); err != nil {
		return errors.InternalErr(err)
	}

	link := fmt.Sprintf("http://localhost:8080/reset-password?code=%s", code)
	s.emailService.Send(
		employee.Email,
		"Password reset",
		fmt.Sprintf("Kliknite ovde da resetujete lozinku: %s", link),
	)

	return nil
}

func (s *EmployeeService) ConfirmPasswordReset(ctx context.Context, code, newPassword string) error {
	resetToken, err := s.resetTokenRepo.FindByCode(ctx, code)
	if err != nil {
		return errors.InternalErr(err)
	}
	if resetToken == nil {
		return errors.BadRequestErr("invalid or expired code")
	}

	if resetToken.ExpiresAt.Before(time.Now()) {
		_ = s.resetTokenRepo.DeleteByEmployeeID(ctx, resetToken.EmployeeID)
		return errors.BadRequestErr("code has expired")
	}

	employee, err := s.repo.FindByID(ctx, resetToken.EmployeeID)
	if err != nil {
		return errors.InternalErr(err)
	}
	if employee == nil {
		return errors.NotFoundErr("employee not found")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.InternalErr(err)
	}

	employee.Password = string(hashedPassword)
	if err := s.repo.Update(ctx, employee); err != nil {
		return errors.InternalErr(err)
	}

	_ = s.resetTokenRepo.DeleteByEmployeeID(ctx, employee.EmployeeID)

	s.emailService.Send(
		employee.Email,
		"Password changed",
		"Vaša lozinka je uspešno promenjena.",
	)

	return nil
}
