package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	flagsmith "github.com/Flagsmith/flagsmith-go-client/v3"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)




var (
	redisClient *redis.Client
	limiter *redis_rate.Limiter
	flagsmithClient *flagsmith.Client
)

func initClients(){
	redisClient = redis.NewClient(&redis.Options {
		Addr: os.Getenv("REDIS_URL"),
	})
	limiter = redis_rate.NewLimiter(redisClient)
	flagsmithClient = flagsmith.NewClient(os.Getenv("FLAGSMITH_KEY"))
}


func main(){
	err := godotenv.Load()

	if err != nil {
		log.Printf("Cannot loading environment")
	} else {
		log.Printf("Loaded environment variables")
	}

	initClients()
	defer redisClient.Close()

	r := gin.Default()

	r.GET("/ping", func(c *gin.Context) {
		remainingRate, err := rateLimitCall(c.ClientIP())

		if err != nil {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Rate Limit Hit"})
		} else {
			c.JSON(http.StatusOK, gin.H{"Your left over API request": remainingRate})
		}
	})

	r.GET("/beta", func(c *gin.Context){
		c.JSON(http.StatusOK, gin.H{"message": "this is beta endpoint"})
	})

	r.GET("/ping-ff", func(c *gin.Context){
		err, flags := getFeatureFlags()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting feature flags"})

		} else {
			c.JSON(http.StatusOK, flags.AllFlags())
		}
	})
	r.Run(":" + os.Getenv("PORT"))
}


func rateLimitCall(ClientIP string)(int, error) {
	ctx := context.Background()
	_, flags := getFeatureFlags()
	rateLimitInterface, _ := flags.GetFeatureValue("rate_limit")

	RATE_LIMIT := int(rateLimitInterface.(float64))

	fmt.Println("Current Rate Limit is", RATE_LIMIT)

	res, err := limiter.Allow(ctx, ClientIP, redis_rate.PerHour(RATE_LIMIT))

	if  err != nil {
		panic(err)
	}

	if res.Remaining == 0 {
		return 0, errors.New("you have hit the Rate Limit for the API. Try again later")
	}

	fmt.Println("remaining request for", ClientIP, "is", res.Remaining)

	return res.Remaining, nil

}

func getFeatureFlags() (error, flagsmith.Flags) {
	ctx := context.Background()
	flags, err := flagsmithClient.GetEnvironmentFlags(ctx)

	return err, flags
}