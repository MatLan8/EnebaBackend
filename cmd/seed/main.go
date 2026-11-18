// Command seed fills the catalog from seed/data/seed.json and adds the seed users (password: devPassword).
// Run from the project root: go run ./cmd/seed [-reset]. -reset empties the catalog, carts and orders
// first but keeps users; existing emails are skipped.
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
	reset := flag.Bool("reset", false, "delete all catalog data before seeding; users and refresh tokens are kept")
	flag.Parse()

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

	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// -reset wipes the catalog before migrating, because 000004 (release_date NOT NULL) and
	// 000005 (NOT NULL base_price_cents) fail on old product rows.
	if *reset {
		if err := truncateCatalog(ctx, pool); err != nil {
			return err
		}
	}

	if err := database.Migrate(cfg.DatabaseURL, "migrations"); err != nil {
		return err
	}

	// Hash once for all seed users; bcrypt takes about a second per hash.
	passwordHash, err := services.HashPassword(devPassword)
	if err != nil {
		return err
	}

	var summary seedSummary
	err = database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if err := ensureCatalogEmpty(ctx, tx); err != nil {
			return err
		}

		inserted, skipped, err := seedUsers(ctx, tx, data.Users, passwordHash)
		if err != nil {
			return err
		}

		result, err := seedCatalog(ctx, tx, data, keyCipher)
		if err != nil {
			return err
		}
		result.UsersInserted = inserted
		result.UsersSkipped = skipped
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
		"Seed complete: %d genres, %d sellers, %d products, %d product genres, %d offers, %d digital keys, %d users inserted, %d users skipped\n",
		summary.Genres, summary.Sellers, summary.Products, summary.ProductGenres, summary.Offers, summary.DigitalKeys,
		summary.UsersInserted, summary.UsersSkipped,
	)
	return nil
}
