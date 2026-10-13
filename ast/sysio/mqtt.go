package sysio


import (
	"fmt"
	"time"
	"crypto/tls"

	"context"
	"sync"
	"sync/atomic"

	"math"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"sessions/ast/util"
)

/* *********************************************************************************************************
 * The basic Message struct for storing mqtt messages
 * *********************************************************************************************************/

type Message struct {
	Topic    string
	Payload  []byte
	QoS      byte
	Retained bool
	Received time.Time
}

/* *********************************************************************************************************
 * Inboxes for internal management of received mqtt messages
 * *********************************************************************************************************/

/******** Inbox (per-topic buffer) ********/

type OverflowPolicy int

const (
	Block OverflowPolicy = iota // block producer when full
	DropNewest                  // drop incoming when full
	DropOldest                  // evict one oldest, then enqueue
	Latest						// always read the latest available message
)

type inbox struct {
	ch      chan *Message
	latest	*Message
	policy  OverflowPolicy
	dropped atomic.Uint64
	closed  atomic.Bool
}

func newInbox(size int, policy OverflowPolicy) *inbox {
	if size < 0 {
		size = 10
	}
	return &inbox{ch: make(chan *Message, size), latest: &Message{}, policy: policy}
}

func (ib *inbox) handler(_ mqtt.Client, m mqtt.Message) {
	if ib.closed.Load() {
		return
	}
	ib.latest = &Message {
		Topic:    m.Topic(),
		Payload:  append([]byte(nil), m.Payload()...), // copy
		QoS:      m.Qos(),
		Retained: m.Retained(),
		Received: time.Now(),
	}
	switch ib.policy {
		case Block:
			ib.ch <- ib.latest
		case DropNewest:
			select {
				case ib.ch <- ib.latest:
				default:
					ib.dropped.Add(1)
			}
		case DropOldest:
			select {
				case ib.ch <- ib.latest:
				default:
					select {
						case <-ib.ch:
						default:
					}
					select {
						case ib.ch <- ib.latest:
						default:
							ib.dropped.Add(1)
					}
			}
	}
}

func (ib *inbox) receive() (*Message, error) {
	if ib.policy != Latest {
		m, ok := <-ib.ch
		if !ok {
			return m, fmt.Errorf("closed topic")
		}
		return m, nil
	}
	return ib.latest, nil
}

func (ib *inbox) receiveCtx(ctx context.Context) (*Message, error) {
	if ib.policy != Latest {
		select {
			case <-ctx.Done():
				return &Message{}, fmt.Errorf("closed context")
			case m, ok := <-ib.ch:
				if !ok {
				return m, fmt.Errorf("closed topic")
			}
			return m, nil
		}
	}
	return ib.latest, nil
}

func (ib *inbox) tryReceive() (*Message, error) {
	select {
		case m, ok := <-ib.ch:
			if !ok {
				return m, fmt.Errorf("closed topic")
			}
			return m, nil
		default:
			return &Message{}, nil
	}
	return ib.latest, nil
}

func (ib *inbox) close() {
	if ib.closed.CompareAndSwap(false, true) { close(ib.ch) }
}

// func (ib *inbox) dropped() uint64 { return ib.dropped.Load() }

/******** Topic router: one Inbox per topic ********/

type topicInboxes struct {
	mu         	sync.RWMutex
	boxes      	map[string]*inbox
	size       	int
	policy     	OverflowPolicy
	autoCreate 	bool
	// optional: a fallback inbox when autoCreate=false and topic not found
	fallback 	*inbox
}

func newTopicInboxes(size int, policy OverflowPolicy, autoCreate bool) *topicInboxes {
	return &topicInboxes {
		boxes:      make(map[string]*inbox),
		size:       size,
		policy:     policy,
		autoCreate: autoCreate,
	}
}

