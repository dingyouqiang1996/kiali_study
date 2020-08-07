package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/util"
)

type dummyHandler struct {
}

func (t dummyHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

// TestStrategyLoginAuthentication checks that a user with no active
// session is logged in successfully
func TestStrategyLoginAuthentication(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	cfg := config.NewConfig()
	cfg.Auth.Strategy = config.AuthStrategyToken
	cfg.LoginToken.SigningKey = util.RandomString(10)
	config.Set(cfg)

	clockTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	util.Clock = util.ClockMock{Time: clockTime}

	request := httptest.NewRequest("GET", "http://kiali/api/authenticate", nil)
	request.SetBasicAuth("foo", "bar")

	// Add a stale token to the request. Authentication should succeed even if a stale
	// session is present. This prevents the user form manually clean browser cookies.
	currentToken, _ := config.GenerateToken("dummy")
	oldCookie := http.Cookie{
		Name:  config.TokenCookieName,
		Value: currentToken.Token,
	}
	request.AddCookie(&oldCookie)

	responseRecorder := httptest.NewRecorder()
	Authenticate(responseRecorder, request)
	response := responseRecorder.Result()

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Len(t, response.Cookies(), 1)

	cookie := response.Cookies()[0]
	assert.Equal(t, config.TokenCookieName, cookie.Name)
	assert.True(t, cookie.HttpOnly)

	newToken, _ := config.GenerateToken("foo")
	assert.Equal(t, cookie.Value, newToken.Token)
	assert.Equal(t, clockTime.Add(time.Second*time.Duration(cfg.LoginToken.ExpirationSeconds)), cookie.Expires)
}

// TestStrategyLoginInvalidSignature checks that an altered JWT token is
// rejected as a valid authentication
func TestStrategyLoginInvalidSignature(t *testing.T) {
	// Set some config values to a known state
	rand.Seed(time.Now().UnixNano())
	cfg := config.NewConfig()
	cfg.Auth.Strategy = config.AuthStrategyToken
	cfg.LoginToken.SigningKey = util.RandomString(10)
	config.Set(cfg)

	// Mock the clock
	clockTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	util.Clock = util.ClockMock{Time: clockTime}
	jwt.TimeFunc = func() time.Time {
		return util.Clock.Now()
	}

	// First go through authentication to get a kiali cookie.
	authRequest := httptest.NewRequest("GET", "http://kiali/api/authenticate", nil)
	authRequest.SetBasicAuth("foo", "bar")

	responseRecorder := httptest.NewRecorder()
	Authenticate(responseRecorder, authRequest)
	response := responseRecorder.Result()

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Len(t, response.Cookies(), 1)

	// Decode the JWT token in the cookie
	cookie := response.Cookies()[0]
	assert.Equal(t, config.TokenCookieName, cookie.Name)

	token, err := jwt.ParseWithClaims(cookie.Value, &config.IanaClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.GetSigningKey()), nil
	})

	assert.Nil(t, err)
	assert.True(t, token.Valid) // Sanity check. The token should always be valid.

	// Get the raw token
	tokenString := token.Raw

	// OK, authentication succeeded and we have a token.
	// Let's create a hacked token with a mutated payload. The header and signature of the
	// token will be kept unchanged.

	// Build custom claims
	timeExpire := util.Clock.Now().Add(time.Second * time.Duration(60)) // 1 minute expiration from now
	customClaims := config.IanaClaims{
		StandardClaims: jwt.StandardClaims{
			Subject:   "foo", // We use the "foo" user which should be valid
			ExpiresAt: timeExpire.Unix(),
			Issuer:    config.AuthStrategyTokenIssuer,
		},
	}

	// Get JSON string of our customized claims
	jsonValue, err := json.Marshal(customClaims)
	assert.Nil(t, err)

	// Hack the token.
	tokenEntries := strings.Split(tokenString, ".")
	tokenEntries[1] = jwt.EncodeSegment(jsonValue) // Second entry is the payload
	tokenString = strings.Join(tokenEntries, ".")

	// Now that we have a "hacked" token with a new expiration date, lets
	// use it to invoke the authentication handler (which is invoked on all protected endpoints).

	// Build the request with the cookie
	maliciousRequest := httptest.NewRequest("GET", "http://kiali/api/foo", nil)
	hackedCookie := http.Cookie{
		Name:  config.TokenCookieName,
		Value: tokenString,
	}
	maliciousRequest.AddCookie(&hackedCookie)

	// Setup authentication handler
	authenticationHandler, _ := NewAuthenticationHandler()
	handler := authenticationHandler.Handle(new(dummyHandler))

	// Run the malicious request
	maliciousResponseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(maliciousResponseRecorder, maliciousRequest)
	hackedResponse := maliciousResponseRecorder.Result()

	// Server should return an unauthorized response code.
	// Body should be the text explanation of the HTTP error
	body, _ := ioutil.ReadAll(hackedResponse.Body)
	assert.Equal(t, http.StatusUnauthorized, hackedResponse.StatusCode)
	assert.Equal(t, fmt.Sprintln(http.StatusText(http.StatusUnauthorized)), string(body))
}

