package admin

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openimsdk/chat/pkg/common/constant"
	"github.com/openimsdk/chat/pkg/common/db/database"
	dbutil "github.com/openimsdk/chat/pkg/common/db/dbutil"
	admmodel "github.com/openimsdk/chat/pkg/common/db/model/admin"
	admindb "github.com/openimsdk/chat/pkg/common/db/table/admin"
	chatdb "github.com/openimsdk/chat/pkg/common/db/table/chat"
	"github.com/openimsdk/tools/apiresp"
	"github.com/openimsdk/tools/db/mongoutil"
	"github.com/openimsdk/tools/errs"
)

// BatchCreateManager handles dynamic username batch user creation.
type BatchCreateManager struct {
	chatDB    database.ChatDatabaseInterface
	whitelist admindb.WhitelistInterface
}

func NewBatchCreateManager(cli *mongoutil.Client) (*BatchCreateManager, error) {
	chatDB, err := database.NewChatDatabase(cli)
	if err != nil {
		return nil, err
	}
	wl, err := admmodel.NewWhitelistUser(cli.GetDB())
	if err != nil {
		return nil, err
	}
	return &BatchCreateManager{chatDB: chatDB, whitelist: wl}, nil
}

// BatchCreateReq — request body for POST /user/batch_create
type BatchCreateReq struct {
	StartUsername string `json:"start_username" binding:"required"`
	Count         int    `json:"count"          binding:"required,min=1,max=999"`
	Password      string `json:"password"       binding:"required,min=6"`
	Role          string `json:"role"`
}

// BatchCreateResp — response body
type BatchCreateResp struct {
	Created   int      `json:"created"`
	Skipped   int      `json:"skipped"`
	Usernames []string `json:"usernames"`
}

var usernameRe = regexp.MustCompile(`^(.*?)(\d+)$`)

// parseUsername dynamically extracts prefix, numeric value, and digit width.
// e.g. "bab001" → prefix="bab", number=1, digits=3
func parseUsername(username string) (prefix string, number int, digits int, err error) {
	m := usernameRe.FindStringSubmatch(username)
	if m == nil {
		return "", 0, 0, fmt.Errorf("USERNAME_FORMAT_INVALID: username must end with digits")
	}
	prefix = m[1]
	digits = len(m[2])
	number, err = strconv.Atoi(m[2])
	return
}

func batchGenUserID() string {
	const l = 10
	data := make([]byte, l)
	rand.Read(data)
	chars := []byte("0123456789")
	for i := 0; i < len(data); i++ {
		if i == 0 {
			data[i] = chars[1:][data[i]%9]
		} else {
			data[i] = chars[data[i]%10]
		}
	}
	return string(data)
}

// BatchCreate handles POST /user/batch_create
func (m *BatchCreateManager) BatchCreate(c *gin.Context) {
	var req BatchCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}

	prefix, number, digits, err := parseUsername(req.StartUsername)
	if err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}

	role := req.Role
	if role == "" {
		role = "user"
	}

	ctx := c.Request.Context()
	var created []string
	skipped := 0

	for i := 0; i < req.Count; i++ {
		current := number + i
		suffix := fmt.Sprintf("%0*d", digits, current)
		username := prefix + suffix

		// Skip if credential already exists
		if _, cerr := m.chatDB.TakeCredentialByAccount(ctx, username); cerr == nil {
			skipped++
			continue
		} else if !dbutil.IsDBNotFound(cerr) {
			apiresp.GinError(c, cerr)
			return
		}

		userID := batchGenUserID()
		now := time.Now()

		register := &chatdb.Register{
			UserID:     userID,
			Mode:       constant.UserMode,
			CreateTime: now,
		}
		account := &chatdb.Account{
			UserID:     userID,
			Password:   req.Password,
			CreateTime: now,
			ChangeTime: now,
		}
		attribute := &chatdb.Attribute{
			UserID:         userID,
			Account:        username,
			Nickname:       username,
			CreateTime:     now,
			ChangeTime:     now,
			AllowVibration: constant.DefaultAllowVibration,
			AllowBeep:      constant.DefaultAllowBeep,
			AllowAddFriend: constant.DefaultAllowAddFriend,
			RegisterType:   constant.AccountRegister,
		}
		credentials := []*chatdb.Credential{
			{
				UserID:      userID,
				Account:     username,
				Type:        constant.CredentialAccount,
				AllowChange: true,
			},
		}

		if err := m.chatDB.RegisterUser(ctx, register, account, attribute, credentials); err != nil {
			apiresp.GinError(c, err)
			return
		}

		// Best-effort whitelist insert; skip on duplicate
		_ = m.whitelist.Create(ctx, []*admindb.WhitelistUser{{
			ID:         uuid.New().String(),
			Identifier: username,
			Type:       3, // account-based identifier
			Role:       role,
			Status:     1,
			CreateTime: now,
			UpdateTime: now,
		}})

		created = append(created, username)
	}

	apiresp.GinSuccess(c, BatchCreateResp{
		Created:   len(created),
		Skipped:   skipped,
		Usernames: created,
	})
}
