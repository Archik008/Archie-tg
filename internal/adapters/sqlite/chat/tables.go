package chat

const createChatsTableQuery = `
CREATE TABLE IF NOT EXISTS chats (
	id INTEGER PRIMARY KEY
);
`

const createChatMembersTableQuery = `
CREATE TABLE IF NOT EXISTS chat_members (
	chat_id INTEGER NOT NULL,
	member_order INTEGER NOT NULL,
	user_id INTEGER NOT NULL,
	username TEXT NOT NULL,
	PRIMARY KEY (chat_id, member_order),
	FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE
);
`
