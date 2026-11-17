// Command seed fills the catalog tables (genres, sellers, products, offers and
// digital keys) from seed/data/seed.json, the cleaned-up export of the scraped
// Eneba data. Run it from the project root:
//
//	go run ./cmd/seed          # only works while the products table is empty
//	go run ./cmd/seed -reset   # wipes the catalog tables first, then seeds again
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"

	"eneba-backend/internal/config"
	"eneba-backend/internal/database"
	"eneba-backend/internal/services"
)

func main() {
	if err := run(); err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	file := flag.String("file", "seed/data/seed.json", "path to the seed data file")
	reset := flag.Bool("reset", false, "delete all catalog data before seeding")
	flag.Parse()

	// Same setup as cmd/api: .env for local runs, then the validated config.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	data, err := loadSeedFile(*file)
	if err != nil {
		return err
	}

	keyCipher, err := services.NewKeyCipher(cfg.KeyEncryptionSecret)
	if err != nil {
		return err
	}

	// The catalog tables come from migration 000003, so make sure it is applied.
	if err := database.Migrate(cfg.DatabaseURL, "migrations"); err != nil {
		return err
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// One transaction: either the whole catalog is inserted or nothing is.
	var summary seedSummary
	err = database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if *reset {
			if err := truncateCatalog(ctx, tx); err != nil {
				return err
			}
		} else if err := ensureCatalogEmpty(ctx, tx); err != nil {
			return err
		}

		result, err := seedCatalog(ctx, tx, data, keyCipher)
		if err != nil {
			return err
		}
		summary = result
		return nil
	})
	if errors.Is(err, errCatalogNotEmpty) {
		fmt.Println("The products table already has data. Run again with -reset to replace it.")
		return nil
	}
	if err != nil {
		return err
	}

	fmt.Printf(
		"Seed complete: %d genres, %d sellers, %d products, %d product genres, %d offers, %d digital keys\n",
		summary.Genres, summary.Sellers, summary.Products, summary.ProductGenres, summary.Offers, summary.DigitalKeys,
	)
	return nil
}
