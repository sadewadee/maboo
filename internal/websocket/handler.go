package websocket

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // TODO: configurable origin check
	},
}

// Handler handles WebSocket upgrade requests and manages connections.
type Handler struct {
	manager *Manager
	logger  *slog.Logger
}

// NewHandler creates a new WebSocket handler.
func NewHandler(manager *Manager, logger *slog.Logger) *Handler {
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	client := h.manager.AddConnection(conn, r)
	h.logger.Debug("websocket connected", "conn_id", client.ID)

	// Read loop
	go h.readPump(client)
}

func (h *Handler) readPump(client *Client) {
	defer func() {
		h.manager.RemoveConnection(client.ID)
		client.Conn.Close()
		h.logger.Debug("websocket disconnected", "conn_id", client.ID)
	}()

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				h.logger.Warn("websocket read error", "conn_id", client.ID, "error", err)
			}
			break
		}

		h.manager.HandleMessage(client, message)
	}
}
