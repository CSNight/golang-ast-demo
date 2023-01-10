package middleware

import (
	"golang-ast/infra"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
)

var signInPath = "/api/auth/sign"
var signOutPath = "/api/auth/logout"

func NewAuthFilter() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHandler := infra.GetAuthHandler()
		match, permit, _ := authHandler.TrieSearch(c.Path())
		tokenStr, ok := extractToken(c)
		if match && permit == "*" || !match {
			// block dup login
			if c.Path() == signInPath && ok {
				claimUser, err := authHandler.ParseToken(tokenStr)
				if err == nil && authHandler.IsAlreadyLogin(claimUser.Name) {
					return infra.OkWithMessage("already login", c)
				}
			}
			// handle logout
			if c.Path() == signOutPath && ok {
				claimUser, err := authHandler.ParseToken(tokenStr)
				if err == nil && authHandler.IsAlreadyLogin(claimUser.Name) {
					return authHandler.OnSignOutHandler(claimUser, c)
				}
			}
			// request continue when url don't need token
			return c.Next()
		}
		if ok {
			claim, err := authHandler.ParseToken(tokenStr)
			if err != nil {
				// token expire renewal
				if err == infra.ErrTokenExpired {
					tokenNew, errTk := authHandler.RefreshToken(tokenStr)
					if errTk != nil {
						return infra.FailWithMessage(http.StatusUnauthorized, errTk.Error(), c)
					}
					claim, errTk = authHandler.ParseToken(tokenNew)
					if err != nil {
						return infra.FailWithMessage(http.StatusUnauthorized, errTk.Error(), c)
					}
					c.Set("Authorization", "Bearer "+tokenNew)
					c.Set("Access-Control-Expose-Headers", "Authorization")
				} else {
					// token parser error
					return infra.FailWithMessage(http.StatusUnauthorized, err.Error(), c)
				}
			}
			// get user authentication
			authentication, err := authHandler.GetAuthentication(claim)
			if err != nil {
				return infra.FailWithMessage(http.StatusUnauthorized, err.Error(), c)
			}

			// request continue when url don't need permit
			if permit == "any" && match {
				// prepare user auth context
				if authentication != nil {
					c.Locals("auth", authentication)
				}
				return c.Next()
			}
			// check user permit
			if !authentication.Authorities().Contains(permit) {
				return infra.FailWithMessage(http.StatusForbidden, "not authorized", c)
			}
			// prepare user auth context
			if authentication != nil {
				c.Locals("auth", authentication)
			}
			return c.Next()
		}
		// request block when url need token, but token not found in header
		return infra.FailWithMessage(http.StatusUnauthorized, "not authorized", c)
	}
}

func extractToken(req *fiber.Ctx) (string, bool) {
	tokenHeader := req.Get("Authorization")
	// The usual convention is for "Bearer" to be title-cased. However, there's no
	// strict rule around this, and it's best to follow the robustness principle here.
	if tokenHeader == "" || !strings.HasPrefix(strings.ToLower(tokenHeader), "bearer ") {
		return "", false
	}
	return tokenHeader[7:], true
}
