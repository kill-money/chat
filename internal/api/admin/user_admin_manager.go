// 二开：用户端管理员管理 + 推荐绑定查询 — 管理端 HTTP handlers（admin-api, port 10009）
package admin

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	admmodel "github.com/openimsdk/chat/pkg/common/db/model/admin"
	admindb "github.com/openimsdk/chat/pkg/common/db/table/admin"
	"github.com/openimsdk/tools/apiresp"
	"github.com/openimsdk/tools/db/mongoutil"
	"github.com/openimsdk/tools/db/pagination"
	"github.com/openimsdk/tools/errs"
)

// UserAdminManager handles admin CRUD for user_admins and referral_bindings.
type UserAdminManager struct {
	userAdmins admindb.UserAdminInterface
	bindings   admindb.ReferralBindingInterface
}

func NewUserAdminManager(cli *mongoutil.Client) (*UserAdminManager, error) {
	ua, err := admmodel.NewUserAdmin(cli.GetDB())
	if err != nil {
		return nil, err
	}
	rb, err := admmodel.NewReferralBinding(cli.GetDB())
	if err != nil {
		return nil, err
	}
	return &UserAdminManager{userAdmins: ua, bindings: rb}, nil
}

// ─── user admin CRUD ──────────────────────────────────────────────────────────

type addUserAdminReq struct {
	UserID string `json:"userID" binding:"required"`
}

// AddUserAdmin POST /user_admin/add
func (m *UserAdminManager) AddUserAdmin(c *gin.Context) {
	var req addUserAdminReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	u := &admindb.UserAdmin{
		ID:        uuid.New().String(),
		UserID:    req.UserID,
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	if err := m.userAdmins.Create(c.Request.Context(), u); err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, nil)
}

type removeUserAdminReq struct {
	UserID string `json:"userID" binding:"required"`
}

// RemoveUserAdmin POST /user_admin/remove
func (m *UserAdminManager) RemoveUserAdmin(c *gin.Context) {
	var req removeUserAdminReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	if err := m.userAdmins.Delete(c.Request.Context(), req.UserID); err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, nil)
}

type searchUserAdminsReq struct {
	Keyword    string              `json:"keyword"`
	Pagination pagination.Pagination `json:"pagination"`
}

// SearchUserAdmins POST /user_admin/search
func (m *UserAdminManager) SearchUserAdmins(c *gin.Context) {
	var req searchUserAdminsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	total, list, err := m.userAdmins.Search(c.Request.Context(), req.Keyword, req.Pagination)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, gin.H{"total": total, "list": list})
}

// ─── referral bindings ────────────────────────────────────────────────────────

type getReferralUsersReq struct {
	AdminID string `json:"adminID" binding:"required"`
}

// GetReferralUsers POST /user_admin/referral/users
func (m *UserAdminManager) GetReferralUsers(c *gin.Context) {
	var req getReferralUsersReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	ctx := c.Request.Context()
	list, err := m.bindings.ListByAdmin(ctx, req.AdminID)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	total, _ := m.bindings.CountByAdmin(ctx, req.AdminID)
	apiresp.GinSuccess(c, gin.H{"total": total, "list": list})
}
