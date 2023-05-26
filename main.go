package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"monitoring.com/monitoring-app/docs"
)

type Measurement struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Timestamp time.Time          `bson:"timestamp"`
	CPU       float64            `bson:"cpu"`
	RAM       float64            `bson:"ram"`
}

func getCPURAMUsage() (float64, float64, error) {
	// Get CPU usage percentage
	percent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return 0.0, 0.0, err
	}
	cpuUsage := percent[0]

	// Get RAM usage percentage
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return 0.0, 0.0, err
	}
	ramUsage := memInfo.UsedPercent

	return cpuUsage, ramUsage, nil
}

// type Measurement struct {
// 	ID        string    `json:"id"`
// 	Timestamp time.Time `json:"timestamp"`
// 	CPU       float64   `json:"cpu"`
// 	RAM       float64   `json:"ram"`
// }

// var client *mongo.Client
// var collection *mongo.Collection

// @Summary Get CPU and RAM usage
// @Description Retrieves the CPU and RAM usage in percentages
// @Tags Measurements
// @Produce json
// @Success 200 {object} Measurement
// @Router /measurements [get]
func getMeasurements(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(),
		10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx,
		options.Client().ApplyURI("mongodb://mongodb:27017"))
	if err != nil {
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": "Failed to connect to MongoDB"})
		return
	}
	defer func() {
		if err = client.Disconnect(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	collection :=
		client.Database("go-database").Collection("resource-mon")

	cur, err := collection.Find(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": "Failed to retrieve measurements"})
		return
	}
	defer cur.Close(ctx)

	var measurements []Measurement
	if err := cur.All(ctx, &measurements); err != nil {
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": "Failed to decode measurements"})
		return
	}

	c.JSON(http.StatusOK, measurements)
}

// @Summary Create a new measurement
// @Description Create a new measurement record
// @Accept json
// @Produce json
// @Param measurement body Measurement true "Measurement object to be created"
// @Success 201 {string} string "Measurement created successfully"
// @Failure 400 {object} string "Bad request"
// @Failure 500 {object} string "Internal server error"
// @Router /measurements [post]
func createMeasurement(c *gin.Context) {
	var measurement Measurement
	if err := c.ShouldBindJSON(&measurement); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	collection, err := getMongoCollection()
	if err != nil {
		log.Fatal(err)
	}

	_, err = collection.InsertOne(nil, measurement)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusCreated)
}
func getMongoCollection() (*mongo.Collection, error) {
	// Set MongoDB connection options
	clientOptions := options.Client().ApplyURI("mongodb://mongodb:27017")

	// Connect to MongoDB
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, err
	}

	// Check the connection
	err = client.Ping(context.Background(), nil)
	if err != nil {
		return nil, err
	}

	// Set the collection
	collection := client.Database("go-database").Collection("resource-mon")

	return collection, nil
}

// @Summary Get a measurement by ID
// @Description Get a measurement record by ID
// @Produce json
// @Param id path string true "Measurement ID"
// @Success 200 {object} Measurement "Measurement object"
// @Failure 404 {object} string "Measurement not found"
// @Failure 500 {object} string "Internal server error"
// @Router /measurements/{id} [get]
func getMeasurement(c *gin.Context) {
	id := c.Param("id")

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	collection, err := getMongoCollection()
	if err != nil {
		log.Fatal(err)
	}

	var measurement Measurement
	err = collection.FindOne(nil, bson.M{"_id": objectID}).Decode(&measurement)

	log.Println(measurement)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.Status(http.StatusNotFound)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, measurement)
}

