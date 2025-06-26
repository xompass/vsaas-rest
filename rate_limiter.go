package rest

import (
	"context"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/redis/go-redis/v9"
)

// Este archivo implementa un limitador de splicitudes (rate limiter) para los endpoints HTTP utilizando Redis.
// Incluye las siguientes funcionalidades principales:
// 1. Configuración del cliente Redis utilizando las variables de entorno para el host, puerto y contraseña.
// 2. Definición de la función rateLimiter que aplica la limitación de tasa a un endpoint específico.
// 3. Implementación de la función checkRateLimit que verifica y aplica la limitación de tasa basada en la dirección IP del cliente y el nombre del endpoint.
// 4. Funciones auxiliares para obtener la configuración de Redis desde las variables de entorno.

var ctx = context.Background()

func newRedisClient() *redis.Client {
	redisHost := getRedisHost()
	redisPort := getRedisPort()
	redisPassword := getRedisPassword()

	return redis.NewClient(&redis.Options{
		Addr:     redisHost + ":" + redisPort,
		Password: redisPassword,
		DB:       1, // Use database 1 for rate limiting
	})
}

func checkRateLimit(e *EndpointContext) error {
	redisClient := e.App.redisClient
	rateLimiter := e.Endpoint.RateLimiter

	if rateLimiter == nil {
		return nil
	}

	rateLimit := e.Endpoint.RateLimiter(e)

	ip := e.IpAddress
	name := e.Endpoint.Name

	key := name + "_" + ip
	if rateLimit.Key != "" {
		key = rateLimit.Key
	}

	pipe := redisClient.TxPipeline()
	incrCmd := pipe.Incr(ctx, key)
	expireCmd := pipe.ExpireNX(ctx, key, rateLimit.Window)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}

	count, err := incrCmd.Result()
	if err != nil {
		return err
	}

	_, err = expireCmd.Result()
	if err != nil {
		return err
	}

	if count > int64(rateLimit.Max) {
		log.Warnf("Rate limit exceeded for %s: %d requests", key, count)
		return fiber.ErrTooManyRequests
	}

	return nil
}

func getRedisHost() string {
	host, ok := os.LookupEnv("REDIS_HOST")
	if !ok {
		log.Warn("REDIS_HOST environment variable not set, using default 'localhost'")
		return "localhost"
	}

	return host
}

func getRedisPort() string {
	port, ok := os.LookupEnv("REDIS_PORT")
	if !ok {
		log.Warn("REDIS_PORT environment variable not set, using default '6379'")
		return "6379"
	}

	return port
}

func getRedisPassword() string {
	password, ok := os.LookupEnv("REDIS_PASSWORD")
	if !ok {
		return ""
	}

	return password
}
