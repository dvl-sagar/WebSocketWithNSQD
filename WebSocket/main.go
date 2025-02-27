package main

import (
	"WebSocket_NSQ_Producer/storage"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/nsqio/go-nsq"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type EchoServer struct {
	Logf func(f string, v ...any)
}

type RequestHandler struct {
}

type websocketRequest struct {
	TrackingId string `json:"trackingId,omitempty"`
}

type Data struct {
	TrackingId string `json:"trackingId"`
	Age int `json:"age"`
	Name string `json:"name"`
}

var (
	clients = make(map[*websocket.Conn]string)
	mu      sync.Mutex
)

func (s *EchoServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if err != nil {
		s.Logf("WebSocket accept error: %v", err)
		return
	}

	handler := RequestHandler{}
	for {
		if err := handler.HandleRequest(c); err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				return
			}
			s.Logf("Request processing error: %v", err)
			return
		}
	}
}

func (h *RequestHandler) HandleRequest(conn *websocket.Conn) error {
	ctx := context.Background()
	typ, r, err := conn.Reader(ctx)
	if err != nil {
		return err
	}

	var req websocketRequest
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}
	data:=storage.FetchInProgressData(req.TrackingId)
	if data!=nil{
		var pendigData Data
		pendingDataBytes,err:=json.Marshal(data)
		if err!=nil{
			return err
		}
		if err:=json.Unmarshal(pendingDataBytes,&pendigData);err!=nil{
			return err
		}
		pendingWebSocketDataBytes,err := json.Marshal(pendigData)
		if err!=nil{
			return err
		}
		err = conn.Write(context.Background(),typ,pendingWebSocketDataBytes)
		if err!=nil{
			return err
		}
	}

	insertedId := storage.SaveRequest(req.TrackingId, nil)
	mu.Lock()
	clients[conn] = req.TrackingId
	mu.Unlock()
	log.Println("Client subscribed to topic:", req.TrackingId)

	startNSQConsumer(conn, typ, insertedId,req.TrackingId)
	return nil
}
func startNSQConsumer(conn *websocket.Conn, typ websocket.MessageType, insertedId any ,trackingId string) {
	config := nsq.NewConfig()
	channel := fmt.Sprintf("channel-%d", time.Now().UnixNano())
	consumer, err := nsq.NewConsumer("Health_claims", channel, config)
	if err != nil {
		log.Fatal("Failed to create NSQ consumer:", err)
	}

	// Handle received messages
	consumer.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		mu.Lock()
		defer mu.Unlock()
		id,ok:=insertedId.(primitive.ObjectID)
		if !ok{
			return errors.New("Invalid type of _id")
		}
		storage.UpdateRequest(id)
		var data Data
		if err:=json.Unmarshal(message.Body,&data);err!=nil{
			return err
		}
		if data.TrackingId== trackingId {
			storage.UpdateData(id,data)
		}
	
		// Send message to WebSocket client
		if err := conn.Write(context.Background(), typ, message.Body); err != nil {
			log.Println("Error sending message to WebSocket:", err)
			return err
		}
		return nil
	}))

	// Connect to NSQ
	err = consumer.ConnectToNSQD("127.0.0.1:4150")
	if err != nil {
		log.Fatal("Failed to connect NSQ consumer:", err)
	}
}

// Start WebSocket server
func main() {
	storage.ConnectDB()
	svr := &EchoServer{
		Logf: log.Printf,
		// Limiter: rate.NewLimiter(rate.Every(100*time.Millisecond), 10),
	}
	http.Handle("/ws", svr)
	log.Println("WebSocket server started at ws://localhost:8080/ws")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// WebSocket connection handler
// func handleConnections(w http.ResponseWriter, r *http.Request) {
// 	conn, err := websocket.Accept(w, r, nil)
// 	if err != nil {
// 		log.Println("WebSocket connection error:", err)
// 		return
// 	}
// 	// defer conn.Close(websocket.StatusNormalClosure, "Closing connection")

// 	typ, wsReqeust, err := conn.Reader(context.Background())
// 	if err != nil {
// 		log.Println("Error reading topic:", err)
// 		return
// 	}
// 	// Read topic from client
// 	var req websocketRequest
// 	err = json.NewDecoder(r.Body).Decode(&wsReqeust)
// 	if err != nil {
// 		log.Println("Error reading topic:", err)
// 		return
// 	}
// 	_, topic, err := conn.Read(r.Context())
// 	if err != nil {
// 		log.Println("Error reading topic:", err)
// 		return
// 	}
// 	// Store new WebSocket connection
// 	mu.Lock()
// 	clients[conn] = req.Topic
// 	mu.Unlock()
// 	log.Println("Client subscribed to topic:", string(topic))

// 	// Start NSQ consumer for the topic
// 	startNSQConsumer(req.Topic, conn, typ)
// }

// Start NSQ Consumer

// func (h *RequestHandler) handleNewRequest(ctx context.Context, conxn *websocket.Conn, typ websocket.MessageType, req md.Request) error {
// 	ID := uuid.New().String()
// 	st.SaveRequest(ID, req.Data)
// 	resp := md.Response{ID: ID}

// 	if err := sendResponse(ctx, conxn, typ, resp); err != nil {
// 		return err
// 	}

// 	h.processAndStoreResult(ID, req.Data)
// 	return h.sendStoredResult(ctx, conxn, typ, ID)
// }

// func (h *RequestHandler) handleExistingRequest(ctx context.Context, conxn *websocket.Conn, typ websocket.MessageType, req md.Request) error {
// 	result, exists := st.GetResult(req.ID)
// 	resp := md.Response{ID: req.ID}

// 	if exists {
// 		resp.Result = result
// 	} else {
// 		resp.Error = "Request ID not found"
// 	}

// 	return sendResponse(ctx, conxn, typ, resp)
// }

// func (h *RequestHandler) processAndStoreResult(requestID string, requestData any) {
// 	result := pc.Process(requestData)
// 	st.SaveResult(requestID, result)
// }

// func (h *RequestHandler) sendStoredResult(ctx context.Context, conxn *websocket.Conn, typ websocket.MessageType, ID string) error {
// 	result, exists := st.GetResult(ID)
// 	if !exists {
// 		return nil
// 	}

// 	resp := md.Response{ID: ID, Result: result}
// 	return sendResponse(ctx, conxn, typ, resp)
// }

// func sendResponse(ctx context.Context, conxn *websocket.Conn, typ websocket.MessageType, resp any) error {
// 	respByte, err := json.Marshal(resp)
// 	if err != nil {
// 		return err
// 	}
// 	return conxn.Write(ctx, typ, respByte)
// }
