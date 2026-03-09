// 二开：推荐系统 — 用户端 HTTP handler（chat-api, port 10008）
// 处理推荐绑定、登录时 user admin 状态、自动消息发送
package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	admmodel "github.com/openimsdk/chat/pkg/common/db/model/admin"
	admindb "github.com/openimsdk/chat/pkg/common/db/table/admin"
	"github.com/openimsdk/chat/pkg/common/imapi"
	"github.com/openimsdk/tools/db/mongoutil"
)

// ReferralChatHandler 管理推荐绑定和 user admin 校验。
type ReferralChatHandler struct {
	userAdmins admindb.UserAdminInterface
	bindings   admindb.ReferralBindingInterface
	imCaller   imapi.CallerInterface
}

func NewReferralChatHandler(cli *mongoutil.Client, imCaller imapi.CallerInterface) (*ReferralChatHandler, error) {
	ua, err := admmodel.NewUserAdmin(cli.GetDB())
	if err != nil {
		return nil, err
	}
	rb, err := admmodel.NewReferralBinding(cli.GetDB())
	if err != nil {
		return nil, err
	}
	return &ReferralChatHandler{
		userAdmins: ua,
		bindings:   rb,
		imCaller:   imCaller,
	}, nil
}

// IsUserAdmin returns true if userID is in the user_admins collection with enabled=true.
func (h *ReferralChatHandler) IsUserAdmin(ctx context.Context, userID string) bool {
	ok, err := h.userAdmins.IsAdmin(ctx, userID)
	if err != nil {
		return false
	}
	return ok
}

// BindAfterRegister creates a referral_binding record for the new user if referrerID is a valid
// user admin, then sends:
//  1. A welcome message to the new user from the admin.
//  2. A system notification to the admin about the new user.
//
// Non-fatal: all errors are silently swallowed.
// imAdminCtx must carry the IM admin token (via mctx.WithApiToken).
func (h *ReferralChatHandler) BindAfterRegister(
	ctx context.Context,
	userID string,
	nickname string,
	registerIP string,
	referrerID string,
	imAdminCtx context.Context,
) {
	if referrerID == "" {
		return
	}
	// Validate: referrerID must be in user_admins
	if !h.IsUserAdmin(ctx, referrerID) {
		return
	}
	// Duplicate check
	if _, err := h.bindings.TakeByUserID(ctx, userID); err == nil {
		// Already bound — skip binding but still send welcome
		h.sendWelcome(imAdminCtx, referrerID, userID)
		return
	}

	binding := &admindb.ReferralBinding{
		ID:           uuid.New().String(),
		AdminID:      referrerID,
		UserID:       userID,
		Nickname:     nickname,
		RegisterIP:   registerIP,
		RegisterTime: time.Now(),
	}
	if err := h.bindings.Create(ctx, binding); err != nil {
		return
	}

	// Auto add friend (best-effort)
	_ = h.imCaller.ImportFriend(imAdminCtx, referrerID, []string{userID})
	_ = h.imCaller.ImportFriend(imAdminCtx, userID, []string{referrerID})

	// Count total referrals for this admin
	total, _ := h.bindings.CountByAdmin(ctx, referrerID)

	h.sendWelcome(imAdminCtx, referrerID, userID)
	h.sendAdminNotification(imAdminCtx, referrerID, userID, nickname, registerIP, binding.RegisterTime, total)
}

func (h *ReferralChatHandler) sendWelcome(ctx context.Context, adminID, userID string) {
	_ = h.imCaller.SendTextMsg(ctx, adminID, userID, "",
		"欢迎加入系统，我是您的专属管理员。有问题可以直接联系我。")
}

func (h *ReferralChatHandler) sendAdminNotification(ctx context.Context, adminID, userID, nickname, ip string, t time.Time, total int64) {
	msg := fmt.Sprintf(
		"系统为你分配一位新用户\n\n账号: %s\n昵称: %s\nIP地址: %s\n注册时间: %s\n当前累计用户: %d",
		userID, nickname, ip, t.Format("2006-01-02 15:04"), total,
	)
	_ = h.imCaller.SendTextMsg(ctx, adminID, adminID, "", msg)
}
