package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/vultisig/vultisig-relay/config"
	"github.com/vultisig/vultisig-relay/contexthelper"
	"github.com/vultisig/vultisig-relay/model"
)

var ErrNotFound = errors.New("not found")

// Storage is an interface that defines the methods to be implemented by a storage.
type Storage interface {
	SetSession(ctx context.Context, key string, participants []string) error
	GetSession(ctx context.Context, key string) ([]string, error)
	DeleteSession(ctx context.Context, key string) error
	GetMessages(ctx context.Context, key string) ([]model.Message, error)
	SetMessage(ctx context.Context, key string, message model.Message) error
	DeleteMessages(ctx context.Context, key string) error
	DeleteMessage(ctx context.Context, key string, hash string) error
	SetValue(ctx context.Context, key string, value string) error
	GetValue(ctx context.Context, key string) (string, error)
}

var _ Storage = (*RedisStorage)(nil)

type RedisStorage struct {
	cfg               config.RedisServer
	client            *redis.Client
	defaultExpiration time.Duration
	defaultUserExpire time.Duration
}

// NewRedisStorage returns a new storage that use redis
func NewRedisStorage(cfg config.RedisServer) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Username: cfg.User,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	status := client.Ping(context.Background())
	if status.Err() != nil {
		return nil, status.Err()
	}
	return &RedisStorage{
		cfg:               cfg,
		client:            client,
		defaultExpiration: time.Minute * 5,
		defaultUserExpire: time.Hour,
	}, nil
}

// SetSession sets a session with a list of participants.
func (s *RedisStorage) SetSession(ctx context.Context, key string, participants []string) error {
	if contexthelper.CheckCancellation(ctx) != nil {
		return ctx.Err()
	}
	existingParticipants, err := s.GetSession(ctx, key)
	if err != nil {
		return fmt.Errorf("fail to get existing session %s, err: %w", key, err)
	}
	var participantsToAdd []string
	for _, p := range participants {
		needAdd := true
		for _, existingP := range existingParticipants {
			if p == existingP {
				needAdd = false
				continue
			}
		}
		// add the participant if it does not exist
		if needAdd {
			participantsToAdd = append(participantsToAdd, p)
		}
	}
	if len(participantsToAdd) > 0 {
		if result := s.client.RPush(ctx, key, participantsToAdd); result.Err() != nil {
			return fmt.Errorf("fail to set session %s, err: %w", key, result.Err())
		}
	}
	if result := s.client.Expire(ctx, key, s.defaultExpiration); result.Err() != nil {
		return fmt.Errorf("fail to set expiration, err: %w", result.Err())
	}
	return nil
}

// GetSession gets a session with a list of participants.
func (s *RedisStorage) GetSession(ctx context.Context, key string) ([]string, error) {
	if contexthelper.CheckCancellation(ctx) != nil {
		return nil, ctx.Err()
	}
	result, err := s.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("fail to get session %s, err: %w", key, err)
	}
	return result, nil

}

// DeleteSession deletes a session.
func (s *RedisStorage) DeleteSession(ctx context.Context, key string) error {
	if contexthelper.CheckCancellation(ctx) != nil {
		return ctx.Err()
	}
	result := s.client.Del(ctx, key)
	if result.Err() != nil {
		return fmt.Errorf("fail to delete session %s, err: %w", key, result.Err())
	}
	return nil
}

// GetMessages gets a message from a session and a participant.
func (s *RedisStorage) GetMessages(ctx context.Context, key string) ([]model.Message, error) {
	if contexthelper.CheckCancellation(ctx) != nil {
		return nil, ctx.Err()
	}
	result, err := s.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("fail to get messages %s, err: %w", key, err)
	}

	var messages []model.Message
	for _, item := range result {
		var message model.Message
		if err := json.Unmarshal([]byte(item), &message); err != nil {
			return nil, fmt.Errorf("fail to unmarshal message, err: %w", err)
		}
		messages = append(messages, message)
	}
	return messages, nil
}

// SetMessage sets a message to a session and a participant.
func (s *RedisStorage) SetMessage(ctx context.Context, key string, message model.Message) error {
	if contexthelper.CheckCancellation(ctx) != nil {
		return ctx.Err()
	}
	existingMessages, err := s.GetMessages(ctx, key)
	if err != nil {
		return fmt.Errorf("fail to get existing messages, err: %w", err)
	}
	for _, m := range existingMessages {
		if m.Hash == message.Hash { // skip the message if it already exists
			return nil
		}
	}
	// add the message to the list
	buf, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("fail to marshal message, err: %w", err)
	}
	if status := s.client.RPush(ctx, key, string(buf)); status.Err() != nil {
		return fmt.Errorf("fail to set message, err: %w", status.Err())
	}
	if result := s.client.Expire(ctx, key, s.defaultExpiration); result.Err() != nil {
		return fmt.Errorf("fail to set expiration, err: %w", result.Err())
	}

	return nil
}

// DeleteMessages deletes a message from a session and a participant.
func (s *RedisStorage) DeleteMessages(ctx context.Context, key string) error {
	if contexthelper.CheckCancellation(ctx) != nil {
		return ctx.Err()
	}
	if status := s.client.Del(ctx, key); status.Err() != nil {
		return fmt.Errorf("fail to delete message, err: %w", status.Err())
	}
	return nil
}

// DeleteMessage deletes a message in the given key with hash equals to the given hash
func (s *RedisStorage) DeleteMessage(ctx context.Context, key, hash string) error {
	if contexthelper.CheckCancellation(ctx) != nil {
		return ctx.Err()
	}
	existingMessages, err := s.GetMessages(ctx, key)
	if err != nil {
		return fmt.Errorf("fail to get existing messages, err: %w", err)
	}
	var messageToRemove model.Message
	for _, m := range existingMessages {
		if m.Hash == hash {
			messageToRemove = m
			break
		}
	}
	if messageToRemove.Hash == "" {
		return nil
	}
	buf, err := json.Marshal(messageToRemove)
	if err != nil {
		return fmt.Errorf("fail to marshal message, err: %w", err)
	}
	if err := s.client.LRem(ctx, key, 1, string(buf)).Err(); err != nil {
		return fmt.Errorf("fail to delete message, err: %w", err)
	}

	if result := s.client.Expire(ctx, key, s.defaultExpiration); result.Err() != nil {
		return fmt.Errorf("fail to set expiration, err: %w", result.Err())
	}
	return nil
}

func (s *RedisStorage) SetValue(ctx context.Context, key string, value string) error {
	if contexthelper.CheckCancellation(ctx) != nil {
		return ctx.Err()
	}
	if status := s.client.Set(ctx, key, value, s.defaultUserExpire); status.Err() != nil {
		return fmt.Errorf("fail to set value %s, err: %w", key, status.Err())
	}

	return nil
}

func (s *RedisStorage) GetValue(ctx context.Context, key string) (string, error) {
	if contexthelper.CheckCancellation(ctx) != nil {
		return "", ctx.Err()
	}
	result, err := s.client.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("fail to get value %s, err: %w", key, err)
	}
	return result, nil
}

func (s *RedisStorage) Close() error {
	return s.client.Close()
}
