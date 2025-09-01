package validation

import (
	"strings"

	"github.com/zurustar/xylitol2/internal/parser"
)

// AuthValidator validates authentication requirements
type AuthValidator struct {
	requireAuth bool
	realm       string
}

// NewAuthValidator creates a new authentication validator
func NewAuthValidator(requireAuth bool, realm string) RequestValidator {
	return &AuthValidator{
		requireAuth: requireAuth,
		realm:       realm,
	}
}

// Priority returns the priority of this validator (lower priority, after Session-Timer)
func (av *AuthValidator) Priority() int {
	return 20
}

// Name returns the name of this validator
func (av *AuthValidator) Name() string {
	return "AuthValidator"
}

// AppliesTo returns true for requests that require authentication
func (av *AuthValidator) AppliesTo(req *parser.SIPMessage) bool {
	// Apply to REGISTER and INVITE requests when authentication is required
	method := req.GetMethod()
	return av.requireAuth && (method == "REGISTER" || method == "INVITE")
}

// Validate performs authentication validation
func (av *AuthValidator) Validate(req *parser.SIPMessage) ValidationResult {
	// Check for Authorization header
	authorization := req.GetHeader("Authorization")
	if authorization == "" {
		// No authorization header - challenge required
		return ValidationResult{
			Valid:       false,
			ErrorCode:   401,
			ErrorReason: "Unauthorized",
			Details:     "Authentication required",
			ShouldStop:  true,
			Context: map[string]interface{}{
				"validator": "AuthValidator",
				"error":     "authentication_required",
				"realm":     av.realm,
			},
		}
	}

	// Basic validation of Authorization header format
	if !strings.HasPrefix(authorization, "Digest ") {
		return ValidationResult{
			Valid:       false,
			ErrorCode:   400,
			ErrorReason: "Bad Request",
			Details:     "Invalid Authorization header format",
			ShouldStop:  true,
			Context: map[string]interface{}{
				"validator": "AuthValidator",
				"error":     "invalid_auth_format",
			},
		}
	}

	// Parse digest parameters
	digestParams := parseDigestAuth(authorization)
	
	// Check required digest parameters
	requiredParams := []string{"username", "realm", "nonce", "uri", "response"}
	for _, param := range requiredParams {
		if digestParams[param] == "" {
			return ValidationResult{
				Valid:       false,
				ErrorCode:   400,
				ErrorReason: "Bad Request",
				Details:     "Missing required digest parameter: " + param,
				ShouldStop:  true,
				Context: map[string]interface{}{
					"validator":      "AuthValidator",
					"error":          "missing_digest_param",
					"missing_param":  param,
				},
			}
		}
	}

	// Check realm matches
	if digestParams["realm"] != av.realm {
		return ValidationResult{
			Valid:       false,
			ErrorCode:   401,
			ErrorReason: "Unauthorized",
			Details:     "Invalid realm",
			ShouldStop:  true,
			Context: map[string]interface{}{
				"validator":      "AuthValidator",
				"error":          "invalid_realm",
				"expected_realm": av.realm,
				"provided_realm": digestParams["realm"],
			},
		}
	}

	// At this point, we have a properly formatted digest auth header
	// The actual credential verification would be done by the authentication service
	return ValidationResult{
		Valid: true,
		Context: map[string]interface{}{
			"validator": "AuthValidator",
			"username":  digestParams["username"],
			"realm":     digestParams["realm"],
		},
	}
}

// parseDigestAuth parses digest authentication parameters
func parseDigestAuth(authorization string) map[string]string {
	params := make(map[string]string)
	
	// Remove "Digest " prefix
	digestStr := strings.TrimPrefix(authorization, "Digest ")
	
	// Split by comma and parse key=value pairs
	parts := strings.Split(digestStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if idx := strings.Index(part, "="); idx > 0 {
			key := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+1:])
			
			// Remove quotes if present
			if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
				value = value[1 : len(value)-1]
			}
			
			params[key] = value
		}
	}
	
	return params
}