// TestStrategyLoginValidatesExpiration checks that the Kiali back-end is
// correctly checking the expiration time of the Kiali token.
//
// Assuming that a malicious user has stolen the Kiali token of a user,
// that user may use it to make requests to the Kiali API. The expiration
// date of the token and the browser's cookie should be in sync. But a malicious
// user may want to create his own cookie and use with the stolen token. Because
// of this, the Kiali backend must check the expiration Claim of the JWT token.
func TestStrategyLoginValidatesExpiration(t *testing.T) {
	// Set some config values to a known state
	rand.Seed(time.Now().UnixNano())
	cfg := config.NewConfig()
	cfg.Auth.Strategy = config.AuthStrategyToken
	cfg.LoginToken.SigningKey = util.RandomString(10)
	config.Set(cfg)

	// Mock the clock
	clockTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	util.Clock = util.ClockMock{Time: clockTime}
	jwt.TimeFunc = func() time.Time {
		return util.Clock.Now()
	}

	// Let's create a valid but expired token.
	timeExpire := util.Clock.Now().Add(-time.Second * time.Duration(1)) // Expiration time is one second in the past
	customClaims := config.IanaClaims{
		StandardClaims: jwt.StandardClaims{
			Subject:   "foo",
			ExpiresAt: timeExpire.Unix(),
			Issuer:    config.AuthStrategyTokenIssuer,
		},
	}

	token, _ := config.GetSignedTokenString(customClaims)

	// Let's simulate a request with the expired token
	request := httptest.NewRequest("GET", "http://kiali/api/foo", nil)
	cookie := http.Cookie{
		Name:  config.TokenCookieName,
		Value: token,
	}
	request.AddCookie(&cookie)

	authenticationHandler, _ := NewAuthenticationHandler()
	handler := authenticationHandler.Handle(new(dummyHandler))

	responseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(responseRecorder, request)
	response := responseRecorder.Result()

	// Server should return an unauthorized response code.
	// Body should be the text explanation of the HTTP error
	body, _ := ioutil.ReadAll(response.Body)
	assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	assert.Equal(t, fmt.Sprintln(http.StatusText(http.StatusUnauthorized)), string(body))
}

// TestStrategyLoginValidatesUserChange checks that the Kiali back-end is verifying that
// the user specified in a Kiali token matches to the user in the configuration.
// This is for a scenario where logged in users should be kicked if an administrator
// changes the Kiali configuration.
func TestStrategyLoginValidatesUserChange(t *testing.T) {
	// Set some config values to a known state
	rand.Seed(time.Now().UnixNano())
	cfg := config.NewConfig()
	cfg.Auth.Strategy = config.AuthStrategyToken
	cfg.LoginToken.SigningKey = util.RandomString(10)
	config.Set(cfg)

	// Mock the clock
	clockTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	util.Clock = util.ClockMock{Time: clockTime}
	jwt.TimeFunc = func() time.Time {
		return util.Clock.Now()
	}

	// Let's create a valid token with a user not matching the one from the config.
	timeExpire := util.Clock.Now().Add(time.Second * time.Duration(1))
	customClaims := config.IanaClaims{
		StandardClaims: jwt.StandardClaims{
			Subject:   "dummy", // wrong user
			ExpiresAt: timeExpire.Unix(),
			Issuer:    config.AuthStrategyTokenIssuer,
		},
	}

	token, _ := config.GetSignedTokenString(customClaims)

	// Let's simulate a request
	request := httptest.NewRequest("GET", "http://kiali/api/foo", nil)
	cookie := http.Cookie{
		Name:  config.TokenCookieName,
		Value: token,
	}
	request.AddCookie(&cookie)

	authenticationHandler, _ := NewAuthenticationHandler()
	handler := authenticationHandler.Handle(new(dummyHandler))

	responseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(responseRecorder, request)
	response := responseRecorder.Result()

	// Server should return an unauthorized response code.
	// Body should be the text explanation of the HTTP error
	body, _ := ioutil.ReadAll(response.Body)
	assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	assert.Equal(t, fmt.Sprintln(http.StatusText(http.StatusUnauthorized)), string(body))
}

