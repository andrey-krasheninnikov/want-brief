package main

import (
	"context"
	"log"

	"github.com/pchkauu/want-brief/internal/bootstrap"
)

func main() {
	cfg := bootstrap.LoadConfig()
	if err := bootstrap.Run(context.Background(), cfg); err != nil {
		log.Fatal(err)
	}
}
