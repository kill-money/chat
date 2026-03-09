// 二开：接待员邀请码管理 — 管理端 HTTP handlers（admin-api, port 10009）
package admin

import (
	"github.com/gin-gonic/gin"
	dbutil "github.com/openimsdk/chat/pkg/common/db/dbutil"
	admmodel "github.com/openimsdk/chat/pkg/common/db/model/admin"
	admindb "github.com/openimsdk/chat/pkg/common/db/table/admin"
	"github.com/openimsdk/tools/apiresp"
	"github.com/openimsdk/tools/db/mongoutil"
	"github.com/openimsdk/tools/db/pagination"
	"github.com/openimsdk/tools/errs"
)

// ReceptionistManager exposes admin CRUD for invite codes + bindings.
type ReceptionistManager struct {
	inviteCodes admindb.ReceptionistInviteCodeInterface
	bindings    admindb.CustomerBindingInterface
}

func NewReceptionistManager(cli *mongoutil.Client) (*ReceptionistManager, error) {
	ic, err := admmodel.NewReceptionistInviteCode(cli.GetDB())
	if err != nil {
		return nil, err
	}
	cb, err := admmodel.NewCustomerBinding(cli.GetDB())
	if err != nil {
		return nil, err
	}
	return &ReceptionistManager{inviteCodes: ic, bindings: cb}, nil
}

// ─── invite code CRUD ─────────────────────────────────────────────────────────

type searchInviteCodesReq struct {
	Keyword  string              `json:"keyword"`
	Pagination pagination.Pagination `json:"pagination"`
}

// SearchInviteCodes POST /receptionist/invite_codes/search
func (m *ReceptionistManager) SearchInviteCodes(c *gin.Context) {
	var req searchInviteCodesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	total, list, err := m.inviteCodes.Search(c.Request.Context(), req.Keyword, req.Pagination)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, gin.H{"total": total, "list": list})
}

type updateStatusReq struct {
	ID     string `json:"id"     binding:"required"`
	Status int32  `json:"status" binding:"required"`
}

// UpdateInviteCodeStatus POST /receptionist/invite_codes/update_status
func (m *ReceptionistManager) UpdateInviteCodeStatus(c *gin.Context) {
	var req updateStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	if err := m.inviteCodes.UpdateStatus(c.Request.Context(), req.ID, req.Status); err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, nil)
}

type deleteInviteCodeReq struct {
	ID string `json:"id" binding:"required"`
}

// DeleteInviteCode POST /receptionist/invite_codes/delete
func (m *ReceptionistManager) DeleteInviteCode(c *gin.Context) {
	var req deleteInviteCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	if err := m.inviteCodes.Delete(c.Request.Context(), req.ID); err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, nil)
}

// ─── binding CRUD ─────────────────────────────────────────────────────────────

type getBindingReq struct {
	CustomerID string `json:"customerID" binding:"required"`
}

// GetBinding POST /receptionist/bindings/get
func (m *ReceptionistManager) GetBinding(c *gin.Context) {
	var req getBindingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	binding, err := m.bindings.TakeByCustomerID(c.Request.Context(), req.CustomerID)
	if err != nil {
		if dbutil.IsDBNotFound(err) {
			apiresp.GinSuccess(c, gin.H{"found": false})
			return
		}
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, gin.H{"found": true, "binding": binding})
}

type listBindingsReq struct {
	ReceptionistID string `json:"receptionistID" binding:"required"`
}

// ListBindings POST /receptionist/bindings/list
func (m *ReceptionistManager) ListBindings(c *gin.Context) {
	var req listBindingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	ctx := c.Request.Context()
	list, err := m.bindings.FindByReceptionist(ctx, req.ReceptionistID)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	total, _ := m.bindings.CountByReceptionist(ctx, req.ReceptionistID)
	apiresp.GinSuccess(c, gin.H{"total": total, "list": list})
}

type deleteBindingReq struct {
	CustomerID string `json:"customerID" binding:"required"`
}

// DeleteBinding POST /receptionist/bindings/delete
func (m *ReceptionistManager) DeleteBinding(c *gin.Context) {
	var req deleteBindingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	if err := m.bindings.Delete(c.Request.Context(), req.CustomerID); err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, nil)
}