// Ensure creates (or returns existing) inbox for an exact topic name.
func (t *topicInboxes) ensure(topic string) *inbox {
	t.mu.Lock()
	defer t.mu.Unlock()

	if ib, ok := t.boxes[topic]; ok {
		return ib
	}

	ib := newInbox(t.size, t.policy)
	t.boxes[topic] = ib
	return ib
}

// InboxFor returns the inbox for topic, creating it if autoCreate.
func (t *topicInboxes) inboxFor(topic string) (*inbox, bool) {
	if topic == "" {
		return nil, false
	}
	t.mu.RLock()
	ib, ok := t.boxes[topic]
	t.mu.RUnlock()
	if ok {
		return ib, true
	}
	if !t.autoCreate {
		return t.fallback, t.fallback != nil
	}
	return t.ensure(topic), true
}

// Handler returns an mqtt.MessageHandler that dispatches to per-topic inboxes.
func (t *topicInboxes) handler() mqtt.MessageHandler {
	return func(c mqtt.Client, m mqtt.Message) {
		topic := m.Topic()
		if ib, ok := t.inboxFor(topic); ok && ib != nil {
			ib.handler(c, m)
		} else if t.fallback != nil {
			t.fallback.handler(c, m)
		}
		// else: silently drop if no inbox/fallback (or you can log)
	}
}

// Topics snapshot (for discovery/monitoring).
func (t *topicInboxes) Topics() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	keys := make([]string, 0, len(t.boxes))
	for k := range t.boxes {
		keys = append(keys, k)
	}
	return keys
}

// CloseAll closes every inbox (idempotent).
func (t *topicInboxes) CloseAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, ib := range t.boxes {
		ib.close()
	}
	if t.fallback != nil {
		t.fallback.close()
	}
}

func (t *topicInboxes) receive(topic string) (*Message, error) {
	if ib, ok := t.inboxFor(topic); ok && ib != nil {
		return ib.receive()
	}
    return &Message{}, fmt.Errorf("receive: subscribe topic not found: %q", topic)
}

/* *********************************************************************************************************
 * mqtt configuration and client creation
 * *********************************************************************************************************/

type mqttConfig struct {
	broker 			string
	clientID 		string
	username 		string
	password 		string
	tlsConfig 		*tls.Config
	keepAlive 		time.Duration // default 60s
	cleanSession 	bool          // default true
	autoReconnect 	bool          // default true
	connectRetry 	bool          // default true
	actionTimeout 	time.Duration // default 5s
	onMessage 		mqtt.MessageHandler
	onConnect 		mqtt.OnConnectHandler
	onLost 			mqtt.ConnectionLostHandler

	subscriptions	map[string]byte
	log 			error
}

func mqttOptions(	broker string, clientID, username, password string, keepAlive, actionTimeout float64,
					/*subscribeTopics []string, qos byte, */handler mqtt.MessageHandler) *mqttConfig {
	if clientID == "" {
		clientID = fmt.Sprintf("clientID-%v", time.Now().Nanosecond())
	}

	var keepAliveDuration time.Duration
	if keepAlive <= 0 {
		keepAliveDuration = 60 * time.Second
	} else {
		keepAliveDuration = time.Duration(math.Round(keepAlive * float64(time.Second)))
	}

	var actionTimeoutDuration time.Duration
	if actionTimeout <= 0 {
		actionTimeoutDuration = 5 * time.Second
	} else {
		actionTimeoutDuration = time.Duration(math.Round(actionTimeout * float64(time.Second)))
	}

	return &mqttConfig {
		broker:			broker,
		clientID: 		clientID,
		username: 		username,
		password: 		password,
		keepAlive: 		keepAliveDuration,
		cleanSession: 	true,
		autoReconnect:	true,
		connectRetry:	true,
		actionTimeout:	actionTimeoutDuration,
		onMessage:		handler,
		subscriptions:	make(map[string]byte),
	}
}

func (c *mqttConfig) subscribe(topic string, qos byte) bool {
	_, ok := c.subscriptions[topic]
	if !ok {
		c.subscriptions[topic] = qos
		return true
	}
	return false
}

