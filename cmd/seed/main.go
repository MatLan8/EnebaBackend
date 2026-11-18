// Command seed fills the catalog tables (genres, sellers, products, offers and
// digital keys) from seed/data/seed.json, the cleaned-up export of the scraped
// Eneba data, and inserts the seed users. Every seed user logs in with
// devPassword ("Password123!"). Run it from the project root:
//
//	go run ./cmd/seed          # only works while the products table is empty
//	go run ./cmd/seed -reset   # wipes the catalog tables first, then migrates and seeds again
//
// -reset does not delete users or refresh tokens. Re-running the seed skips
// emails that already exist, including real accounts.
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

	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// -reset wipes the catalog before the migrations run, because a migration
	// can depend on the new seed data: 000004 makes release_date NOT NULL and
	// would fail on old rows that still have no date, and 000005 adds the
	// NOT NULL base_price_cents column, which fails while products has any rows.
	if *reset {
		if err := truncateCatalog(ctx, pool); err != nil {
			return err
		}
	}

	// The catalog tables come from migration 000003, so make sure it is applied.
	if err := database.Migrate(cfg.DatabaseURL, "migrations"); err != nil {
		return err
	}

	// Hash once and reuse it for every seed user. bcrypt would otherwise
	// spend a second or so per user, for the same password.
	passwordHash, err := services.HashPassword(devPassword)
	if err != nil {
		return err
	}

	// One transaction: either the whole catalog and the new users are
	// inserted, or nothing is.
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
