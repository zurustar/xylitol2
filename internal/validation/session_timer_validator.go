package validation

import (
	"strconv"
	"strings"

	"github.com/zurustar/xylitol2/internal/parser"
)

// SessionTimerValidator validates Session-Timer requirements according to RFC4028
type SessionTimerValidator struct {
	minSE           int  // Minimum Session-Expires value
	defaultSE       int  // Default Session-Expires value
	requireSupport  bool // Whether Session-Timer support is mandatory
}

// NewSessionTimerValidator creates a new Session-Timer validator
func NewSessionTimerValidator(minSE, defaultSE int, requireSupport bool) RequestValidator {
	return &SessionTimerValidator{
		minSE:          minSE,
		defaultSE:      defaultSE,
		requireSupport: requireSupport,
	}
}

// Priority returns the priority of this validator (high priority, before authentication)
func (stv *SessionTimerValidator) Priority() int {
	return 10
}

// Name returns the name of this validator
func (stv *SessionTimerValidator) Name() string {
	return "SessionTimerValidator"
}

// AppliesTo returns true for INVITE requests
func (stv *SessionTimerValidator) AppliesTo(req *parser.SIPMessage) bool {
	return req.GetMethod() == "INVITE"
}

// Validate performs Session-Timer validation according to RFC4028
func (stv *SessionTimerValidator) Validate(req *parser.SIPMessage) ValidationResult {
	// Check if Session-Timer support is required
	if stv.requireSupport {
		// Check for Session-Expires header
		sessionExpires := req.GetHeader("Session-Expires")
		supported := req.GetHeader("Supported")
		
		// If no Session-Expires header and no timer support indicated
		if sessionExpires == "" && !containsTimer(supported) {
			return ValidationResult{
				Valid:       false,
				ErrorCode:   421,
				ErrorReason: "Extension Required",
				Details:     "Session-Timer support is required",
				ShouldStop:  true,
				Context: map[string]interface{}{
					"validator": "SessionTimerValidator",
					"error":     "session_timer_required",
					"required":  "timer",
				},
			}
		}
	}

	// If Session-Expires header is present, validate it
	sessionExpires := req.GetHeader("Session-Expires")
	if sessionExpires != "" {
		// Parse Session-Expires value
		se, refresher, err := parseSessionExpires(sessionExpires)
		if err != nil {
			return ValidationResult{
				Valid:       false,
				ErrorCode:   400,
				ErrorReason: "Bad Request",
				Details:     "Invalid Session-Expires header: " + err.Error(),
				ShouldStop:  true,
				Context: map[string]interface{}{
					"validator":       "SessionTimerValidator",
					"error":           "invalid_session_expires",
					"session_expires": sessionExpires,
				},
			}
		}

		// Check minimum Session-Expires value
		if se < stv.minSE {
			return ValidationResult{
				Valid:       false,
				ErrorCode:   422,
				ErrorReason: "Session Interval Too Small",
				Details:     "Session-Expires value is too small",
				ShouldStop:  true,
				Context: map[string]interface{}{
					"validator":       "SessionTimerValidator",
					"error":           "session_expires_too_small",
					"session_expires": se,
					"min_se":          stv.minSE,
				},
			}
		}

		// Validate refresher parameter if present
		if refresher != "" && refresher != "uac" && refresher != "uas" {
			return ValidationResult{
				Valid:       false,
				ErrorCode:   400,
				ErrorReason: "Bad Request",
				Details:     "Invalid refresher parameter in Session-Expires header",
				ShouldStop:  true,
				Context: map[string]interface{}{
					"validator": "SessionTimerValidator",
					"error":     "invalid_refresher",
					"refresher": refresher,
				},
			}
		}
	}

	// Check Min-SE header if present
	minSE := req.GetHeader("Min-SE")
	if minSE != "" {
		minSEValue, err := strconv.Atoi(strings.TrimSpace(minSE))
		if err != nil {
			return ValidationResult{
				Valid:       false,
				ErrorCode:   400,
				ErrorReason: "Bad Request",
				Details:     "Invalid Min-SE header",
				ShouldStop:  true,
				Context: map[string]interface{}{
					"validator": "SessionTimerValidator",
					"error":     "invalid_min_se",
					"min_se":    minSE,
				},
			}
		}

		// Min-SE should not be greater than Session-Expires
		if sessionExpires != "" {
			se, _, _ := parseSessionExpires(sessionExpires)
			if minSEValue > se {
				return ValidationResult{
					Valid:       false,
					ErrorCode:   400,
					ErrorReason: "Bad Request",
					Details:     "Min-SE value is greater than Session-Expires",
					ShouldStop:  true,
					Context: map[string]interface{}{
						"validator":       "SessionTimerValidator",
						"error":           "min_se_greater_than_se",
						"min_se":          minSEValue,
						"session_expires": se,
					},
				}
			}
		}
	}

	return ValidationResult{
		Valid: true,
		Context: map[string]interface{}{
			"validator": "SessionTimerValidator",
		},
	}
}

// containsTimer checks if the Supported header contains "timer"
func containsTimer(supported string) bool {
	if supported == "" {
		return false
	}
	
	// Split by comma and check each token
	tokens := strings.Split(supported, ",")
	for _, token := range tokens {
		if strings.TrimSpace(token) == "timer" {
			return true
		}
	}
	return false
}

// parseSessionExpires parses the Session-Expires header value
// Returns: (expires_value, refresher_param, error)
func parseSessionExpires(header string) (int, string, error) {
	// Split by semicolon to separate value from parameters
	parts := strings.Split(header, ";")
	if len(parts) == 0 {
		return 0, "", ValidationError{Details: "empty Session-Expires header"}
	}

	// Parse the expires value
	expiresStr := strings.TrimSpace(parts[0])
	expires, err := strconv.Atoi(expiresStr)
	if err != nil {
		return 0, "", ValidationError{Details: "invalid expires value"}
	}

	// Parse parameters
	var refresher string
	for i := 1; i < len(parts); i++ {
		param := strings.TrimSpace(parts[i])
		if strings.HasPrefix(param, "refresher=") {
			refresher = strings.TrimPrefix(param, "refresher=")
		}
	}

	return expires, refresher, nil
}