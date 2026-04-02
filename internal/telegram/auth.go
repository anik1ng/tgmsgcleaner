package telegram

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

func SendCode(ctx context.Context, client *telegram.Client, phone string, sessionPath string) (string, error) {
	res, err := client.Auth().SendCode(ctx, phone, auth.SendCodeOptions{})
	if err != nil {
		var rpcErr *tgerr.Error
		if errors.As(err, &rpcErr) && rpcErr.Type == "AUTH_RESTART" {
			os.Remove(sessionPath)
			res, err = client.Auth().SendCode(ctx, phone, auth.SendCodeOptions{})
		}
		if err != nil {
			return "", fmt.Errorf("send code: %w", err)
		}
	}
	sentCode, ok := res.(*tg.AuthSentCode)
	if !ok {
		return "", fmt.Errorf("unexpected response type")
	}

	return sentCode.PhoneCodeHash, nil
}

func SignIn(ctx context.Context, client *telegram.Client, phone string, code string, codeHash string) error {
	_, err := client.Auth().SignIn(ctx, phone, code, codeHash)
	if err != nil {
		return fmt.Errorf("sign in: %w", err)
	}

	return nil
}

func CheckPassword(ctx context.Context, client *telegram.Client, password string) error {
	_, err := client.Auth().Password(ctx, password)
	if err != nil {
		return fmt.Errorf("check password: %w", err)
	}

	return nil
}
