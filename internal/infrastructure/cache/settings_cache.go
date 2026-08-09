package cache

import (
	"thanawy-backend/internal/infrastructure/cache/settings"
)

// Compatibility facade over the cache/settings package.
// New code should import cache/settings directly.

// InvalidateSettingsCache drops the cached admin settings so the next read
// reloads them from the database.
func InvalidateSettingsCache() { settings.Invalidate() }

// CachedSettings returns the cached admin settings and whether they are still fresh.
func CachedSettings() (map[string]interface{}, bool) { return settings.Get() }

// StoreSettingsCache caches the given admin settings.
func StoreSettingsCache(s map[string]interface{}) { settings.Store(s) }
