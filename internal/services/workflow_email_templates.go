package services

import "fmt"

// =============================================================
// Course Workflow Email Templates
// =============================================================

// GetCourseSubmittedForReviewEmailTemplate notifies admin when a course is submitted for review
func GetCourseSubmittedForReviewEmailTemplate(instructorName, courseName, reviewURL string) string {
	return fmt.Sprintf(`
	<div dir="rtl" style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; max-width: 600px; margin: 0 auto; padding: 30px; border: 1px solid #e2e8f0; border-radius: 12px; background-color: #ffffff; color: #1a202c; text-align: right;">
		<div style="text-align: center; margin-bottom: 20px;">
			<h2 style="color: #6366f1; margin: 0; font-size: 28px; font-weight: 800;">طلب مراجعة كورس جديد</h2>
		</div>
		<hr style="border: none; border-top: 1px solid #edf2f7; margin: 20px 0;">
		<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">مرحباً،</p>
		<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">
			تم إرسال <strong style="color: #1a202c;">"%s"</strong> بواسطة <strong style="color: #1a202c;">%s</strong> للمراجعة.
		</p>
		<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">
			يرجى مراجعة الكورس والموافقة عليه أو طلب تعديلات قبل نشره.
		</p>
		<div style="text-align: center; margin: 30px 0;">
			<a href="%s" style="display: inline-block; padding: 12px 30px; background-color: #6366f1; color: #ffffff; text-decoration: none; font-weight: bold; border-radius: 8px; font-size: 16px; box-shadow: 0 4px 6px -1px rgba(99, 102, 241, 0.4);">مراجعة الكورس</a>
		</div>
		<hr style="border: none; border-top: 1px solid #edf2f7; margin: 25px 0;">
		<p style="color: #a0aec0; font-size: 12px; text-align: center; margin: 0;">&copy; 2026 Tolo Academy</p>
	</div>
	`, courseName, instructorName, reviewURL)
}

// GetCourseApprovedEmailTemplate notifies instructor when their course is approved
func GetCourseApprovedEmailTemplate(instructorName, courseName string, courseURL string) string {
	return fmt.Sprintf(`
	<div dir="rtl" style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; max-width: 600px; margin: 0 auto; padding: 30px; border: 1px solid #86efac; border-radius: 12px; background-color: #f0fdf4; color: #1a202c; text-align: right;">
		<div style="text-align: center; margin-bottom: 20px;">
			<h2 style="color: #16a34a; margin: 0; font-size: 28px; font-weight: 800;">تم الموافقة على كورسك! 🎉</h2>
		</div>
		<hr style="border: none; border-top: 1px solid #bbf7d0; margin: 20px 0;">
		<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">مرحباً <strong style="color: #1a202c;">%s</strong>،</p>
		<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">
			عظيم! تم الموافقة على كورس <strong style="color: #1a202c;">"%s"</strong> ونشره بنجاح.
		</p>
		<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">
			الكورس الآن متاح للطلاب ويمكنهم التسجيل فيه.
		</p>
		<div style="text-align: center; margin: 30px 0;">
			<a href="%s" style="display: inline-block; padding: 12px 30px; background-color: #16a34a; color: #ffffff; text-decoration: none; font-weight: bold; border-radius: 8px; font-size: 16px;">عرض الكورس</a>
		</div>
		<hr style="border: none; border-top: 1px solid #bbf7d0; margin: 25px 0;">
		<p style="color: #a0aec0; font-size: 12px; text-align: center; margin: 0;">&copy; 2026 Tolo Academy</p>
	</div>
	`, instructorName, courseName, courseURL)
}

