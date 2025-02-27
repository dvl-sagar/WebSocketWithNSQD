package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/nsqio/go-nsq"
)

func main() {
	config := nsq.NewConfig()
	producer, err := nsq.NewProducer("127.0.0.1:4150", config)
	if err != nil {
		log.Fatal("Could not create NSQ Producer:", err)
	}
	for {
		// var message string
		fmt.Print("please Enter the Message : ")
		reader := bufio.NewReader(os.Stdin)
		message, _ := reader.ReadString('\n')
		err = producer.Publish("Health_claims", []byte(message))
		if err != nil {
			log.Fatal("Failed to publish message:", err)
		}
		log.Println("Message sent to NSQ:", message)
	}
}
