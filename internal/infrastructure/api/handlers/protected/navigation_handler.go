package protected

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	apiresponse "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"

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
	MenuKey       string               `json:"menuKey,omitempty"`
	ColumnKey     string               `json:"columnKey,omitempty"`
	Items         []NavigationMenuItem `json:"items"`
	IsPriority    bool                 `json:"isPriority,omitempty"`
	PriorityLabel string               `json:"priorityLabel,omitempty"`
}

// NavigationMenuEntry represents a top-level header menu backed by the API.
type NavigationMenuEntry struct {
	Key         string `json:"key"`
	Href        string `json:"href"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Badge       string `json:"badge,omitempty"`
	Order       int    `json:"order"`
}

// NavigationMenu represents the full mega menu structure
type NavigationMenu struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Menus         []NavigationMenuEntry `json:"menus"`
	Categories    []NavigationCategory  `json:"categories"`
	UpdatedAt     time.Time             `json:"updatedAt"`
}

const (
	navigationSchemaVersion = 1
	navigationCacheTTL      = 5 * time.Minute
)

// GetNavigationMenu returns the full mega menu structure for the frontend
func GetNavigationMenu(c *gin.Context) {
	cacheKey := "navigation:mega_menu:v8"

	if cache.Redis != nil {
		cached, err := cache.Redis.Get(c.Request.Context(), cacheKey).Result()
		if err == nil {
			var menu NavigationMenu
			if json.Unmarshal([]byte(cached), &menu) == nil && menu.SchemaVersion == navigationSchemaVersion {
				apiresponse.Success(c, repairNavigationMenu(menu))
				return
			}
		}
	}

	menu := repairNavigationMenu(buildNavigationMenu())

	if cache.Redis != nil {
		if data, err := json.Marshal(menu); err == nil {
			cache.Redis.Set(c.Request.Context(), cacheKey, data, navigationCacheTTL)
		}
	}

	apiresponse.Success(c, menu)
}

// repairNavigationMenu protects the API boundary from legacy menu literals
// that were stored after being decoded as Latin-1 instead of UTF-8.
func repairNavigationMenu(menu NavigationMenu) NavigationMenu {
	for i := range menu.Menus {
		menu.Menus[i].Label = repairUTF8Text(menu.Menus[i].Label)
		menu.Menus[i].Description = repairUTF8Text(menu.Menus[i].Description)
		menu.Menus[i].Badge = repairUTF8Text(menu.Menus[i].Badge)
	}
	for i := range menu.Categories {
		menu.Categories[i].Title = repairUTF8Text(menu.Categories[i].Title)
		menu.Categories[i].PriorityLabel = repairUTF8Text(menu.Categories[i].PriorityLabel)
		for j := range menu.Categories[i].Items {
			menu.Categories[i].Items[j].Label = repairUTF8Text(menu.Categories[i].Items[j].Label)
			menu.Categories[i].Items[j].Description = repairUTF8Text(menu.Categories[i].Items[j].Description)
			menu.Categories[i].Items[j].Badge = repairUTF8Text(menu.Categories[i].Items[j].Badge)
		}
	}
	return menu
}

func repairUTF8Text(value string) string {
	for attempt := 0; attempt < 3 && strings.ContainsAny(value, "ÃÂâØÙ"); attempt++ {
		bytes := make([]byte, 0, len(value))
		validCodePage := true
		for _, character := range value {
			byteValue, ok := windows1252Byte(character)
			if !ok {
				validCodePage = false
				break
			}
			bytes = append(bytes, byteValue)
		}
		if !validCodePage || !utf8.Valid(bytes) {
			break
		}
		candidate := string(bytes)
		if candidate == value {
			break
		}
		value = candidate
	}
	return value
}

func windows1252Byte(character rune) (byte, bool) {
	if character <= 0xff {
		return byte(character), true
	}
	byteValue, ok := map[rune]byte{
		0x0152: 0x8c, 0x0153: 0x9c, 0x0160: 0x8a, 0x0161: 0x9a,
		0x0178: 0x9f, 0x017d: 0x8e, 0x017e: 0x9e, 0x0192: 0x83,
		0x02c6: 0x88, 0x02dc: 0x98, 0x2013: 0x96, 0x2014: 0x97,
		0x2018: 0x91, 0x2019: 0x92, 0x201a: 0x82, 0x201c: 0x93,
		0x201d: 0x94, 0x201e: 0x84, 0x2020: 0x86, 0x2021: 0x87,
		0x2022: 0x95, 0x2026: 0x85, 0x2030: 0x89, 0x2039: 0x8b,
		0x203a: 0x9b, 0x20ac: 0x80, 0x2122: 0x99,
	}[character]
	return byteValue, ok
}

func buildNavigationMenu() NavigationMenu {
	categories := []NavigationCategory{
		// الدورات - Courses
		{
			Title: "الدراسة والتعلم",
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
				{Href: "/ai", Label: "التخطيط والأهداف", Description: "خطط دراستك ونظّم أهدافك التعليمية", Icon: "target"},
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
				{Href: "/blog", Label: "المدونة التعليمية", Description: "مقالات ومشاركات تثقيفية من المعلمين والطلاب", Icon: "file-text"},
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

	for i := range categories {
		categories[i].MenuKey = navigationMenuKey(categories[i].Slug)
		categories[i].ColumnKey = navigationColumnKey(categories[i].Slug)
	}

	return NavigationMenu{
		SchemaVersion: navigationSchemaVersion,
Menus: []NavigationMenuEntry{
	{Key: "home", Href: "/", Label: "الرئيسية", Description: "العودة إلى الصفحة الرئيسية", Icon: "home", Order: 0},
	{Key: "all-features", Href: "/all-features", Label: "المزيد", Description: "المزيد من الخيارات والأدوات", Icon: "sparkles", Order: 10},
	{Key: "schools", Href: "/schools", Label: "مدارس", Description: "المراحل التعليمية", Icon: "graduation-cap", Order: 20},
},
		Categories: categories,
		UpdatedAt:  time.Now(),
	}
}

func navigationColumnKey(slug string) string {
	switch slug {
	case "study", "exams":
		return "study"
	case "time_management", "goals":
		return "planning"
	case "digital_library", "awareness":
		return "content"
	case "dashboard", "leaderboard":
		return "performance"
	case "community":
		return "community"
	case "subscription", "settings":
		return "account"
	case "primary", "middle", "high_school":
		return "schools"
	default:
		return slug
	}
}

func navigationMenuKey(slug string) string {
	switch slug {
	case "primary", "middle", "high_school":
		return "schools"
	default:
		return "all-features"
	}
}

// GetMainNavItems returns the main navigation items with mega menu references
func GetMainNavItems(c *gin.Context) {
	cacheKey := "navigation:main_nav:v3"

	if cache.Redis != nil {
		cached, err := cache.Redis.Get(c.Request.Context(), cacheKey).Result()
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

	if cache.Redis != nil {
		if data, err := json.Marshal(navItems); err == nil {
			cache.Redis.Set(c.Request.Context(), cacheKey, data, 24*time.Hour)
		}
	}

	apiresponse.Success(c, navItems)
}

// InvalidateNavigationCache invalidates the navigation cache when menu structure changes
func InvalidateNavigationCache(c *gin.Context) {
	cache.NewCacheInvalidator().InvalidateNavigation(c.Request.Context())
	apiresponse.Success(c, nil)
}
