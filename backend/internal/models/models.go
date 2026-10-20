// Package models 定义游戏榜单监控平台的核心数据模型。
package models

import "time"

// Game 表示一款游戏的基础信息。
// ID 格式为 platform:region:externalID，例如 steam:global:730、appstore:cn:123456789。
type Game struct {
	ID          string    // 主键，platform:region:externalID 格式
	Platform    string    // 平台：steam / appstore / googleplay
	Region      string    // 区域：steam 为 global，appstore 为 cn/us
	ExternalID  string    // 平台侧的原始 ID（如 Steam appid、App Store im:id）
	Name        string    // 游戏名称
	IconURL     string    // 图标/封面图 URL
	Description string    // 简介
	PriceText   string    // 价格文本（如 "免费"、"¥98.00"）
	TagsJSON    string    // 标签 JSON 数组字符串
	FirstSeen   string    // 首次入库日期 YYYY-MM-DD
	CreatedAt   time.Time // 入库时间
}

// RankSnapshot 表示某款游戏在某日某榜单上的一次上榜快照。
// 主键维度：platform × region × chartType × period × date × gameID。
type RankSnapshot struct {
	GameID      string // 外键 games.id
	Platform    string // 平台
	Region      string // 区域
	ChartType   string // 榜单类型："hot" 热门榜 / "new" 新游榜
	Period      string // 周期："daily" 日榜 / "weekly" 周榜
	Date        string // 榜单日期 YYYY-MM-DD
	Rank        int    // 当日排名（1 开始）
	RankChange  *int   // 排名变化（正数=上升），首日/无历史数据时为 nil
	ReasonCode  string // 上榜理由码：new_entry / re_entry / surge / drop / stable
	ReasonText  string // 上榜理由中文描述
	PlayerCount *int64 // 采集时的在线玩家数（仅 Steam 有效）
}

// PlayerSample 表示某款游戏在某个时刻的玩家人数采样（主要为 Steam 在线人数）。
type PlayerSample struct {
	GameID     string    // 外键 games.id
	SampledAt  time.Time // 采样时间
	Count      int64     // 玩家人数
}

// Discussion 表示一条玩家讨论（评论），目前主要来自 Steam 商店评论。
type Discussion struct {
	GameID    string    // 外键 games.id
	Source    string    // 来源：steam_reviews 等
	Author    string    // 作者（Steam 账号 ID 脱敏后的展示名）
	Content   string    // 评论内容
	Sentiment string    // 情感倾向：positive / negative / neutral
	PostedAt  time.Time // 发布时间
	URL       string    // 来源链接
}
