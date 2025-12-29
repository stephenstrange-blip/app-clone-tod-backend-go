package auth

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Token interface {
	Sign(int) error
}

type CustomToken struct {
	TokenString string // `json:"token,omitempty"`
	Key         []byte
	Claims      *CustomClaim
}

type CustomClaim struct {
	UserID int `json:"userID"`
	jwt.RegisteredClaims
}

// Custom Claims need to have custome validators implemented by adding Validate() method
// In future, this should be expanded to getting authorization details of user
func (c *CustomClaim) Validate() error {
	// local users can only have ids of > 0
	if c.UserID == 0 {
		return errors.New("invalid userID")
	}
	return nil
}

// Sets a token string.
//
// Accepts a userID integer for added validation of the returned token string
func (j *CustomToken) Sign(userID int) error {
	// Call necessary setter methods first, else errors

	// Set secret key
	if err := j.SetKey(); err != nil {
		return err
	}

	// Set claims
	if err := j.SetClaims(userID); err != nil {
		return err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, j.Claims)

	if tokenString, err := token.SignedString(j.Key); err != nil {
		return err
	} else {
		j.TokenString = tokenString
		return nil
	}
}

// Sets a pbkdf2-hashed string from a custom secret
func (j *CustomToken) SetKey() error {
	secret, ok := os.LookupEnv("JWT_SECRET")
	if !ok {
		return errors.New("environment variable not found")
	}

	hashed, err := pbkdf2.Key(sha256.New, secret, make([]byte, 32), 4096, 32)
	if err != nil {
		return err
	}

	// store key
	j.Key = hashed
	return nil
}

// Sets up custom claims config with an embedded jwt RegisteredClaims config struct
func (j *CustomToken) SetClaims(userID int) error {
	var (
		aud string
		iss string
		ok  bool
		err = errors.New("token env missing")
	)

	if aud, ok = os.LookupEnv("FRONTEND_URL"); !ok {
		return err
	}

	if iss, ok = os.LookupEnv("BASE_SERVER_URL"); !ok {
		return err
	}

	claims := &CustomClaim{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    iss,
			Audience:  jwt.ClaimStrings{aud},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		}}

	// store claims
	j.Claims = claims

	return nil
}

// Verifies token and returns userID
func (j *CustomToken) Verify(tokenString string) (int, error) {
	// used by ParseWithClaims to get and verify secret embedded in token
	getSecret := func(t *jwt.Token) (any, error) {
		if err := j.SetKey(); err != nil {
			return nil, err
		} else {
			return j.Key, nil
		}
	}

	token, err := jwt.ParseWithClaims(
		tokenString,
		&CustomClaim{},
		getSecret,
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)

	// Handle token error
	if err != nil {
		return 0, err
	} else if claims, ok := token.Claims.(*CustomClaim); ok {
		return claims.UserID, nil
	}

	return 0, errors.New("unknown claims type: cannot proceed")
}
