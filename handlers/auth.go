package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// The credential database stores Gmail addresses and bcrypt hashes. Set
// CLASSEDGE_USER_DB to choose another location; the default is ./classedge-users.json.
type credentialDB struct {
	Users []account `json:"users"`
	// Legacy fields allow an existing single-user database to be loaded.
	Username     string `json:"username,omitempty"`
	PasswordHash string `json:"passwordHash,omitempty"`
}

type account struct {
	Email        string    `json:"email"`
	PasswordHash string    `json:"passwordHash"`
	Role         string    `json:"role"`
	Disabled     bool      `json:"disabled,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

type sessionRecord struct {
	Email   string
	Expires time.Time
}

var auth = struct {
	sync.Mutex
	path        string
	credentials credentialDB
	sessions    map[string]sessionRecord
}{sessions: make(map[string]sessionRecord)}

const sessionLifetime = 24 * time.Hour

func configureAuth(path string) error {
	auth.Lock()
	defer auth.Unlock()
	auth.path = path
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &auth.credentials); err != nil {
		return err
	}
	if (auth.credentials.Username == "") != (auth.credentials.PasswordHash == "") {
		return errors.New("incomplete user record")
	}
	migrated := false
	if auth.credentials.Username != "" {
		auth.credentials.Users = append(auth.credentials.Users, account{Email: auth.credentials.Username, PasswordHash: auth.credentials.PasswordHash})
		auth.credentials.Username, auth.credentials.PasswordHash = "", ""
		migrated = true
	}
	for i := range auth.credentials.Users {
		user := &auth.credentials.Users[i]
		if user.Email == "" || user.PasswordHash == "" {
			return errors.New("incomplete user record")
		}
		if user.Role == "" {
			if i == 0 {
				user.Role = "admin"
			} else {
				user.Role = "user"
			}
			migrated = true
		}
		if user.Role != "admin" && user.Role != "user" {
			return errors.New("invalid account role")
		}
		if user.CreatedAt.IsZero() {
			user.CreatedAt = time.Now().UTC()
			migrated = true
		}
	}
	if migrated {
		if err := saveCredential(auth.credentials); err != nil {
			return err
		}
	}
	return nil
}

func saveCredential(c credentialDB) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(auth.path); dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	tmp := auth.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := os.Rename(tmp, auth.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	auth.credentials = c
	return nil
}

func requireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := currentAccount(r); !ok {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeJSON(w, http.StatusUnauthorized, apiError{"Please log in to continue."})
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func currentAccount(r *http.Request) (account, bool) {
	c, err := r.Cookie("classedge_session")
	if err != nil {
		return account{}, false
	}
	auth.Lock()
	defer auth.Unlock()
	session, ok := auth.sessions[c.Value]
	if !ok {
		return account{}, false
	}
	if time.Now().After(session.Expires) {
		delete(auth.sessions, c.Value)
		return account{}, false
	}
	for _, user := range auth.credentials.Users {
		if strings.EqualFold(user.Email, session.Email) && !user.Disabled {
			return user, true
		}
	}
	delete(auth.sessions, c.Value)
	return account{}, false
}

func issueSession(w http.ResponseWriter, email string) error {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	token := hex.EncodeToString(b)
	auth.Lock()
	auth.sessions[token] = sessionRecord{Email: email, Expires: time.Now().Add(sessionLifetime)}
	auth.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "classedge_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: int(sessionLifetime.Seconds())})
	return nil
}

func handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"Method not allowed."})
		return
	}
	auth.Lock()
	configured := len(auth.credentials.Users) != 0
	auth.Unlock()
	user, authenticated := currentAccount(r)
	writeJSON(w, http.StatusOK, map[string]any{"configured": configured, "authenticated": authenticated, "email": user.Email, "role": user.Role})
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func decodeAuthRequest(w http.ResponseWriter, r *http.Request) (authRequest, bool) {
	var req authRequest
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"Method not allowed. Use POST."})
		return req, false
	}
	if !decodeJSON(w, r, &req) {
		return req, false
	}
	req.Email = strings.TrimSpace(req.Email)
	if len(req.Email) > 254 || strings.ContainsAny(req.Email, "\r\n\x00") {
		writeJSON(w, http.StatusBadRequest, apiError{"Enter a valid Gmail address."})
		return req, false
	}
	if len(req.Password) > 72 {
		writeJSON(w, http.StatusBadRequest, apiError{"Password must be no more than 72 bytes."})
		return req, false
	}
	return req, true
}

func validGmail(email string) bool {
	parsed, err := mail.ParseAddress(email)
	return err == nil && parsed.Address == email && strings.EqualFold(email[strings.LastIndex(email, "@")+1:], "gmail.com")
}

func strongPassword(password string) bool {
	if len(password) < 8 || len(password) > 72 {
		return false
	}
	var lower, upper, digit, symbol bool
	for _, r := range password {
		switch {
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= '0' && r <= '9':
			digit = true
		default:
			symbol = true
		}
	}
	classes := 0
	for _, present := range []bool{lower, upper, digit, symbol} {
		if present {
			classes++
		}
	}
	return classes >= 3 || len([]rune(password)) >= 12
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeAuthRequest(w, r)
	if !ok {
		return
	}
	if !validGmail(req.Email) {
		writeJSON(w, http.StatusBadRequest, apiError{"Use a valid @gmail.com address."})
		return
	}
	if !strongPassword(req.Password) {
		writeJSON(w, http.StatusBadRequest, apiError{"Use at least 8 characters and include 3 of these: lowercase, uppercase, number, symbol. A passphrase of 12 or more characters is also accepted."})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{"Could not create account."})
		return
	}
	auth.Lock()
	defer auth.Unlock()
	for _, user := range auth.credentials.Users {
		if strings.EqualFold(user.Email, req.Email) {
			writeJSON(w, http.StatusConflict, apiError{"An account already exists for this Gmail address. Please log in."})
			return
		}
	}
	role := "user"
	if len(auth.credentials.Users) == 0 {
		role = "admin"
	}
	users := append(append([]account(nil), auth.credentials.Users...), account{Email: strings.ToLower(req.Email), PasswordHash: string(hash), Role: role, CreatedAt: time.Now().UTC()})
	if err := saveCredential(credentialDB{Users: users}); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{"Could not save account."})
		return
	}
	auth.Unlock()
	err = issueSession(w, strings.ToLower(req.Email))
	auth.Lock()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{"Account created, but could not start a session."})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "registered"})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeAuthRequest(w, r)
	if !ok {
		return
	}
	auth.Lock()
	users := append([]account(nil), auth.credentials.Users...)
	auth.Unlock()
	var matched *account
	for i := range users {
		if subtle.ConstantTimeCompare([]byte(strings.ToLower(req.Email)), []byte(strings.ToLower(users[i].Email))) == 1 {
			matched = &users[i]
		}
	}
	if matched == nil || matched.Disabled || bcrypt.CompareHashAndPassword([]byte(matched.PasswordHash), []byte(req.Password)) != nil {
		writeJSON(w, http.StatusUnauthorized, apiError{"Incorrect username or password."})
		return
	}
	if err := issueSession(w, matched.Email); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{"Could not start session."})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged in"})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"Method not allowed."})
		return
	}
	if c, err := r.Cookie("classedge_session"); err == nil {
		auth.Lock()
		delete(auth.sessions, c.Value)
		auth.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "classedge_session", Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}
