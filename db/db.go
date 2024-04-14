package db

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/voltix-vault/voltix-router/contexthelper"
	"github.com/voltix-vault/voltix-router/model"
	"time"
)

type DBStorage struct {
	ConnectionString string
	db               *sql.DB
}

// NewDBStorage returns a new storage that use mysql
func NewDBStorage(connectionString string) (*DBStorage, error) {
	db, err := sql.Open("mysql", connectionString)
	if err != nil {
		return nil, fmt.Errorf("fail to connect to mysql, err: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("fail to ping mysql, err: %w", err)
	}

	return &DBStorage{
		ConnectionString: connectionString,
		db:               db,
	}, nil
}

// GetUser returns a user by its api key.
func (d *DBStorage) GetUser(ctx context.Context, apiKey string) (*model.User, error) {
	if contexthelper.CheckCancellation(ctx) != nil {
		return nil, ctx.Err()
	}

	var user model.User
	err := d.db.QueryRowContext(ctx, "SELECT id, api_key, created_at, expired_at, no_of_vaults, is_paid FROM users WHERE api_key = ?", apiKey).Scan(
		&user.ID,
		&user.APIKey,
		&user.CreatedAt,
		&user.ExpiredAt,
		&user.NoOfVaults,
		&user.IsPaid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("fail to query user, err: %w", err)
	}
	return &user, nil
}
func (d *DBStorage) NewUser(ctx context.Context, apiKey string) error {
	if contexthelper.CheckCancellation(ctx) != nil {
		return ctx.Err()
	}
	_, err := d.db.ExecContext(ctx, "INSERT INTO users (api_key) VALUES (?)", apiKey)
	if err != nil {
		return fmt.Errorf("fail to insert user, err: %w", err)
	}
	return nil
}
func (d *DBStorage) UpdateUser(ctx context.Context, apiKey string,
	expiryAt time.Time,
	noOfVault int,
	is_paid bool,
	amount float64,
	paymentTxID string) error {
	if contexthelper.CheckCancellation(ctx) != nil {
		return ctx.Err()
	}
	_, err := d.db.ExecContext(ctx, "UPDATE users SET expired_at = ?, no_of_vaults = ?, is_paid = ? WHERE api_key = ?", expiryAt, noOfVault, is_paid, apiKey)
	if err != nil {
		return fmt.Errorf("fail to update user, err: %w", err)
	}
	_, err = d.db.ExecContext(ctx, "insert into payments (user_id, tx_id, amount, created_at) values ((select id from users where api_key = ?), ?, ?, ?)", apiKey, paymentTxID, amount, time.Now())
	if err != nil {
		return fmt.Errorf("fail to insert payment history, err: %w", err)
	}
	return nil
}

func (d *DBStorage) RegisterVault(ctx context.Context, userID int64, pubKeyECDSA, pubKeyEdDSA string, totalAllowedCount int64) error {
	if contexthelper.CheckCancellation(ctx) != nil {
		return ctx.Err()
	}
	var count int64
	if err := d.db.QueryRowContext(ctx, "SELECT count(*) FROM Vaults WHERE user_id = ? and vault_pubkey_ecdsa=? and vault_pubkey_eddsa=?", userID, pubKeyECDSA, pubKeyEdDSA).Scan(&count); err != nil {
		return fmt.Errorf("fail to register vault, err: %w", err)
	}
	if count > 0 { // already registered
		return nil
	}
	if count == totalAllowedCount {
		return fmt.Errorf("vault limit reached")
	}
	_, err := d.db.ExecContext(ctx, "INSERT INTO vaults (user_id, vault_pubkey_ecdsa, vault_pubkey_eddsa) VALUES (?, ?, ?)", userID, pubKeyECDSA, pubKeyEdDSA)
	if err != nil {
		return fmt.Errorf("fail to insert vault, err: %w", err)
	}
	return nil
}

func (d *DBStorage) GetVaultPubKeys(ctx context.Context, userID int64) ([]string, error) {
	if contexthelper.CheckCancellation(ctx) != nil {
		return nil, ctx.Err()
	}
	rows, err := d.db.QueryContext(ctx, "SELECT vault_pubkey_ecdsa, vault_pubkey_eddsa FROM vaults WHERE user_id = ?", userID)
	if err != nil {
		return nil, fmt.Errorf("fail to query vaults, err: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println("fail to close rows", err)
		}
	}(rows)
	var pubKeys []string
	for rows.Next() {
		var pubKeyECDSA, pubKeyEdDSA string
		if err := rows.Scan(&pubKeyECDSA, &pubKeyEdDSA); err != nil {
			return nil, fmt.Errorf("fail to scan vaults, err: %w", err)
		}
		pubKeys = append(pubKeys, pubKeyECDSA, pubKeyEdDSA)
	}
	return pubKeys, nil
}

func (d *DBStorage) GetTotalVaults(ctx context.Context, userID int64) (int, error) {
	if contexthelper.CheckCancellation(ctx) != nil {
		return 0, ctx.Err()
	}
	var totalVaults int
	err := d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM vaults WHERE user_id = ?", userID).Scan(&totalVaults)
	if err != nil {
		return 0, fmt.Errorf("fail to query total vaults, err: %w", err)
	}
	return totalVaults, nil
}

// Close closes the connection to the database.
func (d *DBStorage) Close() error {
	return d.db.Close()
}
