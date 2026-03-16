package messenger

import (
	"context"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
)

func CreateTopic(topic string) error {
    // Basic conceptual flow
    conn, err := kafka.Dial("tcp", os.Getenv("KAFKA_IP"))
    if err != nil {
        return err
    }

    controller, err := conn.Controller()
    if err != nil {
        return err
    }

    controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
    if err != nil {
        return err
    }

    return  controllerConn.CreateTopics(kafka.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1})
}

func DeleteTopic(topic string) error {
    SendMessage(topic, "End of events. Deleting topic.")
    conn, err := kafka.Dial("tcp", os.Getenv("KAFKA_IP"))
    if err != nil {
        return err
    }

    controller, err := conn.Controller()
    if err != nil {
        return err
    }

    controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
    if err != nil {
        return err
    }

    return controllerConn.DeleteTopics(topic)
}

func SendMessage(topic string, message string) error {
    conn, err := kafka.DialLeader(context.Background(), "tcp", os.Getenv("KAFKA_IP"), topic, 0)
    if err != nil {
        return err
    }

    conn.SetWriteDeadline(time.Now().Add(10*time.Second))
    _, err = conn.WriteMessages(
        kafka.Message{Value: []byte(message)},
    )
    if err != nil {
        return err
    }

    if err := conn.Close(); err != nil {
        return err
    }

    return nil
}