package main

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// devPassword is shared by every seed user. Never use it in production.
const devPassword = "Password123!"

const (
	userNameMinLen  = 2
	userNameMaxLen  = 50
	userEmailMaxLen = 254
)

type seedUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func normalizeUsers(users []seedUser) {
	for i := range users {
		users[i].Name = strings.TrimSpace(users[i].Name)
		users[i].Email = strings.TrimSpace(users[i].Email)
	}
}

// validateUsers applies the same limits as POST /api/v1/auth/register, before any database work.
func validateUsers(users []seedUser) error {
	seen := make(map[string]int, len(users))
	for i, u := range users {
		if err := validateUser(i, u); err != nil {
			return err
		}

		key := strings.ToLower(u.Email)
		if first, ok := seen[key]; ok {
			return fmt.Errorf("users %d and %d: duplicate email %q", first, i, u.Email)
		}
		seen[key] = i
	}
	return nil
}

func validateUser(index int, u seedUser) error {
	label := fmt.Sprintf("user %d", index)
	if u.Name != "" {
		label = fmt.Sprintf("user %d %q", index, u.Name)
	}

	nameLen := utf8.RuneCountInString(u.Name)
	if u.Name == "" {
		return fmt.Errorf("%s: name is required", label)
	}
	if nameLen < userNameMinLen || nameLen > userNameMaxLen {
		return fmt.Errorf("%s: name must be between %d and %d characters", label, userNameMinLen, userNameMaxLen)
	}

	if u.Email == "" {
		return fmt.Errorf("%s: email is required", label)
	}
	if u.Email != strings.ToLower(u.Email) {
		return fmt.Errorf("%s: email %q must be lowercase", label, u.Email)
	}
	if len(u.Email) > userEmailMaxLen {
		return fmt.Errorf("%s: email must be at most %d characters", label, userEmailMaxLen)
	}
	if !validSeedEmail(u.Email) {
		return fmt.Errorf("%s: invalid email %q", label, u.Email)
	}
	return nil
}

func validSeedEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return false
	}

	at := strings.LastIndexByte(email, '@')
	return at > 0 && strings.Contains(email[at+1:], ".")
}

// Existing emails, including real accounts, are skipped and left unchanged.
func seedUsers(ctx context.Context, tx pgx.Tx, users []seedUser, passwordHash string) (inserted, skipped int, err error) {
	if len(users) == 0 {
		return 0, 0, nil
	}

	query := psql.Insert("users").Columns("name", "email", "password_hash")
	for _, u := range users {
		query = query.Values(u.Name, u.Email, passwordHash)
	}
	query = query.Suffix("ON CONFLICT (email) DO NOTHING")

	sql, args, err := query.ToSql()
	if err != nil {
		return 0, 0, fmt.Errorf("build insert into users: %w", err)
	}

	tag, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		return 0, 0, fmt.Errorf("insert into users: %w", err)
	}

	inserted = int(tag.RowsAffected())
	skipped = len(users) - inserted
	return inserted, skipped, nil
}