// TestStrategyLoginMissingUser checks that the Kiali back-end is ensuring
// that the username field is populated in the Kiali auth token.
func TestStrategyLoginMissingUser(t *testing.T) {
	// Set some config values to a known state
	rand.Seed(time.Now().UnixNano())
	cfg := config.NewConfig()
	cfg.Auth.Strategy = config.AuthStrategyToken
	cfg.LoginToken.SigningKey = util.RandomString(10)
	config.Set(cfg)

	// Mock the clock
	clockTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	util.Clock = util.ClockMock{Time: clockTime}
	jwt.TimeFunc = func() time.Time {
		return util.Clock.Now()
	}

	// Let's create a valid token without subject.
	timeExpire := util.Clock.Now().Add(time.Second * time.Duration(1))
	customClaims := config.IanaClaims{
		StandardClaims: jwt.StandardClaims{
			// Subject:   "foo",
			ExpiresAt: timeExpire.Unix(),
			Issuer:    config.AuthStrategyTokenIssuer,
		},
	}

	token, _ := config.GetSignedTokenString(customClaims)

	// Let's simulate a request
	request := httptest.NewRequest("GET", "http://kiali/api/foo", nil)
	cookie := http.Cookie{
		Name:  config.TokenCookieName,
		Value: token,
	}
	request.AddCookie(&cookie)

	authenticationHandler, _ := NewAuthenticationHandler()
	handler := authenticationHandler.Handle(new(dummyHandler))

	responseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(responseRecorder, request)
	response := responseRecorder.Result()

	// Server should return an unauthorized response code.
	// Body should be the text explanation of the HTTP error
	body, _ := ioutil.ReadAll(response.Body)
	assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	assert.Equal(t, fmt.Sprintln(http.StatusText(http.StatusUnauthorized)), string(body))
}

// TestStrategyLoginMissingExpiration checks that the Kiali back-end is ensuring
// that the expiration date claim is populated in the Kiali auth token.
func TestStrategyLoginMissingExpiration(t *testing.T) {
	// Set some config values to a known state
	rand.Seed(time.Now().UnixNano())
	cfg := config.NewConfig()
	cfg.Auth.Strategy = config.AuthStrategyToken
	cfg.LoginToken.SigningKey = util.RandomString(10)
	config.Set(cfg)

	// Let's create a valid token that does not expire.
	customClaims := config.IanaClaims{
		StandardClaims: jwt.StandardClaims{
			Subject: "foo",
			// ExpiresAt: timeExpire.Unix(),
			Issuer: config.AuthStrategyTokenIssuer,
		},
	}

	token, _ := config.GetSignedTokenString(customClaims)

	// Let's simulate a request
	request := httptest.NewRequest("GET", "http://kiali/api/foo", nil)
	cookie := http.Cookie{
		Name:  config.TokenCookieName,
		Value: token,
	}
	request.AddCookie(&cookie)

	authenticationHandler, _ := NewAuthenticationHandler()
	handler := authenticationHandler.Handle(new(dummyHandler))

	responseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(responseRecorder, request)
	response := responseRecorder.Result()

	// Server should return an unauthorized response code.
	// Body should be the text explanation of the HTTP error
	body, _ := ioutil.ReadAll(response.Body)
	assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	assert.Equal(t, fmt.Sprintln(http.StatusText(http.StatusUnauthorized)), string(body))
}

// TestStrategyLoginFails checks that a login attempt is
// rejected if user provides wrong credentials
func TestStrategyLoginFails(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Auth.Strategy = config.AuthStrategyToken
	config.Set(cfg)

	// Check wrong user
	request := httptest.NewRequest("GET", "http://kiali/api/authenticate", nil)
	request.SetBasicAuth("jdoe", "bar")

	responseRecorder := httptest.NewRecorder()
	Authenticate(responseRecorder, request)
	response := responseRecorder.Result()

	assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	assert.Len(t, response.Cookies(), 0)

	// Check wrong password
	request = httptest.NewRequest("GET", "http://kiali/api/authenticate", nil)
	request.SetBasicAuth("foo", "baz")

	responseRecorder = httptest.NewRecorder()
	Authenticate(responseRecorder, request)
	response = responseRecorder.Result()

	assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	assert.Len(t, response.Cookies(), 0)
}

