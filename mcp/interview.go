package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

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

var sessionsMu sync.Mutex

// perspectiveDir returns ~/.prism/state/{context_id}/perspectives/{perspective_id}/ and creates it.
func perspectiveDir(contextID, perspectiveID string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	dir := filepath.Join(home, ".prism", "state", contextID, "perspectives", perspectiveID)
	os.MkdirAll(dir, 0755)
	return dir
}

func interviewPath(contextID, perspectiveID string) string {
	return filepath.Join(perspectiveDir(contextID, perspectiveID), "interview.json")
}

func findingsPath(contextID, perspectiveID string) string {
	return filepath.Join(perspectiveDir(contextID, perspectiveID), "findings.json")
}

func saveSession(session *InterviewSession) error {
	session.UpdatedAt = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(interviewPath(session.ContextID, session.PerspectiveID), data, 0644)
}

func loadSession(contextID, perspectiveID string) (*InterviewSession, error) {
	data, err := os.ReadFile(interviewPath(contextID, perspectiveID))
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
	data, err := os.ReadFile(findingsPath(contextID, perspectiveID))
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

	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	// New session — start interview
	if topic != "" {
		perspectiveDir(contextID, perspectiveID)

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

	return queryLLM(ctx, prompt)
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

	return queryLLM(ctx, prompt)
}
