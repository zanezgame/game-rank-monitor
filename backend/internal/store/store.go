// Package store 负责 SQLite 持久化：建表迁移与各模型的 CRUD。
package store

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"gamerank/internal/models"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store 是 SQLite 的封装，持有一个数据库连接池。
type Store struct {
	db *sql.DB
}

// AnalyzeFunc 为快照生成上榜理由的回调（由调用方传入 analyzer.Analyze，
// 避免 store 与 analyzer 包形成循环引用）。
type AnalyzeFunc func(prev *models.RankSnapshot, cur models.RankSnapshot, events []string) (code, text string)

// ChartEntry 是榜单查询返回的联表条目（含游戏名与图标）。
type ChartEntry struct {
	Rank        int
	GameID      string
	GameName    string
	IconURL     string
	Tags        []string
	RankChange  *int
	ReasonCode  string
	ReasonText  string
	PlayerCount *int64
}

// New 打开（不存在则创建）SQLite 数据库并执行迁移。
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath+"?cache=shared")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close 关闭数据库连接。
func (s *Store) Close() error { return s.db.Close() }

// migrate 按文件名顺序执行嵌入的 SQL 迁移脚本。
func (s *Store) migrate() error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("读取迁移目录失败: %w", err)
	}
	for _, e := range entries {
		b, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return fmt.Errorf("读取迁移 %s 失败: %w", e.Name(), err)
		}
		if _, err := s.db.Exec(string(b)); err != nil {
			return fmt.Errorf("执行迁移 %s 失败: %w", e.Name(), err)
		}
	}
	return nil
}

// nullInt64 把 *int64 转为可入库的 sql.NullInt64。
func nullInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

// nullInt 把 *int 转为可入库的 sql.NullInt64。
func nullInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

