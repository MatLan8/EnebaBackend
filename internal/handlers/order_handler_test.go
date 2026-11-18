package handlers

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/uuid"

	"eneba-backend/internal/models"
	"eneba-backend/internal/repository"
	"eneba-backend/internal/services"
)

func newTestKeyCipher(t *testing.T) *services.KeyCipher {
	t.Helper()

	keyCipher, err := services.NewKeyCipher(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}

	return keyCipher
}

func TestNewOrderKeysResponseDecryptFailureNamesKeyNotCode(t *testing.T) {
	keyCipher := newTestKeyCipher(t)
	brokenID := uuid.New()
	rows := []repository.OrderKeyRow{
		{KeyID: uuid.New(), CodeEncrypted: keyCipher.Encrypt("GOOD-CODE")},
		{KeyID: brokenID, CodeEncrypted: []byte("not encrypted with this secret")},
	}

	_, err := newOrderKeysResponse(models.Order{ID: uuid.New()}, rows, keyCipher)
	if err == nil {
		t.Fatal("want an error for the broken key")
	}
	if !strings.Contains(err.Error(), brokenID.String()) {
		t.Errorf("error %q should name the broken key %s", err, brokenID)
	}
	if strings.Contains(err.Error(), "GOOD-CODE") {
		t.Errorf("error %q must not contain a key code", err)
	}
}
