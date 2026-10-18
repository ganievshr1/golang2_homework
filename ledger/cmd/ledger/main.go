package main

import (
	"log"
	"os"

	"example.com/finance-services/ledger"
)

func main() {
	if err := ledger.RunDemo("budgets.json", os.Stdout); err != nil {
		log.Fatal(err)
	}
}
