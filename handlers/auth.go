package handlers

import (
	"cronmonitor/db"
	"cronmonitor/models"
	"cronmonitor/services"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

type AuthInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

func Signup(c *gin.Context) {
	var input AuthInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	var userID string
	err = db.GetDB().QueryRow(
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
		input.Email, string(hash),
	).Scan(&userID)

	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
		return
	}

	token, _ := generateToken(userID, input.Email)
	setAuthCookie(c, token)
	c.JSON(http.StatusCreated, gin.H{"token": token, "redirect": "/"})
}

func Login(c *gin.Context) {
	var input AuthInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	err := db.GetDB().QueryRow(
		`SELECT id, email, password_hash FROM users WHERE email = $1`,
		input.Email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, _ := generateToken(user.ID, user.Email)
	setAuthCookie(c, token)
	c.JSON(http.StatusOK, gin.H{"token": token, "redirect": "/"})
}

func Me(c *gin.Context) {
	userID, _ := c.Get("userID")

	// Complex Fetch with Counts
	var response struct {
		models.User
		JobCount int `json:"job_count"`
		JobLimit int `json:"job_limit"`
	}

	err := db.GetDB().QueryRow(
		`SELECT id, email, subscription_tier, subscription_status, created_at FROM users WHERE id = $1`,
		userID,
	).Scan(&response.ID, &response.Email, &response.SubscriptionTier, &response.SubscriptionStatus, &response.CreatedAt)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Enrich with Counts
	if err := db.GetDB().QueryRow("SELECT COUNT(*) FROM jobs WHERE user_id = $1", userID).Scan(&response.JobCount); err != nil {
		response.JobCount = 0
	}

	response.JobLimit = services.GetJobLimit(response.SubscriptionTier)

	c.JSON(http.StatusOK, response)
}

func generateToken(id, email string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": id,
		"email":   email,
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(),
	})
	return token.SignedString(jwtSecret)
}

func setAuthCookie(c *gin.Context, token string) {
	c.SetCookie("afterrun_jwt", token, 3600*24*7, "/", "", false, true) // HttpOnly=true, Secure=false (dev)
}
