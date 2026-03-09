// 二开：白名单登录控制 — 只有白名单中且状态为 active 的标识符可以登录
package admin

import (
	"context"
	"time"

	"github.com/openimsdk/tools/db/pagination"
)

// Whitelist status
const (
	WhitelistStatusDisabled int32 = 0
	WhitelistStatusActive   int32 = 1
)

// Whitelist identifier type
const (
	WhitelistTypePhone int32 = 1
	WhitelistTypeEmail int32 = 2
)

// WhitelistUser 登录白名单条目
type WhitelistUser struct {
	ID          string    `bson:"_id"`         // UUID
	Identifier  string    `bson:"identifier"`  // +8613800138000 or user@example.com
	Type        int32     `bson:"type"`        // 1=phone 2=email
	Role        string    `bson:"role"`        // admin/operator/user
	Permissions []string  `bson:"permissions"` // view_ip/ban_user/view_chat_log/broadcast
	Status      int32     `bson:"status"`      // 1=active 0=disabled
	Remark      string    `bson:"remark"`
	CreateTime  time.Time `bson:"create_time"`
	UpdateTime  time.Time `bson:"update_time"`
}

func (WhitelistUser) TableName() string { return "whitelist_users" }

type WhitelistInterface interface {
	TakeByIdentifier(ctx context.Context, identifier string) (*WhitelistUser, error)
	TakeByID(ctx context.Context, id string) (*WhitelistUser, error)
	Create(ctx context.Context, users []*WhitelistUser) error
	Update(ctx context.Context, id string, update map[string]any) error
	Delete(ctx context.Context, ids []string) error
	Search(ctx context.Context, keyword string, status int32, pagination pagination.Pagination) (int64, []*WhitelistUser, error)
}
