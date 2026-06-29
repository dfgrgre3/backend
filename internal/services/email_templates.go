package services

import (
	"fmt"
)

// GetVerificationEmailTemplate returns HTML template for email verification OTP
func GetVerificationEmailTemplate(userName, code string) string {
	return fmt.Sprintf(`
		<div dir="rtl" style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; max-width: 600px; margin: 0 auto; padding: 30px; border: 1px solid #e2e8f0; border-radius: 12px; background-color: #ffffff; color: #1a202c; text-align: right;">
			<div style="text-align: center; margin-bottom: 20px;">
				<h2 style="color: #6366f1; margin: 0; font-size: 28px; font-weight: 800;">منصة Tolo التعليمية</h2>
			</div>
			<hr style="border: none; border-top: 1px solid #edf2f7; margin: 20px 0;">
			<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">مرحباً <strong>%s</strong>،</p>
			<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">شكراً لتسجيلك في منصة Tolo. يرجى استخدام رمز التحقق التالي لتأكيد بريدك الإلكتروني وتفعيل حسابك:</p>
			<div style="text-align: center; margin: 30px 0;">
				<span style="display: inline-block; font-size: 32px; font-weight: 800; letter-spacing: 4px; color: #6366f1; background-color: #f0fdf4; border: 2px dashed #86efac; padding: 12px 30px; border-radius: 8px; font-family: monospace;">%s</span>
			</div>
			<p style="font-size: 14px; color: #718096; line-height: 1.5;">هذا الرمز صالح لمدة 15 دقيقة فقط. إذا لم تقم بإنشاء هذا الحساب، يرجى تجاهل هذا البريد الإلكتروني.</p>
			<hr style="border: none; border-top: 1px solid #edf2f7; margin: 25px 0;">
			<p style="color: #a0aec0; font-size: 12px; text-align: center; margin: 0;">هذه رسالة تلقائية، يرجى عدم الرد عليها. &copy; 2026 Tolo Academy.</p>
		</div>
	`, userName, code)
}

// GetWelcomeEmailTemplate returns HTML template for welcome email
func GetWelcomeEmailTemplate(userName, role string) string {
	roleName := "طالب"
	if role == "TEACHER" {
		roleName = "معلم"
	} else if role == "PARENT" {
		roleName = "ولي أمر"
	}

	return fmt.Sprintf(`
		<div dir="rtl" style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; max-width: 600px; margin: 0 auto; padding: 30px; border: 1px solid #e2e8f0; border-radius: 12px; background-color: #ffffff; color: #1a202c; text-align: right;">
			<div style="text-align: center; margin-bottom: 20px;">
				<h2 style="color: #6366f1; margin: 0; font-size: 28px; font-weight: 800;">مرحباً بك في Tolo!</h2>
			</div>
			<hr style="border: none; border-top: 1px solid #edf2f7; margin: 20px 0;">
			<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">مرحباً <strong>%s</strong>،</p>
			<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">يسعدنا انضمامك إلينا كـ <strong>%s</strong> في منصتنا التعليمية الكبرى. هدفنا هو تقديم بيئة تعليمية ذكية، تفاعلية وآمنة بالكامل لمساعدتك على النجاح والتفوق.</p>
			<div style="text-align: center; margin: 30px 0;">
				<a href="https://tolo-academy.com/dashboard" style="display: inline-block; padding: 12px 30px; background-color: #6366f1; color: #ffffff; text-decoration: none; font-weight: bold; border-radius: 8px; font-size: 16px; box-shadow: 0 4px 6px -1px rgba(99, 102, 241, 0.4);">الذهاب إلى لوحة التحكم</a>
			</div>
			<p style="font-size: 14px; color: #718096; line-height: 1.5;">إذا واجهتك أي مشكلة أو كان لديك أي استفسار، لا تتردد في الاتصال بفريق الدعم الفني في أي وقت.</p>
			<hr style="border: none; border-top: 1px solid #edf2f7; margin: 25px 0;">
			<p style="color: #a0aec0; font-size: 12px; text-align: center; margin: 0;">&copy; 2026 Tolo Academy. جميع الحقوق محفوظة.</p>
		</div>
	`, userName, roleName)
}

// GetPasswordResetEmailTemplate returns HTML template for password reset link
func GetPasswordResetEmailTemplate(userName, resetURL string) string {
	return fmt.Sprintf(`
		<div dir="rtl" style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; max-width: 600px; margin: 0 auto; padding: 30px; border: 1px solid #e2e8f0; border-radius: 12px; background-color: #ffffff; color: #1a202c; text-align: right;">
			<div style="text-align: center; margin-bottom: 20px;">
				<h2 style="color: #e11d48; margin: 0; font-size: 26px; font-weight: 800;">طلب إعادة تعيين كلمة المرور</h2>
			</div>
			<hr style="border: none; border-top: 1px solid #edf2f7; margin: 20px 0;">
			<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">مرحباً <strong>%s</strong>،</p>
			<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">لقد تلقينا طلباً لإعادة تعيين كلمة المرور الخاصة بحسابك على منصة Tolo. يرجى النقر على الزر أدناه لإتمام العملية:</p>
			<div style="text-align: center; margin: 30px 0;">
				<a href="%s" style="display: inline-block; padding: 12px 30px; background-color: #e11d48; color: #ffffff; text-decoration: none; font-weight: bold; border-radius: 8px; font-size: 16px; box-shadow: 0 4px 6px -1px rgba(225, 29, 72, 0.4);">إعادة تعيين كلمة المرور</a>
			</div>
			<p style="font-size: 14px; color: #718096; line-height: 1.5;">هذا الرابط متاح لمدة ساعة واحدة فقط. إذا لم تطلب تغيير كلمة المرور، يرجى تجاهل هذا البريد والاطمئنان بأن حسابك آمن.</p>
			<hr style="border: none; border-top: 1px solid #edf2f7; margin: 25px 0;">
			<p style="color: #a0aec0; font-size: 12px; text-align: center; margin: 0;">&copy; 2026 Tolo Academy. جميع الحقوق محفوظة.</p>
		</div>
	`, userName, resetURL)
}

