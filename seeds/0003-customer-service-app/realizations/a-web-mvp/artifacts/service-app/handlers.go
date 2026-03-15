package main

import (
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strings"
)

// ===================== Public Handlers =====================

func (app *App) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query().Get("q")
	var articles []*Article
	if q != "" {
		articles = app.store.SearchArticles(q)
	} else {
		articles = app.store.ListArticles(true, "")
	}
	categories := app.store.ArticleCategories()
	app.render(w, "home", map[string]any{
		"Title":      "Help Center",
		"Articles":   articles,
		"Categories": categories,
		"Query":      q,
	})
}

func (app *App) handleTicketNew(w http.ResponseWriter, r *http.Request) {
	app.render(w, "ticket_new", map[string]any{
		"Title": "Submit a Request",
	})
}

func (app *App) handleTicketCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	subject := strings.TrimSpace(r.FormValue("subject"))
	description := strings.TrimSpace(r.FormValue("description"))

	if name == "" || email == "" || subject == "" || description == "" {
		app.render(w, "ticket_new", map[string]any{
			"Title": "Submit a Request",
			"Error": "All fields are required.",
			"Name": name, "Email": email, "Subject": subject, "Description": description,
		})
		return
	}

	t := app.store.CreateTicket(subject, description, name, email, "")
	t.AccessToken = app.generateAccessToken(t.ID)

	app.render(w, "ticket_created", map[string]any{
		"Title":       "Request Submitted",
		"Ticket":      t,
		"AccessURL":   fmt.Sprintf("/help/tickets/%s?token=%s", t.RefCode, t.AccessToken),
	})
}

func (app *App) handleTicketView(w http.ResponseWriter, r *http.Request) {
	refCode := r.PathValue("refCode")
	t := app.store.GetTicketByRef(refCode)
	if t == nil {
		http.NotFound(w, r)
		return
	}

	// Check access: token in query or agent session
	token := r.URL.Query().Get("token")
	agent := app.getSessionAgent(r)
	if agent == nil && !app.verifyAccessToken(t.ID, token) {
		http.Error(w, "Access denied. Please use your secure ticket link or look up your ticket.", http.StatusForbidden)
		return
	}

	app.render(w, "ticket_view", map[string]any{
		"Title":  "Ticket " + t.RefCode,
		"Ticket": t,
		"Token":  token,
		"IsAgent": agent != nil,
	})
}

func (app *App) handleTicketCustomerReply(w http.ResponseWriter, r *http.Request) {
	refCode := r.PathValue("refCode")
	t := app.store.GetTicketByRef(refCode)
	if t == nil {
		http.NotFound(w, r)
		return
	}

	token := r.FormValue("token")
	if !app.verifyAccessToken(t.ID, token) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	r.ParseForm()
	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		http.Redirect(w, r, fmt.Sprintf("/help/tickets/%s?token=%s", refCode, token), http.StatusSeeOther)
		return
	}

	app.store.AddTicketMessage(t.ID, body, t.RequesterName, "customer")
	http.Redirect(w, r, fmt.Sprintf("/help/tickets/%s?token=%s", refCode, token), http.StatusSeeOther)
}

func (app *App) handleTicketLookup(w http.ResponseWriter, r *http.Request) {
	app.render(w, "ticket_lookup", map[string]any{
		"Title": "Find Your Ticket",
	})
}

func (app *App) handleTicketLookupSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	email := strings.TrimSpace(r.FormValue("email"))
	refCode := strings.TrimSpace(r.FormValue("ref_code"))

	if email == "" || refCode == "" {
		app.render(w, "ticket_lookup", map[string]any{
			"Title": "Find Your Ticket",
			"Error": "Both email and reference code are required.",
			"Email": email, "RefCode": refCode,
		})
		return
	}

	t := app.store.GetTicketByRef(refCode)
	if t == nil || !strings.EqualFold(t.RequesterEmail, email) {
		app.render(w, "ticket_lookup", map[string]any{
			"Title":   "Find Your Ticket",
			"Error":   "No ticket found with that email and reference code.",
			"Email":   email,
			"RefCode": refCode,
		})
		return
	}

	token := app.generateAccessToken(t.ID)
	http.Redirect(w, r, fmt.Sprintf("/help/tickets/%s?token=%s", t.RefCode, token), http.StatusSeeOther)
}

