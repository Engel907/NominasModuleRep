package db

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var (
	migrationUpName = regexp.MustCompile(`^(\d+)_.*\.up\.sql$`)
	forbiddenSQL    = []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"ON DUPLICATE KEY UPDATE", regexp.MustCompile(`(?i)\bON\s+DUPLICATE\s+KEY\s+UPDATE\b`)},
		{"INSERT IGNORE", regexp.MustCompile(`(?i)\bINSERT\s+IGNORE\b`)},
		{"DROP TABLE IF EXISTS", regexp.MustCompile(`(?i)\bDROP\s+TABLE\s+IF\s+EXISTS\b`)},
		{"DELIMITER", regexp.MustCompile(`(?i)\bDELIMITER\b`)},
		{"CREATE DATABASE", regexp.MustCompile(`(?i)\bCREATE\s+DATABASE\b`)},
	}
)

func TestMigracionesLint(t *testing.T) {
	migrationFiles, err := filepath.Glob(filepath.Join("..", "..", "db", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatalf("buscando migraciones: %v", err)
	}

	for _, upPath := range migrationFiles {
		name := filepath.Base(upPath)
		match := migrationUpName.FindStringSubmatch(name)
		if match == nil {
			continue
		}
		number, err := strconv.Atoi(match[1])
		if err != nil {
			t.Errorf("%s: número de migración inválido: %v", name, err)
			continue
		}
		if number < 120 {
			continue
		}

		content, err := os.ReadFile(upPath)
		if err != nil {
			t.Errorf("%s: leyendo migración: %v", name, err)
			continue
		}
		for _, forbidden := range forbiddenSQL {
			if forbidden.pattern.Match(content) {
				t.Errorf("%s contiene SQL prohibido: %s", name, forbidden.name)
			}
		}

		downPath := strings.TrimSuffix(upPath, ".up.sql") + ".down.sql"
		if _, err := os.Stat(downPath); err != nil {
			if os.IsNotExist(err) {
				t.Errorf("%s no tiene su migración down correspondiente (%s)", name, filepath.Base(downPath))
			} else {
				t.Errorf("%s: verificando migración down: %v", name, err)
			}
		}
	}
}