// fromNullInt64 把可空整数读回 *int64。
func fromNullInt64(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

// fromNullInt 把可空整数读回 *int。
func fromNullInt(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

// UpsertGame 插入或更新一款游戏的基础信息。
func (s *Store) UpsertGame(g models.Game) error {
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now()
	}
	_, err := s.db.Exec(`INSERT INTO games(id, platform, region, external_id, name, icon_url, description, price_text, tags_json, first_seen, created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, icon_url=excluded.icon_url,
			description=excluded.description, price_text=excluded.price_text, tags_json=excluded.tags_json`,
		g.ID, g.Platform, g.Region, g.ExternalID, g.Name, g.IconURL, g.Description,
		g.PriceText, g.TagsJSON, g.FirstSeen, g.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("保存游戏 %s 失败: %w", g.ID, err)
	}
	return nil
}

// SaveSnapshots 在一个事务内批量保存榜单快照（重复键则更新）。
func (s *Store) SaveSnapshots(snaps []models.RankSnapshot) error {
	if len(snaps) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO rank_snapshots(game_id, platform, region, chart_type, period, date, rank, rank_change, reason_code, reason_text, player_count)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(platform, region, chart_type, period, date, game_id)
		DO UPDATE SET rank=excluded.rank, rank_change=excluded.rank_change,
			reason_code=excluded.reason_code, reason_text=excluded.reason_text, player_count=excluded.player_count`)
	if err != nil {
		return fmt.Errorf("预编译快照语句失败: %w", err)
	}
	defer stmt.Close()
	for _, sn := range snaps {
		if _, err := stmt.Exec(sn.GameID, sn.Platform, sn.Region, sn.ChartType, sn.Period,
			sn.Date, sn.Rank, nullInt(sn.RankChange), sn.ReasonCode, sn.ReasonText,
			nullInt64(sn.PlayerCount)); err != nil {
			return fmt.Errorf("保存快照 %s %s 失败: %w", sn.GameID, sn.Date, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交快照事务失败: %w", err)
	}
	return nil
}

// LatestChartDate 返回某榜单已入库的最新日期。
func (s *Store) LatestChartDate(platform, region, chartType, period string) (string, bool) {
	var d sql.NullString
	err := s.db.QueryRow(`SELECT MAX(date) FROM rank_snapshots
		WHERE platform=? AND region=? AND chart_type=? AND period=?`,
		platform, region, chartType, period).Scan(&d)
	if err != nil || !d.Valid {
		return "", false
	}
	return d.String, true
}

// GetChart 查询某日榜单，按排名升序返回联表条目。
func (s *Store) GetChart(platform, region, chartType, period, date string) ([]ChartEntry, error) {
	rows, err := s.db.Query(`SELECT rs.rank, rs.game_id, g.name, g.icon_url, g.tags_json,
			rs.rank_change, rs.reason_code, rs.reason_text, rs.player_count
		FROM rank_snapshots rs JOIN games g ON g.id = rs.game_id
		WHERE rs.platform=? AND rs.region=? AND rs.chart_type=? AND rs.period=? AND rs.date=?
		ORDER BY rs.rank ASC`, platform, region, chartType, period, date)
	if err != nil {
		return nil, fmt.Errorf("查询榜单失败: %w", err)
	}
	defer rows.Close()
	var out []ChartEntry
	for rows.Next() {
		var e ChartEntry
		var tagsJSON string
		var rc, pc sql.NullInt64
		if err := rows.Scan(&e.Rank, &e.GameID, &e.GameName, &e.IconURL, &tagsJSON,
			&rc, &e.ReasonCode, &e.ReasonText, &pc); err != nil {
			return nil, fmt.Errorf("读取榜单行失败: %w", err)
		}
		e.RankChange = fromNullInt(rc)
		e.PlayerCount = fromNullInt64(pc)
		var tags []string
		if err := json.Unmarshal([]byte(tagsJSON), &tags); err == nil {
			e.Tags = tags
		} else {
			e.Tags = []string{}
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历榜单失败: %w", err)
	}
	return out, nil
}

// GetGame 按 ID 查询游戏。
func (s *Store) GetGame(id string) (models.Game, bool) {
	var g models.Game
	var created string
	err := s.db.QueryRow(`SELECT id, platform, region, external_id, name, icon_url, description,
			price_text, tags_json, first_seen, created_at FROM games WHERE id=?`, id).
		Scan(&g.ID, &g.Platform, &g.Region, &g.ExternalID, &g.Name, &g.IconURL,
			&g.Description, &g.PriceText, &g.TagsJSON, &g.FirstSeen, &created)
	if err != nil {
		return models.Game{}, false
	}
	if t, err := time.Parse(time.RFC3339, created); err == nil {
		g.CreatedAt = t
	}
	return g, true
}

// scanSnapshot 读取一行 rank_snapshots。
func scanSnapshot(row *sql.Row) (*models.RankSnapshot, bool) {
	var sn models.RankSnapshot
	var rc, pc sql.NullInt64
	err := row.Scan(&sn.GameID, &sn.Platform, &sn.Region, &sn.ChartType, &sn.Period,
		&sn.Date, &sn.Rank, &rc, &sn.ReasonCode, &sn.ReasonText, &pc)
	if err != nil {
		return nil, false
	}
	sn.RankChange = fromNullInt(rc)
	sn.PlayerCount = fromNullInt64(pc)
	return &sn, true
}

// GetPrevSnapshot 返回某游戏在指定榜单、指定日期之前最近的一次快照。
func (s *Store) GetPrevSnapshot(gameID, platform, region, chartType, period, beforeDate string) (*models.RankSnapshot, bool) {
	return scanSnapshot(s.db.QueryRow(`SELECT game_id, platform, region, chart_type, period, date, rank,
			rank_change, reason_code, reason_text, player_count FROM rank_snapshots
		WHERE game_id=? AND platform=? AND region=? AND chart_type=? AND period=? AND date < ?
		ORDER BY date DESC LIMIT 1`, gameID, platform, region, chartType, period, beforeDate))
}

// GetRankHistory 返回某游戏最近 days 天的排名历史（按日期升序）。
func (s *Store) GetRankHistory(gameID string, days int) ([]models.RankSnapshot, error) {
	rows, err := s.db.Query(`SELECT game_id, platform, region, chart_type, period, date, rank,
			rank_change, reason_code, reason_text, player_count FROM rank_snapshots
		WHERE game_id=? ORDER BY date DESC LIMIT ?`, gameID, days)
	if err != nil {
		return nil, fmt.Errorf("查询排名历史失败: %w", err)
	}
	defer rows.Close()
	var out []models.RankSnapshot
	for rows.Next() {
		var sn models.RankSnapshot
		var rc, pc sql.NullInt64
		if err := rows.Scan(&sn.GameID, &sn.Platform, &sn.Region, &sn.ChartType, &sn.Period,
			&sn.Date, &sn.Rank, &rc, &sn.ReasonCode, &sn.ReasonText, &pc); err != nil {
			return nil, fmt.Errorf("读取排名历史失败: %w", err)
		}
		sn.RankChange = fromNullInt(rc)
		sn.PlayerCount = fromNullInt64(pc)
		out = append(out, sn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历排名历史失败: %w", err)
	}
	// 按日期升序返回
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// SavePlayerSamples 批量保存玩家人数采样。
func (s *Store) SavePlayerSamples(samples []models.PlayerSample) error {
	if len(samples) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO player_samples(game_id, sampled_at, count) VALUES(?,?,?)`)
	if err != nil {
		return fmt.Errorf("预编译采样语句失败: %w", err)
	}
	defer stmt.Close()
	for _, p := range samples {
		if _, err := stmt.Exec(p.GameID, p.SampledAt.Format(time.RFC3339), p.Count); err != nil {
			return fmt.Errorf("保存玩家采样失败: %w", err)
		}
	}
	return tx.Commit()
}

