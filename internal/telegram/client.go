// Package telegram wraps gotd/td client for auth, messaging and chat operations.
package telegram

import (
	"context"
	"fmt"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
)

func NewClient(apiID int, apiHash string, sessionPath string) *telegram.Client {
	opts := telegram.Options{
		SessionStorage: &session.FileStorage{Path: sessionPath},
	}
	return telegram.NewClient(apiID, apiHash, opts)
}

func IsAuthorized(ctx context.Context, client *telegram.Client) (bool, error) {
	status, err := client.Auth().Status(ctx)
	if err != nil {
		return false, fmt.Errorf("auth status: %w", err)
	}
	return status.Authorized, nil
}
