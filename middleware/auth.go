package middleware

import (
	"cronmonitor/config"
	"cronmonitor/db"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		features := config.LoadFeatures()

		// 1. Feature Flag Check
		if !features.AuthEnabled {
			// Backward Compatibility: Inject System User
			var systemID string
			err := db.GetDB().QueryRow("SELECT id FROM users WHERE email = 'system@afterrun.internal'").Scan(&systemID)
			if err != nil {
				// Fallback if migration hasn't run yet (should verify in Verify steps)
				c.Set("userID", "system-placeholder")
			} else {
				c.Set("userID", systemID)
			}
			c.Set("userEmail", "system@afterrun.internal")
			c.Next()
			return
		}

		// 2. Token Extraction
		tokenString := ""

		// Check Header
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		}

		// Check Cookie (fallback)
		if tokenString == "" {
			cookie, err := c.Cookie("afterrun_jwt")
			if err == nil {
				tokenString = cookie
			}
		}

		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		// 3. Validation
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		// 4. Claims Extraction
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			// standard claims check
			if exp, ok := claims["exp"].(float64); ok {
				if time.Now().Unix() > int64(exp) {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token expired"})
					return
				}
			}

			c.Set("userID", claims["user_id"])
			c.Set("userEmail", claims["email"])
			c.Next()
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
		}
	}
}
