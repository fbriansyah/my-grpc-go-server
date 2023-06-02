package port

import (
	"time"

	dbank "github.com/fbriansyah/my-grpc-go-server/internal/application/domain/bank"
	"github.com/google/uuid"
)

type HelloServicePort interface {
	GenerateHello(string) string
}

type BankServicePort interface {
	FindCurrentBalance(string) (float64, error)
	CreateExchangeRate(d dbank.ExchangeRate) (uuid.UUID, error)
	FindExchangeRate(fromCur string, toCur string, ts time.Time) (float64, error)
	CreateTransaction(acct string, t dbank.Transaction) (uuid.UUID, error)
	CalculateTransaction(tcur *dbank.TransactionSummary, trans dbank.Transaction) error
	Transfer(tt dbank.TransferTransaction) (uuid.UUID, bool, error)
}

type ResiliencyServicePort interface {
	GenerateResiliency(minDelaySecond int32, maxDelaySecond int32, statusCodes []uint32) (string, uint32)
}
