package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prodops-chronicles/prodops/internal/domain"
	"github.com/prodops-chronicles/prodops/internal/repository"
)

type PlayerRepo struct{ pool *pgxpool.Pool }

func NewPlayerRepo(pool *pgxpool.Pool) *PlayerRepo {
	return &PlayerRepo{pool: pool}
}

func (r *PlayerRepo) GetIdentity(ctx context.Context) (*domain.PlayerIdentity, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, display_name, created_at, current_run_id
		FROM private.player_identity
		LIMIT 1
	`)
	p := &domain.PlayerIdentity{}
	err := row.Scan(&p.ID, &p.DisplayName, &p.CreatedAt, &p.CurrentRunID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	return p, err
}

func (r *PlayerRepo) CreateIdentity(ctx context.Context, displayName string) (*domain.PlayerIdentity, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	p := &domain.PlayerIdentity{}
	err = tx.QueryRow(ctx, `
		INSERT INTO private.player_identity (display_name)
		VALUES ($1)
		RETURNING id, display_name, created_at, current_run_id
	`, displayName).Scan(&p.ID, &p.DisplayName, &p.CreatedAt, &p.CurrentRunID)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO private.player_profile (player_id) VALUES ($1)
	`, p.ID)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO private.player_credentials (player_id) VALUES ($1)
	`, p.ID)
	if err != nil {
		return nil, err
	}

	return p, tx.Commit(ctx)
}

func (r *PlayerRepo) SetCurrentRun(ctx context.Context, playerID uuid.UUID, runID *uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE private.player_identity SET current_run_id = $1 WHERE id = $2
	`, runID, playerID)
	return err
}

func (r *PlayerRepo) GetProfile(ctx context.Context, playerID uuid.UUID) (*domain.PlayerProfile, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, player_id, git_username, git_email, ssh_key_path,
		       sync_remote, telemetry_consent, updated_at
		FROM private.player_profile
		WHERE player_id = $1
	`, playerID)
	p := &domain.PlayerProfile{}
	var gitUsername, gitEmail, sshKeyPath, syncRemote, consent *string
	err := row.Scan(&p.ID, &p.PlayerID, &gitUsername, &gitEmail,
		&sshKeyPath, &syncRemote, &consent, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if gitUsername != nil {
		p.GitUsername = *gitUsername
	}
	if gitEmail != nil {
		p.GitEmail = *gitEmail
	}
	if sshKeyPath != nil {
		p.SSHKeyPath = *sshKeyPath
	}
	if syncRemote != nil {
		p.SyncRemote = *syncRemote
	}
	if consent != nil {
		p.TelemetryConsent = domain.TelemetryConsent(*consent)
	}
	return p, nil
}

func (r *PlayerRepo) SetGitUsername(ctx context.Context, playerID uuid.UUID, username, email string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE private.player_profile
		SET git_username = $1, git_email = $2, updated_at = now()
		WHERE player_id = $3
	`, username, email, playerID)
	return err
}

func (r *PlayerRepo) SetSSHKeyPath(ctx context.Context, playerID uuid.UUID, path string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE private.player_profile
		SET ssh_key_path = $1, updated_at = now()
		WHERE player_id = $2
	`, path, playerID)
	return err
}

func (r *PlayerRepo) SetSyncRemote(ctx context.Context, playerID uuid.UUID, remote string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE private.player_profile
		SET sync_remote = $1, updated_at = now()
		WHERE player_id = $2
	`, remote, playerID)
	return err
}

func (r *PlayerRepo) SetTelemetryConsent(ctx context.Context, playerID uuid.UUID, consent domain.TelemetryConsent) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE private.player_profile
		SET telemetry_consent = $1, updated_at = now()
		WHERE player_id = $2
	`, string(consent), playerID)
	return err
}
