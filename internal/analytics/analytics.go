package analytics

import (
	"database/sql"
	"encoding/json"
	"sort"
)

type Overview struct {
	Events   int64 `json:"events"`
	Sessions int64 `json:"sessions"`
	Projects int64 `json:"projects"`
	Agents   int64 `json:"agents"`
}

type Filter struct {
	From  string
	To    string
	Agent string
	Model string
}

func OverviewStats(db *sql.DB) (Overview, error) {
	return OverviewStatsFiltered(db, Filter{})
}

func OverviewStatsFiltered(db *sql.DB, f Filter) (Overview, error) {
	var o Overview
	where, args := buildWhere(f, false)
	_ = db.QueryRow(`SELECT COUNT(*) FROM events WHERE 1=1`+where, args...).Scan(&o.Events)
	if f.Agent != "" || f.From != "" || f.To != "" || f.Model != "" {
		_ = db.QueryRow(`SELECT COUNT(DISTINCT session) FROM events WHERE session != '' AND session IS NOT NULL`+where, args...).Scan(&o.Sessions)
		_ = db.QueryRow(`SELECT COUNT(DISTINCT project) FROM events WHERE project != '' AND project IS NOT NULL`+where, args...).Scan(&o.Projects)
		_ = db.QueryRow(`SELECT COUNT(DISTINCT agent) FROM events WHERE agent != ''`+where, args...).Scan(&o.Agents)
	} else {
		_ = db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&o.Sessions)
		_ = db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&o.Projects)
		_ = db.QueryRow(`SELECT COUNT(*) FROM agents`).Scan(&o.Agents)
		if o.Agents == 0 {
			_ = db.QueryRow(`SELECT COUNT(DISTINCT agent) FROM events WHERE agent != ''`).Scan(&o.Agents)
		}
	}
	return o, nil
}

type Bucket struct {
	Bucket string `json:"bucket"`
	Count  int64  `json:"count"`
}

func ActivityByDay(db *sql.DB, from, to string) ([]Bucket, error) {
	return ActivityByDayFiltered(db, Filter{From: from, To: to})
}

func ActivityByDayFiltered(db *sql.DB, f Filter) ([]Bucket, error) {
	where, args := buildWhere(f, true)
	q := `SELECT date(timestamp) as bucket, COUNT(*) FROM events WHERE 1=1` + where + ` GROUP BY bucket ORDER BY bucket`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Bucket
	for rows.Next() {
		var b Bucket
		_ = rows.Scan(&b.Bucket, &b.Count)
		out = append(out, b)
	}
	if out == nil {
		out = []Bucket{}
	}
	return out, nil
}

type TokenBucket struct {
	Bucket string  `json:"bucket"`
	Input  int64   `json:"input"`
	Output int64   `json:"output"`
	Total  int64   `json:"total"`
	Count  int64   `json:"count"`
	Cost   float64 `json:"cost"`
}

func TokensByDay(db *sql.DB) ([]TokenBucket, error) {
	return TokensByDayFiltered(db, Filter{})
}

