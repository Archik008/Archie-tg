package whitelist

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/archik008/archie-tg/internal/application/dto"
	"github.com/archik008/archie-tg/internal/domain/entity/account"
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

func TestGet_MapsRepositoryAccountToDTO(t *testing.T) {
	repo := &fakeWhiteListServiceRepository{
		getAccount: account.NewAccount(22, "trusted-user"),
	}
	service := WhiteListService{
		whiteListRepo: repo,
	}

	got, err := service.Get(22)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	want := dto.AccountDTO{
		UserID:   22,
		Username: "trusted-user",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Get() dto mismatch: got %+v want %+v", got, want)
	}
	if !reflect.DeepEqual(repo.getCalls, []int{22}) {
		t.Fatalf("repo Get() calls = %v, want [22]", repo.getCalls)
	}
}

func TestGet_PropagatesRepositoryError(t *testing.T) {
	expectedErr := errors.New("repository unavailable")
	service := WhiteListService{
		whiteListRepo: &fakeWhiteListServiceRepository{
			getErr: expectedErr,
		},
	}

	_, err := service.Get(22)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("Get() error = %v, want %v", err, expectedErr)
	}
}

func TestGetAll_MapsRepositoryAccountsToDTOs(t *testing.T) {
	repo := &fakeWhiteListServiceRepository{
		getAllAccounts: []account.Account{
			account.NewAccount(31, "trusted-one"),
			account.NewAccount(32, "trusted-two"),
		},
	}
	service := WhiteListService{
		whiteListRepo: repo,
	}

	got, err := service.GetAll()
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}

	want := []dto.AccountDTO{
		{UserID: 31, Username: "trusted-one"},
		{UserID: 32, Username: "trusted-two"},
	}
	sortAccountDTOs(got)
	sortAccountDTOs(want)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetAll() dto mismatch: got %+v want %+v", got, want)
	}
	if repo.getAllCalls != 1 {
		t.Fatalf("repo GetAll() calls = %d, want 1", repo.getAllCalls)
	}
}

func TestGetAll_PropagatesRepositoryError(t *testing.T) {
	expectedErr := errors.New("repository unavailable")
	service := WhiteListService{
		whiteListRepo: &fakeWhiteListServiceRepository{
			getAllErr: expectedErr,
		},
	}

	_, err := service.GetAll()
	if !errors.Is(err, expectedErr) {
		t.Fatalf("GetAll() error = %v, want %v", err, expectedErr)
	}
}

func TestDelete_MapsDTOAndDeletesAccount(t *testing.T) {
	repo := &fakeWhiteListServiceRepository{}
	service := WhiteListService{
		whiteListRepo: repo,
	}

	err := service.Delete(dto.AccountDTO{
		UserID:   41,
		Username: "trusted-user",
	})
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	want := []account.Account{
		account.NewAccount(41, "trusted-user"),
	}
	if !reflect.DeepEqual(repo.deleteCalls, want) {
		t.Fatalf("repo Delete() calls = %+v, want %+v", repo.deleteCalls, want)
	}
}

func TestDelete_PropagatesRepositoryError(t *testing.T) {
	expectedErr := errors.New("repository unavailable")
	service := WhiteListService{
		whiteListRepo: &fakeWhiteListServiceRepository{
			deleteErr: expectedErr,
		},
	}

	err := service.Delete(dto.AccountDTO{
		UserID:   41,
		Username: "trusted-user",
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("Delete() error = %v, want %v", err, expectedErr)
	}
}

type fakeWhiteListServiceRepository struct {
	addErr         error
	getErr         error
	getAllErr      error
	deleteErr      error
	getAccount     account.Account
	getAllAccounts []account.Account
	addCalls       []account.Account
	getCalls       []int
	getAllCalls    int
	deleteCalls    []account.Account
}

func (f *fakeWhiteListServiceRepository) Get(userID int) (account.Account, error) {
	f.getCalls = append(f.getCalls, userID)
	if f.getErr != nil {
		return account.Account{}, f.getErr
	}
	return f.getAccount, nil
}

func (f *fakeWhiteListServiceRepository) Add(a account.Account) error {
	f.addCalls = append(f.addCalls, a)
	return f.addErr
}

func (f *fakeWhiteListServiceRepository) GetAll() ([]account.Account, error) {
	f.getAllCalls++
	if f.getAllErr != nil {
		return nil, f.getAllErr
	}
	accs := make([]account.Account, len(f.getAllAccounts))
	copy(accs, f.getAllAccounts)
	return accs, nil
}

func (f *fakeWhiteListServiceRepository) Delete(a account.Account) error {
	f.deleteCalls = append(f.deleteCalls, a)
	return f.deleteErr
}

func sortAccountDTOs(accs []dto.AccountDTO) {
	slices.SortFunc(accs, func(a, b dto.AccountDTO) int {
		switch {
		case a.UserID < b.UserID:
			return -1
		case a.UserID > b.UserID:
			return 1
		default:
			return 0
		}
	})
}
