package authservice

import (
	"strings"
)

// Simple user agent parser for basic info
func parseUserAgent(ua string) (os string, browser string) {
	uaLower := strings.ToLower(ua)

	// Basic OS detection
	if strings.Contains(uaLower, "windows") {
		os = "Windows"
	} else if strings.Contains(uaLower, "mac os") || strings.Contains(uaLower, "macos") {
		os = "MacOS"
	} else if strings.Contains(uaLower, "linux") {
		os = "Linux"
	} else if strings.Contains(uaLower, "android") {
		os = "Android"
	} else if strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "ipad") {
		os = "iOS"
	} else {
		os = "Unknown OS"
	}

	// Basic Browser detection
	if strings.Contains(uaLower, "chrome") && !strings.Contains(uaLower, "edg") {
		browser = "Chrome"
	} else if strings.Contains(uaLower, "safari") && !strings.Contains(uaLower, "chrome") {
		browser = "Safari"
	} else if strings.Contains(uaLower, "firefox") {
		browser = "Firefox"
	} else if strings.Contains(uaLower, "edg") {
		browser = "Edge"
	} else {
		browser = "Unknown Browser"
	}

	return os, browser
}

// detectDeviceType detects device type from user agent string
func detectDeviceType(ua string) string {
	uaLower := strings.ToLower(ua)
	if strings.Contains(uaLower, "mobile") || strings.Contains(uaLower, "android") || strings.Contains(uaLower, "iphone") {
		return "mobile"
	}
	if strings.Contains(uaLower, "tablet") || strings.Contains(uaLower, "ipad") {
		return "tablet"
	}
	return "web"
}
