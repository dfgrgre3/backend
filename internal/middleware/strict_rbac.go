package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AnyAuthenticatedUser يسمح لأي مستخدم موثق بالمرور، ويضع علامة RBAC authorisation.
// يُستخدم في المسارات المحمية (protected routes) التي لا تتطلب دوراً محدداً
// ولكنها تحتاج لاجتياز فحص StrictRBAC.
func AnyAuthenticatedUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, roleExists := c.Get("role")
		if !roleExists || role == nil || role.(string) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Authentication required",
				"message": "هذا المسار يتطلب توثيق هوية",
			})
			return
		}
		MarkRBACAuthorized(c)
		c.Next()
	}
}

// StrictRBAC هو ميدلويار يطبق سياسة "Deny by Default" على جميع المسارات.
// لا يمكن لأي مسار أن يعمل بدون تحديد دور صريح له.
// يستخدم هذا الميدلويار في جميع مجموعات الراوترات (Router Groups) بحيث
// يمنع أي طلب لم يتم تعريف صلاحياته صراحةً.
func StrictRBAC() gin.HandlerFunc {
	return func(c *gin.Context) {
		// سياسة المنع الافتراضي: أي مسار لم يمر على middleware صلاحيات محدد
		// سيتم رفضه تلقائياً.
		// هذا الميدلويار يجب وضعه كآخر ميدلويار في السلسلة،
		// بحيث إذا لم يقم أي ميدلويار صلاحيات بالسماح للطلب (c.Next)
		// يتم رفض الطلب هنا.
		//
		// آلية العمل:
		// 1. نتحقق إذا كان هناك دور في الـ context (تم تعيينه بواسطة Auth())
		// 2. نتحقق إذا كان الطلب قد مر على أحد ميدلويارات الصلاحيات
		//    (AdminRequired, RoleRequired, PermissionRequired, إلخ)
		// 3. إذا لم يمر على أي ميدلويار صلاحيات → نرفض الطلب

		role, roleExists := c.Get("role")
		if !roleExists || role == nil || role.(string) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Authentication required",
				"message": "هذا المسار يتطلب توثيق هوية",
			})
			return
		}

		// تحقق إذا كان هناك علم يشير إلى أن الطلب قد مر على ميدلويار صلاحيات
		_, authzCheckPassed := c.Get("_rbac_authorized")
		if !authzCheckPassed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "Access denied: route not explicitly authorized",
				"message": "ليس لديك صلاحية للوصول إلى هذا المسار",
			})
			return
		}

		c.Next()
	}
}

// MarkRBACAuthorized يُستخدم بواسطة ميدلويارات الصلاحيات (AdminRequired, RoleRequired, PermissionRequired)
// لوضع علامة في الـ context تفيد بأن الطلب قد اجتاز فحص الصلاحيات.
func MarkRBACAuthorized(c *gin.Context) {
	c.Set("_rbac_authorized", true)
}