package mcp

import (
	"context"
	"fmt"

	domainHistory "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/history"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type HistoryHandler struct {
	historyService domainHistory.IHistoryUsecase
}

func InitMcpHistory(historyService domainHistory.IHistoryUsecase) *HistoryHandler {
	return &HistoryHandler{
		historyService: historyService,
	}
}

func (h *HistoryHandler) AddHistoryTools(s *server.MCPServer) {
	// Get history sync status tool
	getStatusTool := mcp.NewTool("whatsapp_history_status",
		mcp.WithDescription("Get the current history sync configuration and status. Returns information about whether history sync is enabled, auto-sync on login, and the maximum days of history configured."),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("The action to perform. Must be 'get_status'"),
			mcp.Enum("get_status"),
		),
	)

	s.AddTool(getStatusTool, h.handleGetHistoryStatus)

	// Request history sync tool
	requestSyncTool := mcp.NewTool("whatsapp_history_sync",
		mcp.WithDescription("Request history sync. History is automatically synced when connecting to WhatsApp based on the configured maximum days setting. Current configuration can be checked using whatsapp_history_status tool."),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("The action to perform. Must be 'request_sync'"),
			mcp.Enum("request_sync"),
		),
		mcp.WithString("chat_jid",
			mcp.Description("Optional: Specific chat JID to sync history for (format: phonenumber@s.whatsapp.net or groupid@g.us)"),
		),
		mcp.WithNumber("max_days",
			mcp.Description("Optional: Override the default maximum days of history to sync (90 = 3 months, 365 = 1 year, 730 = 2 years, 1095 = 3 years, -1 = all)"),
		),
	)

	s.AddTool(requestSyncTool, h.handleRequestHistorySync)
}

func (h *HistoryHandler) handleGetHistoryStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	status, err := h.historyService.GetHistorySyncStatus(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	fallback := fmt.Sprintf("History sync enabled: %v, max days: %d", status.Enabled, status.MaxDays)
	return mcp.NewToolResultStructured(status, fallback), nil
}

func (h *HistoryHandler) handleRequestHistorySync(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var req domainHistory.HistorySyncRequest

	// Parse optional chat_jid
	if chatJID, ok := request.GetArguments()["chat_jid"].(string); ok {
		req.ChatJID = chatJID
	}

	// Parse optional max_days
	if maxDays, ok := request.GetArguments()["max_days"].(float64); ok {
		req.MaxDays = int32(maxDays)
	}

	response, err := h.historyService.RequestHistorySync(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	fallback := fmt.Sprintf("History sync request: %s", response.Status)
	return mcp.NewToolResultStructured(response, fallback), nil
}
