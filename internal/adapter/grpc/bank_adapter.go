package grpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/fbriansyah/my-grpc-proto/protogen/go/bank"
	"github.com/google/uuid"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/datetime"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbank "github.com/fbriansyah/my-grpc-go-server/internal/application/domain/bank"
)

func (a *GrpcAdapter) GetCurrentBalance(ctx context.Context,
	req *bank.CurrentBalanceRequest) (*bank.CurrentBalanceResponse, error) {

	now := time.Now()
	bal, err := a.bankService.FindCurrentBalance(req.AccountNumber)

	if err != nil {
		s := status.New(
			codes.FailedPrecondition,
			fmt.Sprintf("account %v not found", req.AccountNumber),
		)
		s, _ = s.WithDetails(&errdetails.ErrorInfo{})
		return nil, s.Err()
	}

	return &bank.CurrentBalanceResponse{
		Amount: bal,
		CurrentDate: &date.Date{
			Year:  int32(now.Year()),
			Month: int32(now.Month()),
			Day:   int32(now.Day()),
		},
	}, nil

}

func (a *GrpcAdapter) FetchExchangeRates(req *bank.ExchangeRateRequest,
	stream bank.BankService_FetchExchangeRatesServer) error {

	context := stream.Context()

	for {
		select {
		case <-context.Done():
			log.Println("Client cancelled stream")
			return nil
		default:
			now := time.Now().Truncate(time.Second)

			rate, err := a.bankService.FindExchangeRate(req.FromCurrency, req.ToCurrency, now)

			if err != nil {
				s := status.New(
					codes.InvalidArgument,
					"Currency not valid, Please use valid currency for both from and to",
				)
				s, _ = s.WithDetails(&errdetails.ErrorInfo{
					Domain: "my-bank-website.com",
					Reason: "INVALID_CURRENCY",
					Metadata: map[string]string{
						"from_currency": req.FromCurrency,
						"to_currency":   req.ToCurrency,
					},
				})

				return s.Err()
			}

			stream.Send(&bank.ExchangeRateResponse{
				FromCurrency: req.FromCurrency,
				ToCurrency:   req.ToCurrency,
				Rate:         rate,
				Timestamp:    now.Format(time.RFC3339),
			})

			log.Printf("Exchange rate sent to client, %v to %v : %v\n",
				req.FromCurrency, req.ToCurrency, rate)

			time.Sleep(3 * time.Second)
		}

	}
}

func (a *GrpcAdapter) SummarizeTransactions(stream bank.BankService_SummarizeTransactionsServer) error {
	tSum := dbank.TransactionSummary{
		SumIn:         0,
		SumOut:        0,
		SumTotal:      0,
		SummaryOnDate: time.Now(),
	}
	acct := ""

	for {
		req, err := stream.Recv()

		if err == io.EOF {
			res := bank.TransactionSummary{
				AccountNumber: acct,
				SumAmountIn:   tSum.SumIn,
				SumAmountOut:  tSum.SumOut,
				SumTotal:      tSum.SumTotal,
				TransactionDate: &date.Date{
					Year:  int32(tSum.SummaryOnDate.Day()),
					Month: int32(tSum.SummaryOnDate.Month()),
					Day:   int32(tSum.SummaryOnDate.Day()),
				},
			}
			// send calculated transaction
			return stream.SendAndClose(&res)
		}

		if err != nil {
			log.Fatalln("Error while reading from client :", err)
		}

		acct = req.AccountNumber
		ts, err := toTime(req.Timestamp)

		if err != nil {
			log.Fatalf("Error while parsing timestamp %v: %v\n", req.Timestamp, err)
		}

		ttype := dbank.TransactionTypeUnknown

		if req.Type == bank.TransactionType_TRANSACTION_TYPE_IN {
			ttype = dbank.TransactionTypeIn
		} else if req.Type == bank.TransactionType_TRANSACTION_TYPE_OUT {
			ttype = dbank.TransactionTypeOut
		}

		tcur := dbank.Transaction{
			Amount:          req.Amount,
			Timestamp:       ts,
			TransactionType: ttype,
		}

		accountUuid, err := a.bankService.CreateTransaction(req.AccountNumber, tcur)

		if err != nil && accountUuid == uuid.Nil {
			s := status.New(codes.InvalidArgument, err.Error())
			s, _ = s.WithDetails(&errdetails.BadRequest{
				FieldViolations: []*errdetails.BadRequest_FieldViolation{
					{Field: "account_number", Description: "Invalid account number"},
				},
			})

			return s.Err()
		} else if err != nil && accountUuid != uuid.Nil {
			s := status.New(codes.InvalidArgument, err.Error())

			s, _ = s.WithDetails(&errdetails.BadRequest{
				FieldViolations: []*errdetails.BadRequest_FieldViolation{
					{
						Field:       "amount",
						Description: fmt.Sprintf("Requested amount %v exceed available balance", req.Amount),
					},
				},
			})

			return s.Err()
		}

		if err != nil {
			log.Println("Error while CreateTransaction :", err)
		}

		err = a.bankService.CalculateTransaction(&tSum, tcur)

		if err != nil {
			return err
		}
	}

}