func TokensByDayFiltered(db *sql.DB, f Filter) ([]TokenBucket, error) {
	where, args := buildWhere(f, true)
	q := `SELECT timestamp, payload, raw FROM events WHERE 1=1` + where + ` ORDER BY timestamp`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byDay := map[string]*TokenBucket{}
	for rows.Next() {
		var ts, payload, raw sql.NullString
		_ = rows.Scan(&ts, &payload, &raw)
		var p map[string]any
		hasTokens := false
		if payload.Valid && payload.String != "" && payload.String != "null" {
			_ = json.Unmarshal([]byte(payload.String), &p)
			if p != nil {
				if _, ok := p["input"]; ok {
					hasTokens = true
				}
				if _, ok := p["output"]; ok {
					hasTokens = true
				}
				if _, ok := p["input_tokens"]; ok {
					hasTokens = true
				}
				if _, ok := p["tokens"]; ok {
					hasTokens = true
				}
			}
		}
		if !hasTokens && raw.Valid && raw.String != "" {
			var r map[string]any
			_ = json.Unmarshal([]byte(raw.String), &r)
			if r != nil {
				if t, ok := r["tokens"].(map[string]any); ok {
					if p == nil {
						p = map[string]any{}
					}
					for k, v := range t {
						p[k] = v
					}
					hasTokens = true
				}
				if u, ok := r["usage"].(map[string]any); ok {
					if p == nil {
						p = map[string]any{}
					}
					for k, v := range u {
						p[k] = v
					}
					hasTokens = true
				}
				if !hasTokens {
					for _, k := range []string{"input", "output", "input_tokens", "output_tokens", "cached_input", "reasoning"} {
						if _, ok := r[k]; ok {
							if p == nil {
								p = map[string]any{}
							}
							p[k] = r[k]
							hasTokens = true
						}
					}
				}
			}
		}
		if !hasTokens || p == nil {
			continue
		}
		if inner, ok := p["tokens"].(map[string]any); ok {
			for k, v := range inner {
				if _, exists := p[k]; !exists {
					p[k] = v
				}
			}
		}
		day := ts.String
		if len(day) >= 10 {
			day = day[:10]
		}
		b := byDay[day]
		if b == nil {
			b = &TokenBucket{Bucket: day}
			byDay[day] = b
		}
		b.Input += toInt64(p["input"])
		b.Output += toInt64(p["output"])
		if b.Input == 0 {
			b.Input += toInt64(p["input_tokens"])
		}
		if b.Output == 0 {
			b.Output += toInt64(p["output_tokens"])
		}
		cached := toInt64(p["cached_input"])
		if cached == 0 {
			cached = toInt64(p["cache_read"])
		}
		reason := toInt64(p["reasoning"])
		if reason == 0 {
			reason = toInt64(p["reasoning_tokens"])
		}
		b.Total = b.Input + b.Output + cached + reason
		if b.Total == 0 {
			b.Total = toInt64(p["total"])
		}
		b.Cost += toFloat64(p["cost"])
		if b.Cost == 0 {
			b.Cost += toFloat64(p["price"])
		}
		b.Count++
	}
	var out []TokenBucket
	for _, v := range byDay {
		if v.Count == 0 && v.Total == 0 {
			continue
		}
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Bucket < out[j].Bucket })
	if out == nil {
		out = []TokenBucket{}
	}
	return out, nil
}

type HourlyTokenBucket struct {
	Bucket string  `json:"bucket"`
	Input  int64   `json:"input"`
	Output int64   `json:"output"`
	Total  int64   `json:"total"`
	Count  int64   `json:"count"`
	Cost   float64 `json:"cost"`
}

