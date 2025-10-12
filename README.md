# Mail Server

A lightweight SMTP mail server with REST API built in Go. This server receives emails via SMTP and stores them in Redis, providing a REST API to retrieve and manage emails.

## Features

- **SMTP Server**: Receives emails on port 25 with TLS support
- **REST API**: Retrieve and manage emails via HTTPS API
- **Redis Storage**: Fast in-memory storage with 24-hour message retention
- **Rate Limiting**: Built-in rate limiting for both SMTP and API endpoints
- **API Key Authentication**: Secure API access with API key validation
- **Docker Support**: Easy deployment with Docker and Docker Compose

## Architecture

- **SMTP Server**: Port 25 for receiving emails
- **HTTPS API**: Port 443 for API access
- **HTTP**: Port 80 (redirects to HTTPS)
- **Redis**: In-memory data store for email messages

## Prerequisites

Before setting up the mail server, ensure you have:

- A domain name (e.g., `example.com`)
- A server with a public IP address
- Root or sudo access to the server
- Docker and Docker Compose installed
- Ports 25 (SMTP), 80 (HTTP), and 443 (HTTPS) open in your firewall

### System Requirements

- Linux server (Ubuntu 20.04+ or similar)
- Minimum 1GB RAM
- 10GB disk space
- Docker 20.10+
- Docker Compose 2.0+

## DNS Configuration

To receive emails, you must configure your DNS records properly.

### 1. Add A Record

Point your domain to your server's IP address:

```
Type: A
Name: mail (or @ for root domain)
Value: YOUR_SERVER_IP
TTL: 3600
```

Example:
- If your domain is `example.com` and server IP is `203.0.113.10`
- Create A record: `mail.example.com` → `203.0.113.10`

### 2. Add MX Record

Set up the MX (Mail Exchange) record to direct email to your mail server:

```
Type: MX
Name: @ (or your domain)
Value: mail.example.com (or your mail subdomain)
Priority: 10
TTL: 3600
```

**Important Notes:**
- The MX record should point to a hostname (A record), not directly to an IP
- Lower priority numbers are preferred (10 is standard)
- Allow 24-48 hours for DNS propagation

### Verify DNS Configuration

After configuring DNS records, verify them:

```bash
# Check A record
dig mail.example.com A

# Check MX record
dig example.com MX

# Alternative with nslookup
nslookup mail.example.com
nslookup -type=MX example.com
```

## SSL/TLS Certificate Setup with Certbot

The mail server requires SSL/TLS certificates for secure SMTP and HTTPS connections.

### 1. Install Certbot

On Ubuntu/Debian:

```bash
sudo apt update
sudo apt install certbot
```

On CentOS/RHEL:

```bash
sudo yum install certbot
```

### 2. Generate Certificates

Use Certbot with standalone mode to generate certificates:

```bash
# Stop any service using ports 80/443
sudo systemctl stop nginx  # if you have nginx
sudo systemctl stop apache2  # if you have apache

# Generate certificate for your domain
sudo certbot certonly --standalone -d mail.example.com

# Or for multiple domains/subdomains
sudo certbot certonly --standalone -d mail.example.com -d example.com
```

### 3. Certificate Location

Certbot stores certificates in `/etc/letsencrypt/live/YOUR_DOMAIN/`:

```
/etc/letsencrypt/live/mail.example.com/
├── fullchain.pem  (full certificate chain)
├── privkey.pem    (private key)
├── cert.pem       (certificate only)
└── chain.pem      (chain only)
```

### 4. Copy Certificates to Project

Create a `certs` directory in your project and copy the certificates:

```bash
# Create certs directory
mkdir -p /path/to/mailserver/certs

# Copy certificates (requires sudo)
sudo cp /etc/letsencrypt/live/mail.example.com/fullchain.pem /path/to/mailserver/certs/
sudo cp /etc/letsencrypt/live/mail.example.com/privkey.pem /path/to/mailserver/certs/

# Set appropriate permissions
sudo chown $USER:$USER /path/to/mailserver/certs/*.pem
chmod 600 /path/to/mailserver/certs/privkey.pem
chmod 644 /path/to/mailserver/certs/fullchain.pem
```