func (a *GrpcAdapter) TransferMultiple(stream bank.BankService_TransferMultipleServer) error {
	ctx := stream.Context()

	for {
		select {
		case <-ctx.Done():
			log.Println("Client cancelled stream")
			return nil
		default:
			req, err := stream.Recv()

			if err == io.EOF {
				return nil
			}

			if err != nil {
				log.Fatalln("Error while reading from client : ", err)
			}

			tt := dbank.TransferTransaction{
				FromAccountNumber: req.FromAccountNumber,
				ToAccountNumber:   req.ToAccountNumber,
				Currency:          req.Currency,
				Amount:            req.Amount,
			}

			_, transferSuccess, err := a.bankService.Transfer(tt)
			if err != nil {
				return buildTransferErrorStatusGrpc(err, *req)
			}

			res := bank.TransferResponse{
				FromAccountNumber: req.FromAccountNumber,
				ToAccountNumber:   req.ToAccountNumber,
				Currency:          req.Currency,
				Amount:            req.Amount,
				Timestamp:         currentDateTime(),
			}

			if transferSuccess {
				res.Status = bank.TransferStatus_TRANSFER_STATUS_SUCCESS
			} else {
				res.Status = bank.TransferStatus_TRANSFER_STATUS_FAILED
			}

			err = stream.Send(&res)
			if err != nil {
				log.Fatalln("Error while sending response to client : ", err)
			}

		}
	}

}

func toTime(dt *datetime.DateTime) (time.Time, error) {

	if dt == nil {
		now := time.Now()

		dt = &datetime.DateTime{
			Year:    int32(now.Year()),
			Month:   int32(now.Month()),
			Day:     int32(now.Day()),
			Hours:   int32(now.Hour()),
			Minutes: int32(now.Minute()),
			Seconds: int32(now.Second()),
			Nanos:   int32(now.Nanosecond()),
		}
	}

	res := time.Date(int(dt.Year), time.Month(dt.Month), int(dt.Day), int(dt.Hours),
		int(dt.Minutes), int(dt.Seconds), int(dt.Nanos), time.UTC)

	return res, nil
}

func currentDateTime() *datetime.DateTime {
	now := time.Now()

	return &datetime.DateTime{
		Year:       int32(now.Year()),
		Month:      int32(now.Month()),
		Day:        int32(now.Day()),
		Hours:      int32(now.Hour()),
		Minutes:    int32(now.Minute()),
		Seconds:    int32(now.Second()),
		Nanos:      int32(now.Nanosecond()),
		TimeOffset: &datetime.DateTime_UtcOffset{},
	}
}

func buildTransferErrorStatusGrpc(err error, req bank.TransferRequest) error {
	switch {
	case errors.Is(err, dbank.ErrTransferSourceAccountNotFound):
		s := status.New(codes.FailedPrecondition, err.Error())
		s, _ = s.WithDetails(&errdetails.PreconditionFailure{
			Violations: []*errdetails.PreconditionFailure_Violation{
				{
					Type:        "INVALID_ACCOUNT",
					Subject:     "Source account not found",
					Description: fmt.Sprintf("source account (from %v) not found", req.FromAccountNumber),
				},
			},
		})

		return s.Err()
	case errors.Is(err, dbank.ErrTransferDestinationAccountNotFound):
		s := status.New(codes.FailedPrecondition, err.Error())
		s, _ = s.WithDetails(&errdetails.PreconditionFailure{
			Violations: []*errdetails.PreconditionFailure_Violation{
				{
					Type:        "INVALID_ACCOUNT",
					Subject:     "Destination account not found",
					Description: fmt.Sprintf("destination account (to %v) not found", req.ToAccountNumber),
				},
			},
		})

		return s.Err()
	case errors.Is(err, dbank.ErrTransferRecordFailed):
		s := status.New(codes.Internal, err.Error())
		s, _ = s.WithDetails(&errdetails.Help{
			Links: []*errdetails.Help_Link{
				{
					Url:         "my-bank-website.com/faq",
					Description: "Bank FAQ",
				},
			},
		})

		return s.Err()
	case errors.Is(err, dbank.ErrTransferTransactionPair):
		s := status.New(codes.InvalidArgument, err.Error())
		s, _ = s.WithDetails(&errdetails.ErrorInfo{
			Domain: "my-bank-website.com",
			Reason: "TRANSACTION_PAIR_FAILED",
			Metadata: map[string]string{
				"from_account": req.FromAccountNumber,
				"to_account":   req.ToAccountNumber,
				"currency":     req.Currency,
				"amount":       fmt.Sprintf("%f", req.Amount),
			},
		})

		return s.Err()
	default:
		s := status.New(codes.Unknown, err.Error())
		return s.Err()
	}
}
