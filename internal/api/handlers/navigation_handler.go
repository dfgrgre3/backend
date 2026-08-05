package handlers

import (
	"encoding/json"
	"time"

	apiresponse "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/cache"
	"thanawy-backend/internal/db"

	"github.com/gin-gonic/gin"
)

// NavigationMenuItem represents a single item in the mega menu
type NavigationMenuItem struct {
	ID          string `json:"id"`
	Href        string `json:"href"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Badge       string `json:"badge,omitempty"`
}

// NavigationCategory represents a category section in the mega menu
type NavigationCategory struct {
	ID            string               `json:"id"`
	Title         string               `json:"title"`
	Slug          string               `json:"slug"`
	Items         []NavigationMenuItem `json:"items"`
	IsPriority    bool                 `json:"isPriority,omitempty"`
	PriorityLabel string               `json:"priorityLabel,omitempty"`
}

// NavigationMenu represents the full mega menu structure
type NavigationMenu struct {
	Categories []NavigationCategory `json:"categories"`
	UpdatedAt  time.Time            `json:"updatedAt"`
}

// GetNavigationMenu returns the full mega menu structure for the frontend
func GetNavigationMenu(c *gin.Context) {
	cacheKey := "navigation:mega_menu:v2"

	if db.Redis != nil {
		cached, err := db.Redis.Get(c.Request.Context(), cacheKey).Result()
		if err == nil {
			var menu NavigationMenu
			if json.Unmarshal([]byte(cached), &menu) == nil {
				apiresponse.Success(c, menu)
				return
			}
		}
	}

	menu := buildNavigationMenu()

	if db.Redis != nil {
		if data, err := json.Marshal(menu); err == nil {
			db.Redis.Set(c.Request.Context(), cacheKey, data, 24*time.Hour)
		}
	}

	apiresponse.Success(c, menu)
}

func buildNavigationMenu() NavigationMenu {
	categories := []NavigationCategory{
		// الدورات - Courses
		{
			Title: "الدراسة والتعليم",
			Slug:  "study",
			Items: []NavigationMenuItem{
				{Href: "/courses", Label: "جميع الدورات", Description: "استعرض كل الدورات التعليمية المتاحة", Icon: "book-open"},
				{Href: "/my-courses", Label: "دوراتي", Description: "الدورات والمسارات التي تتابعها حالياً", Icon: "book-marked"},
				{Href: "/teachers", Label: "المدرسون", Description: "تواصل مع نخبة من أفضل المدرسين", Icon: "graduation-cap"},
			},
		},
		{
			Title: "التقييمات والامتحانات",
			Slug:  "exams",
			Items: []NavigationMenuItem{
				{Href: "/exams", Label: "الامتحانات والتقييم", Description: "الاختبارات الدورية وقياس المستوى المباشر", Icon: "clipboard-list"},
				{Href: "/teacher-exams", Label: "اختبارات المدرسين", Description: "بنك أسئلة واختبارات خاصة بمدرسي المنصة", Icon: "file-text"},
			},
		},
		{
			Title: "تنظيم الوقت",
			Slug:  "time_management",
			Items: []NavigationMenuItem{
				{Href: "/schedule", Label: "جدول المحاضرات", Description: "جدول الحصص المباشرة والدروس الأسبوعية", Icon: "calendar"},
				{Href: "/time", Label: "إدارة الوقت", Description: "أدوات لتنظيم ساعات الاستذكار والتركيز", Icon: "clock"},
			},
		},
		{
			Title: "التخطيط والأهداف",
			Slug:  "goals",
			Items: []NavigationMenuItem{
				{Href: "/tasks", Label: "قائمة المهام", Description: "متابعة الواجبات والمهام الدراسية اليومية", Icon: "book-marked"},
				{Href: "/goals", Label: "تحديد الأهداف", Description: "وضع أهداف دراسية أسبوعية وشهرية ومتابعتها", Icon: "target"},
			},
		},
		// المكتبة - Library
		{
			Title: "المحتوى التعليمي",
			Slug:  "digital_library",
			Items: []NavigationMenuItem{
				{Href: "/library", Label: "المكتبة الرقمية", Description: "مستودع الكتب والملخصات والملفات التعليمية", Icon: "library"},
				{Href: "/resources", Label: "الموارد والتحميلات", Description: "مركز تحميل المستندات والمذكرات الدراسية", Icon: "folder-open"},
			},
		},
		{
			Title: "المحتوى التثقيفي",
			Slug:  "awareness",
			Items: []NavigationMenuItem{
				{Href: "/tips", Label: "نصائح يومية", Description: "نصائح وتوجيهات عملية للتفوق الدراسي", Icon: "lightbulb"},
			},
		},
		{
			Title: "لوحة التحكم والأداء",
			Slug:  "dashboard",
			Items: []NavigationMenuItem{
				{Href: "/analytics", Label: "لوحة تحليلات الأداء", Description: "تحليلات مفصلة لمستوى دراستك ونقاط قوتك", Icon: "bar-chart"},
				{Href: "/academy", Label: "الأكاديمية", Description: "نظرة عامة على الأداء الأكاديمي العام", Icon: "graduation-cap"},
			},
		},
		// التحديات - Competition
		{
			Title: "التنافس والترتيب",
			Slug:  "leaderboard",
			Items: []NavigationMenuItem{
				{Href: "/leaderboard", Label: "لوحة الصدارة", Description: "ترتيب الطلاب الأوائل والمنافسين على المنصة", Icon: "trophy"},
				{Href: "/contests/new", Label: "تحدي جديد", Description: "إنشاء مسابقة وتحدي دراسي جديد مع زملائك", Icon: "gamepad"},
				{Href: "/events", Label: "الأحداث والفعاليات", Description: "المشاركة في المسابقات والفعاليات الرسمية", Icon: "sparkles"},
			},
		},
		{
			Title: "التواصل والمشاركة",
			Slug:  "community",
			Items: []NavigationMenuItem{
				{Href: "/chat", Label: "الدردشة الجماعية", Description: "غرف دردشة حية لمناقشة الدروس مع زملائك", Icon: "users"},
				{Href: "/forum", Label: "منتدى النقاش", Description: "طرح الأسئلة ومشاركة الإجابات مع مجتمع الطلاب", Icon: "message-square"},
				{Href: "/blog", Label: "المدونة التعليمية", Description: "مقالات ومشاركات تثقيفية من المعلمين والطلاب", Icon: "file-text"},
				{Href: "/announcements", Label: "إعلانات المنصة", Description: "آخر الأخبار والتحديثات الرسمية الهامة", Icon: "megaphone"},
			},
		},
		// المدارس - Schools
		{
			Title: "المرحلة الابتدائية",
			Slug:  "primary",
			Items: []NavigationMenuItem{
				{Href: "/schools/primary/4", Label: "الصف الرابع الابتدائي", Description: "مناهج ومواد الصف الرابع الابتدائي", Icon: "graduation-cap"},
				{Href: "/schools/primary/5", Label: "الصف الخامس الابتدائي", Description: "مناهج ومواد الصف الخامس الابتدائي", Icon: "graduation-cap"},
				{Href: "/schools/primary/6", Label: "الصف السادس الابتدائي", Description: "مناهج ومواد الصف السادس الابتدائي", Icon: "graduation-cap"},
			},
		},
		{
			Title: "المرحلة الإعدادية",
			Slug:  "middle",
			Items: []NavigationMenuItem{
				{Href: "/schools/middle/1", Label: "الصف الأول الإعدادي", Description: "مناهج ومواد الصف الأول الإعدادي", Icon: "graduation-cap"},
				{Href: "/schools/middle/2", Label: "الصف الثاني الإعدادي", Description: "مناهج ومواد الصف الثاني الإعدادي", Icon: "graduation-cap"},
				{Href: "/schools/middle/3", Label: "الصف الثالث الإعدادي", Description: "مناهج ومواد الصف الثالث الإعدادي", Icon: "graduation-cap"},
			},
		},
		{
			Title: "المرحلة الثانوية",
			Slug:  "high_school",
			Items: []NavigationMenuItem{
				{Href: "/schools/secondary/1", Label: "الصف الأول الثانوي", Description: "مناهج ومواد الصف الأول الثانوي", Icon: "graduation-cap"},
				{Href: "/schools/secondary/2", Label: "الصف الثاني الثانوي", Description: "مناهج ومواد الصف الثاني الثانوي", Icon: "graduation-cap"},
				{Href: "/schools/secondary/3", Label: "الصف الثالث الثانوي", Description: "مناهج ومواد الصف الثالث الثانوي", Icon: "graduation-cap"},
			},
		},
		// المزيد - More
		{
			Title: "الحساب والاشتراك",
			Slug:  "subscription",
			Items: []NavigationMenuItem{
				{Href: "/subscription", Label: "الاشتراكات المتاحة", Description: "استعرض باقات الاشتراك وقم بالترقية", Icon: "credit-card"},
				{Href: "/billing", Label: "إدارة الفواتير", Description: "المدفوعات، الفواتير، وطرق الدفع المحفوظة", Icon: "credit-card"},
				{Href: "/billing/referrals", Label: "برنامج الإحالة", Description: "دعوة أصدقائك والحصول على مكافآت ونقاط مجانية", Icon: "user-plus"},
			},
		},
		{
			Title: "الإعدادات والأمان",
			Slug:  "settings",
			Items: []NavigationMenuItem{
				{Href: "/settings", Label: "الإعدادات العامة", Description: "تخصيص الملف الشخصي والمظهر والتفضيلات", Icon: "settings"},
				{Href: "/settings/privacy", Label: "الخصوصية والظهور", Description: "التحكم في بياناتك وظهورك لزملائك", Icon: "shield"},
				{Href: "/settings/security", Label: "الأمان والوصول", Description: "تغيير كلمة المرور وتفعيل حماية الحساب", Icon: "shield"},
				{Href: "/settings/security/logs", Label: "سجل النشاط", Description: "عرض تفاصيل وسجلات الدخول لحسابك", Icon: "history"},
				{Href: "/settings/devices", Label: "الأجهزة المتصلة", Description: "إدارة الأجهزة النشطة التي تستخدم حسابك", Icon: "activity"},
				{Href: "/settings/notifications", Label: "تفضيلات الإشعارات", Description: "تحديد كيفية ووقت تلقي التنبيهات", Icon: "bell"},
			},
		},
	}

	return NavigationMenu{
		Categories: categories,
		UpdatedAt:  time.Now(),
	}
}

// GetMainNavItems returns the main navigation items with mega menu references
func GetMainNavItems(c *gin.Context) {
	cacheKey := "navigation:main_nav:v2"

	if db.Redis != nil {
		cached, err := db.Redis.Get(c.Request.Context(), cacheKey).Result()
		if err == nil {
			var navItems []map[string]interface{}
			if json.Unmarshal([]byte(cached), &navItems) == nil {
				apiresponse.Success(c, navItems)
				return
			}
		}
	}

	navItems := []map[string]interface{}{
		{
			"href":        "/",
			"label":       "الرئيسية",
			"icon":        "home",
			"description": "العودة إلى الصفحة الرئيسية",
		},
		{
			"href":        "/courses",
			"label":       "الدورات",
			"icon":        "book-open",
			"description": "استكشف الدورات التعليمية",
			"badge":       "جديد",
			"megaMenuKey": "courses",
		},
		{
			"href":        "/library",
			"label":       "المكتبة",
			"icon":        "library",
			"description": "مصادر تعليمية متنوعة",
			"megaMenuKey": "library",
		},
		{
			"href":        "/ai",
			"label":       "الذكاء الاصطناعي",
			"icon":        "brain",
			"description": "تعلم أذكى مع AI",
			"badge":       "AI",
		},
		{
			"href":        "/leaderboard",
			"label":       "التحديات",
			"icon":        "gamepad",
			"description": "لوحة الترتيب والمنافسات",
			"megaMenuKey": "competition",
		},
		{
			"href":        "/settings",
			"label":       "المزيد",
			"icon":        "sparkles",
			"description": "المزيد من الخيارات والأدوات",
			"megaMenuKey": "more",
		},
	}

	if db.Redis != nil {
		if data, err := json.Marshal(navItems); err == nil {
			db.Redis.Set(c.Request.Context(), cacheKey, data, 24*time.Hour)
		}
	}

	apiresponse.Success(c, navItems)
}

// InvalidateNavigationCache invalidates the navigation cache when menu structure changes
func InvalidateNavigationCache(c *gin.Context) {
	cache.NewCacheInvalidator().InvalidateNavigation(c.Request.Context())
	apiresponse.Success(c, nil)
}