func (app *App) handleArticles(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	q := r.URL.Query().Get("q")
	var articles []*Article
	if q != "" {
		articles = app.store.SearchArticles(q)
	} else {
		articles = app.store.ListArticles(true, category)
	}
	categories := app.store.ArticleCategories()
	app.render(w, "articles", map[string]any{
		"Title":      "Knowledge Base",
		"Articles":   articles,
		"Categories": categories,
		"Category":   category,
		"Query":      q,
	})
}

func (app *App) handleArticleDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	a := app.store.GetArticleBySlug(slug)
	if a == nil || a.Status != ArticlePublished {
		http.NotFound(w, r)
		return
	}
	// Get related articles from same category
	related := app.store.ListArticles(true, a.Category)
	var filtered []*Article
	for _, r := range related {
		if r.ID != a.ID {
			filtered = append(filtered, r)
		}
	}
	app.render(w, "article_detail", map[string]any{
		"Title":   a.Title,
		"Article": a,
		"Related": filtered,
	})
}

// ===================== Chat Handlers =====================

func (app *App) handleChatStart(w http.ResponseWriter, r *http.Request) {
	app.render(w, "chat_start", map[string]any{
		"Title": "Live Chat",
	})
}

func (app *App) handleChatCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	message := strings.TrimSpace(r.FormValue("message"))

	if name == "" || email == "" || message == "" {
		app.render(w, "chat_start", map[string]any{
			"Title": "Live Chat",
			"Error": "Please fill in all fields.",
			"Name": name, "Email": email, "Message": message,
		})
		return
	}

	cs := app.store.CreateChatSession(name, email)
	app.store.AddChatMessage(cs.ID, message, name, "customer")
	app.startChatTimeout(cs.ID)

	http.Redirect(w, r, "/chat/"+cs.ID, http.StatusSeeOther)
}

func (app *App) handleChatWidget(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	cs := app.store.GetChatSession(sessionID)
	if cs == nil {
		http.NotFound(w, r)
		return
	}
	app.render(w, "chat_widget", map[string]any{
		"Title":   "Chat",
		"Session": cs,
	})
}

func (app *App) handleChatCustomerMessage(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	cs := app.store.GetChatSession(sessionID)
	if cs == nil {
		http.NotFound(w, r)
		return
	}
	if cs.Status == ChatEnded || cs.Status == ChatEscalated {
		http.Error(w, "This chat session has ended.", http.StatusGone)
		return
	}
	r.ParseForm()
	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		w.WriteHeader(204)
		return
	}
	msg := app.store.AddChatMessage(sessionID, body, cs.CustomerName, "customer")
	if msg == nil {
		http.Error(w, "Failed to send message", 500)
		return
	}
	// Return the message HTML fragment for HTMX
	fmt.Fprintf(w, `<div class="chat-msg chat-msg-customer"><strong>%s</strong> <span class="chat-time">%s</span><p>%s</p></div>`,
		template.HTMLEscapeString(msg.AuthorName),
		msg.CreatedAt.Format("15:04"),
		template.HTMLEscapeString(msg.Body),
	)
}

func (app *App) handleChatStream(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	app.streamChat(w, r, sessionID)
}

