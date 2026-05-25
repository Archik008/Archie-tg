package whitelist

import (
	"context"
	"database/sql"
	"errors"

	"github.com/archik008/archie-tg/internal/domain/entity/account"
	repo "github.com/archik008/archie-tg/internal/domain/ports/out/repository/whitelist"
	_ "modernc.org/sqlite"
)

const createWhiteListTableQuery = `
CREATE TABLE IF NOT EXISTS whitelist_users (
	user_id INTEGER PRIMARY KEY,
	username TEXT NOT NULL
);
`

type InSqliteUserWhiteListRepository struct {
	dsn string
	db  *sql.DB
}

func NewInSqliteUserWhiteListRepository(dsn string) *InSqliteUserWhiteListRepository {
	return &InSqliteUserWhiteListRepository{dsn: dsn}
}

func (i *InSqliteUserWhiteListRepository) Connect() error {
	db, err := sql.Open("sqlite", i.dsn)
	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return err
	}

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, createWhiteListTableQuery); err != nil {
		_ = db.Close()
		return err
	}

	i.db = db

	return nil
}

func (i *InSqliteUserWhiteListRepository) Close() error {
	if i.db == nil {
		return nil
	}

	return i.db.Close()
}

func (i *InSqliteUserWhiteListRepository) Add(a account.Account) error {
	ctx := context.Background()

	var exists int
	if err := i.db.QueryRowContext(ctx, "SELECT 1 FROM whitelist_users WHERE user_id = ?", a.UserId).Scan(&exists); err == nil {
		return repo.ErrAccountAlreadyExists
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	_, err := i.db.ExecContext(
		ctx,
		"INSERT INTO whitelist_users(user_id, username) VALUES (?, ?)",
		a.UserId,
		a.Username,
	)
	return err
}

func (i *InSqliteUserWhiteListRepository) Delete(a account.Account) error {
	ctx := context.Background()

	result, err := i.db.ExecContext(ctx, "DELETE FROM whitelist_users WHERE user_id = ?", a.UserId)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return repo.ErrAccountNotFound
	}

	return nil
}

func (i *InSqliteUserWhiteListRepository) Get(userID int) (account.Account, error) {
	ctx := context.Background()

	var username string
	err := i.db.QueryRowContext(
		ctx,
		"SELECT username FROM whitelist_users WHERE user_id = ?",
		userID,
	).Scan(&username)
	if errors.Is(err, sql.ErrNoRows) {
		return account.Account{}, repo.ErrAccountNotFound
	}
	if err != nil {
		return account.Account{}, err
	}

	return account.NewAccount(userID, username), nil
}

func (i *InSqliteUserWhiteListRepository) GetAll() ([]account.Account, error) {
	ctx := context.Background()

	rows, err := i.db.QueryContext(
		ctx,
		"SELECT user_id, username FROM whitelist_users ORDER BY user_id ASC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := make([]account.Account, 0)
	for rows.Next() {
		var userID int
		var username string
		if err := rows.Scan(&userID, &username); err != nil {
			return nil, err
		}

		accounts = append(accounts, account.NewAccount(userID, username))
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return accounts, nil
}
