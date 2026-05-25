package account

type Account struct {
	UserId   int
	Username string
}

func NewAccount(userID int, username string) Account {
	return Account{
		UserId:   userID,
		Username: username,
	}
}
