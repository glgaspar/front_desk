package messenger

import (
	"net"
	"os"

	"github.com/segmentio/kafka-go"
)


func connect(topic string) (*kafka.Conn, error){
    var host string = os.Getenv("KAFKA_IP")
    var port string = os.Getenv("KAFKA_PORT")
    var controllerConn *kafka.Conn
    var err error

    netConn, err := net.Dial("tcp", net.JoinHostPort(host, port))
    if err != nil {
        return controllerConn, err
    }

    controllerConn = kafka.NewConnWith(netConn, kafka.ConnConfig{Topic: topic})
    
    return controllerConn, err
}

func CreateTopic(topic string) error {
    controllerConn, err := connect("")
    if err != nil {
        return err
    }

    return  controllerConn.CreateTopics(kafka.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1})
}

func DeleteTopic(topic string) error {
    SendMessage(topic, "End of events. Deleting topic.")
    
    controllerConn, err := connect(topic)
    if err != nil {
        return err
    }

    return controllerConn.DeleteTopics(topic)
}

func SendMessage(topic string, message string) error {
    controllerConn, err := connect(topic)
    if err != nil {
        return err
    }

    _, err = controllerConn.WriteMessages(kafka.Message{Value: []byte(message)})
    return err
}