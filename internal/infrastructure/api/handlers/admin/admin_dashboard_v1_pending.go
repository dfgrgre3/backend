package admin

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// pendingActionItem is one row in the "needs an admin decision" queue. The queue
// is assembled from several source tables, so the shape is normalized here.
type pendingActionItem struct {
	ID                 string     `json:"id"`
	Type               string     `json:"type"`
	Title              string     `json:"title"`
	EntityType         string     `json:"entityType"`
	EntityID           string     `json:"entityId"`
	Status             string     `json:"status"`
	Priority           string     `json:"priority"`
	CreatedAt          time.Time  `json:"createdAt"`
	DueAt              *time.Time `json:"dueAt"`
	AssignedTo         *string    `json:"assignedTo"`
	RequestedBy        string     `json:"requestedBy"`
	ServiceLevelStatus string     `json:"serviceLevelStatus"`
	ActionURL          string     `json:"actionUrl"`
	RequiredPermission string     `json:"requiredPermission"`
}

// pendingActionSortKeys are the fields a caller may sort the queue by.
var pendingActionSortKeys = map[string]string{
	"createdAt": "createdAt",
	"dueAt":     "dueAt",
	"priority":  "priority",
	"status":    "status",
}

// pendingPriorityRank orders priorities for sorting; higher is more urgent.
var pendingPriorityRank = map[string]int{"low": 1, "medium": 2, "high": 3, "urgent": 4}

// GetDashboardPendingActions returns items awaiting an administrative decision.
//
// GET /api/admin/dashboard/pending-actions
func GetDashboardPendingActions(c *gin.Context) {
	if !dashboardRequire(c, models.PermDashboardViewPendingItems) {
		return
	}
	readDB, abort := safeReadDB(c)
	if abort {
		return
	}
	page, ok := parseDashboardPage(c, pendingActionSortKeys, "createdAt")
	if !ok {
		return
	}

	typeFilter := strings.TrimSpace(c.Query("type"))
	statusFilter := strings.TrimSpace(c.Query("status"))
	priorityFilter := strings.ToLower(strings.TrimSpace(c.Query("priority")))

	var dueBefore *time.Time
	if raw := strings.TrimSpace(c.Query("dueBefore")); raw != "" {
		parsed, valid := parseDashboardDate(raw)
		if !valid {
			api_response.Error(c, http.StatusBadRequest, "dueBefore must be a valid date")
			return
		}
		dueBefore = &parsed
	}

	assignedToMe := false
	if raw := strings.TrimSpace(c.Query("assignedToMe")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			api_response.Error(c, http.StatusBadRequest, "assignedToMe must be a boolean")
			return
		}
		assignedToMe = parsed
	}
	currentUserID, _ := c.Get("userId")
	callerID, _ := currentUserID.(string)

	items := make([]pendingActionItem, 0, 64)

	// Courses awaiting content review.
	if wantsPendingType(typeFilter, "course_review") && dashboardCan(c, models.PermSubjectsView) {
		var courses []models.LmsCourse
		readDB.Where("deleted_at IS NULL AND status = ?", "REVIEW").
			Order(createdAtDescSort).Limit(dashboardMaxPageSize * 3).Find(&courses)
		for _, course := range courses {
			items = append(items, pendingActionItem{
				ID:                 course.ID.String(),
				Type:               "course_review",
				Title:              course.Title,
				EntityType:         "course",
				EntityID:           course.ID.String(),
				Status:             "pending",
				Priority:           agePriority(course.CreatedAt, 72*time.Hour, 168*time.Hour),
				CreatedAt:          course.CreatedAt,
				RequestedBy:        course.PrimaryInstructorID.String(),
				ServiceLevelStatus: ageSLAStatus(course.CreatedAt, nil, 168*time.Hour),
				ActionURL:          "/admin/courses/" + course.ID.String(),
				RequiredPermission: models.PermSubjectsView,
			})
		}
	}

	// Open and escalated support tickets.
	if wantsPendingType(typeFilter, "support_ticket") && dashboardCan(c, models.PermTicketsView) {
		var tickets []models.SupportTicket
		readDB.Where("deleted_at IS NULL AND status IN ?", []string{"open", "escalated"}).
			Order(createdAtDescSort).Limit(dashboardMaxPageSize * 3).Find(&tickets)
		for _, ticket := range tickets {
			items = append(items, pendingActionItem{
				ID:                 ticket.ID,
				Type:               "support_ticket",
				Title:              ticket.Subject,
				EntityType:         "support_ticket",
				EntityID:           ticket.ID,
				Status:             ticket.Status,
				Priority:           normalizePriority(ticket.Priority),
				CreatedAt:          ticket.CreatedAt,
				AssignedTo:         ticket.AssignedTo,
				RequestedBy:        ticket.UserID,
				ServiceLevelStatus: ageSLAStatus(ticket.CreatedAt, nil, 24*time.Hour),
				ActionURL:          "/admin/tickets/" + ticket.ID,
				RequiredPermission: models.PermTicketsView,
			})
		}
	}

	// Subscription orders stuck in PENDING are a financial queue.
	if wantsPendingType(typeFilter, "pending_order") && dashboardCan(c, models.PermDashboardViewFinancialMetrics) {
		var subscriptions []models.UserSubscription
		userSubscriptionScope(readDB).
			Where("status = ?", models.SubscriptionPending).
			Order(createdAtDescSort).Limit(dashboardMaxPageSize * 3).Find(&subscriptions)
		for _, sub := range subscriptions {
			items = append(items, pendingActionItem{
				ID:                 sub.ID,
				Type:               "pending_order",
				Title:              "اشتراك بانتظار التأكيد",
				EntityType:         "subscription",
				EntityID:           sub.ID,
				Status:             string(sub.Status),
				Priority:           agePriority(sub.CreatedAt, 24*time.Hour, 72*time.Hour),
				CreatedAt:          sub.CreatedAt,
				RequestedBy:        sub.UserID,
				ServiceLevelStatus: ageSLAStatus(sub.CreatedAt, nil, 48*time.Hour),
				ActionURL:          "/admin/orders/" + sub.ID,
				RequiredPermission: models.PermDashboardViewFinancialMetrics,
			})
		}
	}

	filtered := items[:0]
	needle := strings.ToLower(page.Query)
	for _, item := range items {
		if statusFilter != "" && !strings.EqualFold(item.Status, statusFilter) {
			continue
		}
		if priorityFilter != "" && item.Priority != priorityFilter {
			continue
		}
		if dueBefore != nil && (item.DueAt == nil || item.DueAt.After(*dueBefore)) {
			continue
		}
		if assignedToMe && (item.AssignedTo == nil || *item.AssignedTo != callerID) {
			continue
		}
		if needle != "" &&
			!strings.Contains(strings.ToLower(item.Title), needle) &&
			!strings.Contains(strings.ToLower(item.RequestedBy), needle) &&
			!strings.Contains(strings.ToLower(item.EntityType), needle) {
			continue
		}
		filtered = append(filtered, item)
	}

	sortPendingActions(filtered, page.SortBy, page.Direction)

	total := int64(len(filtered))
	start := page.Offset
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + page.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	api_response.Success(c, dashboardListResponse(filtered[start:end], total, page, nil))
}

