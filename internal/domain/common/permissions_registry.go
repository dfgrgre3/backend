package models

// ─────────────────────────────────────────────
//  All Permission Keys (for seeding/migration)
// ─────────────────────────────────────────────

// AllPermissions returns every defined permission constant.
func AllPermissions() []string {
	return append(basePermissions(), DashboardWidgetPermissions()...)
}

func basePermissions() []string {
	return []string{
		PermAdminBypass, PermSystemManage, PermSystemSettings,
		PermDashboardView, PermAnalyticsView, PermReportsView, PermReportsManage, PermAuditLogsView,
		PermDashboardManage,
		PermUsersView, PermUsersCreate, PermUsersUpdate, PermUsersDelete, PermUsersManage, PermUsersImpersonate, PermUsersExport, PermUsersImport,
		PermStudentsView, PermStudentsManage,
		PermTeachersView, PermTeachersManage,
		PermParentsView, PermParentsManage,
		PermSubjectsView, PermSubjectsCreate, PermSubjectsUpdate, PermSubjectsDelete, PermSubjectsManage, PermSubjectsPublish, PermSubjectsApprove, PermOwnSubjectsManage,
		PermBooksView, PermBooksCreate, PermBooksUpdate, PermBooksDelete, PermBooksManage, PermBooksPublish, PermOwnBooksManage,
		PermResourcesView, PermResourcesManage, PermResourcesPublish, PermOwnResourcesManage,
		PermExamsView, PermExamsCreate, PermExamsUpdate, PermExamsDelete, PermExamsManage, PermExamsApprove, PermExamsPublish, PermOwnExamsManage,
		PermChallengesView, PermChallengesManage, PermOwnChallengesManage,
		PermContestsView, PermContestsManage,
		PermBlogView, PermBlogCreate, PermBlogUpdate, PermBlogDelete, PermBlogManage, PermBlogPublish,
		PermForumView, PermForumCreate, PermForumUpdate, PermForumDelete, PermForumModerate, PermForumManage,
		PermCommentsView, PermCommentsCreate, PermCommentsModerate,
		PermEventsView, PermEventsManage,
		PermAnnouncementsView, PermAnnouncementsManage,
		PermTicketsView, PermTicketsCreate, PermTicketsUpdate, PermTicketsManage, PermTicketsResolve, PermFAQsManage,
		PermChildrenView, PermChildrenGrades, PermChildrenProgress, PermChildrenAttendance, PermChildrenCommunicate, PermChildrenPayment,
		PermAchievementsView, PermAchievementsManage,
		PermRewardsView, PermRewardsManage,
		PermAiManage, PermAiUsage,
		PermLiveMonitorView, PermMarketingView, PermMarketingManage,
		PermAbTestingView, PermSettingsView,
		PermSeasonsView, PermSeasonsManage,
		PermNotificationsSend, PermNotificationsManage,
		PermUsersSendNotifications,
	}
}

// PermissionModules returns all unique permission modules.
func PermissionModules() []string {
	return []string{
		"admin", "system",
		"dashboard", "analytics", "reports", "audit_logs",
		"users", "students", "teachers", "parents",
		"subjects", "books", "resources",
		"exams", "challenges", "contests",
		"blog", "forum", "comments",
		"events", "announcements",
		"tickets", "faqs",
		"children",
		"achievements", "rewards",
		"ai", "live_monitor", "marketing", "ab_testing",
		"settings", "seasons",
		"notifications",
	}
}
