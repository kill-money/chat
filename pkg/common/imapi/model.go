package imapi

import "github.com/openimsdk/protocol/sdkws"

// SendSingleMsgReq defines the structure for sending a message to multiple recipients.
type SendSingleMsgReq struct {
	// groupMsg should appoint sendID
	SendID          string                 `json:"sendID"`
	Content         string                 `json:"content" binding:"required"`
	OfflinePushInfo *sdkws.OfflinePushInfo `json:"offlinePushInfo"`
	Ex              string                 `json:"ex"`
}
type SendSingleMsgResp struct{}

// 二开：SendMsgReq — send a text message from one user to another via /msg/send_msg
type SendMsgReq struct {
	SendID           string      `json:"sendID"`
	RecvID           string      `json:"recvID"`
	SenderNickname   string      `json:"senderNickname"`
	SenderPlatformID int32       `json:"senderPlatformID"`
	Content          interface{} `json:"content"`
	ContentType      int32       `json:"contentType"`
	SessionType      int32       `json:"sessionType"`
	SendTime         int64       `json:"sendTime"`
}
type SendMsgResp struct{}