// wantsPendingType reports whether a source should contribute to the result.
func wantsPendingType(filter, candidate string) bool {
	return filter == "" || strings.EqualFold(filter, candidate)
}

// sortPendingActions orders the merged queue in memory, since the rows come
// from several tables and cannot be sorted by the database.
func sortPendingActions(items []pendingActionItem, sortBy, direction string) {
	desc := direction != "asc"
	sort.SliceStable(items, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "dueAt":
			switch {
			case items[i].DueAt == nil && items[j].DueAt == nil:
				less = items[i].CreatedAt.Before(items[j].CreatedAt)
			case items[i].DueAt == nil:
				less = false
			case items[j].DueAt == nil:
				less = true
			default:
				less = items[i].DueAt.Before(*items[j].DueAt)
			}
		case "priority":
			less = pendingPriorityRank[items[i].Priority] < pendingPriorityRank[items[j].Priority]
		case "status":
			less = items[i].Status < items[j].Status
		default:
			less = items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		if desc {
			return !less
		}
		return less
	})
}

// agePriority escalates an item's priority the longer it sits unhandled.
func agePriority(createdAt time.Time, highAfter, urgentAfter time.Duration) string {
	age := time.Since(createdAt)
	switch {
	case age >= urgentAfter:
		return "urgent"
	case age >= highAfter:
		return "high"
	default:
		return "medium"
	}
}

// ageSLAStatus classifies an item against its service-level target.
func ageSLAStatus(createdAt time.Time, dueAt *time.Time, target time.Duration) string {
	deadline := createdAt.Add(target)
	if dueAt != nil {
		deadline = *dueAt
	}
	remaining := time.Until(deadline)
	switch {
	case remaining < 0:
		return "breached"
	case remaining < target/4:
		return "at_risk"
	default:
		return "on_track"
	}
}

// normalizePriority maps arbitrary source priorities onto the shared scale.
func normalizePriority(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if _, ok := pendingPriorityRank[value]; ok {
		return value
	}
	return "medium"
}
