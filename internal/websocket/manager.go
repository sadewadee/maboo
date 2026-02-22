package websocket

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/maboo-dev/maboo/internal/protocol"
)

// Client represents a single WebSocket connection.
type Client struct {
	ID         string
	Conn       *websocket.Conn
	RemoteAddr string
	Rooms      map[string]bool
	mu         sync.Mutex
}

// Send sends a message to this WebSocket client.
func (c *Client) Send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteMessage(websocket.TextMessage, data)
}

// Manager manages all WebSocket connections, rooms, and message routing.
type Manager struct {
	clients    map[string]*Client
	rooms      map[string]map[string]*Client
	mu         sync.RWMutex
	logger     *slog.Logger
	onMessage  func(client *Client, message []byte) // handler for incoming messages
	phpForward func(frame *protocol.Frame) (*protocol.Frame, error)
}

// NewManager creates a new WebSocket connection manager.
func NewManager(logger *slog.Logger) *Manager {
	return &Manager{
		clients: make(map[string]*Client),
		rooms:   make(map[string]map[string]*Client),
		logger:  logger,
	}
}

// SetPHPForwarder sets the function to forward WebSocket messages to PHP workers.
func (m *Manager) SetPHPForwarder(fn func(frame *protocol.Frame) (*protocol.Frame, error)) {
	m.phpForward = fn
}

// AddConnection registers a new WebSocket connection.
func (m *Manager) AddConnection(conn *websocket.Conn, r *http.Request) *Client {
	id := generateConnID()
	client := &Client{
		ID:         id,
		Conn:       conn,
		RemoteAddr: r.RemoteAddr,
		Rooms:      make(map[string]bool),
	}

	m.mu.Lock()
	m.clients[id] = client
	m.mu.Unlock()

	// Notify PHP worker of new connection
	if m.phpForward != nil {
		header := &protocol.StreamHeader{
			ConnectionID: id,
			Event:        "connect",
		}
		frame, _ := protocol.EncodeStreamData(0, header, nil)
		m.phpForward(frame)
	}

	return client
}

// RemoveConnection unregisters a WebSocket connection and removes it from all rooms.
func (m *Manager) RemoveConnection(id string) {
	m.mu.Lock()
	client, exists := m.clients[id]
	if !exists {
		m.mu.Unlock()
		return
	}

	// Remove from all rooms
	for room := range client.Rooms {
		if members, ok := m.rooms[room]; ok {
			delete(members, id)
			if len(members) == 0 {
				delete(m.rooms, room)
			}
		}
	}

	delete(m.clients, id)
	m.mu.Unlock()

	// Notify PHP worker of disconnection
	if m.phpForward != nil {
		header := &protocol.StreamHeader{
			ConnectionID: id,
			Event:        "close",
		}
		frame, _ := protocol.EncodeStreamData(0, header, nil)
		m.phpForward(frame)
	}
}

// HandleMessage processes an incoming WebSocket message.
func (m *Manager) HandleMessage(client *Client, message []byte) {
	if m.phpForward != nil {
		header := &protocol.StreamHeader{
			ConnectionID: client.ID,
			Event:        "message",
		}
		frame, err := protocol.EncodeStreamData(0, header, message)
		if err != nil {
			m.logger.Error("encoding stream data", "error", err)
			return
		}

		resp, err := m.phpForward(frame)
		if err != nil {
			m.logger.Error("forwarding to PHP", "error", err)
			return
		}

		// If PHP responds with a stream frame, forward it back to the client
		if resp != nil && resp.Type == protocol.TypeStreamData {
			streamHeader, data, err := protocol.DecodeStreamData(resp)
			if err != nil {
				m.logger.Error("decoding PHP stream response", "error", err)
				return
			}

			// Route response based on PHP's instruction
			if streamHeader.Room != "" {
				m.BroadcastToRoom(streamHeader.Room, data, "")
			} else if streamHeader.ConnectionID != "" {
				m.SendToClient(streamHeader.ConnectionID, data)
			}
		}
	}
}

// JoinRoom adds a client to a room.
func (m *Manager) JoinRoom(clientID, room string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[clientID]
	if !exists {
		return
	}

	if _, ok := m.rooms[room]; !ok {
		m.rooms[room] = make(map[string]*Client)
	}
	m.rooms[room][clientID] = client
	client.Rooms[room] = true
}

// LeaveRoom removes a client from a room.
func (m *Manager) LeaveRoom(clientID, room string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[clientID]
	if !exists {
		return
	}

	if members, ok := m.rooms[room]; ok {
		delete(members, clientID)
		if len(members) == 0 {
			delete(m.rooms, room)
		}
	}
	delete(client.Rooms, room)
}

// BroadcastToRoom sends a message to all clients in a room.
func (m *Manager) BroadcastToRoom(room string, data []byte, excludeID string) {
	m.mu.RLock()
	members, exists := m.rooms[room]
	if !exists {
		m.mu.RUnlock()
		return
	}
	// Copy to avoid holding lock during sends
	clients := make([]*Client, 0, len(members))
	for _, c := range members {
		if c.ID != excludeID {
			clients = append(clients, c)
		}
	}
	m.mu.RUnlock()

	for _, c := range clients {
		if err := c.Send(data); err != nil {
			m.logger.Warn("broadcast send failed", "conn_id", c.ID, "room", room, "error", err)
		}
	}
}

// SendToClient sends a message to a specific client.
func (m *Manager) SendToClient(clientID string, data []byte) {
	m.mu.RLock()
	client, exists := m.clients[clientID]
	m.mu.RUnlock()

	if !exists {
		return
	}
	if err := client.Send(data); err != nil {
		m.logger.Warn("send to client failed", "conn_id", clientID, "error", err)
	}
}

// Broadcast sends a message to all connected clients.
func (m *Manager) Broadcast(data []byte, excludeID string) {
	m.mu.RLock()
	clients := make([]*Client, 0, len(m.clients))
	for _, c := range m.clients {
		if c.ID != excludeID {
			clients = append(clients, c)
		}
	}
	m.mu.RUnlock()

	for _, c := range clients {
		if err := c.Send(data); err != nil {
			m.logger.Warn("broadcast send failed", "conn_id", c.ID, "error", err)
		}
	}
}

// Stats returns current WebSocket statistics.
func (m *Manager) Stats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ManagerStats{
		TotalConnections: len(m.clients),
		TotalRooms:       len(m.rooms),
	}
}

// ManagerStats holds WebSocket manager metrics.
type ManagerStats struct {
	TotalConnections int `json:"total_connections"`
	TotalRooms       int `json:"total_rooms"`
}

func generateConnID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