// @Summary Update a measurement
// @Description Update a measurement record by ID
// @Accept json
// @Produce json
// @Param id path string true "Measurement ID"
// @Param measurement body Measurement true "Measurement object to be updated"
// @Success 200 {string} string "Measurement updated successfully"
// @Failure 400 {object} string "Bad request"
// @Failure 500 {object} string "Internal server error"
// @Router /measurements/{id} [put]
func updateMeasurement(c *gin.Context) {
	id := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	collection, err := getMongoCollection()
	if err != nil {
		log.Fatal(err)
	}
	var measurement Measurement
	if err := c.ShouldBindJSON(&measurement); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err = collection.ReplaceOne(nil, bson.M{"_id": objectID}, measurement)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary Delete a measurement
// @Description Delete a measurement record by ID
// @Param id path string true "Measurement ID"
// @Success 200 {string} string "Measurement deleted successfully"
// @Failure 500 {object} string "Internal server error"
// @Router /measurements/{id} [delete]
func deleteMeasurement(c *gin.Context) {
	id := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	collection, err := getMongoCollection()
	if err != nil {
		log.Fatal(err)
	}

	_, err = collection.DeleteOne(nil, bson.M{"_id": objectID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func storeLocalMeasurement(cpu float64, ram float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection, err := getMongoCollection()
	measurement := Measurement{
		Timestamp: time.Now(),
		CPU:       cpu,
		RAM:       ram,
	}
	log.Println("a new record is inserted")

	_, err = collection.InsertOne(ctx, measurement)
	if err != nil {
		return err
	}

	return nil
}

func runResourceObserver() {
	ticker := time.NewTicker(10 * time.Second) // Change the interval  as per your requirement.
	go func() {
		for range ticker.C {
			cpu, ram, err := getCPURAMUsage()
			if err != nil {
				log.Println("Error getting CPU and RAM usage:",
					err)
				continue
			}

			err = storeLocalMeasurement(cpu, ram)
			if err != nil {
				log.Println("Error storing measurement:", err)
			}
		}
	}()
}

var wg sync.WaitGroup

func main() {
	// Start MQTT in a separate goroutine
	wg.Add(1)
	go runMQTT()
	// Run other tasks or code here
	go runResourceObserver()

	router := gin.Default()

	// Initialize Swagger documentation
	docs.SwaggerInfo.Title = "Your API Title"
	docs.SwaggerInfo.Description = "Your API Description"
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.Host = "localhost:8080"
	docs.SwaggerInfo.BasePath = "/"

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.GET("/measurements", getMeasurements)
	router.POST("/measurements", createMeasurement)
	router.GET("/measurements/:id", getMeasurement)
	router.PUT("/measurements/:id", updateMeasurement)
	router.DELETE("/measurements/:id", deleteMeasurement)

	router.GET("/")

	log.Println("server started")
	router.Run(":8080")
	// Wait for MQTT goroutine to finish
	wg.Wait()
}

func runMQTT() {
	defer wg.Done()

	// MQTT broker URL
	brokerURL := "tcp://mqtt-broker:1883"

	// MQTT client options
	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID("mqtt-client")
	opts.SetDefaultPublishHandler(messageHandler)

	// Create MQTT client
	client := mqtt.NewClient(opts)

	// Connect to the MQTT broker
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}

	// Subscribe to MQTT topics and set the message handler
	if token := client.Subscribe("my-topic", 0, nil); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}

	// Keep the application running
	select {}

}

func sendMessage() {
	// Create MQTT client
	// MQTT broker URL
	brokerURL := "tcp://mqtt-broker:1883"
	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID("mqtt-client")
	client := mqtt.NewClient(opts)
	// Connect to the MQTT broker
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}

}

func messageHandler(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
	var measurement Measurement
	err := json.Unmarshal(msg.Payload(), &measurement)
	if err != nil {
		log.Printf("Error parsing JSON: %s\n", err)
		return
	}

	measurement.Timestamp = time.Now()

	err = storeMQTTMeasurement(measurement)
	if err != nil {
		log.Printf("Error storing measurement: %s\n", err)
		return
	}

	fmt.Println("Measurement stored successfully:", measurement)
}

func storeMQTTMeasurement(measurement Measurement) error {
	collection, err := getMongoCollection()
	if err != nil {
		log.Fatal(err)
	}
	_, err = collection.InsertOne(nil, measurement)
	if err != nil {
		return err
	}

	return nil
}
