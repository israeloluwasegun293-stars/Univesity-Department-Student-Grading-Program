package handlers

import (
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := currentAccount(r)
		if !ok {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeJSON(w, http.StatusUnauthorized, apiError{"Please log in to continue."})
			} else {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
			}
			return
		}
		if user.Role != "admin" {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeJSON(w, http.StatusForbidden, apiError{"Administrator access is required."})
			} else {
				http.Redirect(w, r, "/app", http.StatusSeeOther)
			}
			return
		}
		next(w, r)
	}
}

type adminUser struct {
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Disabled  bool      `json:"disabled"`
	CreatedAt time.Time `json:"createdAt"`
}

func publicAdminUser(user account) adminUser {
	return adminUser{Email: user.Email, Role: user.Role, Disabled: user.Disabled, CreatedAt: user.CreatedAt}
}

func handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"Method not allowed. Use GET."})
		return
	}
	auth.Lock()
	users, active, admins := len(auth.credentials.Users), 0, 0
	for _, user := range auth.credentials.Users {
		if !user.Disabled {
			active++
			if user.Role == "admin" {
				admins++
			}
		}
	}
	auth.Unlock()
	_, metrics := store.snapshot()
	writeJSON(w, http.StatusOK, map[string]any{"totalUsers": users, "activeUsers": active, "adminUsers": admins, "studentsGraded": metrics.Count, "classAverageCGPA": metrics.AverageGPA})
}

func handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		auth.Lock()
		users := make([]adminUser, 0, len(auth.credentials.Users))
		for _, user := range auth.credentials.Users {
			users = append(users, publicAdminUser(user))
		}
		auth.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"users": users})
	case http.MethodPost:
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		req.Email = strings.TrimSpace(req.Email)
		if !validGmail(req.Email) {
			writeJSON(w, http.StatusBadRequest, apiError{"Use a valid @gmail.com address."})
			return
		}
		if !strongPassword(req.Password) {
			writeJSON(w, http.StatusBadRequest, apiError{"Use at least 8 characters and include 3 of these: lowercase, uppercase, number, symbol. A passphrase of 12 or more characters is also accepted."})
			return
		}
		if req.Role == "" {
			req.Role = "user"
		}
		if req.Role != "admin" && req.Role != "user" {
			writeJSON(w, http.StatusBadRequest, apiError{"Role must be admin or user."})
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
				writeJSON(w, http.StatusConflict, apiError{"An account already exists for this Gmail address."})
				return
			}
		}
		users := append(append([]account(nil), auth.credentials.Users...), account{Email: strings.ToLower(req.Email), PasswordHash: string(hash), Role: req.Role, CreatedAt: time.Now().UTC()})
		if err := saveCredential(credentialDB{Users: users}); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{"Could not save account."})
			return
		}
		writeJSON(w, http.StatusCreated, publicAdminUser(users[len(users)-1]))
	default:
		w.Header().Set("Allow", "GET, POST")
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"Method not allowed. Use GET or POST."})
	}
}

func handleAdminUser(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(strings.TrimSpace(r.PathValue("email")))
	if email == "" {
		writeJSON(w, http.StatusBadRequest, apiError{"User email is required."})
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var req struct {
			Role     string `json:"role"`
			Disabled *bool  `json:"disabled"`
			Password string `json:"password"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Role != "" && req.Role != "admin" && req.Role != "user" {
			writeJSON(w, http.StatusBadRequest, apiError{"Role must be admin or user."})
			return
		}
		var passwordHash string
		if req.Password != "" {
			if !strongPassword(req.Password) {
				writeJSON(w, http.StatusBadRequest, apiError{"Use at least 8 characters and include 3 of these: lowercase, uppercase, number, symbol. A passphrase of 12 or more characters is also accepted."})
				return
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, apiError{"Could not reset password."})
				return
			}
			passwordHash = string(hash)
		}
		current, _ := currentAccount(r)
		auth.Lock()
		defer auth.Unlock()
		users := append([]account(nil), auth.credentials.Users...)
		index := -1
		for i := range users {
			if strings.EqualFold(users[i].Email, email) {
				index = i
				break
			}
		}
		if index < 0 {
			writeJSON(w, http.StatusNotFound, apiError{"Account not found."})
			return
		}
		target := &users[index]
		if strings.EqualFold(target.Email, current.Email) && ((req.Role != "" && req.Role != "admin") || (req.Disabled != nil && *req.Disabled)) {
			writeJSON(w, http.StatusConflict, apiError{"You cannot disable or demote your own administrator account."})
			return
		}
		if req.Role != "" {
			target.Role = req.Role
		}
		if req.Disabled != nil {
			target.Disabled = *req.Disabled
		}
		if passwordHash != "" {
			target.PasswordHash = passwordHash
		}
		if !hasActiveAdmin(users) {
			writeJSON(w, http.StatusConflict, apiError{"At least one active administrator must remain."})
			return
		}
		if err := saveCredential(credentialDB{Users: users}); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{"Could not update account."})
			return
		}
		writeJSON(w, http.StatusOK, publicAdminUser(*target))
	case http.MethodDelete:
		current, _ := currentAccount(r)
		if strings.EqualFold(current.Email, email) {
			writeJSON(w, http.StatusConflict, apiError{"You cannot delete your own account."})
			return
		}
		auth.Lock()
		defer auth.Unlock()
		users := make([]account, 0, len(auth.credentials.Users))
		found := false
		for _, user := range auth.credentials.Users {
			if strings.EqualFold(user.Email, email) {
				found = true
				continue
			}
			users = append(users, user)
		}
		if !found {
			writeJSON(w, http.StatusNotFound, apiError{"Account not found."})
			return
		}
		if !hasActiveAdmin(users) {
			writeJSON(w, http.StatusConflict, apiError{"At least one active administrator must remain."})
			return
		}
		if err := saveCredential(credentialDB{Users: users}); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{"Could not remove account."})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		w.Header().Set("Allow", "PATCH, DELETE")
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"Method not allowed. Use PATCH or DELETE."})
	}
}

func hasActiveAdmin(users []account) bool {
	for _, user := range users {
		if user.Role == "admin" && !user.Disabled {
			return true
		}
	}
	return false
}