// GetPlayerHistory 返回某游戏最近 days 天的玩家人数历史（按采样时间升序）。
func (s *Store) GetPlayerHistory(gameID string, days int) ([]models.PlayerSample, error) {
	since := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)
	rows, err := s.db.Query(`SELECT game_id, sampled_at, count FROM player_samples
		WHERE game_id=? AND sampled_at >= ? ORDER BY sampled_at ASC`, gameID, since)
	if err != nil {
		return nil, fmt.Errorf("查询玩家人数历史失败: %w", err)
	}
	defer rows.Close()
	var out []models.PlayerSample
	for rows.Next() {
		var p models.PlayerSample
		var at string
		if err := rows.Scan(&p.GameID, &at, &p.Count); err != nil {
			return nil, fmt.Errorf("读取玩家人数历史失败: %w", err)
		}
		if t, err := time.Parse(time.RFC3339, at); err == nil {
			p.SampledAt = t
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历玩家人数历史失败: %w", err)
	}
	return out, nil
}

// SaveDiscussions 批量保存玩家讨论，按 (game_id, source, author, content前200字) 去重。
func (s *Store) SaveDiscussions(discs []models.Discussion) error {
	if len(discs) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO discussions(game_id, source, author, content, sentiment, posted_at, url)
		VALUES(?,?,?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("预编译讨论语句失败: %w", err)
	}
	defer stmt.Close()
	for _, d := range discs {
		if _, err := stmt.Exec(d.GameID, d.Source, d.Author, d.Content, d.Sentiment,
			d.PostedAt.Format(time.RFC3339), d.URL); err != nil {
			return fmt.Errorf("保存讨论失败: %w", err)
		}
	}
	return tx.Commit()
}

