package manager

import (
	"bytes"
	"context"
	"encoding/base64"

	"github.com/google/tink/go/aead"
	"github.com/google/tink/go/insecurecleartextkeyset"
	"github.com/google/tink/go/keyset"
	"github.com/google/tink/go/tink"
	"github.com/google/wire"
	"github.com/puppetlabs/relay-pls/pkg/model"
)

var KeyManagerProviderSet = wire.NewSet(
	NewKeyManager,
)

type KeyManager struct {
}

func (m *KeyManager) Create(ctx context.Context) (string, error) {
	kt := aead.AES256GCMKeyTemplate()

	kh, err := keyset.NewHandle(kt)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	writer := keyset.NewBinaryWriter(&buf)

	if err := insecurecleartextkeyset.Write(kh, writer); err != nil {
		return "", err
	}

	return base64.RawStdEncoding.EncodeToString(buf.Bytes()), nil
}

func (m *KeyManager) Decrypt(ctx context.Context, key string, data []byte) ([]byte, error) {
	a, err := m.cipher(ctx, key, data)
	if err != nil {
		return nil, err
	}
	return a.Decrypt(data, nil)
}

func (m *KeyManager) Encrypt(ctx context.Context, key string, data []byte) ([]byte, error) {
	a, err := m.cipher(ctx, key, data)
	if err != nil {
		return nil, err
	}
	return a.Encrypt(data, nil)
}

func (m *KeyManager) cipher(ctx context.Context, key string, data []byte) (tink.AEAD, error) {
	r, _ := base64.RawStdEncoding.DecodeString(key)
	read := bytes.NewBuffer(r)
	reader := keyset.NewBinaryReader(read)
	kh, err := insecurecleartextkeyset.Read(reader)
	if err != nil {
		return nil, err
	}

	a, err := aead.New(kh)
	if err != nil {
		return nil, err
	}

	return a, nil
}

func NewKeyManager() model.KeyManager {
	return &KeyManager{}
}
