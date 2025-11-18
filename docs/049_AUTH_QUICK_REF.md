# Authentication Quick Reference

## Setup

1. **Configure JWT Secret**
   ```bash
   cd /home/vishv/websites/cloned/CloudAI/master
   cp .env.example .env
   # Edit .env and set JWT_SECRET to a secure random string
   ```

2. **Start the System**
   ```bash
   ./runMaster.sh
   ```

## API Endpoints

| Method | Endpoint | Auth Required | Description |
|--------|----------|---------------|-------------|
| POST | `/api/auth/register` | No | Register new user |
| POST | `/api/auth/login` | No | Login user |
| POST | `/api/auth/logout` | No | Logout user |
| GET | `/api/auth/me` | Yes | Get current user |

## Frontend Routes

| Route | Access | Description |
|-------|--------|-------------|
| `/login` | Public | Login page |
| `/register` | Public | Registration page |
| `/dashboard` | Protected | Dashboard with welcome message |
| `/tasks` | Protected | Tasks page |
| `/workers` | Protected | Workers page |
| `/submit` | Protected | Submit task page |

## User Flow

```
1. Visit http://localhost:5173
2. Redirected to /login (not authenticated)
3. Click "Register here"
4. Fill: Name, Email, Password, Confirm Password
5. Auto-login after registration
6. See dashboard: "Welcome back, {name}!"
7. Visit count: "You've logged in 1 time"
8. Click logout in sidebar
9. Redirected to /login
10. Login again
11. Visit count: "You've logged in 2 times"
```

## Testing Credentials

Try creating:
```
Name: Admin User
Email: admin@cloudai.com
Password: admin123
```

## Key Features

✅ **Bcrypt password hashing** - Passwords encrypted with cost 10  
✅ **JWT tokens in httpOnly cookies** - Secure, not accessible via JS  
✅ **Visit count tracking** - Increments on every login  
✅ **Protected routes** - All pages require authentication  
✅ **User info in Dashboard** - Shows name and visit count  
✅ **Logout functionality** - Clears cookie and redirects  
✅ **CORS with credentials** - Allows cookies from frontend  

## MongoDB Collection

```
Database: cloudai
Collection: USERS
Schema: {
  email: string (unique),
  name: string,
  password_hash: string (bcrypt),
  visit_count: int,
  created_at: timestamp,
  updated_at: timestamp
}
```

## Security Notes

- Passwords are never stored in plain text
- JWT tokens expire after 24 hours
- Cookies are httpOnly and SameSite=Lax
- CORS restricted to http://localhost:5173
- Password minimum length: 6 characters
- Email must be unique

## Troubleshooting

**401 Unauthorized Error:**
- Check if JWT_SECRET is set in .env
- Verify MongoDB is running
- Check cookie is being sent (Network tab)

**Cannot login:**
- Verify email and password are correct
- Check MongoDB connection
- Ensure bcrypt comparison is working

**Redirect loop:**
- Clear browser cookies
- Check AuthContext is working
- Verify ProtectedRoute logic

**Visit count not incrementing:**
- Check IncrementVisitCount is called in HandleLogin
- Verify MongoDB update is successful
- Check user data in response

---

**Quick Commands:**

```bash
# View MongoDB users
mongosh cloudai
db.USERS.find().pretty()

# Check cookies in browser
# DevTools → Application → Cookies → http://localhost:5173

# Test login API
curl -c cookies.txt -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@cloudai.com","password":"admin123"}'

# Test protected endpoint
curl -b cookies.txt http://localhost:8080/api/auth/me
```
