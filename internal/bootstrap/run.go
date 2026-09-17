package bootstrap

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pchkauu/want-brief/internal/application"
	httpserver "github.com/pchkauu/want-brief/internal/delivery/http"
	"github.com/pchkauu/want-brief/internal/domain"
	"github.com/pchkauu/want-brief/internal/infrastructure/crypto"
	"github.com/pchkauu/want-brief/internal/infrastructure/jira"
	"github.com/pchkauu/want-brief/internal/infrastructure/postgres"
	"github.com/pchkauu/want-brief/internal/infrastructure/todoist"
)

type Config struct {
	DatabaseURL       string
	HTTPAddr          string
	CORSOrigin        string
	BootstrapPassword string
	TokenKey          string
	MigrationsDir     string
	MigrateOnly       bool
}

func LoadConfig() Config {
	migrateOnly := flag.Bool("migrate-only", false, "run migrations and exit")
	flag.Parse()
	return Config{
		DatabaseURL:       env("DATABASE_URL", "postgres://wantbrief:wantbrief@127.0.0.1:5432/wantbrief?sslmode=disable"),
		HTTPAddr:          env("HTTP_ADDR", ":8080"),
		CORSOrigin:        env("CORS_ORIGIN", "http://127.0.0.1:5173"),
		BootstrapPassword: env("BOOTSTRAP_PASSWORD", "wantbrief"),
		TokenKey:          env("TOKEN_KEY", "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"),
		MigrationsDir:     env("MIGRATIONS_DIR", "migrations"),
		MigrateOnly:       *migrateOnly,
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func Run(ctx context.Context, cfg Config) error {
	dir := cfg.MigrationsDir
	if !filepath.IsAbs(dir) {
		if wd, err := os.Getwd(); err == nil {
			dir = filepath.Join(wd, dir)
		}
	}
	if err := postgres.Migrate(cfg.DatabaseURL, dir); err != nil {
		return err
	}
	if cfg.MigrateOnly {
		return nil
	}

	store, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	box, err := crypto.NewBox(cfg.TokenKey)
	if err != nil {
		return err
	}

	app := &application.Service{
		Users:             postgres.UserRepo{Store: store},
		Sessions:          postgres.SessionRepo{Store: store},
		Projects:          postgres.ProjectRepo{Store: store},
		Sources:           postgres.SourceRepo{Store: store},
		Items:             postgres.ItemRepo{Store: store},
		Notes:             postgres.NoteRepo{Store: store},
		Intervals:         postgres.IntervalRepo{Store: store},
		Stress:            postgres.StressRepo{Store: store},
		Events:            postgres.EventRepo{Store: store},
		EventOverrides:    postgres.EventOverrideRepo{Store: store},
		EventNotes:        postgres.EventNoteRepo{Store: store},
		Companies:         postgres.CompanyRepo{Store: store},
		CompanyNotes:      postgres.CompanyNoteRepo{Store: store},
		People:            postgres.PersonRepo{Store: store},
		PersonNotes:       postgres.PersonNoteRepo{Store: store},
		PersonContacts:    postgres.PersonContactRepo{Store: store},
		PersonSites:       postgres.PersonSiteRepo{Store: store},
		PersonBonds:       postgres.PersonBondRepo{Store: store},
		PersonProfessions: postgres.PersonProfessionRepo{Store: store},
		PersonAbsences:    postgres.PersonAbsenceRepo{Store: store},
		ScheduleSettings:  postgres.ScheduleSettingsRepo{Store: store},
		DayOverrides:      postgres.DayOverrideRepo{Store: store},
		Snapshots:         postgres.ScheduleSnapshotRepo{Store: store},
		ProjectNotes:      postgres.ProjectNoteRepo{Store: store},
		ItemNotes:         postgres.ItemNoteRepo{Store: store},
		ItemChecks:        postgres.ItemCheckRepo{Store: store},
		ItemEvents:        postgres.ItemEventRepo{Store: store},
		Journal:           postgres.JournalRepo{Store: store},
		Tokens:            box,
		Pullers: map[domain.SourceKind]domain.Puller{
			domain.SourceJira:    jira.New(),
			domain.SourceTodoist: todoist.New(),
		},
		Password: cfg.BootstrapPassword,
	}
	if err := app.EnsureReady(ctx); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	srv := &httpserver.Server{App: app, CORSOrigin: cfg.CORSOrigin}
	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("want brief api on %s", cfg.HTTPAddr)
	return httpSrv.ListenAndServe()
}
