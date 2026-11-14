package model

import "time"

// TxStatus 是某个连接/库上"手动提交"会话的当前状态，界面据此显示未提交条数和提交、回滚按钮。
type TxStatus struct {
	// Manual：该库已开启手动提交模式。
	Manual bool `json:"manual"`
	// Active：事务已开启（执行过写语句且尚未提交或回滚）。
	Active bool `json:"active"`
	// Pending：自事务开始以来成功执行的写语句数。
	Pending int `json:"pending"`
	// Since：事务开始的时间；未开启时为零值。
	Since time.Time `json:"since"`
	// Lost：因取消、断线等原因被迫回滚的累计次数；界面发现数字变大时提示用户。
	Lost int `json:"lost"`
}
