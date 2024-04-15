package server

import (
	"errors"
	"fmt"
	"github.com/rs/xid"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/voltix-vault/voltix-router/contexthelper"
	"github.com/voltix-vault/voltix-router/db"
	"github.com/voltix-vault/voltix-router/model"
	"github.com/voltix-vault/voltix-router/storage"
)

type Server struct {
	port int64
	s    storage.Storage
	dbs  *db.DBStorage
}

// NewServer returns a new server.
func NewServer(port int64, s storage.Storage, dbs *db.DBStorage) *Server {
	return &Server{
		port: port,
		s:    s,
		dbs:  dbs,
	}
}

func (s *Server) StartServer() error {
	e := echo.New()
	e.Logger.SetLevel(log.DEBUG)
	e.Pre(middleware.RemoveTrailingSlash())
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit("100M")) // set maximum allowed size for a request body to 100M
	e.GET("/ping", s.Ping)
	group := e.Group("", middleware.BasicAuth(s.checkBasicAuthentication))
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
	group.POST("/register", s.RegisterVault)

	reg := e.Group("/register", middleware.BasicAuth(s.checkBasicAuthenticationUserOnly))
	reg.POST("/vault", s.RegisterVault)
	return e.Start(fmt.Sprintf(":%d", s.port))

}
func (s *Server) Ping(c echo.Context) error {
	return c.String(http.StatusOK, "Voltix Router is running")
}

func (s *Server) checkBasicAuthentication(username, password string, c echo.Context) (bool, error) {
	// allow keygen to bypass basic auth
	if strings.EqualFold(c.Request().Header.Get("keygen"), "voltix") {
		return true, nil
	}
	// client should encode the basic authentication
	// apikey:pubkey
	apiKey := username
	vaultPubKey := password
	// check cache
	user, err := s.s.GetUser(c.Request().Context(), apiKey)
	if err != nil {
		c.Logger().Errorf("fail to get user %s from cache, err: %s", username, err)
	}
	if user != nil {
		// check number of vaults
		return s.checkUserAndVaultPubKey(c, user, vaultPubKey)
	}
	// fallback to database
	user, err = s.dbs.GetUser(c.Request().Context(), apiKey)
	if err != nil {
		c.Logger().Errorf("fail to get user %s from db, err: %s", apiKey, err)
		return false, err
	}
	if user != nil {
		if user.IsValid() {
			// save the user to cache
			if err := s.s.SetUser(c.Request().Context(), user.APIKey, *user); err != nil {
				c.Logger().Errorf("fail to save user to cache for %s, err: %s", user.APIKey, err)
			}
		}
		return s.checkUserAndVaultPubKey(c, user, vaultPubKey)
	}
	return false, nil
}

func (s *Server) checkBasicAuthenticationUserOnly(username, password string, c echo.Context) (bool, error) {
	// client should encode the basic authentication
	// apikey:pubkey
	apiKey := username
	// check cache
	user, err := s.s.GetUser(c.Request().Context(), apiKey)
	if err != nil {
		c.Logger().Errorf("fail to get user %s from cache, err: %s", username, err)
	}
	if user != nil {
		if user.IsValid() {
			c.Set("user", user)
			return true, nil
		}
		return false, nil
	}
	// fallback to database
	user, err = s.dbs.GetUser(c.Request().Context(), apiKey)
	if err != nil {
		c.Logger().Errorf("fail to get user %s from db, err: %s", apiKey, err)
		return false, err
	}
	if user != nil {
		if user.IsValid() {
			c.Set("user", user)
			// save the user to cache
			if err := s.s.SetUser(c.Request().Context(), user.APIKey, *user); err != nil {
				c.Logger().Errorf("fail to save user to cache for %s, err: %s", user.APIKey, err)
			}
			return true, nil
		}
	}
	return false, nil
}

func (s *Server) checkUserAndVaultPubKey(c echo.Context, user *model.User, vaultPubKey string) (bool, error) {
	if contexthelper.CheckCancellation(c.Request().Context()) != nil {
		return false, c.Request().Context().Err()
	}
	if !user.IsValid() {
		return false, nil
	}
	c.Set("user", user)
	if vaultPubKey == "" {
		return false, nil
	}
	vaults, err := s.s.GetUserVault(c.Request().Context(), user.APIKey)
	if err != nil {
		c.Logger().Errorf("fail to get user vault %s, err: %s", user.APIKey, err)
	}
	if len(vaults) > 0 {
		if slices.Contains(vaults, vaultPubKey) {
			return true, nil
		}
	}
	vaults, err = s.dbs.GetVaultPubKeys(c.Request().Context(), user.ID)
	if err != nil {
		c.Logger().Errorf("fail to get user vault %s, err: %s", user.APIKey, err)
		return false, fmt.Errorf("fail to get user vault %s, err: %w", user.APIKey, err)
	}
	if len(vaults) > 0 {
		// save it to cache
		if err := s.s.SetUserVault(c.Request().Context(), user.APIKey, vaults); err != nil {
			c.Logger().Errorf("fail to set user vault %s, err: %s", user.APIKey, err)
		}
		if slices.Contains(vaults, vaultPubKey) {
			return true, nil
		}
	}
	return false, nil
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
	if contexthelper.CheckCancellation(c.Request().Context()) != nil {
		return c.NoContent(http.StatusRequestTimeout)
	}
	sessionID := strings.TrimSpace(c.Param("sessionID"))
	rawParticipantID, err := url.QueryUnescape(c.Param("participantID"))
	if err != nil {
		c.Logger().Errorf("fail to unescape participant ID %s, err: %w", c.Param("participantID"), err)
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
		c.Logger().Errorf("fail to unescape participant ID %s, err: %w", c.Param("participantID"), err)
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
		c.Logger().Errorf("fail to delete message %s, err: %w", key, err)
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
func (s *Server) RegisterVault(c echo.Context) error {
	if contexthelper.CheckCancellation(c.Request().Context()) != nil {
		return c.NoContent(http.StatusRequestTimeout)
	}
	user := c.Get("user").(*model.User)
	if user == nil {
		return c.NoContent(http.StatusUnauthorized)
	}

	var vaultKeys struct {
		PubKeyECDSA string `json:"pub_key_ecdsa,omitempty"`
		PubKeyEdDSA string `json:"pub_key_eddsa,omitempty"`
	}

	if err := c.Bind(&vaultKeys); err != nil {
		c.Logger().Error(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := s.dbs.RegisterVault(c.Request().Context(), user.ID, vaultKeys.PubKeyECDSA, vaultKeys.PubKeyEdDSA, user.NoOfVaults); err != nil {
		c.Logger().Error(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusCreated)
}

func (s *Server) CreateUserAPIKey(c echo.Context) error {
	if contexthelper.CheckCancellation(c.Request().Context()) != nil {
		return c.NoContent(http.StatusRequestTimeout)
	}
	apiKey := xid.New().String()
	if err := s.dbs.NewUser(c.Request().Context(), apiKey); err != nil {
		c.Logger().Errorf("fail to create user %s, err: %s", apiKey, err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.JSON(http.StatusCreated, apiKey)
}
