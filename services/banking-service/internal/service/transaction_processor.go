package service

import (
	"banking-service/internal/model"
	"banking-service/internal/repository"
	"common/pkg/errors"
	"context"
)

var BankAccounts = map[model.CurrencyCode]string{
	model.RSD: "444000000000000000",
	model.EUR: "444000000000000001",
	model.USD: "444000000000000002",
	model.CHF: "444000000000000003",
	model.GBP: "444000000000000004",
	model.JPY: "444000000000000005",
	model.CAD: "444000000000000006",
	model.AUD: "444000000000000007",
}

type TransactionProcessor struct {
	accountRepo     repository.AccountRepository
	transactionRepo repository.TransactionRepository
	txManager       repository.TransactionManager
}

func NewTransactionProcessor(accountRepo repository.AccountRepository, transactionRepo repository.TransactionRepository, txManager repository.TransactionManager) *TransactionProcessor {
	return &TransactionProcessor{accountRepo: accountRepo, transactionRepo: transactionRepo, txManager: txManager}
}

func (tp *TransactionProcessor) Process(ctx context.Context, transactionID uint) error {
	return tp.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		transaction, err := tp.transactionRepo.GetByID(ctx, transactionID)
		if err != nil {
			return errors.InternalErr(err)
		}

		if transaction.Status != model.TransactionProcessing {
			return errors.BadRequestErr("transaction already processed")
		}

		payer, err := tp.accountRepo.FindByAccountNumber(ctx, transaction.PayerAccountNumber)
		if err != nil {
			return errors.InternalErr(err)
		}

		// Check funds
		if payer.AvailableBalance < transaction.StartAmount {
			return errors.BadRequestErr("insufficient payer funds")
		}

		// Check limits
		if payer.DailySpending+transaction.StartAmount > payer.DailyLimit {
			return errors.BadRequestErr("daily limit exceeded")
		}

		if payer.MonthlySpending+transaction.StartAmount > payer.MonthlyLimit {
			return errors.BadRequestErr("monthly limit exceeded")
		}

		recipient, err := tp.accountRepo.FindByAccountNumber(ctx, transaction.RecipientAccountNumber)
		if err != nil {
			return errors.InternalErr(err)
		}

		if recipient.AccountNumber == payer.AccountNumber {
			return errors.BadRequestErr("cannot make payment to the same account")
		}

		for _, acc := range BankAccounts {
			if recipient.AccountNumber == acc {
				return errors.BadRequestErr("recipient account cannot be one of the banks accounts")
			}
		}

		if transaction.StartCurrencyCode != transaction.EndCurrencyCode {
			banksAccountTo, err := tp.accountRepo.FindByAccountNumber(ctx, BankAccounts[transaction.StartCurrencyCode])
			if err != nil {
				return errors.InternalErr(err)
			}

			banksAccountFrom, err := tp.accountRepo.FindByAccountNumber(ctx, BankAccounts[transaction.EndCurrencyCode])
			if err != nil {
				return errors.InternalErr(err)
			}

			if banksAccountFrom.AvailableBalance < transaction.EndAmount {
				return errors.BadRequestErr("insufficient banks funds")
			}

			model.UpdateBalances(payer, -transaction.StartAmount)
			model.UpdateBalances(banksAccountTo, transaction.StartAmount)
			model.UpdateBalances(banksAccountFrom, -transaction.EndAmount)
			model.UpdateBalances(recipient, transaction.EndAmount)

			if err := tp.accountRepo.UpdateBalance(ctx, payer); err != nil {
				return errors.InternalErr(err)
			}
			if err := tp.accountRepo.UpdateBalance(ctx, banksAccountTo); err != nil {
				return errors.InternalErr(err)
			}
			if err := tp.accountRepo.UpdateBalance(ctx, banksAccountFrom); err != nil {
				return errors.InternalErr(err)
			}
			if err := tp.accountRepo.UpdateBalance(ctx, recipient); err != nil {
				return errors.InternalErr(err)
			}
		} else {
			model.UpdateBalances(payer, -transaction.StartAmount)
			model.UpdateBalances(recipient, transaction.EndAmount)

			if err := tp.accountRepo.UpdateBalance(ctx, payer); err != nil {
				return errors.InternalErr(err)
			}
			if err := tp.accountRepo.UpdateBalance(ctx, recipient); err != nil {
				return errors.InternalErr(err)
			}
		}

		transaction.Status = model.TransactionCompleted
		return tp.transactionRepo.Update(ctx, transaction)
	})
}
func (tp *TransactionProcessor) ProcessLoanInstallment(ctx context.Context, transactionID uint) error {
	return tp.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		transaction, err := tp.transactionRepo.GetByID(ctx, transactionID)
		if err != nil {
			return errors.InternalErr(err)
		}

		if transaction.Status != model.TransactionProcessing {
			return errors.BadRequestErr("transaction already processed")
		}

		payer, err := tp.accountRepo.FindByAccountNumber(ctx, transaction.PayerAccountNumber)
		if err != nil {
			return errors.InternalErr(err)
		}

		if payer.AvailableBalance < transaction.StartAmount {
			return errors.BadRequestErr("insufficient payer funds")
		}

		recipient, err := tp.accountRepo.FindByAccountNumber(ctx, transaction.RecipientAccountNumber)
		if err != nil {
			return errors.InternalErr(err)
		}

		model.UpdateBalances(payer, -transaction.StartAmount)
		model.UpdateBalances(recipient, transaction.EndAmount)

		if err := tp.accountRepo.UpdateBalance(ctx, payer); err != nil {
			return errors.InternalErr(err)
		}
		if err := tp.accountRepo.UpdateBalance(ctx, recipient); err != nil {
			return errors.InternalErr(err)
		}

		transaction.Status = model.TransactionCompleted
		return tp.transactionRepo.Update(ctx, transaction)
	})
}
