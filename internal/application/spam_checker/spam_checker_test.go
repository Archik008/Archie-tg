package spamchecker

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/archik008/archie-tg/internal/application/dto"
	chatagg "github.com/archik008/archie-tg/internal/domain/aggregate/chat"
	"github.com/archik008/archie-tg/internal/domain/entity/account"
	chatrepo "github.com/archik008/archie-tg/internal/domain/ports/out/repository/chat"
	whitelistrepo "github.com/archik008/archie-tg/internal/domain/ports/out/repository/whitelist"
)

func TestProcessUser_SkipsWhitelistedUsers(t *testing.T) {
	chatRepo := &fakeChatRepository{}
	whiteListRepo := &fakeWhiteListRepository{
		getAccount: account.NewAccount(42, "trusted-user"),
	}
	chatDeleter := &fakeChatDeleter{}
	userBlocker := &fakeUserBlocker{}

	service := NewSpamCheckerService(
		chatRepo,
		whiteListRepo,
		chatDeleter,
		userBlocker,
		account.NewAccount(1, "base"),
	)

	err := service.ProcessUser(context.Background(), dto.AccountDTO{
		UserID:   42,
		Username: "trusted-user",
	})
	if err != nil {
		t.Fatalf("ProcessUser() error = %v", err)
	}
	if len(whiteListRepo.getCalls) != 1 || whiteListRepo.getCalls[0] != 42 {
		t.Fatalf("white list Get() calls = %v, want [42]", whiteListRepo.getCalls)
	}
	if len(chatRepo.getCalls) != 0 {
		t.Fatalf("chat repo Get() should not be called, got %v", chatRepo.getCalls)
	}
	if len(chatRepo.createCalls) != 0 {
		t.Fatalf("chat repo Create() should not be called, got %d calls", len(chatRepo.createCalls))
	}
	if len(chatRepo.deleteCalls) != 0 {
		t.Fatalf("chat repo Delete() should not be called, got %d calls", len(chatRepo.deleteCalls))
	}
	if len(chatDeleter.deleteCalls) != 0 {
		t.Fatalf("DeleteChat() should not be called, got %v", chatDeleter.deleteCalls)
	}
	if len(userBlocker.blockCalls) != 0 {
		t.Fatalf("BlockUser() should not be called, got %v", userBlocker.blockCalls)
	}
}

func TestProcessUser_CreatesChatForNonWhitelistedUserWhenChatMissing(t *testing.T) {
	chatRepo := &fakeChatRepository{
		getErr: chatrepo.ErrChatNotFound,
	}
	whiteListRepo := &fakeWhiteListRepository{
		getErr: whitelistrepo.ErrAccountNotFound,
	}
	chatDeleter := &fakeChatDeleter{}
	userBlocker := &fakeUserBlocker{}
	baseAccount := account.NewAccount(1, "base")

	service := NewSpamCheckerService(
		chatRepo,
		whiteListRepo,
		chatDeleter,
		userBlocker,
		baseAccount,
	)

	err := service.ProcessUser(context.Background(), dto.AccountDTO{
		UserID:   77,
		Username: "spam-user",
	})
	if err != nil {
		t.Fatalf("ProcessUser() error = %v", err)
	}
	if len(chatRepo.createCalls) != 1 {
		t.Fatalf("Create() calls = %d, want 1", len(chatRepo.createCalls))
	}

	wantChat := mustNewServiceChat(t, 77, baseAccount, account.NewAccount(77, "spam-user"))
	if !reflect.DeepEqual(chatRepo.createCalls[0], wantChat) {
		t.Fatalf("Create() chat mismatch: got %+v want %+v", chatRepo.createCalls[0], wantChat)
	}
	if len(chatDeleter.deleteCalls) != 0 {
		t.Fatalf("DeleteChat() should not be called, got %v", chatDeleter.deleteCalls)
	}
	if len(chatRepo.deleteCalls) != 0 {
		t.Fatalf("Delete() should not be called, got %d calls", len(chatRepo.deleteCalls))
	}
}

