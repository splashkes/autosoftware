package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

// App holds the application state and dependencies.
type App struct {
	store     *Store
	tmpl      map[string]*template.Template
	hmacKey   []byte
	sessions  map[string]string // sessionID -> agentID
	sessionMu sync.RWMutex
	chatTimeout time.Duration
}

func main() {
	addr := os.Getenv("AS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8095"
	}

	chatTimeout := 2 * time.Minute
	if t := os.Getenv("CHAT_TIMEOUT"); t != "" {
		if d, err := time.ParseDuration(t); err == nil {
			chatTimeout = d
		}
	}

	app := &App{
		store:       NewStore(),
		sessions:    make(map[string]string),
		chatTimeout: chatTimeout,
	}

	// Generate HMAC key for ticket access tokens
	app.hmacKey = make([]byte, 32)
	if _, err := rand.Read(app.hmacKey); err != nil {
		log.Fatal("failed to generate HMAC key:", err)
	}

	app.loadTemplates()

	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("GET /", app.handleHome)
	mux.HandleFunc("GET /help/tickets/new", app.handleTicketNew)
	mux.HandleFunc("POST /help/tickets", app.handleTicketCreate)
	mux.HandleFunc("GET /help/tickets/lookup", app.handleTicketLookup)
	mux.HandleFunc("POST /help/tickets/lookup", app.handleTicketLookupSubmit)
	mux.HandleFunc("GET /help/tickets/{refCode}", app.handleTicketView)
	mux.HandleFunc("POST /help/tickets/{refCode}/reply", app.handleTicketCustomerReply)
	mux.HandleFunc("GET /help/articles", app.handleArticles)
	mux.HandleFunc("GET /help/articles/{slug}", app.handleArticleDetail)

	// Chat routes
	mux.HandleFunc("GET /chat", app.handleChatStart)
	mux.HandleFunc("POST /chat", app.handleChatCreate)
	mux.HandleFunc("GET /chat/{sessionID}", app.handleChatWidget)
	mux.HandleFunc("POST /chat/{sessionID}/message", app.handleChatCustomerMessage)
	mux.HandleFunc("GET /chat/{sessionID}/stream", app.handleChatStream)

	// Agent routes
	mux.HandleFunc("GET /agent/login", app.handleLogin)
	mux.HandleFunc("POST /agent/login", app.handleLoginSubmit)
	mux.HandleFunc("POST /agent/logout", app.handleLogout)
	mux.HandleFunc("GET /agent", app.handleInbox)
	mux.HandleFunc("GET /agent/inbox", app.handleInbox)
	mux.HandleFunc("GET /agent/tickets/{id}", app.handleAgentTicketDetail)
	mux.HandleFunc("POST /agent/tickets/{id}/reply", app.handleAgentTicketReply)
	mux.HandleFunc("POST /agent/tickets/{id}/note", app.handleAgentTicketNote)
	mux.HandleFunc("POST /agent/tickets/{id}/assign", app.handleAgentTicketAssign)
	mux.HandleFunc("POST /agent/tickets/{id}/status", app.handleAgentTicketStatus)
	mux.HandleFunc("POST /agent/tickets/{id}/priority", app.handleAgentTicketPriority)

	// Agent chat routes
	mux.HandleFunc("GET /agent/chats", app.handleAgentChatConsole)
	mux.HandleFunc("GET /agent/chats/{id}", app.handleAgentChatSession)
	mux.HandleFunc("POST /agent/chats/{id}/join", app.handleAgentChatJoin)
	mux.HandleFunc("POST /agent/chats/{id}/message", app.handleAgentChatMessage)
	mux.HandleFunc("GET /agent/chats/{id}/stream", app.handleAgentChatStream)
	mux.HandleFunc("POST /agent/chats/{id}/escalate", app.handleAgentChatEscalate)
	mux.HandleFunc("POST /agent/chats/{id}/end", app.handleAgentChatEnd)

	// Agent KB routes
	mux.HandleFunc("GET /agent/articles", app.handleAgentArticleList)
	mux.HandleFunc("GET /agent/articles/new", app.handleAgentArticleNew)
	mux.HandleFunc("POST /agent/articles", app.handleAgentArticleCreate)
	mux.HandleFunc("GET /agent/articles/{id}/edit", app.handleAgentArticleEdit)
	mux.HandleFunc("POST /agent/articles/{id}", app.handleAgentArticleUpdate)
	mux.HandleFunc("POST /agent/articles/{id}/publish", app.handleAgentArticlePublish)
	mux.HandleFunc("POST /agent/articles/{id}/unpublish", app.handleAgentArticleUnpublish)
	mux.HandleFunc("POST /agent/articles/{id}/delete", app.handleAgentArticleDelete)

	log.Printf("Customer Service App listening on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, app.logMiddleware(mux)))
}

// --- Template loading ---

func (app *App) loadTemplates() {
	funcMap := template.FuncMap{
		"timeAgo": func(t time.Time) string {
			d := time.Since(t)
			switch {
			case d < time.Minute:
				return "just now"
			case d < time.Hour:
				m := int(d.Minutes())
				if m == 1 {
					return "1 minute ago"
				}
				return fmt.Sprintf("%d minutes ago", m)
			case d < 24*time.Hour:
				h := int(d.Hours())
				if h == 1 {
					return "1 hour ago"
				}
				return fmt.Sprintf("%d hours ago", h)
			default:
				return t.Format("Jan 2, 2006 3:04 PM")
			}
		},
		"formatTime": func(t time.Time) string {
			return t.Format("Jan 2, 2006 3:04 PM")
		},
		"nl2br": func(s string) template.HTML {
			return template.HTML(strings.ReplaceAll(template.HTMLEscapeString(s), "\n", "<br>"))
		},
		"statusClass": func(s TicketStatus) string {
			switch s {
			case StatusNew:
				return "badge-blue"
			case StatusOpen:
				return "badge-green"
			case StatusPendingCustomer:
				return "badge-yellow"
			case StatusResolved:
				return "badge-purple"
			case StatusClosed:
				return "badge-gray"
			}
			return ""
		},
		"priorityClass": func(p Priority) string {
			switch p {
			case PriorityLow:
				return "badge-gray"
			case PriorityNormal:
				return "badge-blue"
			case PriorityHigh:
				return "badge-orange"
			case PriorityUrgent:
				return "badge-red"
			}
			return ""
		},
		"chatStatusClass": func(s ChatStatus) string {
			switch s {
			case ChatWaiting:
				return "badge-yellow"
			case ChatActive:
				return "badge-green"
			case ChatEnded:
				return "badge-gray"
			case ChatEscalated:
				return "badge-purple"
			}
			return ""
		},
		"agentName": func(agents []*Agent, id string) string {
			for _, a := range agents {
				if a.ID == id {
					return a.Name
				}
			}
			return ""
		},
		"dict": func(pairs ...any) map[string]any {
			m := make(map[string]any)
			for i := 0; i+1 < len(pairs); i += 2 {
				if k, ok := pairs[i].(string); ok {
					m[k] = pairs[i+1]
				}
			}
			return m
		},
		"markdown": func(s string) template.HTML {
			// Simple markdown-like rendering for articles
			lines := strings.Split(s, "\n")
			var out strings.Builder
			inList := false
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "## ") {
					if inList {
						out.WriteString("</ul>")
						inList = false
					}
					out.WriteString("<h3>" + template.HTMLEscapeString(trimmed[3:]) + "</h3>")
				} else if strings.HasPrefix(trimmed, "# ") {
					if inList {
						out.WriteString("</ul>")
						inList = false
					}
					out.WriteString("<h2>" + template.HTMLEscapeString(trimmed[2:]) + "</h2>")
				} else if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
					if !inList {
						out.WriteString("<ul>")
						inList = true
					}
					out.WriteString("<li>" + template.HTMLEscapeString(trimmed[2:]) + "</li>")
				} else if len(trimmed) > 0 && trimmed[0] >= '1' && trimmed[0] <= '9' && strings.Contains(trimmed, ". ") {
					idx := strings.Index(trimmed, ". ")
					if !inList {
						out.WriteString("<ol>")
						inList = true
					}
					out.WriteString("<li>" + template.HTMLEscapeString(trimmed[idx+2:]) + "</li>")
				} else if trimmed == "" {
					if inList {
						out.WriteString("</ul>")
						inList = false
					}
					out.WriteString("<br>")
				} else {
					if inList {
						out.WriteString("</ul>")
						inList = false
					}
					// Handle inline bold
					escaped := template.HTMLEscapeString(trimmed)
					for strings.Contains(escaped, "**") {
						start := strings.Index(escaped, "**")
						rest := escaped[start+2:]
						end := strings.Index(rest, "**")
						if end == -1 {
							break
						}
						escaped = escaped[:start] + "<strong>" + rest[:end] + "</strong>" + rest[end+2:]
					}
					out.WriteString("<p>" + escaped + "</p>")
				}
			}
			if inList {
				out.WriteString("</ul>")
			}
			return template.HTML(out.String())
		},
	}

	pages := []string{
		"home", "ticket_new", "ticket_created", "ticket_view", "ticket_lookup",
		"articles", "article_detail",
		"chat_start", "chat_widget",
		"login", "inbox", "ticket_detail",
		"chat_console", "chat_session",
		"kb_list", "kb_edit",
	}

	app.tmpl = make(map[string]*template.Template)
	for _, p := range pages {
		t := template.Must(
			template.New("").Funcs(funcMap).ParseFS(templateFS,
				"templates/base.html",
				"templates/"+p+".html",
			),
		)
		app.tmpl[p] = t
	}
}

