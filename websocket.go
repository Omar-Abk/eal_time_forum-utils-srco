package srco

import (
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	// WebSocket upgrader
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// Map to store online users and their connections
	onlineUsers = struct {
		sync.RWMutex
		users map[int]*websocket.Conn
	}{users: make(map[int]*websocket.Conn)}
)

// Add WebSocket handler
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		return
	}

	uid, err := strconv.Atoi(userID)
	if err != nil {
		return
	}

	// Add user to online users
	onlineUsers.Lock()
	if oldConn, exists := onlineUsers.users[uid]; exists {
		oldConn.Close() // Close old connection if exists
	}
	onlineUsers.users[uid] = conn
	onlineUsers.Unlock()

	// Broadcast updated user list
	broadcastOnlineUsers()

	// Keep connection alive and handle messages
	for {
		var msg struct {
			Type    string      `json:"type"`
			Content interface{} `json:"content"`
		}

		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			onlineUsers.Lock()
			delete(onlineUsers.users, uid)
			onlineUsers.Unlock()
			broadcastOnlineUsers()
			break
		}

		switch msg.Type {
		case "requestUserList":
			onlineUsers.RLock()
			if _, ok := onlineUsers.users[uid]; ok {
				broadcastOnlineUsers()
			}
			onlineUsers.RUnlock()
		case "chat_message":
			if chatMsg, ok := msg.Content.(map[string]interface{}); ok {
				toID := int(chatMsg["to"].(float64))
				content := chatMsg["content"].(string)

				// Get sender's nickname
				var fromNick string
				err := db.QueryRow("SELECT nickname FROM users WHERE id = ?", uid).Scan(&fromNick)
				if err != nil {
					continue
				}

				// Create message object
				message := ChatMessage{
					FromID:    uid,
					FromNick:  fromNick,
					ToID:      toID,
					Content:   content,
					Timestamp: time.Now().Format(time.RFC3339),
				}

				// Store message in database
				_, err = db.Exec(`
					INSERT INTO chat_messages (from_id, to_id, content, created_at)
					VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
					message.FromID, message.ToID, message.Content)
				if err != nil {
					log.Printf("Error storing chat message: %v", err)
					continue
				}

				// Send message to recipient if online
				onlineUsers.RLock()
				if recipientConn, ok := onlineUsers.users[toID]; ok {
					recipientConn.WriteJSON(map[string]interface{}{
						"type":    "chat_message",
						"message": message,
					})
				}
				onlineUsers.RUnlock()
			}
		case "typing_status":
			if typingStatus, ok := msg.Content.(map[string]interface{}); ok {
				toID := int(typingStatus["to"].(float64))
				isTyping := typingStatus["isTyping"].(bool)

				// Send typing status to recipient if online
				onlineUsers.RLock()
				if recipientConn, ok := onlineUsers.users[toID]; ok {
					recipientConn.WriteJSON(map[string]interface{}{
						"type":      "typing_status",
						"from_id":   uid,
						"is_typing": isTyping,
					})
				}
				onlineUsers.RUnlock()
			}
		}
	}
}