func (app *App) streamChat(w http.ResponseWriter, r *http.Request, sessionID string) {
	cs := app.store.GetChatSession(sessionID)
	if cs == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", 500)
		return
	}

	ch := app.store.SubscribeChat(sessionID)
	defer app.store.UnsubscribeChat(sessionID, ch)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			cssClass := "chat-msg-agent"
			if msg.AuthorType == "customer" {
				cssClass = "chat-msg-customer"
			} else if msg.AuthorType == "system" {
				cssClass = "chat-msg-system"
			}
			data := fmt.Sprintf(`<div class="chat-msg %s"><strong>%s</strong> <span class="chat-time">%s</span><p>%s</p></div>`,
				cssClass,
				template.HTMLEscapeString(msg.AuthorName),
				msg.CreatedAt.Format("15:04"),
				template.HTMLEscapeString(msg.Body),
			)
			for _, line := range strings.Split(data, "\n") {
				fmt.Fprintf(w, "data: %s\n", line)
			}
			fmt.Fprintf(w, "\n")
			flusher.Flush()
		}
	}
}

// ===================== Agent Auth Handlers =====================

func (app *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if app.getSessionAgent(r) != nil {
		http.Redirect(w, r, "/agent/inbox", http.StatusSeeOther)
		return
	}
	app.render(w, "login", map[string]any{
		"Title": "Agent Login",
	})
}

func (app *App) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	agent := app.store.GetAgentByEmail(email)
	if agent == nil || agent.Password != password {
		app.render(w, "login", map[string]any{
			"Title": "Agent Login",
			"Error": "Invalid email or password.",
			"Email": email,
		})
		return
	}

	sid := app.createSession(agent.ID)
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/agent/inbox", http.StatusSeeOther)
}

