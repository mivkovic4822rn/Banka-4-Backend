package model

import "time"

type LoanRequestStatus string

const (
	LoanRequestPending  LoanRequestStatus = "PENDING"
	LoanRequestApproved LoanRequestStatus = "APPROVED"
	LoanRequestRejected LoanRequestStatus = "REJECTED"
)

type LoanType struct {
	LoanTypeID         uint   `gorm:"primaryKey"`
	Name               string `gorm:"size:100;not null"`
	Description        string
	BankMargin         float64
	BaseInterestRate   float64
	MinRepaymentPeriod int
	MaxRepaymentPeriod int
}

type LoanRequest struct {
	ID                 uint    `gorm:"primaryKey"`
	ClientID           uint    `gorm:"not null"`
	AccountNumber      string  `gorm:"size:20;not null"`
	LoanTypeID         uint    `gorm:"not null"`
	Amount             float64 `gorm:"not null"`
	RepaymentPeriod    int     `gorm:"not null"`
	CalculatedRate     float64
	MonthlyInstallment float64
	Status             LoanRequestStatus `gorm:"size:20;default:'PENDING'"`
	CreatedAt          time.Time

	LoanType LoanType
}

type LoanStatus string

const (
	LoanStatusActive    LoanStatus = "ACTIVE"
	LoanStatusCompleted LoanStatus = "COMPLETED"
)

type InstallmentStatus string

const (
	InstallmentStatusPending  InstallmentStatus = "PENDING"
	InstallmentStatusPaid     InstallmentStatus = "PAID"
	InstallmentStatusUnpaid   InstallmentStatus = "UNPAID"
	InstallmentStatusRetrying InstallmentStatus = "RETRYING"
)

type Loan struct {
	ID            uint `gorm:"primaryKey"`
	LoanRequestID uint `gorm:"not null;uniqueIndex"`
	LoanRequest   LoanRequest

	TransactionID *uint `gorm:"index"`

	MonthlyInstallment float64 `gorm:"not null"`
	InterestRate       float64 `gorm:"not null"`
	IsVariableRate     bool    `gorm:"not null;default:false"`

	RemainingDebt    float64 `gorm:"not null"`
	RepaymentPeriod  int     `gorm:"not null"`
	PaidInstallments int     `gorm:"not null;default:0"`

	StartDate           time.Time `gorm:"not null"`
	NextInstallmentDate time.Time `gorm:"not null;index"`

	Status LoanStatus `gorm:"size:20;not null;default:'ACTIVE'"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Installments []LoanInstallment
}

type LoanInstallment struct {
	ID     uint `gorm:"primaryKey"`
	LoanID uint `gorm:"not null;index"`
	Loan   Loan

	InstallmentNumber int     `gorm:"not null"`
	Amount            float64 `gorm:"not null"`
	InterestRate      float64 `gorm:"not null"`

	TransactionID *uint

	DueDate time.Time `gorm:"not null;index"`
	PaidAt  *time.Time
	RetryAt *time.Time

	Status InstallmentStatus `gorm:"size:20;not null;default:'PENDING'"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
