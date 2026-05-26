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

type overflowPolicy int

const (
	Block overflowPolicy = iota // block producer when full
	DropNewest                  // drop incoming when full
	DropOldest                  // evict one oldest, then enqueue
)

type inbox struct {
	ch      chan *Message
	policy  overflowPolicy
	dropped atomic.Uint64
	closed  atomic.Bool
}

func newInbox(size int, policy overflowPolicy) *inbox {
	if size < 0 {
		size = 10
	}
	return &inbox{ch: make(chan *Message, size), policy: policy}
}

func (ib *inbox) handler(_ mqtt.Client, m mqtt.Message) {
	if ib.closed.Load() {
		return
	}
	msg := &Message {
		Topic:    m.Topic(),
		Payload:  append([]byte(nil), m.Payload()...), // copy
		QoS:      m.Qos(),
		Retained: m.Retained(),
		Received: time.Now(),
	}
	switch ib.policy {
		case Block:
			ib.ch <- msg
		case DropNewest:
			select {
				case ib.ch <- msg:
				default:
					ib.dropped.Add(1)
			}
		case DropOldest:
			select {
				case ib.ch <- msg:
				default:
					select {
						case <-ib.ch:
						default:
					}
					select {
						case ib.ch <- msg:
						default:
							ib.dropped.Add(1)
					}
			}
	}
}

func (ib *inbox) receive() (*Message, bool) {
	m, ok := <-ib.ch
	return m, ok
}

func (ib *inbox) receiveCtx(ctx context.Context) (*Message, bool) {
	select {
		case <-ctx.Done():
			return &Message{}, false
		case m, ok := <-ib.ch:
			return m, ok
	}
}

func (ib *inbox) tryReceive() (*Message, bool) {
	select {
		case m, ok := <-ib.ch:
			return m, ok;
		default:
			return &Message{}, false
	}
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
	policy     	overflowPolicy
	autoCreate 	bool
	// optional: a fallback inbox when autoCreate=false and topic not found
	fallback 	*inbox
}

func newTopicInboxes(size int, policy overflowPolicy, autoCreate bool) *topicInboxes {
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


func (t *topicInboxes) receive(topic string) (*Message, bool) {
	if ib, ok := t.inboxFor(topic); ok && ib != nil {
		return ib.receive()
	}
	return &Message{}, false
}


/* *********************************************************************************************************
 * mqtt configuration and client creation
 * *********************************************************************************************************/

type mqttConfig struct {
	brokers        []string
	clientID       string
	username       string
	password       string
	tlsConfig      *tls.Config
	keepAlive      time.Duration // default 60s
	cleanSession   bool          // default true
	autoReconnect  bool          // default true
	connectRetry   bool          // default true
	actionTimeout  time.Duration // default 5s
	onMessage      mqtt.MessageHandler
	onConnect      mqtt.OnConnectHandler
	onLost         mqtt.ConnectionLostHandler

	subscriptions []struct {
		topic string
		qos   byte
	}
}

func mqttOptions(brokers []string, clientID, username, password string, keepAlive, actionTimeout float64, inboxes *topicInboxes) *mqttConfig {
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
		brokers:		brokers,
		clientID: 		clientID,
		username: 		username,
		password: 		password,
		keepAlive: 		keepAliveDuration,
		cleanSession: 	true,
		autoReconnect:	true,
		connectRetry:	true,
		actionTimeout:	actionTimeoutDuration,
		onMessage:		inboxes.handler(),
		// subscriptions:	subscriptions,
	}
}

func (c *mqttConfig) subscribe(topic string, qos byte) {
	c.subscriptions = append(
							c.subscriptions,
							struct{
								topic string
								qos byte
							}{
								topic: topic,
								qos: qos,
							},
						)
}

func (cfg *mqttConfig) newClient() (mqtt.Client, error) {
	if len(cfg.brokers) == 0 {
		cfg.brokers = []string{"tcp://localhost:1883"}
	}
	opts := mqtt.NewClientOptions()
	for _, b := range cfg.brokers {
		opts.AddBroker(b)
	}
	if cfg.clientID != "" {
		opts.SetClientID(cfg.clientID)
	}
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
	if cfg.onLost != nil { opts.SetConnectionLostHandler(cfg.onLost) }

	// Re/subscribe on (re)connect
	opts.SetOnConnectHandler(
		func(c mqtt.Client) {
			if cfg.onConnect != nil {
				cfg.onConnect(c)
			}
			for _, s := range cfg.subscriptions {
				if s.topic == "" { continue }
				tok := c.Subscribe(s.topic, s.qos, cfg.onMessage)
			    tok.WaitTimeout(cfg.actionTimeout)
				// tok.Wait()

				_ = tok.Error() // optionally log
			}
		},
	)

	c := mqtt.NewClient(opts)
	tok := c.Connect()
	if !tok.WaitTimeout(cfg.actionTimeout) {
		c.Disconnect(0)
		//return nil, fmt.Errorf("mqtt connect to %q: timeout after %v", cfg.brokers, cfg.actionTimeout)
	}
	if err := tok.Error(); err != nil {
		c.Disconnect(0)
		return nil, err
	}
	return c, nil
}


/* ********************************************************************************************************************************
 * An mqtt client wrapper
 * ********************************************************************************************************************************/

type MQTTClient struct {
	client		mqtt.Client
	inboxes 	*topicInboxes
	config		*mqttConfig
}

func NewMQTTClient(brokers []string, clientID, username, password string, size int) (*MQTTClient, error) {
	if size <= 0 {
		size = 10
	}
	inboxes := newTopicInboxes(size, DropOldest, true)
	config  := mqttOptions(brokers, clientID, username, password, 0, 0, inboxes)
	client, err := config.newClient()
	if err != nil {
		return nil, err
	}

	return &MQTTClient {
		client: client,
		inboxes: inboxes,
		config: config,
	}, nil
}

func (c *MQTTClient) Subscribe(topic string, qos byte) error {
    if topic == "" {
        return fmt.Errorf("subscribe: empty topic")
    }
    if qos < 0 || qos > 2 {
        return fmt.Errorf("subscribe: invalid QoS %d", qos)
    }

    // Save for re-subscribe on reconnect
    c.config.subscribe(topic, qos)

    tok := c.client.Subscribe(topic, qos, c.config.onMessage)
    if !tok.WaitTimeout(c.config.actionTimeout) {
        return fmt.Errorf("subscribe %q: timeout after %v", topic, c.config.actionTimeout)
    }
    // tok.Wait()
    return tok.Error()
}

func (c *MQTTClient) Publish(topic string, qos byte, retain bool, payload []byte) error {
	if topic == "" {
		return fmt.Errorf("empty topic")
	}
	tok := c.client.Publish(topic, qos, retain, payload)
	if !tok.WaitTimeout(c.config.actionTimeout) {
		return fmt.Errorf("publish on topic %q: timeout after %v", topic, c.config.actionTimeout)
	}
	return tok.Error()
}

func (c *MQTTClient) Receive(topic string) (*Message, bool) {
	return c.inboxes.receive(topic)
}
