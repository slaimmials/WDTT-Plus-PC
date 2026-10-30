package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type HashStatus string

const (
	HashOK           HashStatus = "ok"
	HashAuthRequired HashStatus = "auth_required"
	HashCaptcha      HashStatus = "captcha"
	HashDead         HashStatus = "dead"
	HashBlocked      HashStatus = "blocked"
	HashFull         HashStatus = "full"
	HashLimited      HashStatus = "limited"
	HashNetwork      HashStatus = "network"
	HashError        HashStatus = "error"
)

type HashCheckResult struct {
	Hash    string     `json:"hash"`
	Status  HashStatus `json:"status"`
	Message string     `json:"message"`
}

// CheckHashes asks VK for TURN credentials with every hash and reports which
// ones are usable. It shares process-wide VK settings with Start, so it cannot
// run while a session is active.
func CheckHashes(ctx context.Context, cfg Config, hooks Hooks, report func(index int, result HashCheckResult)) error {
	if len(cfg.Hashes) == 0 {
		return errors.New("не указан ни один VK-хеш")
	}
	if !runMu.TryLock() {
		return errors.New("проверка недоступна, пока активно подключение")
	}
	defer runMu.Unlock()
	if err := cfg.applyGlobals(); err != nil {
		return err
	}
	activeHooks.Store(&hooks)
	defer activeHooks.Store(nil)

	for i, hash := range cfg.Hashes {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		checkCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		_, _, turnURLs, err := GetCreds(checkCtx, hash, 9000+i)
		cancel()

		result := HashCheckResult{Hash: hash}
		if err == nil {
			result.Status = HashOK
			result.Message = fmt.Sprintf("TURN-серверов: %d", len(turnURLs))
		} else {
			result.Status, result.Message = classifyHashCheckError(err)
		}
		report(i, result)
	}
	return nil
}

func classifyHashCheckError(err error) (HashStatus, string) {
	if err == nil {
		return HashOK, ""
	}
	text := strings.ToLower(err.Error())
	containsAny := func(parts ...string) bool {
		for _, part := range parts {
			if strings.Contains(text, part) {
				return true
			}
		}
		return false
	}
	switch {
	case containsAny("error_code:9005", "call requires auth"):
		return HashAuthRequired, "Звонок требует входа в VK"
	case containsAny("captcha_required", "captcha_wait_required"):
		return HashCaptcha, "VK просит капчу"
	case containsAny("invalid_join_link", "call not found", "join link is not valid",
		"error 9000", "error 9008", "error_code:9000", "error_code:9008"):
		return HashDead, "Звонок не найден или закрыт"
	case containsAny("anon_blocked", "anonymous join is disabled"):
		return HashBlocked, "В звонке запрещён анонимный вход"
	case containsAny("call_full", "call is full"):
		return HashFull, "В звонке нет свободных мест"
	case containsAny("flood", "rate limit", "error_code:29"):
		return HashLimited, "VK временно ограничил запросы"
	case containsAny("timeout", "deadline", "lookup", "network", "vk https"):
		return HashNetwork, "Сетевая ошибка"
	default:
		return HashError, err.Error()
	}
}
