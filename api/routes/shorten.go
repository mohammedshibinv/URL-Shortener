package routes

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/mohammedshibinv/URL-Shortner/database"
	"github.com/mohammedshibinv/URL-Shortner/helpers"
	"github.com/redis/go-redis/v9"
)

type request struct {
	URL      string        `json:"url"`
	ShortURL string        `json:"short"`
	Expiry   time.Duration `json:"expiry"`
}

type response struct {
	URL            string        `json:"url"`
	ShortURL       string        `json:"short"`
	Expiry         time.Duration `json:"expiry"`
	XRateRemaining int           `json:"rate_limit"`
	XRateLimit     time.Duration `json:"rate_limit_reset"`
}

func ShortenURL(c *fiber.Ctx) error {
	body := new(request)

	// Log: Start parsing the request body
	log.Println("Start parsing the request body")
	if err := c.BodyParser(&body); err != nil {
		log.Printf("Error parsing JSON: %v\n", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot Parse Json"})
	}

	// Log: Implementing rate limiting based on IP
	log.Println("Implementing rate limiting based on IP")
	r2 := database.CreateClient(1)
	defer r2.Close()

	value, err := r2.Get(database.Ctx, c.IP()).Result()
	log.Printf("Value of err is %v",err)
	if err == redis.Nil {
		log.Println("No rate limit found, setting initial limit")
		_ = r2.Set(database.Ctx, c.IP(), os.Getenv("API_QUOTA"), 30*60*time.Second).Err()
	} else {
		valInt, _ := strconv.Atoi(value)
		if valInt <= 0 {
			limit, _ := r2.TTL(database.Ctx, c.IP()).Result()
			log.Printf("Rate limit exceeded, reset in %v minutes\n", limit/time.Nanosecond/time.Minute)
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error":            "Rate limit exceeded",
				"rate_limit_reset": limit / time.Nanosecond / time.Minute,
			})
		}
	}

	// Log: Validating URL
	log.Println("Validating URL")
	if !govalidator.IsURL(body.URL) {
		log.Println("Invalid URL")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid URL"})
	}

	// Log: Validating domain
	log.Println("Validating domain")
	if !helpers.RemoveDomainError(body.URL) {
		log.Println("Access restricted for the domain")
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Access restricted"})
	}

	// Log: Enforcing https
	log.Println("Enforcing https")
	body.URL = helpers.EnforceHTTP(body.URL)

	var id string

	// Log: Generating short URL
	if body.ShortURL == "" {
		id = uuid.New().String()[:6]
		log.Printf("Generated short URL ID: %s\n", id)
	} else {
		id = body.ShortURL
		log.Printf("Using provided short URL ID: %s\n", id)
	}

	r := database.CreateClient(0)
	defer r.Close()

	val, _ := r.Get(database.Ctx, id).Result()
	if val != "" {
		log.Printf("Short URL ID %s already in use\n", id)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "URL custom short is already in use",
		})
	}

	if body.Expiry == 0 {
		body.Expiry = 24
		log.Println("No expiry provided, defaulting to 24 hours")
	}

	// Log: Setting URL in Redis with expiry
	log.Printf("Setting URL in Redis with ID %s and expiry %d hours\n", id, body.Expiry)
	err = r.Set(database.Ctx, id, body.URL, body.Expiry*3600*time.Second).Err()
	if err != nil {
		log.Printf("Error setting URL in Redis: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Unable to connect to server",
		})
	}

	resp := response{
		URL:            body.URL,
		ShortURL:       "",
		Expiry:         body.Expiry,
		XRateRemaining: 10,
		XRateLimit:     30,
	}

	// Log: Decrementing rate limit count
	log.Println("Decrementing rate limit count")
	r2.Decr(database.Ctx, c.IP())

	val, _ = r2.Get(database.Ctx, c.IP()).Result()
	resp.XRateRemaining, _ = strconv.Atoi(val)
	resp.ShortURL = os.Getenv("DOMAIN") + "/" + id

	// Log: Responding with success
	log.Printf("Responding with short URL: %s\n", resp.ShortURL)
	return c.Status(fiber.StatusOK).JSON(resp)
}

// func ShortenURL(c *fiber.Ctx) error {
// 	body := new(request)

// 	if err := c.BodyParser(&body); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot Parse Json"})
// 	}

// 	// implement rate limiting based on IP
// 	r2 := database.CreateClient(1)
// 	defer r2.Close()

// 	value, err := r2.Get(database.Ctx, c.IP()).Result()

// 	if err == redis.Nil {
// 		_ = r2.Set(database.Ctx, c.IP(), os.Getenv("API_QUOTA"), 30*60*time.Second).Err()
// 	} else {
// 		valInt, _ := strconv.Atoi(value)
// 		if valInt <= 0 {
// 			limit, _ := r2.TTL(database.Ctx, c.IP()).Result()
// 			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
// 				"error":            "Rate limit exceeded",
// 				"rate_limit_reset": limit / time.Nanosecond / time.Minute,
// 			})
// 		}
// 	}

// 	// Validate URL
// 	if !govalidator.IsURL(body.URL) {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid URL"})
// 	}

// 	// Validate domain
// 	if !helpers.RemoveDomainError(body.URL) {
// 		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Access restricted"})
// 	}

// 	// Enforce https, SSL
// 	body.URL = helpers.EnforceHTTP(body.URL)

// 	var id string

// 	if body.ShortURL == "" {
// 		id = uuid.New().String()[:6]
// 	} else {
// 		id = body.ShortURL
// 	}

// 	r := database.CreateClient(0)
// 	defer r.Close()

// 	val, _ := r.Get(database.Ctx, id).Result()
// 	if val != "" {
// 		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
// 			"error": "URL custom short is already in use",
// 		})
// 	}

// 	if body.Expiry == 0 {
// 		body.Expiry = 24
// 	}

// 	err = r.Set(database.Ctx, id, body.URL, body.Expiry*3600*time.Second).Err()

// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Unable to connect to server",
// 		})
// 	}

// 	resp := response{
// 		URL:            body.URL,
// 		ShortURL:       "",
// 		Expiry:         body.Expiry,
// 		XRateRemaining: 10,
// 		XRateLimit:     30,
// 	}

// 	r2.Decr(database.Ctx, c.IP())

// 	val, _ = r2.Get(database.Ctx, c.IP()).Result()
// 	resp.XRateRemaining, _ = strconv.Atoi(val)
// 	resp.ShortURL = os.Getenv("DOMAIN") + "/" + id

// 	return c.Status(fiber.StatusOK).JSON(resp)
// }