func (app *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("session"); err == nil {
		app.deleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   "session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ===================== Agent Ticket Handlers =====================

func (app *App) handleInbox(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	status := r.URL.Query().Get("status")
	assignee := r.URL.Query().Get("assignee")
	tickets := app.store.ListTickets(status, assignee)
	agents := app.store.ListAgents()
	app.render(w, "inbox", map[string]any{
		"Title":         "Inbox",
		"Agent":         agent,
		"Tickets":       tickets,
		"Agents":        agents,
		"FilterStatus":  status,
		"FilterAssignee": assignee,
	})
}

func (app *App) handleAgentTicketDetail(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	id := r.PathValue("id")
	t := app.store.GetTicket(id)
	if t == nil {
		http.NotFound(w, r)
		return
	}
	agents := app.store.ListAgents()
	app.render(w, "ticket_detail", map[string]any{
		"Title":  "Ticket " + t.RefCode,
		"Agent":  agent,
		"Ticket": t,
		"Agents": agents,
	})
}

func (app *App) handleAgentTicketReply(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	id := r.PathValue("id")
	r.ParseForm()
	body := strings.TrimSpace(r.FormValue("body"))
	if body != "" {
		app.store.AddTicketMessage(id, body, agent.Name, "agent")
	}
	http.Redirect(w, r, "/agent/tickets/"+id, http.StatusSeeOther)
}

func (app *App) handleAgentTicketNote(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	id := r.PathValue("id")
	r.ParseForm()
	body := strings.TrimSpace(r.FormValue("body"))
	if body != "" {
		app.store.AddTicketNote(id, body, agent.Name)
	}
	http.Redirect(w, r, "/agent/tickets/"+id, http.StatusSeeOther)
}

func (app *App) handleAgentTicketAssign(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	id := r.PathValue("id")
	r.ParseForm()
	assigneeID := r.FormValue("assignee_id")
	app.store.SetTicketAssignee(id, assigneeID)
	http.Redirect(w, r, "/agent/tickets/"+id, http.StatusSeeOther)
}

func (app *App) handleAgentTicketStatus(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	id := r.PathValue("id")
	r.ParseForm()
	status := TicketStatus(r.FormValue("status"))
	switch status {
	case StatusOpen, StatusPendingCustomer, StatusResolved, StatusClosed:
		app.store.SetTicketStatus(id, status)
	}
	http.Redirect(w, r, "/agent/tickets/"+id, http.StatusSeeOther)
}

func (app *App) handleAgentTicketPriority(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	id := r.PathValue("id")
	r.ParseForm()
	priority := Priority(r.FormValue("priority"))
	switch priority {
	case PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent:
		app.store.SetTicketPriority(id, priority)
	}
	http.Redirect(w, r, "/agent/tickets/"+id, http.StatusSeeOther)
}

// ===================== Agent Chat Handlers =====================

func (app *App) handleAgentChatConsole(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	filter := r.URL.Query().Get("status")
	var sessions []*ChatSession
	if filter != "" {
		sessions = app.store.ListChatSessions(filter)
	} else {
		// Show waiting and active by default
		waiting := app.store.ListChatSessions("waiting")
		active := app.store.ListChatSessions("active")
		sessions = append(waiting, active...)
	}
	app.render(w, "chat_console", map[string]any{
		"Title":    "Chat Console",
		"Agent":    agent,
		"Sessions": sessions,
		"Filter":   filter,
	})
}

func (app *App) handleAgentChatSession(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	id := r.PathValue("id")
	cs := app.store.GetChatSession(id)
	if cs == nil {
		http.NotFound(w, r)
		return
	}
	app.render(w, "chat_session", map[string]any{
		"Title":   "Chat with " + cs.CustomerName,
		"Agent":   agent,
		"Session": cs,
	})
}

func (app *App) handleAgentChatJoin(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	id := r.PathValue("id")
	cs := app.store.GetChatSession(id)
	if cs == nil {
		http.NotFound(w, r)
		return
	}
	if cs.Status == ChatWaiting {
		app.store.JoinChat(id, agent.ID, agent.Name)
		app.store.AddChatMessage(id, agent.Name+" has joined the chat.", "System", "system")
	}
	http.Redirect(w, r, "/agent/chats/"+id, http.StatusSeeOther)
}

func (app *App) handleAgentChatMessage(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	id := r.PathValue("id")
	r.ParseForm()
	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		w.WriteHeader(204)
		return
	}
	msg := app.store.AddChatMessage(id, body, agent.Name, "agent")
	if msg == nil {
		http.Error(w, "Failed to send message", 500)
		return
	}
	fmt.Fprintf(w, `<div class="chat-msg chat-msg-agent"><strong>%s</strong> <span class="chat-time">%s</span><p>%s</p></div>`,
		template.HTMLEscapeString(msg.AuthorName),
		msg.CreatedAt.Format("15:04"),
		template.HTMLEscapeString(msg.Body),
	)
}

func (app *App) handleAgentChatStream(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	id := r.PathValue("id")
	app.streamChat(w, r, id)
}

func (app *App) handleAgentChatEscalate(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	id := r.PathValue("id")
	cs := app.store.GetChatSession(id)
	if cs == nil {
		http.NotFound(w, r)
		return
	}
	if cs.Status != ChatActive && cs.Status != ChatWaiting {
		http.Redirect(w, r, "/agent/chats/"+id, http.StatusSeeOther)
		return
	}

	// Build ticket from chat
	var desc strings.Builder
	desc.WriteString("(Escalated from live chat by " + agent.Name + ")\n\n")
	for _, m := range cs.Messages {
		if m.AuthorType != "system" {
			desc.WriteString(fmt.Sprintf("[%s] %s: %s\n", m.CreatedAt.Format("15:04"), m.AuthorName, m.Body))
		}
	}

	t := app.store.CreateTicket(
		"Chat escalation from "+cs.CustomerName,
		desc.String(),
		cs.CustomerName,
		cs.CustomerEmail,
		"",
	)
	t.AccessToken = app.generateAccessToken(t.ID)
	app.store.SetTicketAssignee(t.ID, agent.ID)

	app.store.SetChatStatus(id, ChatEscalated)
	app.store.SetChatTicketID(id, t.ID)

	sysMsg := fmt.Sprintf("This chat has been converted to ticket %s. You can continue at /help/tickets/%s?token=%s",
		t.RefCode, t.RefCode, t.AccessToken)
	app.store.AddChatMessage(id, sysMsg, "System", "system")

	http.Redirect(w, r, "/agent/tickets/"+t.ID, http.StatusSeeOther)
}

func (app *App) handleAgentChatEnd(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	id := r.PathValue("id")
	cs := app.store.GetChatSession(id)
	if cs == nil {
		http.NotFound(w, r)
		return
	}
	app.store.SetChatStatus(id, ChatEnded)
	app.store.AddChatMessage(id, "This chat session has ended.", "System", "system")
	http.Redirect(w, r, "/agent/chats", http.StatusSeeOther)
}

// ===================== Agent KB Handlers =====================

func (app *App) handleAgentArticleList(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	articles := app.store.ListArticles(false, "")
	app.render(w, "kb_list", map[string]any{
		"Title":    "Knowledge Base",
		"Agent":    agent,
		"Articles": articles,
	})
}

func (app *App) handleAgentArticleNew(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	categories := app.store.ArticleCategories()
	app.render(w, "kb_edit", map[string]any{
		"Title":      "New Article",
		"Agent":      agent,
		"IsNew":      true,
		"Categories": categories,
	})
}

func (app *App) handleAgentArticleCreate(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	r.ParseForm()
	title := strings.TrimSpace(r.FormValue("title"))
	body := strings.TrimSpace(r.FormValue("body"))
	category := strings.TrimSpace(r.FormValue("category"))
	slug := slugify(title)

	if title == "" || body == "" {
		categories := app.store.ArticleCategories()
		app.render(w, "kb_edit", map[string]any{
			"Title": "New Article", "Agent": agent, "IsNew": true,
			"Error": "Title and body are required.",
			"ArticleTitle": title, "Body": body, "Category": category,
			"Categories": categories,
		})
		return
	}

	a := app.store.CreateArticle(title, slug, body, category, agent.Name)
	http.Redirect(w, r, "/agent/articles/"+a.ID+"/edit", http.StatusSeeOther)
}

func (app *App) handleAgentArticleEdit(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	id := r.PathValue("id")
	a := app.store.GetArticle(id)
	if a == nil {
		http.NotFound(w, r)
		return
	}
	categories := app.store.ArticleCategories()
	app.render(w, "kb_edit", map[string]any{
		"Title":      "Edit: " + a.Title,
		"Agent":      agent,
		"Article":    a,
		"IsNew":      false,
		"Categories": categories,
	})
}

func (app *App) handleAgentArticleUpdate(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	id := r.PathValue("id")
	r.ParseForm()
	title := strings.TrimSpace(r.FormValue("title"))
	body := strings.TrimSpace(r.FormValue("body"))
	category := strings.TrimSpace(r.FormValue("category"))
	slug := slugify(title)

	if title == "" || body == "" {
		a := app.store.GetArticle(id)
		categories := app.store.ArticleCategories()
		app.render(w, "kb_edit", map[string]any{
			"Title": "Edit: " + a.Title, "Agent": agent, "Article": a,
			"Error": "Title and body are required.", "Categories": categories,
		})
		return
	}

	app.store.UpdateArticle(id, title, slug, body, category)
	http.Redirect(w, r, "/agent/articles/"+id+"/edit?saved=1", http.StatusSeeOther)
}

func (app *App) handleAgentArticlePublish(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	id := r.PathValue("id")
	app.store.SetArticleStatus(id, ArticlePublished)
	http.Redirect(w, r, "/agent/articles/"+id+"/edit", http.StatusSeeOther)
}

func (app *App) handleAgentArticleUnpublish(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	id := r.PathValue("id")
	app.store.SetArticleStatus(id, ArticleDraft)
	http.Redirect(w, r, "/agent/articles/"+id+"/edit", http.StatusSeeOther)
}

func (app *App) handleAgentArticleDelete(w http.ResponseWriter, r *http.Request) {
	agent := app.requireAgent(w, r)
	if agent == nil {
		return
	}
	id := r.PathValue("id")
	app.store.DeleteArticle(id)
	http.Redirect(w, r, "/agent/articles", http.StatusSeeOther)
}

// --- Helpers ---

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
