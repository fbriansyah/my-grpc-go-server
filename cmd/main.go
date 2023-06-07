package main

import (
	"database/sql"
	"log"
	"math/rand"
	"time"

	dbmigration "github.com/fbriansyah/my-grpc-go-server/db"
	_ "github.com/jackc/pgx/v4/stdlib"

	mydb "github.com/fbriansyah/my-grpc-go-server/internal/adapter/database"
	mygrpc "github.com/fbriansyah/my-grpc-go-server/internal/adapter/grpc"
	app "github.com/fbriansyah/my-grpc-go-server/internal/application"
	"github.com/fbriansyah/my-grpc-go-server/internal/application/domain/bank"
)

func main() {
	log.SetFlags(0)
	log.SetOutput(&logWriter{})

	sqlDB, err := sql.Open("pgx", "postgres://postgres:ZfJ6MV_Ex!ab@localhost:5432/grpc?sslmode=disable")

	if err != nil {
		log.Fatalln("Can't connect database", err)
	}

	dbmigration.Migrate(sqlDB)

	databaseadapter, err := mydb.NewDatabaseAdapter(sqlDB)
	if err != nil {
		log.Fatalln("Can't create db adapter", err)
	}

	hs := &app.ApplicationService{}
	bs := app.NewBankService(databaseadapter)
	rs := &app.ResiliencyService{}

	go generateExchangeRates(bs, "USD", "IDR", (5 * time.Second))

	grpcAdapter := mygrpc.NewGrpcAdapter(hs, bs, rs, 9090)

	grpcAdapter.Run()
}

func generateExchangeRates(bs *app.BankService, fromCurrency, toCurrency string, duration time.Duration) {
	ticker := time.NewTicker(duration)

	for range ticker.C {
		now := time.Now()
		validFrom := now.Truncate(time.Second).Add(3 * time.Second)
		validTo := validFrom.Add(duration).Add(-1 * time.Millisecond)

		dummyRate := bank.ExchangeRate{
			FromCurrency:       fromCurrency,
			ToCurrency:         toCurrency,
			ValidFromTimestamp: validFrom,
			ValidToTimestamp:   validTo,
			Rate:               2000 + float64(rand.Intn(300)),
		}

		bs.CreateExchangeRate(dummyRate)
	}
}
