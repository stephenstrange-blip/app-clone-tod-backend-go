package auth

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	customUtil "github.com/app-clone-tod-utils"
	"github.com/golang-jwt/jwt/v5"
)

type AppDescription struct {
	Name  string
	Year  int
	Token *jwt.Token
}

// Setting cookies with JWT tokens as values
func SetCookieWithToken(wr http.ResponseWriter, params *AuthRequest) error {
	var (
		customJWT = &CustomToken{}
		err       error
	)

	// Sign token
	if err = customJWT.Sign(params.ID); err != nil {
		return err
	}

	if err = SetCookie(wr, customJWT.TokenString); err != nil {
		return err
	}

	return nil
}

// Setting cookies with encoded structs as values
func SetCookieWithGob(wr http.ResponseWriter) error {
	var (
		app      = &AppDescription{Name: "anonio", Year: time.Now().Year()}
		gobValue string
		err      error
	)

	if gobValue, err = EncodeGob(app); err != nil {
		return err
	}

	if err = SetCookie(wr, gobValue); err != nil {
		return err
	}

	return nil
}

func SetCookie(wr http.ResponseWriter, value string) error {
	if value == "" {
		return errors.New("cookie value empty")
	}

	var err error

	// Include Path to prevent creating duplicate cookies
	cookie := http.Cookie{
		Name:     customUtil.COOKIE_NAME,
		Value:    value,
		MaxAge:   int(24 * time.Minute),
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
		Path:     "/",

		// "Many modern browsers (including Firefox and Chrome) also consider unencrypted connections to localhost to be 'secure'.
		// This means that the cookie should work on localhost even if our web application is only using HTTP.
		Secure: true,
	}

	// Sign and encrypt cookies first
	if err = WriteEncrypted(&cookie); err != nil {
		return err
	}

	// Check if signed cookie value is valid
	if err = Write(wr, &cookie); err != nil {
		return err
	}

	http.SetCookie(wr, &cookie)
	return nil
}

func GetCookie(r *http.Request) (string, error) {
	cookieVal, err := ReadEncrypted(r, customUtil.COOKIE_NAME)

	if err != nil {
		switch {
		case errors.Is(err, http.ErrNoCookie):
			return "", errors.New("token cookie not found")
		default:
			return "", err
		}
	}
	return cookieVal, nil
}

func GetCookieWithToken(r *http.Request) (int, error) {
	tokenString, err := GetCookie(r)

	if err != nil {
		return 0, err
	}

	customJWT := &CustomToken{}

	// Verify token
	userID, err := customJWT.Verify(tokenString)
	if err != nil {
		return 0, err
	}

	return userID, nil
}

func GetCookieWithGob(r *http.Request) (string, error) {
	cookieVal, err := GetCookie(r)
	if err != nil {
		return "", err
	}

	app, err := DecodeGob(cookieVal)
	if err != nil {
		return "", err
	}

	if app.Name != "anonio" || app.Year != time.Now().Year() {
		return "", errors.New("invalid cookie")
	}

	return cookieVal, nil
}

func RemoveCookie(wr http.ResponseWriter, r *http.Request) {
	// Path must be same when setting the cookie
	// MaxAge<0 tells the browser to delete cookie immediately
	cookie := http.Cookie{
		Name:     customUtil.COOKIE_NAME,
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		Expires:  time.Unix(0, 0),
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
		Secure:   true,
	}

	http.SetCookie(wr, &cookie)
}

func Write(wr http.ResponseWriter, c *http.Cookie) error {
	c.Value = base64.StdEncoding.EncodeToString([]byte(c.Value))

	if len(c.String()) > 4096 {
		return errors.New("cookie value too long")
	}

	return nil
}

func WriteSigned(c *http.Cookie) error {
	secret, err := GetCookieSecret()
	if err != nil {
		return err
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(c.Name))
	mac.Write([]byte(c.Value))
	sig := mac.Sum(nil)

	c.Value = string(sig) + c.Value
	return nil
}

