package srco

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	// Convert both nickname and email to lowercase
	user.Nickname = strings.ToLower(user.Nickname)
	user.Email = strings.ToLower(user.Email)

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Error processing password", http.StatusInternalServerError)
		return
	}

	_, err = db.Exec("INSERT INTO users (nickname, age, gender, first_name, last_name, email, password) VALUES (?, ?, ?, ?, ?, ?, ?)",
		user.Nickname, user.Age, user.Gender, user.FirstName, user.LastName, user.Email, hashedPassword)
	if err != nil {
		http.Error(w, "Error registering user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var credentials struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	var user struct {
		ID             int    `json:"user_id"`
		Nickname       string `json:"nickname"`
		ProfileThought string `json:"profile_thought"`
		Password       string `json:"-"`
	}

	// Use LOWER() to make the query case-insensitive for both nickname and email
	query := `SELECT id, nickname, COALESCE(profile_thought, '') as profile_thought, password
              FROM users
              WHERE LOWER(nickname) = LOWER(?) OR LOWER(email) = LOWER(?)`
	err := db.QueryRow(query, credentials.Identifier, credentials.Identifier).Scan(
		&user.ID,
		&user.Nickname,
		&user.ProfileThought,
		&user.Password)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "User not found", http.StatusUnauthorized)
		} else {
			log.Printf("Database error: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(credentials.Password)); err != nil {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":         user.ID,
		"nickname":        user.Nickname,
		"profile_thought": user.ProfileThought,
		"status":          "success",
	})
}

func updateProfileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var profile struct {
		UserID         int    `json:"user_id"`
		ProfilePic     string `json:"profile_pic"`
		ProfileThought string `json:"profile_thought"`
	}

	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	// Validate profile thought length
	if len(profile.ProfileThought) > 30 {
		http.Error(w, "Profile thought must be 30 characters or less", http.StatusBadRequest)
		return
	}

	_, err := db.Exec(`
        UPDATE users 
        SET profile_pic = ?, profile_thought = ? 
        WHERE id = ?`,
		profile.ProfilePic, profile.ProfileThought, profile.UserID)
	if err != nil {
		http.Error(w, "Error updating profile", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func getUserProfileHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "User ID is required"})
		return
	}

	var profile UserProfile
	err := db.QueryRow(`
        SELECT id, nickname, age, gender, first_name, last_name, email,
               COALESCE(profile_pic, 'default-profile.jpg') as profile_pic,
               COALESCE(profile_thought, '') as profile_thought
        FROM users 
        WHERE id = ?`, userID).Scan(
		&profile.ID, &profile.Nickname, &profile.Age, &profile.Gender,
		&profile.FirstName, &profile.LastName, &profile.Email,
		&profile.ProfilePic, &profile.ProfileThought)

	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"error": "User not found",
			})
			return
		} else {
			log.Printf("Database error: %v", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Internal server error",
			})
			return
		}
	}

	// Fetch user's posts
	rows, err := db.Query(`
        SELECT id, title, content, category, created_at
        FROM posts
        WHERE user_id = ?
        ORDER BY created_at DESC`, userID)
	if err != nil {
		log.Printf("Error fetching user posts: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var post Post
			if err := rows.Scan(&post.ID, &post.Title, &post.Content,
				&post.Category, &post.CreatedAt); err != nil {
				log.Printf("Error scanning post: %v", err)
				continue
			}
			profile.UserPosts = append(profile.UserPosts, post)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}
