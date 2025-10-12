package main

import (
	"context"
	"encoding/json"
	"fmt"

	"crypto/tls"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-smtp"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type App struct {
	Redis *redis.Client
}

var (
	allowedDomain string
)

func init() {

	allowedDomain = os.Getenv("DOMAIN")

}

type Backend struct{ app *App }

type Session struct {
	app  *App
	from string
	to   string
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	return nil
}

func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	addr, err := mail.ParseAddress(to)
	if err != nil {
		return err
	}

	parts := strings.Split(addr.Address, "@")
	if len(parts) != 2 {
		return fmt.Errorf("invalid recipient address: %s", addr.Address)
	}

	domain := parts[1]

	if !strings.EqualFold(domain, allowedDomain) {
		return fmt.Errorf("550 5.1.1 recipient domain not allowed: %s", domain)
	}

	s.to = addr.Address

	return nil
}

func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{app: b.app}, nil
}

func (s *Session) Data(r io.Reader) error {

	ctx := context.Background()

	if !allow(ctx, s.app.Redis, fmt.Sprintf("smtp:%s", s.from), 50, time.Minute) {
		return fmt.Errorf("rate limit exceeded")
	}

	mr, err := mail.CreateReader(r)
	if err != nil {
		return err
	}

	var bodyText string
	var bodyHTML string
	attachments := []string{}

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch h := part.Header.(type) {
		case *mail.InlineHeader:
			ct, _, _ := h.ContentType()
			b, _ := io.ReadAll(part.Body)
			switch ct {
			case "text/html":
				bodyHTML = string(b)
			case "text/plain":
				bodyText = string(b)
			default:
			}

		case *mail.AttachmentHeader:
			filename, _ := h.Filename()
			attachments = append(attachments, filename)
		}
	}

	username := parseUsername(s.to)
	msg := map[string]any{
		"id":          fmt.Sprintf("%d", time.Now().UnixNano()),
		"from":        s.from,
		"to":          s.to,
		"subject":     mr.Header.Get("Subject"),
		"body_text":   bodyText,
		"body_html":   bodyHTML,
		"attachments": attachments,
		"received_at": time.Now().Unix(),
	}

	j, _ := json.Marshal(msg)
	key := fmt.Sprintf("inbox:%s", username)
	s.app.Redis.LPush(ctx, key, j)
	s.app.Redis.Expire(ctx, key, 24*time.Hour)

	return nil
}

func (s *Session) Reset()        {}
func (s *Session) Logout() error { return nil }

func allow(ctx context.Context, rdb *redis.Client, key string, limit int, window time.Duration) bool {
	pipe := rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	_, _ = pipe.Exec(ctx)
	return incr.Val() <= int64(limit)
}

func APIKeyAuthMiddleware(validKeys map[string]bool) gin.HandlerFunc {

	return func(c *gin.Context) {
		key := c.GetHeader("X-API-KEY")
		if key == "" || !validKeys[key] {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid API key"})
			return
		}
		c.Next()
	}
}

func parseUsername(addr string) string {
	at := strings.Index(addr, "@")
	if at == -1 {
		return addr
	}
	return addr[:at]
}

func main() {
	redisAddr := getenv("REDIS_ADDR", "localhost:6379")
	redis := redis.NewClient(&redis.Options{Addr: redisAddr})
	app := &App{Redis: redis}
	certFile := "./certs/fullchain.pem"
	keyFile := "./certs/privkey.pem"

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatalf("❌ Failed to load TLS cert: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	go func() {
		s := smtp.NewServer(&Backend{app: app})
		s.Addr = ":25"
		s.Domain = allowedDomain
		s.TLSConfig = tlsConfig
		s.AllowInsecureAuth = false
		log.Println("SMTP listening on :25")
		if err := s.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	key := os.Getenv("API_KEY")
	validAPIKeys := map[string]bool{
		key: true,
	}

	router := gin.Default()
	router.Use(func(c *gin.Context) {
		ip := c.ClientIP()
		ctx := c.Request.Context()
		if !allow(ctx, app.Redis, fmt.Sprintf("api:%s", ip), 100, time.Minute) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	})

	router.Use(APIKeyAuthMiddleware(validAPIKeys))

	router.GET("/emails/:username", func(c *gin.Context) {

		ctx := c.Request.Context()
		username := c.Param("username")
		key := fmt.Sprintf("inbox:%s", username)
		msgs, err := app.Redis.LRange(ctx, key, 0, 49).Result()

		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		var out []map[string]any
		for _, m := range msgs {
			var v map[string]any
			_ = json.Unmarshal([]byte(m), &v)
			out = append(out, v)
		}
		c.JSON(200, out)
	})

	router.DELETE("/emails/:username", func(c *gin.Context) {
		ctx := c.Request.Context()

		username := c.Param("username")
		key := fmt.Sprintf("inbox:%s", username)

		err := app.Redis.Del(ctx, key).Err()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("All messages for %s have been cleared", username),
		})
	})

	httpsServer := &http.Server{
		Addr:      ":443",
		Handler:   router,
		TLSConfig: tlsConfig,
	}

	httpServer := &http.Server{
		Addr:    ":80",
		Handler: http.HandlerFunc(redirectToHTTPS),
	}

	go func() {
		log.Println("🚀 Starting HTTPS on :443")
		if err := httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTPS server failed: %v", err)
		}
	}()

	go func() {
		log.Println("🌐 Starting HTTP redirect on :80")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🧹 Shutting down servers gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpsServer.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down HTTPS server: %v", err)
	}
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down HTTP server: %v", err)
	}

	log.Println("✅ Servers stopped cleanly")
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func redirectToHTTPS(w http.ResponseWriter, r *http.Request) {
	target := "https://" + r.Host + r.URL.RequestURI()
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}
