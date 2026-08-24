package protected

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"
	paymentservice "thanawy-backend/internal/domain/payment/service"
	"time"

	"thanawy-backend/internal/infrastructure/api/middleware"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"thanawy-backend/internal/infrastructure/storage"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// AdminExamsBulkUpload imports questions for a new exam from a CSV file.
func AdminExamsBulkUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "No file uploaded")
		return
	}

	subjectID := c.PostForm("subjectId")
	examTitle := c.PostForm("title")
	if subjectID == "" || examTitle == "" {
		api_response.Error(c, http.StatusBadRequest, "Subject ID and Exam Title are required")
		return
	}

	f, err := file.Open()
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to open file")
		return
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Failed to parse CSV")
		return
	}

	if len(records) < 2 {
		api_response.Error(c, http.StatusBadRequest, "CSV is empty or missing headers")
		return
	}

	exam := models.Exam{
		SubjectID: subjectID,
		Title:     examTitle,
		Type:      models.ExamTypeQuiz,
	}
	if err := SafeCreate(db.DB, &exam); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create exam")
		return
	}

	importedCount := 0
	for i := 1; i < len(records); i++ {
		row := records[i]
		if len(row) < 6 {
			continue
		}

		text := row[0]
		options := []string{row[1], row[2], row[3], row[4]}
		answer := row[5]

		optionsJSON, _ := json.Marshal(options)

		question := models.Question{
			ExamID:  exam.ID,
			Text:    text,
			Options: string(optionsJSON),
			Answer:  answer,
		}

		if err := SafeCreate(db.DB, &question); err == nil {
			importedCount++
		}
	}

	api_response.Success(c, gin.H{
		"examId":   exam.ID,
		"imported": importedCount,
		"message":  fmt.Sprintf("تم استيراد %d سؤال بنجاح في اختبار %s", importedCount, examTitle),
	})
	LogAudit(c, "BULK_UPLOAD_EXAM", "exam", exam.ID, gin.H{"importedCount": importedCount})
}

func ValidateCoupon(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var coupon models.Coupon
	if err := db.DB.Where("code = ? AND "+isActiveQuery, req.Code, true).First(&coupon).Error; err != nil {
		api_response.Success(c, gin.H{
			"valid":   false,
			"message": "كود الخصم غير صالح",
		})
		return
	}

	if coupon.ExpiryDate != nil && coupon.ExpiryDate.Before(time.Now()) {
		api_response.Success(c, gin.H{
			"valid":   false,
			"message": "كود الخصم منتهي الصلاحية",
		})
		return
	}

	if coupon.MaxUses != nil && coupon.UsedCount >= *coupon.MaxUses {
		api_response.Success(c, gin.H{
			"valid":   false,
			"message": "تم استنفاذ عدد مرات استخدام الكود",
		})
		return
	}

	api_response.Success(c, gin.H{
		"valid":        true,
		"discountType": coupon.DiscountType,
		"discount":     coupon.DiscountValue,
		"message":      "تم تطبيق الخصم بنجاح",
	})
}

func applyCoupon(couponCode string, price float64) (*models.Coupon, float64) {
	if couponCode == "" {
		return nil, price
	}
	var coupon models.Coupon
	if err := db.DB.Where("code = ? AND "+isActiveQuery, couponCode, true).First(&coupon).Error; err != nil {
		return nil, price
	}

	isExpired := coupon.ExpiryDate != nil && coupon.ExpiryDate.Before(time.Now())
	isMaxUsed := coupon.MaxUses != nil && coupon.UsedCount >= *coupon.MaxUses
	isBelowMin := coupon.MinOrderAmount.GreaterThan(decimal.Zero) && decimal.NewFromFloat(price).LessThan(coupon.MinOrderAmount)

	if isExpired || isMaxUsed || isBelowMin {
		return nil, price
	}

	if coupon.DiscountType == "PERCENTAGE" {
		discountPercent := coupon.DiscountValue.Div(decimal.NewFromInt(100))
		price = price * (1 - discountPercent.InexactFloat64())
	} else {
		price = price - coupon.DiscountValue.InexactFloat64()
	}

	if price < 0 {
		price = 0
	}
	return &coupon, price
}

