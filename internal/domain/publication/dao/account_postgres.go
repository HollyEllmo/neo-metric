package dao

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AccountPostgres implements AccountRepository using existing Laravel tables
type AccountPostgres struct {
	pool *pgxpool.Pool
}

// NewAccountPostgres creates a new PostgreSQL account repository
func NewAccountPostgres(pool *pgxpool.Pool) *AccountPostgres {
	return &AccountPostgres{pool: pool}
}

// GetAccessToken retrieves the access token for an account
// Uses existing instagram_access_tokens table from Laravel
func (r *AccountPostgres) GetAccessToken(ctx context.Context, accountID string) (string, error) {
	query := `
		SELECT iat.access_token
		FROM instagram_access_tokens iat
		WHERE iat.instagram_account_id = $1
		ORDER BY iat.updated_at DESC
		LIMIT 1
	`

	var token string
	err := r.pool.QueryRow(ctx, query, accountID).Scan(&token)
	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("no access token found for account %s", accountID)
	}
	if err != nil {
		return "", fmt.Errorf("querying access token: %w", err)
	}

	return token, nil
}

// GetInstagramUserID retrieves the Instagram user ID for an account
func (r *AccountPostgres) GetInstagramUserID(ctx context.Context, accountID string) (string, error) {
	query := `
		SELECT instagram_user_id
		FROM instagram_accounts
		WHERE id = $1 AND deleted_at IS NULL
	`

	var userID string
	err := r.pool.QueryRow(ctx, query, accountID).Scan(&userID)
	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("account %s not found", accountID)
	}
	if err != nil {
		return "", fmt.Errorf("querying instagram user id: %w", err)
	}

	return userID, nil
}

// GetUsername retrieves the Instagram username for an account
func (r *AccountPostgres) GetUsername(ctx context.Context, accountID string) (string, error) {
	query := `
		SELECT username
		FROM instagram_accounts
		WHERE id = $1 AND deleted_at IS NULL
	`

	var username string
	err := r.pool.QueryRow(ctx, query, accountID).Scan(&username)
	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("account %s not found", accountID)
	}
	if err != nil {
		return "", fmt.Errorf("querying username: %w", err)
	}

	return username, nil
}

// GetAccountByInstagramID retrieves account info by Instagram ID
func (r *AccountPostgres) GetAccountByInstagramID(ctx context.Context, instagramID string) (*AccountInfo, error) {
	query := `
		SELECT ia.id, ia.instagram_user_id, ia.username, iat.access_token
		FROM instagram_accounts ia
		LEFT JOIN instagram_access_tokens iat ON ia.id = iat.instagram_account_id
		WHERE ia.instagram_id = $1 AND ia.deleted_at IS NULL
		ORDER BY iat.updated_at DESC
		LIMIT 1
	`

	var info AccountInfo
	err := r.pool.QueryRow(ctx, query, instagramID).Scan(
		&info.ID,
		&info.InstagramUserID,
		&info.Username,
		&info.AccessToken,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying account: %w", err)
	}

	return &info, nil
}

// AccountInfo represents Instagram account information
type AccountInfo struct {
	ID              string
	InstagramUserID string
	Username        string
	AccessToken     string
}

// ListAccounts returns all active Instagram accounts
func (r *AccountPostgres) ListAccounts(ctx context.Context) ([]AccountInfo, error) {
	query := `
		SELECT DISTINCT ON (ia.id)
			ia.id, ia.instagram_user_id, ia.username, iat.access_token
		FROM instagram_accounts ia
		LEFT JOIN instagram_access_tokens iat ON ia.id = iat.instagram_account_id
		WHERE ia.deleted_at IS NULL
		ORDER BY ia.id, iat.updated_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying accounts: %w", err)
	}
	defer rows.Close()

	var accounts []AccountInfo
	for rows.Next() {
		var info AccountInfo
		var token *string
		err := rows.Scan(&info.ID, &info.InstagramUserID, &info.Username, &token)
		if err != nil {
			return nil, fmt.Errorf("scanning account: %w", err)
		}
		if token != nil {
			info.AccessToken = *token
		}
		accounts = append(accounts, info)
	}

	return accounts, nil
}