func TokensByHourFiltered(db *sql.DB, f Filter) ([]HourlyTokenBucket, error) {
	where, args := buildWhere(f, true)
	q := `SELECT timestamp, payload, raw FROM events WHERE 1=1` + where + ` ORDER BY timestamp`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byHour := map[string]*HourlyTokenBucket{}
	for rows.Next() {
		var ts, payload, raw sql.NullString
		_ = rows.Scan(&ts, &payload, &raw)
		var p map[string]any
		hasTokens := false
		if payload.Valid && payload.String != "" && payload.String != "null" {
			_ = json.Unmarshal([]byte(payload.String), &p)
			if p != nil {
				for _, k := range []string{"input", "output", "input_tokens", "output_tokens", "tokens", "usage"} {
					if _, ok := p[k]; ok {
						hasTokens = true
					}
				}
			}
		}
		if !hasTokens && raw.Valid && raw.String != "" {
			var r map[string]any
			_ = json.Unmarshal([]byte(raw.String), &r)
			if r != nil {
				for _, k := range []string{"input", "output", "input_tokens", "tokens", "usage"} {
					if _, ok := r[k]; ok {
						if p == nil {
							p = map[string]any{}
						}
						p[k] = r[k]
						hasTokens = true
					}
				}
				if t, ok := r["tokens"].(map[string]any); ok {
					if p == nil {
						p = map[string]any{}
					}
					for k, v := range t {
						if _, exists := p[k]; !exists {
							p[k] = v
						}
					}
					hasTokens = true
				}
				if u, ok := r["usage"].(map[string]any); ok {
					if p == nil {
						p = map[string]any{}
					}
					for k, v := range u {
						if _, exists := p[k]; !exists {
							p[k] = v
						}
					}
					hasTokens = true
				}
			}
		}
		if !hasTokens || p == nil {
			continue
		}
		if inner, ok := p["tokens"].(map[string]any); ok {
			for k, v := range inner {
				if _, exists := p[k]; !exists {
					p[k] = v
				}
			}
		}
		hour := ts.String
		if len(hour) >= 13 {
			hour = hour[:13] + ":00"
		}
		b := byHour[hour]
		if b == nil {
			b = &HourlyTokenBucket{Bucket: hour}
			byHour[hour] = b
		}
		b.Input += toInt64(p["input"])
		b.Output += toInt64(p["output"])
		if b.Input == 0 {
			b.Input += toInt64(p["input_tokens"])
		}
		if b.Output == 0 {
			b.Output += toInt64(p["output_tokens"])
		}
		cached := toInt64(p["cached_input"])
		if cached == 0 {
			cached = toInt64(p["cache_read"])
		}
		reason := toInt64(p["reasoning"])
		if reason == 0 {
			reason = toInt64(p["reasoning_tokens"])
		}
		b.Total = b.Input + b.Output + cached + reason
		if b.Total == 0 {
			b.Total = toInt64(p["total"])
		}
		b.Cost += toFloat64(p["cost"])
		if b.Cost == 0 {
			b.Cost += toFloat64(p["price"])
		}
		b.Count++
	}
	var out []HourlyTokenBucket
	for _, v := range byHour {
		if v.Count == 0 && v.Total == 0 {
			continue
		}
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Bucket < out[j].Bucket })
	if out == nil {
		out = []HourlyTokenBucket{}
	}
	return out, nil
}

type ToolStat struct {
	Tool        string  `json:"tool"`
	Calls       int64   `json:"calls"`
	AvgMs       float64 `json:"avg_duration_ms"`
	SuccessRate float64 `json:"success_rate"`
}

func ToolsStats(db *sql.DB) ([]ToolStat, error) {
	return ToolsStatsFiltered(db, Filter{})
}

func ToolsStatsFiltered(db *sql.DB, f Filter) ([]ToolStat, error) {
	where, args := buildWhereSimple(f)
	q := `SELECT type, payload FROM events WHERE type LIKE 'tool.%'` + where
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]*struct{ count, success int64; dur int64 }{}
	for rows.Next() {
		var typ, payload sql.NullString
		_ = rows.Scan(&typ, &payload)
		tool := "unknown"
		var dur int64
		var success int64 = 1
		if payload.Valid && payload.String != "" {
			var p map[string]any
			_ = json.Unmarshal([]byte(payload.String), &p)
			if v, ok := p["tool"].(string); ok && v != "" {
				tool = v
			} else if v, ok := p["name"].(string); ok && v != "" {
				tool = v
			}
			dur = toInt64(p["duration_ms"])
			if dur == 0 {
				dur = toInt64(p["duration"])
			}
			if s, ok := p["success"].(bool); ok && !s {
				success = 0
			}
			if s, ok := p["status"].(string); ok && (s == "error" || s == "failed") {
				success = 0
			}
		}
		e := m[tool]
		if e == nil {
			e = &struct{ count, success int64; dur int64 }{}
			m[tool] = e
		}
		e.count++
		e.success += success
		e.dur += dur
	}
	var out []ToolStat
	for k, v := range m {
		avg := 0.0
		if v.count > 0 {
			avg = float64(v.dur) / float64(v.count)
		}
		rate := 0.0
		if v.count > 0 {
			rate = float64(v.success) / float64(v.count)
		}
		out = append(out, ToolStat{Tool: k, Calls: v.count, AvgMs: avg, SuccessRate: rate})
	}
	if out == nil {
		out = []ToolStat{}
	}
	return out, nil
}

