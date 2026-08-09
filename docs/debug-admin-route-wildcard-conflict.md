# Debug Session: admin-route-wildcard-conflict

Status: [AWAITING USER CONFIRMATION]

## Symptom
Gin panics during router initialization because `/api/admin/courses/lessons/:id/interactive-questions` conflicts with an existing wildcard `:lessonId` under `/api/admin/courses/lessons/:lessonId`.

## Hypotheses
1. Two routes share the same prefix and differ only by wildcard name (`:lessonId` vs `:id`).
2. The `interactive-questions` route is registered under the wrong route group or URL shape.
3. The route is registered more than once in `SetupAdminRoutes`.
4. Registration order exposes a Gin route-tree conflict even though the handlers are otherwise valid.

## Evidence
- Runtime stack trace points to `internal/router/admin_routes.go:413` during `SetupAdminRoutes`.
- Gin reports the new path `/api/admin/courses/lessons/:id/interactive-questions` conflicts with existing prefix `/api/admin/courses/lessons/:lessonId`.
- Code inspection confirms the existing lesson-management routes use `:lessonId` at `internal/router/admin_routes.go:337-343`, while the interactive-question routes used `:id` at lines 413-414.
- After changing only those two route patterns to `:lessonId`, startup passed router construction and reached server binding. The remaining failure was environmental: ports `50051` and `8082` were already in use; the original Gin wildcard panic did not recur.

## Conclusion
- Hypothesis 1 confirmed: the conflict was caused by different wildcard names at the same route-tree position.
- Hypotheses 2 and 3 rejected by the route definitions inspected.
- Hypothesis 4 is a consequence of Gin's route-tree validation, not a separate handler issue.

## Changes
- Unified the interactive-question lesson route wildcard with the existing lesson routes: `:id` → `:lessonId`.
