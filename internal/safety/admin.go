// Admin SOS operations: the moderation queue of active SOS events with ride
// context, plus resolution. Admins cannot trigger or list participant-level
// safety events here; these endpoints only operate on the response workflow.
package safety

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
)

// AdminActiveSOS lists active SOS events with ride context for the dispatch
// / moderation queue.
func (h *Handler) AdminActiveSOS(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	events, err := h.q.ListActiveSOSEvents(c.Request.Context(), int32(limit))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list active SOS events"})
		return
	}
	out := make([]gin.H, 0, len(events))
	for _, event := range events {
		out = append(out, adminSOSJSON(event))
	}
	c.JSON(http.StatusOK, gin.H{"sos": out})
}

type resolveSOSRequest struct {
	Note string `json:"note" binding:"max=500"`
}

// AdminResolveSOS closes an active SOS event. The optional note is appended
// as a ride audit event so the response trail is preserved.
func (h *Handler) AdminResolveSOS(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		return
	}
	sosID, err := uuid.Parse(c.Param("sosId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sos id"})
		return
	}
	var req resolveSOSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	event, err := h.q.ResolveRideSOSEvent(c.Request.Context(), db.ResolveRideSOSEventParams{
		ID: sosID, ResolvedBy: pgtype.UUID{Bytes: uid, Valid: true},
	})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "SOS event is not active"})
		return
	}
	payload := gin.H{"sosId": sosID, "resolvedBy": uid.String()}
	if req.Note != "" {
		payload["note"] = req.Note
	}
	if err := h.audit(c, event.RideID, "sos_resolved", uid, payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "SOS resolved but audit event failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sos": sosJSON(event)})
}

func adminSOSJSON(event db.ListActiveSOSEventsRow) gin.H {
	out := sosJSON(db.RideSosEvent{
		ID: event.ID, RideID: event.RideID, TriggeredBy: event.TriggeredBy,
		Status: event.Status, Location: event.Location, Note: event.Note,
		CreatedAt: event.CreatedAt,
	})
	out["rideStatus"] = event.RideStatus
	out["rideDriverId"] = event.RideDriverID
	out["rideRiderId"] = event.RideRiderID
	return out
}
