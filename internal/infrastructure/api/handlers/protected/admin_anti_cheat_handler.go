package protected

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────
//  Anti-Cheat (مكافحة الغش)
//
//  Two-layer model:
//    * AntiCheatEvent — raw proctoring events (tab switch, blur, copy/paste...)
//    * AntiCheatFlag  — aggregated review case per (user, exam, attempt):
//                       risk score, status, evidence, review outcome.
// ─────────────────────────────────────────────

// severityRisk weights a single event's contribution to the flag risk score.
func severityRisk(sev models.AntiCheatSeverity) int {
	switch sev {
	case models.AntiCheatSeverityCritical:
		return 50
	case models.AntiCheatSeverityHigh:
		return 30
	case models.AntiCheatSeverityMedium:
		return 15
	default:
		return 5
	}
}

// defaultSeverityForEventType picks a sensible default severity when the
// reporting client does not supply one.
func defaultSeverityForEventType(eventType string) models.AntiCheatSeverity {
	switch models.AntiCheatEventType(eventType) {
	case models.AntiCheatMultiDevice, models.AntiCheatScreenshot:
		return models.AntiCheatSeverityCritical
	case models.AntiCheatCopyPaste, models.AntiCheatFullscreenExit,
		models.AntiCheatCameraOff, models.AntiCheatVoiceDetected,
		models.AntiCheatIdleTimeout:
		return models.AntiCheatSeverityHigh
	case models.AntiCheatTabSwitch:
		return models.AntiCheatSeverityMedium
	default:
		return models.AntiCheatSeverityLow
	}
}

func validSeverity(sev models.AntiCheatSeverity) bool {
	switch sev {
	case models.AntiCheatSeverityLow, models.AntiCheatSeverityMedium,
		models.AntiCheatSeverityHigh, models.AntiCheatSeverityCritical:
		return true
	}
	return false
}

func validStatus(status models.AntiCheatStatus) bool {
	switch status {
	case models.AntiCheatStatusOpen, models.AntiCheatStatusUnderReview,
		models.AntiCheatStatusCleared, models.AntiCheatStatusDismissed,
		models.AntiCheatStatusBlocked:
		return true
	}
	return false
}

// antiCheatEventScope returns a query scoped to the same (user, exam, attempt)
// key as a flag, matching NULL keys with IS NOT DISTINCT FROM.
func antiCheatEventScope(userID string, examID, attemptID *string) *gorm.DB {
	return db.DB.Where(
		"user_id = ? AND exam_id IS NOT DISTINCT FROM ? AND attempt_id IS NOT DISTINCT FROM ?",
		userID, examID, attemptID,
	)
}

// ─────────────────────────────────────────────
//  List flags (main admin table)
// ─────────────────────────────────────────────

type antiCheatFlagRow struct {
	ID          string     `gorm:"column:id"`
	UserID      string     `gorm:"column:user_id"`
	ExamID      *string    `gorm:"column:exam_id"`
	AttemptID   *string    `gorm:"column:attempt_id"`
	RiskScore   int        `gorm:"column:risk_score"`
	Status      string     `gorm:"column:status"`
	Reason      string     `gorm:"column:reason"`
	EventCount  int        `gorm:"column:event_count"`
	IPAddress   string     `gorm:"column:ip_address"`
	ReviewerID  *string    `gorm:"column:reviewer_id"`
	ReviewedAt  *time.Time `gorm:"column:reviewed_at"`
	ReviewNote  *string    `gorm:"column:review_note"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
	UserName    string     `gorm:"column:user_name"`
	UserEmail   string     `gorm:"column:user_email"`
	ExamTitle   string     `gorm:"column:exam_title"`
	LastEventAt *time.Time `gorm:"column:last_event_at"`
}

type antiCheatEventRow struct {
	ID        string    `gorm:"column:id"`
	UserID    string    `gorm:"column:user_id"`
	ExamID    *string   `gorm:"column:exam_id"`
	AttemptID *string   `gorm:"column:attempt_id"`
	EventType string    `gorm:"column:event_type"`
	Severity  string    `gorm:"column:severity"`
	Detail    *string   `gorm:"column:detail"`
	Metadata  []byte    `gorm:"column:metadata"`
	IPAddress string    `gorm:"column:ip_address"`
	UserAgent string    `gorm:"column:user_agent"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UserName  string    `gorm:"column:user_name"`
	UserEmail string    `gorm:"column:user_email"`
	ExamTitle string    `gorm:"column:exam_title"`
}

