package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/josephburgess/breeze/internal/models"
)

const stateTTL = 10 * time.Minute

var ErrInvalidState = errors.New("unknown or expired oauth state")

type Users interface {
	SaveUser(user *models.User) error
	IssueAPIKey(ctx context.Context, githubUserID int64) (string, error)
}

type pending struct {
	redirectURI string
	expires     time.Time
}

// Flow runs the GitHub sign in for gust. Each login keeps its own state and
// redirect URI, so parallel logins do not mix.
type Flow struct {
	github *GitHub
	users  Users
	Now    func() time.Time

	mu     sync.Mutex
	states map[string]pending
}

func NewFlow(github *GitHub, users Users) *Flow {
	return &Flow{github: github, users: users, Now: time.Now, states: map[string]pending{}}
}

func callbackURL(port int) string {
	return fmt.Sprintf("http://localhost:%d/callback", port)
}

// Begin returns the GitHub URL to open and the state gust must send back.
func (f *Flow) Begin(port int) (string, string) {
	state := uuid.NewString()
	redirectURI := callbackURL(port)

	f.mu.Lock()
	now := f.Now()
	for s, p := range f.states {
		if now.After(p.expires) {
			delete(f.states, s)
		}
	}
	f.states[state] = pending{redirectURI: redirectURI, expires: now.Add(stateTTL)}
	f.mu.Unlock()

	return f.github.AuthURL(redirectURI, state), state
}

// RedirectFor returns the gust callback for a live state.
func (f *Flow) RedirectFor(state string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.states[state]
	if !ok || f.Now().After(p.expires) {
		return "", false
	}
	return p.redirectURI, true
}

// Complete swaps the GitHub code for a gust API key. gust versions before
// state checks send no state; for them the callback port gives the redirect URI.
func (f *Flow) Complete(ctx context.Context, code, state string, port int) (string, string, error) {
	redirectURI := callbackURL(port)
	if state != "" {
		f.mu.Lock()
		p, ok := f.states[state]
		delete(f.states, state)
		f.mu.Unlock()
		if !ok || f.Now().After(p.expires) {
			return "", "", ErrInvalidState
		}
		redirectURI = p.redirectURI
	}

	token, err := f.github.Exchange(ctx, code, redirectURI)
	if err != nil {
		return "", "", err
	}
	user, err := f.github.User(ctx, token)
	if err != nil {
		return "", "", err
	}
	if err := f.users.SaveUser(user); err != nil {
		return "", "", fmt.Errorf("save user: %w", err)
	}
	key, err := f.users.IssueAPIKey(ctx, user.GithubID)
	if err != nil {
		return "", "", err
	}
	return user.Login, key, nil
}