type ModelStat struct {
	Model  string  `json:"model"`
	Calls  int64   `json:"calls"`
	Input  int64   `json:"input"`
	Output int64   `json:"output"`
	Cost   float64 `json:"cost"`
}

func ModelsStats(db *sql.DB) ([]ModelStat, error) {
	return ModelsStatsFiltered(db, Filter{})
}

func ModelsStatsFiltered(db *sql.DB, f Filter) ([]ModelStat, error) {
	where, args := buildWhere(f, true)
	q := `SELECT payload, raw FROM events WHERE 1=1` + where + ` ORDER BY timestamp`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]*ModelStat{}
	for rows.Next() {
		var payload, raw sql.NullString
		_ = rows.Scan(&payload, &raw)
		var p map[string]any
		if payload.Valid && payload.String != "" && payload.String != "null" {
			_ = json.Unmarshal([]byte(payload.String), &p)
		}
		if (p == nil || p["model"] == nil) && raw.Valid && raw.String != "" {
			var r map[string]any
			_ = json.Unmarshal([]byte(raw.String), &r)
			if r != nil {
				if p == nil {
					p = map[string]any{}
				}
				for _, k := range []string{"model", "model_name", "input", "output", "input_tokens", "output_tokens", "tokens", "usage", "cost", "price"} {
					if v, ok := r[k]; ok {
						if _, exists := p[k]; !exists {
							p[k] = v
						}
					}
				}
				if t, ok := r["tokens"].(map[string]any); ok {
					for k, v := range t {
						if _, exists := p[k]; !exists {
							p[k] = v
						}
					}
				}
			}
		}
		if p == nil {
			continue
		}
		hasTokens := false
		for _, k := range []string{"input", "output", "input_tokens", "output_tokens", "tokens", "usage"} {
			if _, ok := p[k]; ok {
				hasTokens = true
				break
			}
		}
		if inner, ok := p["tokens"].(map[string]any); ok {
			for k, v := range inner {
				if _, exists := p[k]; !exists {
					p[k] = v
				}
			}
			hasTokens = true
		}
		model, _ := p["model"].(string)
		if model == "" {
			model, _ = p["model_name"].(string)
		}
		if model == "" {
			if !hasTokens {
				continue
			}
			model = "unknown"
		}
		s := m[model]
		if s == nil {
			s = &ModelStat{Model: model}
			m[model] = s
		}
		s.Calls++
		s.Input += toInt64(p["input"])
		if s.Input == 0 {
			s.Input += toInt64(p["input_tokens"])
		}
		s.Output += toInt64(p["output"])
		if s.Output == 0 {
			s.Output += toInt64(p["output_tokens"])
		}
		if s.Input == 0 && s.Output == 0 {
			if t, ok := p["tokens"].(map[string]any); ok {
				s.Input += toInt64(t["input"])
				s.Output += toInt64(t["output"])
			}
		}
		s.Cost += toFloat64(p["cost"])
		if toFloat64(p["cost"]) == 0 {
			s.Cost += toFloat64(p["price"])
		}
	}
	var out []ModelStat
	for _, v := range m {
		if f.Model != "" && v.Model != f.Model {
			continue
		}
		if v.Calls == 0 {
			continue
		}
		out = append(out, *v)
	}
	if out == nil {
		out = []ModelStat{}
	}
	return out, nil
}

