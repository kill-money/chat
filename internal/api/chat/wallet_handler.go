// 二开：钱包系统 — 用户端 HTTP handlers（chat-api, port 10008）
package chat

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

// WalletChatHandler handles user-facing wallet endpoints.
type WalletChatHandler struct {
	accounts     admindb.WalletAccountInterface
	transactions admindb.WalletTransactionInterface
	cards        admindb.BankCardInterface
}

func NewWalletChatHandler(cli *mongoutil.Client) (*WalletChatHandler, error) {
	a, err := admmodel.NewWalletAccount(cli.GetDB())
	if err != nil {
		return nil, err
	}
	t, err := admmodel.NewWalletTransaction(cli.GetDB())
	if err != nil {
		return nil, err
	}
	c, err := admmodel.NewBankCard(cli.GetDB())
	if err != nil {
		return nil, err
	}
	return &WalletChatHandler{accounts: a, transactions: t, cards: c}, nil
}

// ─── wallet info ──────────────────────────────────────────────────────────────

// GetWalletInfo POST /wallet/info
func (h *WalletChatHandler) GetWalletInfo(c *gin.Context) {
	userID := mctx.GetOpUserID(c)
	if userID == "" {
		apiresp.GinError(c, errs.ErrNoPermission.WrapMsg("not authenticated"))
		return
	}
	acc, err := h.accounts.GetOrCreate(c.Request.Context(), userID)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, gin.H{
		"userID":    acc.UserID,
		"balance":   acc.Balance,
		"currency":  acc.Currency,
		"updatedAt": acc.UpdatedAt,
	})
}

// ─── bank cards ───────────────────────────────────────────────────────────────

// ListCards POST /wallet/cards
func (h *WalletChatHandler) ListCards(c *gin.Context) {
	userID := mctx.GetOpUserID(c)
	if userID == "" {
		apiresp.GinError(c, errs.ErrNoPermission.WrapMsg("not authenticated"))
		return
	}
	list, err := h.cards.ListByUserID(c.Request.Context(), userID)
	if err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, gin.H{"list": list, "total": len(list)})
}

type addCardReq struct {
	BankName   string `json:"bankName" binding:"required"`
	CardNumber string `json:"cardNumber" binding:"required"`
	CardHolder string `json:"cardHolder" binding:"required"`
}

// AddCard POST /wallet/card/add
func (h *WalletChatHandler) AddCard(c *gin.Context) {
	userID := mctx.GetOpUserID(c)
	if userID == "" {
		apiresp.GinError(c, errs.ErrNoPermission.WrapMsg("not authenticated"))
		return
	}
	var req addCardReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	// Mask card number — keep only last 4 digits
	masked := req.CardNumber
	if len(masked) > 4 {
		masked = masked[len(masked)-4:]
	}
	card := &admindb.BankCard{
		ID:         uuid.New().String(),
		UserID:     userID,
		BankName:   req.BankName,
		CardNumber: masked,
		CardHolder: req.CardHolder,
		CreatedAt:  time.Now(),
	}
	if err := h.cards.Create(c.Request.Context(), card); err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, card)
}

type removeCardReq struct {
	ID string `json:"id" binding:"required"`
}

// RemoveCard POST /wallet/card/remove
func (h *WalletChatHandler) RemoveCard(c *gin.Context) {
	userID := mctx.GetOpUserID(c)
	if userID == "" {
		apiresp.GinError(c, errs.ErrNoPermission.WrapMsg("not authenticated"))
		return
	}
	var req removeCardReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	if err := h.cards.Delete(c.Request.Context(), req.ID, userID); err != nil {
		apiresp.GinError(c, err)
		return
	}
	apiresp.GinSuccess(c, nil)
}

// ─── withdraw (always rejected) ───────────────────────────────────────────────

type withdrawReq struct {
	Amount   int64  `json:"amount" binding:"required,min=1"`
	CardID   string `json:"cardID" binding:"required"`
	Note     string `json:"note"`
}

// Withdraw POST /wallet/withdraw — always returns a pending/rejection message.
func (h *WalletChatHandler) Withdraw(c *gin.Context) {
	userID := mctx.GetOpUserID(c)
	if userID == "" {
		apiresp.GinError(c, errs.ErrNoPermission.WrapMsg("not authenticated"))
		return
	}
	var req withdrawReq
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresp.GinError(c, errs.ErrArgs.WrapMsg(err.Error()))
		return
	}
	apiresp.GinSuccess(c, gin.H{
		"status":  "rejected",
		"message": "提现申请审核不通过，当前不满足提现条件，请联系客服处理。",
	})
}
