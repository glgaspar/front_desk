package messenger

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/segmentio/kafka-go"
)

func getKafkaBroker() string {
	host := os.Getenv("KAFKA_IP")
	port := os.Getenv("KAFKA_PORT")

	if host != "" && port != "" {
		return net.JoinHostPort(host, port)
	}
	// Default to the internal Docker network address for Kafka
	return "kafka:29092"
}

func CreateTopic(topic string) error {
	controllerConn, err := kafka.Dial("tcp", getKafkaBroker())
	if err != nil {
		return err
	}
	defer controllerConn.Close()

	topicConfigs := []kafka.TopicConfig{{Topic: topic, NumPartitions: 1, ReplicationFactor: 1}}

	return controllerConn.CreateTopics(topicConfigs...)
}

func DeleteTopic(topic string) error {
	controllerConn, err := kafka.Dial("tcp", getKafkaBroker())
	if err != nil {
		return err
	}
	defer controllerConn.Close()

	return controllerConn.DeleteTopics(topic)
}

func SendMessage(topic string, message string) error {
	controllerConn, err := kafka.Dial("tcp", getKafkaBroker())
	if err != nil {
		return err
	}
	defer controllerConn.Close()

	_, err = controllerConn.WriteMessages(kafka.Message{Topic: topic, Value: []byte(message)})
	return err
}

func ListTopics() ([]string, error) {
	topicLsit := []string{}
	conn, err := kafka.Dial("tcp", getKafkaBroker())
	if err != nil {
		return topicLsit, err
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions()
	if err != nil {
		return topicLsit, err
	}

	for _, p := range partitions {
		topicLsit = append(topicLsit, p.Topic)
	}

	return topicLsit, err
}

func Subscribe(ctx context.Context, topic string, logs chan<- string) error {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{getKafkaBroker()},
		Topic:    topic,
		MaxBytes: 10e6, // 10MB
	})
	defer reader.Close()

	for {
		m, err := reader.ReadMessage(ctx)
		if err != nil {
			fmt.Println(err)
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
        fmt.Println(string(m.Value))
		select {
		case logs <- string(m.Value):
		case <-ctx.Done():
			return nil
		}
	}
}
