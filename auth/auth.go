package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/drouian-m/gimme/config"
	"github.com/golang-jwt/jwt/v4"
)

type AuthManager struct {
	config *config.Config
}

func NewAuthManager(config *config.Config) *AuthManager {
	return &AuthManager{
		config,
	}
}

type CreateTokenRequest struct {
	Name           string `json:"name"`
	ExpirationDate string `json:"expirationDate"`
}

func (am *AuthManager) CreateToken(name string, expirationDate string) (string, error) {
	var expiration time.Duration
	if len(expirationDate) > 0 {
		format := "2006-01-02"
		end, _ := time.Parse(format, expirationDate)
		expiration = time.Minute * time.Duration(end.Sub(time.Now()).Minutes())
	} else {
		expiration = time.Minute * 15
	}

	if expiration <= 0 {
		return "", errors.New("Expiration date must be greater than the current date.")
	}

	claims := &jwt.RegisteredClaims{
		ID:        name,
		ExpiresAt: &jwt.NumericDate{time.Now().Add(expiration)},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString([]byte(am.config.Secret))
	if err != nil {
		return "", err
	}
	return signedToken, nil
}

func (am *AuthManager) extractToken(authHeader string) string {
	strArr := strings.Split(authHeader, " ")
	if len(strArr) == 2 {
		return strArr[1]
	}
	return ""
}

func (am *AuthManager) decodeToken(token string, config *config.Config) (*jwt.Token, error) {
	decoded, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	return decoded, nil
}

func (am *AuthManager) getClaimsFromJWT(token *jwt.Token) jwt.MapClaims {
	claims := jwt.MapClaims{}
	for key, value := range token.Claims.(jwt.MapClaims) {
		claims[key] = value
	}

	return claims
}

func (am *AuthManager) AuthenticateMiddleware(c *gin.Context) {
	tokenString := am.extractToken(c.GetHeader("Authorization"))
	token, err := am.decodeToken(tokenString, am.config)
	if err != nil || !token.Valid {
		c.Status(http.StatusUnauthorized)
		c.Abort()
		return
	}

	tokenClaims := am.getClaimsFromJWT(token)
	if tokenClaims["exp"] == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing exp field"})
		c.Abort()
		return
	}

	c.Next()
}
