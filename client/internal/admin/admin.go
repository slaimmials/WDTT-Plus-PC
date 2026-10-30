// Package admin talks to the wdtt-server admin interface over SSH.
package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"wdtt-plus/client/internal/sshx"
)

const adminCommand = "/usr/local/bin/wdtt-server admin --config-dir /etc/wdtt --request-stdin"

type request struct {
	MainPassword string   `json:"main_password"`
	Args         []string `json:"args"`
}

type response struct {
	OK      bool   `json:"ok"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Call runs one admin command and returns the server's JSON response.
func Call(ctx context.Context, client *sshx.Client, mainPassword string, args ...string) (json.RawMessage, error) {
	if len(args) == 0 {
		return nil, errors.New("не указана команда")
	}
	payload, err := json.Marshal(request{MainPassword: mainPassword, Args: args})
	if err != nil {
		return nil, err
	}
	output, runErr := client.RunRoot(ctx, adminCommand, append(payload, '\n'))
	raw := lastJSONLine(output)
	if raw == nil {
		if runErr != nil {
			return nil, friendlyError(runErr.Error())
		}
		return nil, errors.New("сервер не вернул ответ")
	}
	var head response
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, err
	}
	if !head.OK {
		return nil, friendlyError(head.Message)
	}
	return raw, nil
}

// Restart restarts the wdtt service.
func Restart(ctx context.Context, client *sshx.Client) error {
	_, err := client.RunRoot(ctx, "systemctl restart wdtt", nil)
	return err
}

func lastJSONLine(output string) json.RawMessage {
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := bytes.TrimSpace([]byte(lines[i]))
		if len(line) > 1 && line[0] == '{' && line[len(line)-1] == '}' && json.Valid(line) {
			return line
		}
	}
	return nil
}

func friendlyError(message string) error {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "no such file or directory") && strings.Contains(lower, "wdtt-server"):
		return errors.New("wdtt-server не установлен: сначала выполните установку сервера")
	case strings.Contains(lower, "no_passwords_json"):
		return errors.New("на сервере нет базы WDTT Plus: сначала выполните установку")
	case strings.Contains(lower, "admin_socket_unavailable"):
		return errors.New("служба wdtt не запущена или устарела: перезапустите или переустановите сервер")
	case strings.Contains(lower, "неизвестная admin-команда"), strings.Contains(lower, "flag provided but not defined"):
		return errors.New("версия сервера не поддерживает эту операцию: обновите сервер")
	}
	return errors.New(strings.TrimSpace(message))
}
