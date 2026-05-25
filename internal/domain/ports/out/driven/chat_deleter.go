package driven

type ChatDeleter interface {
	DeleteChat(chatId int) error
}