// GetSecurityAlertEmailTemplate returns HTML template for security alerts
func GetSecurityAlertEmailTemplate(userName, action, details string) string {
	return fmt.Sprintf(`
		<div dir="rtl" style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; max-width: 600px; margin: 0 auto; padding: 30px; border: 1px solid #fca5a5; border-radius: 12px; background-color: #fff8f8; color: #1a202c; text-align: right;">
			<div style="text-align: center; margin-bottom: 20px;">
				<h2 style="color: #dc2626; margin: 0; font-size: 26px; font-weight: 800;">تنبيه أمني هام!</h2>
			</div>
			<hr style="border: none; border-top: 1px solid #fee2e2; margin: 20px 0;">
			<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">مرحباً <strong>%s</strong>،</p>
			<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">نود إخطارك بأنه تم إجراء تعديل أمني هام على حسابك:</p>
			<div style="background-color: #ffffff; border-right: 4px solid #dc2626; padding: 15px; margin: 20px 0; border-radius: 4px; box-shadow: 0 1px 3px rgba(0,0,0,0.05);">
				<p style="margin: 0 0 8px 0; font-size: 15px;"><strong>الحدث:</strong> %s</p>
				<p style="margin: 0; font-size: 14px; color: #718096;"><strong>التفاصيل:</strong> %s</p>
			</div>
			<p style="font-size: 14px; color: #4a5568; line-height: 1.5;">إذا كنت أنت من قام بهذا الإجراء، فلا داعي لاتخاذ أي خطوة. أما إذا لم تكن كذلك، فيرجى **تغيير كلمة المرور فوراً** والاتصال بمدير المنصة لتأمين حسابك.</p>
			<hr style="border: none; border-top: 1px solid #fee2e2; margin: 25px 0;">
			<p style="color: #a0aec0; font-size: 12px; text-align: center; margin: 0;">&copy; 2026 Tolo Academy. جميع الحقوق محفوظة.</p>
		</div>
	`, userName, action, details)
}

// GetLoginAlertEmailTemplate returns HTML template for new device login alerts
func GetLoginAlertEmailTemplate(userName, ip, device, location, timeStr string) string {
	return fmt.Sprintf(`
		<div dir="rtl" style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; max-width: 600px; margin: 0 auto; padding: 30px; border: 1px solid #e2e8f0; border-radius: 12px; background-color: #ffffff; color: #1a202c; text-align: right;">
			<div style="text-align: center; margin-bottom: 20px;">
				<h2 style="color: #4f46e5; margin: 0; font-size: 26px; font-weight: 800;">تسجيل دخول جديد لحسابك</h2>
			</div>
			<hr style="border: none; border-top: 1px solid #edf2f7; margin: 20px 0;">
			<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">مرحباً <strong>%s</strong>،</p>
			<p style="font-size: 16px; line-height: 1.6; color: #4a5568;">تم رصد تسجيل دخول جديد لحسابك على منصة Tolo بالبيانات التالية:</p>
			<div style="background-color: #f8fafc; border-right: 4px solid #4f46e5; padding: 15px; margin: 20px 0; border-radius: 4px;">
				<p style="margin: 0 0 8px 0; font-size: 14px; color: #334155;"><strong>الجهاز:</strong> %s</p>
				<p style="margin: 0 0 8px 0; font-size: 14px; color: #334155;"><strong>عنوان الـ IP:</strong> %s</p>
				<p style="margin: 0 0 8px 0; font-size: 14px; color: #334155;"><strong>الموقع الجغرافي:</strong> %s</p>
				<p style="margin: 0; font-size: 14px; color: #334155;"><strong>التوقيت:</strong> %s</p>
			</div>
			<p style="font-size: 14px; color: #4a5568; line-height: 1.5;">إذا كنت أنت من قام بالدخول، فالحالة طبيعية ولا تتطلب إجراءً. أما إذا لم يكن هذا جهازك، فيرجى الانتقال إلى صفحة الجلسات النشطة لإنهاء هذه الجلسة وتغيير كلمة مرورك فوراً.</p>
			<hr style="border: none; border-top: 1px solid #edf2f7; margin: 25px 0;">
			<p style="color: #a0aec0; font-size: 12px; text-align: center; margin: 0;">&copy; 2026 Tolo Academy. جميع الحقوق محفوظة.</p>
		</div>
	`, userName, device, ip, location, timeStr)
}