const antiCheatFlagSelect = `SELECT f.id, f.user_id, f.exam_id, f.attempt_id, f.risk_score, f.status,
	f.reason, f.event_count, f.ip_address, f.reviewer_id, f.reviewed_at, f.review_note,
	f.created_at, f.updated_at,
	COALESCE(u.name, '') AS user_name, COALESCE(u.email, '') AS user_email,
	COALESCE(e.title, '') AS exam_title,
	(SELECT MAX(ev.created_at) FROM "AntiCheatEvent" ev
	 WHERE ev.user_id = f.user_id
	   AND ev.exam_id IS NOT DISTINCT FROM f.exam_id
	   AND ev.attempt_id IS NOT DISTINCT FROM f.attempt_id) AS last_event_at
	FROM "AntiCheatFlag" f
	LEFT JOIN "User" u ON u.id = f.user_id
	LEFT JOIN "Exam" e ON e.id = f.exam_id`

// buildFlagFilter appends the shared flag filters (search/status/exam/minRisk)
// to a SQL WHERE clause, returning the joined condition string and args.
func buildFlagFilter(c *gin.Context) (string, []interface{}) {
	conds := []string{}
	args := []interface{}{}

	if search := strings.TrimSpace(c.Query("search")); search != "" {
		conds = append(conds, "(u.name ILIKE ? OR u.email ILIKE ?)")
		like := "%" + search + "%"
		args = append(args, like, like)
	}
	if status := c.Query("status"); status != "" && status != "all" {
		conds = append(conds, "f.status = ?")
		args = append(args, status)
	}
	if examID := c.Query("examId"); examID != "" {
		conds = append(conds, "f.exam_id = ?")
		args = append(args, examID)
	}
	if minRisk, _ := strconv.Atoi(c.Query("minRisk")); minRisk > 0 {
		conds = append(conds, "f.risk_score >= ?")
		args = append(args, minRisk)
	}

	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func AdminListAntiCheatFlags(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	where, args := buildFlagFilter(c)

	var total int64
	countSQL := `SELECT COUNT(*) FROM "AntiCheatFlag" f
		LEFT JOIN "User" u ON u.id = f.user_id
		LEFT JOIN "Exam" e ON e.id = f.exam_id` + where
	if err := db.DB.Raw(countSQL, args...).Scan(&total).Error; err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch anti-cheat flags", err)
		return
	}

	var rows []antiCheatFlagRow
	selectSQL := antiCheatFlagSelect + where +
		` ORDER BY f.created_at DESC, f.id DESC LIMIT ? OFFSET ?`
	selectArgs := append(append([]interface{}{}, args...), limit, offset)
	if err := db.DB.Raw(selectSQL, selectArgs...).Scan(&rows).Error; err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch anti-cheat flags", err)
		return
	}

	items := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		items = append(items, antiCheatFlagToGin(r))
	}

	api_response.Success(c, gin.H{
		"flags":      items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    antiCheatSummary(c),
	})
}

func antiCheatFlagToGin(r antiCheatFlagRow) gin.H {
	return gin.H{
		"id":          r.ID,
		"userId":      r.UserID,
		"userName":    r.UserName,
		"userEmail":   r.UserEmail,
		"examId":      r.ExamID,
		"examTitle":   r.ExamTitle,
		"attemptId":   r.AttemptID,
		"riskScore":   r.RiskScore,
		"status":      r.Status,
		"reason":      r.Reason,
		"eventCount":  r.EventCount,
		"ipAddress":   r.IPAddress,
		"reviewerId":  r.ReviewerID,
		"reviewedAt":  r.ReviewedAt,
		"reviewNote":  r.ReviewNote,
		"lastEventAt": r.LastEventAt,
		"createdAt":   r.CreatedAt,
		"updatedAt":   r.UpdatedAt,
	}
}

