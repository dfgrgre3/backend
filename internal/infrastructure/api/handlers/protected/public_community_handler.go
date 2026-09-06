package protected

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func GetPublicAnnouncements(c *gin.Context) {
	var notifications []models.Notification
	if err := db.DB.Order("created_at DESC").Limit(50).Find(&notifications).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch announcements")
		return
	}

	items := make([]gin.H, 0, len(notifications))
	for _, n := range notifications {
		items = append(items, gin.H{
			"id":          n.ID,
			"title":       n.Title,
			"content":     n.Message,
			"publishedAt": n.CreatedAt,
			"priority":    strings.ToLower(defaultString(n.Priority, "medium")),
			"category":    strings.ToLower(defaultString(n.Category, "general")),
			"authorName":  "System",
			"tags":        []string{},
			"views":       0,
		})
	}

	api_response.Success(c, items)
}

func CreatePublicAnnouncement(c *gin.Context) {
	userID := c.GetString("userId")
	var input struct {
		Title    string   `json:"title"`
		Content  string   `json:"content"`
		Priority string   `json:"priority"`
		Category string   `json:"category"`
		Tags     []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Content = strings.TrimSpace(input.Content)
	if input.Title == "" || input.Content == "" {
		api_response.Error(c, http.StatusBadRequest, "title and content are required")
		return
	}

	notification := models.Notification{
		UserID:   userID,
		Title:    input.Title,
		Message:  input.Content,
		Type:     models.NotificationInfo,
		Category: strings.ToUpper(defaultString(input.Category, "general")),
		Priority: strings.ToUpper(defaultString(input.Priority, "medium")),
		IsRead:   false,
	}
	if err := SafeCreate(db.DB, &notification); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create announcement")
		return
	}

	api_response.Success(c, gin.H{"id": notification.ID})
}

// chatConversationsSQL returns the latest message per counterpart plus the
// unread count, for either direction of the conversation.
const chatConversationsSQL = `
WITH mine AS (
	SELECT id,
	       CASE WHEN sender_id = ? THEN receiver_id ELSE sender_id END AS counterpart_id,
	       content, created_at
	FROM "DirectMessage"
	WHERE sender_id = ? OR receiver_id = ?
),
latest AS (
	SELECT DISTINCT ON (counterpart_id) counterpart_id, content, created_at
	FROM mine
	ORDER BY counterpart_id, created_at DESC
)
SELECT l.counterpart_id,
       l.content,
       l.created_at,
       COALESCE(u.cnt, 0) AS unread
FROM latest l
LEFT JOIN (
	SELECT sender_id, COUNT(*) AS cnt
	FROM "DirectMessage"
	WHERE receiver_id = ? AND is_read = false
	GROUP BY sender_id
) u ON u.sender_id = l.counterpart_id
ORDER BY l.created_at DESC
LIMIT 100`

// GetChatConversations returns the caller's chat threads. Identity is
// session-scoped (JWT): the userId is never accepted from the client.
func GetChatConversations(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}

	type conversationRow struct {
		CounterpartID string
		Content       string
		CreatedAt     time.Time
		Unread        int64
	}

	rows := make([]conversationRow, 0, 20)
	if err := readDB.
		Raw(chatConversationsSQL, userID, userID, userID, userID).
		WithContext(c.Request.Context()).
		Scan(&rows).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch conversations")
		return
	}

	// Batch-load counterpart profiles (name + avatar) for the thread list.
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.CounterpartID)
	}
	users := make([]models.User, 0, len(ids))
	if len(ids) > 0 {
		if err := readDB.
			Select("id", "name", "avatar").
			Where("id IN ?", ids).
			WithContext(c.Request.Context()).
			Find(&users).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to fetch conversation users")
			return
		}
	}
	userMap := make(map[string]*models.User, len(users))
	for i := range users {
		userMap[users[i].ID] = &users[i]
	}

	items := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		u, ok := userMap[r.CounterpartID]
		if !ok {
			// Counterpart no longer exists (soft-deleted) — skip the thread.
			continue
		}
		var avatar interface{}
		if u.Avatar != nil {
			avatar = *u.Avatar
		}
		items = append(items, gin.H{
			"id":              r.CounterpartID,
			"userId":          r.CounterpartID,
			"name":            u.GetName(),
			"avatar":          avatar,
			"lastMessage":     r.Content,
			"lastMessageTime": r.CreatedAt,
			"unreadCount":     r.Unread,
		})
	}

	api_response.Success(c, items)
}

