package protected

import (
	"math"
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/api/handlers/shared"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// AdminListPlans returns paginated subscription plans, newest first.
// Registered at GET /api/admin/plans (admin panel: إدارة الخطط والاشتراكات).
func AdminListPlans(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 50
	}

	query := db.DB.Model(&models.SubscriptionPlan{})
	if search := c.Query("search"); search != "" {
		query = query.Where("name ILIKE ? OR name_ar ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var plans []models.SubscriptionPlan
	if err := query.
		Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&plans).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch plans")
		return
	}

	api_response.Success(c, gin.H{
		"items": plans,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}

// AdminGetPlan returns a single plan by id.
// Registered at GET /api/admin/plans/:id.
func AdminGetPlan(c *gin.Context) {
	id := c.Param("id")
	var plan models.SubscriptionPlan
	if err := db.DB.First(&plan, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Plan not found")
		return
	}
	api_response.Success(c, plan)
}

type adminPlanRequest struct {
	Name        string   `json:"name" binding:"required,min=2"`
	NameAr      string   `json:"nameAr" binding:"required,min=2"`
	Description string   `json:"description"`
	Price       float64  `json:"price" binding:"min=0"`
	Currency    string   `json:"currency"`
	Interval    string   `json:"interval" binding:"required,oneof=MONTHLY YEARLY FOREVER"`
	IsActive    *bool    `json:"isActive"`
	Features    []string `json:"features"`
	// GroupKey links this plan to sibling interval variants (e.g. the
	// monthly and yearly rows of the same tier) so the storefront can offer
	// a billing-cycle toggle that switches between real, separately-priced
	// plan records. Optional: a plan with no group is its own single-member
	// group (defaults to its own id — see SubscriptionPlan.BeforeCreate).
	GroupKey string `json:"groupKey"`
}

// AdminCreatePlan creates a new subscription plan.
// Registered at POST /api/admin/plans.
func AdminCreatePlan(c *gin.Context) {
	var req adminPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	currency := req.Currency
	if currency == "" {
		currency = "EGP"
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	plan := models.SubscriptionPlan{
		Name:        req.Name,
		NameAr:      req.NameAr,
		Description: req.Description,
		Price:       decimal.NewFromFloat(req.Price),
		Currency:    currency,
		Interval:    models.SubscriptionInterval(req.Interval),
		IsActive:    isActive,
		GroupKey:    req.GroupKey,
		Features:    models.JSONStringArray(req.Features),
	}

	if err := SafeCreate(db.DB, &plan); err != nil {
		if err.Error() == "record already exists" {
			api_response.Error(c, http.StatusConflict, "يوجد بالفعل خطة بنفس هذا الاسم")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to create plan")
		return
	}
	api_response.Success(c, plan)
}

// AdminUpdatePlan updates an existing subscription plan. Partial updates are
// supported — only fields present in the payload are changed.
// Registered at PATCH /api/admin/plans/:id.
func AdminUpdatePlan(c *gin.Context) {
	id := c.Param("id")
	var plan models.SubscriptionPlan
	if err := db.DB.First(&plan, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Plan not found")
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if v, ok := req["name"]; ok {
		updates["name"] = v
	}
	if v, ok := req["nameAr"]; ok {
		updates["name_ar"] = v
	}
	if v, ok := req["description"]; ok {
		updates["description"] = v
	}
	if v, ok := req["price"]; ok {
		updates["price"] = v
	}
	if v, ok := req["currency"]; ok {
		updates["currency"] = v
	}
	if v, ok := req["interval"]; ok {
		updates["interval"] = v
	}
	if v, ok := req["isActive"]; ok {
		updates["is_active"] = v
	}
	if v, ok := req["groupKey"]; ok {
		updates["group_key"] = v
	}
	if v, ok := req["features"]; ok {
		updates["features"] = v
	}

	if len(updates) == 0 {
		api_response.Success(c, plan)
		return
	}

	if err := db.DB.Model(&plan).Updates(updates).Error; err != nil {
		if shared.IsDuplicateKeyError(err) {
			api_response.Error(c, http.StatusConflict, "يوجد بالفعل خطة بنفس هذا الاسم")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to update plan")
		return
	}

	db.DB.First(&plan, idQuery, id)
	api_response.Success(c, plan)
}

// AdminDeletePlan deletes a subscription plan. Plans that already have
// active subscriptions or payments referencing them are kept (deactivated
// instead) to avoid breaking historical invoices/payment records.
// Registered at DELETE /api/admin/plans/:id.
func AdminDeletePlan(c *gin.Context) {
	id := c.Param("id")
	var plan models.SubscriptionPlan
	if err := db.DB.First(&plan, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Plan not found")
		return
	}

	var refCount int64
	db.DB.Model(&models.UserSubscription{}).Where("plan_id = ?", id).Count(&refCount)
	if refCount == 0 {
		db.DB.Model(&models.Payment{}).Where("plan_id = ?", id).Count(&refCount)
	}

	if refCount > 0 {
		// Referenced by existing subscriptions/payments — deactivate instead
		// of deleting so historical records keep a valid foreign key.
		if err := db.DB.Model(&plan).Update("is_active", false).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to deactivate plan")
			return
		}
		api_response.Success(c, gin.H{"message": "الخطة مستخدمة من قبل مشتركين حاليين، تم تعطيلها بدلاً من حذفها"})
		return
	}

	if err := db.DB.Delete(&models.SubscriptionPlan{}, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete plan")
		return
	}
	api_response.Success(c, gin.H{"message": "Plan deleted"})
}
