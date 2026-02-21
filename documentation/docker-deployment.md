# Docker Deployment Guide for ViSiON/3

This guide covers deploying ViSiON/3 using Docker and Docker Compose.

## Prerequisites

- Docker Engine 20.10+
- Docker Compose 2.0+ (optional, but recommended)

## Quick Start with Docker Compose

1. **Clone the repository:**

   ```bash
   git clone https://github.com/stlalpha/vision3.git
   cd vision3
   ```

2. **Start the BBS:**

   ```bash
   docker-compose up -d
   ```

   This will:
   - Build the Docker image with libssh support
   - Compile all Go binaries (ViSiON3, v3mail, helper, strings)
   - Build lrzsz from source for zmodem file transfers
   - Bundle binkd (FTN mailer) from `bin/binkd`
   - Create necessary directories (`configs/`, `data/`, `menus/`, `temp/`)
   - Generate SSH host keys automatically
   - Initialize config files from templates
   - Start the BBS on ports 2222 (SSH) and 2323 (telnet)

3. **Check logs:**

   ```bash
   docker-compose logs -f
   ```

4. **Connect to the BBS:**

   ```bash
   ssh felonius@localhost -p 2222
   # Default password: password
   ```

## Manual Docker Build

If you prefer not to use Docker Compose:

1. **Build the image:**

   ```bash
   docker build -t vision3:latest .
   ```

2. **Create host directories:**

   ```bash
   mkdir -p configs data menus
   ```

3. **Run the container:**

   ```bash
   docker run -d \
     --name vision3-bbs \
     -p 2222:2222 \
     -p 2323:2323 \
     -v "$(pwd)/configs:/vision3/configs" \
     -v "$(pwd)/data:/vision3/data" \
     -v "$(pwd)/menus:/vision3/menus" \
     vision3:latest
   ```

## Important Notes

### CGO and libssh Requirement

ViSiON/3 **requires libssh via CGO** for SSH server functionality. The Dockerfile:

- Enables CGO in the build stage (`CGO_ENABLED=1`)
- Installs `libssh-dev` during build
- Includes `libssh` runtime library in the final image
- Builds all Go binaries: `ViSiON3`, `v3mail`, `helper`, `strings`
- Builds `lrzsz` from source for zmodem file transfers (`sz`/`rz`)
- Copies the static `binkd` binary for FTN mailer support

**Do not disable CGO** or the SSH server will not work.

### Persistent Data

The following directories are mounted as volumes and persist across container restarts:

- **`configs/`** - Configuration files (created from templates on first run)
  - `config.json` - Main BBS configuration (ports, security levels, connection limits)
  - `message_areas.json` - Message area definitions (includes PRIVMAIL)
  - `file_areas.json` - File area definitions
  - `doors.json` - Door configurations
  - `ssh_host_rsa_key` - SSH host key (auto-generated)
  - Other config files

- **`data/`** - Runtime data
  - `users/` - User database and call history
  - `msgbases/` - JAM message bases (including `privmail/`)
  - `files/` - File areas
  - `ftn/` - FidoNet/echomail data
  - `logs/` - Application logs (vision3.log, v3mail.log, binkd.log)

- **`menus/`** - Menu files (ANSI screens, configs)
  - Mount your custom menu set here

### First Run Initialization

On first run, the entrypoint script will:

1. Create necessary directories
2. Generate SSH host keys (RSA and ED25519)
3. Copy template configs to `configs/` if missing
4. Create default user (felonius/password)

### Configuration

After first run, edit the configuration files in the `configs/` directory:

```bash
# Edit main config
nano configs/config.json

# Restart to apply changes
docker-compose restart
```

## Private Mail Setup

The PRIVMAIL area is automatically configured in `configs/message_areas.json`. The Docker setup ensures:

- `data/msgbases/privmail/` directory is created
- JAM message base files are initialized on first message
- EMAILM menu is accessible via the E key from main menu

## Updating

To update to the latest version:

```bash
# Pull latest code
git pull

# Rebuild and restart
docker-compose up -d --build
```

Your data in `configs/`, `data/`, and `menus/` volumes will be preserved.

## Troubleshooting

### SSH Connection Refused

If you can't connect via SSH:

1. Check container logs: `docker-compose logs`
2. Verify port 2222 is not already in use: `netstat -ln | grep 2222`
3. Check SSH keys were generated: `ls -l configs/ssh_host_*`

### libssh Errors

If you see libssh-related errors:

- Ensure the image was built with CGO enabled
- Check that `libssh` is installed in the container:

  ```bash
  docker-compose exec vision3 apk info libssh
  ```

### Configuration Not Loading

If config changes aren't applied:

1. Ensure config files exist in the `configs/` volume
2. Restart the container: `docker-compose restart`
3. Check file permissions (should be readable by container)

### Message Base Errors

If you see JAM-related errors:

1. Check directory permissions in `data/msgbases/`
2. Ensure PRIVMAIL area exists in `configs/message_areas.json`
3. Delete corrupted JAM files and let them regenerate

## Advanced Usage

### Custom Ports

To use different ports, edit `docker-compose.yml`:

```yaml
ports:
  - "2323:2222"  # Expose SSH on port 2323
  - "2324:2323"  # Expose telnet on port 2324
```

### Resource Limits

Uncomment the `deploy` section in `docker-compose.yml` to set CPU/memory limits.

### Multiple Nodes

To run multiple BBS nodes:

```yaml
services:
  vision3-node1:
    # ... same config, different port
    ports:
      - "2222:2222"

  vision3-node2:
    # ... same config, different port
    ports:
      - "2223:2222"
```

### Custom Menu Set

Mount a custom menu directory:

```yaml
volumes:
  - ./my-custom-menus:/vision3/menus
```

## Production Deployment

For production deployments:

1. **Use a reverse proxy** (nginx, Caddy) for SSH multiplexing if needed
2. **Set up backups** for the `data/` volume
3. **Monitor logs** with a logging solution (ELK, Loki, etc.)
4. **Set resource limits** to prevent runaway processes
5. **Use Docker secrets** for sensitive configuration
6. **Enable auto-restart**: `restart: unless-stopped` (already set)

## Security Considerations

- Change the default password immediately after first login
- Use strong SSH host keys (automatically generated)
- Keep the Docker image updated
- Limit exposed ports (only expose 2222)
- Consider running with a non-root user inside the container
- Set up firewall rules on the host
- Configure connection limits in `config.json`:
  - `maxNodes`: Maximum simultaneous connections (default: 10)
  - `maxConnectionsPerIP`: Maximum connections per IP address (default: 3)
  - Set to 0 to disable limits (not recommended for public BBSes)
- Configure IP filtering (optional):
  - `ipBlocklistPath`: Path to file containing blocked IPs/CIDR ranges
  - `ipAllowlistPath`: Path to file containing allowed IPs (bypasses all limits)
  - File format: one IP or CIDR range per line, # for comments

## Support

For issues related to Docker deployment:

- Check logs: `docker-compose logs -f`
- GitHub Issues: <https://github.com/stlalpha/vision3/issues>
- Include Docker version, OS, and error logs when reporting issues
