package config

import (
	"crypto/tls"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var MQTTClient mqtt.Client

// ConnectMQTT menghubungkan server ke broker HiveMQ Cloud via TLS.
func ConnectMQTT() (mqtt.Client, error) {
	broker := Env("MQTT_BROKER", "ssl://localhost:8883")
	clientID := Env("MQTT_CLIENT_ID", "checkweigher-server")
	username := Env("MQTT_USERNAME", "")
	password := Env("MQTT_PASSWORD", "")

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)
	opts.SetKeepAlive(30 * time.Second)

	if username != "" {
		opts.SetUsername(username)
		opts.SetPassword(password)
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}
	opts.SetTLSConfig(tlsConfig)

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		log.Println("[MQTT] Terhubung ke broker HiveMQ:", broker)
	})

	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		log.Println("[MQTT] Koneksi terputus:", err)
	})

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(3 * time.Second) {
		log.Println("[MQTT] Timeout menghubungkan ke broker HiveMQ (server tetap lanjut)")
		return nil, nil
	}

	if token.Error() != nil {
		log.Printf("[MQTT] Gagal terhubung ke broker: %v", token.Error())
		return nil, token.Error()
	}

	MQTTClient = client
	log.Println("[MQTT] Berhasil terhubung ke HiveMQ Cloud")
	return client, nil
}

