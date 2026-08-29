package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders sets baseline security response headers on every request.
//
// This is a JSON API (no HTML/browser-rendered pages served from here), so
// the CSP/frame-ancestors values are deliberately locked down to "no active
// content, ever" rather than tuned for a page that needs scripts/styles —
// there is no legitimate reason for a browser to render this API's
// responses as a document or frame.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()

		// Prevent the browser from MIME-sniffing a response into executing
		// as something other than its declared Content-Type (relevant for
		// any endpoint that reflects/serves user-supplied content, e.g.
		// uploads or exports).
		h.Set("X-Content-Type-Options", "nosniff")

		// Swagger UI is the one route that actually is a browser-rendered
		// HTML page with its own scripts/styles — a locked-down CSP here
		// would break it. Everything else on this API is JSON-only.
		if !strings.HasPrefix(c.Request.URL.Path, "/swagger") {
			// This API is never meant to be framed by another site.
			h.Set("X-Frame-Options", "DENY")

			// Belt-and-suspenders CSP for the same "never render as a page"
			// intent as X-Frame-Options, expressed in the header browsers
			// actually prefer today.
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		}

		// Don't leak the full referring URL (which may contain tokens/IDs
		// in query strings) to third-party destinations.
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Opt this origin out of powerful browser features it never uses.
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		// Force HTTPS on every subsequent request. Safe to send unconditionally:
		// browsers ignore it entirely over a plain-HTTP connection, and in
		// production this API sits behind a TLS-terminating proxy/LB.
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")

		c.Next()
	}
}