// antiCheatSummary computes whole-table counts for the header stats cards.
func antiCheatSummary(c *gin.Context) gin.H {
	var total, open, underReview, cleared, dismissed, blocked, highRisk, uniqueStudents, totalEvents, criticalEvents, todayEvents int64
	db.DB.Model(&models.AntiCheatFlag{}).Count(&total)
	db.DB.Model(&models.AntiCheatFlag{}).Where("status = ?", models.AntiCheatStatusOpen).Count(&open)
	db.DB.Model(&models.AntiCheatFlag{}).Where("status = ?", models.AntiCheatStatusUnderReview).Count(&underReview)
	db.DB.Model(&models.AntiCheatFlag{}).Where("status = ?", models.AntiCheatStatusCleared).Count(&cleared)
	db.DB.Model(&models.AntiCheatFlag{}).Where("status = ?", models.AntiCheatStatusDismissed).Count(&dismissed)
	db.DB.Model(&models.AntiCheatFlag{}).Where("status = ?", models.AntiCheatStatusBlocked).Count(&blocked)
	db.DB.Model(&models.AntiCheatFlag{}).Where("risk_score >= ?", 70).Count(&highRisk)
	db.DB.Model(&models.AntiCheatFlag{}).Distinct("user_id").Count(&uniqueStudents)
	db.DB.Model(&models.AntiCheatEvent{}).Count(&totalEvents)
	db.DB.Model(&models.AntiCheatEvent{}).Where("severity = ?", models.AntiCheatSeverityCritical).Count(&criticalEvents)
	db.DB.Model(&models.AntiCheatEvent{}).Where("created_at >= ?", time.Now().Truncate(24*time.Hour)).Count(&todayEvents)

	return gin.H{
		"totalFlags":       total,
		"open":             open,
		"underReview":      underReview,
		"cleared":          cleared,
		"dismissed":        dismissed,
		"blocked":          blocked,
		"highRisk":         highRisk,
		"uniqueStudents":   uniqueStudents,
		"totalEvents":      totalEvents,
		"criticalEvents":   criticalEvents,
		"todayEvents":      todayEvents,
	}
}

// ─────────────────────────────────────────────
//  Flag detail + events timeline
// ─────────────────────────────────────────────

func AdminGetAntiCheatFlag(c *gin.Context) {
	id := c.Param("id")
	var flag models.AntiCheatFlag
	if err := db.DB.Where("id = ?", id).First(&flag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, "Anti-cheat flag not found")
			return
		}
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch anti-cheat flag", err)
		return
	}

	var events []models.AntiCheatEvent
	if err := antiCheatEventScope(flag.UserID, flag.ExamID, flag.AttemptID).
		Order("created_at ASC, id ASC").
		Find(&events).Error; err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch anti-cheat events", err)
		return
	}

	userName, userEmail := resolveUserNameEmail(flag.UserID)
	examTitle := ""
	if flag.ExamID != nil {
		var exam models.Exam
		if err := db.DB.Select("title").Where("id = ?", *flag.ExamID).First(&exam).Error; err == nil {
			examTitle = exam.Title
		}
	}

	eventItems := make([]gin.H, 0, len(events))
	for _, ev := range events {
		eventItems = append(eventItems, antiCheatEventToGin(ev, userName, userEmail, examTitle))
	}

	api_response.Success(c, gin.H{
		"flag": gin.H{
			"id":          flag.ID,
			"userId":      flag.UserID,
			"userName":    userName,
			"userEmail":   userEmail,
			"examId":      flag.ExamID,
			"examTitle":   examTitle,
			"attemptId":   flag.AttemptID,
			"riskScore":   flag.RiskScore,
			"status":      flag.Status,
			"reason":      flag.Reason,
			"eventCount":  flag.EventCount,
			"evidence":    rawJSONOrEmpty(flag.Evidence),
			"ipAddress":   flag.IPAddress,
			"reviewerId":  flag.ReviewerID,
			"reviewedAt":  flag.ReviewedAt,
			"reviewNote":  flag.ReviewNote,
			"createdAt":   flag.CreatedAt,
			"updatedAt":   flag.UpdatedAt,
		},
		"events": eventItems,
	})
}

// resolveUserNameEmail looks up a user's display name and email.
func resolveUserNameEmail(userID string) (string, string) {
	var user models.User
	if err := db.DB.Select("name", "email").Where("id = ?", userID).First(&user).Error; err != nil {
		return "", ""
	}
	name := ""
	if user.Name != nil {
		name = *user.Name
	}
	return name, user.Email
}

func rawJSONOrEmpty(b []byte) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("{}")
	}
	return json.RawMessage(b)
}

// ─────────────────────────────────────────────
//  Update flag status / review note
// ─────────────────────────────────────────────

