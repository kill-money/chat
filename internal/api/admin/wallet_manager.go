// 二开：钱包系统 — 管理端 HTTP handlers（admin-api, port 10009）
package admin

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	admmodel "github.com/openimsdk/chat/pkg/common/db/model/admin"
	admindb "github.com/openimsdk/chat/pkg/common/db/table/admin"
	"github.com/openimsdk/chat/pkg/common/mctx"
	"github.com/openimsdk/tools/apiresp"
	"github.com/openimsdk/tools/db/mongoutil"
	"github.com/openimsdk/tools/errs"
)

// walletPage is a concrete pagination params struct for JSON binding.
type walletPage struct {
	PageNumber int32 `json:"pageNumber"`
	ShowNumber int32 `json:"showNumber"`
}

func (p *walletPage) GetPageNumber() int32 {
	if p == nil || p.PageNumber <= 0 {
		return 1
	}
	return p.PageNumber
}

func (p *walletPage) GetShowNumber() int32 {
	if p == nil || p.ShowNumber <= 0 {
		return 20
	}
	return p.ShowNumber
}

// WalletManager handles admin CRUD for wallet_accounts, wallet_transactions.
type WalletManager struct {
	accounts     admindb.WalletAccountInterface
	transactions admindb.WalletTransactionInterface
}

func NewWalletManager(cli *mongoutil.Client) (*WalletManager, error) {
	a, err := admmodel.NewWalletAccount(cli.GetDB())
	if err != nil {
		return nil, err
	}
	t, err := admmodel.NewWalletTransaction(cli.GetDB())
	if err != nil {
		return nil, err
	}
	return &WalletManager{accounts: a, transactions: t}, nil
}

// ─── get user wallet ──────────────────────────────────────────────────────────

type getUserWalletReq struct {
	UserID string `json:"userID" binding:"required"`
}

// GetUserWallet POST /wallet/user
func (m *WalletManager) GetUserWallet(c *gin.Context) {
	var req getUserWalletReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	acc, err := m.accounts.GetOrCreate(c.Request.Context(), req.UserID)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, acc)
}

// ─── adjust balance ───────────────────────────────────────────────────────────

type adjustBalanceReq struct {
	UserID string `json:"userID" binding:"required"`
	// Amount in cents, positive = credit, negative = debit
	Amount int64  `json:"amount" binding:"required"`
	Note   string `json:"note"`
}

// AdjustBalance POST /wallet/adjust
func (m *WalletManager) AdjustBalance(c *gin.Context) {
	var req adjustBalanceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	ctx := c.Request.Context()

	acc, err := m.accounts.AdjustBalance(ctx, req.UserID, req.Amount)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}

	opAdmin := mctx.GetOpUserID(c)
	tx := &admindb.WalletTransaction{
		ID:           uuid.New().String(),
		UserID:       req.UserID,
		Amount:       req.Amount,
		BalanceAfter: acc.Balance,
		Note:         req.Note,
		OpAdminID:    opAdmin,
		CreatedAt:    time.Now(),
	}
	_ = m.transactions.Create(ctx, tx) // best-effort

	apiresp.GinSuccess(c, gin.H{"balance": acc.Balance, "transaction": tx})
}

// ─── get transactions ─────────────────────────────────────────────────────────

type getTransactionsReq struct {
	UserID     string      `json:"userID" binding:"required"`
	Pagination *walletPage `json:"pagination"`
}

// GetTransactions POST /wallet/transactions
func (m *WalletManager) GetTransactions(c *gin.Context) {
	var req getTransactionsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	total, list, err := m.transactions.SearchByUserID(c.Request.Context(), req.UserID, req.Pagination)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, gin.H{"total": total, "list": list})
}
