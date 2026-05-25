package chat

import (
	"context"
	"database/sql"
	"errors"

	chatagg "github.com/archik008/archie-tg/internal/domain/aggregate/chat"
	"github.com/archik008/archie-tg/internal/domain/entity/account"
	repo "github.com/archik008/archie-tg/internal/domain/ports/out/repository/chat"
	_ "modernc.org/sqlite"
)

type InSqliteChatRepository struct {
	dsn string
	db  *sql.DB
}

func NewInSqliteChatRepository(dsn string) *InSqliteChatRepository {
	return &InSqliteChatRepository{dsn: dsn}
}

func (i *InSqliteChatRepository) Connect() error {
	db, err := sql.Open("sqlite", i.dsn)
	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return err
	}

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return err
	}

	if _, err := db.ExecContext(ctx, createChatsTableQuery); err != nil {
		_ = db.Close()
		return err
	}
	if _, err := db.ExecContext(ctx, createChatMembersTableQuery); err != nil {
		_ = db.Close()
		return err
	}

	i.db = db

	return nil
}

func (i *InSqliteChatRepository) Close() error {
	if i.db == nil {
		return nil
	}

	return i.db.Close()
}

func (i *InSqliteChatRepository) Create(c chatagg.Chat) error {
	ctx := context.Background()

	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT 1 FROM chats WHERE id = ?", c.ID).Scan(&exists); err == nil {
		return repo.ErrChatExists
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if _, err := tx.ExecContext(ctx, "INSERT INTO chats(id) VALUES (?)", c.ID); err != nil {
		return err
	}

	for idx, member := range c.Accounts {
		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO chat_members(chat_id, member_order, user_id, username) VALUES (?, ?, ?, ?)",
			c.ID,
			idx,
			member.UserId,
			member.Username,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (i *InSqliteChatRepository) Get(chatID int) (chatagg.Chat, error) {
	ctx := context.Background()

	var id int
	if err := i.db.QueryRowContext(ctx, "SELECT id FROM chats WHERE id = ?", chatID).Scan(&id); errors.Is(err, sql.ErrNoRows) {
		return chatagg.Chat{}, repo.ErrChatNotFound
	} else if err != nil {
		return chatagg.Chat{}, err
	}

	rows, err := i.db.QueryContext(
		ctx,
		"SELECT user_id, username FROM chat_members WHERE chat_id = ? ORDER BY member_order ASC",
		chatID,
	)
	if err != nil {
		return chatagg.Chat{}, err
	}
	defer rows.Close()

	accounts := make([]account.Account, 0)
	for rows.Next() {
		var userID int
		var username string
		if err := rows.Scan(&userID, &username); err != nil {
			return chatagg.Chat{}, err
		}

		accounts = append(accounts, account.NewAccount(userID, username))
	}

	if err := rows.Err(); err != nil {
		return chatagg.Chat{}, err
	}

	userChat, err := chatagg.NewChat(accounts...)
	if err != nil {
		return chatagg.Chat{}, err
	}

	return userChat.WithID(chatID), nil
}

func (i *InSqliteChatRepository) Delete(c chatagg.Chat) error {
	ctx := context.Background()

	result, err := i.db.ExecContext(ctx, "DELETE FROM chats WHERE id = ?", c.ID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return repo.ErrChatNotFound
	}

	return nil
}
