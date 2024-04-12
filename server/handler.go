package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/voltix-vault/voltix-router/model"
	"github.com/voltix-vault/voltix-router/storage"
)

type Server struct {
	port int64
	s    storage.Storage
}

// NewServer returns a new server.
func NewServer(port int64, s storage.Storage) *Server {
	return &Server{s: s}
}

// StartSession is to start a new session that will be used to send and receive messages.
func (s *Server) StartSession(c echo.Context) error {
	sessionID := strings.TrimSpace(c.Param("sessionID"))
	if sessionID == "" {
		return c.NoContent(http.StatusBadRequest)
	}
	var p []string
	if err := c.Bind(&p); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := s.s.SetSession(c.Request().Context(), sessionID, p); err != nil {
		c.Logger().Error(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusCreated)
}

func (s *Server) GetSession(c echo.Context) error {
	sessionID := strings.TrimSpace(c.Param("sessionID"))
	if sessionID == "" {
		return c.NoContent(http.StatusBadRequest)
	}
	p, err := s.s.GetSession(c.Request().Context(), sessionID)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	return c.JSON(http.StatusOK, p)
}

// DeleteSession is to end a session. Remove all relevant messages
func (s *Server) DeleteSession(c echo.Context) error {
	sessionID := strings.TrimSpace(c.Param("sessionID"))
	if sessionID == "" {
		return c.NoContent(http.StatusBadRequest)
	}

	if err := s.s.DeleteSession(c.Request().Context(), sessionID); err != nil { // delete session
		c.Logger().Errorf("fail to delete session %s,err: %w", sessionID, err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusOK)
}

func (s *Server) GetMessage(c echo.Context) error {
	sessionID := strings.TrimSpace(c.Param("sessionID"))
	participantID := strings.TrimSpace(c.Param("participantID"))
	if sessionID == "" || participantID == "" {
		return c.NoContent(http.StatusBadRequest)
	}
	messageID := c.Request().Header.Get("message_id")
	c.Logger().Debug("session ID is ", sessionID, ", participant ID is ", participantID, ", message ID is ", messageID)
	key := fmt.Sprintf("%s-%s", sessionID, participantID)
	if messageID != "" {
		key = fmt.Sprintf("%s-%s-%s", sessionID, participantID, messageID)
	}
	messages, err := s.s.GetMessages(c.Request().Context(), key)
	if errors.Is(err, storage.ErrNotFound) {
		return c.NoContent(http.StatusOK)
	}

	return c.JSON(http.StatusOK, messages)
}

// DeleteMessage is to delete a message.
func (s *Server) DeleteMessage(c echo.Context) error {
	sessionID := strings.TrimSpace(c.Param("sessionID"))
	participantID := strings.TrimSpace(c.Param("participantID"))
	messageID := c.Request().Header.Get("message_id")
	msgHash := strings.TrimSpace(c.Param("hash"))
	if sessionID == "" || participantID == "" || msgHash == "" {
		return c.NoContent(http.StatusBadRequest)
	}
	key := fmt.Sprintf("%s-%s", sessionID, participantID)
	if messageID != "" {
		key = fmt.Sprintf("%s-%s-%s", sessionID, participantID, messageID)
	}
	if err := s.s.DeleteMessage(c.Request().Context(), key, msgHash); err != nil {
		c.Logger().Errorf("fail to delete message %s, err: %w", key, err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusOK)
}

func (s *Server) PostMessage(c echo.Context) error {
	sessionID := strings.TrimSpace(c.Param("sessionID"))
	if sessionID == "" {
		c.Logger().Error("session ID is empty")
		return c.NoContent(http.StatusBadRequest)
	}
	c.Logger().Debug("session ID is ", sessionID)
	messageID := c.Request().Header.Get("message_id")
	var m model.Message
	if err := c.Bind(&m); err != nil {
		c.Logger().Error(err)
		return c.NoContent(http.StatusBadRequest)
	}
	for _, item := range m.To {
		key := fmt.Sprintf("%s-%s", sessionID, item)
		if messageID != "" {
			key = fmt.Sprintf("%s-%s-%s", sessionID, item, messageID)
		}
		if err := s.s.SetMessage(c.Request().Context(), key, m); err != nil {
			c.Logger().Error(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}
	return c.NoContent(http.StatusAccepted)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
