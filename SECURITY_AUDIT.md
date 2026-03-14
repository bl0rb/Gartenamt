# Security Audit Report

**Generated:** 14. März 2026  
**Project:** Kleingarten-Verwaltung  
**Status:** Multiple security and code quality issues identified

## Executive Summary

The application has a solid foundation with good use of parameterized queries to prevent SQL injection and reasonable session management. However, there are **critical security issues** that must be addressed before production deployment:

- ⚠️ **CRITICAL:** Hardcoded default admin password, CSRF vulnerability, open redirect vulnerability
- 🔴 **HIGH:** In-memory session storage, weak login rate limiting, error handling gaps
- 🟠 **MEDIUM:** Input validation issues, file upload content validation missing
- 🟡 **LOW:** Best practices violations, configuration concerns

---

## 🚨 Critical Issues (Must Fix)

### 1. Default Admin Password Hardcoded
**File:** `services/auth_service.go` (line 142)  
**Severity:** CRITICAL  
**Issue:** Default admin password `"admin123"` is hardcoded and widely known

**Recommendation:**
```go
// TODO: Implement one of the following:
// 1. Generate random password on first run
// 2. Start with no admin user - require setup wizard
// 3. Hash a strong default and log it to console on first run
```

### 2. CSRF Token Missing from Login
**File:** `handlers/auth_handler.go` (line 28-45)  
**Severity:** CRITICAL  
**Issue:** No CSRF token validation on login POST request

**Fix:**
```bash
go get github.com/gorilla/csrf
```

Then add CSRF middleware:
```go
import "github.com/gorilla/csrf"

CSRF := csrf.Protect([]byte("32-character-auth-key"))
r.Use(CSRF)
```

### 3. Open Redirect Vulnerability
**File:** `handlers/auth_handler.go` (line 61)  
**Severity:** CRITICAL  
**Issue:** Login redirect uses weak validation: `strings.Contains(redirectURL, "login")`

This allows redirects to external URLs like `https://evil.com/?login=true`

**Fix:**
```go
// Whitelist allowed redirect URLs
allowedRedirects := map[string]bool{
    "/":         true,
    "/parzellen": true,
    "/profile":   true,
}

if !allowedRedirects[redirectURL] {
    redirectURL = "/" // Default to home
}
```

---

## 🔴 High Priority Issues

### 1. Sessions Stored in Memory Only
**File:** `services/auth_service.go`  
**Severity:** HIGH  
**Issue:** Sessions lost on application restart; not suitable for production with multiple instances

**Fix:** Migrate to persistent storage
- Option A: Store sessions in SQLite database
- Option B: Use Redis for session store
- Option C: Use JWT tokens (stateless)

### 2. Session Timeout Too Long
**File:** `services/auth_service.go` (line 73)  
**Severity:** HIGH  
**Issue:** 24-hour session timeout is too long for an admin tool

**Recommendation:** Reduce to 2 hours maximum, add "Remember Me" option

### 3. No Rate Limiting on Login
**File:** `handlers/auth_handler.go`  
**Severity:** HIGH  
**Issue:** Allows brute force attacks on user passwords (no XPA/XPS attempts threshold)

**Fix:** Add rate limiting middleware
```go
// Implement per-IP rate limiter
// Limit to 5 failed login attempts per 15 minutes
```

### 4. Error Handling Issues
**Severity:** HIGH  
**Files with ignored errors:**
- `handlers/admin_handler.go` (line 26-27): `filepath.Glob()` errors ignored
- `handlers/admin_handler.go` (line 244-248): SQL `Scan()` errors ignored  
- `models/admin_delete_models.go` (line 90): `json.Unmarshal()` errors ignored
- `models/parzelle.go` (line 30): No transaction rollback on failure

**Fix:** Add explicit error handling for all operations:
```go
// Bad:
result, _ := models.DB.Exec(sql)

// Good:
result, err := models.DB.Exec(sql)
if err != nil {
    log.Printf("Error executing SQL: %v", err)
    http.Error(w, "Database error", http.StatusInternalServerError)
    return
}
```

---

## 🟠 Medium Priority Issues

### 1. Insecure Cookie Flags
**File:** `middleware/auth_middleware.go` (line 92)  
**Severity:** MEDIUM  
**Issue:** `Secure` flag set to `false` in production

**Fix:**
```go
cookie := &http.Cookie{
    Name:     "session_id",
    Value:    session.ID,
    Path:     "/",
    HttpOnly: true,
    Secure:   os.Getenv("ENV") == "production", // Set based on environment
    SameSite: http.SameSiteLaxMode,
    MaxAge:   7200, // 2 hours
}
```

### 2. CSV File Validation Insufficient
**File:** `handlers/admin_csv_handler.go` (line 135)  
**Severity:** MEDIUM  
**Issue:** Only file extension checked (`.csv`), not actual content

**Fix:**
```go
// Validate actual file content, not just extension
// Check first few bytes for CSV format
// Or use `http.DetectContentType()` 
content := make([]byte, 512)
n, _ := file.Read(content)
contentType := http.DetectContentType(content[:n])
if contentType != "text/plain" && !strings.Contains(contentType, "text") {
    return errors.New("invalid file content")
}
```

### 3. Input Validation Missing
**File:** `handlers/admin_handler.go` (line 127-128)  
**Severity:** MEDIUM  
**Issue:** Form values converted without validation