// GetDiscussions 返回某游戏最近的 limit 条讨论（按发布时间倒序）。
func (s *Store) GetDiscussions(gameID string, limit int) ([]models.Discussion, error) {
	rows, err := s.db.Query(`SELECT game_id, source, author, content, sentiment, posted_at, url
		FROM discussions WHERE game_id=? ORDER BY posted_at DESC LIMIT ?`, gameID, limit)
	if err != nil {
		return nil, fmt.Errorf("查询讨论失败: %w", err)
	}
	defer rows.Close()
	var out []models.Discussion
	for rows.Next() {
		var d models.Discussion
		var at string
		if err := rows.Scan(&d.GameID, &d.Source, &d.Author, &d.Content, &d.Sentiment, &at, &d.URL); err != nil {
			return nil, fmt.Errorf("读取讨论失败: %w", err)
		}
		if t, err := time.Parse(time.RFC3339, at); err == nil {
			d.PostedAt = t
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历讨论失败: %w", err)
	}
	return out, nil
}

// BuildWeeklySnapshots 根据过去 7 天的 daily 快照聚合生成周榜快照。
// 对每个游戏取 7 天内排名的平均值（四舍五入）作为周排名，再与上周周榜对比
// 调用 analyze 生成上榜理由，最后入库并返回生成的快照列表。
func (s *Store) BuildWeeklySnapshots(platform, region, chartType, weekEndDate string, analyze AnalyzeFunc) ([]models.RankSnapshot, error) {
	end, err := time.Parse("2006-01-02", weekEndDate)
	if err != nil {
		return nil, fmt.Errorf("周榜截止日期格式错误: %w", err)
	}
	start := end.AddDate(0, 0, -6).Format("2006-01-02")

	rows, err := s.db.Query(`SELECT game_id, rank, date, player_count FROM rank_snapshots
		WHERE platform=? AND region=? AND chart_type=? AND period='daily' AND date>=? AND date<=?
		ORDER BY date ASC`, platform, region, chartType, start, weekEndDate)
	if err != nil {
		return nil, fmt.Errorf("查询周内日榜快照失败: %w", err)
	}
	defer rows.Close()

	type agg struct {
		sum   int
		n     int
		pc    *int64 // 最近一次非空的玩家数
	}
	byGame := map[string]*agg{}
	order := []string{}
	for rows.Next() {
		var gid, date string
		var rank int
		var pc sql.NullInt64
		if err := rows.Scan(&gid, &rank, &date, &pc); err != nil {
			return nil, fmt.Errorf("读取日榜快照失败: %w", err)
		}
		a, ok := byGame[gid]
		if !ok {
			a = &agg{}
			byGame[gid] = a
			order = append(order, gid)
		}
		a.sum += rank
		a.n++
		if pc.Valid {
			v := pc.Int64
			a.pc = &v
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历日榜快照失败: %w", err)
	}
	if len(byGame) == 0 {
		return nil, nil // 该周无日榜数据，跳过
	}

	type scored struct {
		gid string
		avg float64
		rk  int
	}
	var list []scored
	for gid, a := range byGame {
		avg := float64(a.sum) / float64(a.n)
		rk := int(avg + 0.5)
		if rk < 1 {
			rk = 1
		}
		list = append(list, scored{gid: gid, avg: avg, rk: rk})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].avg == list[j].avg {
			return list[i].gid < list[j].gid
		}
		return list[i].avg < list[j].avg
	})

	var snaps []models.RankSnapshot
	for i, sc := range list {
		a := byGame[sc.gid]
		cur := models.RankSnapshot{
			GameID:      sc.gid,
			Platform:    platform,
			Region:      region,
			ChartType:   chartType,
			Period:      "weekly",
			Date:        weekEndDate,
			Rank:        i + 1,
			PlayerCount: a.pc,
		}
		prev, ok := s.GetPrevSnapshot(sc.gid, platform, region, chartType, "weekly", weekEndDate)
		if ok {
			d := prev.Rank - cur.Rank
			cur.RankChange = &d
			code, text := analyze(prev, cur, nil)
			cur.ReasonCode, cur.ReasonText = code, text
		} else {
			code, text := analyze(nil, cur, nil)
			cur.ReasonCode, cur.ReasonText = code, text
		}
		snaps = append(snaps, cur)
	}
	if err := s.SaveSnapshots(snaps); err != nil {
		return nil, err
	}
	return snaps, nil
}
