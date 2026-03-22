package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/heechul/prism-mcp/internal/engine"
	"github.com/mark3labs/mcp-go/mcp"
)

var safeID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

const maxRounds = 20

type QA struct {
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	Timestamp string `json:"timestamp"`
}

type InterviewResponse struct {
	ContextID     string `json:"context_id"`
	PerspectiveID string `json:"perspective_id"`
	Round         int    `json:"round"`
	Continue      *bool  `json:"continue,omitempty"`
	Question      string `json:"question,omitempty"`
	Score         string `json:"score,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

func jsonResponse(resp InterviewResponse) string {
	data, _ := json.Marshal(resp)
	return string(data)
}

type InterviewSession struct {
	ContextID    string `json:"context_id"`
	PerspectiveID string `json:"perspective_id"`
	Topic         string `json:"topic"`
	Rounds        []QA   `json:"rounds"`
	Pending       string `json:"pending"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// sessionLocks provides per-session mutex granularity to enable parallel
// interview execution across different perspective sessions. Each session
// (identified by contextID + perspectiveID) gets its own lock, so concurrent
// interviews for different specialists within the same or different analysis
// tasks do not block each other.
var sessionLocks = &sessionLockMap{locks: make(map[string]*sync.Mutex)}

type sessionLockMap struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// get returns the mutex for a specific session, creating one if needed.
// The meta-lock (mu) is only held briefly to look up or create the per-session lock.
func (m *sessionLockMap) get(contextID, perspectiveID string) *sync.Mutex {
	key := contextID + "/" + perspectiveID
	m.mu.Lock()
	lk, ok := m.locks[key]
	if !ok {
		lk = &sync.Mutex{}
		m.locks[key] = lk
	}
	m.mu.Unlock()
	return lk
}

// perspectiveDir returns ~/.prism/state/{context_id}/perspectives/{perspective_id}/ and creates it.
func perspectiveDir(contextID, perspectiveID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve home directory: %w", err)
	}
	dir := filepath.Join(home, ".prism", "state", contextID, "perspectives", perspectiveID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("cannot create directory %s: %w", dir, err)
	}
	return dir, nil
}

func interviewPath(contextID, perspectiveID string) (string, error) {
	dir, err := perspectiveDir(contextID, perspectiveID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "interview.json"), nil
}

func findingsPath(contextID, perspectiveID string) (string, error) {
	dir, err := perspectiveDir(contextID, perspectiveID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "findings.json"), nil
}