func (cfg *mqttConfig) newClient() (mqtt.Client, error) {
	if cfg.broker == "" {
		cfg.broker = "tcp://localhost:1883"
	}
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.broker)

	opts.SetClientID(cfg.clientID)
	if cfg.username != "" {
		opts.SetUsername(cfg.username)
		opts.SetPassword(cfg.password)
	}
	if cfg.tlsConfig != nil {
		opts.SetTLSConfig(cfg.tlsConfig)
	}
	opts.SetKeepAlive(cfg.keepAlive)
	opts.SetCleanSession(cfg.cleanSession)
	opts.SetAutoReconnect(cfg.autoReconnect)
	opts.SetConnectRetry(cfg.connectRetry)
	if cfg.onMessage != nil { opts.SetDefaultPublishHandler(cfg.onMessage) }
	if cfg.onLost != nil 	{ opts.SetConnectionLostHandler(cfg.onLost) }

	 opts.SetOnConnectHandler(
					func(c mqtt.Client) {
						cfg.log = nil
						for subscription, qos := range cfg.subscriptions {
							if subscription != "" {
								tok := c.Subscribe(subscription, qos, cfg.onMessage)
								tok.Wait()
								if err := tok.Error(); err != nil {
									cfg.log = util.JoinSep(";", cfg.log, err)
								}
							}
						}
					},
	)
	
	c := mqtt.NewClient(opts)
	tok := c.Connect()
	tok.Wait()
	if err := tok.Error(); err != nil {
		c.Disconnect(0)
		return nil, err
	}
	if cfg.log != nil {
		return nil, cfg.log
	}
	return c, nil
}

/* ********************************************************************************************************************************
 * An mqtt client wrapper
 * ********************************************************************************************************************************/

var (
	mqttClientRegistry = make(map[string]*MQTTClient)
	mu         	sync.RWMutex
)

type MQTTClient struct {
	client			mqtt.Client
	inboxes  		*topicInboxes
	config			*mqttConfig
}

func NewMQTTClient(id string, broker string, clientID, username, password string, policy OverflowPolicy, size int) (*MQTTClient, error) {
	mu.Lock()
	defer mu.Unlock()
	client, ok := mqttClientRegistry[id]
	if !ok {
		var err error
		client, err = newMQTTClient(broker, clientID, username, password, policy, size)
		if err != nil {
			return nil, err
		}
		mqttClientRegistry[id] = client
	}
	return client, nil
}

func newMQTTClient(broker string, clientID, username, password string, policy OverflowPolicy, size int) (*MQTTClient, error) {
	if size <= 0 {
		size = 10
	}

	inboxes := newTopicInboxes(size, policy, true) //TODO: how to treat autoCreate

    config  := mqttOptions(	broker, clientID, username, password, 0, 0, inboxes.handler() )

	client, err := config.newClient()
	if err != nil {
		return nil, err
	}

	return &MQTTClient {
		client:			client,
		inboxes: 		inboxes,
		config:			config,
	}, nil
}

func (c *MQTTClient) Subscribe(topic string, qos byte) error {
	mu.Lock()
	defer mu.Unlock()
	if topic == "" {
        return fmt.Errorf("subscribe: empty topic")
    }
    if qos < 0 || qos > 2 {
        return fmt.Errorf("subscribe: invalid QoS %d", qos)
    }

    // Save for re-subscribe on reconnect
    if c.config.subscribe(topic, qos) {
	    tok := c.client.Subscribe(topic, qos, c.config.onMessage)
	    tok.Wait()
	    return tok.Error()
	}
	return nil
}

func (c *MQTTClient) Publish(topic string, qos byte, retained bool, payload []byte) error {
	if topic == "" {
		return fmt.Errorf("empty publish topic")
	}
	tok := c.client.Publish(topic, qos, retained, payload)
	tok.Wait()
	return tok.Error()
}

func (c *MQTTClient) Receive(topic string) (*Message, error) {
	return c.inboxes.receive(topic)
}