func AdminUpdateAntiCheatFlag(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Status     *string `json:"status"`
		ReviewNote *string `json:"reviewNote"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	var flag models.AntiCheatFlag
	if err := db.DB.Where("id = ?", id).First(&flag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, "Anti-cheat flag not found")
			return
		}
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch anti-cheat flag", err)
		return
	}

	changed := false
	if body.Status != nil {
		status := models.AntiCheatStatus(*body.Status)
		if !validStatus(status) {
			api_response.Error(c, http.StatusBadRequest, "Invalid status")
			return
		}
		if flag.Status != status {
			flag.Status = status
			changed = true
		}
	}
	if body.ReviewNote != nil {
		if flag.ReviewNote == nil || *flag.ReviewNote != *body.ReviewNote {
			flag.ReviewNote = body.ReviewNote
			changed = true
		}
	}

	if changed {
		now := time.Now()
		if reviewerID := c.GetString("userId"); reviewerID != "" {
			flag.ReviewerID = &reviewerID
		}
		flag.ReviewedAt = &now
		flag.UpdatedAt = now
		if err := db.DB.Save(&flag).Error; err != nil {
			api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to update anti-cheat flag", err)
			return
		}
	}

	userName, userEmail := resolveUserNameEmail(flag.UserID)
	examTitle := ""
	if flag.ExamID != nil {
		var exam models.Exam
		if err := db.DB.Select("title").Where("id = ?", *flag.ExamID).First(&exam).Error; err == nil {
			examTitle = exam.Title
		}
	}

	api_response.Success(c, gin.H{
		"flag": gin.H{
			"id":          flag.ID,
			"userId":      flag.UserID,
			"userName":    userName,
			"userEmail":   userEmail,
			"examId":      flag.ExamID,
			"examTitle":   examTitle,
			"attemptId":   flag.AttemptID,
			"riskScore":   flag.RiskScore,
			"status":      flag.Status,
			"reason":      flag.Reason,
			"eventCount":  flag.EventCount,
			"evidence":    rawJSONOrEmpty(flag.Evidence),
			"ipAddress":   flag.IPAddress,
			"reviewerId":  flag.ReviewerID,
			"reviewedAt":  flag.ReviewedAt,
			"reviewNote":  flag.ReviewNote,
			"createdAt":   flag.CreatedAt,
			"updatedAt":   flag.UpdatedAt,
		},
	})
}

// ─────────────────────────────────────────────
//  Events list (raw proctoring log)
// ─────────────────────────────────────────────

func AdminListAntiCheatEvents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	conds := []string{}
	args := []interface{}{}

	if search := strings.TrimSpace(c.Query("search")); search != "" {
		conds = append(conds, "(u.name ILIKE ? OR u.email ILIKE ?)")
		like := "%" + search + "%"
		args = append(args, like, like)
	}
	if eventType := c.Query("type"); eventType != "" && eventType != "all" {
		conds = append(conds, "ev.event_type = ?")
		args = append(args, eventType)
	}
	if severity := c.Query("severity"); severity != "" && severity != "all" {
		conds = append(conds, "ev.severity = ?")
		args = append(args, severity)
	}
	if userID := c.Query("userId"); userID != "" {
		conds = append(conds, "ev.user_id = ?")
		args = append(args, userID)
	}
	if examID := c.Query("examId"); examID != "" {
		conds = append(conds, "ev.exam_id = ?")
		args = append(args, examID)
	}

	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	base := `FROM "AntiCheatEvent" ev
		LEFT JOIN "User" u ON u.id = ev.user_id
		LEFT JOIN "Exam" e ON e.id = ev.exam_id`

	var total int64
	if err := db.DB.Raw(`SELECT COUNT(*) `+base+where, args...).Scan(&total).Error; err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch anti-cheat events", err)
		return
	}

	selectSQL := `SELECT ev.id, ev.user_id, ev.exam_id, ev.attempt_id, ev.event_type, ev.severity,
		ev.detail, ev.metadata, ev.ip_address, ev.user_agent, ev.created_at,
		COALESCE(u.name, '') AS user_name, COALESCE(u.email, '') AS user_email,
		COALESCE(e.title, '') AS exam_title
		` + base + where + ` ORDER BY ev.created_at DESC, ev.id DESC LIMIT ? OFFSET ?`
	selectArgs := append(append([]interface{}{}, args...), limit, offset)

	var rows []antiCheatEventRow
	if err := db.DB.Raw(selectSQL, selectArgs...).Scan(&rows).Error; err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch anti-cheat events", err)
		return
	}

	items := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		items = append(items, antiCheatEventRowToGin(r))
	}

	// Summary (whole table, unfiltered).
	var totalEvents, criticalCount, highCount, mediumCount, lowCount, todayCount, uniqueStudents int64
	db.DB.Model(&models.AntiCheatEvent{}).Count(&totalEvents)
	db.DB.Model(&models.AntiCheatEvent{}).Where("severity = ?", models.AntiCheatSeverityCritical).Count(&criticalCount)
	db.DB.Model(&models.AntiCheatEvent{}).Where("severity = ?", models.AntiCheatSeverityHigh).Count(&highCount)
	db.DB.Model(&models.AntiCheatEvent{}).Where("severity = ?", models.AntiCheatSeverityMedium).Count(&mediumCount)
	db.DB.Model(&models.AntiCheatEvent{}).Where("severity = ?", models.AntiCheatSeverityLow).Count(&lowCount)
	db.DB.Model(&models.AntiCheatEvent{}).Where("created_at >= ?", time.Now().Truncate(24*time.Hour)).Count(&todayCount)
	db.DB.Model(&models.AntiCheatEvent{}).Distinct("user_id").Count(&uniqueStudents)

	api_response.Success(c, gin.H{
		"events":     items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary": gin.H{
			"totalEvents":    totalEvents,
			"criticalCount":  criticalCount,
			"highCount":      highCount,
			"mediumCount":    mediumCount,
			"lowCount":       lowCount,
			"todayCount":     todayCount,
			"uniqueStudents": uniqueStudents,
		},
	})
}

func antiCheatEventRowToGin(r antiCheatEventRow) gin.H {
	return gin.H{
		"id":         r.ID,
		"userId":     r.UserID,
		"userName":   r.UserName,
		"userEmail":  r.UserEmail,
		"examId":     r.ExamID,
		"examTitle":  r.ExamTitle,
		"attemptId":  r.AttemptID,
		"eventType":  r.EventType,
		"severity":   r.Severity,
		"detail":     r.Detail,
		"metadata":   rawJSONOrEmpty(r.Metadata),
		"ipAddress":  r.IPAddress,
		"userAgent":  r.UserAgent,
		"createdAt":  r.CreatedAt,
	}
}

func antiCheatEventToGin(ev models.AntiCheatEvent, userName, userEmail, examTitle string) gin.H {
	return gin.H{
		"id":        ev.ID,
		"userId":    ev.UserID,
		"userName":  userName,
		"userEmail": userEmail,
		"examId":    ev.ExamID,
		"examTitle": examTitle,
		"attemptId": ev.AttemptID,
		"eventType": ev.EventType,
		"severity":  ev.Severity,
		"detail":    ev.Detail,
		"metadata":  rawJSONOrEmpty(ev.Metadata),
		"ipAddress": ev.IPAddress,
		"userAgent": ev.UserAgent,
		"createdAt": ev.CreatedAt,
	}
}

// ─────────────────────────────────────────────
//  Stats (header cards, whole table)
// ─────────────────────────────────────────────

func AdminAntiCheatStats(c *gin.Context) {
	api_response.Success(c, gin.H{"summary": antiCheatSummary(c)})
}

// ─────────────────────────────────────────────
//  Record event + upsert flag
// ─────────────────────────────────────────────

func AdminRecordAntiCheatEvent(c *gin.Context) {
	var body struct {
		UserID    string          `json:"userId"`
		ExamID    *string         `json:"examId"`
		AttemptID *string         `json:"attemptId"`
		EventType string          `json:"eventType"`
		Severity  string          `json:"severity"`
		Detail    *string         `json:"detail"`
		Metadata  json.RawMessage `json:"metadata"`
		IPAddress string          `json:"ipAddress"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	eventType := strings.ToUpper(strings.TrimSpace(body.EventType))
	if eventType == "" {
		api_response.Error(c, http.StatusBadRequest, "eventType is required")
		return
	}
	if strings.TrimSpace(body.UserID) == "" {
		api_response.Error(c, http.StatusBadRequest, "userId is required")
		return
	}

	// Validate that the referenced user/exam actually exist so the client gets
	// a clear message instead of a raw foreign-key violation.
	var user models.User
	if err := db.DB.Select("id").Where("id = ?", body.UserID).First(&user).Error; err != nil {
		api_response.Error(c, http.StatusBadRequest, "User not found")
		return
	}
	if body.ExamID != nil && *body.ExamID != "" {
		var exam models.Exam
		if err := db.DB.Select("id").Where("id = ?", *body.ExamID).First(&exam).Error; err != nil {
			api_response.Error(c, http.StatusBadRequest, "Exam not found")
			return
		}
	}

	severity := models.AntiCheatSeverity(strings.ToUpper(body.Severity))
	if !validSeverity(severity) {
		severity = defaultSeverityForEventType(eventType)
	}

	metadata := []byte("{}")
	if len(body.Metadata) > 0 && string(body.Metadata) != "null" {
		metadata = body.Metadata
	}

	event := models.AntiCheatEvent{
		UserID:    body.UserID,
		ExamID:    body.ExamID,
		AttemptID: body.AttemptID,
		EventType: models.AntiCheatEventType(eventType),
		Severity:  severity,
		Detail:    body.Detail,
		Metadata:  metadata,
		IPAddress: body.IPAddress,
		UserAgent: c.Request.UserAgent(),
		CreatedAt: time.Now(),
	}
	if err := db.DB.Create(&event).Error; err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to record anti-cheat event", err)
		return
	}

	// Upsert the aggregated flag for this (user, exam, attempt) key.
	contribution := severityRisk(severity)
	var flag models.AntiCheatFlag
	err := db.DB.Where(
		"user_id = ? AND exam_id IS NOT DISTINCT FROM ? AND attempt_id IS NOT DISTINCT FROM ?",
		body.UserID, body.ExamID, body.AttemptID,
	).First(&flag).Error

	switch {
	case err == gorm.ErrRecordNotFound:
		flag = models.AntiCheatFlag{
			UserID:     body.UserID,
			ExamID:     body.ExamID,
			AttemptID:  body.AttemptID,
			RiskScore:  min(100, contribution),
			Status:     models.AntiCheatStatusOpen,
			EventCount: 1,
			IPAddress:  body.IPAddress,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if body.Detail != nil {
			flag.Reason = *body.Detail
		} else {
			flag.Reason = eventType
		}
		if err := db.DB.Create(&flag).Error; err != nil {
			api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to create anti-cheat flag", err)
			return
		}
	case err == nil:
		flag.EventCount++
		flag.RiskScore = min(100, flag.RiskScore+contribution)
		if flag.Reason == "" {
			if body.Detail != nil {
				flag.Reason = *body.Detail
			} else {
				flag.Reason = eventType
			}
		}
		flag.UpdatedAt = time.Now()
		if err := db.DB.Save(&flag).Error; err != nil {
			api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to update anti-cheat flag", err)
			return
		}
	default:
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to load anti-cheat flag", err)
		return
	}

	// Refresh the evidence summary (per-type event breakdown).
	evidence := buildAntiCheatEvidence(flag)
	db.DB.Model(&models.AntiCheatFlag{}).Where("id = ?", flag.ID).Update("evidence", evidence)

	api_response.Success(c, gin.H{
		"event": antiCheatEventToGin(event, "", "", ""),
		"flag":  gin.H{"id": flag.ID, "riskScore": flag.RiskScore, "eventCount": flag.EventCount, "status": flag.Status},
	})
}