func (app *App) render(w http.ResponseWriter, page string, data map[string]any) {
	t := app.tmpl[page]
	if t == nil {
		http.Error(w, "template not found", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("template error: %v", err)
	}
}

// --- Auth helpers ---

func (app *App) generateAccessToken(ticketID string) string {
	mac := hmac.New(sha256.New, app.hmacKey)
	mac.Write([]byte(ticketID))
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

func (app *App) verifyAccessToken(ticketID, token string) bool {
	expected := app.generateAccessToken(ticketID)
	return hmac.Equal([]byte(expected), []byte(token))
}

func (app *App) createSession(agentID string) string {
	b := make([]byte, 32)
	rand.Read(b)
	sid := base64.URLEncoding.EncodeToString(b)
	app.sessionMu.Lock()
	app.sessions[sid] = agentID
	app.sessionMu.Unlock()
	return sid
}

func (app *App) getSessionAgent(r *http.Request) *Agent {
	c, err := r.Cookie("session")
	if err != nil {
		return nil
	}
	app.sessionMu.RLock()
	agentID := app.sessions[c.Value]
	app.sessionMu.RUnlock()
	if agentID == "" {
		return nil
	}
	return app.store.GetAgent(agentID)
}

func (app *App) deleteSession(sid string) {
	app.sessionMu.Lock()
	delete(app.sessions, sid)
	app.sessionMu.Unlock()
}

func (app *App) requireAgent(w http.ResponseWriter, r *http.Request) *Agent {
	agent := app.getSessionAgent(r)
	if agent == nil {
		http.Redirect(w, r, "/agent/login", http.StatusSeeOther)
		return nil
	}
	return agent
}

// --- Middleware ---

func (app *App) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start).Round(time.Microsecond))
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// --- Chat timeout ---

