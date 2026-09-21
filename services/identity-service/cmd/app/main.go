package main

import (
	"log/slog"
	"os"

	"github.com/ZheglY/SmartQuarter/services/identity-service/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		slog.Error("application startup error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
