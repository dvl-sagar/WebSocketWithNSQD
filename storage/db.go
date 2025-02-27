package storage

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var client *mongo.Client
var collection *mongo.Collection

type DbRequest struct {
	Id         primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	TrackingId string             `bson:"trackingId" json:"trackingId"`
	Data       any                `bson:"data" json:"data"`
	Status     string             `bson:"status" json:"status"`
	CreatedAt  time.Time          `bson:"createdAt" json:"createdAt"`
}

type PendingRequest struct {
	TrackingId string `json:"trackingId" bson:"trackingId"`
	Data       any    `json:"status" bson:"status"`
}



func ConnectDB() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	client, err = mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}

	// Ping
	err = client.Ping(context.Background(), readpref.Primary())
	if err != nil {
		log.Fatal("Ping Failed", err)
	}

	collection = client.Database("websocket-server").Collection("websocket-requests")
	log.Println("Connected to MongoDB")
}

func SaveRequest(trackingId string, data any) any {
	req := DbRequest{
		TrackingId: trackingId,
		Data:       data,
		Status:     "in-progress",
		CreatedAt:  time.Now(),
	}
	result, err := collection.InsertOne(context.TODO(), req)
	if err != nil {
		log.Println("Error saving request:", err)
	}
	return result.InsertedID
}

func UpdateRequest(id primitive.ObjectID) {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{"status": "done"}}

	_, err := collection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		log.Println("Error saving result:", err)
	}
}
func UpdateData(id primitive.ObjectID,data any) {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{"data": data}}

	_, err := collection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		log.Println("Error saving result:", err)
	}
}

func FetchInProgressData(trackingId string)any{
	filter := bson.M{"trackingId": trackingId,"status":"in-progress"}
	var result map[string]interface{}
	err:=collection.FindOne(context.Background(),filter).Decode(&result)
	if err != nil {
		log.Println("Error fetching result:", err)
	}
	return result["data"]
}

func GetResult(id string) (any, bool) {
	var req DbRequest
	err := collection.FindOne(context.TODO(), bson.M{"_id": id}).Decode(&req)
	if err != nil {
		return "", false
	}
	return req.Data, true
}
func GetPendingRequests() []string {
	var pendingIDs []string

	ctx := context.Background()

	cursor, err := collection.Find(ctx, bson.M{"status": "in-progress"})
	if err != nil {
		log.Println("Error fetching pending requests:", err)
		return nil
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var req PendingRequest
		if err := cursor.Decode(&req); err != nil {
			log.Println("Error decoding pending request:", err)
			continue
		}
		pendingIDs = append(pendingIDs, req.TrackingId)
	}
	return pendingIDs
}