func (app *App) startChatTimeout(sessionID string) {
	go func() {
		time.Sleep(app.chatTimeout)
		cs := app.store.GetChatSession(sessionID)
		if cs == nil || cs.Status != ChatWaiting {
			return
		}
		// Fallback: convert chat to ticket
		log.Printf("Chat %s timed out, converting to ticket", sessionID)

		// Build description from chat messages
		var desc strings.Builder
		desc.WriteString("(Converted from chat — no agent was available)\n\n")
		for _, m := range cs.Messages {
			desc.WriteString(fmt.Sprintf("[%s] %s: %s\n", m.CreatedAt.Format("15:04"), m.AuthorName, m.Body))
		}

		token := app.generateAccessToken(cs.ID + "-ticket")
		subject := "Chat support request from " + cs.CustomerName
		t := app.store.CreateTicket(subject, desc.String(), cs.CustomerName, cs.CustomerEmail, "")
		t.AccessToken = app.generateAccessToken(t.ID)

		app.store.SetChatStatus(sessionID, ChatEscalated)
		app.store.SetChatTicketID(sessionID, t.ID)

		// Notify via SSE
		_ = token
		sysMsg := fmt.Sprintf("No agent was available. Your conversation has been saved as ticket %s. You can continue at /help/tickets/%s?token=%s",
			t.RefCode, t.RefCode, t.AccessToken)
		app.store.AddChatMessage(sessionID, sysMsg, "System", "system")
	}()
}
