// Package cache implements the optional, fail-open report cache.
package cache

import (
 "context"
 "fmt"
 "os"
 "strconv"
 "time"
 "github.com/redis/go-redis/v9"
)
const TTL=30*time.Second
type Client struct { redis *redis.Client }
func Open(ctx context.Context)(*Client,error){
 addr:=os.Getenv("REDIS_ADDR");if addr=="" {addr="localhost:6379"}
 db:=0
 if raw:=os.Getenv("REDIS_DB");raw!="" {var err error;db,err=strconv.Atoi(raw);if err!=nil || db<0 {return nil,fmt.Errorf("REDIS_DB must be a non-negative integer")}}
 r:=redis.NewClient(&redis.Options{Addr:addr,DB:db,Password:os.Getenv("REDIS_PASSWORD"),DialTimeout:2*time.Second,ReadTimeout:time.Second,WriteTimeout:time.Second,MaxRetries:-1,ContextTimeoutEnabled:true})
 if err:=r.Ping(ctx).Err();err!=nil {_ = r.Close();return nil,fmt.Errorf("connect Redis at %s: %w",addr,err)}
 return &Client{redis:r},nil
}
// Version invalidates all date ranges without scanning unrelated keys.
func(c *Client)Version(ctx context.Context)(string,error){v,err:=c.redis.Get(ctx,"report:summary:version").Result();if err==redis.Nil{return "0",nil};return v,err}
func(c *Client)Get(ctx context.Context,key string)([]byte,error){return c.redis.Get(ctx,key).Bytes()}
func(c *Client)Set(ctx context.Context,key string,data []byte)error{return c.redis.Set(ctx,key,data,TTL).Err()}
func(c *Client)Invalidate(ctx context.Context)error{return c.redis.Incr(ctx,"report:summary:version").Err()}
func(c *Client)Close()error{return c.redis.Close()}