### 5. Certificate Renewal

Let's Encrypt certificates expire after 90 days. Set up automatic renewal:

```bash
# Test renewal (dry run)
sudo certbot renew --dry-run

# Set up automatic renewal with cron
sudo crontab -e

# Add this line to renew certificates twice daily
0 0,12 * * * certbot renew --quiet --post-hook "systemctl restart mailserver"
```

**Important**: Remember to copy renewed certificates to your project directory after renewal.

## Installation & Setup

### Option 1: Docker Deployment (Recommended)

1. **Clone the repository**

```bash
git clone https://github.com/adgai19/mailserver.git
cd mailserver
```

2. **Prepare certificates**

Place your SSL/TLS certificates in the `certs` directory:

```bash
mkdir -p certs
# Copy your fullchain.pem and privkey.pem to certs/
# (See SSL/TLS Certificate Setup section above)
```

3. **Configure environment variables**

Create a `.env` file:

```bash
cat > .env << EOF
DOMAIN=example.com
API_KEY=your-secure-random-api-key-here
EOF
```

Replace:
- `example.com` with your actual domain
- `your-secure-random-api-key-here` with a strong random string

Generate a secure API key:

```bash
# Generate a random API key
openssl rand -base64 32
```

4. **Start the services**

```bash
docker-compose up -d
```

5. **Verify the services are running**

```bash
docker-compose ps
docker-compose logs -f
```

### Option 2: Local Development Setup

1. **Install Go**

Ensure you have Go 1.25+ installed:

```bash
go version
```

2. **Install Redis**

```bash
# Ubuntu/Debian
sudo apt install redis-server
sudo systemctl start redis-server

# macOS
brew install redis
brew services start redis
```

3. **Clone and build**

```bash
git clone https://github.com/adgai19/mailserver.git
cd mailserver

# Install dependencies
go mod download

# Build the application
go build -o server main.go
```

4. **Setup certificates**

```bash
mkdir -p certs
# Copy your fullchain.pem and privkey.pem to certs/
```

5. **Set environment variables**

```bash
export DOMAIN=example.com
export API_KEY=your-secure-api-key
export REDIS_ADDR=localhost:6379
```

6. **Run the server**

```bash
# Run with sudo if binding to port 25 (privileged port)
sudo -E ./server
```

## Configuration

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `DOMAIN` | Your mail server domain | - | Yes |
| `API_KEY` | API authentication key | - | Yes |
| `REDIS_ADDR` | Redis server address | `localhost:6379` | No |
| `GIN_MODE` | Gin framework mode (`debug` or `release`) | `debug` | No |

### Ports

| Port | Service | Description |
|------|---------|-------------|
| 25 | SMTP | Email receiving (requires TLS) |
| 80 | HTTP | Redirects to HTTPS |
| 443 | HTTPS | REST API endpoint |
| 6379 | Redis | Internal (not exposed) |

## API Usage

### Authentication

All API requests require an API key in the header:

```
X-API-KEY: your-api-key-here
```

### Endpoints

#### Get Emails for a User

Retrieve emails for a specific username:

```bash
curl -H "X-API-KEY: your-api-key" \
  https://mail.example.com/emails/username
```

Response:

```json
[
  {
    "id": "1234567890123456789",
    "from": "sender@example.com",
    "to": "username@example.com",
    "subject": "Test Email",
    "body_text": "Plain text content",
    "body_html": "<p>HTML content</p>",
    "attachments": ["file.pdf"],
    "received_at": 1234567890
  }
]
```

#### Delete All Emails for a User

Clear all emails for a specific username:

```bash
curl -X DELETE \
  -H "X-API-KEY: your-api-key" \
  https://mail.example.com/emails/username
```

Response:

```json
{
  "message": "All messages for username have been cleared"
}
```

### Rate Limits

