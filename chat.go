package srco

import (
	"encoding/json"
	"log"
	"net/http"
)

func getUserStatsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	var stats struct {
		PostCount     int `json:"post_count"`
		LikesReceived int `json:"likes_received"`
	}

	// Get post count
	err := db.QueryRow(`
		SELECT COUNT(*) 
		FROM posts 
		WHERE user_id = ?`, userID).Scan(&stats.PostCount)
	if err != nil {
		log.Printf("Error counting posts: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Get total likes received
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM likes_dislikes 
		WHERE post_id IN (SELECT id FROM posts WHERE user_id = ?) 
		AND is_like = 1`, userID).Scan(&stats.LikesReceived)
	if err != nil {
		log.Printf("Error counting likes: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// Update broadcastOnlineUsers function
func broadcastOnlineUsers() {
	onlineUsers.RLock()
	defer onlineUsers.RUnlock()

	// Get all users from database
	rows, err := db.Query("SELECT id, nickname FROM users ORDER BY nickname")
	if err != nil {
		log.Printf("Error fetching users: %v", err)
		return
	}
	defer rows.Close()

	var allUsers []map[string]interface{}
	for rows.Next() {
		var id int
		var nickname string
		if err := rows.Scan(&id, &nickname); err != nil {
			continue
		}

		// Check if user is online
		_, isOnline := onlineUsers.users[id]

		allUsers = append(allUsers, map[string]interface{}{
			"id":       id,
			"nickname": nickname,
			"status":   isOnline,
		})
	}

	message := map[string]interface{}{
		"type":  "userList",
		"users": allUsers,
	}

	// Send updated user list to all connected users
	for _, conn := range onlineUsers.users {
		if err := conn.WriteJSON(message); err != nil {
			log.Printf("Error sending user list: %v", err)
		}
	}
}

// Update getChatHistoryHandler to support pagination
func getChatHistoryHandler(w http.ResponseWriter, r *http.Request) {
	userID1 := r.URL.Query().Get("user1")
	userID2 := r.URL.Query().Get("user2")
	offset := r.URL.Query().Get("offset")
	limit := r.URL.Query().Get("limit")

	// Set default values if not provided
	if offset == "" {
		offset = "0"
	}
	if limit == "" {
		limit = "10"
	}

	// Get chat messages with pagination
	rows, err := db.Query(`
		SELECT cm.from_id, cm.to_id, cm.content, cm.created_at, u.nickname as from_nick
		FROM chat_messages cm
		JOIN users u ON cm.from_id = u.id
		WHERE (cm.from_id = ? AND cm.to_id = ?) OR (cm.from_id = ? AND cm.to_id = ?)
		ORDER BY cm.created_at DESC
		LIMIT ? OFFSET ?`,
		userID1, userID2, userID2, userID1, limit, offset)

	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var messages []ChatMessage
	for rows.Next() {
		var msg ChatMessage
		err := rows.Scan(&msg.FromID, &msg.ToID, &msg.Content, &msg.Timestamp, &msg.FromNick)
		if err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	// Get total count of messages
	var totalCount int
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM chat_messages 
		WHERE (from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?)`,
		userID1, userID2, userID2, userID1).Scan(&totalCount)

	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"messages": messages,
		"total":    totalCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
