package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"secunda/internal/service"
)

type TeamService interface {
	Create(context.Context, int64, service.CreateTeamInput) (service.Team, error)
	List(context.Context, int64) ([]service.Team, error)
	Invite(context.Context, int64, service.InviteInput) (service.Membership, error)
}

type Teams struct {
	service TeamService
	logger  *slog.Logger
}

func NewTeams(service TeamService, logger *slog.Logger) *Teams {
	return &Teams{service: service, logger: logger}
}

// Create godoc
// @Summary Create a team with the current user as owner
// @Tags teams
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body CreateTeamRequest true "Team"
// @Success 201 {object} TeamResponse
// @Failure 400,401,413,415,500 {object} ErrorResponse
// @Router /teams [post]
func (h *Teams) Create(w http.ResponseWriter, r *http.Request) {
	var request CreateTeamRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	team, err := h.service.Create(r.Context(), actorID(r.Context()), request.toService())
	if err != nil {
		serviceError(h.logger, w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, teamResponse(team))
}

// List godoc
// @Summary List the current user's teams
// @Tags teams
// @Security BearerAuth
// @Produce json
// @Success 200 {object} TeamsResponse
// @Failure 401,500 {object} ErrorResponse
// @Router /teams [get]
func (h *Teams) List(w http.ResponseWriter, r *http.Request) {
	teams, err := h.service.List(r.Context(), actorID(r.Context()))
	if err != nil {
		serviceError(h.logger, w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, teamsResponse(teams))
}

// Invite godoc
// @Summary Add a registered user to a team
// @Description Owner may invite an admin or member. Admin may invite a member. Repeated invitations return 409 without changing the role.
// @Tags teams
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Team ID" minimum(1)
// @Param request body InviteRequest true "Invitation"
// @Success 201 {object} MembershipResponse
// @Failure 400,401,403,404,409,413,415,500 {object} ErrorResponse
// @Router /teams/{id}/invite [post]
func (h *Teams) Invite(w http.ResponseWriter, r *http.Request) {
	teamID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || teamID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "team id must be a positive integer")
		return
	}
	var request InviteRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	member, err := h.service.Invite(r.Context(), actorID(r.Context()), request.toService(teamID))
	if err != nil {
		serviceError(h.logger, w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, membershipResponse(member))
}
