package srco

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
)

func createPostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var post Post
	if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	_, err := db.Exec("INSERT INTO posts (user_id, title, content, category) VALUES (?, ?, ?, ?)",
		post.UserID, post.Title, post.Content, post.Category)
	if err != nil {
		http.Error(w, "Error creating post", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func getPostsHandler(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

	query := `
        SELECT 
            p.id, 
            p.user_id, 
            p.title, 
            p.content, 
            p.category, 
            p.created_at,
            COALESCE(l.likes, 0) as likes,
            COALESCE(l.dislikes, 0) as dislikes,
            u.nickname as author_nickname
        FROM posts p
        LEFT JOIN users u ON p.user_id = u.id
        LEFT JOIN (
            SELECT 
                post_id,
                SUM(CASE WHEN is_like = 1 THEN 1 ELSE 0 END) as likes,
                SUM(CASE WHEN is_like = 0 THEN 1 ELSE 0 END) as dislikes
            FROM likes_dislikes
            GROUP BY post_id
        ) l ON p.id = l.post_id
    `

	var rows *sql.Rows
	var err error

	if category != "all" && category != "" {
		query += " WHERE p.category = ? ORDER BY p.created_at DESC"
		rows, err = db.Query(query, category)
	} else {
		query += " ORDER BY p.created_at DESC"
		rows, err = db.Query(query)
	}

	if err != nil {
		log.Printf("Database error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	defer rows.Close()

	posts := []PostWithAuthor{}
	for rows.Next() {
		var post PostWithAuthor
		if err := rows.Scan(
			&post.ID,
			&post.UserID,
			&post.Title,
			&post.Content,
			&post.Category,
			&post.CreatedAt,
			&post.Likes,
			&post.Dislikes,
			&post.AuthorNickname); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		posts = append(posts, post)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Rows error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(posts); err != nil {
		log.Printf("JSON encoding error: %v", err)
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
}

func getPostHandler(w http.ResponseWriter, r *http.Request) {
	postID := r.URL.Query().Get("id")
	userID := r.URL.Query().Get("user_id")

	var post PostWithAuthor
	err := db.QueryRow(`
        SELECT 
            p.id, 
            p.user_id, 
            p.title, 
            p.content, 
            p.category, 
            p.created_at,
            COALESCE(l.likes, 0) as likes,
            COALESCE(l.dislikes, 0) as dislikes,
            u.nickname as author_nickname
        FROM posts p
        LEFT JOIN users u ON p.user_id = u.id
        LEFT JOIN (
            SELECT 
                post_id,
                SUM(CASE WHEN is_like = 1 THEN 1 ELSE 0 END) as likes,
                SUM(CASE WHEN is_like = 0 THEN 1 ELSE 0 END) as dislikes
            FROM likes_dislikes
            GROUP BY post_id
        ) l ON p.id = l.post_id
        WHERE p.id = ?`, postID).Scan(
		&post.ID,
		&post.UserID,
		&post.Title,
		&post.Content,
		&post.Category,
		&post.CreatedAt,
		&post.Likes,
		&post.Dislikes,
		&post.AuthorNickname)

	if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	// Fetch user's reaction if logged in
	if userID != "" {
		var isLike sql.NullBool
		err = db.QueryRow(`
            SELECT is_like 
            FROM likes_dislikes 
            WHERE post_id = ? AND user_id = ?`, postID, userID).Scan(&isLike)
		if err == nil && isLike.Valid {
			if isLike.Bool {
				post.UserReaction = "like"
			} else {
				post.UserReaction = "dislike"
			}
		}
	}

	// Add comments query
	rows, err := db.Query(`
        SELECT c.id, c.content, c.created_at, u.nickname
        FROM comments c
        JOIN users u ON c.user_id = u.id
        WHERE c.post_id = ?
        ORDER BY c.created_at DESC`, postID)
	if err != nil {
		log.Printf("Error fetching comments: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var comment Comment
			if err := rows.Scan(&comment.ID, &comment.Content, &comment.CreatedAt, &comment.Author); err != nil {
				log.Printf("Error scanning comment: %v", err)
				continue
			}
			post.Comments = append(post.Comments, comment)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(post); err != nil {
		log.Printf("JSON encoding error: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

func likeDislikeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var reaction struct {
		UserID int  `json:"user_id"`
		PostID int  `json:"post_id"`
		IsLike bool `json:"is_like"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reaction); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	// Check if the user has already reacted
	var existingReactionID int
	err := db.QueryRow(`
		SELECT id FROM likes_dislikes 
		WHERE user_id = ? AND post_id = ?`,
		reaction.UserID, reaction.PostID).Scan(&existingReactionID)

	if err == sql.ErrNoRows {
		// Insert new reaction
		_, err = db.Exec(`
			INSERT INTO likes_dislikes (user_id, post_id, is_like) 
			VALUES (?, ?, ?)`,
			reaction.UserID, reaction.PostID, reaction.IsLike)
		if err != nil {
			http.Error(w, "Error processing like/dislike", http.StatusInternalServerError)
			return
		}
	} else if err == nil {
		// Update existing reaction
		_, err = db.Exec(`
			UPDATE likes_dislikes 
			SET is_like = ? 
			WHERE id = ?`,
			reaction.IsLike, existingReactionID)
		if err != nil {
			http.Error(w, "Error updating like/dislike", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func addCommentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var comment struct {
		PostID  int    `json:"post_id"`
		UserID  int    `json:"user_id"`
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	_, err := db.Exec(`
        INSERT INTO comments (post_id, user_id, content) 
        VALUES (?, ?, ?)`,
		comment.PostID, comment.UserID, comment.Content)
	if err != nil {
		http.Error(w, "Error adding comment", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func deletePostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		PostID int `json:"post_id"`
		UserID int `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	_, err := db.Exec("DELETE FROM posts WHERE id = ? AND user_id = ?",
		request.PostID, request.UserID)
	if err != nil {
		http.Error(w, "Error deleting post", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func updatePostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var post struct {
		ID       int    `json:"id"`
		UserID   int    `json:"user_id"`
		Title    string `json:"title"`
		Content  string `json:"content"`
		Category string `json:"category"`
	}

	if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	// Verify that the user owns this post
	var postUserID int
	err := db.QueryRow("SELECT user_id FROM posts WHERE id = ?", post.ID).Scan(&postUserID)
	if err != nil {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	if postUserID != post.UserID {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	_, err = db.Exec(`
		UPDATE posts 
		SET title = ?, content = ?, category = ? 
		WHERE id = ? AND user_id = ?`,
		post.Title, post.Content, post.Category, post.ID, post.UserID)

	if err != nil {
		http.Error(w, "Error updating post", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func deleteCommentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		CommentID int `json:"comment_id"`
		UserID    int `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	// Verify the user owns this comment
	var commentUserID int
	err := db.QueryRow("SELECT user_id FROM comments WHERE id = ?", request.CommentID).Scan(&commentUserID)
	if err != nil {
		http.Error(w, "Comment not found", http.StatusNotFound)
		return
	}

	if commentUserID != request.UserID {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	_, err = db.Exec("DELETE FROM comments WHERE id = ?", request.CommentID)
	if err != nil {
		http.Error(w, "Error deleting comment", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func updateCommentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		CommentID int    `json:"comment_id"`
		UserID    int    `json:"user_id"`
		Content   string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	// Verify the user owns this comment
	var commentUserID int
	err := db.QueryRow("SELECT user_id FROM comments WHERE id = ?", request.CommentID).Scan(&commentUserID)
	if err != nil {
		http.Error(w, "Comment not found", http.StatusNotFound)
		return
	}

	if commentUserID != request.UserID {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	_, err = db.Exec("UPDATE comments SET content = ? WHERE id = ?", request.Content, request.CommentID)
	if err != nil {
		http.Error(w, "Error updating comment", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
