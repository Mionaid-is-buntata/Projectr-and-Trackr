package handlers

import "net/http"

// Handlers for this domain — see routes.go for endpoint definitions.
// Return HTML fragments for htmx requests (HX-Request header present),
// or JSON for programmatic API access.

var _ = http.MethodGet // placeholder import usage