// GetCourseRejectedEmailTemplate notifies instructor when their course is rejected
func GetCourseRejectedEmailTemplate(instructorName, courseName, reason string, editURL string) string {
	return fmt.Sprintf(`
	<div dir="rtl" style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; max-width: 600px; margin: 0 auto; padding: 30px; border: 1px solid #fca5a5; border-radius: 12px; background-color: #fff8f8; color: #1a202c; text-align: right;">
		<div style="text-align: center; margin-bottom: 20px;">
			<h2 style="color: #dc2626; margin: 0; font-size: 28px; font-weight: 800;">تم طلب تعديلات على كورسك</h2>
		</div>
		<hr style="border: none; border-top: 1px solid #fecaca; margin: 20px 0;">
		<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">مرحباً <strong style="color: #1a202c;">%s</strong>،</p>
		<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">
			نأسف، لكننا نحتاج بعض التعديلات على كورس <strong style="color: #1a202c;">"%s"</strong> قبل نشره.
		</p>
		<div style="background-color: #fef2f2; border: 1px solid #fecaca; border-radius: 8px; padding: 16px; margin: 20px 0;">
			<p style="font-size: 14px; font-weight: bold; color: #dc2626; margin: 0 0 8px 0;">سبب الرفض:</p>
			<p style="font-size: 14px; color: #7f1d1d; margin: 0; line-height: 1.6;">%s</p>
		</div>
		<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">
			يرجى تعديل الكورس بناءً على الملاحظات وإعادة إرساله للمراجعة.
		</p>
		<div style="text-align: center; margin: 30px 0;">
			<a href="%s" style="display: inline-block; padding: 12px 30px; background-color: #dc2626; color: #ffffff; text-decoration: none; font-weight: bold; border-radius: 8px; font-size: 16px;">تعديل الكورس</a>
		</div>
		<hr style="border: none; border-top: 1px solid #fecaca; margin: 25px 0;">
		<p style="color: #a0aec0; font-size: 12px; text-align: center; margin: 0;">&copy; 2026 Tolo Academy</p>
	</div>
	`, instructorName, courseName, reason, editURL)
}

// GetCourseArchivedEmailTemplate notifies instructor when their course is archived
func GetCourseArchivedEmailTemplate(instructorName, courseName, reason string) string {
	return fmt.Sprintf(`
	<div dir="rtl" style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; max-width: 600px; margin: 0 auto; padding: 30px; border: 1px solid #fde68a; border-radius: 12px; background-color: #fefce8; color: #1a202c; text-align: right;">
		<div style="text-align: center; margin-bottom: 20px;">
			<h2 style="color: #ca8a04; margin: 0; font-size: 28px; font-weight: 800;">تم أرشفة الكورس</h2>
		</div>
		<hr style="border: none; border-top: 1px solid #fef08a; margin: 20px 0;">
		<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">مرحباً <strong style="color: #1a202c;">%s</strong>،</p>
		<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">
			تم أرشفة كورس <strong style="color: #1a202c;">"%s"</strong>不会再对学生显示.
		</p>
		%s
		<hr style="border: none; border-top: 1px solid #fef08a; margin: 25px 0;">
		<p style="color: #a0aec0; font-size: 12px; text-align: center; margin: 0;">&copy; 2026 Tolo Academy</p>
	</div>
	`, instructorName, courseName, func() string {
		if reason != "" {
			return fmt.Sprintf(`<p style="font-size: 14px; color: #713f12; background-color: #fef9c3; padding: 12px; border-radius: 8px;">%s</p>`, reason)
		}
		return ""
	}())
}

// GetDripContentReleasedEmailTemplate notifies students when new content is released
func GetDripContentReleasedEmailTemplate(studentName, courseName, lessonName string, lessonURL string) string {
	return fmt.Sprintf(`
	<div dir="rtl" style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; max-width: 600px; margin: 0 auto; padding: 30px; border: 1px solid #a5b4fc; border-radius: 12px; background-color: #eef2ff; color: #1a202c; text-align: right;">
		<div style="text-align: center; margin-bottom: 20px;">
			<h2 style="color: #4f46e5; margin: 0; font-size: 28px; font-weight: 800;">درس جديد متاح! 📚</h2>
		</div>
		<hr style="border: none; border-top: 1px solid #c7d2fe; margin: 20px 0;">
		<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">مرحباً <strong style="color: #1a202c;">%s</strong>،</p>
		<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">
			تم إضافة درس جديد في كورس <strong style="color: #1a202c;">"%s"</strong>:
		</p>
		<div style="background-color: #ffffff; border: 1px solid #c7d2fe; border-radius: 8px; padding: 16px; margin: 20px 0;">
			<p style="font-size: 18px; font-weight: bold; color: #4f46e5; margin: 0;">%s</p>
		</div>
		<div style="text-align: center; margin: 30px 0;">
			<a href="%s" style="display: inline-block; padding: 12px 30px; background-color: #4f46e5; color: #ffffff; text-decoration: none; font-weight: bold; border-radius: 8px; font-size: 16px;">مشاهدة الدرس</a>
		</div>
		<hr style="border: none; border-top: 1px solid #c7d2fe; margin: 25px 0;">
		<p style="color: #a0aec0; font-size: 12px; text-align: center; margin: 0;">&copy; 2026 Tolo Academy</p>
	</div>
	`, studentName, courseName, lessonName, lessonURL)
}

