# RBAC/ABAC System Documentation

## Overview

This document describes the comprehensive Role-Based Access Control (RBAC) and Attribute-Based Access Control (ABAC) system implemented in the Thanawy backend.

## Features

### 1. Role-Based Access Control (RBAC)

- **Custom Roles**: Create and manage custom roles with inheritance support
- **Hierarchical Roles**: Roles can have parent roles for permission inheritance
- **System Roles**: Pre-defined system roles (Super Admin, Admin, Moderator, Support, Teacher, Parent, Student)
- **Role Levels**: Numeric levels for role hierarchy (0-7)

### 2. Attribute-Based Access Control (ABAC)

- **Module-Level Permissions**: Granular permissions per module (Users, Courses, Exams, etc.)
- **Action-Level Permissions**: Specific actions (View, Create, Edit, Delete, Approve, Export)
- **Record-Level Permissions**: Access control at the individual record level
- **Conditional Permissions**: JSONB-based conditions for dynamic permission evaluation
- **Field-Level Permissions**: Control over which fields are visible/editable per role

### 3. Permission Management

- **Permission Inheritance**: Child roles inherit permissions from parent roles
- **Custom Roles**: Create roles with specific permission sets
- **Permission Conflicts**: Built-in conflict detection
- **Temporal Constraints**: Time-based role assignments (start/end dates)

### 4. Multi-Step Approval Workflows

- **Workflow Definitions**: Define approval workflows for specific module:action combinations
- **Multiple Steps**: Support for multi-step approval processes
- **Approver Assignment**: Assign approvers by role or specific user
- **Timeout Handling**: Automatic escalation and timeout management
- **Auto-Approval**: Conditional auto-approval based on payload conditions

### 5. Maker-Checker Pattern

- **Dual Control**: Separate maker (creator) and checker (approver) roles
- **Module-Specific**: Define maker-checker patterns per module
- **Conditional Activation**: Activate based on payload conditions
- **Timeout Management**: Automatic expiration and escalation

### 6. Audit Logging

- **Complete Audit Trail**: Log all permission changes
- **Change Tracking**: Track old and new values
- **User Attribution**: Record who made changes
- **IP/User Agent**: Capture request metadata
- **Searchable Logs**: Filter by entity type, change type, date range

## Database Schema

### Core Tables

1. **CustomRole**: Role definitions with inheritance support
2. **CustomPermission**: Granular permission definitions
3. **CustomRolePermission**: Role-permission mappings with ABAC conditions
4. **CustomRoleFieldOverride**: Field-level permission overrides
5. **CustomUserRoleAssignment**: User role assignments with temporal constraints

### Approval Workflow Tables

6. **ApprovalWorkflow**: Workflow definitions
7. **ApprovalStep**: Individual steps in workflows
8. **ApprovalRequest**: Pending approval requests
9. **ApprovalStepInstance**: Step-level approval tracking

### Maker-Checker Tables

10. **MakerCheckerPattern**: Maker-checker pattern definitions
11. **MakerCheckerRequest**: Maker-checker operation requests

### Audit Tables

12. **PermissionChangeLog**: Complete audit trail for permission changes

## API Endpoints

### Role Management

```
POST   /api/rbac/roles                    - Create a new role
GET    /api/rbac/roles                    - List all roles
GET    /api/rbac/roles/:roleId            - Get role details
PUT    /api/rbac/roles/:roleId            - Update a role
DELETE /api/rbac/roles/:roleId            - Delete a role
```

### Permission Management

```
POST   /api/rbac/roles/:roleId/permissions/:permissionId  - Grant permission
DELETE /api/rbac/roles/:roleId/permissions/:permissionId  - Revoke permission
```

### User Role Assignment

```
POST   /api/rbac/users/:userId/roles/:roleId              - Assign role to user
DELETE /api/rbac/users/:userId/roles/:roleId              - Revoke role from user
GET    /api/rbac/users/:userId/permissions                - Get user permissions
```

### Approval Workflows

```
POST   /api/rbac/approval-workflows                       - Create workflow
POST   /api/rbac/approval-workflows/:workflowId/requests  - Create approval request
POST   /api/rbac/approval-requests/:requestId/steps/:stepInstanceId/approve  - Approve
POST   /api/rbac/approval-requests/:requestId/steps/:stepInstanceId/reject   - Reject
```

### Maker-Checker

```
POST   /api/rbac/maker-checker-patterns                   - Create pattern
POST   /api/rbac/maker-checker-patterns/:patternId/requests - Create request
POST   /api/rbac/maker-checker-requests/:requestId/approve - Approve request
```

### Audit Logs

```
GET    /api/rbac/audit-logs                                - Get permission change logs
```

### Permission Check

```
GET    /api/rbac/check-permission?permission=users:view   - Check user permission
```

## Middleware

### HasPermission(permission)

Checks if the current user has a specific permission. Supports both legacy and new RBAC systems.

```go
router.GET("/api/users", middleware.HasPermission("users:view"), handlers.GetUsers)
```

### CanAccessResource(resourceType, resourceID)

Checks if the user can access a specific resource (ABAC).

```go
router.GET("/api/courses/:id", middleware.CanAccessResource("courses", ":id"), handlers.GetCourse)
```

### RequiresApproval(module, action)

Flags requests that require approval workflow.

```go
router.POST("/api/courses", middleware.RequiresApproval("courses", "create"), handlers.CreateCourse)
```

