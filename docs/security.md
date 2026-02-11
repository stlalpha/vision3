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
- Files are loaded at startup (restart required for changes)

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

2. Restart BBS to apply:
   ```bash
   # If running with systemd
   sudo systemctl restart vision3

   # If running manually
   # Ctrl+C and restart ./vision3

   # If running with Docker
   docker-compose restart
   ```

3. Monitor logs:
   ```bash
   tail -f data/logs/vision3.log | grep "192.0.2"
   ```

### Logging

IP filtering actions are logged:

```
INFO: IP blocklist enabled from configs/blocklist.txt
INFO: Loaded IP list from configs/blocklist.txt: 5 IPs, 3 CIDR ranges
INFO: IP allowlist enabled from configs/allowlist.txt
INFO: Loaded IP list from configs/allowlist.txt: 2 IPs, 1 CIDR ranges
INFO: Rejecting Telnet connection from 192.0.2.100: IP address is blocked
DEBUG: IP 203.0.113.42 is on allowlist, bypassing all checks
```

### Troubleshooting

**Problem:** Changes to blocklist/allowlist don't apply

**Solution:** Restart the BBS. Lists are loaded at startup, not dynamically.

---

**Problem:** Allowlisted IP still getting rate-limited

**Solution:** Check the logs. Ensure:
- File path is correct in config.json
- IP format is valid
- BBS was restarted after changes

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

### SSH Authentication

SSH-authenticated users bypass the login screen:

- User must exist in `data/users/users.json`
- Username from SSH auth is looked up in database
- If found, user is automatically logged in
- If not found, user goes through normal login

**Security implications:**
- Disable SSH password authentication, use keys only
- Keep SSH keys secure
- Monitor `data/users/call_history.json` for suspicious activity

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
- GitHub Issues: https://github.com/robbiew/vision3/issues
- Email: robbiew@gmail.com (for sensitive security issues)

**Note:** Never share your SSH host keys, user database, or configuration files containing sensitive information.
