package tests

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type ClientMessage struct {
	Client    *websocket.Conn
	Message   []byte
	Timestamp time.Time
	Type      int // websocket.TextMessage, websocket.BinaryMessage, etc.
}

type MockWebSocketServer struct {
	addr       string
	clients    map[*websocket.Conn]bool
	clientsMu  sync.RWMutex
	messages   []ClientMessage
	messagesMu sync.RWMutex
	upgrader   websocket.Upgrader
	server     *http.Server
}

func NewMockWebSocketServer(addr string) *MockWebSocketServer {
	return &MockWebSocketServer{
		addr:     addr,
		clients:  make(map[*websocket.Conn]bool),
		messages: make([]ClientMessage, 0),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for testing
			},
		},
	}
}

func (m *MockWebSocketServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", m.handleWebSocket)

	m.server = &http.Server{
		Addr:    m.addr,
		Handler: mux,
	}

	log.Printf("Mock WebSocket server starting on %s", m.addr)
	return m.server.ListenAndServe()
}

func (m *MockWebSocketServer) Stop() error {
	if m.server != nil {
		return m.server.Close()
	}
	return nil
}

func (m *MockWebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	// Add client to the list
	m.clientsMu.Lock()
	m.clients[conn] = true
	m.clientsMu.Unlock()

	log.Printf("Client connected from %s", conn.RemoteAddr())

	// Remove client when done
	defer func() {
		m.clientsMu.Lock()
		delete(m.clients, conn)
		m.clientsMu.Unlock()
		log.Printf("Client disconnected: %s", conn.RemoteAddr())
	}()

	// Keep connection alive - read and cache messages
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		log.Printf("Received message from %s: %s", conn.RemoteAddr(), message)

		// Cache the message
		m.messagesMu.Lock()
		m.messages = append(m.messages, ClientMessage{
			Client:    conn,
			Message:   message,
			Timestamp: time.Now(),
			Type:      messageType,
		})
		m.messagesMu.Unlock()

		// Handle ping messages
		if messageType == websocket.PingMessage {
			conn.WriteMessage(websocket.PongMessage, message)
		}
	}
}

// SendMessage sends a message to all connected clients
func (m *MockWebSocketServer) SendMessage(message []byte) {
	m.clientsMu.RLock()
	defer m.clientsMu.RUnlock()

	for conn := range m.clients {
		err := conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("Failed to send message to client: %v", err)
			// Remove failed connection
			go func(c *websocket.Conn) {
				m.clientsMu.Lock()
				delete(m.clients, c)
				m.clientsMu.Unlock()
				c.Close()
			}(conn)
		}
	}
}

// SendToClient sends a message to a specific client
func (m *MockWebSocketServer) SendToClient(conn *websocket.Conn, message []byte) error {
	return conn.WriteMessage(websocket.TextMessage, message)
}

// GetConnectedClients returns the number of connected clients
func (m *MockWebSocketServer) GetConnectedClients() int {
	m.clientsMu.RLock()
	defer m.clientsMu.RUnlock()
	return len(m.clients)
}

// GetMessagesFromClient returns all messages received from a specific client
func (m *MockWebSocketServer) GetMessagesFromClient(client *websocket.Conn) []ClientMessage {
	m.messagesMu.RLock()
	defer m.messagesMu.RUnlock()

	var clientMessages []ClientMessage
	for _, msg := range m.messages {
		if msg.Client == client {
			clientMessages = append(clientMessages, msg)
		}
	}
	return clientMessages
}

// GetAllMessages returns all messages received from all clients
func (m *MockWebSocketServer) GetAllMessages() []ClientMessage {
	m.messagesMu.RLock()
	defer m.messagesMu.RUnlock()

	// Return a copy of the slice
	result := make([]ClientMessage, len(m.messages))
	copy(result, m.messages)
	return result
}

// ClearMessages clears all cached messages
func (m *MockWebSocketServer) ClearMessages() {
	m.messagesMu.Lock()
	defer m.messagesMu.Unlock()
	m.messages = m.messages[:0]
}
