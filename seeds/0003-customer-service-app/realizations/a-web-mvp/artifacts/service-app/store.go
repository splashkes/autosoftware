package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// --- Ticket types ---

type TicketStatus string

const (
	StatusNew             TicketStatus = "new"
	StatusOpen            TicketStatus = "open"
	StatusPendingCustomer TicketStatus = "pending_customer"
	StatusResolved        TicketStatus = "resolved"
	StatusClosed          TicketStatus = "closed"
)

func (s TicketStatus) Label() string {
	switch s {
	case StatusNew:
		return "New"
	case StatusOpen:
		return "Open"
	case StatusPendingCustomer:
		return "Pending Customer"
	case StatusResolved:
		return "Resolved"
	case StatusClosed:
		return "Closed"
	}
	return string(s)
}

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

func (p Priority) Label() string {
	switch p {
	case PriorityLow:
		return "Low"
	case PriorityNormal:
		return "Normal"
	case PriorityHigh:
		return "High"
	case PriorityUrgent:
		return "Urgent"
	}
	return string(p)
}

type Ticket struct {
	ID             string
	RefCode        string
	Subject        string
	Description    string
	Status         TicketStatus
	Priority       Priority
	AssigneeID     string
	RequesterName  string
	RequesterEmail string
	AccessToken    string
	Messages       []Message
	InternalNotes  []InternalNote
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Message struct {
	ID         string
	Body       string
	AuthorName string
	AuthorType string // "customer" or "agent"
	CreatedAt  time.Time
}

type InternalNote struct {
	ID         string
	Body       string
	AuthorName string
	CreatedAt  time.Time
}

// --- Chat types ---

type ChatStatus string

const (
	ChatWaiting   ChatStatus = "waiting"
	ChatActive    ChatStatus = "active"
	ChatEnded     ChatStatus = "ended"
	ChatEscalated ChatStatus = "escalated"
)

func (s ChatStatus) Label() string {
	switch s {
	case ChatWaiting:
		return "Waiting"
	case ChatActive:
		return "Active"
	case ChatEnded:
		return "Ended"
	case ChatEscalated:
		return "Escalated"
	}
	return string(s)
}

type ChatSession struct {
	ID            string
	CustomerName  string
	CustomerEmail string
	Status        ChatStatus
	AgentID       string
	AgentName     string
	Messages      []ChatMessage
	TicketID      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ChatMessage struct {
	ID         string
	Body       string
	AuthorName string
	AuthorType string // "customer", "agent", "system"
	CreatedAt  time.Time
}

// --- Article types ---

type ArticleStatus string

const (
	ArticleDraft     ArticleStatus = "draft"
	ArticlePublished ArticleStatus = "published"
)

type Article struct {
	ID          string
	Title       string
	Slug        string
	Body        string
	Category    string
	Status      ArticleStatus
	AuthorName  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	PublishedAt *time.Time
}

// --- Agent type ---

type Agent struct {
	ID       string
	Name     string
	Email    string
	Password string
	Role     string // "agent" or "admin"
}

// --- Store ---

type Store struct {
	mu sync.RWMutex

	tickets      map[string]*Ticket
	ticketsByRef map[string]*Ticket
	ticketSeq    int

	chatSessions map[string]*ChatSession
	chatSeq      int
	chatListeners map[string][]chan ChatMessage

	articles     map[string]*Article
	articlesBySlug map[string]*Article
	articleSeq   int

	agents       map[string]*Agent
	agentsByEmail map[string]*Agent
}

func NewStore() *Store {
	s := &Store{
		tickets:       make(map[string]*Ticket),
		ticketsByRef:  make(map[string]*Ticket),
		chatSessions:  make(map[string]*ChatSession),
		chatListeners: make(map[string][]chan ChatMessage),
		articles:      make(map[string]*Article),
		articlesBySlug: make(map[string]*Article),
		agents:        make(map[string]*Agent),
		agentsByEmail: make(map[string]*Agent),
	}
	s.seedData()
	return s
}

func (s *Store) seedData() {
	// Default agents
	s.agents["agent-1"] = &Agent{
		ID: "agent-1", Name: "Admin User",
		Email: "admin@support.local", Password: "admin", Role: "admin",
	}
	s.agentsByEmail["admin@support.local"] = s.agents["agent-1"]

	s.agents["agent-2"] = &Agent{
		ID: "agent-2", Name: "Support Agent",
		Email: "agent@support.local", Password: "agent", Role: "agent",
	}
	s.agentsByEmail["agent@support.local"] = s.agents["agent-2"]

	// Sample KB articles
	now := time.Now()
	s.articleSeq = 2
	a1 := &Article{
		ID: "article-1", Title: "Getting Started", Slug: "getting-started",
		Body:       "Welcome to our support center. Here you can find answers to common questions, create support tickets, or start a live chat with our team.\n\n## How to Get Help\n\n1. **Search our Knowledge Base** - Browse articles for instant answers\n2. **Create a Ticket** - Submit a detailed request and we'll respond via your secure ticket page\n3. **Start a Chat** - Talk to an agent in real time (when available)",
		Category:   "General", Status: ArticlePublished, AuthorName: "Admin User",
		CreatedAt: now, UpdatedAt: now, PublishedAt: &now,
	}
	s.articles["article-1"] = a1
	s.articlesBySlug["getting-started"] = a1

	a2 := &Article{
		ID: "article-2", Title: "How to Track Your Ticket", Slug: "how-to-track-your-ticket",
		Body:       "When you create a support ticket, you receive a **reference code** (like CS-0001) and a **secure link**.\n\n## Using Your Secure Link\n\nClick the link sent to you to view your ticket status, read agent replies, and post follow-up messages.\n\n## Lost Your Link?\n\nVisit our ticket lookup page and enter your email address and reference code to recover access.",
		Category:   "General", Status: ArticlePublished, AuthorName: "Admin User",
		CreatedAt: now, UpdatedAt: now, PublishedAt: &now,
	}
	s.articles["article-2"] = a2
	s.articlesBySlug["how-to-track-your-ticket"] = a2
}

// --- Ticket methods ---

func (s *Store) CreateTicket(subject, description, name, email, accessToken string) *Ticket {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ticketSeq++
	id := fmt.Sprintf("ticket-%d", s.ticketSeq)
	ref := fmt.Sprintf("CS-%04d", s.ticketSeq)
	now := time.Now()
	t := &Ticket{
		ID: id, RefCode: ref, Subject: subject, Description: description,
		Status: StatusNew, Priority: PriorityNormal,
		RequesterName: name, RequesterEmail: email,
		AccessToken: accessToken,
		Messages: []Message{{
			ID: fmt.Sprintf("msg-%d-1", s.ticketSeq), Body: description,
			AuthorName: name, AuthorType: "customer", CreatedAt: now,
		}},
		CreatedAt: now, UpdatedAt: now,
	}
	s.tickets[id] = t
	s.ticketsByRef[ref] = t
	return t
}

func (s *Store) GetTicket(id string) *Ticket {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tickets[id]
}

func (s *Store) GetTicketByRef(ref string) *Ticket {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ticketsByRef[strings.ToUpper(ref)]
}

func (s *Store) ListTickets(status string, assignee string) []*Ticket {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Ticket
	for _, t := range s.tickets {
		if status != "" && string(t.Status) != status {
			continue
		}
		if assignee == "unassigned" && t.AssigneeID != "" {
			continue
		} else if assignee != "" && assignee != "unassigned" && t.AssigneeID != assignee {
			continue
		}
		result = append(result, t)
	}
	// Sort by created descending
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].CreatedAt.After(result[i].CreatedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

func (s *Store) AddTicketMessage(ticketID, body, authorName, authorType string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.tickets[ticketID]
	if t == nil {
		return
	}
	t.Messages = append(t.Messages, Message{
		ID:         fmt.Sprintf("msg-%s-%d", ticketID, len(t.Messages)+1),
		Body:       body,
		AuthorName: authorName,
		AuthorType: authorType,
		CreatedAt:  time.Now(),
	})
	t.UpdatedAt = time.Now()
	// Customer reply reopens ticket
	if authorType == "customer" && (t.Status == StatusPendingCustomer || t.Status == StatusResolved || t.Status == StatusNew) {
		t.Status = StatusOpen
	}
}

func (s *Store) AddTicketNote(ticketID, body, authorName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.tickets[ticketID]
	if t == nil {
		return
	}
	t.InternalNotes = append(t.InternalNotes, InternalNote{
		ID:         fmt.Sprintf("note-%s-%d", ticketID, len(t.InternalNotes)+1),
		Body:       body,
		AuthorName: authorName,
		CreatedAt:  time.Now(),
	})
	t.UpdatedAt = time.Now()
}

func (s *Store) SetTicketStatus(ticketID string, status TicketStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t := s.tickets[ticketID]; t != nil {
		t.Status = status
		t.UpdatedAt = time.Now()
	}
}

func (s *Store) SetTicketPriority(ticketID string, priority Priority) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t := s.tickets[ticketID]; t != nil {
		t.Priority = priority
		t.UpdatedAt = time.Now()
	}
}

func (s *Store) SetTicketAssignee(ticketID, agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.tickets[ticketID]
	if t == nil {
		return
	}
	t.AssigneeID = agentID
	t.UpdatedAt = time.Now()
	if t.Status == StatusNew {
		t.Status = StatusOpen
	}
}

func (s *Store) FindTicketsByEmail(email string) []*Ticket {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Ticket
	emailLower := strings.ToLower(email)
	for _, t := range s.tickets {
		if strings.ToLower(t.RequesterEmail) == emailLower {
			result = append(result, t)
		}
	}
	return result
}

// --- Chat methods ---

func (s *Store) CreateChatSession(name, email string) *ChatSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chatSeq++
	id := fmt.Sprintf("chat-%d", s.chatSeq)
	now := time.Now()
	cs := &ChatSession{
		ID: id, CustomerName: name, CustomerEmail: email,
		Status: ChatWaiting, CreatedAt: now, UpdatedAt: now,
	}
	s.chatSessions[id] = cs
	return cs
}

func (s *Store) GetChatSession(id string) *ChatSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.chatSessions[id]
}

