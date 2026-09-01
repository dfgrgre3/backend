package models

// ─────────────────────────────────────────────
//  Default Permissions Per Role
// ─────────────────────────────────────────────

// GetDefaultPermissions returns the default permission set for a given role.
func GetDefaultPermissions(role UserRole) []string {
	switch role {
	case RoleSuperAdmin:
		return getSuperAdminPermissions()
	case RoleAdmin:
		return getAdminPermissions()
	case RoleModerator:
		return getModeratorPermissions()
	case RoleSupport:
		return getSupportPermissions()
	case RoleTeacher:
		return getTeacherPermissions()
	case RoleParent:
		return getParentPermissions()
	case RoleStudent:
		return getStudentPermissions()
	default:
		return []string{}
	}
}

func getSuperAdminPermissions() []string {
	return []string{PermAdminBypass}
}

func getAdminPermissions() []string {
	// NOTE: PermAdminBypass is intentionally NOT granted here. It is a wildcard
	// that satisfies every permission check (see PermissionGrantMatches), so it
	// must remain exclusive to RoleSuperAdmin. Granting it to the plain ADMIN
	// role made every admin a de-facto super-admin regardless of their actual
	// permission list — restricted/moderator-style admins were impossible.
	return append([]string{
		PermDashboardView, PermAnalyticsView, PermReportsView, PermReportsManage, PermAuditLogsView,
		PermUsersView, PermUsersCreate, PermUsersUpdate, PermUsersDelete, PermUsersManage, PermUsersImpersonate, PermUsersExport, PermUsersImport,
		PermStudentsView, PermStudentsManage,
		PermTeachersView, PermTeachersManage,
		PermParentsView, PermParentsManage,
		PermSubjectsView, PermSubjectsCreate, PermSubjectsUpdate, PermSubjectsDelete, PermSubjectsManage, PermSubjectsPublish, PermSubjectsApprove,
		PermBooksView, PermBooksCreate, PermBooksUpdate, PermBooksDelete, PermBooksManage, PermBooksPublish,
		PermResourcesView, PermResourcesManage, PermResourcesPublish,
		PermExamsView, PermExamsCreate, PermExamsUpdate, PermExamsDelete, PermExamsManage, PermExamsApprove, PermExamsPublish,
		PermChallengesView, PermChallengesManage,
		PermContestsView, PermContestsManage,
		PermBlogView, PermBlogCreate, PermBlogUpdate, PermBlogDelete, PermBlogManage, PermBlogPublish,
		PermForumView, PermForumCreate, PermForumUpdate, PermForumDelete, PermForumModerate, PermForumManage,
		PermCommentsView, PermCommentsCreate, PermCommentsModerate,
		PermEventsView, PermEventsManage,
		PermAnnouncementsView, PermAnnouncementsManage,
		PermTicketsView, PermTicketsCreate, PermTicketsUpdate, PermTicketsManage, PermTicketsResolve, PermFAQsManage,
		PermAchievementsView, PermAchievementsManage,
		PermRewardsView, PermRewardsManage,
		PermAiManage, PermAiUsage,
		PermLiveMonitorView, PermMarketingView, PermMarketingManage,
		PermAbTestingView, PermSettingsView,
		PermSeasonsView, PermSeasonsManage,
		PermSystemManage, PermSystemSettings,
		PermNotificationsSend, PermNotificationsManage,
	}, DashboardWidgetPermissions()...)
}

func getModeratorPermissions() []string {
	return []string{
		PermDashboardView, PermAnalyticsView, PermReportsView,
		// Operational dashboard visibility without financial or system internals.
		PermDashboardAccess, PermDashboardViewKPIs,
		PermDashboardViewLearningMetrics, PermDashboardViewContentMetrics,
		PermDashboardViewSupportMetrics, PermDashboardViewRecentActivity,
		PermDashboardViewPendingItems, PermDashboardViewAlerts,
		PermDashboardViewTopCourses, PermDashboardAcknowledgeAlerts,
		PermDashboardSaveFilters, PermDashboardDeleteSavedFilters, PermDashboardApplySavedFilters,
		PermUsersView, PermStudentsView, PermTeachersView, PermParentsView,
		PermSubjectsView, PermSubjectsApprove,
		PermBooksView, PermBooksPublish,
		PermResourcesView, PermResourcesPublish,
		PermExamsView, PermExamsApprove, PermExamsPublish,
		PermChallengesView,
		PermContestsView,
		PermBlogView, PermBlogCreate, PermBlogUpdate, PermBlogDelete, PermBlogPublish,
		PermForumView, PermForumCreate, PermForumUpdate, PermForumDelete, PermForumModerate,
		PermCommentsView, PermCommentsModerate,
		PermEventsView, PermEventsManage,
		PermAnnouncementsView, PermAnnouncementsManage,
		PermTicketsView, PermTicketsManage, PermTicketsResolve,
		PermAchievementsView,
		PermRewardsView,
		PermLiveMonitorView, PermMarketingView,
		PermSettingsView,
		PermNotificationsSend, PermNotificationsManage,
	}
}

func getSupportPermissions() []string {
	return []string{
		PermDashboardView,
		// Support sees the support queue widgets only.
		PermDashboardAccess, PermDashboardViewSupportMetrics,
		PermDashboardViewPendingItems, PermDashboardViewAlerts,
		PermDashboardAcknowledgeAlerts, PermDashboardApplySavedFilters,
		PermUsersView, PermStudentsView, PermTeachersView, PermParentsView,
		PermTicketsView, PermTicketsCreate, PermTicketsUpdate, PermTicketsManage, PermTicketsResolve,
		PermFAQsManage,
		PermForumView,
		PermCommentsView,
		PermAnnouncementsView,
		PermNotificationsSend,
		PermSettingsView,
	}
}

func getTeacherPermissions() []string {
	return []string{
		PermDashboardView, PermAnalyticsView,
		PermStudentsView,
		PermSubjectsView, PermOwnSubjectsManage,
		PermBooksView, PermOwnBooksManage,
		PermResourcesView, PermOwnResourcesManage,
		PermExamsView, PermOwnExamsManage,
		PermChallengesView, PermOwnChallengesManage,
		PermBlogView, PermBlogCreate, PermBlogUpdate,
		PermForumView, PermForumCreate,
		PermCommentsView, PermCommentsCreate,
		PermAchievementsView,
		PermRewardsView,
		PermAiUsage,
	}
}

func getParentPermissions() []string {
	return []string{
		PermDashboardView,
		PermChildrenView, PermChildrenGrades, PermChildrenProgress, PermChildrenAttendance, PermChildrenCommunicate, PermChildrenPayment,
		PermSubjectsView,
		PermBooksView,
		PermExamsView,
		PermBlogView,
		PermForumView,
		PermCommentsView,
		PermAchievementsView,
		PermNotificationsSend,
	}
}

func getStudentPermissions() []string {
	return []string{
		PermDashboardView, PermAnalyticsView,
		PermSubjectsView,
		PermBooksView,
		PermResourcesView,
		PermExamsView,
		PermChallengesView,
		PermBlogView,
		PermForumView, PermForumCreate,
		PermCommentsView, PermCommentsCreate,
		PermAchievementsView,
		PermRewardsView,
		PermAiUsage,
	}
}
