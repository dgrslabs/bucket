package middleware

import (
	"log"

	"github.com/dgrslabs/bucket/client/web/domain"
	"github.com/gin-gonic/gin"
)

type bucketMiddleware struct {
	logger  *log.Logger
	mounted bool
}

func NewBucketMiddleware(bucketLog *log.Logger, mounted bool) domain.Middleware {
	return &bucketMiddleware{
		logger:  bucketLog,
		mounted: mounted,
	}
}

func (b *bucketMiddleware) APIGuardMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !b.mounted {
			// Read user_id from header
			userID := c.GetHeader("bucket-User-ID")
			if userID == "" {
				b.logger.Printf("unauthorised: user_id not found in headers")
				c.AbortWithStatusJSON(401, gin.H{"error": "unauthorised", "message": "user_id not found in headers"})
				return
			}

			// Set in Gin context for further use
			c.Set("owner_id", userID)
			c.Next()
			return
		}

		c.Set("owner_id", "default")
		c.Next()
	}
}

// WebGuardMiddleware implements domain.Middleware.
func (b *bucketMiddleware) WebGuardMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		c.Set("owner_id", "default")

		c.Next()
	}
}