// buildAntiCheatEvidence aggregates the flag's events into a JSON summary.
func buildAntiCheatEvidence(flag models.AntiCheatFlag) []byte {
	type typeCount struct {
		EventType string `json:"eventType"`
		Count     int64  `json:"count"`
	}
	type severityCount struct {
		Severity string `json:"severity"`
		Count    int64  `json:"count"`
	}

	var byType []typeCount
	db.DB.Model(&models.AntiCheatEvent{}).
		Select("event_type, COUNT(*) AS count").
		Where("user_id = ? AND exam_id IS NOT DISTINCT FROM ? AND attempt_id IS NOT DISTINCT FROM ?",
			flag.UserID, flag.ExamID, flag.AttemptID).
		Group("event_type").Order("count DESC").Scan(&byType)

	var bySeverity []severityCount
	db.DB.Model(&models.AntiCheatEvent{}).
		Select("severity, COUNT(*) AS count").
		Where("user_id = ? AND exam_id IS NOT DISTINCT FROM ? AND attempt_id IS NOT DISTINCT FROM ?",
			flag.UserID, flag.ExamID, flag.AttemptID).
		Group("severity").Order("count DESC").Scan(&bySeverity)

	if byType == nil {
		byType = []typeCount{}
	}
	if bySeverity == nil {
		bySeverity = []severityCount{}
	}

	out, _ := json.Marshal(gin.H{
		"riskScore":   flag.RiskScore,
		"eventCount":  flag.EventCount,
		"byEventType": byType,
		"bySeverity":  bySeverity,
	})
	return out
}
