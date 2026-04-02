package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"context"

	"github.com/anik1ng/tgmsgcleaner/internal/config"
	tg "github.com/anik1ng/tgmsgcleaner/internal/telegram"
	"github.com/anik1ng/tgmsgcleaner/internal/tui"
	tea "charm.land/bubbletea/v2"
)

func main() {
	resetFlag := flag.Bool("reset", false, "Reset all settings and accounts")
	flag.Parse()

	if *resetFlag {
		if err := config.Reset(); err != nil {
			fmt.Printf("failed to reset: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("All settings and accounts have been reset.")
		return
	}

	cfg, err := config.LoadGlobal()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Printf("failed to load config: %v\n", err)
			os.Exit(1)
		}
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("First run setup. Get api_id and api_hash from https://my.telegram.org/apps")
		fmt.Print("Enter api_id: ")
		apiIDStr, _ := reader.ReadString('\n')
		apiIDStr = strings.TrimSpace(apiIDStr)
		apiID, err := strconv.Atoi(apiIDStr)
		if err != nil {
			fmt.Printf("invalid api_id: %v\n", err)
			os.Exit(1)
		}
		fmt.Print("Enter api_hash: ")
		apiHash, _ := reader.ReadString('\n')
		apiHash = strings.TrimSpace(apiHash)
		if apiHash == "" {
			fmt.Println("api_hash cannot be empty")
			os.Exit(1)
		}
		cfg = config.GlobalConfig{APIID: apiID, APIHash: apiHash}
		if err := config.SaveGlobal(cfg); err != nil {
			fmt.Printf("failed to save config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Config saved.")
	}

	addAccount := false
	for {
		phone, err := pickAccount(addAccount)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			os.Exit(1)
		}

		sessionPath, err := config.SessionPath(phone)
		if err != nil {
			fmt.Printf("failed to get session path: %v\n", err)
			os.Exit(1)
		}

		accounts, _ := config.ListAccounts()

		exitAction := ""
		client := tg.NewClient(cfg.APIID, cfg.APIHash, sessionPath)
		client.Run(context.Background(), func(ctx context.Context) error {
			isAuth, err := tg.IsAuthorized(ctx, client)
			if err != nil {
				fmt.Printf("failed to check auth status: %v\n", err)
				return err
			}

			app := tui.NewApp(client, ctx, phone, sessionPath, isAuth, len(accounts))
			finalModel, _ := tea.NewProgram(app).Run()
			if final, ok := finalModel.(tui.App); ok {
				exitAction = final.ExitAction
			}
			return nil
		})

		switch exitAction {
		case "add":
			addAccount = true
			continue
		case "switch":
			addAccount = false
			continue
		default:
			return
		}
	}
}

func pickAccount(forceNew bool) (string, error) {
	accounts, err := config.ListAccounts()
	if err != nil {
		return "", fmt.Errorf("list accounts: %w", err)
	}

	if forceNew || len(accounts) == 0 {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter phone number (e.g. +79001234567): ")
		phone, _ := reader.ReadString('\n')
		phone = strings.TrimSpace(phone)
		if phone == "" {
			return "", fmt.Errorf("phone number cannot be empty")
		}
		return phone, nil
	}

	if len(accounts) == 1 {
		return accounts[0], nil
	}

	app := tui.NewPickerApp(accounts)
	finalModel, err := tea.NewProgram(app).Run()
	if err != nil {
		return "", fmt.Errorf("account picker: %w", err)
	}
	if final, ok := finalModel.(tui.App); ok && final.Selected != "" {
		return final.Selected, nil
	}
	return "", fmt.Errorf("no account selected")
}