func TestProcessUser_DeletesChatBeforeBlockingKnownSpamUser(t *testing.T) {
	callLog := []string{}
	existingChat := mustNewServiceChat(
		t,
		88,
		account.NewAccount(1, "base"),
		account.NewAccount(88, "spam-user"),
	)
	chatRepo := &fakeChatRepository{
		getChat: existingChat,
		callLog: &callLog,
	}
	whiteListRepo := &fakeWhiteListRepository{
		getErr:  whitelistrepo.ErrAccountNotFound,
		callLog: &callLog,
	}
	chatDeleter := &fakeChatDeleter{callLog: &callLog}
	userBlocker := &fakeUserBlocker{callLog: &callLog}

	service := NewSpamCheckerService(
		chatRepo,
		whiteListRepo,
		chatDeleter,
		userBlocker,
		account.NewAccount(1, "base"),
	)

	err := service.ProcessUser(context.Background(), dto.AccountDTO{
		UserID:   88,
		Username: "spam-user",
	})
	if err != nil {
		t.Fatalf("ProcessUser() error = %v", err)
	}
	if !reflect.DeepEqual(callLog, []string{
		"whitelist.get",
		"chat.get",
		"chatDeleter.delete",
		"chat.delete",
		"userBlocker.block",
	}) {
		t.Fatalf("call order = %v", callLog)
	}
	if len(chatRepo.deleteCalls) != 1 || !reflect.DeepEqual(chatRepo.deleteCalls[0], existingChat) {
		t.Fatalf("Delete() calls = %+v, want [%+v]", chatRepo.deleteCalls, existingChat)
	}
	if len(chatDeleter.deleteCalls) != 1 || chatDeleter.deleteCalls[0] != existingChat.ID {
		t.Fatalf("DeleteChat() calls = %v, want [%d]", chatDeleter.deleteCalls, existingChat.ID)
	}
	if len(userBlocker.blockCalls) != 1 || userBlocker.blockCalls[0] != 88 {
		t.Fatalf("BlockUser() calls = %v, want [88]", userBlocker.blockCalls)
	}
}

func TestProcessUser_PropagatesDependencyErrors(t *testing.T) {
	baseAccount := account.NewAccount(1, "base")

	t.Run("white list failure", func(t *testing.T) {
		expectedErr := errors.New("white list unavailable")
		service := NewSpamCheckerService(
			&fakeChatRepository{},
			&fakeWhiteListRepository{getErr: expectedErr},
			&fakeChatDeleter{},
			&fakeUserBlocker{},
			baseAccount,
		)

		err := service.ProcessUser(context.Background(), dto.AccountDTO{UserID: 10, Username: "spam-user"})
		if !errors.Is(err, expectedErr) {
			t.Fatalf("ProcessUser() error = %v, want %v", err, expectedErr)
		}
	})

	t.Run("chat repository get failure", func(t *testing.T) {
		expectedErr := errors.New("chat repo get failed")
		service := NewSpamCheckerService(
			&fakeChatRepository{getErr: expectedErr},
			&fakeWhiteListRepository{getErr: whitelistrepo.ErrAccountNotFound},
			&fakeChatDeleter{},
			&fakeUserBlocker{},
			baseAccount,
		)

		err := service.ProcessUser(context.Background(), dto.AccountDTO{UserID: 10, Username: "spam-user"})
		if !errors.Is(err, expectedErr) {
			t.Fatalf("ProcessUser() error = %v, want %v", err, expectedErr)
		}
	})

	t.Run("chat repository create failure", func(t *testing.T) {
		expectedErr := errors.New("chat repo create failed")
		service := NewSpamCheckerService(
			&fakeChatRepository{
				getErr:    chatrepo.ErrChatNotFound,
				createErr: expectedErr,
			},
			&fakeWhiteListRepository{getErr: whitelistrepo.ErrAccountNotFound},
			&fakeChatDeleter{},
			&fakeUserBlocker{},
			baseAccount,
		)

		err := service.ProcessUser(context.Background(), dto.AccountDTO{UserID: 10, Username: "spam-user"})
		if !errors.Is(err, expectedErr) {
			t.Fatalf("ProcessUser() error = %v, want %v", err, expectedErr)
		}
	})

	t.Run("chat deleter failure", func(t *testing.T) {
		expectedErr := errors.New("delete chat failed")
		chatRepo := &fakeChatRepository{
			getChat: mustNewServiceChat(
				t,
				10,
				baseAccount,
				account.NewAccount(10, "spam-user"),
			),
		}
		userBlocker := &fakeUserBlocker{}
		service := NewSpamCheckerService(
			chatRepo,
			&fakeWhiteListRepository{getErr: whitelistrepo.ErrAccountNotFound},
			&fakeChatDeleter{deleteErr: expectedErr},
			userBlocker,
			baseAccount,
		)

		err := service.ProcessUser(context.Background(), dto.AccountDTO{UserID: 10, Username: "spam-user"})
		if !errors.Is(err, expectedErr) {
			t.Fatalf("ProcessUser() error = %v, want %v", err, expectedErr)
		}
		if len(chatRepo.deleteCalls) != 0 {
			t.Fatalf("Delete() should not be called after DeleteChat() failure, got %d calls", len(chatRepo.deleteCalls))
		}
		if len(userBlocker.blockCalls) != 0 {
			t.Fatalf("BlockUser() should not be called after DeleteChat() failure, got %d calls", len(userBlocker.blockCalls))
		}
	})

	t.Run("chat repository delete failure", func(t *testing.T) {
		expectedErr := errors.New("chat repo delete failed")
		userBlocker := &fakeUserBlocker{}
		service := NewSpamCheckerService(
			&fakeChatRepository{
				getChat: mustNewServiceChat(
					t,
					10,
					baseAccount,
					account.NewAccount(10, "spam-user"),
				),
				deleteErr: expectedErr,
			},
			&fakeWhiteListRepository{getErr: whitelistrepo.ErrAccountNotFound},
			&fakeChatDeleter{},
			userBlocker,
			baseAccount,
		)

		err := service.ProcessUser(context.Background(), dto.AccountDTO{UserID: 10, Username: "spam-user"})
		if !errors.Is(err, expectedErr) {
			t.Fatalf("ProcessUser() error = %v, want %v", err, expectedErr)
		}
		if len(userBlocker.blockCalls) != 0 {
			t.Fatalf("BlockUser() should not be called after Delete() failure, got %d calls", len(userBlocker.blockCalls))
		}
	})

	t.Run("user blocker failure", func(t *testing.T) {
		expectedErr := errors.New("block user failed")
		service := NewSpamCheckerService(
			&fakeChatRepository{
				getChat: mustNewServiceChat(
					t,
					10,
					baseAccount,
					account.NewAccount(10, "spam-user"),
				),
			},
			&fakeWhiteListRepository{getErr: whitelistrepo.ErrAccountNotFound},
			&fakeChatDeleter{},
			&fakeUserBlocker{blockErr: expectedErr},
			baseAccount,
		)

		err := service.ProcessUser(context.Background(), dto.AccountDTO{UserID: 10, Username: "spam-user"})
		if !errors.Is(err, expectedErr) {
			t.Fatalf("ProcessUser() error = %v, want %v", err, expectedErr)
		}
	})
}

