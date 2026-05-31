package odyssey

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const groqURL = "https://api.groq.com/openai/v1/chat/completions"

type GroqClient struct {
	apiKey string
	http   *http.Client
}

func NewGroqClient(apiKey string) *GroqClient {
	return &GroqClient{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 15 * time.Second},
	}
}

type groqRequest struct {
	Model       string        `json:"model"`
	Messages    []groqMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqResponse struct {
	Choices []struct {
		Message groqMessage `json:"message"`
	} `json:"choices"`
}

// GenerateQuestion asks Groq for a question about the given destination topic.
func (g *GroqClient) GenerateQuestion(ctx context.Context, dest Destination) (*Question, error) {
	prompt := fmt.Sprintf(`You are a space exploration quiz master. Generate a single multiple-choice question about: %s.

Respond ONLY with valid JSON in this exact format (no markdown, no explanation):
{
  "question": "...",
  "choices": ["A. ...", "B. ...", "C. ...", "D. ..."],
  "answer": 0,
  "hint": "..."
}

Rules:
- question: one clear factual question about the topic
- choices: exactly 4 options labeled A-D, only ONE is correct
- answer: zero-based index of the correct choice (0=A, 1=B, 2=C, 3=D)
- hint: one sentence that guides toward the answer without giving it away
- difficulty: intermediate — not too easy, not too obscure`, dest.Topic)

	body := groqRequest{
		Model:       "llama-3.3-70b-versatile",
		Messages:    []groqMessage{{Role: "user", Content: prompt}},
		Temperature: 0.7,
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqURL, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("groq request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("groq read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("groq status %d: %s", resp.StatusCode, string(raw))
	}

	var gr groqResponse
	if err := json.Unmarshal(raw, &gr); err != nil {
		return nil, fmt.Errorf("groq unmarshal response: %w", err)
	}
	if len(gr.Choices) == 0 {
		return nil, fmt.Errorf("groq returned no choices")
	}

	content := strings.TrimSpace(gr.Choices[0].Message.Content)
	// Some models wrap JSON in ```json ... ``` even when instructed not to.
	// Strip the fences before unmarshalling to avoid a parse error.
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var raw2 struct {
		Question string   `json:"question"`
		Choices  []string `json:"choices"`
		Answer   int      `json:"answer"`
		Hint     string   `json:"hint"`
	}
	if err := json.Unmarshal([]byte(content), &raw2); err != nil {
		return nil, fmt.Errorf("groq parse question JSON: %w, raw: %s", err, content)
	}
	if len(raw2.Choices) != 4 {
		return nil, fmt.Errorf("groq returned %d choices, want 4", len(raw2.Choices))
	}
	if raw2.Answer < 0 || raw2.Answer > 3 {
		return nil, fmt.Errorf("groq answer index %d out of range", raw2.Answer)
	}

	choices, _ := json.Marshal(raw2.Choices)
	return &Question{
		DestinationID: dest.ID,
		Question:      raw2.Question,
		Choices:       choices,
		Answer:        raw2.Answer,
		Hint:          raw2.Hint,
		Source:        "groq",
		Topic:         dest.Topic,
	}, nil
}
