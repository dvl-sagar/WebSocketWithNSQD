package main

import (
	"bytes"
	"log"
	"net/http"

	"github.com/nsqio/go-nsq"
)

// NSQ Consumer - Receives messages and sends them to WebSocket server
type NSQConsumer struct{}

func (n *NSQConsumer) HandleMessage(msg *nsq.Message) error {
	log.Println("Received NSQ Message:", string(msg.Body))

	// Send message to WebSocket server via HTTP
	resp, err := http.Post("http://localhost:8080/send", "text/plain", bytes.NewBuffer(msg.Body))
	if err != nil {
		log.Println("Failed to send message to WebSocket server:", err)
		return err
	}
	defer resp.Body.Close()

	return nil
}

func main() {
	cfg := nsq.NewConfig()
	consumer, err := nsq.NewConsumer("Health_claims", "channel1", cfg)
	if err != nil {
		log.Fatal("Failed to create NSQ consumer:", err)
	}

	consumer.AddHandler(&NSQConsumer{})
	err = consumer.ConnectToNSQD("127.0.0.1:4150")
	if err != nil {
		log.Fatal("Failed to connect to NSQ:", err)
	}

	select {} // Keep the program running
}