// GetBundlePurchasedEmailTemplate notifies student after purchasing a bundle
func GetBundlePurchasedEmailTemplate(studentName, bundleName string, courses []string, accessURL string) string {
	courseList := ""
	for _, course := range courses {
		courseList += fmt.Sprintf(`<li style="font-size: 14px; color: #4a5568; margin: 8px 0;">✓ %s</li>`, course)
	}

	return fmt.Sprintf(`
	<div dir="rtl" style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; max-width: 600px; margin: 0 auto; padding: 30px; border: 1px solid #86efac; border-radius: 12px; background-color: #f0fdf4; color: #1a202c; text-align: right;">
		<div style="text-align: center; margin-bottom: 20px;">
			<h2 style="color: #16a34a; margin: 0; font-size: 28px; font-weight: 800;">تم purchase الباقة بنجاح! 🎉</h2>
		</div>
		<hr style="border: none; border-top: 1px solid #bbf7d0; margin: 20px 0;">
		<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">مرحباً <strong style="color: #1a202c;">%s</strong>،</p>
		<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">
			تم إضافة <strong style="color: #1a202c;">"%s"</strong> إلى حسابك!
		</p>
		<p style="font-size: 14px; font-weight: bold; color: #4a5568; margin-top: 20px;">الكورسات المشمولة:</p>
		<ul style="list-style: none; padding: 0; margin: 0;">%s</ul>
		<div style="text-align: center; margin: 30px 0;">
			<a href="%s" style="display: inline-block; padding: 12px 30px; background-color: #16a34a; color: #ffffff; text-decoration: none; font-weight: bold; border-radius: 8px; font-size: 16px;">ابدأ التعلم الآن</a>
		</div>
		<hr style="border: none; border-top: 1px solid #bbf7d0; margin: 25px 0;">
		<p style="color: #a0aec0; font-size: 12px; text-align: center; margin: 0;">&copy; 2026 Tolo Academy</p>
	</div>
	`, studentName, bundleName, courseList, accessURL)
}

// =============================================================
// Workflow Notification Templates (In-App)
// =============================================================

// WorkflowNotificationTitle returns the notification title
func WorkflowNotificationTitle(event, courseName string) string {
	switch event {
	case "submitted":
		return "تم إرسال كورسك للمراجعة"
	case "approved":
		return "تم الموافقة على كورسك! 🎉"
	case "rejected":
		return "تم طلب تعديلات على كورسك"
	case "archived":
		return "تم أرشفة الكورس"
	case "drip_released":
		return "درس جديد متاح!"
	default:
		return "تحديث على كورس: " + courseName
	}
}

// WorkflowNotificationBody returns the notification body
func WorkflowNotificationBody(event, courseName, extra string) string {
	switch event {
	case "submitted":
		return "كورس \"" + courseName + "\" ينتظر مراجعة الإدارة"
	case "approved":
		return "كورس \"" + courseName + "\" تمت الموافقة عليه ونشره بنجاح"
	case "rejected":
		return "كورس \"" + courseName + "\" يحتاج تعديلات: " + extra
	case "archived":
		return "كورس \"" + courseName + "\" تم أرشفته不会再 يظهر للطلاب"
	case "drip_released":
		return "درس جديد \"" + extra + "\" متاح الآن في كورس \"" + courseName + "\""
	default:
		return extra
	}
}
