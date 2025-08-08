package api

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	Router       *gin.Engine
	DB           *pgxpool.Pool
	APIKeyHeader string
	Cache        Cache
}

func NewServer(db *pgxpool.Pool) *Server {
	g := gin.New()
	g.Use(gin.Recovery())
	g.Use(gin.Logger())

	s := Server{Router: g, DB: db, APIKeyHeader: getenv("API_KEY_HEADER", "X-API-Key")}
	if addr := os.Getenv("REDIS_ADDR"); addr != "" && getenv("ENABLE_CACHE", "true") == "true" {
		s.Cache = NewRedisCache(addr)
	}

	g.GET("/health", s.health)
	v1 := g.Group("/v1")
	{
		v1.GET("/permits", s.listPermits)
		v1.GET("/permits/:id", s.getPermit)
	}
	return s
}

func (s *Server) health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := s.DB.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "db": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) listPermits(c *gin.Context) {
	// Optional read-through cache
	if s.Cache != nil {
		key := cacheKey(c.Request.URL.Query())
		if val, err := s.Cache.Get(c.Request.Context(), key); err == nil && val != "" {
			c.Data(http.StatusOK, "application/json", []byte(val))
			return
		}
	}
	// Parse query params
	bbox := c.Query("bbox")                 // minLon,minLat,maxLon,maxLat
	lat := c.Query("lat")
	lon := c.Query("lon")
	radius := c.DefaultQuery("radius_m", "0")
	types := c.QueryArray("permit_type")     // repeated
	if len(types) == 0 {
		if t := c.Query("permit_type"); t != "" {
			types = strings.Split(t, ",")
		}
	}
	startFrom := c.Query("start_date_from")
	startTo := c.Query("start_date_to")
	endFrom := c.Query("end_date_from")
	endTo := c.Query("end_date_to")
	status := c.Query("status")
	limit := c.DefaultQuery("limit", "100")

	var conds []string
	var args []any
	argIdx := 1

	// Spatial filter
	if bbox != "" {
		// ST_Intersects with envelope
		conds = append(conds, fmt.Sprintf("ST_Intersects(location_geom, ST_MakeEnvelope($%d,$%d,$%d,$%d,4326))", argIdx, argIdx+1, argIdx+2, argIdx+3))
		parts := strings.Split(bbox, ",")
		if len(parts) != 4 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bbox"})
			return
		}
		for _, p := range parts { args = append(args, p) }
		argIdx += 4
	} else if lat != "" && lon != "" && radius != "0" {
		conds = append(conds, fmt.Sprintf("ST_DWithin(location_geom::geography, ST_SetSRID(ST_Point($%d,$%d),4326)::geography, $%d)", argIdx+1, argIdx, argIdx+2))
		args = append(args, lon, lat, radius)
		argIdx += 3
	}

	// Temporal overlap via daterange
	if startFrom != "" && endTo != "" {
		conds = append(conds, fmt.Sprintf("daterange(start_date, end_date, '[]') && daterange($%d::date, $%d::date, '[]')", argIdx, argIdx+1))
		args = append(args, startFrom, endTo)
		argIdx += 2
	} else {
		if startFrom != "" {
			conds = append(conds, fmt.Sprintf("start_date >= $%d::date", argIdx))
			args = append(args, startFrom)
			argIdx++
		}
		if startTo != "" {
			conds = append(conds, fmt.Sprintf("start_date <= $%d::date", argIdx))
			args = append(args, startTo)
			argIdx++
		}
		if endFrom != "" {
			conds = append(conds, fmt.Sprintf("end_date >= $%d::date", argIdx))
			args = append(args, endFrom)
			argIdx++
		}
		if endTo != "" {
			conds = append(conds, fmt.Sprintf("end_date <= $%d::date", argIdx))
			args = append(args, endTo)
			argIdx++
		}
	}

	// Types
	if len(types) > 0 {
		conds = append(conds, fmt.Sprintf("permit_type = ANY($%d)", argIdx))
		args = append(args, types)
		argIdx++
	}

	// Status
	if status != "" {
		conds = append(conds, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}

	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	query := fmt.Sprintf(`SELECT id, source_id, source_url, permit_type, title, description, issuing_entity, start_date, end_date, location_str, geocode_confidence, status, artifact_key, hash, created_at, updated_at FROM permits%s ORDER BY created_at DESC, id DESC LIMIT $%d`, where, argIdx)
	args = append(args, limit)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type Permit struct {
		ID               string    `json:"id"`
		SourceID         string    `json:"source_id"`
		SourceURL        *string   `json:"source_url,omitempty"`
		PermitType       string    `json:"permit_type"`
		Title            *string   `json:"title,omitempty"`
		Description      *string   `json:"description,omitempty"`
		IssuingEntity    *string   `json:"issuing_entity,omitempty"`
		StartDate        *string   `json:"start_date,omitempty"`
		EndDate          *string   `json:"end_date,omitempty"`
		LocationStr      *string   `json:"location_str,omitempty"`
		GeocodeConfidence *float64 `json:"geocode_confidence,omitempty"`
		Status           *string   `json:"status,omitempty"`
		ArtifactKey      *string   `json:"artifact_key,omitempty"`
		Hash             *string   `json:"hash,omitempty"`
		CreatedAt        time.Time `json:"created_at"`
		UpdatedAt        time.Time `json:"updated_at"`
	}

	var list []Permit
	for rows.Next() {
		var p Permit
		if err := rows.Scan(&p.ID, &p.SourceID, &p.SourceURL, &p.PermitType, &p.Title, &p.Description, &p.IssuingEntity, &p.StartDate, &p.EndDate, &p.LocationStr, &p.GeocodeConfidence, &p.Status, &p.ArtifactKey, &p.Hash, &p.CreatedAt, &p.UpdatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		list = append(list, p)
	}
	resp := gin.H{"items": list}
	if s.Cache != nil {
		if b, err := jsonMarshal(resp); err == nil {
			_ = s.Cache.Set(c.Request.Context(), cacheKey(c.Request.URL.Query()), string(b), 60*time.Second)
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) getPermit(c *gin.Context) {
	id := c.Param("id")
	if id == "" { c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"}); return }
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	row := s.DB.QueryRow(ctx, `SELECT id, source_id, source_url, permit_type, title, description, issuing_entity, start_date, end_date, location_str, geocode_confidence, status, artifact_key, hash, created_at, updated_at FROM permits WHERE id = $1`, id)
	type Permit struct {
		ID               string    `json:"id"`
		SourceID         string    `json:"source_id"`
		SourceURL        *string   `json:"source_url,omitempty"`
		PermitType       string    `json:"permit_type"`
		Title            *string   `json:"title,omitempty"`
		Description      *string   `json:"description,omitempty"`
		IssuingEntity    *string   `json:"issuing_entity,omitempty"`
		StartDate        *string   `json:"start_date,omitempty"`
		EndDate          *string   `json:"end_date,omitempty"`
		LocationStr      *string   `json:"location_str,omitempty"`
		GeocodeConfidence *float64 `json:"geocode_confidence,omitempty"`
		Status           *string   `json:"status,omitempty"`
		ArtifactKey      *string   `json:"artifact_key,omitempty"`
		Hash             *string   `json:"hash,omitempty"`
		CreatedAt        time.Time `json:"created_at"`
		UpdatedAt        time.Time `json:"updated_at"`
	}
	var p Permit
	if err := row.Scan(&p.ID, &p.SourceID, &p.SourceURL, &p.PermitType, &p.Title, &p.Description, &p.IssuingEntity, &p.StartDate, &p.EndDate, &p.LocationStr, &p.GeocodeConfidence, &p.Status, &p.ArtifactKey, &p.Hash, &p.CreatedAt, &p.UpdatedAt); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, p)
}

// cacheKey creates a normalized cache key from query parameters.
func cacheKey(query map[string][]string) string {
	keys := make([]string, 0, len(query))
	for k := range query { keys = append(keys, k) }
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString("=")
		vals := query[k]
		sort.Strings(vals)
		b.WriteString(strings.Join(vals, ","))
		b.WriteString("&")
	}
	sum := sha1.Sum([]byte(b.String()))
	return "permits:" + hex.EncodeToString(sum[:])
}

func getenv(k, def string) string { if v := os.Getenv(k); v != "" { return v }; return def }

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

