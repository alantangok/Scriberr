# Deployment and Common Issues

## Deployment Best Practices

### Always Use deploy.sh Script
- **Command**: `./deploy.sh` (defaults to full deployment)
- **What it does**: Builds binary, builds frontend, creates backups, deploys both, restarts service
- **Never** manually deploy with `scp` - causes sync issues between frontend/backend

### Deployment Modes
- `./deploy.sh` or `./deploy.sh --full` - Deploy both binary and frontend (DEFAULT)
- `./deploy.sh --binary-only` - Deploy only Go binary
- `./deploy.sh --frontend-only` - Deploy only React frontend
- `./deploy.sh --rollback` - Rollback to previous backup

## Common Issues and Solutions

### Issue: Login Returns 401 Unauthorized

**Symptoms:**
- Login form stuck on "Signing in..."
- Console shows: `POST /api/v1/auth/login 401`
- Multiple failed `/api/v1/auth/refresh` attempts

**Root Causes:**
1. **Corrupt or empty database** - Database file is 0 bytes or missing users table
2. **Password hash mismatch** - Stored hash doesn't match expected password
3. **Stale frontend** - Frontend code out of sync with backend API

**Solution:**
1. Check database exists and has data:
   ```bash
   ssh likshing "ls -lh /opt/scriberr/data/scriberr.db"
   ssh likshing "sqlite3 /opt/scriberr/data/scriberr.db 'SELECT id, username FROM users;'"
   ```

2. If database is corrupt or users table is empty, use registration API to create user:
   ```bash
   ssh likshing 'curl -s -X POST http://localhost:8080/api/v1/auth/register \
     -H "Content-Type: application/json" \
     -d '"'"'{"username":"admin","password":"admin123","confirmPassword":"admin123"}'"'"''
   ```

3. Deploy full stack to ensure sync:
   ```bash
   ./deploy.sh --full
   ```

### Issue: Need to Update Admin Password

**Solution:**

1. Generate bcrypt hash locally:
   ```bash
   # Create hash generator (one-time)
   cat > /tmp/hash_password.go << 'EOF'
   package main
   import (
       "fmt"
       "os"
       "golang.org/x/crypto/bcrypt"
   )
   func main() {
       password := os.Args[1]
       hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
       if err != nil {
           fmt.Println("Error:", err)
           os.Exit(1)
       }
       fmt.Println(string(hash))
   }
   EOF

   # Generate hash
   go run /tmp/hash_password.go 'YourNewPassword'
   ```

2. Update password in database:
   ```bash
   # Save hash to file (avoids shell escaping issues)
   echo '$2a$10$...' > /tmp/pw_hash.txt
   scp /tmp/pw_hash.txt likshing:/tmp/
   
   # Update in database
   ssh likshing "cd /opt/scriberr && HASH=\$(cat /tmp/pw_hash.txt) && \
     sqlite3 data/scriberr.db \"UPDATE users SET password='\${HASH}' WHERE username='admin';\""
   ```

3. Verify hash was saved correctly:
   ```bash
   ssh likshing "sqlite3 /opt/scriberr/data/scriberr.db \
     'SELECT username, length(password), substr(password,1,20) FROM users WHERE username=\"admin\";'"
   # Should show: admin|60|$2a$10$...
   ```

4. Test login:
   ```bash
   # Create test JSON
   cat > /tmp/test_login.json << 'EOF'
   {"username":"admin","password":"YourNewPassword"}
   EOF
   
   # Test via API
   scp /tmp/test_login.json likshing:/tmp/
   ssh likshing 'curl -s -X POST http://localhost:8080/api/v1/auth/login \
     -H "Content-Type: application/json" -d @/tmp/test_login.json'
   # Should return: {"token":"...","user":{...}}
   ```

**Important Notes:**
- Bcrypt hash length should always be 60 characters
- Hash starts with `$2a$10$` or similar bcrypt identifier
- Use files to transfer hashes to avoid shell escaping issues with special characters
- Verify the hash locally before deploying: `go run /tmp/test_password.go 'password' '$2a$10$...'`

### Issue: Database Corruption

**Symptoms:**
- `sqlite3: no such table: users`
- Database file is 0 bytes
- Service starts but can't authenticate

**Solution:**
1. Stop service and remove corrupt DB:
   ```bash
   ssh likshing "sudo systemctl stop scriberr && rm -f /opt/scriberr/data/scriberr.db"
   ```

2. Restart service (auto-creates new DB):
   ```bash
   ssh likshing "sudo systemctl start scriberr"
   ```

3. Create admin user via registration API (see above)

## Database Location

- **Old location** (deprecated): `/opt/scriberr/scriberr.db`
- **Current location**: `/opt/scriberr/data/scriberr.db`

Always check both locations when troubleshooting.

## Testing Checklist After Deployment

1. **Service Status**: `ssh likshing "systemctl status scriberr"`
2. **Database**: `ssh likshing "sqlite3 /opt/scriberr/data/scriberr.db 'SELECT COUNT(*) FROM users;'"`
3. **Login**: Test via web UI at https://scriberr.hachitg4ever.com/login
4. **API Health**: `curl -s https://scriberr.hachitg4ever.com/api/v1/auth/registration-status`
