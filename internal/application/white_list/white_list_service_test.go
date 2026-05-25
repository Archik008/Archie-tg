package whitelist

import (
	"errors"
	"reflect"
	"testing"

	"github.com/archik008/archie-tg/internal/application/dto"
	"github.com/archik008/archie-tg/internal/domain/entity/account"
	whitelistrepo "github.com/archik008/archie-tg/internal/domain/ports/out/repository/whitelist"
)

func TestAddToWhiteList_MapsDTOAndStoresAccount(t *testing.T) {
	repo := &fakeWhiteListServiceRepository{}
	service := WhiteListService{
		whiteListRepo: repo,
	}

	err := service.AddToWhiteList(dto.AccountDTO{
		UserID:   15,
		Username: "trusted-user",
	})
	if err != nil {
		t.Fatalf("AddToWhiteList() error = %v", err)
	}

	want := []account.Account{
		account.NewAccount(15, "trusted-user"),
	}
	if !reflect.DeepEqual(repo.addCalls, want) {
		t.Fatalf("Add() calls = %+v, want %+v", repo.addCalls, want)
	}
}

func TestAddToWhiteList_PropagatesRepositoryError(t *testing.T) {
	expectedErr := errors.New("repository unavailable")
	service := WhiteListService{
		whiteListRepo: &fakeWhiteListServiceRepository{
			addErr: expectedErr,
		},
	}

	err := service.AddToWhiteList(dto.AccountDTO{
		UserID:   15,
		Username: "trusted-user",
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("AddToWhiteList() error = %v, want %v", err, expectedErr)
	}
}

type fakeWhiteListServiceRepository struct {
	addErr   error
	addCalls []account.Account
}

func (f *fakeWhiteListServiceRepository) Get(userID int) (account.Account, error) {
	return account.Account{}, whitelistrepo.ErrAccountNotFound
}

func (f *fakeWhiteListServiceRepository) Add(a account.Account) error {
	f.addCalls = append(f.addCalls, a)
	return f.addErr
}

func (f *fakeWhiteListServiceRepository) Delete(a account.Account) error {
	return nil
}