func ReadSigned(r *http.Request, name string) (string, error) {
	signedValue, err := Read(r, name)
	if err != nil {
		return "", err
	}

	if len(signedValue) < sha256.Size {
		return "", errors.New("invalid signed cookie")
	}

	secret, err := GetCookieSecret()
	if err != nil {
		return "", err
	}

	sig := signedValue[:sha256.Size]
	value := signedValue[sha256.Size:]

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(name))
	mac.Write([]byte(value))
	expectedSig := mac.Sum(nil)

	if !hmac.Equal([]byte(sig), expectedSig) {
		return "", errors.New("invalid signed cookie")
	}

	return value, nil
}

func WriteEncrypted(c *http.Cookie) error {
	secret, err := GetCookieSecret()
	if err != nil {
		return err
	}

	block, err := aes.NewCipher([]byte(secret))
	if err != nil {
		return err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	// Cookie value could ba a simple string or JSON-WEB-encoded token (JWT)
	plainText := fmt.Sprintf("%s:%s", c.Name, c.Value)

	encryptedValue := aesGCM.Seal(nonce, nonce, []byte(plainText), nil)

	c.Value = string(encryptedValue)

	return nil
}

func Read(r *http.Request, name string) (string, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return "", err
	}

	value, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		return "", err
	}

	return string(value), nil
}

func ReadEncrypted(r *http.Request, name string) (string, error) {
	encryptedValue, err := Read(r, name)
	if err != nil {
		return "", err
	}

	secret, err := GetCookieSecret()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(secret)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()

	if len(encryptedValue) < nonceSize {
		return "", errors.New("invalid cookie value")
	}

	nonce := encryptedValue[:nonceSize]
	cipherText := encryptedValue[nonceSize:]

	plainText, err := aesGCM.Open(nil, []byte(nonce), []byte(cipherText), nil)
	if err != nil {
		return "", err
	}

	expectedName, value, found := strings.Cut(string(plainText), ":")
	if !found {
		return "", errors.New("invalid cookie value")
	}

	if expectedName != name {
		return "", errors.New("invalid cookie value")
	}

	return value, nil
}

func EncodeGob(app *AppDescription) (string, error) {
	var buffer bytes.Buffer

	err := gob.NewEncoder(&buffer).Encode(app)
	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}

func DecodeGob(encodedValue string) (AppDescription, error) {
	var app AppDescription

	reader := strings.NewReader(encodedValue)

	err := gob.NewDecoder(reader).Decode(&app)
	if err != nil {
		return app, err
	}

	return app, nil
}

func GetCookieSecret() ([]byte, error) {
	secret, ok := os.LookupEnv("COOKIE_SECRET")
	if !ok {
		return nil, errors.New("environment variable not found")
	}

	hashed, err := pbkdf2.Key(sha256.New, secret, make([]byte, 32), 4096, 32)
	if err != nil {
		return nil, err
	}

	// logger.Info("Hashing Debug",
	// 	slog.Group("Hashed",
	// 		slog.String("Value", string(hashed)),
	// 		slog.Int("Length", len(hashed)),
	// 	),
	// )

	return hashed, nil
}

func VerifyCookieSecret(hashedSecret []byte) error {
	expectedHashed, err := GetCookieSecret()
	if err != nil {
		return err
	}

	// logger.Info("Comparing Secret",
	// 	slog.Group(
	// 		"Expected",
	// 		slog.String("Value", string(expectedHashed)),
	// 		slog.Int("Length", len(expectedHashed)),
	// 	),
	// 	slog.Group(
	// 		"Passed",
	// 		slog.String("Value", string(hashedSecret)),
	// 		slog.Int("Length", len(hashedSecret)),
	// 	),
	// )

	if string(expectedHashed) != string(hashedSecret) {
		return errors.New("invalid secret")
	}

	return nil
}
