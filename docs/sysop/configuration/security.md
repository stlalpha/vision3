# ViSiON/3 Security Guide

This guide covers security features and best practices for protecting your BBS.

## Table of Contents

- [Connection Security](#connection-security)
- [IP Filtering](#ip-filtering)
- [Access Control](#access-control)
- [Best Practices](#best-practices)

## Connection Security

ViSiON/3 includes built-in connection management to prevent resource exhaustion and abuse.

### Configuration Options

Edit `configs/config.json`:

```json
{
  "maxNodes": 10,
  "maxConnectionsPerIP": 3,
  "ipBlocklistPath": "configs/blocklist.txt",
  "ipAllowlistPath": "configs/allowlist.txt"
}
```

### Max Nodes

Limits the total number of simultaneous connections to your BBS.

- **Default:** 10
- **Range:** 1-999 (0 = unlimited, not recommended)
- **Purpose:** Prevents server overload from too many connections

**Example:**

```json
{
  "maxNodes": 5
}
```

This allows only 5 users to be connected at the same time. When the limit is reached, new connections receive:

```
Connection rejected: maximum nodes reached
Please try again later.
```

### Max Connections Per IP

Limits how many simultaneous connections a single IP address can make.

- **Default:** 3
- **Range:** 1-99 (0 = unlimited, not recommended)
- **Purpose:** Prevents connection flooding from a single source

**Example:**

```json
{
  "maxConnectionsPerIP": 2
}
```

This allows each IP address to maintain up to 2 concurrent connections. Useful for:

- Preventing multi-connection abuse
- Limiting resource usage per user
- Mitigating simple DoS attempts

## IP Filtering

Control which IP addresses can connect to your BBS using blocklists and allowlists.

### Blocklist

Block specific IPs or IP ranges from connecting.

**Setup:**

1. Create `configs/blocklist.txt`:

   ```text
   # Blocked IPs - one per line
   # Comments start with #

   # Block specific troublemakers
   192.168.1.100
   10.0.0.50

   # Block entire subnets
   192.168.100.0/24
   172.16.0.0/16

   # Block known bot networks
   185.220.100.0/22

   # IPv6 support
   2001:db8::bad:1
   2001:db8::/32
   ```

2. Update `configs/config.json`:

   ```json
   {
     "ipBlocklistPath": "configs/blocklist.txt"
   }
   ```

3. Restart the BBS

**When to use:**

- Ban abusive users
- Block known bot networks
- Prevent access from problematic IP ranges
- Comply with regional restrictions

### Allowlist

Allow specific IPs to bypass all connection limits.

**Setup:**

1. Create `configs/allowlist.txt`:

   ```text
   # Allowed IPs - bypass all limits
   # Comments start with #

   # Always allow localhost
   127.0.0.1
   ::1

   # Allow your admin IP
   203.0.113.42

   # Allow your entire office network
   192.168.1.0/24

   # IPv6 examples
   2001:db8:admin::/48
   ```

2. Update `configs/config.json`:

   ```json
   {
     "ipAllowlistPath": "configs/allowlist.txt"
   }
   ```

3. Restart the BBS

**When to use:**

- Ensure admin access is never blocked
- Exempt trusted networks from rate limits
- Provide unrestricted access to Co-SysOps
- Allow testing/monitoring services

### File Format

Both blocklist and allowlist use the same simple format:

```text
# Comments start with # (entire line)
# Blank lines are ignored

# Individual IPv4 addresses
192.168.1.100
10.0.0.50

# IPv4 CIDR ranges
192.168.100.0/24    # Class C network (256 addresses)
172.16.0.0/16       # Class B network (65,536 addresses)
10.0.0.0/8          # Class A network (16,777,216 addresses)

# Individual IPv6 addresses
2001:db8::1
fe80::1

# IPv6 CIDR ranges
2001:db8::/32
fe80::/10
```

**Notes:**

- One IP or CIDR range per line
- Whitespace is trimmed
- Invalid entries are logged and skipped
- Files are loaded at startup and **automatically reloaded** when changed (no restart required)

### Auto-Reload Feature

The BBS **automatically watches** blocklist.txt and allowlist.txt for changes and reloads them on the fly.

**How it works:**

1. Files are monitored using file system watching (fsnotify)
2. When you save changes to either file, the BBS detects it within seconds
3. Lists are reloaded automatically (with 500ms debounce to handle rapid edits)
4. Changes take effect immediately for new connection attempts
5. No BBS restart needed - zero downtime

**Benefits:**

- ✅ **Respond to attacks immediately** - block IPs without disrupting users
- ✅ **No downtime** - no need to restart the BBS
- ✅ **Easy testing** - edit, save, test immediately
- ✅ **Safe updates** - existing connections remain active

**Example workflow:**

```bash
# 1. Edit the blocklist
vim configs/blocklist.txt
# Add: 192.0.2.100

# 2. Save the file
# (Auto-reload happens within 500ms)

# 3. Verify in logs
tail -f data/logs/vision3.log
# You'll see:
# DEBUG: File change detected: configs/blocklist.txt (WRITE)
# INFO: Reloading IP filter lists...
# INFO: Blocklist reloaded from configs/blocklist.txt

# 4. IP is now blocked immediately
# New connections from 192.0.2.100 will be rejected
```

### Priority Order

Connection requests are evaluated in this order:

1. **Allowlist Check**
   - If IP is on allowlist → **Accept immediately** (bypass all other checks)

2. **Blocklist Check**
   - If IP is on blocklist → **Reject**

3. **Max Nodes Check**
   - If total connections ≥ maxNodes → **Reject**

4. **Per-IP Limit Check**
   - If connections from this IP ≥ maxConnectionsPerIP → **Reject**

5. **Accept Connection**

**Key Points:**

- Allowlist overrides everything (including blocklist)
- Blocklist overrides connection limits
- Allowlisted IPs are never rate-limited

### Real-World Examples

#### Example 1: Public BBS with Admin Protection

```json
{
  "maxNodes": 20,
  "maxConnectionsPerIP": 3,
  "ipBlocklistPath": "configs/blocklist.txt",
  "ipAllowlistPath": "configs/allowlist.txt"
}
```

`configs/allowlist.txt`:

```text
# Admin home IP
203.0.113.42

# Admin work IP
198.51.100.10
```

`configs/blocklist.txt`:

```text
# Known bad actors
192.0.2.100
192.0.2.200

# Datacenter ranges with bots
185.220.100.0/22
```

**Result:**

- Admin IPs never rate-limited or blocked
- Known bad IPs blocked
- Everyone else limited to 3 connections
- Max 20 total users

#### Example 2: Private BBS (Members Only)

```json
{
  "maxNodes": 10,
  "maxConnectionsPerIP": 2,
  "ipAllowlistPath": "configs/allowlist.txt"
}
```

`configs/allowlist.txt`:

```text
# Member 1
203.0.113.1

# Member 2
203.0.113.2

# Member network
192.168.1.0/24
```

**Result:**

- Only allowlisted IPs can connect
- Everyone else is rejected (empty allowlist = accept all, but add at least one entry)
- Actually, this won't work as intended - allowlist doesn't reject others

**Better approach for private BBS:**
Use allowlist for admins only, and rely on authentication + monitoring for access control.

#### Example 3: Dealing with Attack

During an attack from `192.0.2.0/24`:

1. Quickly add to blocklist:

   ```bash
   echo "192.0.2.0/24" >> configs/blocklist.txt
   ```

2. Changes apply **automatically within seconds** (no restart needed):

   ```bash
   # Watch the logs to confirm reload
   tail -f data/logs/vision3.log
   # You'll see:
   # INFO: Reloading IP filter lists...
   # INFO: Blocklist reloaded from configs/blocklist.txt
   ```

3. Monitor blocked connections:

   ```bash
   tail -f data/logs/vision3.log | grep "192.0.2"
   # You'll see:
   # INFO: Rejecting SSH connection from 192.0.2.x: IP address is blocked
   ```

### Logging

IP filtering actions are logged:

```
INFO: IP blocklist enabled from configs/blocklist.txt (auto-reload on file change)
INFO: Watching configs/blocklist.txt for changes (auto-reload enabled)
INFO: IP allowlist enabled from configs/allowlist.txt (auto-reload on file change)
INFO: Watching configs/allowlist.txt for changes (auto-reload enabled)
DEBUG: File change detected: configs/blocklist.txt (WRITE)
INFO: Reloading IP filter lists...
INFO: Blocklist reloaded from configs/blocklist.txt
INFO: Rejecting Telnet connection from 192.0.2.100: IP address is blocked
DEBUG: IP 203.0.113.42 is on allowlist, bypassing all checks
```

### Troubleshooting

**Problem:** Changes to blocklist/allowlist don't apply immediately

**Solution:** Auto-reload should happen within seconds. Check the logs for "Reloading IP filter lists..." message. If you don't see it:
- Ensure the file was saved properly
- Check file permissions (must be readable by the BBS process)
- Look for file watcher errors in the logs

---

**Problem:** Allowlisted IP still getting rate-limited

**Solution:** Check the logs. Ensure:

- File path is correct in config.json
- IP format is valid (no typos, valid CIDR notation)
- Check for reload confirmation in logs

---

**Problem:** Can't connect from any IP

**Solution:**

- Check if you accidentally blocked your own IP
- Verify file format (no typos, valid CIDR notation)
- Add your IP to allowlist temporarily

---

**Problem:** Invalid IP warning in logs

**Solution:** Check the line number in the warning:

```
WARN: Invalid CIDR in configs/blocklist.txt line 15: 192.168.1.1/33
```

CIDR mask must be 0-32 for IPv4, 0-128 for IPv6.

## Access Control

### Security Levels

ViSiON/3 uses numeric security levels for access control:

```json
{
  "sysOpLevel": 255,
  "coSysOpLevel": 250,
  "logonLevel": 100,
  "anonymousLevel": 5
}
```

- **sysOpLevel (255):** Full system access
- **coSysOpLevel (250):** Co-SysOp privileges
- **logonLevel (100):** Standard user after login
- **anonymousLevel (5):** Guest/unvalidated users

### Authentication Lockout

Protect against brute-force attacks with automatic **IP-based** lockout after failed login attempts.

**Configuration** (`configs/config.json`):

```json
{
  "maxFailedLogins": 5,
  "lockoutMinutes": 30
}
```

- **maxFailedLogins:** Number of failed login attempts from a single IP before lockout (0 = disabled)
- **lockoutMinutes:** Duration the IP remains locked after threshold is reached

**How it works:**

1. Each failed login attempt from an IP address is tracked in memory
2. After reaching the threshold, the IP is locked out
3. During lockout, login attempts from that IP show:
   ```
   Too many failed login attempts from your IP.
   Please try again in X minutes.
   ```
4. Successful login from an IP clears the failed attempt counter for that IP
5. Lockout automatically expires after the configured time

**Why IP-based instead of user-based?**

IP-based lockout prevents **Denial of Service (DoS)** attacks where an attacker repeatedly tries to log in to legitimate user accounts to lock them out. With IP-based lockout:
- Attackers lock themselves out, not your users
- Multiple users behind the same IP (like a NAT) share a counter (use higher limits or allowlist trusted IPs)
- Better suited for BBS/community systems where user accounts are valuable

**Persistence:**

IP lockout data is held **in memory only** (not persisted to disk). This means:
- Lockouts are cleared on BBS restart
- No permanent lockout records
- Fast in-memory lookups
- Lockouts automatically expire based on timestamp, even without restart

**Logging:**

All authentication events are logged:
```
SECURITY: Node 3: Failed authentication attempt for user: johndoe from IP: 203.0.113.42
SECURITY: Node 3: IP 203.0.113.42 has been locked out after too many failed attempts
SECURITY: Node 3: Login attempt from locked IP 203.0.113.42 (locked until 2026-02-11 14:30:00, 5 attempts)
DEBUG: Node 3: Cleared failed login attempts for IP 203.0.113.42
```

**Recommendations:**

- **Public BBS:** `maxFailedLogins: 5`, `lockoutMinutes: 30`
- **High Security:** `maxFailedLogins: 3`, `lockoutMinutes: 60`
- **Development:** `maxFailedLogins: 0` (disabled)
- **Shared IPs (NAT):** Consider higher limits or use IP allowlist for trusted networks

**Manual unlock:**

To manually unlock an IP, simply restart the BBS (lockouts are in-memory only). Alternatively, wait for the lockout duration to expire — the system automatically allows logins after the configured time.

### SSH Authentication

ViSiON/3 implements a **two-tier authentication design** that balances BBS usability (supporting new user registration) with security (rate limiting and lockout tracking).

#### How SSH Authentication Works

**For existing users:**
- SSH password is validated against the bcrypt hash in `data/users/users.json`
- If correct, user is automatically logged in to the BBS
- If incorrect, SSH authentication fails at the protocol level

**For unknown users (not in database):**
- SSH authentication **intentionally allows** the connection through
- User then goes through the normal BBS login/new user registration flow
- This allows new users to create accounts while still providing security

#### Security Considerations

**Why allow unknown users through SSH auth?**

This design enables the classic BBS new user experience while maintaining security through application-level controls:

1. **Connection-level protection:**
   - IP blocklist/allowlist filtering (see [IP Filtering](#ip-filtering))
   - Per-IP connection limits (see [Max Connections Per IP](#max-connections-per-ip))
   - Total node limits (see [Max Nodes](#max-nodes))

2. **Authentication-level protection:**
   - **IP-based lockout** after failed BBS login attempts (see [Authentication Lockout](#authentication-lockout))
   - Failed attempts tracked at application layer, not SSH layer
   - Prevents brute force attacks at the BBS login prompt

3. **Why not reject unknown users at SSH level?**
   - Would prevent new user registration entirely
   - Would require all accounts to be created by SysOp
   - SSH brute force attacks are mitigated by IP-based lockout at BBS layer

**Attack mitigation:**

The system protects against SSH brute force attempts through:
- **IP connection limits:** Prevents connection flooding
- **Failed login tracking:** After `maxFailedLogins` failed BBS login attempts from an IP, that IP is locked out for `lockoutMinutes`
- **IP blocklist:** Persistent IPs can be permanently blocked
- **Monitoring:** All authentication attempts are logged for analysis

**Important:** While SSH protocol auth allows unknown users through, the BBS login layer provides the actual security enforcement. An attacker repeatedly trying invalid credentials will:
1. Be allowed to connect (SSH auth succeeds)
2. Reach the BBS login prompt
3. Fail BBS authentication multiple times
4. Trigger IP-based lockout (default: 5 attempts = 30-minute lockout)
5. Be unable to make further login attempts from that IP

#### Configuration Example

For a secure public BBS that allows new users:

```json
{
  "maxFailedLogins": 5,
  "lockoutMinutes": 30,
  "maxConnectionsPerIP": 3,
  "maxNodes": 20
}
```

This configuration:
- Allows new users to register via SSH
- Limits each IP to 3 concurrent connections
- Locks out IPs after 5 failed BBS logins for 30 minutes
- Prevents resource exhaustion with 20 total nodes max

#### SSH Key Authentication (Optional)

For admin accounts, consider using SSH key-only authentication by configuring your SSH client:

```bash
# In ~/.ssh/config
Host mybbs.example.com
    User sysop
    IdentityFile ~/.ssh/bbs_sysop_key
    PasswordAuthentication no
```

However, password authentication is still needed for regular users to support the standard BBS login flow.

#### Monitoring Recommendations

Monitor authentication attempts regularly:

```bash
# Watch for suspicious patterns
grep "Failed authentication" data/logs/vision3.log | tail -50

# Check IP lockouts
grep "locked out" data/logs/vision3.log

# Monitor successful logins
grep "authenticated successfully" data/logs/vision3.log
```

## Best Practices

### General Security

1. **Change Default Passwords**
   - Default user: `felonius` / `password`
   - Change immediately after installation

2. **Use Strong Passwords**
   - Passwords are bcrypt-hashed
   - Encourage users to use strong passwords
   - Consider password complexity requirements

3. **Keep Software Updated**

   ```bash
   git pull
   go build ./cmd/vision3
   # or
   docker-compose up -d --build
   ```

4. **Monitor Logs**

   ```bash
   tail -f data/logs/vision3.log
   ```

   Watch for:
   - Repeated failed login attempts
   - Unusual connection patterns
   - Rate limit hits

5. **Backup Regularly**

   ```bash
   # Backup critical data
   tar -czf bbs-backup-$(date +%Y%m%d).tar.gz \
     configs/ data/users/ data/msgbases/
   ```

### Connection Security Recommendations

**Public BBS:**

- `maxNodes`: 10-50 (based on server capacity)
- `maxConnectionsPerIP`: 2-3
- Use blocklist for known bad actors
- Use allowlist for admins only

**Private/Community BBS:**

- `maxNodes`: 5-20 (based on community size)
- `maxConnectionsPerIP`: 1-2
- Strict allowlist (optional)
- Active monitoring

**Test/Development:**

- `maxNodes`: 0 (unlimited)
- `maxConnectionsPerIP`: 0 (unlimited)
- Empty blocklist/allowlist

### SSH Hardening

1. **Disable Password Authentication**
   Edit `/etc/ssh/sshd_config`:

   ```
   PasswordAuthentication no
   PubkeyAuthentication yes
   ```

2. **Use Non-Standard Port**

   ```json
   {
     "sshPort": 2222
   }
   ```

3. **Limit SSH to Specific IPs**
   Use allowlist for trusted admin IPs.

### Monitoring and Alerts

**Watch for suspicious activity:**

```bash
# Monitor connection attempts
grep "Connection from" data/logs/vision3.log

# Check for rate limit hits
grep "maximum" data/logs/vision3.log

# Watch for blocked IPs
grep "blocked" data/logs/vision3.log

# Track authentication failures
grep "authentication failed" data/logs/vision3.log
```

**Set up alerts** (example with systemd):

Create `/usr/local/bin/bbs-monitor.sh`:

```bash
#!/bin/bash
LOGFILE=/path/to/vision3/data/logs/vision3.log
ALERT_EMAIL=admin@example.com

# Check for excessive failed logins
FAILED_COUNT=$(grep -c "authentication failed" "$LOGFILE" | tail -1000)
if [ "$FAILED_COUNT" -gt 50 ]; then
    echo "BBS Alert: $FAILED_COUNT failed logins detected" | \
        mail -s "BBS Security Alert" "$ALERT_EMAIL"
fi
```

### Firewall Configuration

Use a firewall in addition to BBS-level security:

**UFW (Ubuntu):**

```bash
# Allow BBS port
sudo ufw allow 2222/tcp

# Allow only from specific IP
sudo ufw allow from 203.0.113.42 to any port 2222
```

**iptables:**

```bash
# Rate limit connections
iptables -A INPUT -p tcp --dport 2222 -m state --state NEW \
  -m recent --set
iptables -A INPUT -p tcp --dport 2222 -m state --state NEW \
  -m recent --update --seconds 60 --hitcount 10 -j DROP
```

## Security Checklist

- [ ] Change default password
- [ ] Configure maxNodes and maxConnectionsPerIP
- [ ] Set up IP blocklist (if needed)
- [ ] Set up IP allowlist for admins
- [ ] Enable SSH key authentication only
- [ ] Use non-standard SSH port
- [ ] Set up log monitoring
- [ ] Configure automated backups
- [ ] Enable firewall rules
- [ ] Review user access levels regularly
- [ ] Test emergency access (allowlist)
- [ ] Document your security policies

## Additional Resources

- [Configuration Guide](configuration.md) - Detailed configuration options
- [Installation Guide](installation.md) - Setup and deployment
- [Docker Deployment](docker-deployment.md) - Containerized security
- [User Management](user-management.md) - User access control

## Support

For security issues:

- GitHub Issues: <https://github.com/stlalpha/vision3/issues>
- Email: <robbiew@gmail.com> (for sensitive security issues)

**Note:** Never share your SSH host keys, user database, or configuration files containing sensitive information.
