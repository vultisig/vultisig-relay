package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"

	"github.com/vultisig/vultisig-relay/contexthelper"
	"github.com/vultisig/vultisig-relay/model"
	"github.com/vultisig/vultisig-relay/storage"
)

type Server struct {
	port int64
	s    storage.Storage
	e    *echo.Echo
}

// NewServer returns a new server.
func NewServer(port int64, s storage.Storage) *Server {
	return &Server{
		port: port,
		s:    s,
		e:    echo.New(),
	}
}

func (s *Server) StartServer() error {
	e := s.e
	e.Logger.SetLevel(log.DEBUG)
	e.Pre(middleware.RemoveTrailingSlash())
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	//enable cors
	e.Use(middleware.CORS())
	e.Use(middleware.BodyLimit("100M")) // set maximum allowed size for a request body to 100M
	e.GET("/ping", s.Ping)
	group := e.Group("")
	group.POST("/:sessionID", s.StartSession)
	group.GET("/:sessionID", s.GetSession)
	group.DELETE("/:sessionID", s.DeleteSession)
	group.POST("/message/:sessionID", s.PostMessage)
	group.GET("/message/:sessionID/:participantID", s.GetMessage)
	group.DELETE("/message/:sessionID/:participantID/:hash", s.DeleteMessage)
	group.POST("/start/:sessionID", s.StartTSSSession)
	group.GET("/start/:sessionID", s.GetStartTSSSession)
	group.POST("/complete/:sessionID", s.SetCompleteTSSSession)
	group.GET("/complete/:sessionID", s.GetCompleteTSSSession)
	group.POST("/complete/:sessionID/keysign", s.SetKeysignFinished)
	group.GET("/complete/:sessionID/keysign", s.GetKeysignFinished)
	return e.Start(fmt.Sprintf(":%d", s.port))
}

func (s *Server) StopServer() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return s.e.Shutdown(ctx)
}
func (s *Server) Ping(c echo.Context) error {
	return c.String(http.StatusOK, "Voltix Router is running")
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
		c.Logger().Errorf("fail to delete session %s,err: %s", sessionID, err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusOK)
}

func (s *Server) GetMessage(c echo.Context) error {
	if contexthelper.CheckCancellation(c.Request().Context()) != nil {
		return c.NoContent(http.StatusRequestTimeout)
	}
	sessionID := strings.TrimSpace(c.Param("sessionID"))
	rawParticipantID, err := url.QueryUnescape(c.Param("participantID"))
	if err != nil {
		c.Logger().Errorf("fail to unescape participant ID %s, err: %s", c.Param("participantID"), err)
		return c.NoContent(http.StatusBadRequest)
	}
	participantID := strings.TrimSpace(rawParticipantID)
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
	if messages == nil {
		messages = []model.Message{}
	}
	return c.JSON(http.StatusOK, messages)
}

// DeleteMessage is to delete a message.
func (s *Server) DeleteMessage(c echo.Context) error {
	if contexthelper.CheckCancellation(c.Request().Context()) != nil {
		return c.NoContent(http.StatusRequestTimeout)
	}
	sessionID := strings.TrimSpace(c.Param("sessionID"))
	rawParticipantID, err := url.QueryUnescape(c.Param("participantID"))
	if err != nil {
		c.Logger().Errorf("fail to unescape participant ID %s, err: %s", c.Param("participantID"), err)
		return c.NoContent(http.StatusBadRequest)
	}
	participantID := strings.TrimSpace(rawParticipantID)
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
		c.Logger().Errorf("fail to delete message %s, err: %s", key, err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusOK)
}

func (s *Server) PostMessage(c echo.Context) error {
	if contexthelper.CheckCancellation(c.Request().Context()) != nil {
		return c.NoContent(http.StatusRequestTimeout)
	}
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
func (s *Server) handleTSSSession(c echo.Context, sessionPrefix string) error {
	if contexthelper.CheckCancellation(c.Request().Context()) != nil {
		return c.NoContent(http.StatusRequestTimeout)
	}
	sessionID := strings.TrimSpace(c.Param("sessionID"))
	if sessionID == "" {
		return c.NoContent(http.StatusBadRequest)
	}
	var p []string
	if err := c.Bind(&p); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	key := fmt.Sprintf("%s-%s", sessionPrefix, sessionID)
	if err := s.s.SetSession(c.Request().Context(), key, p); err != nil {
		c.Logger().Error(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusOK)
}
func (s *Server) getTSSSession(c echo.Context, sessionPrefix string) error {
	if contexthelper.CheckCancellation(c.Request().Context()) != nil {
		return c.NoContent(http.StatusRequestTimeout)
	}
	sessionID := strings.TrimSpace(c.Param("sessionID"))
	if sessionID == "" {
		return c.NoContent(http.StatusBadRequest)
	}
	key := fmt.Sprintf("%s-%s", sessionPrefix, sessionID)
	participants, err := s.s.GetSession(c.Request().Context(), key)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	return c.JSON(http.StatusOK, participants)
}

func (s *Server) StartTSSSession(c echo.Context) error {
	return s.handleTSSSession(c, "start")
}

func (s *Server) GetStartTSSSession(c echo.Context) error {
	return s.getTSSSession(c, "start")
}

func (s *Server) SetCompleteTSSSession(c echo.Context) error {
	return s.handleTSSSession(c, "complete")
}

func (s *Server) GetCompleteTSSSession(c echo.Context) error {
	return s.getTSSSession(c, "complete")
}
func (s *Server) SetKeysignFinished(c echo.Context) error {
	if contexthelper.CheckCancellation(c.Request().Context()) != nil {
		return c.NoContent(http.StatusRequestTimeout)
	}
	sessionID := strings.TrimSpace(c.Param("sessionID"))
	if sessionID == "" {
		return c.NoContent(http.StatusBadRequest)
	}
	messageID := c.Request().Header.Get("message_id")
	key := fmt.Sprintf("keysign-%s-%s-complete", sessionID, messageID)
	input, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if s.s.SetValue(c.Request().Context(), key, string(input)) != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusOK)
}
func (s *Server) GetKeysignFinished(c echo.Context) error {
	if contexthelper.CheckCancellation(c.Request().Context()) != nil {
		return c.NoContent(http.StatusRequestTimeout)
	}
	sessionID := strings.TrimSpace(c.Param("sessionID"))
	if sessionID == "" {
		return c.NoContent(http.StatusBadRequest)
	}
	messageID := c.Request().Header.Get("message_id")
	key := fmt.Sprintf("keysign-%s-%s-complete", sessionID, messageID)
	value, err := s.s.GetValue(c.Request().Context(), key)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	return c.String(http.StatusOK, value)
}
