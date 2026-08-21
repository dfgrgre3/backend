// Command seed-content populates the local development database with realistic
// Arabic content so the admin panel shows real rows instead of empty tables.
//
// It complements cmd/seed-admin (which only creates the SUPER_ADMIN account) and
// follows the same approach: raw SQL against the real column names, so it never
// depends on GORM AutoMigrate (SKIP_AUTO_MIGRATE=true in .env) and never drifts
// from whatever the versioned migrations actually created.
//
// Every row uses a fixed, deterministic UUID and ON CONFLICT DO NOTHING, so the
// command is idempotent — running it twice adds nothing and changes nothing.
//
// Usage (from d:/backend):
//
//	go run ./cmd/seed-content
package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------------
// Deterministic IDs. Fixed UUIDs keep the seed idempotent and make it possible
// to reference a row from a later INSERT without a round-trip.
// ---------------------------------------------------------------------------

func catID(n int) string  { return fmt.Sprintf("11111111-0000-4000-8000-%012d", n) }
func userID(n int) string { return fmt.Sprintf("22222222-0000-4000-8000-%012d", n) }
func crsID(n int) string  { return fmt.Sprintf("33333333-0000-4000-8000-%012d", n) }
func secID(n int) string  { return fmt.Sprintf("44444444-0000-4000-8000-%012d", n) }
func lsnID(n int) string  { return fmt.Sprintf("55555555-0000-4000-8000-%012d", n) }
func prcID(n int) string  { return fmt.Sprintf("66666666-0000-4000-8000-%012d", n) }
func enrID(n int) string  { return fmt.Sprintf("77777777-0000-4000-8000-%012d", n) }
func payID(n int) string  { return fmt.Sprintf("88888888-0000-4000-8000-%012d", n) }
func invID(n int) string  { return fmt.Sprintf("99999999-0000-4000-8000-%012d", n) }
func revID(n int) string  { return fmt.Sprintf("aaaaaaaa-0000-4000-8000-%012d", n) }

// ---------------------------------------------------------------------------
// Seed data
// ---------------------------------------------------------------------------

type category struct {
	name string
	slug string
}

var categories = []category{
	{"الرياضيات", "mathematics"},
	{"الفيزياء", "physics"},
	{"الكيمياء", "chemistry"},
	{"الأحياء", "biology"},
	{"اللغة العربية", "arabic"},
	{"اللغة الإنجليزية", "english"},
}

type person struct {
	name   string
	email  string
	role   string
	grade  string
	bio    string
	status string
}

// Teachers first (indices 1..4), then the moderator (5), then students (6..17).
var people = []person{
	{"أحمد محمود عبد الرحمن", "a.mahmoud@example.com", "TEACHER", "", "مدرس رياضيات — 12 سنة خبرة في الثانوية العامة", "ACTIVE"},
	{"منى سيد إبراهيم", "m.sayed@example.com", "TEACHER", "", "مدرسة فيزياء وحاصلة على ماجستير في المناهج", "ACTIVE"},
	{"خالد فتحي السيد", "k.fathy@example.com", "TEACHER", "", "مدرس كيمياء ومؤلف مراجعات نهائية", "ACTIVE"},
	{"هالة عبد العزيز", "h.abdelaziz@example.com", "TEACHER", "", "مدرسة لغة عربية — نحو وبلاغة", "ACTIVE"},

	{"يوسف كامل", "y.kamel@example.com", "MODERATOR", "", "مشرف محتوى ومراجعة الكورسات", "ACTIVE"},

	{"مريم حسن علي", "maryam.hassan@example.com", "STUDENT", "الثالث الثانوي", "", "ACTIVE"},
	{"عمر طارق زكي", "omar.tarek@example.com", "STUDENT", "الثالث الثانوي", "", "ACTIVE"},
	{"سلمى أشرف", "salma.ashraf@example.com", "STUDENT", "الثاني الثانوي", "", "ACTIVE"},
	{"محمد إبراهيم شعبان", "m.ibrahim@example.com", "STUDENT", "الثالث الثانوي", "", "ACTIVE"},
	{"نور الهدى مصطفى", "nour.mostafa@example.com", "STUDENT", "الأول الثانوي", "", "ACTIVE"},
	{"كريم وليد", "karim.walid@example.com", "STUDENT", "الثاني الثانوي", "", "ACTIVE"},
	{"فاطمة الزهراء سعيد", "fatma.saeed@example.com", "STUDENT", "الثالث الثانوي", "", "ACTIVE"},
	{"زياد أنور", "ziad.anwar@example.com", "STUDENT", "الثاني الثانوي", "", "INACTIVE"},
	{"جنى ماهر", "gana.maher@example.com", "STUDENT", "الأول الثانوي", "", "ACTIVE"},
	{"عبد الله رفعت", "a.refaat@example.com", "STUDENT", "الثالث الثانوي", "", "SUSPENDED"},
	{"ملك حسام", "malak.hossam@example.com", "STUDENT", "الثاني الثانوي", "", "ACTIVE"},
	{"طارق صلاح", "tarek.salah@example.com", "STUDENT", "الثالث الثانوي", "", "ACTIVE"},
}