```go
// Bad:
standardpreis, _ := strconv.ParseFloat(r.FormValue("standardpreis"), 64)

// Good:
priceStr := strings.TrimSpace(r.FormValue("standardpreis"))
if priceStr == "" {
    return errors.New("price is required")
}
standardpreis, err := strconv.ParseFloat(priceStr, 64)
if err != nil || standardpreis < 0 {
    return errors.New("invalid price format")
}
```

### 4. Email/Phone Validation Missing
**File:** `handlers/parzelle_handler.go` (line 28-35)  
**Severity:** MEDIUM  
**Issue:** Email and phone inserted into database without validation

**Fix:**
```go
import "regexp"

emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
if !emailRegex.MatchString(email) && email != "" {
    return errors.New("invalid email format")
}

phoneRegex := regexp.MustCompile(`^\+?[0-9\s()\-]{7,}$`)
if !phoneRegex.MatchString(phone) && phone != "" {
    return errors.New("invalid phone format")
}
```

### 5. Database Connection Pooling Not Configured
**File:** `models/database.go` (line 14-20)  
**Severity:** MEDIUM  
**Issue:** No explicit connection pool configuration

**Fix:**
```go
DB.SetMaxOpenConns(25)      // For SQLite, typically 1-5
DB.SetMaxIdleConns(5)
DB.SetConnMaxLifetime(5 * time.Minute)
```

### 6. File Path Traversal Risk
**File:** `handlers/admin_csv_handler.go` (line 177)  
**Severity:** MEDIUM  
**Issue:** Filepath concatenation for backup downloads without sanitization

**Fix:**
```go
// Sanitize filename to prevent path traversal
filename := filepath.Base(requestedFile) // Remove any path components
filepath.Join("exports", filename)        // Safely construct path
```

---

## 🟡 Low & Medium Best Practices

### Global Variables
- `models/database.go`: Global `DB` variable
- `services/auth_service.go`: Global `GlobalAuth` variable

**Recommendation:** Use dependency injection for better testability

### Error Panic Patterns
**File:** `handlers/auth_handler.go` (line 101-104)  
```go
// Bad:
template.Must(template.ParseFiles("templates/login.html"))

// Good:
tmpl, err := template.ParseFiles("templates/login.html")
if err != nil {
    log.Printf("Template parse error: %v", err)
    http.Error(w, "Internal server error", http.StatusInternalServerError)
    return
}
```

### Inconsistent Error Handling
- **`services/csv_service.go` (line 151):** JSON verification via string contains instead of unmarshaling
- **`models/admin_delete_models.go` (line 85-100):** Multiple time.Parse attempts without context

### Silent Failures
**File:** `services/berechnungs_service.go` (line 150)  
**Issue:** Fallback hardcoded prices when query fails - silently degrades functionality

---

## ✅ What's Done Well

1. ✅ **SQL Injection Prevention** - Consistently uses parameterized queries
2. ✅ **Password Hashing** - Uses bcrypt for password storage
3. ✅ **Session Structure** - Good session design with metadata (IP, UserAgent)
4. ✅ **RBAC Implementation** - Clear role-based access control
5. ✅ **Middleware Pattern** - Clean middleware for auth checks
6. ✅ **Audit Logging** - Administrative actions logged

---

## 📋 Remediation Checklist

### Immediate (Before Production)
- [ ] Change default admin password
- [ ] Add CSRF token middleware to all POST endpoints
- [ ] Fix open redirect vulnerability in auth_handler
- [ ] Add rate limiting on login endpoint
- [ ] Fix all ignored errors in handlers and models
- [ ] Add input validation for all form fields
- [ ] Enable Secure flag on cookies for production

### Important (Within 1 Sprint)
- [ ] Implement persistent session storage
- [ ] Add file upload content validation
- [ ] Reduce session timeout to 2 hours
- [ ] Add email/phone format validation
- [ ] Fix database connection pool configuration
- [ ] Add HTTPS redirect configuration

### Short Term (Within 2 Sprints)
- [ ] Migrate to dependency injection
- [ ] Replace `template.Must()` with error handling
- [ ] Move time parsing to SQL layer
- [ ] Implement structured security logging
- [ ] Add content security headers

### Long Term
- [ ] Consider JWT tokens instead of session state
- [ ] Implement database migrations framework
- [ ] Add comprehensive test suite
- [ ] Set up CI/CD security scanning
- [ ] Consider distributed session support

---

## 🔒 Deployment Recommendations

### Pre-Deployment Checklist
1. Run all security fixes listed above
2. Set environment variable: `ENV=production`
3. Enable HTTPS/TLS with valid certificate
4. Configure firewall rules (restrict admin routes by IP if possible)
5. Set up regular backups
6. Enable audit logging to persistent storage
7. Change default database location outside project root
8. Use environment variables for configuration

### Ongoing
- Monitor audit logs regularly
- Keep Go and dependencies updated
- Run periodic security audits
- Implement WAF (Web Application Firewall) if public-facing

---

## Reference

- Go Security Best Practices: https://golang.org/doc/effective_go
- OWASP Top 10: https://owasp.org/www-project-top-ten/
- Gorilla CSRF: https://github.com/gorilla/csrf
- Session Security: https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html
