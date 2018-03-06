package pubsub

import (
	"fmt"
	"net"
	"time"

	"github.com/launchdarkly/foundation/concurrent"
	"github.com/launchdarkly/foundation/fmetrics"
	"github.com/launchdarkly/foundation/logger"
	msg "github.com/launchdarkly/foundation/message"
	"github.com/launchdarkly/go-metrics"
	"github.com/nats-io/go-nats"
)

const (
	pingInterval   = time.Second * 1
	dialTimeout    = time.Second * 3
	timeout        = time.Second * 5
	flusherTimeout = time.Second * 5
	maxReconnect   = 2147483647 // basically retry forever.
	reconnectWait  = time.Millisecond * 500
)

type NatsConfig struct {
	Name        string //this should be a short, graphite-friendly name.
	Url         []string
	Disabled    bool
	NoRandomize bool
}

type Nats struct {
	name                    string
	conn                    *nats.Conn
	metrix                  metrics.Registry
	allPublishMsgsCounter   metrics.Counter
	allSubscribeMsgsCounter metrics.Counter
	publishErrCounter       metrics.Counter
	subscribeErrCounter     metrics.Counter
	unsubscribeErrCounter   metrics.Counter
	reconnectCounter        metrics.Counter
	disconnectCounter       metrics.Counter
	asyncErrorCounter       metrics.Counter
}

type NatsSubscription struct {
	subscription          *nats.Subscription
	unsubscribeErrCounter metrics.Counter
}

func (n NatsSubscription) Unsubscribe() error {
	err := n.subscription.Unsubscribe()
	if err != nil {
		n.unsubscribeErrCounter.Inc(1)
	}
	return err
}

func NewNatsPublisher(config NatsConfig) (Publisher, error) {
	if config.Disabled {
		logger.Info.Printf("[%s]Nats Config is disabled! using no-op Nats publisher", config.Name)
		return &NoopPublisher{}, nil
	}
	return newNats(config)
}

func NewNatsSubscriber(config NatsConfig) (Subscriber, error) {
	if config.Disabled {
		logger.Info.Printf("[%s]Nats Config is disabled! using no-op Nats subscriber", config.Name)
		return &NoopSubscriber{}, nil
	}
	return newNats(config)
}

func newNats(natsConfig NatsConfig) (*Nats, error) {
	if natsConfig.Name == "" {
		natsConfig.Name = "main"
	}
	if len(natsConfig.Url) == 0 {
		return nil, fmt.Errorf("[%s]Missing required url field in NATS config!", natsConfig.Name)
	}

	m := metrics.NewPrefixedChildRegistry(metrics.DefaultRegistry, "pubsub.nats."+natsConfig.Name+".")
	allPublishMsgsCounter := metrics.GetOrRegisterCounter("publish.msgs.all", m)
	allSubscribeMsgsCounter := metrics.GetOrRegisterCounter("subscribe.msgs.all", m)
	publishErrCounter := metrics.GetOrRegisterCounter("publish.error", m)
	subscribeErrCounter := metrics.GetOrRegisterCounter("subscribe.error", m)
	unsubscribeErrCounter := metrics.GetOrRegisterCounter("unsubscribe.error", m)
	reconnectCounter := metrics.GetOrRegisterCounter("reconnect", m)
	disconnectCounter := metrics.GetOrRegisterCounter("disconnect", m)
	asyncErrorCounter := metrics.GetOrRegisterCounter("asyncError", m)

	opts := nats.GetDefaultOptions()
	opts.Name = natsConfig.Name
	opts.Servers = natsConfig.Url
	opts.NoRandomize = natsConfig.NoRandomize
	opts.Pedantic = true
	opts.MaxReconnect = maxReconnect
	opts.ReconnectWait = reconnectWait
	opts.PingInterval = pingInterval
	opts.Timeout = timeout
	dialer := net.Dialer{
		Timeout: dialTimeout,
	}
	opts.CustomDialer = &dialer
	opts.FlusherTimeout = flusherTimeout
	opts.ReconnectedCB = func(c *nats.Conn) {
		logger.Info.Printf("[%s]Reconnected NATS client. Total client reconnects: %d", c.Opts.Name, c.Statistics.Reconnects)
		logConnectionInfo(c)
		reconnectCounter.Inc(1)
	}
	opts.DisconnectedCB = func(c *nats.Conn) {
		logger.Warn.Printf("[%s]NATS client disconnected!", c.Opts.Name)
		disconnectCounter.Inc(1)
	}
	opts.AsyncErrorCB = func(c *nats.Conn, s *nats.Subscription, err error) {
		logger.Warn.Printf("[%s]NATS client got async error: %s. Subject: %s connected url: %s",
			c.Opts.Name, err.Error(), s.Subject, c.ConnectedUrl())
		logConnectionInfo(c)
		asyncErrorCounter.Inc(1)
	}

	logOptions(opts, dialer)
	conn, err := opts.Connect()
	if err != nil {
		return nil, fmt.Errorf("[%s]Could not initialize NATS client: %s", natsConfig.Name, err.Error())
	}
	metrics.NewPrefixedChildRegistry(metrics.DefaultRegistry, "pubsub.nats."+natsConfig.Name+".")
	metrics.NewRegisteredFunctionalGauge("conn.OutBytes", m, func() int64 {
		return int64(conn.OutBytes)
	})
	metrics.NewRegisteredFunctionalGauge("conn.OutMsgs", m, func() int64 {
		return int64(conn.OutMsgs)
	})
	metrics.NewRegisteredFunctionalGauge("conn.InMsgs", m, func() int64 {
		return int64(conn.InMsgs)
	})
	metrics.NewRegisteredFunctionalGauge("conn.InBytes", m, func() int64 {
		return int64(conn.InBytes)
	})
	n := Nats{
		name:                    natsConfig.Name,
		conn:                    conn,
		metrix:                  m,
		allPublishMsgsCounter:   allPublishMsgsCounter,
		allSubscribeMsgsCounter: allSubscribeMsgsCounter,
		publishErrCounter:       publishErrCounter,
		unsubscribeErrCounter:   unsubscribeErrCounter,
		subscribeErrCounter:     subscribeErrCounter,
		reconnectCounter:        reconnectCounter,
		disconnectCounter:       disconnectCounter,
		asyncErrorCounter:       asyncErrorCounter,
	}
	logConnectionInfo(conn)
	concurrent.GoSafely(func() {
		for range time.Tick(1 * time.Minute) {
			logConnectionInfo(n.conn)
		}
	})
	return &n, nil
}

