package xemoji

import (
	"encoding/json"
	"fmt"
	"strings"

	emoji "github.com/Andrew-M-C/go.emoji"
	"github.com/google/uuid"
)

func EncodeEmojiToID(allSettings map[string]any) (map[string]string, error) {
	by, err := json.Marshal(allSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal allSettings: %w", err)
	}

	data := string(by)

	emojiKv := make(map[string]string)
	final := emoji.ReplaceAllEmojiFunc(data, func(emoji string) string {
		id, ok := emojiKv[emoji]
		if ok {
			return id
		}

		id = "{{.%EMOJI%}}" + uuid.NewString() + "{{.%EMOJI%}}"

		emojiKv[emoji] = id

		return id
	})

	err = json.Unmarshal([]byte(final), &allSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal allSettings: %w", err)
	}

	return emojiKv, nil
}

func DecodeEmojiFromID(by []byte, emojiKv map[string]string) []byte {
	data := string(by)

	for em, id := range emojiKv {
		data = strings.ReplaceAll(data, id, em)
	}

	return []byte(data)
}
