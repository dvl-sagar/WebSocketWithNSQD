package main

import (
	"WebSocket_NSQ_Producer/storage"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/coder/websocket"
	"github.com/nsqio/go-nsq"
)

type EchoServer struct {
	Logf func(f string, v ...any)
}

type RequestHandler struct {
}

type websocketRequest struct {
	TrackingId []string `json:"trackingId,omitempty"`
}

type Data struct {
	TrackingId string `json:"trackingId,omitempty"`
	Data       any    `json:"data,omitempty"`
}

func (s *EchoServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusUnauthorized)
		return
	}
	userId, err := getRoleFromToken(token)
	if err != nil {
		log.Println(err)
		return
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if err != nil {
		s.Logf("WebSocket accept error: %v", err)
		return
	}

	handler := RequestHandler{}

	if err := handler.HandleRequest(c, userId); err != nil {
		if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
			return
		}
		s.Logf("Request processing error: %v", err)
		return
	}

}

var (
	clients   = make(map[*websocket.Conn]string)
	consumers = make(map[string]*nsq.Consumer) // Track consumers per user
	mu        sync.Mutex
)

func (h *RequestHandler) HandleRequest(conn *websocket.Conn, userId string) error {
	ctx := context.Background()

	// Read WebSocket request
	typ, r, err := conn.Reader(ctx)
	if err != nil {
		return err
	}

	var req websocketRequest
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		log.Printf("Invalid request sent")
		return nil
	}

	// Fetch and send pending data
	data := storage.FetchInProgressData(req.TrackingId, userId)
	if len(data) > 0 {
		for _, innerData := range data {
			pendingData := Data{
				TrackingId: innerData.TrackingId,
				Data:       innerData.Data,
			}
			pendingDataBytes, err := json.Marshal(pendingData)
			if err != nil {
				return err
			}
			err = conn.Write(ctx, typ, pendingDataBytes)
			if err != nil {
				return err
			}
			storage.CompleteRequest(innerData.TrackingId, userId)
		}
	}

	// Save request for tracking
	storage.SaveRequest(req.TrackingId, nil, userId)

	mu.Lock()
	clients[conn] = userId
	mu.Unlock()

	log.Println("Client subscribed:", userId, "Tracking IDs:", req.TrackingId)

	// Start NSQ Consumer
	consumer := startNSQConsumer(conn, typ, req.TrackingId, userId)

	// Close NSQ Consumer on WebSocket disconnect
	defer func() {
		mu.Lock()
		delete(clients, conn)
		if consumer != nil {
			consumer.Stop()
			delete(consumers, userId)
			log.Println("NSQ Consumer closed for user:", userId)
		}
		mu.Unlock()
	}()

	// Keep the connection alive
	for {
		_, _, err := conn.Reader(ctx)
		if err != nil {
			return err
		}
	}
}

func startNSQConsumer(conn *websocket.Conn, typ websocket.MessageType, trackingId []string, userId string) *nsq.Consumer {
	mu.Lock()
	defer mu.Unlock()

	// If a consumer for this user already exists, return
	if _, exists := consumers[userId]; exists {
		log.Println("Consumer already exists for user:", userId)
		return consumers[userId]
	}

	config := nsq.NewConfig()
	UserChannel := fmt.Sprintf("channel-%v", userId)

	consumer, err := nsq.NewConsumer("Topic_1", UserChannel, config)
	if err != nil {
		log.Fatal("Failed to create NSQ consumer:", err)
	}

	// Handle received messages
	consumer.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		mu.Lock()
		defer mu.Unlock()

		var data Data
		if err := json.Unmarshal(message.Body, &data); err != nil {
			return err
		}

		if slices.Contains(trackingId, data.TrackingId) {
			storage.UpdateRequest(data.TrackingId, userId, data.Data)

			// Send message to WebSocket client
			if err := conn.Write(context.Background(), typ, message.Body); err != nil {
				log.Println("Error sending message to WebSocket:", err)
				return err
			}

			storage.CompleteRequest(data.TrackingId, userId)
		}
		return nil
	}))

	// Connect to NSQ
	err = consumer.ConnectToNSQD("127.0.0.1:4150")
	if err != nil {
		log.Fatal("Failed to connect NSQ consumer:", err)
	}

	consumers[userId] = consumer
	log.Println("NSQ Consumer started for user:", userId)
	return consumer
}

func getRequestToken(r *http.Request) (string, error) {
	if len(r.Header.Values("Authorization")) != 1 {
		return "", errors.New("ErrInvalidAuthorizationToken")
	}
	parts := strings.Split(r.Header.Values("Authorization")[0], " ")
	if len(parts) != 2 {
		return "", errors.New("ErrInvalidAuthorizationToken")
	}
	return getRoleFromToken(parts[1])
}

func getRoleFromToken(token string) (string, error) {
	tokenComponents := strings.Split(token, ".")
	if len(tokenComponents) != 3 {
		return "", errors.New("ErrInvalidTokenProvided")
	}

	payload := tokenComponents[1]

	payloadJsonStr, err := DecodeBase64(payload)

	if err != nil {
		return "", err
	}

	var jsonMap map[string]interface{}
	_ = json.Unmarshal([]byte(payloadJsonStr), &jsonMap)
	requestMap := make(map[string]interface{})

	additionalData := jsonMap["additionalData"]
	d, err := json.Marshal(additionalData)
	if err != nil {
		return "", errors.New("ErrInvalidRequest")
	}
	err = json.Unmarshal(d, &requestMap)
	if err != nil {
		return "", errors.New("ErrInvalidRequest")
	}

	role := requestMap["id"]
	roleStr := fmt.Sprintf("%v", role)

	return roleStr, nil
}

func DecodeBase64(payload string) (string, error) {
	padding := ""
	if i := len(payload) % 4; i != 0 {
		padding = strings.Repeat("=", 4-i)
	}

	payload = payload + padding

	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// Start WebSocket server
func main() {
	storage.ConnectDB()
	svr := &EchoServer{
		Logf: log.Printf,
		// Limiter: rate.NewLimiter(rate.Every(100*time.Millisecond), 10),
	}
	http.Handle("/ws", svr)
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	log.Println("Server started on http://localhost:8080")
	log.Println("WebSocket server started at ws://localhost:8080/ws")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