func logOptions(o nats.Options, dialer net.Dialer) {
	logger.Info.Printf("[%s]Initializing NATS pubsub client with options:", o.Name)
	logger.Info.Printf("[%s]  Name: %v", o.Name, o.Name)
	logger.Info.Printf("[%s]  Servers: %v", o.Name, o.Servers)
	logger.Info.Printf("[%s]  Verbose: %v", o.Name, o.Verbose)
	logger.Info.Printf("[%s]  Pedantic: %v", o.Name, o.Pedantic)
	logger.Info.Printf("[%s]  NoRandomize: %v", o.Name, o.NoRandomize)
	logger.Info.Printf("[%s]  AllowReconnect: %v", o.Name, o.AllowReconnect)
	logger.Info.Printf("[%s]  MaxReconnect: %d", o.Name, o.MaxReconnect)
	logger.Info.Printf("[%s]  ReconnectWait: %v", o.Name, o.ReconnectWait)
	logger.Info.Printf("[%s]  Timeout: %v", o.Name, o.Timeout)
	logger.Info.Printf("[%s]  FlusherTimeout: %v", o.Name, o.FlusherTimeout)
	logger.Info.Printf("[%s]  PingInterval: %v", o.Name, o.PingInterval)
	logger.Info.Printf("[%s]  MaxPingsOut: %d", o.Name, o.MaxPingsOut)
	logger.Info.Printf("[%s]  DialerTimeout: %v", o.Name, dialer.Timeout)
}

func logConnectionInfo(c *nats.Conn) {
	logger.Info.Printf("[%s]NATS client: Connected? %v, Connected URL: %s Server Id: %s, Discovered Servers: %s",
		c.Opts.Name, c.IsConnected(), c.ConnectedUrl(), c.ConnectedServerId(), c.DiscoveredServers())
}

func (n *Nats) Publish(message msg.BaseMessage) error {
	channel := message.Channel()
	n.recordPublishMetrics(channel)
	err := n.conn.Publish(channel, message.Payload(true))
	if err != nil {
		n.publishErrCounter.Inc(1)
	}
	return err
}

func (n *Nats) IsConnected() bool {
	return n != nil && n.conn != nil && n.conn.IsConnected()
}

func (n *Nats) Close() {
	logger.Info.Printf("[%s]Closing NATS client", n.name)
	if n != nil && n.conn != nil {
		n.conn.Close()
	}
}

func (n *Nats) SubscribeCallback(channel string, cb func([]byte)) (Subscription, error) {
	sub, err := n.conn.Subscribe(channel, func(message *nats.Msg) {
		n.recordSubscribeMetrics(channel)
		cb(message.Data)
	})
	if err != nil {
		n.subscribeErrCounter.Inc(1)
		return nil, err
	}
	return n.newNatsSubscription(sub), nil
}

func (n *Nats) newNatsSubscription(ns *nats.Subscription) Subscription {
	return NatsSubscription{
		subscription: ns, unsubscribeErrCounter: n.unsubscribeErrCounter}
}

func (n *Nats) recordPublishMetrics(channel string) {
	metricName := "publish.msgs." + fmetrics.SanitizeForGraphite(channel)
	metrics.GetOrRegisterCounter(metricName, n.metrix).Inc(1)
	n.allPublishMsgsCounter.Inc(1)
}

func (n *Nats) recordSubscribeMetrics(channel string) {
	metricName := "subscribe.msgs." + fmetrics.SanitizeForGraphite(channel)
	metrics.GetOrRegisterCounter(metricName, n.metrix).Inc(1)
	n.allSubscribeMsgsCounter.Inc(1)
}
