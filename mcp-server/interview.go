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

const maxRounds = 30

type QA struct {
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	Timestamp string `json:"timestamp"`
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
	home, _ := os.UserHomeDir()
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
	args := request.GetArguments()
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

		return mcp.NewToolResultText(fmt.Sprintf(`{"context_id": %q, "perspective_id": %q, "round": 1, "question": %q}`, contextID, perspectiveID, question)), nil
	}

	// Continue existing session
	session, err := loadSession(contextID, perspectiveID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if response == "" {
		return mcp.NewToolResultError("response is required for follow-up rounds"), nil
	}

	if len(session.Rounds) >= maxRounds {
		return mcp.NewToolResultText(fmt.Sprintf(`{"context_id": %q, "perspective_id": %q, "round": %d, "question": "INTERVIEW_COMPLETE", "reason": "max rounds (%d) reached"}`, contextID, perspectiveID, len(session.Rounds), maxRounds)), nil
	}

	session.Rounds = append(session.Rounds, QA{
		Question:  session.Pending,
		Answer:    response,
		Timestamp: time.Now().Format(time.RFC3339),
	})

	question, err := generateFollowUpQuestion(ctx, session)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to generate question: %v", err)), nil
	}

	session.Pending = question
	if err := saveSession(session); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to save session: %v", err)), nil
	}

	round := len(session.Rounds) + 1

	return mcp.NewToolResultText(fmt.Sprintf(`{"context_id": %q, "perspective_id": %q, "round": %d, "question": %q}`, contextID, perspectiveID, round, question)), nil
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

Identify the most ambiguous or unclear aspects in the findings and ask about them.`, content)
}

func generateFirstQuestion(ctx context.Context, session *InterviewSession) (string, error) {
	prompt := fmt.Sprintf(`You are a Socratic interviewer. Your role is to ask ONE probing question to clarify vague or ambiguous aspects.

Topic: %s%s

Focus on: goal clarity, constraints, success criteria, edge cases

Rules:
- Ask exactly ONE question
- Focus on the most ambiguous or undefined aspect
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
- Focus on: goal clarity, constraints, success criteria, edge cases
- Do not suggest answers or solutions
- Do not implement anything
- Keep the question concise and specific
- Ask in the same language as the topic
- If requirements are already very clear, say "INTERVIEW_COMPLETE" instead of asking another question

Respond with ONLY the question (or INTERVIEW_COMPLETE), nothing else.`, session.Topic, findingsBlock(session), history.String())

	return queryLLM(ctx, prompt)
}
