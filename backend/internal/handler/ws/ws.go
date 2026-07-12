package ws

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/your-org/job-scheduler/internal/platform/cache"
	"github.com/your-org/job-scheduler/internal/platform/logger"
)

type Hub struct {
	clients   map[*websocket.Conn]bool
	clientsMu sync.RWMutex
	redis     cache.Client
	log       *logger.Logger
	broadcast chan []byte
}

func NewHub(rdb cache.Client, log *logger.Logger) *Hub {
	h := &Hub{
		clients:   make(map[*websocket.Conn]bool),
		redis:     rdb,
		log:       log.WithField("component", "ws-hub"),
		broadcast: make(chan []byte, 256),
	}

	// Start the background broadcast loop
	go h.run()

	// Start subscribing to Redis
	go h.subscribeToRedis()

	return h
}

func (h *Hub) run() {
	for msg := range h.broadcast {
		h.clientsMu.RLock()
		for client := range h.clients {
			if err := client.WriteMessage(websocket.TextMessage, msg); err != nil {
				h.log.Error("websocket write error", logger.Err(err))

				client.Close()
				// This is read-lock context, so we let the unregister handler clean it up.
			}
		}
		h.clientsMu.RUnlock()
	}
}

func (h *Hub) subscribeToRedis() {
	pubsub := h.redis.Subscribe(context.Background(), "scheduler_events")
	defer pubsub.Close()

	for {
		msg, err := pubsub.ReceiveMessage(context.Background())
		if err != nil {
			h.log.Error("redis subscribe error", logger.Err(err))
			time.Sleep(time.Second)
			continue
		}

		h.broadcast <- []byte(msg.Payload)
	}
}

// Publish is used by other parts of the system to broadcast events globally.
func Publish(ctx context.Context, rdb cache.Client, eventType string, data any) error {
	payload, err := json.Marshal(map[string]any{
		"type": eventType,
		"data": data,
		"time": time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	return rdb.Publish(ctx, "scheduler_events", payload)
}

func (h *Hub) RegisterRoutes(router fiber.Router) {
	router.Use("/live", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	router.Get("/live", websocket.New(func(c *websocket.Conn) {
		h.clientsMu.Lock()
		h.clients[c] = true
		h.clientsMu.Unlock()

		defer func() {
			h.clientsMu.Lock()
			delete(h.clients, c)
			h.clientsMu.Unlock()
			c.Close()
		}()

		// Keep connection alive, listen for client pings or messages
		for {
			mt, msg, err := c.ReadMessage()
			if err != nil {
				break
			}

			// Handle ping/pong if necessary. Usually browser websocket handles this automatically.
			if mt == websocket.TextMessage && string(msg) == "ping" {
				_ = c.WriteMessage(websocket.TextMessage, []byte("pong"))
			}
		}
	}))
}