func (s *Store) ListChatSessions(status string) []*ChatSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*ChatSession
	for _, cs := range s.chatSessions {
		if status != "" && string(cs.Status) != status {
			continue
		}
		result = append(result, cs)
	}
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].CreatedAt.After(result[i].CreatedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

func (s *Store) AddChatMessage(sessionID, body, authorName, authorType string) *ChatMessage {
	s.mu.Lock()
	cs := s.chatSessions[sessionID]
	if cs == nil {
		s.mu.Unlock()
		return nil
	}
	msg := ChatMessage{
		ID:         fmt.Sprintf("cmsg-%s-%d", sessionID, len(cs.Messages)+1),
		Body:       body,
		AuthorName: authorName,
		AuthorType: authorType,
		CreatedAt:  time.Now(),
	}
	cs.Messages = append(cs.Messages, msg)
	cs.UpdatedAt = time.Now()
	listeners := s.chatListeners[sessionID]
	s.mu.Unlock()

	// Notify listeners outside the lock
	for _, ch := range listeners {
		select {
		case ch <- msg:
		default:
		}
	}
	return &msg
}

func (s *Store) JoinChat(sessionID, agentID, agentName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cs := s.chatSessions[sessionID]
	if cs == nil {
		return
	}
	cs.AgentID = agentID
	cs.AgentName = agentName
	cs.Status = ChatActive
	cs.UpdatedAt = time.Now()
}

func (s *Store) SetChatStatus(sessionID string, status ChatStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cs := s.chatSessions[sessionID]; cs != nil {
		cs.Status = status
		cs.UpdatedAt = time.Now()
	}
}

func (s *Store) SetChatTicketID(sessionID, ticketID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cs := s.chatSessions[sessionID]; cs != nil {
		cs.TicketID = ticketID
		cs.UpdatedAt = time.Now()
	}
}

func (s *Store) SubscribeChat(sessionID string) chan ChatMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan ChatMessage, 16)
	s.chatListeners[sessionID] = append(s.chatListeners[sessionID], ch)
	return ch
}

