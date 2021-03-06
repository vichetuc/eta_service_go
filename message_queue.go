package main

import (
	"github.com/streadway/amqp"
)

type AMQPMessager struct {
	url        string
	connection *amqp.Connection
	channel    *amqp.Channel
}

func NewAMQPMessager(url string) *AMQPMessager {
	messager := &AMQPMessager{
		url: url,
	}
	return messager
}

func (messager *AMQPMessager) NewAMQPSender(queue_name string) *AMQPSender {
	queue, err := messager.channel.QueueDeclare(
		queue_name, // name
		false,      // durable
		false,      // delete when unused
		false,      // exclusive
		false,      // no-wait
		nil,        // arguments
	)
	failOnError(err, "Failed to declare a queue")
	sender := &AMQPSender{
		messager: messager,
		queue:    queue,
	}

	return sender
}

func (messager *AMQPMessager) DeclareQueue(queueName string, exclusive bool) *AMQPQueue {
	queue, err := messager.channel.QueueDeclare(
		queueName, // name
		false,     // durable
		false,     // delete when usused
		exclusive, // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	failOnError(err, "Failed to declare a queue")
	return &AMQPQueue{
		queue:    queue,
		messager: messager,
	}
}

func (messager *AMQPMessager) DeclareFanoutExchange(exchangeName string) *AMQPFanoutExchange {
	err := messager.channel.ExchangeDeclare(
		exchangeName, // name
		"fanout",     // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	failOnError(err, "Failed to declare an exchange")

	return &AMQPFanoutExchange{
		messager:     messager,
		exchangeName: exchangeName,
	}
}

func (messager *AMQPMessager) Connect() *AMQPMessager {
	connection, err := amqp.Dial(messager.url)
	failOnError(err, "Failed to connect to RabbitMQ")
	messager.connection = connection

	channel, err := connection.Channel()
	failOnError(err, "Failed to open a channel")
	messager.channel = channel

	return messager
}

func (messager *AMQPMessager) Disconnect() *AMQPMessager {
	messager.channel.Close()
	messager.connection.Close()
	return messager
}

type AMQPSender struct {
	messager *AMQPMessager
	queue    amqp.Queue //check this pointer out
}

func (sender *AMQPSender) Send(message []byte) *AMQPSender {
	err := sender.messager.channel.Publish(
		"",                // exchange
		sender.queue.Name, // routing key
		false,             // mandatory
		false,             // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        message,
		})
	failOnError(err, "Failed to publish a message")

	return sender
}

type AMQPQueue struct {
	queue    amqp.Queue
	messager *AMQPMessager
}

type consumeCallback func(amqp.Delivery)

func (queue *AMQPQueue) Consume(callback consumeCallback) {
	messages, err := queue.messager.channel.Consume(
		queue.queue.Name, // queue
		"",               // consumer
		true,             // auto-ack
		false,            // exclusive
		false,            // no-local
		false,            // no-wait
		nil,              // args
	)
	failOnError(err, "Failed to register a consumer")

	go func() {
		for message := range messages {
			callback(message)
		}
	}()
}

func (queue *AMQPQueue) Bind(exchange *AMQPFanoutExchange) *AMQPQueue {
	err := queue.messager.channel.QueueBind(
		queue.queue.Name, // queue name
		"",               // routing key
		exchange.exchangeName, // exchange
		false,
		nil)
	failOnError(err, "Failed to bind a queue")
	return queue
}

type AMQPFanoutExchange struct {
	messager     *AMQPMessager
	exchangeName string
}

func (exchange *AMQPFanoutExchange) Publish(message []byte) {
	err := exchange.messager.channel.Publish(
		exchange.exchangeName, // exchange
		"",    // routing key
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        message,
		})
	failOnError(err, "Failed to publish a message")
}
func MessagerCabUpdate(message amqp.Delivery) {
	LogInfo("Received a message: %s", message.Body)
	NewDbQuery(IndexName).Put(NewCabFromJson(message.Body))
}

func CabUpdatedCallback(message amqp.Delivery) {
	LogInfo("Received a message: %s", message.Body)
}

func InitMessageQueue() {
	amqpMessager = NewAMQPMessager(messager_queue_url).Connect()
	amqpCabUpdatedExchange = amqpMessager.DeclareFanoutExchange(messager_cab_updated_exchange_name)
}
func InitMessagerConsumer() {
	amqpMessager.DeclareQueue(messager_cab_queue_name, false).Consume(MessagerCabUpdate)
}

func InitMessagerSender() {
	amqpSender = amqpMessager.NewAMQPSender(messager_cab_queue_name)
}

func InitMessagerLogger() {
	amqpMessager.DeclareQueue(messager_cab_updated_name, true).Bind(amqpCabUpdatedExchange).Consume(CabUpdatedCallback)
	forever := make(chan bool)
	<-forever

}

// This function is only for one time send for now
func SendMessage(message []byte) {
	amqpSender.Send(message)
}
