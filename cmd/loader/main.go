package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PermitIn struct {
	ID               string   `json:"id"`
	SourceSlug       string   `json:"source_slug"`
	SourceID         string   `json:"source_id"`
	SourceURL        string   `json:"source_url"`
	PermitType       string   `json:"permit_type"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	IssuingEntity    string   `json:"issuing_entity"`
	StartDate        string   `json:"start_date"`   // YYYY-MM-DD
	EndDate          string   `json:"end_date"`     // YYYY-MM-DD
	LocationStr      string   `json:"location_str"`
	Lon              *float64 `json:"lon,omitempty"`
	Lat              *float64 `json:"lat,omitempty"`
	GeocodeConfidence *float64 `json:"geocode_confidence,omitempty"`
	Status           string   `json:"status"`
	ArtifactKey      string   `json:"artifact_key"`
	Hash             string   `json:"hash"`
}

func main() {
	file := flag.String("file", "", "Path to input file (JSON array or CSV)")
	format := flag.String("format", "json", "Input format: json|csv")
	flag.Parse()

	if *file == "" {
		log.Fatal("-file is required")
	}
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		log.Fatal("POSTGRES_DSN is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil { log.Fatal(err) }
	defer pool.Close()

	perms, err := readPermits(*file, *format)
	if err != nil { log.Fatal(err) }

	if err := upsertPermits(ctx, pool, perms); err != nil { log.Fatal(err) }
	fmt.Printf("Loaded %d permits\n", len(perms))
}

func readPermits(path, format string) ([]PermitIn, error) {
	f, err := os.Open(path)
	if err != nil { return nil, err }
	defer f.Close()
	switch strings.ToLower(format) {
	case "json":
		var arr []PermitIn
		dec := json.NewDecoder(f)
		if err := dec.Decode(&arr); err != nil { return nil, err }
		return arr, nil
	case "csv":
		r := csv.NewReader(f)
		r.FieldsPerRecord = -1
		head, err := r.Read()
		if err != nil { return nil, err }
		idx := make(map[string]int)
		for i, h := range head { idx[strings.ToLower(h)] = i }
		var out []PermitIn
		for {
			rec, err := r.Read()
			if err == io.EOF { break }
			if err != nil { return nil, err }
			get := func(k string) string { if j, ok := idx[k]; ok && j < len(rec) { return rec[j] }; return "" }
			p := PermitIn{
				ID:            get("id"),
				SourceSlug:    get("source_slug"),
				SourceID:      get("source_id"),
				SourceURL:     get("source_url"),
				PermitType:    get("permit_type"),
				Title:         get("title"),
				Description:   get("description"),
				IssuingEntity: get("issuing_entity"),
				StartDate:     get("start_date"),
				EndDate:       get("end_date"),
				LocationStr:   get("location_str"),
				Status:        get("status"),
				ArtifactKey:   get("artifact_key"),
				Hash:          get("hash"),
			}
			// lon/lat parsing is optional; skip for brevity
			out = append(out, p)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

func upsertPermits(ctx context.Context, pool *pgxpool.Pool, arr []PermitIn) error {
	for _, p := range arr {
		// Resolve source_id from slug if needed
		sourceID := p.SourceID
		if sourceID == "" && p.SourceSlug != "" {
			if err := pool.QueryRow(ctx, `SELECT id FROM data_sources WHERE slug=$1`, p.SourceSlug).Scan(&sourceID); err != nil {
				return fmt.Errorf("resolve source_slug %s: %w", p.SourceSlug, err)
			}
		}
		if sourceID == "" { return fmt.Errorf("missing source_id or source_slug for permit: %v", p.SourceURL) }

		// Build geom SQL fragment if lon/lat provided
		geomExpr := "NULL"
		args := []any{p.ID, sourceID, nullIfEmpty(p.SourceURL), p.PermitType, nullIfEmpty(p.Title), nullIfEmpty(p.Description), nullIfEmpty(p.IssuingEntity), nullIfEmpty(p.StartDate), nullIfEmpty(p.EndDate), nullIfEmpty(p.LocationStr), p.GeocodeConfidence, nullIfEmpty(p.Status), nullIfEmpty(p.ArtifactKey), nullIfEmpty(p.Hash)}
		if p.Lon != nil && p.Lat != nil {
			geomExpr = "ST_SetSRID(ST_Point($15,$16),4326)"
			args = append(args, *p.Lon, *p.Lat)
		}

		q := `INSERT INTO permits (id, source_id, source_url, permit_type, title, description, issuing_entity, start_date, end_date, location_str, location_geom, geocode_confidence, status, artifact_key, hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7, NULLIF($8,'')::date, NULLIF($9,'')::date, $10, ` + geomExpr + `, $11, $12, $13, $14)
		ON CONFLICT (id) DO UPDATE SET
		  source_id=EXCLUDED.source_id,
		  source_url=EXCLUDED.source_url,
		  permit_type=EXCLUDED.permit_type,
		  title=EXCLUDED.title,
		  description=EXCLUDED.description,
		  issuing_entity=EXCLUDED.issuing_entity,
		  start_date=EXCLUDED.start_date,
		  end_date=EXCLUDED.end_date,
		  location_str=EXCLUDED.location_str,
		  location_geom=EXCLUDED.location_geom,
		  geocode_confidence=EXCLUDED.geocode_confidence,
		  status=EXCLUDED.status,
		  artifact_key=EXCLUDED.artifact_key,
		  hash=EXCLUDED.hash,
		  updated_at=now()`

		if _, err := pool.Exec(ctx, q, args...); err != nil {
			return err
		}
	}
	return nil
}

func nullIfEmpty(s string) any { if strings.TrimSpace(s) == "" { return nil }; return s }