func saveSession(session *InterviewSession) error {
	session.UpdatedAt = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	path, err := interviewPath(session.ContextID, session.PerspectiveID)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func loadSession(contextID, perspectiveID string) (*InterviewSession, error) {
	path, err := interviewPath(contextID, perspectiveID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("session not found for context=%q perspective=%q", contextID, perspectiveID)
	}
	var session InterviewSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// loadFindings reads the findings from the perspective directory.
func loadFindings(contextID, perspectiveID string) string {
	path, err := findingsPath(contextID, perspectiveID)
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func handleInterview(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	contextID, _ := args["context_id"].(string)
	perspectiveID, _ := args["perspective_id"].(string)
	topic, _ := args["topic"].(string)
	response, _ := args["response"].(string)

	if contextID == "" || perspectiveID == "" {
		return mcp.NewToolResultError("context_id and perspective_id are required"), nil
	}
	if !safeID.MatchString(contextID) || !safeID.MatchString(perspectiveID) {
		return mcp.NewToolResultError("context_id and perspective_id must contain only alphanumeric characters, hyphens, and underscores"), nil
	}

	// Per-session lock: only serializes concurrent calls to the SAME session,
	// allowing parallel interviews across different perspectives/tasks.
	sessionMu := sessionLocks.get(contextID, perspectiveID)
	sessionMu.Lock()
	defer sessionMu.Unlock()

	// New session — start interview
	if topic != "" {
		if _, err := perspectiveDir(contextID, perspectiveID); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create session directory: %v", err)), nil
		}

		session := &InterviewSession{
			ContextID:    contextID,
			PerspectiveID: perspectiveID,
			Topic:         topic,
			CreatedAt:     time.Now().Format(time.RFC3339),
		}

		findings := loadFindings(contextID, perspectiveID)
		if findings == "" {
			return mcp.NewToolResultError(fmt.Sprintf("findings.json not found — write to ~/.prism/state/%s/perspectives/%s/findings.json first", contextID, perspectiveID)), nil
		}

		question, err := generateFirstQuestion(ctx, session)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to generate question: %v", err)), nil
		}

		session.Pending = question
		if err := saveSession(session); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to save session: %v", err)), nil
		}

		return mcp.NewToolResultText(jsonResponse(InterviewResponse{ContextID: contextID, PerspectiveID: perspectiveID, Round: 1, Question: question})), nil
	}

	// Continue existing session
	session, err := loadSession(contextID, perspectiveID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if response == "" {
		return mcp.NewToolResultError("response is required for follow-up rounds"), nil
	}

	// Record the answer
	session.Rounds = append(session.Rounds, QA{
		Question:  session.Pending,
		Answer:    response,
		Timestamp: time.Now().Format(time.RFC3339),
	})
	session.Pending = ""

	round := len(session.Rounds)

	// Max rounds check
	if round >= maxRounds {
		if err := saveSession(session); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to save session: %v", err)), nil
		}
		scoreResult, _ := scoreSession(ctx, session)
		cont := false
		return mcp.NewToolResultText(jsonResponse(InterviewResponse{ContextID: contextID, PerspectiveID: perspectiveID, Round: round, Continue: &cont, Reason: "max_rounds", Score: scoreResult})), nil
	}

	// Score FIRST (all Q&A complete, no pending question)
	scoreResult, err := scoreSession(ctx, session)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("scoring failed: %v", err)), nil
	}

	pass := strings.Contains(scoreResult, "pass: true")

	if pass {
		// Score > 0.8 — stop, no next question generated
		if err := saveSession(session); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to save session: %v", err)), nil
		}
		cont := false
		return mcp.NewToolResultText(jsonResponse(InterviewResponse{ContextID: contextID, PerspectiveID: perspectiveID, Round: round, Continue: &cont, Reason: "pass", Score: scoreResult})), nil
	}

	// Score <= 0.8 — generate next question
	question, err := generateFollowUpQuestion(ctx, session)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to generate question: %v", err)), nil
	}

	// LLM may signal no more questions needed
	if strings.TrimSpace(question) == "INTERVIEW_COMPLETE" {
		if err := saveSession(session); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to save session: %v", err)), nil
		}
		cont := false
		return mcp.NewToolResultText(jsonResponse(InterviewResponse{ContextID: contextID, PerspectiveID: perspectiveID, Round: round, Continue: &cont, Reason: "interview_complete", Score: scoreResult})), nil
	}

	session.Pending = question
	if err := saveSession(session); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to save session: %v", err)), nil
	}

	cont := true
	return mcp.NewToolResultText(jsonResponse(InterviewResponse{ContextID: contextID, PerspectiveID: perspectiveID, Round: round, Continue: &cont, Score: scoreResult, Question: question})), nil
}

func findingsBlock(session *InterviewSession) string {
	content := loadFindings(session.ContextID, session.PerspectiveID)
	if content == "" {
		return ""
	}
	return fmt.Sprintf(`

Reference material (analyst findings):
---
%s
---

Identify the single weakest point in the findings and ask about it.`, content)
}

func generateFirstQuestion(ctx context.Context, session *InterviewSession) (string, error) {
	prompt := fmt.Sprintf(`You are a Socratic interviewer. Your role is to ask ONE probing question to clarify vague or ambiguous aspects.

Topic: %s%s

Focus on: relevance to original problem, assumption verification, analysis constraints

Rules:
- Ask exactly ONE question
- Focus on the single weakest point — whether a relevance gap, unverified assumption, or blind spot
- Do not suggest answers or solutions
- Do not implement anything
- Keep the question concise and specific
- Ask in the same language as the topic

Respond with ONLY the question, nothing else.`, session.Topic, findingsBlock(session))

	return engine.QueryLLM(ctx, prompt)
}

func generateFollowUpQuestion(ctx context.Context, session *InterviewSession) (string, error) {
	var history strings.Builder
	for i, qa := range session.Rounds {
		history.WriteString(fmt.Sprintf("Q%d: %s\nA%d: %s\n\n", i+1, qa.Question, i+1, qa.Answer))
	}

	prompt := fmt.Sprintf(`You are a Socratic interviewer clarifying requirements through probing questions.

Topic: %s%s

Previous interview:
%s

Rules:
- Ask exactly ONE question about what remains unclear or ambiguous
- Build on previous answers — do not repeat already-clarified areas
- Focus on the single weakest point — whether a relevance gap, unverified assumption, or blind spot
- Do not suggest answers or solutions
- Do not implement anything
- Keep the question concise and specific
- Ask in the same language as the topic
- If requirements are already very clear, say "INTERVIEW_COMPLETE" instead of asking another question

Respond with ONLY the question (or INTERVIEW_COMPLETE), nothing else.`, session.Topic, findingsBlock(session), history.String())

	return engine.QueryLLM(ctx, prompt)
}