### RequiresMakerChecker(module, operation)

Flags operations that require maker-checker pattern.

```go
router.DELETE("/api/users/:id", middleware.RequiresMakerChecker("users", "delete"), handlers.DeleteUser)
```

### FilterSensitiveFields()

Removes sensitive fields from responses based on user role.

```go
router.GET("/api/users/:id", middleware.FilterSensitiveFields(), handlers.GetUser)
```

### AuditLog(action, resource)

Logs permission-related actions for audit trail.

```go
router.POST("/api/users", middleware.AuditLog("user.create", "users"), handlers.CreateUser)
```

## Usage Examples

### Creating a Custom Role

```bash
POST /api/rbac/roles
{
  "name": "Branch Manager",
  "nameAr": "مدير الفرع",
  "description": "Manages a specific branch",
  "level": 4
}
```

### Granting Permission with Conditions

```bash
POST /api/rbac/roles/{roleId}/permissions/{permissionId}
{
  "conditions": {
    "branchId": "123",
    "department": "sales"
  }
}
```

### Assigning Role with Temporal Constraints

```bash
POST /api/rbac/users/{userId}/roles/{roleId}
{
  "startsAt": "2024-01-01T00:00:00Z",
  "expiresAt": "2024-12-31T23:59:59Z",
  "resourceType": "branch",
  "resourceId": "123"
}
```

### Creating an Approval Workflow

```bash
POST /api/rbac/approval-workflows
{
  "name": "Course Publication Workflow",
  "triggerModule": "courses",
  "triggerAction": "publish",
  "minApprovals": 2,
  "steps": [
    {
      "stepOrder": 1,
      "stepName": "Department Head Approval",
      "approverRole": "department_head",
      "timeoutHours": 24
    },
    {
      "stepOrder": 2,
      "stepName": "Admin Approval",
      "approverRole": "admin",
      "timeoutHours": 48
    }
  ]
}
```

### Creating a Maker-Checker Pattern

```bash
POST /api/rbac/maker-checker-patterns
{
  "name": "User Deletion Pattern",
  "module": "users",
  "actions": ["delete"],
  "checkerRole": "admin",
  "checkTimeoutHours": 24,
  "condition": {
    "userRole": { "$ne": "admin" }
  }
}
```

## Permission Naming Convention

Permissions follow the pattern: `module:action`

### Common Permissions

- `users:view`, `users:create`, `users:update`, `users:delete`, `users:manage`
- `courses:view`, `courses:create`, `courses:update`, `courses:delete`, `courses:manage`
- `exams:view`, `exams:create`, `exams:update`, `exams:delete`, `exams:approve`
- `subjects:view`, `subjects:create`, `subjects:update`, `subjects:delete`
- `reports:view`, `reports:manage`
- `settings:view`, `settings:manage`
- `payments:view`, `payments:manage`, `payments:refund`
- `invoices:view`, `invoices:manage`
- `refunds:view`, `refunds:manage`
- `subscriptions:view`, `subscriptions:manage`
- `coupons:view`, `coupons:manage`
- `categories:view`, `categories:manage`
- `lessons:view`, `lessons:manage`
- `integrations:view`, `integrations:manage`

### Wildcard Permissions

- `*:manage` - Grants manage permission for all modules
- `module:*` - Grants all permissions for a specific module
- `own_module:manage` - Grants manage permission for user's own resources

## Field-Level Permissions

Field permissions control which fields are visible/editable per role:

```json
{
  "sensitive_fields": {
    "email": false,
    "phone": false,
    "salary": false
  },
  "visible_groups": {
    "personal_info": true,
    "academic_info": true,
    "financial_info": false
  }
}
```

## Best Practices

1. **Use System Roles**: Leverage pre-defined system roles for common use cases
2. **Custom Roles**: Create custom roles for specific organizational needs
3. **Permission Inheritance**: Use role hierarchy to avoid duplication
4. **ABAC Conditions**: Use conditions for fine-grained access control
5. **Audit Logging**: Regularly review audit logs for security compliance
6. **Approval Workflows**: Implement for sensitive operations
7. **Maker-Checker**: Use for critical operations requiring dual control
8. **Field-Level Security**: Apply to protect sensitive data

## Migration

The RBAC system is added via migration `0102_add_rbac_abac_system.sql`:

```bash
# Run migrations
go run cmd/migrate/main.go
```

## Security Considerations

1. **Principle of Least Privilege**: Grant minimum required permissions
2. **Regular Audits**: Review permissions and role assignments regularly
3. **Temporal Constraints**: Use time-based assignments for temporary access
4. **Conflict Detection**: Monitor for conflicting permissions
5. **Audit Trail**: Maintain complete audit logs for compliance
6. **Approval Workflows**: Require approval for sensitive operations
7. **Maker-Checker**: Implement dual control for critical actions

## Troubleshooting

### Permission Denied

1. Check user's active role assignments
2. Verify role has required permission
3. Check ABAC conditions (resource type, resource ID)
4. Verify temporal constraints (startsAt, expiresAt)
5. Check field-level permissions

### Approval Workflow Not Triggering

1. Verify workflow is active
2. Check module:action matches
3. Verify workflow has steps defined
4. Check auto-approve conditions

### Maker-Checker Not Activating

1. Verify pattern is active
2. Check module and action match
3. Verify condition evaluation
4. Check checker role assignment

## Support

For issues or questions, contact the development team or refer to the main project documentation.