type fakeChatRepository struct {
	getChat     chatagg.Chat
	getErr      error
	createErr   error
	deleteErr   error
	getCalls    []int
	createCalls []chatagg.Chat
	deleteCalls []chatagg.Chat
	callLog     *[]string
}

func (f *fakeChatRepository) Create(_ context.Context, c chatagg.Chat) error {
	f.createCalls = append(f.createCalls, c)
	f.appendCall("chat.create")
	return f.createErr
}

func (f *fakeChatRepository) Delete(_ context.Context, c chatagg.Chat) error {
	f.deleteCalls = append(f.deleteCalls, c)
	f.appendCall("chat.delete")
	return f.deleteErr
}

func (f *fakeChatRepository) Get(_ context.Context, chatID int) (chatagg.Chat, error) {
	f.getCalls = append(f.getCalls, chatID)
	f.appendCall("chat.get")
	if f.getErr != nil {
		return chatagg.Chat{}, f.getErr
	}
	return f.getChat, nil
}

func (f *fakeChatRepository) appendCall(name string) {
	if f.callLog != nil {
		*f.callLog = append(*f.callLog, name)
	}
}

type fakeWhiteListRepository struct {
	getAccount account.Account
	getErr     error
	getCalls   []int
	callLog    *[]string
}

func (f *fakeWhiteListRepository) Get(_ context.Context, userID int) (account.Account, error) {
	f.getCalls = append(f.getCalls, userID)
	if f.callLog != nil {
		*f.callLog = append(*f.callLog, "whitelist.get")
	}
	if f.getErr != nil {
		return account.Account{}, f.getErr
	}
	return f.getAccount, nil
}

func (f *fakeWhiteListRepository) Add(_ context.Context, a account.Account) error {
	return nil
}

func (f *fakeWhiteListRepository) GetAll(_ context.Context) ([]account.Account, error) {
	return nil, nil
}

func (f *fakeWhiteListRepository) Delete(_ context.Context, a account.Account) error {
	return nil
}

type fakeChatDeleter struct {
	deleteErr   error
	deleteCalls []int
	callLog     *[]string
}

func (f *fakeChatDeleter) DeleteChat(_ context.Context, chatID int) error {
	f.deleteCalls = append(f.deleteCalls, chatID)
	if f.callLog != nil {
		*f.callLog = append(*f.callLog, "chatDeleter.delete")
	}
	return f.deleteErr
}

type fakeUserBlocker struct {
	blockErr   error
	blockCalls []int
	callLog    *[]string
}

func (f *fakeUserBlocker) BlockUser(_ context.Context, userID int) error {
	f.blockCalls = append(f.blockCalls, userID)
	if f.callLog != nil {
		*f.callLog = append(*f.callLog, "userBlocker.block")
	}
	return f.blockErr
}

func mustNewServiceChat(t *testing.T, id int, accounts ...account.Account) chatagg.Chat {
	t.Helper()

	userChat, err := chatagg.NewChat(accounts...)
	if err != nil {
		t.Fatalf("NewChat() error = %v", err)
	}

	return userChat.WithID(id)
}