// TestStrategyLoginExtend checks that a user with an active session
// received a refreshed token to extend his session time
func TestStrategyLoginExtend(t *testing.T) {
	jwt.TimeFunc = func() time.Time {
		return util.Clock.Now()
	}

	rand.Seed(time.Now().UnixNano())
	cfg := config.NewConfig()
	cfg.Auth.Strategy = config.AuthStrategyToken
	cfg.LoginToken.SigningKey = util.RandomString(10)
	config.Set(cfg)

	clockTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	util.Clock = util.ClockMock{Time: clockTime}

	request := httptest.NewRequest("GET", "http://kiali/api/authenticate", nil)
	currentToken, _ := config.GenerateToken("foo")
	oldCookie := http.Cookie{
		Name:  config.TokenCookieName,
		Value: currentToken.Token,
	}
	request.AddCookie(&oldCookie)

	clockTime.Add(time.Second * time.Duration(cfg.LoginToken.ExpirationSeconds-10))
	util.Clock = util.ClockMock{Time: clockTime}

	responseRecorder := httptest.NewRecorder()
	Authenticate(responseRecorder, request)
	response := responseRecorder.Result()

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Len(t, response.Cookies(), 1)

	cookie := response.Cookies()[0]
	assert.Equal(t, config.TokenCookieName, cookie.Name)
	assert.True(t, cookie.HttpOnly)

	expectedToken, _ := config.GenerateToken("foo")
	assert.NotEmpty(t, cookie.Value)
	assert.Equal(t, expectedToken.Token, cookie.Value)
	assert.Equal(t, clockTime.Add(time.Second*time.Duration(cfg.LoginToken.ExpirationSeconds)), cookie.Expires)
}

// TestStrategyLoginRejectStaleExtension checks that a stale
// session (because of a signing key, username or password change) won't
// be extended, but rejected as unauthorized.
func TestStrategyLoginRejectStaleExtension(t *testing.T) {
	jwt.TimeFunc = func() time.Time {
		return util.Clock.Now()
	}

	rand.Seed(time.Now().UnixNano())
	cfg := config.NewConfig()
	cfg.Auth.Strategy = config.AuthStrategyToken
	cfg.LoginToken.SigningKey = util.RandomString(10)
	config.Set(cfg)

	clockTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	util.Clock = util.ClockMock{Time: clockTime}

	// If signing key has changed, session shouldn't be extended
	request := httptest.NewRequest("GET", "http://kiali/api/authenticate", nil)
	token, _ := config.GenerateToken("foo")
	cfg.LoginToken.SigningKey = "myNewKey"
	config.Set(cfg)

	request = httptest.NewRequest("GET", "http://kiali/api/authenticate", nil)
	oldCookie := http.Cookie{
		Name:  config.TokenCookieName,
		Value: token.Token,
	}
	request.AddCookie(&oldCookie)

	responseRecorder := httptest.NewRecorder()
	Authenticate(responseRecorder, request)
	response := responseRecorder.Result()

	assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	assert.Len(t, response.Cookies(), 0)
}

// TestLogoutWhenNoSession checks that the Logout handler
// returns a blank response with no cookies being set when the
// user is not logged in.
func TestLogoutWhenNoSession(t *testing.T) {
	request := httptest.NewRequest("GET", "http://kiali/api/logout", nil)
	responseRecorder := httptest.NewRecorder()
	Logout(responseRecorder, request)

	response := responseRecorder.Result()
	assert.Equal(t, http.StatusNoContent, response.StatusCode)
	assert.Zero(t, len(response.Cookies()))
}

// TestLogout checks that the Logout handler
// sets a blank cookie to terminate the user's session
func TestLogout(t *testing.T) {
	clockTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	util.Clock = util.ClockMock{Time: clockTime}

	request := httptest.NewRequest("GET", "http://kiali/api/logout", nil)
	request.AddCookie(&http.Cookie{
		Name:  config.TokenCookieName,
		Value: "foo",
	})

	responseRecorder := httptest.NewRecorder()
	Logout(responseRecorder, request)

	response := responseRecorder.Result()
	assert.Equal(t, http.StatusNoContent, response.StatusCode)
	assert.Equal(t, 1, len(response.Cookies()))

	cookie := response.Cookies()[0]
	assert.Equal(t, config.TokenCookieName, cookie.Name)
	assert.True(t, cookie.HttpOnly)
	// assert.Equal(t,, http.SameSiteStrictMode, cookie.SameSite) ** Commented out because unsupported in go < 1.11

	assert.Equal(t, "", cookie.Value)
	assert.True(t, cookie.Expires.Before(clockTime))
}
