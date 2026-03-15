package service

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/prodops-chronicles/prodops/internal/domain"
	"github.com/prodops-chronicles/prodops/internal/repository"
)

type PlayerService struct {
	players  repository.PlayerRepository
	progress repository.ProgressRepository
	runs     repository.RunRepository
}

func NewPlayerService(
	players repository.PlayerRepository,
	progress repository.ProgressRepository,
	runs repository.RunRepository,
) *PlayerService {
	return &PlayerService{players: players, progress: progress, runs: runs}
}

// SetupPlayer creates the player identity and profile for the first time.
func (s *PlayerService) SetupPlayer(ctx context.Context, displayName string) (*domain.PlayerIdentity, error) {
	if strings.TrimSpace(displayName) == "" {
		return nil, fmt.Errorf("%w: display name is required", domain.ErrInvalidInput)
	}
	existing, err := s.players.GetIdentity(ctx)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("%w: player already set up", domain.ErrConflict)
	}
	return s.players.CreateIdentity(ctx, displayName)
}

// DetectGitIdentities reads ~/.gitconfig and returns all [user] blocks found.
func (s *PlayerService) DetectGitIdentities(ctx context.Context) ([]domain.GitIdentity, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	gitconfigPath := home + "/.gitconfig"
	f, err := os.Open(gitconfigPath)
	if err != nil {
		return nil, nil
	}
	defer f.Close()

	var identities []domain.GitIdentity
	var current domain.GitIdentity
	inUserSection := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "[user]" {
			inUserSection = true
			current = domain.GitIdentity{Source: gitconfigPath}
			continue
		}
		if strings.HasPrefix(line, "[") {
			if inUserSection && (current.Username != "" || current.Email != "") {
				identities = append(identities, current)
			}
			inUserSection = false
			continue
		}
		if inUserSection {
			if strings.HasPrefix(line, "name") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					current.Username = strings.TrimSpace(parts[1])
				}
			}
			if strings.HasPrefix(line, "email") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					current.Email = strings.TrimSpace(parts[1])
				}
			}
		}
	}
	if inUserSection && (current.Username != "" || current.Email != "") {
		identities = append(identities, current)
	}
	return identities, scanner.Err()
}

// ConfirmGitIdentity saves git identity to private.player_profile
// and default branch name to public.player_setup_meta.
func (s *PlayerService) ConfirmGitIdentity(ctx context.Context, playerID uuid.UUID, runID uuid.UUID, identity domain.GitIdentity) error {
	if err := s.players.SetGitUsername(ctx, playerID, identity.Username, identity.Email); err != nil {
		return err
	}
	return s.progress.SetDefaultBranchName(ctx, runID, "main")
}

// SetSSHKeyPath records the path to the learner's SSH key (not the key itself).
func (s *PlayerService) SetSSHKeyPath(ctx context.Context, playerID uuid.UUID, path string) error {
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return fmt.Errorf("%w: SSH key not found at %s", domain.ErrInvalidInput, path)
	}
	return s.players.SetSSHKeyPath(ctx, playerID, path)
}

// SetTelemetryConsent writes to private.player_profile AND public.player_setup_meta.
func (s *PlayerService) SetTelemetryConsent(ctx context.Context, playerID uuid.UUID, runID uuid.UUID, consent domain.TelemetryConsent) error {
	if err := s.players.SetTelemetryConsent(ctx, playerID, consent); err != nil {
		return err
	}
	return s.progress.SetTelemetryConsent(ctx, runID, string(consent))
}