// SubscriptionCheckout is the single entry point the storefront's plan
// upgrade flow calls, covering all three payment methods it offers:
// card / wallet (Paymob) and internal_wallet (platform balance). It looks
// the plan up by real id — resolving billingCycle to the sibling plan row
// sharing the same GroupKey when the caller wants the other interval — so
// pricing always comes from the SubscriptionPlan table, never a guess.
func SubscriptionCheckout(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	var req struct {
		PlanID        string `json:"planId" binding:"required"`
		BillingCycle  string `json:"billingCycle"` // "monthly" | "yearly" (optional)
		PaymentMethod string `json:"paymentMethod" binding:"required,oneof=card wallet internal_wallet"`
		CouponCode    string `json:"couponCode"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	plan, err := resolveCheckoutPlan(req.PlanID, req.BillingCycle)
	if err != nil {
		api_response.Error(c, http.StatusNotFound, errPlanNotFound)
		return
	}

	price, _ := plan.Price.Float64()
	appliedCoupon, discountedPrice := applyCoupon(req.CouponCode, price)
	price = discountedPrice

	if req.PaymentMethod == "internal_wallet" {
		checkoutPurchasePlanWithWallet(c, userId, plan, price, appliedCoupon)
		return
	}

	var user models.User
	db.DB.First(&user, queryID, userId)

	paymobSvc := paymentservice.NewPaymobService()
	token, err := paymobSvc.Authenticate()
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "فشل الاتصال ببوابة الدفع")
		return
	}

	amountCents := int64(price * 100)
	items := []interface{}{
		map[string]interface{}{
			"name":         fmt.Sprintf("Subscription: %s", plan.Name),
			"amount_cents": amountCents,
			"description":  fmt.Sprintf("Plan ID: %s", plan.ID),
			"quantity":     1,
		},
	}

	orderID, err := paymobSvc.RegisterOrder(token, amountCents, items)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "فشل إنشاء طلب الدفع")
		return
	}

	billingData := getBillingData(user)
	integrationID := getIntegrationID(paymobSvc, req.PaymentMethod)

	paymentKey, err := paymobSvc.GetPaymentKey(token, orderID, amountCents, integrationID, billingData)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "فشل استخراج مفتاح الدفع")
		return
	}

	payment := models.Payment{
		UserID:        userId,
		PlanID:        plan.ID,
		Amount:        decimal.NewFromFloat(price),
		Currency:      plan.Currency,
		Method:        "PAYMOB_" + req.PaymentMethod,
		Status:        models.PaymentPending,
		Reference:     generateSecureReference("SUB"),
		PaymobOrderID: orderID,
	}
	if err := SafeCreate(db.DB, &payment); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to save payment record")
		return
	}

	if appliedCoupon != nil {
		db.DB.Model(appliedCoupon).UpdateColumn("used_count", appliedCoupon.UsedCount+1)
	}

	if req.PaymentMethod == "wallet" {
		walletUrl, _ := paymobSvc.CreateWalletRequest(paymentKey, billingData["phone_number"])
		api_response.Success(c, gin.H{"success": true, "paymentKey": paymentKey, "redirectUrl": walletUrl, "orderId": orderID})
		return
	}

	api_response.Success(c, gin.H{
		"success":    true,
		"paymentKey": paymentKey,
		"iframeId":   paymobSvc.IframeID,
		"orderId":    orderID,
	})
}

// resolveCheckoutPlan looks the requested plan up by id, then — if the
// caller asked for the other billing cycle — swaps it for the sibling plan
// sharing the same GroupKey with a matching Interval, when one exists.
// Falls back to the originally requested plan if no such sibling was set up
// by an admin, so a tier without a yearly variant still checks out fine.
func resolveCheckoutPlan(planID, billingCycle string) (models.SubscriptionPlan, error) {
	var plan models.SubscriptionPlan
	if err := db.DB.First(&plan, idQuery, planID).Error; err != nil {
		return plan, err
	}

	wantInterval := models.SubscriptionInterval("")
	switch billingCycle {
	case "yearly":
		wantInterval = models.IntervalYearly
	case "monthly":
		wantInterval = models.IntervalMonthly
	}
	if wantInterval == "" || plan.Interval == wantInterval {
		return plan, nil
	}

	var sibling models.SubscriptionPlan
	if err := db.DB.Where("group_key = ? AND interval = ? AND "+isActiveQuery, plan.GroupKey, wantInterval, true).
		First(&sibling).Error; err != nil {
		return plan, nil
	}
	return sibling, nil
}

// checkoutPurchasePlanWithWallet handles the internal_wallet payment method:
// deduct the platform balance immediately and activate the subscription,
// mirroring PurchasePlan's transaction but reusing the coupon already
// applied by the caller (avoids redeeming the same coupon twice).
func checkoutPurchasePlanWithWallet(c *gin.Context, userId string, plan models.SubscriptionPlan, finalPrice float64, appliedCoupon *models.Coupon) {
	paymentRef := generateSecureReference("SUB")

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Where(idQuery, userId).First(&user).Error; err != nil {
			return err
		}
		if err := subDeductUserBalance(tx, userId, &user, finalPrice, fmt.Sprintf("ترقية اشتراك: %s", plan.NameAr), paymentRef); err != nil {
			return err
		}
		if appliedCoupon != nil {
			tx.Model(appliedCoupon).UpdateColumn("used_count", appliedCoupon.UsedCount+1)
		}
		return createSubscriptionRecords(tx, userId, plan, finalPrice, paymentRef)
	})

	if err != nil {
		subHandlePurchaseError(c, err)
		return
	}
	api_response.Success(c, gin.H{"success": true})
}

func handleBookFileUpload(c *gin.Context, fieldName, prefix string) string {
	file, err := c.FormFile(fieldName)
	if err != nil {
		return ""
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	filename := fmt.Sprintf("%s_%d%s", prefix, time.Now().UnixNano(), ext)

	f, err := file.Open()
	if err != nil {
		return ""
	}
	defer f.Close()

	if storage.GlobalStorage == nil {
		return ""
	}

	url, err := storage.GlobalStorage.Upload(c.Request.Context(), filename, f, file.Size, file.Header.Get("Content-Type"))
	if err != nil {
		return ""
	}
	return url
}

func parseBookMultipartForm(c *gin.Context) models.Book {
	var book models.Book
	book.Title = c.PostForm("title")
	book.Author = c.PostForm("author")
	book.Description = c.PostForm("description")
	if subjectId := c.PostForm("subjectId"); subjectId != "" {
		book.SubjectID = &subjectId
	}
	price, _ := strconv.ParseFloat(c.PostForm("price"), 64)
	book.Price = price
	book.IsFree = c.PostForm("isFree") == "true"

	book.CoverUrl = handleBookFileUpload(c, "cover", "book_cover")
	book.DownloadUrl = handleBookFileUpload(c, "file", "book")

	return book
}

func CreateLibraryBook(c *gin.Context) {
	var book models.Book

	if strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data") {
		book = parseBookMultipartForm(c)
	} else if err := c.ShouldBindJSON(&book); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if book.Title == "" {
		api_response.Error(c, http.StatusBadRequest, "Book title is required")
		return
	}

	if err := SafeCreate(db.DB, &book); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create book record")
		return
	}

	api_response.Created(c, book)
}

func GetLibraryCategories(c *gin.Context) {
	var categories []models.Category
	if err := db.DB.Where("type = ?", "LIBRARY").Find(&categories).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch library categories")
		return
	}

	api_response.Success(c, categories)
}

func DeleteImpersonation(c *gin.Context) {
	middleware.ClearImpersonationCookies(c)
	api_response.Success(c, gin.H{
		"message": "تم إنهاء جلسة انتحال الشخصية والعودة لحسابك الأصلي",
	})
}
