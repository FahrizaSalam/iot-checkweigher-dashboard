package config

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	MQTTClient mqtt.Client
	mqttMu     sync.RWMutex
)

// GetMQTTClient mengembalikan instance client MQTT yang aktif secara thread-safe.
func GetMQTTClient() mqtt.Client {
	mqttMu.RLock()
	defer mqttMu.RUnlock()
	return MQTTClient
}

// ConnectMQTT menghubungkan aplikasi ke broker MQTT beserta handler auto-reconnect.
func ConnectMQTT(onConnect ...func()) (mqtt.Client, error) {
	broker := Env("MQTT_BROKER", "tcp://broker.hivemq.com:1883")
	clientID := Env("MQTT_CLIENT_ID", "checkweigher-server")
	username := Env("MQTT_USERNAME", "")
	password := Env("MQTT_PASSWORD", "")

	if strings.Contains(broker, "broker.hivemq.com") && clientID == "checkweigher-server" {
		clientID = fmt.Sprintf("%s-%d-%d", clientID, os.Getpid(), time.Now().UnixNano()%10000)
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetConnectTimeout(5 * time.Second)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(3 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)
	opts.SetKeepAlive(30 * time.Second)

	if username != "" {
		opts.SetUsername(username)
		opts.SetPassword(password)
	}

	isTLS := strings.HasPrefix(broker, "ssl://") || strings.HasPrefix(broker, "tls://") || strings.HasPrefix(broker, "tcps://")
	if isTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		}
		opts.SetTLSConfig(tlsConfig)
	}

	opts.SetCustomOpenConnectionFn(func(uri *url.URL, options mqtt.ClientOptions) (net.Conn, error) {
		dialer := &net.Dialer{Timeout: 5 * time.Second}
		host := uri.Host
		if strings.HasPrefix(uri.Scheme, "ssl") || strings.HasPrefix(uri.Scheme, "tls") || strings.HasPrefix(uri.Scheme, "tcps") {
			return tls.DialWithDialer(dialer, "tcp4", host, options.TLSConfig)
		}
		return dialer.DialContext(context.Background(), "tcp4", host)
	})

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		mqttMu.Lock()
		MQTTClient = c
		mqttMu.Unlock()
		log.Printf("[MQTT] Terhubung ke broker HiveMQ: %s (ClientID: %s)", broker, clientID)
		for _, fn := range onConnect {
			if fn != nil {
				fn()
			}
		}
	})

	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		log.Println("[MQTT] Koneksi terputus:", err)
	})

	client := mqtt.NewClient(opts)
	mqttMu.Lock()
	MQTTClient = client
	mqttMu.Unlock()

	token := client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		log.Println("[MQTT] Koneksi awal ke broker HiveMQ sedang berlangsung di background (server tetap lanjut)")
		return client, nil
	}

	if token.Error() != nil {
		log.Printf("[MQTT] Koneksi awal gagal (%v), mencoba ulang di background...", token.Error())
		return client, token.Error()
	}

	log.Println("[MQTT] Berhasil terhubung ke broker HiveMQ")
	return client, nil
}

