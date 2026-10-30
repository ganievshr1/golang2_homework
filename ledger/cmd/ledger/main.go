package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"example.com/finance-services/ledger"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
	)
	defer stop()

	if err := ledger.Connect(ctx); err != nil {
		return err
	}
	defer ledger.Close()

	log.Print(
		"Ledger готов. Данные хранятся в PostgreSQL. Остановка: Ctrl+C",
	)

	<-ctx.Done()

	return nil
}
