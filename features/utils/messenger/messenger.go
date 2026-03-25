package messenger

import (
	"net"
	"os"

	"github.com/segmentio/kafka-go"
)

var (
    HOST string = os.Getenv("KAFKA_IP")
    PORT string = os.Getenv("KAFKA_PORT")
)

func CreateTopic(topic string) error {
    controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(HOST, PORT))
    if err != nil {
        return err
    }
    defer controllerConn.Close()


    topicConfigs := []kafka.TopicConfig{{ Topic: topic, NumPartitions: 1, ReplicationFactor: 1 }}

    return controllerConn.CreateTopics(topicConfigs...)
}

func DeleteTopic(topic string) error {
    controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(HOST, PORT))
    if err != nil {
        return err
    }
    defer controllerConn.Close()

    return controllerConn.DeleteTopics(topic)
}

func SendMessage(topic string, message string) error {
    controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(HOST, PORT))
    if err != nil {
        return err
    }
    defer controllerConn.Close()

    _, err = controllerConn.WriteMessages(kafka.Message{Value: []byte(message)})
    return err
}

func ListTopics() ([]string, error) {
    topicLsit := []string{}
    conn, err := kafka.Dial("tcp", net.JoinHostPort(HOST, PORT))
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