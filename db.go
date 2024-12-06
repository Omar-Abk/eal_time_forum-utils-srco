package srco

import "log"

func createTables() {
	createUsersTable := `
    CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        nickname TEXT UNIQUE,
        age INTEGER,
        gender TEXT,
        first_name TEXT,
        last_name TEXT,
        email TEXT UNIQUE,
        password TEXT,
        profile_pic TEXT DEFAULT 'default-profile.jpg',
        profile_thought TEXT
    );`

	createPostsTable := `
    CREATE TABLE IF NOT EXISTS posts (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER,
        title TEXT,
        content TEXT,
        category TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY(user_id) REFERENCES users(id)
    );`

	createLikesDislikesTable := `
    CREATE TABLE IF NOT EXISTS likes_dislikes (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER,
        post_id INTEGER,
        is_like BOOLEAN,
        FOREIGN KEY(user_id) REFERENCES users(id),
        FOREIGN KEY(post_id) REFERENCES posts(id)
    );`

	createCommentsTable := `
    CREATE TABLE IF NOT EXISTS comments (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        post_id INTEGER,
        user_id INTEGER,
        content TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY(post_id) REFERENCES posts(id),
        FOREIGN KEY(user_id) REFERENCES users(id)
    );`

	createChatMessagesTable := `
    CREATE TABLE IF NOT EXISTS chat_messages (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        from_id INTEGER,
        to_id INTEGER,
        content TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY(from_id) REFERENCES users(id),
        FOREIGN KEY(to_id) REFERENCES users(id)
    );`

	tables := []string{createUsersTable, createPostsTable, createLikesDislikesTable, createCommentsTable, createChatMessagesTable}
	for _, query := range tables {
		if _, err := db.Exec(query); err != nil {
			log.Fatal("Could not create table:", err)
		}
	}
}
