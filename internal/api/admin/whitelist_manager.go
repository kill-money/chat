// 二开：白名单管理 HTTP 处理器（admin API 层直接访问 MongoDB）
package admin

import (
	"time"

	"github.com/gin-gonic/gin"
	adminmodel "github.com/openimsdk/chat/pkg/common/db/model/admin"
	admindb "github.com/openimsdk/chat/pkg/common/db/table/admin"
	"github.com/google/uuid"
	"github.com/openimsdk/tools/apiresp"
	"github.com/openimsdk/tools/errs"
	"go.mongodb.org/mongo-driver/mongo"
)

// simplePage implements pagination.Pagination for whitelist searches
type simplePage struct {
	pageNum int32
	showNum int32
}

func (p *simplePage) GetPageNumber() int32 { return p.pageNum }
func (p *simplePage) GetShowNumber() int32 { return p.showNum }

// WhitelistManager 管理白名单的 HTTP 处理器
type WhitelistManager struct {
	db admindb.WhitelistInterface
}

func NewWhitelistManager(mongoDB *mongo.Database) (*WhitelistManager, error) {
	wl, err := adminmodel.NewWhitelistUser(mongoDB)
	if err != nil {
		return nil, err
	}
	return &WhitelistManager{db: wl}, nil
}

// AddWhitelistReq 添加白名单请求
type AddWhitelistReq struct {
	Identifier  string   `json:"identifier" binding:"required"` // +8613800138000 or email
	Type        int32    `json:"type" binding:"required"`       // 1=phone 2=email
	Role        string   `json:"role"`                          // admin/operator/user
	Permissions []string `json:"permissions"`                   // view_ip/ban_user/view_chat_log/broadcast
	Remark      string   `json:"remark"`
}

// UpdateWhitelistReq 修改白名单请求
type UpdateWhitelistReq struct {
	ID          string   `json:"id" binding:"required"`
	Role        *string  `json:"role"`
	Permissions []string `json:"permissions"`
	Status      *int32   `json:"status"` // 0=禁用 1=启用
	Remark      *string  `json:"remark"`
}

// DelWhitelistReq 删除白名单请求
type DelWhitelistReq struct {
	IDs []string `json:"ids" binding:"required"`
}

// SearchWhitelistReq 搜索白名单请求
type SearchWhitelistReq struct {
	Keyword string `json:"keyword"`
	Status  int32  `json:"status"`  // -1=全部 0=禁用 1=启用
	PageNum int32  `json:"pageNum"` // 1-based
	ShowNum int32  `json:"showNum"`
}

// AddWhitelist POST /whitelist/add
func (m *WhitelistManager) AddWhitelist(c *gin.Context) {
	var req AddWhitelistReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	if req.Role == "" {
		req.Role = "user"
	}
	now := time.Now()
	entry := &admindb.WhitelistUser{
		ID:          uuid.New().String(),
		Identifier:  req.Identifier,
		Type:        req.Type,
		Role:        req.Role,
		Permissions: req.Permissions,
		Status:      admindb.WhitelistStatusActive,
		Remark:      req.Remark,
		CreateTime:  now,
		UpdateTime:  now,
	}
	if err := m.db.Create(c, []*admindb.WhitelistUser{entry}); err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, entry)
}

// UpdateWhitelist POST /whitelist/update
func (m *WhitelistManager) UpdateWhitelist(c *gin.Context) {
	var req UpdateWhitelistReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	update := map[string]any{"update_time": time.Now()}
	if req.Role != nil {
		update["role"] = *req.Role
	}
	if req.Permissions != nil {
		update["permissions"] = req.Permissions
	}
	if req.Status != nil {
		update["status"] = *req.Status
	}
	if req.Remark != nil {
		update["remark"] = *req.Remark
	}
	if err := m.db.Update(c, req.ID, update); err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, nil)
}

// DelWhitelist POST /whitelist/del
func (m *WhitelistManager) DelWhitelist(c *gin.Context) {
	var req DelWhitelistReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	if err := m.db.Delete(c, req.IDs); err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, nil)
}

// SearchWhitelist POST /whitelist/search
func (m *WhitelistManager) SearchWhitelist(c *gin.Context) {
	var req SearchWhitelistReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	if req.ShowNum <= 0 {
		req.ShowNum = 20
	}
	if req.PageNum <= 0 {
		req.PageNum = 1
	}
	total, list, err := m.db.Search(c, req.Keyword, req.Status, &simplePage{
		pageNum: req.PageNum,
		showNum: req.ShowNum,
	})
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, map[string]any{
		"total": total,
		"list":  list,
	})
}
