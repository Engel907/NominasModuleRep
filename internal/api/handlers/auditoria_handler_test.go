package handlers

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactarJSONSensibles_RedactaPasswordHashesYTokensAnidados(t *testing.T) {
	input := json.RawMessage(`{"nombre":"Ana","password_hash":"secret-hash","datos":{"access_token":"secret-token","safe":12},"items":[{"refreshToken":"secret-refresh"}]}`)
	output, err := redactarJSONSensibles(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, secreto := range []string{"secret-hash", "secret-token", "secret-refresh"} {
		if strings.Contains(string(output), secreto) {
			t.Fatalf("la respuesta de auditoría conserva %q: %s", secreto, output)
		}
	}
	for _, clave := range []string{`"nombre":"Ana"`, `"safe":12`, `"password_hash":"***REDACTED***"`} {
		if !strings.Contains(string(output), clave) {
			t.Fatalf("la salida perdió o no redactó %q: %s", clave, output)
		}
	}
}
