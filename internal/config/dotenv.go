package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// loadDotEnv loads an optional .env beside the executable or in the current
// directory. Values already supplied by the process environment always win;
// this keeps service-manager and container configuration authoritative.
func loadDotEnv() error {
	paths := make([]string, 0, 2)
	if executable, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(executable), ".env"))
	}
	if current, err := os.Getwd(); err == nil {
		currentEnv := filepath.Join(current, ".env")
		alreadyListed := false
		for _, path := range paths {
			if samePath(path, currentEnv) {
				alreadyListed = true
				break
			}
		}
		if !alreadyListed {
			paths = append(paths, currentEnv)
		}
	}

	for _, path := range paths {
		if err := loadDotEnvFile(path); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("carregar .env (%s): %w", path, err)
		}
	}
	return nil
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return filepath.Clean(left) == filepath.Clean(right)
	}
	return strings.EqualFold(filepath.Clean(leftAbs), filepath.Clean(rightAbs))
}

func loadDotEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		key, value, ok, err := parseDotEnvLine(scanner.Text())
		if err != nil {
			return fmt.Errorf("linha %d: %w", lineNumber, err)
		}
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("definir %s: %w", key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func parseDotEnvLine(line string) (key, value string, ok bool, err error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false, nil
	}
	line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	separator := strings.IndexByte(line, '=')
	if separator <= 0 {
		return "", "", false, fmt.Errorf("entrada inválida; esperado NOME=VALOR")
	}
	key = strings.TrimSpace(line[:separator])
	if !validDotEnvKey(key) {
		return "", "", false, fmt.Errorf("nome de variável inválido: %q", key)
	}
	value, err = parseDotEnvValue(strings.TrimSpace(line[separator+1:]))
	if err != nil {
		return "", "", false, err
	}
	return key, value, true, nil
}

func validDotEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for index, char := range key {
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || char == '_' || (index > 0 && char >= '0' && char <= '9') {
			continue
		}
		return false
	}
	return true
}

func parseDotEnvValue(value string) (string, error) {
	if len(value) < 2 {
		return value, nil
	}
	if value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1], nil
	}
	if value[0] != '"' {
		return value, nil
	}
	parsed, err := strconv.Unquote(value)
	if err != nil {
		return "", fmt.Errorf("valor entre aspas inválido: %w", err)
	}
	return parsed, nil
}
