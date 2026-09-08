package api

import (
	admindelivery "thanawy-backend/internal/infrastructure/api/handlers/admin"
	handlers "thanawy-backend/internal/infrastructure/api/handlers/protected"

	"github.com/gin-gonic/gin"
)

// registerAdminLessonPaymentRoutes registers Lessons, Payments, Dunning,
// Exams, Refunds and Tax management routes.
func registerAdminLessonPaymentRoutes(admin *gin.RouterGroup) {
	// -------------------------------
	// Lessons Management
	// -------------------------------
	admin.GET("/lessons", admindelivery.AdminListLessons)
	admin.GET("/lessons/:id", admindelivery.AdminGetLesson)
	admin.POST("/lessons", admindelivery.AdminCreateLesson)
	admin.PATCH("/lessons/:id", admindelivery.AdminUpdateLesson)
	admin.DELETE("/lessons/:id", admindelivery.AdminDeleteLesson)

	// -------------------------------
	// Payments Management
	// -------------------------------
	admin.GET("/payments", handlers.GetAdminPayments)
	admin.POST("/payments/refund", handlers.AdminRefundPayment)
	admin.POST("/payments/refund/bulk", handlers.AdminBulkRefundPayments)

	// Invoices — subscription & billing invoices
	admin.GET("/invoices", handlers.GetAdminInvoices)
	admin.GET("/invoices/:id", handlers.GetAdminInvoice)

	// Wallets — platform-wide wallet balances & transactions
	admin.GET("/wallets", admindelivery.AdminListWallets)
	admin.GET("/wallet-transactions", admindelivery.AdminListWalletTransactions)

	// Installments — split-payment plans
	admin.GET("/installments", admindelivery.AdminListInstallments)
	admin.POST("/installments", admindelivery.AdminCreateInstallmentPlan)
	admin.POST("/installments/:id/mark-paid", admindelivery.AdminMarkInstallmentPaid)
	admin.POST("/installments/:id/cancel", admindelivery.AdminCancelInstallment)

	// Dunning — subscription payment failure tracking
	admin.GET("/dunning", admindelivery.AdminListDunning)

	// -------------------------------
	// Exams Management
	// -------------------------------
	admin.GET("/exams", handlers.GetExams)
	admin.POST("/exams", handlers.CreateExam)
	admin.PATCH("/exams", handlers.UpdateExam)
	admin.DELETE("/exams", handlers.DeleteExam)
	admin.GET("/exam-results", admindelivery.AdminListExamResults)

	// -------------------------------
	// Refunds Management
	// -------------------------------
	admin.GET("/refunds", admindelivery.AdminListRefunds)
	admin.GET("/refunds/:id", admindelivery.AdminGetRefund)
	admin.POST("/refunds/:id/approve", admindelivery.AdminApproveRefund)
	admin.POST("/refunds/:id/reject", admindelivery.AdminRejectRefund)
	admin.POST("/refunds/:id/process", admindelivery.AdminProcessRefund)

	// -------------------------------
	// Tax Management
	// -------------------------------
	admin.GET("/taxes", admindelivery.AdminListTaxRates)
	admin.GET("/taxes/:id", admindelivery.AdminGetTaxRate)
	admin.POST("/taxes", admindelivery.AdminCreateTaxRate)
	admin.PATCH("/taxes/:id", admindelivery.AdminUpdateTaxRate)
	admin.DELETE("/taxes/:id", admindelivery.AdminDeleteTaxRate)

	// -------------------------------
	// Subscription Plans Management
	// -------------------------------
	admin.GET("/plans", handlers.AdminListPlans)
	admin.GET("/plans/:id", handlers.AdminGetPlan)
	admin.POST("/plans", handlers.AdminCreatePlan)
	admin.PATCH("/plans/:id", handlers.AdminUpdatePlan)
	admin.DELETE("/plans/:id", handlers.AdminDeletePlan)
}
