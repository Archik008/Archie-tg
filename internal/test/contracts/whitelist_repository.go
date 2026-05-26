package contracts

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/archik008/archie-tg/internal/domain/entity/account"
	whitelistrepo "github.com/archik008/archie-tg/internal/domain/ports/out/repository/whitelist"
)

type WhiteListRepositoryFactory func(t *testing.T) whitelistrepo.UserWhiteListRepositoryPort

func RunWhiteListRepositoryContractTests(t *testing.T, newRepo WhiteListRepositoryFactory) {
	t.Helper()

	t.Run("Add stores account and Get returns it", func(t *testing.T) {
		repo := newRepo(t)
		ctx := context.Background()
		want := account.NewAccount(201, "trusted")

		if err := repo.Add(ctx, want); err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		got, err := repo.Get(ctx, want.UserId)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Get() account mismatch: got %+v want %+v", got, want)
		}
	})

	t.Run("Add returns ErrAccountAlreadyExists for duplicate users", func(t *testing.T) {
		repo := newRepo(t)
		ctx := context.Background()
		existing := account.NewAccount(202, "trusted")

		if err := repo.Add(ctx, existing); err != nil {
			t.Fatalf("Add() first call error = %v", err)
		}

		err := repo.Add(ctx, existing)
		if !errors.Is(err, whitelistrepo.ErrAccountAlreadyExists) {
			t.Fatalf("Add() duplicate error = %v, want %v", err, whitelistrepo.ErrAccountAlreadyExists)
		}
	})

	t.Run("Get returns ErrAccountNotFound for missing user", func(t *testing.T) {
		repo := newRepo(t)
		ctx := context.Background()

		_, err := repo.Get(ctx, 999)
		if !errors.Is(err, whitelistrepo.ErrAccountNotFound) {
			t.Fatalf("Get() missing error = %v, want %v", err, whitelistrepo.ErrAccountNotFound)
		}
	})

	t.Run("GetAll returns all stored accounts", func(t *testing.T) {
		repo := newRepo(t)
		ctx := context.Background()
		first := account.NewAccount(211, "trusted-one")
		second := account.NewAccount(212, "trusted-two")

		if err := repo.Add(ctx, first); err != nil {
			t.Fatalf("Add() first account error = %v", err)
		}
		if err := repo.Add(ctx, second); err != nil {
			t.Fatalf("Add() second account error = %v", err)
		}

		got, err := repo.GetAll(ctx)
		if err != nil {
			t.Fatalf("GetAll() error = %v", err)
		}

		sortAccounts(got)
		want := []account.Account{first, second}
		sortAccounts(want)

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("GetAll() accounts mismatch: got %+v want %+v", got, want)
		}
	})

	t.Run("Delete removes existing account", func(t *testing.T) {
		repo := newRepo(t)
		ctx := context.Background()
		existing := account.NewAccount(203, "trusted")

		if err := repo.Add(ctx, existing); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if err := repo.Delete(ctx, existing); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		_, err := repo.Get(ctx, existing.UserId)
		if !errors.Is(err, whitelistrepo.ErrAccountNotFound) {
			t.Fatalf("Get() after delete error = %v, want %v", err, whitelistrepo.ErrAccountNotFound)
		}
	})

	t.Run("Delete returns ErrAccountNotFound for missing user", func(t *testing.T) {
		repo := newRepo(t)
		ctx := context.Background()

		err := repo.Delete(ctx, account.NewAccount(204, "missing"))
		if !errors.Is(err, whitelistrepo.ErrAccountNotFound) {
			t.Fatalf("Delete() missing error = %v, want %v", err, whitelistrepo.ErrAccountNotFound)
		}
	})
}

func sortAccounts(accs []account.Account) {
	slices.SortFunc(accs, func(a, b account.Account) int {
		switch {
		case a.UserId < b.UserId:
			return -1
		case a.UserId > b.UserId:
			return 1
		default:
			return 0
		}
	})
}
