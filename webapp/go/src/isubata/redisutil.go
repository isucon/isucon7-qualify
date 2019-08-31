package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/garyburd/redigo/redis"
)

//redisの汎用関数Set、データが増えたら随時caseを追加
func redisSet(tag string, id string, object interface{}) bool {

	c, err := redis.Dial("tcp", ":6379")
	if err != nil {
		log.Fatal(err)
		return false
	}
	defer c.Close()

	switch objectFormat := object.(type) {
	case *[]ChannelInfo:
		key := fmt.Sprintf("%s:%s", tag, id)
		serialized, _ := json.Marshal(objectFormat)
		val, err := c.Do("Set", key, serialized, "NX", "EX", "3600")
		if val == nil {
			log.Printf("redisSet:This object exist already")
			return false
		} else if err != nil {
			log.Printf("redisSet:Error :", err)
			return false
		} else {
			log.Printf("redisSet:Success!! (%s,%s)", tag, id)
			return true
		}
	default:
		log.Printf("redisSet:Assertion Error")
		return false
	}

}

//redisの汎用関数Get、データが増えたら随時caseを追加
func redisGet(tag string, id string, object interface{}) bool {

	c, err := redis.Dial("tcp", ":6379")
	if err != nil {
		log.Fatal(err)
		return false
	}
	defer c.Close()

	key := fmt.Sprintf("%s:%s", tag, id)
	val, err := redis.Bytes(c.Do("Get", key))
	if err != nil {
		log.Printf("redisGet:Error :", err)
		return false
	}

	if val != nil {
		switch objectFormat := object.(type) {
		case *[]ChannelInfo:
			err := json.Unmarshal(val, objectFormat)
			if err != nil {
				log.Printf("redisGet:Assertion Error :", err)
				return false
			} else {
				log.Printf("redisGet:Success!! (%s,%s)", tag, id)
				return true
			}
		default:
			log.Printf("redisGet:Invalid type")
			return false
		}
	} else {
		log.Printf("redisGet:Invalid Key")
		return false
	}
}

//redisの汎用関数Delete
func redisDel(tag string, id string) bool {

	c, err := redis.Dial("tcp", ":6379")
	if err != nil {
		log.Fatal(err)
		return false
	}
	defer c.Close()

	key := fmt.Sprintf("%s:%s", tag, id)
	if _, err := c.Do("DEL", key); err != nil {
		log.Printf("redisDel:Error :", err)
		return false
	} else {
		log.Printf("redisDel:Success!! (%s,%s)", tag, id)
		return true
	}
}

//redisの汎用関数FLUSHALL
func redisFLUSHALL() bool {

	c, err := redis.Dial("tcp", ":6379")
	if err != nil {
		log.Fatal(err)
		return false
	}
	defer c.Close()

	if _, err := c.Do("FLUSHALL"); err != nil {
		log.Printf("redisFLUSHALL:Error :", err)
		return false
	} else {
		log.Printf("redisFLUSHALL:Success!!")
		return true
	}
}