const (
	firstTeacher = 1
	lastTeacher  = 4
	firstStudent = 6
	lastStudent  = 17
)

type course struct {
	title      string
	slug       string
	status     string
	level      string
	instructor int // index into people
	category   int // index into categories
	short      string
	mins       int
	price      float64
	featured   bool
	trending   bool
}

var courses = []course{
	{"التفاضل والتكامل — الثالث الثانوي", "calculus-3sec", "PUBLISHED", "ADVANCED", 1, 1,
		"شرح كامل لمنهج التفاضل والتكامل مع حل امتحانات السنوات السابقة.", 1440, 750, true, true},
	{"الجبر والهندسة الفراغية", "algebra-geometry", "PUBLISHED", "INTERMEDIATE", 1, 1,
		"أساسيات الجبر والهندسة الفراغية بأسلوب مبسط وتمارين متدرجة.", 960, 550, false, true},
	{"الفيزياء الحديثة والكهرباء", "modern-physics", "PUBLISHED", "ADVANCED", 2, 2,
		"الكهرباء التيارية والفيزياء الحديثة مع تجارب توضيحية بالفيديو.", 1200, 700, true, false},
	{"الديناميكا والحركة", "dynamics-motion", "UNDER_REVIEW", "INTERMEDIATE", 2, 2,
		"مراجعة شاملة على الديناميكا وقوانين الحركة.", 720, 450, false, false},
	{"الكيمياء العضوية من الصفر", "organic-chemistry", "PUBLISHED", "BEGINNER", 3, 3,
		"مقدمة كاملة في الكيمياء العضوية للمبتدئين مع خرائط ذهنية.", 840, 500, false, true},
	{"الكيمياء التحليلية — مراجعة نهائية", "analytical-chemistry", "DRAFT", "ADVANCED", 3, 3,
		"مراجعة نهائية مكثفة قبل الامتحان.", 480, 300, false, false},
	{"النحو والصرف — تأسيس", "arabic-grammar", "PUBLISHED", "BEGINNER", 4, 5,
		"تأسيس قوي في النحو والصرف مع تدريبات إعراب يومية.", 600, 350, false, false},
	{"البلاغة والأدب", "arabic-rhetoric", "ARCHIVED", "INTERMEDIATE", 4, 5,
		"شرح البلاغة والأدب مع نماذج تحليل نصوص.", 540, 0, false, false},
}

// sections[i] are the section titles used for every course, keeping the seed
// compact while still producing a realistic curriculum tree.
var sectionTitles = []string{
	"الوحدة الأولى — التأسيس",
	"الوحدة الثانية — الشرح التفصيلي",
	"الوحدة الثالثة — المراجعة وحل الامتحانات",
}

type lesson struct {
	title    string
	kind     string
	duration int
	free     bool
}

// Lesson types must come from common.LessonType (entity.go): VIDEO, TEXT, AUDIO,
// FILE, EXTERNAL_LINK, INTERACTIVE_QUIZ. There is no ARTICLE or QUIZ.
var lessonTemplates = []lesson{
	{"مقدمة الوحدة وأهدافها", "VIDEO", 480, true},
	{"الشرح النظري بالتفصيل", "VIDEO", 1560, false},
	{"ملخص مكتوب وخريطة ذهنية", "TEXT", 0, false},
	{"اختبار قصير على الوحدة", "INTERACTIVE_QUIZ", 0, false},
}

// devPassword is shared by every seeded account so the seeded teachers and
// students can actually be used to log in locally. seed-admin keeps its own,
// separate SUPER_ADMIN credential.
const devPassword = "Thanawy@2026"

var reviewComments = []string{
	"شرح واضح ومنظم، والتمارين ساعدتني كتير في المراجعة.",
	"أفضل كورس اتفرجت عليه في المادة — الأمثلة مترتبة بذكاء.",
	"المحتوى ممتاز بس نفسي في تدريبات أكتر على الأجزاء الصعبة.",
	"المدرس بيوصل الفكرة من أول مرة، وجودة الفيديو عالية.",
	"استفدت جداً من الملخصات المكتوبة قبل الامتحان.",
	"كورس محترم، محتاج بس تحديث لبعض الفيديوهات القديمة.",
}