func DistinctAgents(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT agent FROM events WHERE agent != '' AND agent IS NOT NULL ORDER BY agent`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s sql.NullString
		_ = rows.Scan(&s)
		if s.Valid && s.String != "" {
			out = append(out, s.String)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out, nil
}

func DistinctModels(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT COALESCE(json_extract(payload,'$.model'), json_extract(payload,'$.model_name'), '') as m FROM events WHERE type='generation.completed' AND m != '' ORDER BY m`)
	if err != nil {
		rows2, err2 := db.Query(`SELECT payload FROM events WHERE type='generation.completed'`)
		if err2 != nil {
			return nil, err
		}
		defer rows2.Close()
		set := map[string]struct{}{}
		for rows2.Next() {
			var p sql.NullString
			_ = rows2.Scan(&p)
			if !p.Valid || p.String == "" {
				continue
			}
			var m map[string]any
			_ = json.Unmarshal([]byte(p.String), &m)
			if v, _ := m["model"].(string); v != "" {
				set[v] = struct{}{}
			} else if v, _ := m["model_name"].(string); v != "" {
				set[v] = struct{}{}
			}
		}
		var out []string
		for k := range set {
			out = append(out, k)
		}
		sort.Strings(out)
		if out == nil {
			out = []string{}
		}
		return out, nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s sql.NullString
		_ = rows.Scan(&s)
		if s.Valid && s.String != "" {
			out = append(out, s.String)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out, nil
}

func buildWhere(f Filter, includeModel bool) (string, []any) {
	q := ""
	var args []any
	if f.From != "" {
		q += ` AND timestamp >= ?`
		args = append(args, f.From)
	}
	if f.To != "" {
		q += ` AND timestamp <= ?`
		args = append(args, f.To)
	}
	if f.Agent != "" {
		q += ` AND agent = ?`
		args = append(args, f.Agent)
	}
	if includeModel && f.Model != "" {
		q += ` AND COALESCE(json_extract(payload,'$.model'), json_extract(payload,'$.model_name'), '') = ?`
		args = append(args, f.Model)
	}
	return q, args
}

func buildWhereSimple(f Filter) (string, []any) {
	q := ""
	var args []any
	if f.From != "" {
		q += ` AND timestamp >= ?`
		args = append(args, f.From)
	}
	if f.To != "" {
		q += ` AND timestamp <= ?`
		args = append(args, f.To)
	}
	if f.Agent != "" {
		q += ` AND agent = ?`
		args = append(args, f.Agent)
	}
	return q, args
}

func ErrorCount(db *sql.DB) (int64, error) {
	var n int64
	err := db.QueryRow(`SELECT COUNT(*) FROM events WHERE type='error'`).Scan(&n)
	return n, err
}

func ErrorCountFiltered(db *sql.DB, f Filter) (int64, error) {
	where, args := buildWhereSimple(f)
	var n int64
	err := db.QueryRow(`SELECT COUNT(*) FROM events WHERE type='error'`+where, args...).Scan(&n)
	return n, err
}

type LatencyStat struct {
	P50   float64 `json:"p50_ms"`
	P95   float64 `json:"p95_ms"`
	P99   float64 `json:"p99_ms"`
	Avg   float64 `json:"avg_ms"`
	Count int64   `json:"count"`
}

func LatencyStats(db *sql.DB) (LatencyStat, error) {
	return LatencyStatsFiltered(db, Filter{})
}

func LatencyStatsFiltered(db *sql.DB, f Filter) (LatencyStat, error) {
	where, args := buildWhereSimple(f)
	q := `SELECT payload FROM events WHERE type IN ('generation.completed','tool.completed','command.completed')` + where
	rows, err := db.Query(q, args...)
	if err != nil {
		return LatencyStat{}, err
	}
	defer rows.Close()
	var vals []float64
	var sum float64
	for rows.Next() {
		var payload sql.NullString
		_ = rows.Scan(&payload)
		if !payload.Valid {
			continue
		}
		var p map[string]any
		_ = json.Unmarshal([]byte(payload.String), &p)
		v := toFloat64(p["duration_ms"])
		if v == 0 {
			v = toFloat64(p["latency_ms"])
		}
		if v == 0 {
			v = toFloat64(p["duration"])
		}
		if v > 0 {
			vals = append(vals, v)
			sum += v
		}
	}
	if len(vals) == 0 {
		return LatencyStat{}, nil
	}
	sortFloat64s(vals)
	return LatencyStat{
		P50:   percentile(vals, 50),
		P95:   percentile(vals, 95),
		P99:   percentile(vals, 99),
		Avg:   sum / float64(len(vals)),
		Count: int64(len(vals)),
	}, nil
}

func sortFloat64s(a []float64) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func percentile(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func toFloat64(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	}
	return 0
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	}
	return 0
}