- **API**: 100 requests per minute per IP
- **SMTP**: 50 emails per minute per sender address

## Email Address Format

Emails sent to your domain will be stored by username:

```
user@example.com → stored under "user"
admin@example.com → stored under "admin"
```

Retrieve emails by username via the API:

```bash
curl -H "X-API-KEY: your-api-key" \
  https://mail.example.com/emails/user
```

## Testing Your Mail Server

### Test SMTP Connection

```bash
# Test SMTP connection
telnet mail.example.com 25

# Or use openssl for TLS
openssl s_client -connect mail.example.com:25 -starttls smtp
```

### Send a Test Email

```bash
# Using swaks (SMTP testing tool)
swaks --to user@example.com \
  --from test@external.com \
  --server mail.example.com \
  --port 25 \
  --tls

# Or using sendmail
echo "Subject: Test
This is a test email" | sendmail -v user@example.com
```

### Verify Email Delivery

```bash
# Check if email was received
curl -H "X-API-KEY: your-api-key" \
  https://mail.example.com/emails/user
```

## Monitoring and Logs

### View Logs with Docker

```bash
# All services
docker-compose logs -f

# Just the mail server
docker-compose logs -f mailapi

# Just Redis
docker-compose logs -f redis
```

### Check Service Status

```bash
# Service status
docker-compose ps

# Resource usage
docker stats
```

## Troubleshooting

### Issue: Cannot receive emails

**Check DNS records:**

```bash
dig example.com MX
dig mail.example.com A
```

Ensure MX record points to your mail server and A record resolves to correct IP.

**Check port 25 is open:**

```bash
# From your server
sudo netstat -tlnp | grep :25

# From external
telnet mail.example.com 25
```

**Check firewall:**

```bash
# UFW (Ubuntu)
sudo ufw allow 25/tcp

# firewalld (CentOS)
sudo firewall-cmd --permanent --add-port=25/tcp
sudo firewall-cmd --reload
```

### Issue: Certificate errors

**Verify certificate files:**

```bash
ls -la certs/
# Should show fullchain.pem and privkey.pem
```

**Check certificate validity:**

```bash
openssl x509 -in certs/fullchain.pem -text -noout
```

**Check certificate permissions:**

```bash
# Private key should be readable
chmod 600 certs/privkey.pem
```

### Issue: API returns 401 Unauthorized

**Verify API key:**

Ensure you're sending the correct API key in the `X-API-KEY` header.

**Check environment variables:**

```bash
docker-compose exec mailapi env | grep API_KEY
```

### Issue: Rate limit exceeded

Wait for the rate limit window to reset (1 minute) or adjust rate limits in the code if needed.

### Issue: Redis connection failed

**Check Redis status:**

```bash
docker-compose exec redis redis-cli ping
# Should return: PONG
```

**Check Redis connection:**

```bash
docker-compose logs redis
```

## Security Considerations

1. **Keep certificates updated**: Set up automatic renewal for Let's Encrypt certificates
2. **Use strong API keys**: Generate cryptographically random API keys (32+ characters)
3. **Firewall configuration**: Only expose necessary ports (25, 80, 443)
4. **Regular updates**: Keep Docker images and dependencies updated
5. **Monitor logs**: Regularly check logs for suspicious activity
6. **Backup Redis data**: Set up regular backups if needed
7. **SPF/DKIM/DMARC**: Configure these DNS records for better email deliverability

## Maintenance

### Backup

```bash
# Backup Redis data
docker-compose exec redis redis-cli save
docker cp redis:/data/dump.rdb ./backup/

# Backup certificates
tar -czf certs-backup.tar.gz certs/
```

### Update

```bash
# Pull latest changes
git pull

# Rebuild and restart
docker-compose down
docker-compose build
docker-compose up -d
```

### Clean Up

```bash
# Remove old containers
docker-compose down

# Clean up Docker resources
docker system prune -a
```

## License

This project is open source. Please check the repository for license details.

## Support

For issues, questions, or contributions, please visit:
https://github.com/adgai19/mailserver

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
