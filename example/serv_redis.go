package main

import (
	"fmt"
	"log"

	redis "github.com/r0123r/go-redis-server"
)

type MyHandler struct {
	*redis.DefaultHandler
}

func NewMyHandler() *MyHandler {
	h := redis.NewDefaultHandler()
	return &MyHandler{h}
}

// Test implement a new command. Non-redis standard, but it is possible.
func (h *MyHandler) Test() ([]byte, error) {
	return []byte("Awesome custom redis command!"), nil
}

// Get override the DefaultHandler's method.
func (h *MyHandler) Set(key string, args ...[]byte) error {
	// However, we still can call the DefaultHandler GET method and use it.
	err := h.DefaultHandler.Set(key, args...)
	if err != nil {
		return err
	}
	notify := fmt.Sprint("__keyspace@", h.DefaultHandler.CurrentDb, "__:", key)
	log.Print(notify)
	h.Publish(notify, []byte("set"))
	return nil
}

// Test2 implement a new command. Non-redis standard, but it is possible.
// This function needs to be registered.
func Test2() ([]byte, error) {
	return []byte("Awesome custom redis command via function!"), nil
}

func redis_start() {
	myhandler := NewMyHandler()
	srv, err := redis.NewServer(redis.DefaultConfig().Proto("tcp").Port(88).Host("localhost").Handler(myhandler))
	if err != nil {
		log.Println(err)
	}
	if err := srv.RegisterFct("test2", Test2); err != nil {
		log.Println(err)
	}
	if err := srv.Start(); err != nil {
		log.Println(err)
	}
}
