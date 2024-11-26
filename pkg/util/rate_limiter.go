package util

// import (
// 	"sync"

// 	"ucode/ucode_go_api_gateway/storage"

// 	"golang.org/x/time/rate"
// )

// type ApiKeyRateLimiter struct {
// 	redisClient storage.RedisStorageI
// 	apiKeys     map[string]*rate.Limiter
// 	mu          *sync.RWMutex
// 	r           rate.Limit
// 	b           int
// }

// func NewApiKeyRateLimiter(redisClient storage.RedisStorageI, r rate.Limit, b int) *ApiKeyRateLimiter {
// 	i := &ApiKeyRateLimiter{
// 		redisClient: redisClient,
// 		apiKeys:     make(map[string]*rate.Limiter),
// 		mu:          &sync.RWMutex{},
// 		r:           r,
// 		b:           b,
// 	}

// 	return i
// }

// func (i *ApiKeyRateLimiter) AddIP(ip string) *rate.Limiter {
// 	i.mu.Lock()
// 	defer i.mu.Unlock()

// 	limiter := rate.NewLimiter(i.r, i.b)

// 	i.apiKeys[ip] = limiter

// 	return limiter
// }

// func (i *ApiKeyRateLimiter) GetLimiter(ip string) *rate.Limiter {
// 	i.mu.Lock()
// 	limiter, exists := i.apiKeys[ip]

// 	if !exists {
// 		i.mu.Unlock()
// 		return i.AddIP(ip)
// 	}

// 	i.mu.Unlock()

// 	return limiter
// }

// // func (i *ApiKeyRateLimiter) AddApiKey(apiKey string) *rate.Limiter {
// // 	ctx := context.Background()
// // 	i.mu.Lock()
// // 	defer i.mu.Unlock()

// // 	limiter := rate.NewLimiter(i.r, i.b)
// // 	lmtrMarshal, err := json.Marshal(&limiter)
// // 	if err != nil {
// // 		log.Println("Error while marshalling limiter", err)
// // 		return nil
// // 	}
// // 	err = i.redisClient.SetLimiter(ctx, apiKey, lmtrMarshal, 0, "", "u-code")

// // 	if err != nil {
// // 		log.Println("Error while adding api key to redis", err)
// // 		return nil
// // 	}

// // 	return limiter
// // }

// // func (i *ApiKeyRateLimiter) GetLimiter(apiKey string) *rate.Limiter {
// // 	ctx := context.Background()
// // 	i.mu.Lock()
// // 	val, err := i.redisClient.GetLimiter(ctx, apiKey, "", "u-code")
// // 	if err != nil {
// // 		if errors.Is(err, redis.Nil) {
// // 			i.mu.Unlock()
// // 			return i.AddApiKey(apiKey)
// // 		}
// // 		log.Println("Error while getting api key from redis", err)
// // 		return nil
// // 	}

// // 	var lmtr *rate.Limiter
// // 	err = json.Unmarshal([]byte(val), &lmtr)
// // 	if err != nil {
// // 		log.Println("Error while unmarshalling limiter from redis", err)
// // 		return nil
// // 	}

// // 	i.mu.Unlock()

// // 	return lmtr
// // }
