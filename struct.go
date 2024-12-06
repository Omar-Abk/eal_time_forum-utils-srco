package srco

import "database/sql"

var db *sql.DB

type User struct {
	ID        int    `json:"id"`
	Nickname  string `json:"nickname"`
	Age       int    `json:"age"`
	Gender    string `json:"gender"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

type Post struct {
	ID           int    `json:"id"`
	UserID       int    `json:"user_id"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	Category     string `json:"category"`
	CreatedAt    string `json:"created_at"`
	Likes        int    `json:"likes"`
	Dislikes     int    `json:"dislikes"`
	UserReaction string `json:"user_reaction"`
}

type Comment struct {
	ID        int    `json:"id"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
	Author    string `json:"author"`
}

type PostWithAuthor struct {
	ID             int       `json:"id"`
	UserID         int       `json:"user_id"`
	Title          string    `json:"title"`
	Content        string    `json:"content"`
	Category       string    `json:"category"`
	CreatedAt      string    `json:"created_at"`
	Likes          int       `json:"likes"`
	Dislikes       int       `json:"dislikes"`
	AuthorNickname string    `json:"author_nickname"`
	UserReaction   string    `json:"user_reaction,omitempty"`
	Comments       []Comment `json:"comments"`
}

type UserProfile struct {
	ID             int    `json:"id"`
	Nickname       string `json:"nickname"`
	Age            int    `json:"age"`
	Gender         string `json:"gender"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Email          string `json:"email"`
	ProfilePic     string `json:"profile_pic"`
	ProfileThought string `json:"profile_thought"`
	UserPosts      []Post `json:"user_posts"`
}

type ChatMessage struct {
	FromID    int    `json:"from_id"`
	FromNick  string `json:"from_nick"`
	ToID      int    `json:"to_id"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}
