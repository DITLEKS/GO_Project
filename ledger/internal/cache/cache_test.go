package cache

import (
 "context"
 "os"
 "testing"
 "time"
)
func TestCacheRoundTripAndTTL(t *testing.T){
 if os.Getenv("TEST_DATABASE_URL")==""{t.Skip("Redis integration: enable with TEST_DATABASE_URL and use a dedicated REDIS_DB")}
 ctx:=context.Background();c,err:=Open(ctx);if err!=nil{t.Fatal(err)};t.Cleanup(func(){_ = c.Close()})
 key:="report:summary:test:ttl";t.Cleanup(func(){c.redis.Del(ctx,key)})
 if err:=c.Set(ctx,key,[]byte(`{"ok":true}`));err!=nil{t.Fatal(err)}
 data,err:=c.Get(ctx,key);if err!=nil || string(data)!=`{"ok":true}`{t.Fatal(string(data),err)}
 ttl,err:=c.redis.TTL(ctx,key).Result();if err!=nil || ttl<=0 || ttl>TTL{t.Fatal(ttl,err)}
 if err:=c.redis.PExpire(ctx,key,time.Millisecond).Err();err!=nil{t.Fatal(err)}
 time.Sleep(10*time.Millisecond)
 if _,err:=c.Get(ctx,key);err==nil{t.Fatal("expired entry still exists")}
}
func TestInvalidRedisDB(t *testing.T){
 t.Setenv("REDIS_DB","not-a-number");if _,err:=Open(context.Background());err==nil{t.Fatal("invalid REDIS_DB accepted")}
 t.Setenv("REDIS_DB","-1");if _,err:=Open(context.Background());err==nil{t.Fatal("negative REDIS_DB accepted")}
}