// GetChatMessages returns the message thread between the caller and the
// counterpart user, and marks incoming messages as read. The counterpart id
// is the only parameter — the caller's identity comes from the JWT.
func GetChatMessages(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	chatUserID := strings.TrimSpace(c.Param("chatUserId"))
	if chatUserID == "" {
		api_response.Error(c, http.StatusBadRequest, "chatUserId is required")
		return
	}

	// Validate that the counterpart exists before returning anything.
	var counterpart models.User
	if err := db.DB.
		Select("id").
		Where("id = ?", chatUserID).
		First(&counterpart).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "User not found")
		return
	}

	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}

	messages := make([]models.DirectMessage, 0, 50)
	if err := readDB.
		Where(
			"(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
			userID, chatUserID, chatUserID, userID,
		).
		Order("created_at ASC").
		Limit(500).
		WithContext(c.Request.Context()).
		Find(&messages).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch messages")
		return
	}

	// The thread is now open — mark incoming messages as read.
	if err := db.DB.
		Model(&models.DirectMessage{}).
		Where("sender_id = ? AND receiver_id = ? AND is_read = false", chatUserID, userID).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": time.Now(),
		}).Error; err != nil {
		// Non-fatal: the messages were fetched; only the badge count lags.
		_ = err
	}

	items := make([]gin.H, 0, len(messages))
	for _, m := range messages {
		items = append(items, gin.H{
			"id":         m.ID,
			"senderId":   m.SenderID,
			"receiverId": m.ReceiverID,
			"content":    m.Content,
			"createdAt":  m.CreatedAt,
			"isRead":     m.IsRead,
		})
	}

	api_response.Success(c, items)
}

// SendChatMessage sends a direct message. The sender is always the session
// user — a client-supplied senderId is never trusted (IDOR/BOLA hardening).
func SendChatMessage(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var input struct {
		ReceiverID string `json:"receiverId" binding:"required"`
		Content    string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	input.Content = strings.TrimSpace(input.Content)
	if input.Content == "" {
		api_response.Error(c, http.StatusBadRequest, "content must not be empty")
		return
	}
	if utf8.RuneCountInString(input.Content) > 4000 {
		api_response.Error(c, http.StatusBadRequest, "content is too long")
		return
	}
	if input.ReceiverID == userID {
		api_response.Error(c, http.StatusBadRequest, "cannot send a message to yourself")
		return
	}

	// Validate that the receiver exists.
	var receiver models.User
	if err := db.DB.
		Select("id").
		Where("id = ?", input.ReceiverID).
		First(&receiver).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Receiver not found")
		return
	}

	message := models.DirectMessage{
		SenderID:   userID,
		ReceiverID: input.ReceiverID,
		Content:    input.Content,
	}
	if err := SafeCreate(db.DB, &message); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to send message")
		return
	}

	api_response.Created(c, gin.H{
		"id":         message.ID,
		"senderId":   message.SenderID,
		"receiverId": message.ReceiverID,
		"content":    message.Content,
		"createdAt":  message.CreatedAt,
		"isRead":     message.IsRead,
	})
}

// GetCommunityUsers returns a lightweight directory of users (id, name,
// avatar) the caller can start a chat with. Session-scoped: the caller's own
// entry is excluded based on the JWT identity, never a client-supplied id.
func GetCommunityUsers(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}

	limit := 50
	query := readDB.
		Model(&models.User{}).
		Select("id", "name", "avatar").
		Where("id <> ?", userID).
		Order("name ASC").
		Limit(limit)
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

	users := make([]models.User, 0, limit)
	if err := query.
		WithContext(c.Request.Context()).
		Find(&users).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch users")
		return
	}

	items := make([]gin.H, 0, len(users))
	for i := range users {
		var avatar interface{}
		if users[i].Avatar != nil {
			avatar = *users[i].Avatar
		}
		items = append(items, gin.H{
			"id":     users[i].ID,
			"name":   users[i].GetName(),
			"avatar": avatar,
		})
	}

	api_response.Success(c, items)
}

// GetCommunityUserProfile returns the public profile (id, name, avatar) of a
// single user — used for the chat header of a counterpart.
func GetCommunityUserProfile(c *gin.Context) {
	if c.GetString("userId") == "" {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	targetID := strings.TrimSpace(c.Param("id"))
	if targetID == "" {
		api_response.Error(c, http.StatusBadRequest, "id is required")
		return
	}

	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}

	var u models.User
	if err := readDB.
		Select("id", "name", "avatar").
		Where("id = ?", targetID).
		First(&u).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "User not found")
		return
	}

	var avatar interface{}
	if u.Avatar != nil {
		avatar = *u.Avatar
	}
	api_response.Success(c, gin.H{
		"id":     u.ID,
		"name":   u.GetName(),
		"avatar": avatar,
	})
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