// LmsReview.status is free text validated at the service layer as
// APPROVED | PENDING | REJECTED (see lms.go:477). Seeding all three gives the
// reviews-moderation page something to actually moderate.
var reviewStatuses = []string{"APPROVED", "APPROVED", "APPROVED", "PENDING", "APPROVED", "REJECTED"}

// ---------------------------------------------------------------------------

type seeder struct {
	db    *gorm.DB
	steps []string
	fail  int
}

// run executes one statement and records the outcome. A failure is logged and
// counted but does not abort the seed: one unexpected column should not block
// every remaining table.
func (s *seeder) run(label, sql string, args ...any) {
	if err := s.db.Exec(sql, args...).Error; err != nil {
		s.fail++
		log.Printf("  [FAIL] %-26s %v", label, err)
		return
	}
	s.steps = append(s.steps, label)
}

func main() {
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set (run from d:/backend so .env is picked up)")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	now := time.Now().UTC()
	s := &seeder{db: db}

	log.Println("→ seeding categories")
	for i, c := range categories {
		s.run("category "+c.slug, `
			INSERT INTO "LmsCategory" (id, name, slug, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
			catID(i+1), c.name, c.slug, now, now)
	}

	log.Println("→ seeding users (teachers, moderator, students)")
	// One bcrypt hash reused for every seeded account — hashing 17 times at cost
	// 12 would add several seconds for no benefit in a dev seed.
	hash, err := bcrypt.GenerateFromPassword([]byte(devPassword), 12)
	if err != nil {
		log.Fatalf("failed to hash the dev password: %v", err)
	}
	for i, p := range people {
		n := i + 1
		username := strings.Split(p.email, "@")[0]
		// Staggered creation dates make the "new users" charts non-flat.
		created := now.AddDate(0, 0, -(90 - n*3))
		s.run("user "+username, `
			INSERT INTO "User"
			  (id, email, name, username, role, status, email_verified, phone,
			   bio, grade_level, avatar, created_at, updated_at, balance, total_xp, level)
			VALUES (?, ?, ?, ?, ?::"UserRole", ?::"UserStatus", true, ?,
			        ?, ?, NULL, ?, ?, ?, ?, ?)
			ON CONFLICT DO NOTHING`,
			userID(n), p.email, p.name, username, p.role, p.status,
			fmt.Sprintf("+2010%08d", 10000000+n*137), p.bio, p.grade,
			created, created, 0, (n*137)%4000, 1+(n%9))

		// Passwords live in a separate table (see cmd/seed-admin), not on "User".
		s.run("credential "+username, `
			INSERT INTO "UserCredential" (user_id, password_hash, created_at, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (user_id) DO UPDATE
			  SET password_hash = EXCLUDED.password_hash, updated_at = EXCLUDED.updated_at`,
			userID(n), string(hash), created, created)
	}

	log.Println("→ seeding courses, curriculum, pricing")
	lessonCounter := 0
	for i, c := range courses {
		cid := crsID(i + 1)
		created := now.AddDate(0, 0, -(60 - i*5))

		s.run("course "+c.slug, `
			INSERT INTO "LmsCourse"
			  (id, title, slug, short_description, long_description, status, level, language,
			   estimated_duration_mins, has_certificate, version, is_featured, is_trending, is_new,
			   primary_instructor_id, seo_title, seo_description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, 'ar',
			        ?, true, 1, ?, ?, ?,
			        ?, ?, ?, ?, ?)
			ON CONFLICT DO NOTHING`,
			cid, c.title, c.slug, c.short, c.short+" الكورس يشمل شرحاً نظرياً وتمارين تفاعلية واختبارات دورية.",
			c.status, c.level, c.mins, c.featured, c.trending, i < 3,
			userID(c.instructor), c.title, c.short, created, created)

		s.run("course-category "+c.slug, `
			INSERT INTO "LmsCourseCategory" (course_id, category_id)
			VALUES (?, ?) ON CONFLICT DO NOTHING`,
			cid, catID(c.category))

		priceType := "PAID"
		if c.price == 0 {
			priceType = "FREE"
		}
		s.run("pricing "+c.slug, `
			INSERT INTO "LmsPricing"
			  (id, course_id, type, amount, currency_code, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, 'EGP', true, ?, ?) ON CONFLICT DO NOTHING`,
			prcID(i+1), cid, priceType, c.price, created, created)

		for j, title := range sectionTitles {
			sid := secID(i*10 + j + 1)
			s.run(fmt.Sprintf("section %s#%d", c.slug, j+1), `
				INSERT INTO "LmsSection" (id, course_id, title, order_index, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
				sid, cid, title, j+1, created, created)

			for k, l := range lessonTemplates {
				lessonCounter++
				s.run(fmt.Sprintf("lesson %s#%d.%d", c.slug, j+1, k+1), `
					INSERT INTO "LmsLesson"
					  (id, section_id, title, type, content, duration_seconds,
					   is_free_preview, order_index, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
					lsnID(lessonCounter), sid, l.title, l.kind,
					"محتوى الدرس: "+l.title+" — "+c.title,
					l.duration, l.free && j == 0, k+1, created, created)
			}
		}
	}

	log.Println("→ seeding enrollments, payments, invoices, reviews")
	// Enroll each student in 2 published courses; pay for one of them.
	publishedIdx := []int{1, 2, 3, 5, 7} // 1-based indices of PUBLISHED courses
	n := 0
	for st := firstStudent; st <= lastStudent; st++ {
		for offset := 0; offset < 2; offset++ {
			n++
			ci := publishedIdx[(st+offset)%len(publishedIdx)]
			cid := crsID(ci)
			enrolledAt := now.AddDate(0, 0, -(40 - n))
			progress := float64((st*17+offset*31)%101)

			s.run(fmt.Sprintf("enrollment #%d", n), `
				INSERT INTO "LmsEnrollment"
				  (id, course_id, user_id, progress, enrolled_at, completed_at, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
				enrID(n), cid, userID(st), progress, enrolledAt,
				nullIfNotComplete(progress, enrolledAt), enrolledAt, enrolledAt)

			// A review from any student who made real progress — deliberately
			// outside the payment branch below so reviews spread across every
			// published course, not just the ones that produced a payment.
			if progress >= 40 {
				s.run(fmt.Sprintf("review #%d", n), `
					INSERT INTO "LmsReview"
					  (id, course_id, user_id, rating, comment, status, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
					revID(n), cid, userID(st), 3+(st%3),
					reviewComments[n%len(reviewComments)],
					reviewStatuses[n%len(reviewStatuses)],
					enrolledAt.AddDate(0, 0, 7), enrolledAt.AddDate(0, 0, 7))
			}

			if offset != 0 {
				continue // only the first enrollment of each student produces a payment
			}

			amount := courses[ci-1].price
			if amount == 0 {
				continue
			}

			// Spread payment states so the finance pages have all three cases.
			// Values are lowercase on purpose: common.PaymentStatus is
			// pending|completed|failed|refunded|cancelled and the admin dashboard
			// queries them literally (admin_dashboard_v1_summary.go).
			status, completedAt := "completed", &enrolledAt
			switch st % 5 {
			case 3:
				status, completedAt = "pending", nil
			case 4:
				status, completedAt = "failed", nil
			}

			s.run(fmt.Sprintf("payment #%d", n), `
				INSERT INTO "Payment"
				  (id, user_id, amount, currency, status, method, reference,
				   completed_at, created_at, updated_at)
				VALUES (?, ?, ?, 'EGP', ?, 'PAYMOB_CARD', ?, ?, ?, ?)
				ON CONFLICT DO NOTHING`,
				payID(n), userID(st), amount, status,
				fmt.Sprintf("TXN-2026-%06d", 400000+n), completedAt, enrolledAt, enrolledAt)

			invStatus := "PAID"
			if status != "completed" {
				invStatus = "OPEN"
			}
			s.run(fmt.Sprintf("invoice #%d", n), `
				INSERT INTO "Invoice"
				  (id, invoice_number, user_id, payment_id, amount, currency,
				   status, due_date, items, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, 'EGP', ?::"InvoiceStatus", ?, ?, ?, ?)
				ON CONFLICT DO NOTHING`,
				invID(n), fmt.Sprintf("INV-2026-%05d", 1000+n), userID(st), payID(n),
				amount, invStatus, enrolledAt.AddDate(0, 0, 14),
				fmt.Sprintf(`[{"description":%q,"qty":1,"amount":%.2f}]`, courses[ci-1].title, amount),
				enrolledAt, enrolledAt)
		}
	}

	log.Printf("\n✔ seed finished — %d statements applied, %d failed", len(s.steps), s.fail)
	log.Printf("  seeded logins: %d teachers, 1 moderator, %d students — password for all: %s",
		lastTeacher-firstTeacher+1, lastStudent-firstStudent+1, devPassword)
	if s.fail > 0 {
		log.Println("  (failures above are usually a column that differs from this script's assumptions)")
		os.Exit(1)
	}
}

// nullIfNotComplete returns a completion timestamp only for finished enrollments,
// so "completed" counters in the admin panel are not universally true.
func nullIfNotComplete(progress float64, enrolledAt time.Time) any {
	if progress < 100 {
		return nil
	}
	t := enrolledAt.AddDate(0, 0, 21)
	return t
}