func (s *Store) UnsubscribeChat(sessionID string, ch chan ChatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	listeners := s.chatListeners[sessionID]
	for i, l := range listeners {
		if l == ch {
			s.chatListeners[sessionID] = append(listeners[:i], listeners[i+1:]...)
			break
		}
	}
	close(ch)
}

// --- Article methods ---

func (s *Store) CreateArticle(title, slug, body, category, authorName string) *Article {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.articleSeq++
	id := fmt.Sprintf("article-%d", s.articleSeq)
	now := time.Now()
	a := &Article{
		ID: id, Title: title, Slug: slug, Body: body,
		Category: category, Status: ArticleDraft,
		AuthorName: authorName, CreatedAt: now, UpdatedAt: now,
	}
	s.articles[id] = a
	s.articlesBySlug[slug] = a
	return a
}

func (s *Store) GetArticle(id string) *Article {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.articles[id]
}

func (s *Store) GetArticleBySlug(slug string) *Article {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.articlesBySlug[slug]
}

func (s *Store) ListArticles(onlyPublished bool, category string) []*Article {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Article
	for _, a := range s.articles {
		if onlyPublished && a.Status != ArticlePublished {
			continue
		}
		if category != "" && a.Category != category {
			continue
		}
		result = append(result, a)
	}
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].UpdatedAt.After(result[i].UpdatedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

func (s *Store) UpdateArticle(id, title, slug, body, category string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.articles[id]
	if a == nil {
		return
	}
	// Remove old slug mapping if slug changed
	if a.Slug != slug {
		delete(s.articlesBySlug, a.Slug)
		s.articlesBySlug[slug] = a
	}
	a.Title = title
	a.Slug = slug
	a.Body = body
	a.Category = category
	a.UpdatedAt = time.Now()
}

func (s *Store) SetArticleStatus(id string, status ArticleStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.articles[id]
	if a == nil {
		return
	}
	a.Status = status
	a.UpdatedAt = time.Now()
	if status == ArticlePublished {
		now := time.Now()
		a.PublishedAt = &now
	}
}

func (s *Store) DeleteArticle(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.articles[id]
	if a == nil {
		return
	}
	delete(s.articlesBySlug, a.Slug)
	delete(s.articles, id)
}

func (s *Store) SearchArticles(query string) []*Article {
	s.mu.RLock()
	defer s.mu.RUnlock()
	q := strings.ToLower(query)
	var result []*Article
	for _, a := range s.articles {
		if a.Status != ArticlePublished {
			continue
		}
		if strings.Contains(strings.ToLower(a.Title), q) ||
			strings.Contains(strings.ToLower(a.Body), q) ||
			strings.Contains(strings.ToLower(a.Category), q) {
			result = append(result, a)
		}
	}
	return result
}

func (s *Store) ArticleCategories() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := make(map[string]bool)
	var cats []string
	for _, a := range s.articles {
		if a.Category != "" && !seen[a.Category] {
			seen[a.Category] = true
			cats = append(cats, a.Category)
		}
	}
	return cats
}

// --- Agent methods ---

func (s *Store) GetAgent(id string) *Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.agents[id]
}

func (s *Store) GetAgentByEmail(email string) *Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.agentsByEmail[email]
}

func (s *Store) ListAgents() []*Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Agent
	for _, a := range s.agents {
		result = append(result, a)
	}
	return result
}
