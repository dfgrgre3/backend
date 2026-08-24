package protected

const channelInApp = "in-app"
const errTicketNotFound = "Ticket not found"

// CreateTicketRequest represents a request to create a support ticket
type CreateTicketRequest struct {
	UserID            string `json:"userId" binding:"required"`
	Subject           string `json:"subject" binding:"required,max=200"`
	Description       string `json:"description" binding:"required,max=5000"`
	Category          string `json:"category" binding:"required,oneof=technical billing content account other"`
	Priority          string `json:"priority" binding:"omitempty,oneof=low medium high urgent"`
	RelatedEntityType string `json:"relatedEntityType,omitempty"`
	RelatedEntityID   string `json:"relatedEntityId,omitempty"`
}

// SendMessageRequest represents a message to be sent
type SendMessageRequest struct {
	Message    string `json:"message" binding:"required,max=5000"`
	IsInternal bool   `json:"isInternal"`
}

// UpdateTicketStatusRequest represents a status update
type UpdateTicketStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=open in_progress resolved closed escalated"`
}

// UpdateTicketPriorityRequest represents a priority update
type UpdateTicketPriorityRequest struct {
	Priority string `json:"priority" binding:"required,oneof=low medium high urgent"`
}
