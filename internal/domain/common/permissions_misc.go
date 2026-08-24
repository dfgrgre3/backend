package models

// ── Support ─────────────────────────────────
const (
	PermTicketsView    = "tickets:view"
	PermTicketsCreate  = "tickets:create"
	PermTicketsUpdate  = "tickets:update"
	PermTicketsManage  = "tickets:manage" // Assign, escalate, close
	PermTicketsResolve = "tickets:resolve"
	PermFAQsManage     = "faqs:manage"
)

// ── Parent Dashboard ─────────────────────────
const (
	PermChildrenView        = "children:view"
	PermChildrenGrades      = "children:grades"
	PermChildrenProgress    = "children:progress"
	PermChildrenAttendance  = "children:attendance"
	PermChildrenCommunicate = "children:communicate"
	PermChildrenPayment     = "children:payment"
)

// ── Misc ────────────────────────────────────
const (
	// PermPermissionsCustom is a sentinel that disables role-default merging.
	// It is never matched by any permission check.
	PermPermissionsCustom   = "permissions:custom"
	PermAchievementsView    = "achievements:view"
	PermAchievementsManage  = "achievements:manage"
	PermRewardsView         = "rewards:view"
	PermRewardsManage       = "rewards:manage"
	PermAiManage            = "ai:manage"
	PermAiUsage             = "ai:usage"
	PermLiveMonitorView     = "live_monitor:view"
	PermMarketingView       = "marketing:view"
	PermMarketingManage     = "marketing:manage"
	PermAbTestingView       = "ab_testing:view"
	PermSettingsView        = "settings:view"
	PermSeasonsView         = "seasons:view"
	PermSeasonsManage       = "seasons:manage"
	PermNotificationsSend   = "notifications:send"
	PermNotificationsManage = "notifications:manage"
)
