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

### Issue: Transcription Fails with "Audio file might be corrupted or unsupported"

**Symptoms:**
- Job status: `failed`
- Error message: `OpenAI API error (status 400): Audio file might be corrupted or unsupported`
- Logs show: "failed to transcribe chunk X"
- Chunk has very short duration (< 1 second)

**Root Cause:**
Audio files with duration slightly exceeding threshold (e.g., 300.024s > 300s) trigger splitting, and FFmpeg creates tiny remainder chunks at the end:
- Audio: 5 minutes 0.024 seconds (300.024s)
- Chunk splitting: 5-minute chunks
- Result:
  - chunk_000.mp3: 300 seconds ✅
  - chunk_001.mp3: 0.024 seconds ❌ (too short for OpenAI)

OpenAI API rejects audio chunks shorter than ~1 second.

**Solution:**

**Already Fixed in Code** (commit `eb763c3`):
- Added `MinChunkDurationSeconds = 1.0` filter
- Chunks < 1 second automatically filtered and removed
- No manual intervention needed for new transcriptions

**For Old Failed Jobs:**

1. Check job error details:
   ```bash
   ssh likshing "sqlite3 /opt/scriberr/data/scriberr.db \
     \"SELECT id, status, error_message FROM transcription_jobs WHERE status='failed' LIMIT 5;\""
   ```

2. Identify chunk-related failures:
   ```bash
   # Look for "failed to transcribe chunk" errors
   ssh likshing "journalctl -u scriberr --since '24 hours ago' | grep 'failed to transcribe chunk'"
   ```

3. Retry failed job:
   ```bash
   # Reset job to pending status
   ssh likshing "sqlite3 /opt/scriberr/data/scriberr.db \
     \"UPDATE transcription_jobs SET status='pending', error_message=NULL WHERE id='JOB_ID';\""
   
   # Restart service to pick up pending job
   ssh likshing "sudo systemctl restart scriberr"
   ```

4. Monitor transcription progress:
   ```bash
   # Watch logs
   ssh likshing "journalctl -u scriberr -f | grep -E 'chunk|completed|failed'"
   
   # Check job status
   ssh likshing "sqlite3 /opt/scriberr/data/scriberr.db \
     \"SELECT id, status, length(transcript) FROM transcription_jobs WHERE id='JOB_ID';\""
   ```

**Verification:**
- Logs should show: "Skipping chunk that is too short"
- Valid chunks processed successfully
- Job status changes to: `processing` → `completed`
- Transcript appears in UI

**Example Log Output (Fixed):**
```
time=17:32:17 level="WARN " msg="Skipping chunk that is too short" 
  chunk=data/temp/.../chunk_001.mp3 duration_sec=0.072 min_duration_sec=1
time=17:32:17 level="INFO " msg="Audio split complete" 
  total_chunks=2 valid_chunks=1 chunk_duration_sec=300
time=17:34:13 level="INFO " msg="Model processing completed" 
  model_id=openai_whisper processing_time=1m55.798602134s
```

**Prevention:**
- Fix is already in production code
- Automatically handles edge cases near duration thresholds
- No configuration changes needed

## Testing Checklist After Deployment

1. **Service Status**: `ssh likshing "systemctl status scriberr"`
2. **Database**: `ssh likshing "sqlite3 /opt/scriberr/data/scriberr.db 'SELECT COUNT(*) FROM users;'"`
3. **Login**: Test via web UI at https://scriberr.hachitg4ever.com/login
4. **API Health**: `curl -s https://scriberr.hachitg4ever.com/api/v1/auth/registration-status`
